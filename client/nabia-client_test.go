package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

// Mock server for testing HTTP requests
func mockServer(statusCode int, responseBody string, headers map[string]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set custom headers
		for key, value := range headers {
			w.Header().Set(key, value)
		}
		
		// Set status code
		w.WriteHeader(statusCode)
		
		// Write response body
		w.Write([]byte(responseBody))
	}))
}

func TestDetectBytesliceMimetype(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "Plain text",
			input:    []byte("Hello, World!"),
			expected: "text/plain; charset=utf-8",
		},
		{
			name:     "Empty bytes",
			input:    []byte{},
			expected: "text/plain", // mimetype library returns plain text/plain for empty
		},
		{
			name:     "Binary data",
			input:    []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, // Full PNG header
			expected: "image/png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectBytesliceMimetype(tt.input)
			if result != tt.expected {
				t.Errorf("detectBytesliceMimetype() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMakeRequest(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		key            string
		value          []byte
		contentType    []string
		serverStatus   int
		serverResponse string
		expectError    bool
	}{
		{
			name:           "GET request",
			method:         "GET",
			key:            "/test-key",
			value:          nil,
			serverStatus:   200,
			serverResponse: "test-value",
			expectError:    false,
		},
		{
			name:           "POST request with data",
			method:         "POST",
			key:            "/test-key",
			value:          []byte("test-value"),
			serverStatus:   201,
			serverResponse: "",
			expectError:    false,
		},
		{
			name:           "PUT request with custom content-type",
			method:         "PUT",
			key:            "/test-key",
			value:          []byte("test-value"),
			contentType:    []string{"text/plain"},
			serverStatus:   200,
			serverResponse: "",
			expectError:    false,
		},
		{
			name:           "DELETE request",
			method:         "DELETE",
			key:            "/test-key",
			value:          nil,
			serverStatus:   204,
			serverResponse: "",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := mockServer(tt.serverStatus, tt.serverResponse, nil)
			defer server.Close()

			// Extract host and port from server URL
			serverURL := strings.TrimPrefix(server.URL, "http://")
			parts := strings.Split(serverURL, ":")
			host := parts[0]
			port := 80
			if len(parts) > 1 {
				fmt.Sscanf(parts[1], "%d", &port)
			}

			response, err := makeRequest(tt.method, tt.key, host, uint16(port), tt.value, tt.contentType...)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if response != nil {
				defer response.Body.Close()
				
				if response.StatusCode != tt.serverStatus {
					t.Errorf("Expected status code %d, got %d", tt.serverStatus, response.StatusCode)
				}

				// Verify User-Agent header
				if response.Request.Header.Get("User-Agent") != "nabia-client/0.1" {
					t.Errorf("Expected User-Agent 'nabia-client/0.1', got '%s'", response.Request.Header.Get("User-Agent"))
				}

				// Verify Content-Type header for requests with body
				if tt.value != nil {
					expectedContentType := "application/octet-stream"
					if len(tt.contentType) > 0 {
						expectedContentType = tt.contentType[0]
					}
					if response.Request.Header.Get("Content-Type") != expectedContentType {
						t.Errorf("Expected Content-Type '%s', got '%s'", expectedContentType, response.Request.Header.Get("Content-Type"))
					}
				}
			}
		})
	}
}

func TestMakeRequestWithBadHost(t *testing.T) {
	_, err := makeRequest("GET", "/test", "invalid-host-that-does-not-exist", 1234, nil)
	if err == nil {
		t.Error("Expected error for invalid host, but got none")
	}
}

func TestOptionsData(t *testing.T) {
	tests := []struct {
		name           string
		allowHeader    string
		expectError    bool
		expectedResult string
	}{
		{
			name:           "Valid OPTIONS response",
			allowHeader:    "GET, POST, PUT, DELETE, HEAD, OPTIONS",
			expectError:    false,
			expectedResult: "GET, POST, PUT, DELETE, HEAD, OPTIONS",
		},
		{
			name:        "Empty Allow header",
			allowHeader: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{"Allow": tt.allowHeader}
			server := mockServer(200, "", headers)
			defer server.Close()

			serverURL := strings.TrimPrefix(server.URL, "http://")
			parts := strings.Split(serverURL, ":")
			host := parts[0]
			port := 80
			if len(parts) > 1 {
				fmt.Sscanf(parts[1], "%d", &port)
			}

			result, err := optionsData("/test-key", host, uint16(port))
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.expectError && result != tt.expectedResult {
				t.Errorf("Expected result '%s', got '%s'", tt.expectedResult, result)
			}
		})
	}
}

