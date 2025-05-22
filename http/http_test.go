package custom_http

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Test the request parsing functionality
func TestParseRequest(t *testing.T) {
	tests := []struct {
		name     string
		rawData  string
		expected HTTPRequest
		hasError bool
	}{
		{
			name: "Simple GET request",
			rawData: "GET /hello HTTP/1.1\r\n" +
				"Host: localhost:8080\r\n" +
				"User-Agent: test\r\n" +
				"\r\n",
			expected: HTTPRequest{
				Method:  "GET",
				Path:    "/hello",
				Version: "HTTP/1.1",
				Headers: map[string]string{
					"Host":       "localhost:8080",
					"User-Agent": "test",
				},
				Body: "",
			},
			hasError: false,
		},
		{
			name: "POST request with body",
			rawData: "POST /echo HTTP/1.1\r\n" +
				"Host: localhost:8080\r\n" +
				"Content-Type: text/plain\r\n" +
				"Content-Length: 11\r\n" +
				"\r\n" +
				"Hello World",
			expected: HTTPRequest{
				Method:  "POST",
				Path:    "/echo",
				Version: "HTTP/1.1",
				Headers: map[string]string{
					"Host":           "localhost:8080",
					"Content-Type":   "text/plain",
					"Content-Length": "11",
				},
				Body: "Hello World",
			},
			hasError: false,
		},
		{
			name:     "Invalid request line",
			rawData:  "INVALID\r\n\r\n",
			hasError: true,
		},
		{
			name:     "Empty request",
			rawData:  "",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := parseRequest(tt.rawData)
			
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if req.Method != tt.expected.Method {
				t.Errorf("Method: expected %s, got %s", tt.expected.Method, req.Method)
			}
			
			if req.Path != tt.expected.Path {
				t.Errorf("Path: expected %s, got %s", tt.expected.Path, req.Path)
			}
			
			if req.Version != tt.expected.Version {
				t.Errorf("Version: expected %s, got %s", tt.expected.Version, req.Version)
			}
			
			for key, expectedValue := range tt.expected.Headers {
				if actualValue, exists := req.Headers[key]; !exists || actualValue != expectedValue {
					t.Errorf("Header %s: expected %s, got %s", key, expectedValue, actualValue)
				}
			}
			
			if req.Body != tt.expected.Body {
				t.Errorf("Body: expected %s, got %s", tt.expected.Body, req.Body)
			}
		})
	}
}

// Test response formatting
func TestHTTPResponseFormat(t *testing.T) {
	response := &HTTPResponse{
		Version:    "HTTP/1.1",
		StatusCode: 200,
		StatusText: "OK",
		Headers: map[string]string{
			"Content-Type":   "text/plain",
			"Content-Length": "11",
		},
		Body: "Hello World",
	}
	
	formatted := response.formatResponse()
	expected := "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 11\r\n\r\nHello World"
	
	// Check if all parts are present (order of headers might vary)
	if !strings.Contains(formatted, "HTTP/1.1 200 OK") {
		t.Error("Status line not found")
	}
	
	if !strings.Contains(formatted, "Content-Type: text/plain") {
		t.Error("Content-Type header not found")
	}
	
	if !strings.Contains(formatted, "Content-Length: 11") {
		t.Error("Content-Length header not found")
	}
	
	if !strings.HasSuffix(formatted, "Hello World") {
		t.Error("Body not found at end")
	}
}

// Test router functionality
func TestRouter(t *testing.T) {
	router := NewRouter()
	
	// Add test routes
	router.GET("/hello", func(req *HTTPRequest) *HTTPResponse {
		return createResponse(200, "OK", "text/plain", "Hello, World!")
	})
	
	router.GET("/users/:id", func(req *HTTPRequest) *HTTPResponse {
		id := req.Params["id"]
		return createResponse(200, "OK", "text/plain", fmt.Sprintf("User ID: %s", id))
	})
	
	router.POST("/echo", func(req *HTTPRequest) *HTTPResponse {
		return createResponse(200, "OK", "text/plain", req.Body)
	})
	
	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		expectedStatus int
		expectedBody   string
		expectedParams map[string]string
	}{
		{
			name:           "GET /hello",
			method:         "GET",
			path:           "/hello",
			expectedStatus: 200,
			expectedBody:   "Hello, World!",
		},
		{
			name:           "GET /users/123",
			method:         "GET",
			path:           "/users/123",
			expectedStatus: 200,
			expectedBody:   "User ID: 123",
			expectedParams: map[string]string{"id": "123"},
		},
		{
			name:           "POST /echo",
			method:         "POST",
			path:           "/echo",
			body:           "test message",
			expectedStatus: 200,
			expectedBody:   "test message",
		},
		{
			name:           "GET /nonexistent",
			method:         "GET",
			path:           "/nonexistent",
			expectedStatus: 404,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &HTTPRequest{
				Method: tt.method,
				Path:   tt.path,
				Body:   tt.body,
			}
			
			response := router.Handle(req)
			
			if response.StatusCode != tt.expectedStatus {
				t.Errorf("Status code: expected %d, got %d", tt.expectedStatus, response.StatusCode)
			}
			
			if tt.expectedBody != "" && response.Body != tt.expectedBody {
				t.Errorf("Body: expected %s, got %s", tt.expectedBody, response.Body)
			}
			
			if tt.expectedParams != nil {
				for key, expectedValue := range tt.expectedParams {
					if actualValue, exists := req.Params[key]; !exists || actualValue != expectedValue {
						t.Errorf("Param %s: expected %s, got %s", key, expectedValue, actualValue)
					}
				}
			}
		})
	}
}

