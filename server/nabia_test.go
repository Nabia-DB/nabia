package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

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

func TestNabiaHTTPMethods(t *testing.T) {
	filename := "test_http_methods.db"
	cleanup(filename, t)
	defer cleanup(filename, t)

	db, err := engine.NewNabiaDB(filename)
	if err != nil {
		t.Fatalf("Failed to create Nabia DB: %q", err)
	}
	defer db.Stop()

	h := NewNabiaHttp(db)

	// Test write method
	testData := []byte("test data")
	testCT := "text/plain"
	nsr, _ := newNabiaServerRecord(testData, testCT)
	h.write("/test-key", *nsr)

	// Test read method
	readNsr, err := h.read("/test-key")
	if err != nil {
		t.Errorf("Failed to read data: %q", err)
	}
	if !bytes.Equal(readNsr.getRawData(), testData) {
		t.Errorf("Read data mismatch: got %q, expected %q", readNsr.getRawData(), testData)
	}
	if readNsr.getContentType() != testCT {
		t.Errorf("Content type mismatch: got %q, expected %q", readNsr.getContentType(), testCT)
	}

	// Test exists method
	if !h.exists("/test-key") {
		t.Error("Key should exist after write")
	}

	// Test delete method
	h.delete("/test-key")
	if h.exists("/test-key") {
		t.Error("Key should not exist after delete")
	}

	// Test read non-existent key
	_, err = h.read("/non-existent")
	if err == nil {
		t.Error("Reading non-existent key should return error")
	}
}

func TestSerializationEdgeCases(t *testing.T) {
	// Test with very long content type (max allowed is 255)
	longCT := strings.Repeat("a", 200) + "/test"
	nsr, err := newNabiaServerRecord([]byte("data"), longCT)
	if err != nil {
		t.Fatalf("Failed to create record with long content type: %q", err)
	}
	
	serialized, err := nsr.serialize()
	if err != nil {
		t.Fatalf("Failed to serialize with long content type: %q", err)
	}
	
	deserialized, err := serialized.deserialize()
	if err != nil {
		t.Fatalf("Failed to deserialize with long content type: %q", err)
	}
	
	if deserialized.getContentType() != longCT {
		t.Errorf("Content type mismatch after serialization: got %q, expected %q", 
			deserialized.getContentType(), longCT)
	}

	// Test with content type that's too long
	tooLongCT := strings.Repeat("a", 256) + "/test"
	nsr2, _ := newNabiaServerRecord([]byte("data"), tooLongCT)
	_, err = nsr2.serialize()
	if err == nil {
		t.Error("Serialization should fail with content type > 255 chars")
	}

	// Test deserialization with invalid version
	invalidVersion := []byte{1, 10, 97, 47, 98, 116, 101, 115, 116}
	_, err = byteSlice(invalidVersion).deserialize()
	if err == nil {
		t.Error("Deserialization should fail with unsupported version")
	}

	// Test deserialization with empty content type
	emptyContentType := []byte{0, 0} // version 0, content type length 0
	_, err = byteSlice(emptyContentType).deserialize()
	if err == nil {
		t.Error("Deserialization should fail with empty content type")
	}

	// Test with large data payload
	largeData := make([]byte, 10000)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	nsr3, _ := newNabiaServerRecord(largeData, "application/octet-stream")
	serialized3, err := nsr3.serialize()
	if err != nil {
		t.Fatalf("Failed to serialize large data: %q", err)
	}
	
	deserialized3, err := serialized3.deserialize()
	if err != nil {
		t.Fatalf("Failed to deserialize large data: %q", err)
	}
	
	if !bytes.Equal(deserialized3.getRawData(), largeData) {
		t.Error("Large data mismatch after serialization")
	}
}

func TestHTTPOptionsMethod(t *testing.T) {
	filename := "test_options.db"
	cleanup(filename, t)
	defer cleanup(filename, t)

	db, err := engine.NewNabiaDB(filename)
	if err != nil {
		t.Fatalf("Failed to create Nabia DB: %q", err)
	}
	serverReady := make(chan struct{})
	stopServer := make(chan struct{})
	defer close(stopServer)
	go startServer(db, serverReady, stopServer)
	<-serverReady

	// Test OPTIONS for non-existent key
	req, _ := http.NewRequest("OPTIONS", getURL("/new-key"), nil)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to send OPTIONS request: %q", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", response.StatusCode)
	}
	allowHeader := response.Header.Get("Allow")
	if allowHeader != "PUT, POST, HEAD" {
		t.Errorf("Expected Allow header 'PUT, POST, HEAD', got %q", allowHeader)
	}

	// Create a key
	http.Post(getURL("/existing-key"), "text/plain", bytes.NewReader([]byte("test")))

	// Test OPTIONS for existing key
	req2, _ := http.NewRequest("OPTIONS", getURL("/existing-key"), nil)
	response2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("Failed to send OPTIONS request: %q", err)
	}
	if response2.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", response2.StatusCode)
	}
	allowHeader2 := response2.Header.Get("Allow")
	if allowHeader2 != "GET, PUT, DELETE, HEAD" {
		t.Errorf("Expected Allow header 'GET, PUT, DELETE, HEAD', got %q", allowHeader2)
	}
}

