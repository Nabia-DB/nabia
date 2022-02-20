package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	engine "github.com/Nabia-DB/nabia-core/engine"
)

type NabiaHTTP struct {
	db *engine.NabiaDB
}

func NewNabiaHttp(ns *engine.NabiaDB) *NabiaHTTP {
	return &NabiaHTTP{db: ns}
}

// These are the higher-level HTTP API calls exposed via the desired port, which
// in turn call the CRUD primitives.

func (h *NabiaHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var response []byte
	key := r.URL.Path
	switch r.Method {
	case "GET": // TODO tests
		// Only Read
		value, err := h.db.Read(key)
		if err != nil {
			// TODO handle error
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.Header().Set("Content-Type", string(value.ContentType))
			response = value.RawData
		}
	case "HEAD": // TODO tests
		w.Header().Del("Content-Type")
		// Only check if exists
		if h.db.Exists(key) {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
		response = nil
	case "POST": // TODO tests
		// Writes without checking, overwriting where necessary
		// TODO POST should only create
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			ct := r.Header.Get("Content-Type")
			if ct == "" {
				ct = "application/octet-stream" // Set generic Content-Type if not provided by the client
			}
			record := engine.NewNabiaRecord(body, engine.ContentType(ct))
			h.db.Write(key, *record)
		}
	case "DELETE": // TODO tests
		// Only Destroy
		if h.db.Exists(key) {
			h.db.Destroy(key)
			w.WriteHeader(http.StatusOK)
		} else { // TODO change if else with case
			w.WriteHeader(http.StatusNotFound)
			// TODO DRY
		}
	case "PATCH": // TODO complete
		// ! use https://docs.microsoft.com/en-us/iis-administration/api/crud#update-patch--put as reference
	}
	io.WriteString(w, string(response))
}

func startServer(db *engine.NabiaDB) {
	http_handler := NewNabiaHttp(db)
	http.ListenAndServe(":5380", http_handler)
}

func main() {
	fmt.Println("Starting Nabia...")
	db := *engine.NewNabiaDB()
	startServer(&db)
}
