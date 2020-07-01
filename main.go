package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
)

type handlerFuncEx func(w http.ResponseWriter, req *http.Request) error

func (fn handlerFuncEx) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if err := fn(w, req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func await(rdb *redis.Client, id string, timeout time.Duration) bool {
	sub := rdb.Subscribe(context.Background(), id)
	defer sub.Unsubscribe(context.Background(), id)
	ch := sub.Channel()
	select {
	case <-ch:
		return true
	case <-time.After(time.Second * 15):
		return false
	}
}

func main() {
	var (
		host     = flag.String("c", "", "host of redis to connect")
		password = flag.String("p", "", "password of redis")
		db       = flag.Int("db", 0, "redis db")
		bind     = flag.String("bind", ":8765", "http bind")
	)

	flag.Parse()
	if *host == "" {
		flag.Usage()
		os.Exit(1)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     *host,
		Password: *password,
		DB:       *db,
	})

	if _, err := rdb.Ping(context.Background()).Result(); err != nil {
		fmt.Fprintln(os.Stderr, "failed to connect to redis", err)
		os.Exit(1)
	}

	fmt.Println("connected to redis @", *host)

	router := mux.NewRouter()

	// POST /{id}
	router.Path("/{id}").
		Methods(http.MethodPost).
		Handler(handlerFuncEx(func(w http.ResponseWriter, req *http.Request) error {
			data, err := ioutil.ReadAll(req.Body)
			if err != nil {
				return err
			}
			id := mux.Vars(req)["id"]
			exist, err := rdb.HExists(context.Background(), id, "d").Result()
			if err != nil {
				return err
			}

			if exist {
				w.WriteHeader(http.StatusAccepted)
				return nil
			}

			// set only when not exists
			contentType := req.Header.Get("content-type")
			origin := req.Header.Get("origin")
			if _, err := rdb.HMSet(context.Background(), id, map[string]interface{}{
				"d": string(data),
				"t": contentType,
				"o": origin,
			}).Result(); err != nil {
				return err
			}
			rdb.Publish(context.Background(), id, true)
			return nil
		}))

	// GET /{id}
	router.Path("/{id}").
		Methods(http.MethodGet).
		Handler(handlerFuncEx(func(w http.ResponseWriter, req *http.Request) error {
			id := mux.Vars(req)["id"]

			waitFlag := req.URL.Query().Get("wait")
			longPolling := waitFlag == "1" || waitFlag == "true"

			for {
				result, err := rdb.HMGet(context.Background(), id, "d", "t", "o").Result()
				if err != nil {
					return err
				}

				if result[0] != nil && result[1] != nil && result[2] != nil {
					w.Header().Set("content-type", result[1].(string))
					w.Header().Set("x-data-origin", result[2].(string))
					w.Write([]byte(result[0].(string)))
					return nil
				}

				if !longPolling || !await(rdb, id, time.Second*15) {
					break
				}
			}
			w.WriteHeader(http.StatusNoContent)
			return nil
		}))

	if err := http.ListenAndServe(*bind, router); err != nil {
		fmt.Fprintln(os.Stderr, "failed to listen http", err)
	}
}