func TestContentTypeEdgeCases(t *testing.T) {
	table := []struct {
		ct   string
		pass bool
	}{
		{"text/plain", true},
		{"text/plain; charset=utf-8", true},
		{"application/json", true},
		{"multipart/form-data; boundary=----WebKitFormBoundary", true},
		{"image/png", true},
		{"video/mp4", true},
		{"application/x-custom+json", true},
		{"text/plain;charset=utf-8;q=0.8", true},
		{"application/vnd.api+json", true},
		{strings.Repeat("a", 255), false}, // too long without slash
		{strings.Repeat("a", 250) + "/test", true}, // long but valid
		{"text/", false}, // missing subtype
		{"/plain", false}, // missing type
		{"text//plain", false}, // double slash
		{"text plain", false}, // space instead of slash
		{"text/plain; invalid", false}, // invalid parameter
		{"", false}, // empty
		{" ", false}, // whitespace only
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

func TestErrorHandlingInHTTPMethods(t *testing.T) {
	filename := "test_errors.db"
	cleanup(filename, t)
	defer cleanup(filename, t)

	db, err := engine.NewNabiaDB(filename)
	if err != nil {
		t.Fatalf("Failed to create Nabia DB: %q", err)
	}
	serverReady := make(chan struct{})
	stopServer := make(chan struct{})
	defer close(stopServer)
	go startServer(db, serverReady, stopServer)
	<-serverReady

	// Test invalid HTTP method
	req, _ := http.NewRequest("PATCH", getURL("/test-key"), nil)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to send PATCH request: %q", err)
	}
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", response.StatusCode)
	}

	// Test PUT with missing Content-Type
	req2, _ := http.NewRequest("PUT", getURL("/test-key"), bytes.NewReader([]byte("data")))
	// Explicitly not setting Content-Type
	response2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("Failed to send PUT request: %q", err)
	}
	if response2.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 for missing Content-Type, got %d", response2.StatusCode)
	}

	// Test POST with very long Content-Type
	req3, _ := http.NewRequest("POST", getURL("/test-key"), bytes.NewReader([]byte("data")))
	req3.Header.Set("Content-Type", strings.Repeat("a", 260)+"/test")
	response3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatalf("Failed to send POST request: %q", err)
	}
	if response3.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 for too long Content-Type, got %d", response3.StatusCode)
	}
}

func TestConcurrentRequests(t *testing.T) {
	filename := "test_concurrent.db"
	cleanup(filename, t)
	defer cleanup(filename, t)

	db, err := engine.NewNabiaDB(filename)
	if err != nil {
		t.Fatalf("Failed to create Nabia DB: %q", err)
	}
	serverReady := make(chan struct{})
	stopServer := make(chan struct{})
	defer close(stopServer)
	go startServer(db, serverReady, stopServer)
	<-serverReady

	// Run multiple concurrent requests
	numRequests := 10
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(index int) {
			key := fmt.Sprintf("/concurrent-key-%d", index)
			data := fmt.Sprintf("data-%d", index)
			
			// POST request
			resp, err := http.Post(getURL(key), "text/plain", bytes.NewReader([]byte(data)))
			if err != nil {
				t.Errorf("Concurrent POST failed: %q", err)
			} else if resp.StatusCode != http.StatusCreated {
				t.Errorf("Expected status 201, got %d", resp.StatusCode)
			}
			
			// GET request
			resp2, err := http.Get(getURL(key))
			if err != nil {
				t.Errorf("Concurrent GET failed: %q", err)
			} else if resp2.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp2.StatusCode)
			}
			
			done <- true
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		<-done
	}
}

