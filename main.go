package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
)

type handlerFuncEx func(w http.ResponseWriter, req *http.Request) error

func (fn handlerFuncEx) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if err := fn(w, req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type object struct {
	data        []byte
	contentType string
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
			result, err := rdb.HExists(context.Background(), id, "d").Result()
			if err != nil {
				return err
			}
			// set only when not exists
			if !result {
				contentType := req.Header.Get("content-type")
				origin := req.Header.Get("origin")
				if _, err := rdb.HMSet(context.Background(), id, map[string]interface{}{
					"d": string(data),
					"t": contentType,
					"o": origin,
				}).Result(); err != nil {
					return err
				}
			}

			w.WriteHeader(http.StatusOK)
			return nil
		}))

	// GET /{id}
	router.Path("/{id}").
		Methods(http.MethodGet).
		Handler(handlerFuncEx(func(w http.ResponseWriter, req *http.Request) error {
			id := mux.Vars(req)["id"]

			result, err := rdb.HMGet(context.Background(), id, "d", "t", "o").Result()
			if err != nil {
				return err
			}

			if result[0] == nil || result[1] == nil || result[2] == nil {
				// TODO: long polling

				w.WriteHeader(http.StatusNoContent)
				return nil
			}

			w.Header().Set("content-type", result[1].(string))
			w.Header().Set("x-data-origin", result[2].(string))
			w.Write([]byte(result[0].(string)))
			return nil
		}))

	if err := http.ListenAndServe(*bind, router); err != nil {
		fmt.Fprintln(os.Stderr, "failed to listen http", err)
	}
}
