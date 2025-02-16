package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
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

func newNabiaServerRecord(data []byte, ct string) (nabiaServerRecord, error) {

	nsr := nabiaServerRecord{
		data:        data,
		contentType: ct,
	}

	return nsr, nil
}

func convertToNabiaServerRecord(record []byte) (nabiaServerRecord, error) {
	buf := bytes.NewReader(record)
	var dataLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &dataLen); err != nil {
		return nabiaServerRecord{}, err
	}
	data := make([]byte, dataLen)
	if _, err := buf.Read(data); err != nil {
		return nabiaServerRecord{}, err
	}
	contentType, err := io.ReadAll(buf)
	if err != nil {
		return nabiaServerRecord{}, err
	}
	return nabiaServerRecord{
		data:        data,
		contentType: string(contentType),
	}, nil
}

func convertToByteSlice(nsr *nabiaServerRecord) ([]byte, error) {
	if len(nsr.data) > int(math.MaxUint32) {
		// TODO test opportunity
		return nil, fmt.Errorf("data is too large; its length must be less than %d", math.MaxUint32)
	}

	var buf bytes.Buffer

	binary.Write(&buf, binary.LittleEndian, uint32(len(nsr.data)))
	buf.Write(nsr.data)
	buf.WriteString(nsr.contentType)

	return buf.Bytes(), nil
}

func (h *NabiaHTTP) Write(key string, nsr nabiaServerRecord) {
	record := convertToByteSlice(&nsr)
	h.db.Write(key, record)
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
		log.Printf("Error: %s\n", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(nil)
		return
	} else {
		log.Printf("%s %s from %s", r.Method, key, clientIP)
	}
	switch r.Method {
	case "GET": // TODO tests
		// Only Read
		// TODO: Read data from DB as NabiaRecord, convert to NabiaServerRecord, then extract data and content type
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
			// TODO: Read body and content type, create NSR, then convert to NR and write to DB, but only if it doesn't already exist
			if h.db.Exists(key) {
				w.WriteHeader(http.StatusConflict)
				// TODO elaborate
			} else {
				nsr := newNabiaServerRecord(body, r.Header.Get("Content-Type"))
				// TODO continue
			}
		}
	case "PUT":
		// Overwrites if exists, otherwise creates
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println("Error: " + err.Error())
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			// TODO similar story to POST, but always succeeds, overwriting where necessary
		}
	case "DELETE": // TODO tests
		// Only Delete
		if h.db.Exists(key) {
			h.db.Delete(key)
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
			// TODO DRY
		}
	case "OPTIONS":
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
