package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	engine "github.com/Nabia-DB/nabia-core/engine"
)

func getURL(key string) string {
	var result string

	host := "http://localhost" // TODO ensure this is the default
	port := 5380               // TODO ensure this is the default
	result = host + ":" + fmt.Sprint(port) + key

	return result
}

func TestHTTP(t *testing.T) { // Tests the implementation of the HTTP API
	db := *engine.NewNabiaDB()
	go startServer(&db)

	var response *http.Response
	var err error

	table := []struct {
		verb         string
		key          string
		value        []byte // expected (GET)
		content_type string // expected (GET)
		status_code  int    // expected (all methods)
	}{
		{"HEAD", "/a1", []byte(nil), "", http.StatusNotFound}, // the DB must be empty on first boot
		{"GET", "/a1", []byte(nil), "", http.StatusNotFound},
		{"POST", "/a1", []byte("test"), "application/octet-stream", http.StatusOK}, // first upload
		{"HEAD", "/a1", []byte(nil), "", http.StatusOK},
		{"GET", "/a1", []byte("test"), "application/octet-stream", http.StatusOK},
		{"POST", "/a1", []byte("edited test"), "application/octet-stream", http.StatusOK}, // second upload, overwriting first upload
		{"GET", "/a1", []byte("edited test"), "application/octet-stream", http.StatusOK},
		{"DELETE", "/a1", []byte(nil), "", http.StatusOK}, // after deletion, no method should find /a1
		{"DELETE", "/a1", []byte(nil), "", http.StatusNotFound},
		{"HEAD", "/a1", []byte(nil), "", http.StatusNotFound},
		{"GET", "/a1", []byte(nil), "", http.StatusNotFound},
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
			// TODO implement deletion
			req, e := http.NewRequest("DELETE", getURL(row.key), nil)
			if e != nil {
				t.Errorf("Unexpected error when trying to %q %q.\n%s", row.verb, row.key, err.Error())
			}
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
