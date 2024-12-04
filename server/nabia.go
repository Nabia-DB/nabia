package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	engine "github.com/Nabia-DB/nabia/core/engine"
	"github.com/spf13/viper"
)

type NabiaHTTP struct {
	db *engine.NabiaDB
}

type nabiaServerRecord struct {
	data        []byte
	contentType string
}

func (nsr *nabiaServerRecord) GetRawData() []byte {
	return nsr.data
}

func (nsr *nabiaServerRecord) GetContentType() string {
	return nsr.contentType
}

func extractDataAndContentType(record *nabiaServerRecord) ([]byte, string, error) {
	return record.GetRawData(), record.GetContentType(), nil
}

func newNabiaServerRecord(data []byte, ct string) (*engine.NabiaRecord[nabiaServerRecord], error) {
	// TODO add content type validation
	nsr := nabiaServerRecord{
		data:        data,
		contentType: ct,
	}
	nr, err := engine.NewNabiaRecord(nsr)
	if err != nil {
		return nil, err
	}
	return nr, nil
}

func NewNabiaHttp(ns *engine.NabiaDB) *NabiaHTTP {
	return &NabiaHTTP{db: ns}
}

// These are the higher-level HTTP API calls exposed via the desired port, which
// in turn call the CRUD primitives from core.

func (h *NabiaHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var response []byte
	key := r.URL.Path
	clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		log.Printf("Error: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(nil)
		return
	} else {
		log.Printf("%s %s from %s", r.Method, key, clientIP)
	}
	switch r.Method {
	case "GET": // TODO tests
		// Only Read
		value, err := h.db.Read(key)
		if err != nil {
			log.Printf("Error: %s", err.Error())
			w.WriteHeader(http.StatusNotFound)
		} else {
			nsr := value.(engine.NabiaRecord[nabiaServerRecord])
			data, ct, err := extractDataAndContentType(&nsr.RawData)
			if err != nil {
				log.Printf("Error: %s", err.Error())
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				log.Printf("Info: Serving data from key %q", key)
				w.Header().Set("Content-Type", ct)
				response = data
			}
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
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println("Error: " + err.Error())
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			if h.db.Exists(key) {
				w.WriteHeader(http.StatusConflict)
			} else {
				ct := r.Header.Get("Content-Type")
				if ct == "" {
					ct = "application/octet-stream"
				} // TODO Content-Type validation needs more checks
				record, err := newNabiaServerRecord(body, ct)
				if err != nil {
					fmt.Printf("Error: %s", err)
					w.WriteHeader(http.StatusInternalServerError)
				} else {
					h.db.Write(key, *record)
					w.WriteHeader(http.StatusCreated)
				}
			}
		}
	case "PUT":
		// Overwrites if exists, otherwise creates
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println("Error: " + err.Error())
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			ct := r.Header.Get("Content-Type")
			if ct == "" {
				ct = "application/octet-stream" // Set generic Content-Type if not provided by the client
			}
			existed := h.db.Exists(key)
			record, err := newNabiaServerRecord(body, ct)
			if err != nil {
				fmt.Printf("Error: %s", err)
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				h.db.Write(key, *record)
				if existed {
					w.WriteHeader(http.StatusOK)
				} else {
					w.WriteHeader(http.StatusCreated)
				}
			}
		}
	case "DELETE": // TODO tests
		// Only Destroy
		if h.db.Exists(key) {
			engine.Delete(h.db, key)
			w.WriteHeader(http.StatusOK)
		} else { // TODO change if else with case
			w.WriteHeader(http.StatusNotFound)
			// TODO DRY
		}
	case "OPTIONS": // TODO complete
		// TODO tests
		if h.db.Exists(key) {
			w.Header().Set("Allow", "GET, PUT, DELETE, HEAD")
		} else {
			w.Header().Set("Allow", "PUT, POST, HEAD")
		}
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
	w.Write(response)
}

// startServer forks into a goroutine to make a server, then, making use of the
// ready channel, informs the caller when the server is ready to receive requests
func startServer(db *engine.NabiaDB, ready chan struct{}) {
	http_handler := NewNabiaHttp(db)
	viper.SetDefault("port", 5380)
	port := viper.GetString("port")
	log.Println("Listening on port " + port)
	server := &http.Server{Addr: ":" + port, Handler: http_handler}
	go func() {
		// Start the server
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()
	// Check if the server is ready by trying to connect to it
	for {
		conn, err := net.Dial("tcp", ":"+port)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		conn.Close()
		break
	}
	// Signal that the server is ready
	close(ready)
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
	log.Println("Found configuration file:", viper.ConfigFileUsed())

	dbLocation := viper.GetString("db_location")

	db, err := engine.NewNabiaDB(dbLocation)
	if err != nil {
		log.Fatalf("Failed to start NabiaDB: %s", err)
	}
	ready := make(chan struct{})
	startServer(db, ready)
	<-ready
	select {}
}