func TestHeadData(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		expectExists bool
		expectError  bool
	}{
		{
			name:         "Key exists (200)",
			statusCode:   200,
			expectExists: true,
			expectError:  false,
		},
		{
			name:         "Key exists (204)",
			statusCode:   204,
			expectExists: true,
			expectError:  false,
		},
		{
			name:         "Key does not exist (404)",
			statusCode:   404,
			expectExists: false,
			expectError:  false,
		},
		{
			name:         "Server error (500)",
			statusCode:   500,
			expectExists: false,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := mockServer(tt.statusCode, "", nil)
			defer server.Close()

			serverURL := strings.TrimPrefix(server.URL, "http://")
			parts := strings.Split(serverURL, ":")
			host := parts[0]
			port := 80
			if len(parts) > 1 {
				fmt.Sscanf(parts[1], "%d", &port)
			}

			exists, err := headData("/test-key", host, uint16(port))
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if exists != tt.expectExists {
				t.Errorf("Expected exists=%v, got %v", tt.expectExists, exists)
			}
		})
	}
}

func TestGetData(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		contentType    string
		expectError    bool
		expectedBody   string
		expectedCType  string
	}{
		{
			name:           "Successful GET",
			statusCode:     200,
			responseBody:   "Hello, World!",
			contentType:    "text/plain; charset=utf-8",
			expectError:    false,
			expectedBody:   "Hello, World!",
			expectedCType:  "text/plain; charset=utf-8",
		},
		{
			name:        "Not found (404)",
			statusCode:  404,
			expectError: true,
		},
		{
			name:        "Server error (500)",
			statusCode:  500,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{}
			if tt.contentType != "" {
				headers["Content-Type"] = tt.contentType
			}
			
			server := mockServer(tt.statusCode, tt.responseBody, headers)
			defer server.Close()

			serverURL := strings.TrimPrefix(server.URL, "http://")
			parts := strings.Split(serverURL, ":")
			host := parts[0]
			port := 80
			if len(parts) > 1 {
				fmt.Sscanf(parts[1], "%d", &port)
			}

			body, ctype, err := getData("/test-key", host, uint16(port))
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.expectError {
				if string(body) != tt.expectedBody {
					t.Errorf("Expected body '%s', got '%s'", tt.expectedBody, string(body))
				}
				if ctype != tt.expectedCType {
					t.Errorf("Expected content-type '%s', got '%s'", tt.expectedCType, ctype)
				}
			}
		})
	}
}

