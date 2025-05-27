package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	engine "github.com/Nabia-DB/nabia/core/engine"
)

func getURL(key string) string {
	var result string

	host := "http://localhost" // TODO ensure this is the default
	port := 5380               // TODO ensure this is the default
	result = host + ":" + fmt.Sprint(port) + key

	return result
}

func cleanup(filename string, t *testing.T) {
	if _, err := os.Stat(filename); err == nil {
		// File exists, attempt to delete it
		err := os.Remove(filename)
		if err != nil {
			t.Fatalf("Failed to delete file: %q", err)
		}
	} else if !os.IsNotExist(err) {
		t.Fatalf("Unknown error: %q", err)
	}
}

func TestFundamentals(t *testing.T) {
	nsr, err := newNabiaServerRecord([]byte("test"), "application/octet-stream")
	if err != nil {
		t.Fatalf("Failed to create NabiaServerRecord: %q", err)
	}
	if !bytes.Equal(nsr.getRawData(), []byte("test")) {
		t.Fatalf("Unexpected data: %q", nsr.getRawData())
	}
	if nsr.getContentType() != "application/octet-stream" {
		t.Fatalf("Unexpected content type: %q", nsr.getContentType())
	}
}

func TestSerialization(t *testing.T) {
	nsr, err := newNabiaServerRecord([]byte("test"), "application/octet-stream")
	if err != nil {
		t.Fatalf("Failed to create NabiaServerRecord: %q", err)
	}
	bs, err := nsr.serialize()
	if err != nil {
		t.Fatalf("Failed to serialize: %q", err)
	}
	if !bytes.Equal(bs, []byte{0, // version
		24, // Length of Content-Type
		97, 112, 112, 108, 105, 99, 97, 116, 105, 111, 110, 47, 111, 99, 116,
		101, 116, 45, 115, 116, 114, 101, 97, 109, // Content type ("application/octet-stream")
		116, 101, 115, 116, // data ("test")
	}) {
		t.Fatalf("Unexpected serialized data: %q", bs)
	}
}

func TestDeserialization(t *testing.T) {
	var bs byteSlice = []byte{0, // version
		24, // Length of Content-Type
		97, 112, 112, 108, 105, 99, 97, 116, 105, 111, 110, 47, 111, 99, 116,
		101, 116, 45, 115, 116, 114, 101, 97, 109, // Content type ("application/octet-stream")
		116, 101, 115, 116, // data ("test")
	}
	nsr, err := bs.deserialize()
	if err != nil {
		t.Fatalf("Failed to deserialize: %q", err)
	}
	if !bytes.Equal(nsr.getRawData(), []byte("test")) {
		t.Fatalf("Unexpected data: %q", nsr.getRawData())
	}
	if nsr.getContentType() != "application/octet-stream" {
		t.Fatalf("Unexpected content type: %q", nsr.getContentType())
	}
}

func TestContentTypeValidation(t *testing.T) {
	table := []struct {
		ct   string
		pass bool
	}{
		{"application/octet-stream", true},
		{"application/octet-stream; charset=utf-8", true},
		{"application/octet-stream; charset=utf-8; boundary=abcdef", true},
		{"application/octet-stream; charset=utf-8; boundary=abcdef; q=0.5", true},
		{"", false},
		{"a", false},
		{" a ", false},
		{"a / a", false},
	}
	for _, row := range table {
		err := validateContentType(row.ct)
		if row.pass && err != nil {
			t.Errorf("Unexpected error with Content-Type %q: %q", row.ct, err)
		}
		if !row.pass && err == nil {
			t.Errorf("Expected error on Content-Type %q", row.ct)
		}
	}
}

