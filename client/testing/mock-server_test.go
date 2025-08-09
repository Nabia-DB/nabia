package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		expectedStatus int
	}{
		{
			name:           "GET request",
			method:         "GET",
			path:           "/test",
			body:           "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST request with body",
			method:         "POST",
			path:           "/api/data",
			body:           `{"key": "value"}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "PUT request",
			method:         "PUT",
			path:           "/update",
			body:           "update data",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "DELETE request",
			method:         "DELETE",
			path:           "/delete/123",
			body:           "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "HEAD request",
			method:         "HEAD",
			path:           "/check",
			body:           "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "OPTIONS request",
			method:         "OPTIONS",
			path:           "/options",
			body:           "",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a request
			var bodyReader io.Reader
			if tt.body != "" {
				bodyReader = strings.NewReader(tt.body)
			}
			
			req := httptest.NewRequest(tt.method, tt.path, bodyReader)
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			
			// Create a ResponseRecorder to record the response
			rr := httptest.NewRecorder()
			
			// Call the handler
			handler(rr, req)
			
			// Check the status code
			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.expectedStatus)
			}
		})
	}
}

func TestHandlerWithDifferentContentTypes(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
	}{
		{
			name:        "JSON content",
			contentType: "application/json",
			body:        `{"test": "data"}`,
		},
		{
			name:        "Plain text content",
			contentType: "text/plain",
			body:        "plain text data",
		},
		{
			name:        "XML content",
			contentType: "application/xml",
			body:        "<data>test</data>",
		},
		{
			name:        "Form data",
			contentType: "application/x-www-form-urlencoded",
			body:        "key=value&another=data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			
			rr := httptest.NewRecorder()
			handler(rr, req)
			
			if status := rr.Code; status != http.StatusOK {
				t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
			}
		})
	}
}

func TestHandlerWithHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-client/1.0")
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("Accept", "application/json")
	
	rr := httptest.NewRecorder()
	handler(rr, req)
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestHandlerWithLargeBody(t *testing.T) {
	// Create a large body (1MB)
	largeBody := strings.Repeat("x", 1024*1024)
	
	req := httptest.NewRequest("POST", "/large", strings.NewReader(largeBody))
	req.Header.Set("Content-Type", "text/plain")
	
	rr := httptest.NewRecorder()
	handler(rr, req)
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestHandlerWithSpecialPaths(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{
			name: "Root path",
			path: "/",
		},
		{
			name: "Path with query params",
			path: "/test?param1=value1&param2=value2",
		},
		{
			name: "Path with special characters",
			path: "/test%20path/with%20spaces",
		},
		{
			name: "Deep nested path",
			path: "/api/v1/users/123/posts/456/comments/789",
		},
		{
			name: "Path with unicode",
			path: "/тест/データ/🔑",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rr := httptest.NewRecorder()
			
			handler(rr, req)
			
			if status := rr.Code; status != http.StatusOK {
				t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
			}
		})
	}
}

func TestHandlerResponseTime(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	
	start := time.Now()
	handler(rr, req)
	duration := time.Since(start)
	
	// Mock server should respond very quickly (under 10ms for simple requests)
	if duration > 10*time.Millisecond {
		t.Errorf("handler took too long: %v", duration)
	}
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

// Test the mock server as an actual HTTP server
func TestMockServerIntegration(t *testing.T) {
	// Start the mock server
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()
	
	// Make a real HTTP request to the server
	resp, err := http.Get(server.URL + "/test")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestMockServerWithClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()
	
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	
	tests := []struct {
		name   string
		method string
		body   string
	}{
		{"GET", "GET", ""},
		{"POST", "POST", "test data"},
		{"PUT", "PUT", "update data"},
		{"DELETE", "DELETE", ""},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bodyReader io.Reader
			if tt.body != "" {
				bodyReader = strings.NewReader(tt.body)
			}
			
			req, err := http.NewRequest(tt.method, server.URL+"/test", bodyReader)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			
			if tt.body != "" {
				req.Header.Set("Content-Type", "text/plain")
			}
			
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()
			
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}
		})
	}
}

// Test error conditions
func TestHandlerErrorConditions(t *testing.T) {
	// Test with nil request (this would normally not happen in real HTTP server, but test robustness)
	// Note: This test is mainly for completeness, as the http package ensures requests are never nil
	
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	
	// Should not panic and should return OK
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("handler panicked: %v", r)
		}
	}()
	
	handler(rr, req)
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

// Benchmark tests
func BenchmarkHandler(b *testing.B) {
	req := httptest.NewRequest("GET", "/test", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler(rr, req)
	}
}

func BenchmarkHandlerWithBody(b *testing.B) {
	body := strings.NewReader("test data for benchmarking")
	req := httptest.NewRequest("POST", "/test", body)
	req.Header.Set("Content-Type", "text/plain")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset the body reader for each iteration
		body.Seek(0, 0)
		rr := httptest.NewRecorder()
		handler(rr, req)
	}
}

func BenchmarkHandlerWithLargeBody(b *testing.B) {
	largeData := strings.Repeat("x", 10*1024) // 10KB
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		body := strings.NewReader(largeData)
		req := httptest.NewRequest("POST", "/test", body)
		req.Header.Set("Content-Type", "text/plain")
		
		rr := httptest.NewRecorder()
		handler(rr, req)
	}
}

// Test concurrent requests
func TestHandlerConcurrency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()
	
	const numConcurrentRequests = 100
	results := make(chan error, numConcurrentRequests)
	
	for i := 0; i < numConcurrentRequests; i++ {
		go func(id int) {
			resp, err := http.Get(server.URL + "/test")
			if err != nil {
				results <- err
				return
			}
			defer resp.Body.Close()
			
			if resp.StatusCode != http.StatusOK {
				results <- http.ErrNotSupported // Use as generic error
				return
			}
			
			results <- nil
		}(i)
	}
	
	// Collect results
	for i := 0; i < numConcurrentRequests; i++ {
		if err := <-results; err != nil {
			t.Errorf("Concurrent request %d failed: %v", i, err)
		}
	}
}