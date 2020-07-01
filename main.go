package main

import (
	"io/ioutil"
	"net/http"
	"sync"

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
		objectMap = make(map[string]*object)
		lock      sync.Mutex
		router    = mux.NewRouter()
	)

	router.Path("/{key}").
		Methods(http.MethodGet).
		Handler(handlerFuncEx(func(w http.ResponseWriter, req *http.Request) error {
			key := mux.Vars(req)["key"]
			var obj *object
			lock.Lock()
			obj = objectMap[key]
			lock.Unlock()

			if obj == nil {
				w.WriteHeader(http.StatusNotFound)
				return nil
			}
			w.Header().Set("content-type", obj.contentType)
			w.Write(obj.data)
			return nil
		}))

	router.Path("/{key}").
		Methods(http.MethodPost).
		Handler(handlerFuncEx(func(w http.ResponseWriter, req *http.Request) error {
			data, err := ioutil.ReadAll(req.Body)
			if err != nil {
				return err
			}
			key := mux.Vars(req)["key"]

			// long polling
			// wait:=req.URL.Query().Get("wait")
			// if wait == "1" || wait == "true" {

			// }

			lock.Lock()
			if objectMap[key] == nil {
				objectMap[key] = &object{data, req.Header.Get("content-type")}
			}
			lock.Unlock()

			w.WriteHeader(http.StatusOK)
			return nil
		}))

	http.ListenAndServe(":8765", router)
}
