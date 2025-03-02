package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	engine "github.com/Nabia-DB/nabia/core/engine"
	"github.com/spf13/viper"
)

type byteSlice []byte

type nabiaHTTP struct {
	db *engine.NabiaDB
}

type nabiaServerRecord struct {
	data        byteSlice
	contentType string
}

func (nsr nabiaServerRecord) getRawData() byteSlice {
	return nsr.data
}

func (nsr nabiaServerRecord) getContentType() string {
	return nsr.contentType
}

func (nsr nabiaServerRecord) extractDataAndContentType() (byteSlice, string, error) {
	return nsr.getRawData(), nsr.getContentType(), nil
}

func newNabiaServerRecord(data byteSlice, ct string) (*nabiaServerRecord, error) {
	nsr := nabiaServerRecord{
		data:        data,
		contentType: ct,
	}

	return &nsr, nil
}

func validateContentType(ct string) error {
	if len(ct) == 0 {
		return errors.New("Content-Type cannot be empty")
	}
	if !strings.Contains(ct, "/") {
		return errors.New("Content-Type must contain a '/'")
	}
	_, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return err
	}
	return nil
}

func (bs byteSlice) deserialize() (*nabiaServerRecord, error) {
	var version uint8  // Length of the version indicator is one byte
	var ctLength uint8 // Length of the content type is one byte
	var contentType string
	var ctBytes byteSlice
	nsr := &nabiaServerRecord{}
	buf := bytes.NewReader(bs)
	// Read version byte
	if err := binary.Read(buf, binary.LittleEndian, &version); err != nil { // Read version
		return nsr, err
	}
	if bytes.Equal([]byte{version}, []byte{0}) { // Check version of serialization
		if err := binary.Read(buf, binary.LittleEndian, &ctLength); err != nil { // Read content-type length
			return nsr, err
		}
		if !bytes.Equal([]byte{ctLength}, []byte{0}) { // Content-Type is not empty
			ctBytes = make(byteSlice, ctLength)
			if _, err := buf.Read(ctBytes); err != nil {
				return nsr, err
			}
			contentType = string(ctBytes)
			if err := validateContentType(contentType); err != nil {
				return nsr, err
			}
			data, err := io.ReadAll(buf)
			if err != nil {
				return nsr, err
			}
			nsr.data = data
			nsr.contentType = contentType
		} else {
			return nsr, fmt.Errorf("Content-Type cannot be empty (read CT length 0)")
		}
	} else {
		return nsr, fmt.Errorf("unsupported version: %d", version)
	}

	return nsr, nil
}

func (nsr nabiaServerRecord) serialize() (byteSlice, error) {
	currentVersion := uint8(0)
	if len(nsr.contentType) > int(math.MaxUint8) {
		// TODO test opportunity
		return nil, fmt.Errorf("Content-Type is too large; its length must be less than %d", math.MaxUint8)
	}

	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, currentVersion)
	binary.Write(&buf, binary.LittleEndian, uint8(len(nsr.contentType)))
	buf.WriteString(nsr.contentType)
	buf.Write(nsr.data)

	return buf.Bytes(), nil
}

// write is a wrapper for database write. It will always overwrite the data
func (h *nabiaHTTP) write(key string, nsr nabiaServerRecord) {
	if record, err := nsr.serialize(); err != nil {
		log.Println("Error: " + err.Error())
	} else {
		h.db.Write(key, record)
	}
}

// read is a wrapper for database Read
func (h *nabiaHTTP) read(key string) (nabiaServerRecord, error) {
	if record, err := h.db.Read(key); err != nil {
		return nabiaServerRecord{}, err
	} else {
		bs, error := byteSlice(record).deserialize()
		if error != nil {
			return nabiaServerRecord{}, error
		}
		return *bs, nil
	}
}

// delete wraps around DB Delete
func (h *nabiaHTTP) delete(key string) {
	h.db.Delete(key)
}

// exists wraps around DB Exists
func (h *nabiaHTTP) exists(key string) bool {
	return h.db.Exists(key)
}

func NewNabiaHttp(ns *engine.NabiaDB) *nabiaHTTP {
	return &nabiaHTTP{db: ns}
}

// These are the higher-level HTTP API calls exposed via the desired port, which
// in turn call the CRUD primitives from engine.
func (h *nabiaHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	case "GET":
		// Only Read
		nsr, error := h.read(key)
		if error != nil {
			log.Default().Println("Error: " + error.Error())
			w.WriteHeader(http.StatusNotFound)
			return
		}
		data, contentType, error := nsr.extractDataAndContentType()
		response = data
		w.Header().Set("Content-Type", contentType)
		if error != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	case "HEAD":
		w.Header().Del("Content-Type")
		// Only check if exists
		if h.exists(key) {
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
			break
		}
		if h.exists(key) {
			w.WriteHeader(http.StatusConflict)
			break
		}
		if len(body) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			break
		}
		ct := r.Header.Get("Content-Type")
		if err := validateContentType(ct); err != nil { // Content-Type must be valid
			w.WriteHeader(http.StatusBadRequest)
			break
		}
		nsr, err := newNabiaServerRecord(body, ct)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			break
		}
		h.write(key, *nsr)
		w.WriteHeader(http.StatusCreated)
	case "PUT":
		// Overwrites if exists, otherwise creates
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println("Error: " + err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			break
		}
		if len(body) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			break
		}
		ct := r.Header.Get("Content-Type")
		if err := validateContentType(ct); err != nil { // Content-Type must be valid
			w.WriteHeader(http.StatusBadRequest)
			break
		}
		nsr, err := newNabiaServerRecord(body, ct)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			break
		}
		h.write(key, *nsr)
		w.WriteHeader(http.StatusOK)
	case "DELETE":
		// Only Delete
		if h.exists(key) {
			h.delete(key)
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	case "OPTIONS":
		// TODO tests
		if h.exists(key) {
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
func startServer(db *engine.NabiaDB, ready chan struct{}, stopSignal <-chan struct{}) {
	http_handler := NewNabiaHttp(db)
	viper.SetDefault("port", 5380)
	port := viper.GetString("port")
	log.Println("Listening on port " + port)
	server := &http.Server{Addr: ":" + port, Handler: http_handler}

	serverErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
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

	go func() {
		<-stopSignal // Wait for shutdown signal

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			select {
			case err := <-serverErr:
				log.Printf("Server shutdown error: %v", err)
			default:
				log.Printf("Server forced to shutdown: %v", err)
			}
		}
		stopServer(db)
	}()

}

func stopServer(db *engine.NabiaDB) {
	log.Println()
	log.Println("Shutdown requested. Saving data...")
	db.Stop()
	log.Printf("Data saved to %q. Quitting...", viper.GetString("db_location"))
	os.Exit(0)
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
	stopSignal := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		close(stopSignal)
	}()
	startServer(db, ready, stopSignal)
	<-ready
	select {}
}