// Test route pattern compilation
func TestRoutePatternCompilation(t *testing.T) {
	router := NewRouter()
	
	tests := []struct {
		pattern     string
		testPath    string
		shouldMatch bool
		expectedParams map[string]string
	}{
		{
			pattern:     "/users/:id",
			testPath:    "/users/123",
			shouldMatch: true,
			expectedParams: map[string]string{"id": "123"},
		},
		{
			pattern:     "/users/:id/posts/:postId",
			testPath:    "/users/123/posts/456",
			shouldMatch: true,
			expectedParams: map[string]string{"id": "123", "postId": "456"},
		},
		{
			pattern:     "/users/:id",
			testPath:    "/users",
			shouldMatch: false,
		},
		{
			pattern:     "/users/:id",
			testPath:    "/users/123/extra",
			shouldMatch: false,
		},
		{
			pattern:     "/api/v1/status",
			testPath:    "/api/v1/status",
			shouldMatch: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s vs %s", tt.pattern, tt.testPath), func(t *testing.T) {
			regex, params := router.compilePattern(tt.pattern)
			matches := regex.FindStringSubmatch(tt.testPath)
			
			if tt.shouldMatch {
				if matches == nil {
					t.Errorf("Expected pattern %s to match path %s", tt.pattern, tt.testPath)
					return
				}
				
				// Check parameters
				extractedParams := make(map[string]string)
				for i, paramName := range params {
					if i+1 < len(matches) {
						extractedParams[paramName] = matches[i+1]
					}
				}
				
				for key, expectedValue := range tt.expectedParams {
					if actualValue, exists := extractedParams[key]; !exists || actualValue != expectedValue {
						t.Errorf("Param %s: expected %s, got %s", key, expectedValue, actualValue)
					}
				}
			} else {
				if matches != nil {
					t.Errorf("Expected pattern %s to NOT match path %s", tt.pattern, tt.testPath)
				}
			}
		})
	}
}

// Test middleware functionality
func TestMiddleware(t *testing.T) {
	router := NewRouter()
	
	// Test middleware that adds a header
	testMiddleware := func(next HandlerFunc) HandlerFunc {
		return func(req *HTTPRequest) *HTTPResponse {
			response := next(req)
			response.Headers["X-Test-Middleware"] = "applied"
			return response
		}
	}
	
	router.Use(testMiddleware)
	router.GET("/test", func(req *HTTPRequest) *HTTPResponse {
		return createResponse(200, "OK", "text/plain", "test")
	})
	
	req := &HTTPRequest{
		Method: "GET",
		Path:   "/test",
	}
	
	response := router.Handle(req)
	
	if response.Headers["X-Test-Middleware"] != "applied" {
		t.Error("Middleware was not applied")
	}
}

// Integration test using actual HTTP client
func TestHTTPServerIntegration(t *testing.T) {
	// Start server on a random port
	server := NewHTTPServer("localhost", "0") // Port 0 means random available port
	router := server.Router()
	
	// Add a simple test route
	router.GET("/test", func(req *HTTPRequest) *HTTPResponse {
		return createResponse(200, "OK", "text/plain", "integration test")
	})
	
	// Start server in a goroutine
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()
	
	// Get the actual port
	port := listener.Addr().(*net.TCPAddr).Port
	serverURL := fmt.Sprintf("http://localhost:%d", port)
	
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return // Listener closed
			}
			go server.handleConnection(conn)
		}
	}()
	
	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)
	
	// Test the server
	resp, err := http.Get(serverURL + "/test")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	
	// Read response body
	body := make([]byte, 1024)
	n, _ := resp.Body.Read(body)
	bodyStr := string(body[:n])
	
	if bodyStr != "integration test" {
		t.Errorf("Expected body 'integration test', got '%s'", bodyStr)
	}
}

// Benchmark route matching
func BenchmarkRouteMatching(b *testing.B) {
	router := NewRouter()
	
	// Add many routes
	for i := 0; i < 100; i++ {
		path := fmt.Sprintf("/api/v1/resource%d/:id", i)
		router.GET(path, func(req *HTTPRequest) *HTTPResponse {
			return createResponse(200, "OK", "text/plain", "test")
		})
	}
	
	req := &HTTPRequest{
		Method: "GET",
		Path:   "/api/v1/resource50/123",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.Handle(req)
	}
}

// Test concurrent requests
func TestConcurrentRequests(t *testing.T) {
	router := NewRouter()
	
	counter := 0
	router.GET("/counter", func(req *HTTPRequest) *HTTPResponse {
		counter++
		return createResponse(200, "OK", "text/plain", fmt.Sprintf("count: %d", counter))
	})
	
	// Start server
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()
	
	server := &HTTPServer{router: router}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go server.handleConnection(conn)
		}
	}()
	
	port := listener.Addr().(*net.TCPAddr).Port
	serverURL := fmt.Sprintf("http://localhost:%d/counter", port)
	
	// Wait for server to start
	time.Sleep(100 * time.Millisecond)
	
	// Make concurrent requests
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			resp, err := http.Get(serverURL)
			if err != nil {
				t.Errorf("Request failed: %v", err)
			} else {
				resp.Body.Close()
			}
			done <- true
		}()
	}
	
	// Wait for all requests to complete
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent requests")
		}
	}
	
	if counter != 10 {
		t.Errorf("Expected counter to be 10, got %d", counter)
	}
}