func TestExtractDataAndContentType(t *testing.T) {
	testData := []byte("test data")
	testCT := "text/plain"
	nsr, _ := newNabiaServerRecord(testData, testCT)
	
	data, ct, err := nsr.extractDataAndContentType()
	if err != nil {
		t.Errorf("Unexpected error: %q", err)
	}
	if !bytes.Equal(data, testData) {
		t.Errorf("Data mismatch: got %q, expected %q", data, testData)
	}
	if ct != testCT {
		t.Errorf("Content type mismatch: got %q, expected %q", ct, testCT)
	}
}

func TestServerShutdown(t *testing.T) {
	filename := "test_shutdown.db"
	cleanup(filename, t)
	defer cleanup(filename, t)

	db, err := engine.NewNabiaDB(filename)
	if err != nil {
		t.Fatalf("Failed to create Nabia DB: %q", err)
	}
	
	serverReady := make(chan struct{})
	stopSignal := make(chan struct{})
	
	// Start server
	go startServer(db, serverReady, stopSignal)
	<-serverReady
	
	// Add some data
	http.Post(getURL("/shutdown-test"), "text/plain", bytes.NewReader([]byte("test data")))
	
	// Trigger shutdown
	close(stopSignal)
	
	// Give server time to shut down gracefully
	time.Sleep(100 * time.Millisecond)
	
	// Verify server is no longer accepting connections
	_, err = http.Get(getURL("/shutdown-test"))
	if err == nil {
		t.Error("Server should not accept connections after shutdown")
	}
}

func TestSpecialCharactersInKeys(t *testing.T) {
	filename := "test_special_keys.db"
	cleanup(filename, t)
	defer cleanup(filename, t)

	db, err := engine.NewNabiaDB(filename)
	if err != nil {
		t.Fatalf("Failed to create Nabia DB: %q", err)
	}
	serverReady := make(chan struct{})
	stopServer := make(chan struct{})
	defer close(stopServer)
	go startServer(db, serverReady, stopServer)
	<-serverReady

	specialKeys := []struct {
		key    string
		expect string // "success", "error", or "url_error"
	}{
		{"/key-with-dash", "success"},
		{"/key_with_underscore", "success"},
		{"/key.with.dots", "success"},
		{"/path/to/nested/key", "success"},
		{"/KEY_WITH_CAPS", "success"},
		{"/key123", "success"},
		{"/123key", "success"},
		{"/key!@#$%", "url_error"}, // URL parsing will fail
	}

	for _, test := range specialKeys {
		data := []byte("test data for " + test.key)
		resp, err := http.Post(getURL(test.key), "text/plain", bytes.NewReader(data))
		
		if test.expect == "url_error" {
			if err == nil {
				t.Errorf("Expected URL error for key %q but got none", test.key)
			}
			continue
		}
		
		if err != nil {
			t.Errorf("Failed to POST to key %q: %q", test.key, err)
			continue
		}
		
		if test.expect == "success" {
			if resp.StatusCode != http.StatusCreated {
				t.Errorf("Key %q should be accepted, got status %d", test.key, resp.StatusCode)
			} else {
				// Verify we can read it back
				getResp, err := http.Get(getURL(test.key))
				if err != nil {
					t.Errorf("Failed to GET key %q: %q", test.key, err)
				} else if getResp.StatusCode != http.StatusOK {
					t.Errorf("Expected status 200 for key %q, got %d", test.key, getResp.StatusCode)
				}
			}
		}
	}
}

func TestBodyReadingErrors(t *testing.T) {
	filename := "test_body_errors.db"
	cleanup(filename, t)
	defer cleanup(filename, t)

	db, err := engine.NewNabiaDB(filename)
	if err != nil {
		t.Fatalf("Failed to create Nabia DB: %q", err)
	}
	h := NewNabiaHttp(db)

	// Create a mock request with a body that fails on read
	failingBody := &failingReader{err: errors.New("read error")}
	req, _ := http.NewRequest("POST", "/test-key", failingBody)
	req.Header.Set("Content-Type", "text/plain")
	
	// Create a mock response writer
	w := &mockResponseWriter{
		header: make(http.Header),
		code:   0,
	}
	
	h.ServeHTTP(w, req)
	
	if w.code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for body read error, got %d", w.code)
	}
}

// Helper types for testing
type failingReader struct {
	err error
}

func (fr *failingReader) Read(p []byte) (n int, err error) {
	return 0, fr.err
}

type mockResponseWriter struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func (m *mockResponseWriter) Header() http.Header {
	return m.header
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	return m.body.Write(b)
}

