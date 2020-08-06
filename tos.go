package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/mat/besticon/besticon"
	"github.com/qianbin/tos/kv"
)

type handlerFuncEx func(w http.ResponseWriter, req *http.Request) error

func (fn handlerFuncEx) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if err := fn(w, req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type entity struct {
	C []byte // content
	T string // content type
	O string // origin
}

func poll(ctx context.Context, kv kv.KV, key string, sec int) ([]byte, error) {
	for i := 0; i < sec; i++ {
		val, err := kv.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		if len(val) > 0 {
			return val, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, nil
}

func main() {
	var (
		url  = flag.String("c", "", "url of remote store to connect")
		bind = flag.String("bind", ":5678", "http bind")
	)

	flag.Parse()

	kv, err := kv.New(context.Background(), *url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to connect to remote store", err)
		os.Exit(1)
	}
	if *url != "" {
		fmt.Println("connected to", *url)
	}

	router := mux.NewRouter()

	// POST /{id}
	router.Path("/{id}").
		Methods(http.MethodPost).
		Handler(handlerFuncEx(func(w http.ResponseWriter, req *http.Request) error {
			content, err := ioutil.ReadAll(req.Body)
			if err != nil {
				return err
			}
			id := mux.Vars(req)["id"]

			entity := &entity{
				C: content,
				T: req.Header.Get("content-type"),
				O: req.Header.Get("origin"),
			}
			data, err := json.Marshal(entity)
			if err != nil {
				return err
			}

			// TODO race condition
			exist, err := kv.Get(req.Context(), id)
			if err != nil {
				return err
			}
			if len(exist) > 0 {
				// exists
				// ok if data not changed
				if bytes.Compare(exist, data) == 0 {
					return nil
				}
				// reject mutating data
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte("id already exists"))
				return nil
			}

			if err := kv.Set(req.Context(), id, data, time.Minute*10); err != nil {
				return err
			}
			return nil
		}))

	// GET /{id}
	router.Path("/{id}").
		Methods(http.MethodGet).
		Handler(handlerFuncEx(func(w http.ResponseWriter, req *http.Request) (err error) {
			var (
				id          = mux.Vars(req)["id"]
				waitFlag    = req.FormValue("wait")
				longPolling = waitFlag == "1" || waitFlag == "true"
				data        []byte
			)

			if longPolling {
				data, err = poll(req.Context(), kv, id, 15)
			} else {
				data, err = kv.Get(req.Context(), id)
			}

			if err != nil {
				return err
			}

			if len(data) == 0 {
				w.WriteHeader(http.StatusNoContent)
				return nil
			}

			var entity entity
			if err := json.Unmarshal(data, &entity); err != nil {
				return err
			}
			if entity.T != "" {
				w.Header().Set("content-type", entity.T)
			}
			if entity.O != "" {
				w.Header().Set("x-data-origin", entity.O)
			}
			w.Write(entity.C)

			return nil
		}))

	router.Path("/{id}/icon").
		Methods(http.MethodGet).
		Handler(handlerFuncEx(func(w http.ResponseWriter, req *http.Request) (err error) {
			var (
				id      = mux.Vars(req)["id"]
				size    = req.FormValue("size")
				formats = req.FormValue("formats")
			)

			sizeRange, err := besticon.ParseSizeRange(size)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return nil
			}

			data, err := kv.Get(req.Context(), id)
			if err != nil {
				return err
			}

			if len(data) == 0 {
				w.WriteHeader(http.StatusNoContent)
				return nil
			}
			var entity entity
			if err := json.Unmarshal(data, &entity); err != nil {
				return err
			}

			finder := besticon.IconFinder{}
			if formats != "" {
				finder.FormatsAllowed = strings.Split(formats, ",")
			}

			finder.FetchIcons(entity.O)

			icon := finder.IconInSizeRange(*sizeRange)
			if icon != nil {
				http.Redirect(w, req, icon.URL, http.StatusFound)
				return
			}

			w.WriteHeader(http.StatusNoContent)
			return
		}))

	listener, err := net.Listen("tcp", *bind)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to listen http on", *bind, err)
	}

	fmt.Println("http listening on", listener.Addr())

	http.Serve(listener, router)
}
