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
		t.Errorf("Failed to create NabiaServerRecord: %q", err)
	}
	if !bytes.Equal(nsr.GetRawData(), []byte("test")) {
		t.Errorf("Unexpected data: %q", nsr.GetRawData())
	}
	if nsr.GetContentType() != "application/octet-stream" {
		t.Errorf("Unexpected content type: %q", nsr.GetContentType())
	}
}

func TestSerialization(t *testing.T) {
	nsr, err := newNabiaServerRecord([]byte("test"), "application/octet-stream")
	if err != nil {
		t.Errorf("Failed to create NabiaServerRecord: %q", err)
	}
	bs, err := nsr.serialize()
	if err != nil {
		t.Errorf("Failed to serialize: %q", err)
	}
	if !bytes.Equal(bs, []byte{4, 0, 0, 0, // Length of data (4)
		116, 101, 115, 116, // data ("test")
		97, 112, 112, 108, 105, 99, 97, 116, 105, 111, 110, 47, 111, 99, 116,
		101, 116, 45, 115, 116, 114, 101, 97, 109, // Content type ("application/octet-stream")
	}) {
		t.Errorf("Unexpected serialized data: %q", bs)
	}
}

func testDeserialization(t *testing.T) {
	var bs byteSlice = []byte{4, 0, 0, 0, // Length of data (4)
		116, 101, 115, 116, // data ("test")
		97, 112, 112, 108, 105, 99, 97, 116, 105, 111, 110, 47, 111, 99, 116,
		101, 116, 45, 115, 116, 114, 101, 97, 109, // Content type ("application/octet-stream")
	}
	nsr, err := bs.deserialize()
	if err != nil {
		t.Errorf("Failed to deserialize: %q", err)
	}
	if !bytes.Equal(nsr.GetRawData(), []byte("test")) {
		t.Errorf("Unexpected data: %q", nsr.GetRawData())
	}
	if nsr.GetContentType() != "application/octet-stream" {
		t.Errorf("Unexpected content type: %q", nsr.GetContentType())
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
	go startServer(db, serverReady)
	<-serverReady // blocks until ready

	var response *http.Response

	table := []struct {
		verb         string
		key          string
		value        []byte // expected (GET)
		content_type string // expected (GET)
		status_code  int    // expected (all methods)
	}{
		{"HEAD", "/a1", []byte(nil), "", http.StatusNotFound}, // the DB must be empty on first boot
		{"HEAD", "/a2", []byte(nil), "", http.StatusNotFound},
		{"GET", "/a1", []byte(nil), "", http.StatusNotFound},
		{"POST", "/a1", []byte("test"), "application/octet-stream", http.StatusCreated},  // first upload
		{"POST", "/a1", []byte("test"), "application/octet-stream", http.StatusConflict}, // second upload, should fail
		{"POST", "/a2", []byte("test2"), "application/octet-stream", http.StatusCreated}, // Uploading to different key should always work
		{"HEAD", "/a1", []byte(nil), "", http.StatusOK},
		{"GET", "/a1", []byte("test"), "application/octet-stream", http.StatusOK},
		{"PUT", "/a1", []byte("edited test"), "application/octet-stream", http.StatusOK}, // second upload, overwriting first upload
		{"GET", "/a1", []byte("edited test"), "application/octet-stream", http.StatusOK},
		{"DELETE", "/a1", []byte(nil), "", http.StatusOK}, // after deletion, no method should find /a1
		{"DELETE", "/a1", []byte(nil), "", http.StatusNotFound},
		{"HEAD", "/a1", []byte(nil), "", http.StatusNotFound},
		{"GET", "/a1", []byte(nil), "", http.StatusNotFound},
		{"GET", "/a2", []byte("test2"), "application/octet-stream", http.StatusOK}, // This one was never deleted, so it should still work
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

	// first HEAD should not return results (404)

	// first POST

	// TODO compare body
	// TODO compare content_type

	// TODO GET

	// TODO POST (update)

	// TODO HEAD

	// TODO DELETE

	// TODO HEAD

	// TODO POST to bad key

	// key = "/a/"

	// TODO GET from bad key

	// TODO POST bad content-type https://stackoverflow.com/questions/7924474/regex-to-extract-content-type

	// TODO GET bad content type https://stackoverflow.com/questions/7924474/regex-to-extract-content-type

}