func (m *mockResponseWriter) WriteHeader(code int) {
	m.code = code
}

func TestBinaryData(t *testing.T) {
	filename := "test_binary.db"
	cleanup(filename, t)
	defer cleanup(filename, t)

	db, err := engine.NewNabiaDB(filename)
	if err != nil {
		t.Fatalf("Failed to create Nabia DB: %q", err)
	}
	serverReady := make(chan struct{})
	stopServer := make(chan struct{})
	defer close(stopServer)
	go startServer(db, serverReady, stopServer)
	<-serverReady

	// Test with binary data including null bytes
	binaryData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD, 0x00, 0x00}
	
	resp, err := http.Post(getURL("/binary-key"), "application/octet-stream", 
		bytes.NewReader(binaryData))
	if err != nil {
		t.Fatalf("Failed to POST binary data: %q", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}
	
	// Read it back
	getResp, err := http.Get(getURL("/binary-key"))
	if err != nil {
		t.Fatalf("Failed to GET binary data: %q", err)
	}
	if getResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", getResp.StatusCode)
	}
	
	readData, _ := ioutil.ReadAll(getResp.Body)
	if !bytes.Equal(readData, binaryData) {
		t.Errorf("Binary data mismatch: got %v, expected %v", readData, binaryData)
	}
}

func TestDeserializationCorruptedData(t *testing.T) {
	// Test various corrupted data scenarios
	corruptedData := []struct {
		name string
		data []byte
	}{
		{"truncated version", []byte{}},
		{"truncated content length", []byte{0}},
		{"truncated content type", []byte{0, 10}}, // claims 10 bytes but has none
		{"invalid content after header", []byte{0, 5, 116, 101, 120, 116}}, // missing slash
		{"incomplete content type", []byte{0, 10, 116, 101, 120}}, // claims 10 bytes, has 3
	}

	for _, test := range corruptedData {
		_, err := byteSlice(test.data).deserialize()
		if err == nil {
			t.Errorf("Expected error for %s, but got none", test.name)
		}
	}
}

func TestRemoteAddressHandling(t *testing.T) {
	filename := "test_remote_addr.db"
	cleanup(filename, t)
	defer cleanup(filename, t)

	db, err := engine.NewNabiaDB(filename)
	if err != nil {
		t.Fatalf("Failed to create Nabia DB: %q", err)
	}
	h := NewNabiaHttp(db)

	// Test with invalid remote address
	req, _ := http.NewRequest("GET", "/test-key", nil)
	req.RemoteAddr = "invalid-address" // No port specified
	
	w := &mockResponseWriter{
		header: make(http.Header),
		code:   0,
	}
	
	h.ServeHTTP(w, req)
	
	// Should handle gracefully with internal server error
	if w.code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for invalid remote address, got %d", w.code)
	}
}

func TestPUTMethodErrorCases(t *testing.T) {
	filename := "test_put_errors.db"
	cleanup(filename, t)
	defer cleanup(filename, t)

	db, err := engine.NewNabiaDB(filename)
	if err != nil {
		t.Fatalf("Failed to create Nabia DB: %q", err)
	}
	h := NewNabiaHttp(db)

	// Test PUT with body read error
	failingBody := &failingReader{err: errors.New("read error")}
	req, _ := http.NewRequest("PUT", "/test-key", failingBody)
	req.Header.Set("Content-Type", "text/plain")
	req.RemoteAddr = "127.0.0.1:1234"
	
	w := &mockResponseWriter{
		header: make(http.Header),
		code:   0,
	}
	
	h.ServeHTTP(w, req)
	
	if w.code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for PUT body read error, got %d", w.code)
	}
}

func TestMockResponseWriter(t *testing.T) {
	// Test the mock response writer functionality
	w := &mockResponseWriter{
		header: make(http.Header),
		code:   0,
	}
	
	// Test Header method
	w.Header().Set("Content-Type", "text/plain")
	if w.header.Get("Content-Type") != "text/plain" {
		t.Error("Header setting failed")
	}
	
	// Test WriteHeader
	w.WriteHeader(404)
	if w.code != 404 {
		t.Errorf("Expected code 404, got %d", w.code)
	}
	
	// Test Write
	n, err := w.Write([]byte("test data"))
	if err != nil {
		t.Errorf("Write error: %v", err)
	}
	if n != 9 {
		t.Errorf("Expected 9 bytes written, got %d", n)
	}
	if w.body.String() != "test data" {
		t.Errorf("Expected body 'test data', got %q", w.body.String())
	}
}