func TestPostData(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		expectError bool
	}{
		{
			name:        "Successful POST (201)",
			statusCode:  201,
			expectError: false,
		},
		{
			name:        "Successful POST (200)",
			statusCode:  200,
			expectError: false,
		},
		{
			name:        "Conflict (409)",
			statusCode:  409,
			expectError: true,
		},
		{
			name:        "Server error (500)",
			statusCode:  500,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := mockServer(tt.statusCode, "", nil)
			defer server.Close()

			serverURL := strings.TrimPrefix(server.URL, "http://")
			parts := strings.Split(serverURL, ":")
			host := parts[0]
			port := 80
			if len(parts) > 1 {
				fmt.Sscanf(parts[1], "%d", &port)
			}

			err := postData("/test-key", host, uint16(port), []byte("test-value"), "text/plain")
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestPutData(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		expectError bool
	}{
		{
			name:        "Successful PUT (200)",
			statusCode:  200,
			expectError: false,
		},
		{
			name:        "Successful PUT (201)",
			statusCode:  201,
			expectError: false,
		},
		{
			name:        "Bad request (400)",
			statusCode:  400,
			expectError: true,
		},
		{
			name:        "Server error (500)",
			statusCode:  500,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := mockServer(tt.statusCode, "", nil)
			defer server.Close()

			serverURL := strings.TrimPrefix(server.URL, "http://")
			parts := strings.Split(serverURL, ":")
			host := parts[0]
			port := 80
			if len(parts) > 1 {
				fmt.Sscanf(parts[1], "%d", &port)
			}

			err := putData("/test-key", host, uint16(port), []byte("test-value"), "text/plain")
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestDeleteData(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		expectError bool
	}{
		{
			name:        "Successful DELETE (200)",
			statusCode:  200,
			expectError: false,
		},
		{
			name:        "Successful DELETE (204)",
			statusCode:  204,
			expectError: false,
		},
		{
			name:        "Not found (404)",
			statusCode:  404,
			expectError: true,
		},
		{
			name:        "Server error (500)",
			statusCode:  500,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := mockServer(tt.statusCode, "", nil)
			defer server.Close()

			serverURL := strings.TrimPrefix(server.URL, "http://")
			parts := strings.Split(serverURL, ":")
			host := parts[0]
			port := 80
			if len(parts) > 1 {
				fmt.Sscanf(parts[1], "%d", &port)
			}

			err := deleteData("/test-key", host, uint16(port))
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// Test edge cases and error conditions
func TestEdgeCases(t *testing.T) {
	t.Run("Empty key", func(t *testing.T) {
		server := mockServer(200, "test", nil)
		defer server.Close()

		serverURL := strings.TrimPrefix(server.URL, "http://")
		parts := strings.Split(serverURL, ":")
		host := parts[0]
		port := 80
		if len(parts) > 1 {
			fmt.Sscanf(parts[1], "%d", &port)
		}

		// Test with empty key - should still work as it becomes "/"
		_, err := makeRequest("GET", "", host, uint16(port), nil)
		if err != nil {
			t.Errorf("Unexpected error with empty key: %v", err)
		}
	})

	t.Run("Large data", func(t *testing.T) {
		server := mockServer(200, "", nil)
		defer server.Close()

		serverURL := strings.TrimPrefix(server.URL, "http://")
		parts := strings.Split(serverURL, ":")
		host := parts[0]
		port := 80
		if len(parts) > 1 {
			fmt.Sscanf(parts[1], "%d", &port)
		}

		// Test with large data
		largeData := bytes.Repeat([]byte("x"), 1024*1024) // 1MB
		_, err := makeRequest("POST", "/large-key", host, uint16(port), largeData)
		if err != nil {
			t.Errorf("Unexpected error with large data: %v", err)
		}
	})

	t.Run("UTF-8 validation", func(t *testing.T) {
		validUTF8 := []byte("Hello, 世界")
		invalidUTF8 := []byte{0xff, 0xfe, 0xfd}

		if !utf8.Valid(validUTF8) {
			t.Error("Valid UTF-8 should be detected as valid")
		}
		if utf8.Valid(invalidUTF8) {
			t.Error("Invalid UTF-8 should be detected as invalid")
		}
	})
}

// Test network error handling
func TestNetworkErrors(t *testing.T) {
	t.Run("Connection refused", func(t *testing.T) {
		_, err := makeRequest("GET", "/test", "localhost", 9999, nil)
		if err == nil {
			t.Error("Expected connection error but got none")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "connect") {
			t.Errorf("Expected connection error, got: %v", err)
		}
	})

	t.Run("Invalid port", func(t *testing.T) {
		_, err := makeRequest("GET", "/test", "localhost", 0, nil)
		if err == nil {
			t.Error("Expected error with invalid port but got none")
		}
	})
}

// Benchmark tests
func BenchmarkMakeRequestGET(b *testing.B) {
	server := mockServer(200, "benchmark-response", nil)
	defer server.Close()

	serverURL := strings.TrimPrefix(server.URL, "http://")
	parts := strings.Split(serverURL, ":")
	host := parts[0]
	port := 80
	if len(parts) > 1 {
		fmt.Sscanf(parts[1], "%d", &port)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := makeRequest("GET", "/benchmark", host, uint16(port), nil)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func BenchmarkMakeRequestPOST(b *testing.B) {
	server := mockServer(201, "", nil)
	defer server.Close()

	serverURL := strings.TrimPrefix(server.URL, "http://")
	parts := strings.Split(serverURL, ":")
	host := parts[0]
	port := 80
	if len(parts) > 1 {
		fmt.Sscanf(parts[1], "%d", &port)
	}

	data := []byte("benchmark-data")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := makeRequest("POST", "/benchmark", host, uint16(port), data)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func BenchmarkDetectBytesliceMimetype(b *testing.B) {
	data := []byte("Hello, World! This is some test data for benchmarking.")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = detectBytesliceMimetype(data)
	}
}

// Test with temporary files
func TestFileHandling(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "nabia-test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	testContent := "This is test content for file handling"
	if _, err := tmpFile.WriteString(testContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	// Test reading the file
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read temp file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("Expected content '%s', got '%s'", testContent, string(content))
	}

	// Test MIME type detection on file content
	mimeType := detectBytesliceMimetype(content)
	if mimeType != "text/plain; charset=utf-8" {
		t.Errorf("Expected MIME type 'text/plain; charset=utf-8', got '%s'", mimeType)
	}
}