func TestHTTP(t *testing.T) { // Tests the implementation of the HTTP API
	filename := "test.db"
	cleanup(filename, t)
	defer cleanup(filename, t) // Ensure cleaning up test files

	db, err := engine.NewNabiaDB(filename)
	if err != nil {
		t.Errorf("Failed to create Nabia DB: %q", err)
	}
	serverReady := make(chan struct{})
	stopServer := make(chan struct{})
	defer close(stopServer) // Ensure server is stopped
	go startServer(db, serverReady, stopServer)
	<-serverReady // blocks until ready

	var response *http.Response

	table := []struct {
		verb         string
		key          string
		value        []byte // expected (GET only)
		content_type string // expected (GET only)
		status_code  int    // expected (all methods)
	}{
		{"HEAD", "/a1", []byte(nil), "", http.StatusNotFound},                                        // the DB must be empty on first boot
		{"HEAD", "/a2", []byte(nil), "", http.StatusNotFound},                                        // the DB must be empty on first boot
		{"GET", "/a1", []byte(nil), "", http.StatusNotFound},                                         // the DB must be empty on first boot
		{"POST", "/a1", []byte("test"), "application/octet-stream", http.StatusCreated},              // first upload
		{"POST", "/a1", []byte("test"), "application/octet-stream", http.StatusConflict},             // second upload, should fail because POST doesn't overwrite
		{"POST", "/a2", []byte("test2"), "application/octet-stream", http.StatusCreated},             // Uploading to different key should always work
		{"HEAD", "/a1", []byte(nil), "", http.StatusOK},                                              // after upload, all methods should find /a1
		{"GET", "/a1", []byte("test"), "application/octet-stream", http.StatusOK},                    // after upload, all methods should find /a1
		{"PUT", "/a1", []byte("edited test"), "application/octet-stream", http.StatusOK},             // second upload, overwriting first upload
		{"GET", "/a1", []byte("edited test"), "application/octet-stream", http.StatusOK},             // after upload, all methods should find /a1
		{"DELETE", "/a1", []byte(nil), "", http.StatusOK},                                            // after deletion, no method should find /a1
		{"DELETE", "/a1", []byte(nil), "", http.StatusNotFound},                                      // after deletion, deleting again should fail
		{"HEAD", "/a1", []byte(nil), "", http.StatusNotFound},                                        // after deletion, no method should find /a1
		{"GET", "/a1", []byte(nil), "", http.StatusNotFound},                                         // after deletion, no method should find /a1
		{"GET", "/a2", []byte("test2"), "application/octet-stream", http.StatusOK},                   // This one was never deleted, so it should still work
		{"POST", "/NoContentType", []byte("random text"), "", http.StatusBadRequest},                 // This should fail because of missing Content-Type
		{"PUT", "/NoContentType", []byte("random text"), "", http.StatusBadRequest},                  // This should fail because of missing Content-Type
		{"POST", "/BadContentType", []byte("data data data"), " fakeCT abcd", http.StatusBadRequest}, // This should fail because of bad Content-Type
		{"POST", "/NoSlash", []byte("data data data"), "fakeCTabcd", http.StatusBadRequest},          // This should fail because of bad Content-Type
		{"PUT", "/BadContentType", []byte("data data data"), " fakeCT abcd", http.StatusBadRequest},  // This should fail because of bad Content-Type
		{"POST", "/NoData", []byte(""), "application/octet-stream", http.StatusBadRequest},           // This should fail because of empty data
		{"PUT", "/NoData", []byte(""), "application/octet-stream", http.StatusBadRequest},            // This should fail because of empty data
	}

	for _, row := range table {
		switch row.verb {
		case "POST":
			response, err = http.Post(getURL(row.key), row.content_type,
				bytes.NewReader(row.value))
		case "GET":
			response, err = http.Get(getURL(row.key))
		case "HEAD":
			response, err = http.Head(getURL(row.key))
		case "DELETE":
			req, e := http.NewRequest("DELETE", getURL(row.key), nil)
			if e != nil {
				t.Errorf("Unexpected error when trying to %q %q.\n%s", row.verb, row.key, err.Error())
			}
			response, err = http.DefaultClient.Do(req)
		case "PUT":
			req, e := http.NewRequest("PUT", getURL(row.key), bytes.NewReader(row.value))
			if e != nil {
				t.Errorf("Unexpected error when trying to %q %q.\n%s", row.verb, row.key, err.Error())
			}
			req.Header.Set("Content-Type", row.content_type)
			response, err = http.DefaultClient.Do(req)
		default:
			err = errors.New("Unknown method " + row.verb)
		}
		// There are seven things to check:
		// - "err" should always be nil, otherwise there may be connection problems
		// - Status code must match
		// - Response body must match for GET
		// - Response body for other verbs must be empty
		// - Content-Type must match for GET
		// - Any verb to malformed keys must never succeed. A malformed key doesn't
		// match the RegEx: `^\/[\w\d\/]+[^\/]$`
		// - Any malformed POSTed Content-Type must be replaced with "application/octetstream".
		// All correct Content-Types are listed on https://www.iana.org/assignments/media-types/media-types.xhtml .
		// A bad Content-Type doesn't match the RegEx:
		// application|audio|font|image|message|model|multipart|text|video\/[\w\.\-\+]+
		if err == nil {
			if row.status_code != response.StatusCode {
				t.Errorf("Unexpected status code when trying to %q %q.\n",
					row.verb, row.key)
				t.Errorf("Got %q, expected %q.",
					fmt.Sprint(response.StatusCode),
					fmt.Sprint(row.status_code))
			}
			response_body, response_error := ioutil.ReadAll(response.Body)
			if response_error != nil {
				t.Errorf("Unexpected error when accessing response body %q.\n",
					response_error.Error())
			} else {
				if row.verb == "GET" { // Check Content-Type and body with GET
					if response.Header.Get("Content-Type") != row.content_type {
						t.Errorf("Unexpected Content-Type when %q %q.\n",
							row.verb, row.key)
						t.Errorf("Got %q, expected %q.",
							fmt.Sprint(response.Header.Get("Content-Type")),
							fmt.Sprint(row.content_type))
					}
					if !bytes.Equal(response_body, row.value) {
						t.Errorf("Unexpected []byte when %q %q.\n", row.verb, row.key)
						t.Errorf("Got %s, expected %s.", response_body, row.value)
					}
				} else {
					if !bytes.Equal(response_body, []byte(nil)) { // body must be empty when not using GET, including POST
						t.Errorf("Unexpected []byte when %q %q.\n", row.verb, row.key)
						t.Errorf("Got %s, expected %s.", response_body, "nil")
					}
				}
			}
		} else { // this should never happen
			t.Errorf("Unexpected error when trying to %q %q.\n%s",
				row.verb, row.key, err.Error())
		}
	}
}
