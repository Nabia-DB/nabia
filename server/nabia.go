package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"

	engine "github.com/Nabia-DB/nabia-core/engine"
	"github.com/spf13/viper"
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
	case "POST":
		// Creates if not exists, otherwise denies
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			if h.db.Exists(key) {
				w.WriteHeader(http.StatusConflict)
			} else {
				ct := r.Header.Get("Content-Type")
				if ct == "" {
					ct = "application/octet-stream"
				}
				record := engine.NewNabiaRecord(body, engine.ContentType(ct))
				h.db.Write(key, *record)
				w.WriteHeader(http.StatusCreated)
			}
		}
	case "PUT":
		// Overwrites if exists, otherwise creates
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			ct := r.Header.Get("Content-Type")
			if ct == "" {
				ct = "application/octet-stream" // Set generic Content-Type if not provided by the client
			}
			record := engine.NewNabiaRecord(body, engine.ContentType(ct))
			existed := h.db.Exists(key)
			h.db.Write(key, *record)
			if existed {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusCreated)
			}
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
		// Overwrites if exists, otherwise denies
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			if h.db.Exists(key) {
				ct := r.Header.Get("Content-Type")
				if ct == "" {
					ct = "application/octet-stream" // Set generic Content-Type if not provided by the client
				}
				record := engine.NewNabiaRecord(body, engine.ContentType(ct))
				h.db.Write(key, *record)
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}
	}
	io.WriteString(w, string(response))
}

func startServer(db *engine.NabiaDB) {
	http_handler := NewNabiaHttp(db)
	port := viper.GetString("port")
	log.Println("Listening on port " + port)
	http.ListenAndServe(":"+port, http_handler)
}

func main() {
	log.Println("Starting Nabia...")

	viper.SetConfigName("config")       // name of config file (without extension)
	viper.SetConfigType("yaml")         // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath("/etc/nabia/")  // path to look for the config file in
	viper.AddConfigPath("$HOME/.nabia") // call multiple times to add many search paths
	viper.AddConfigPath(".")            // optionally look for config in the working directory
	err := viper.ReadInConfig()         // Find and read the config file
	if err != nil {                     // Handle errors reading the config file
		panic(fmt.Errorf("fatal error config file: %s", err))
	}

	db := *engine.NewNabiaDB()
	startServer(&db)
}
