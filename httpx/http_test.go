package httpx

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func setupTestServer(t *testing.T, handler HandlerFunc) (*HTTPServer, string, func()) {
	config := HTTPServerConfig{
		Addr:                 "localhost",
		Port:                 "0",
		MaxRequestSize:       DefaultMaxRequestSize,
		MaxHeaderSize:        DefaultMaxHeaderSize,
		ReadTimeout:          5 * time.Second,
		WriteTimeout:         5 * time.Second,
		KeepAliveTimeout:     DefaultKeepAliveTimeout,
		MaxKeepAliveRequests: DefaultMaxKeepAliveRequests,
		EnableKeepAlive:      true,
	}

	server := NewHTTPServer(config)
	server.Handler = handler

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	addr := listener.Addr().String()

	done := make(chan bool)

	go func() {
		defer listener.Close()
		for {
			select {
			case <-done:
				return
			default:
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				go server.handleConnection(conn)
			}
		}
	}()

	cleanup := func() {
		close(done)
		listener.Close()
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(10 * time.Millisecond)

	return server, addr, cleanup
}

func makeRequest(t *testing.T, addr, request string) string {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	_, err = conn.Write([]byte(request))
	if err != nil {
		t.Fatalf("Failed to write request: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	var response bytes.Buffer
	buffer := make([]byte, 1024)

	for {
		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, err := conn.Read(buffer)
		if n > 0 {
			response.Write(buffer[:n])
		}
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "timeout") {
				break
			}
			t.Fatalf("Failed to read response: %v", err)
		}

		if n < len(buffer) {
			break
		}
	}

	return response.String()
}

func makeRawConnection(t *testing.T, addr string) net.Conn {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	return conn
}

func TestHTTP11BasicRequest(t *testing.T) {
	handler := func(req *HTTPRequest) *HTTPResponse {
		if req.Method != "GET" || req.Path != "/test" {
			t.Errorf("Expected GET /test, got %s %s", req.Method, req.Path)
		}
		return &HTTPResponse{
			StatusCode: 200,
			StatusText: "OK",
			Headers:    map[string]string{"content-type": "text/plain"},
			Body:       strings.NewReader("Hello World"),
		}
	}

	_, addr, cleanup := setupTestServer(t, handler)
	defer cleanup()

	request := "GET /test HTTP/1.1\r\nHost: localhost\r\n\r\n"
	response := makeRequest(t, addr, request)

	if !strings.Contains(response, "HTTP/1.1 200 OK") {
		t.Errorf("Expected HTTP/1.1 200 OK in response, got: %s", response)
	}
	if !strings.Contains(response, "Hello World") {
		t.Errorf("Expected 'Hello World' in response body, got: %s", response)
	}
}

func TestHTTP10BasicRequest(t *testing.T) {
	handler := func(req *HTTPRequest) *HTTPResponse {
		if req.Version != "HTTP/1.0" {
			t.Errorf("Expected HTTP/1.0, got %s", req.Version)
		}
		return &HTTPResponse{
			StatusCode: 200,
			StatusText: "OK",
			Body:       strings.NewReader("HTTP 1.0 Response"),
		}
	}

	_, addr, cleanup := setupTestServer(t, handler)
	defer cleanup()

	request := "GET / HTTP/1.0\r\n\r\n"
	response := makeRequest(t, addr, request)

	if !strings.Contains(response, "HTTP/1.0 200 OK") {
		t.Errorf("Expected HTTP/1.0 200 OK in response, got: %s", response)
	}
}

func TestPOSTWithContentLength(t *testing.T) {
	expectedBody := "test data"

	handler := func(req *HTTPRequest) *HTTPResponse {
		if req.Method != "POST" {
			t.Errorf("Expected POST, got %s", req.Method)
		}
		if req.BodySize != int64(len(expectedBody)) {
			t.Errorf("Expected body size %d, got %d", len(expectedBody), req.BodySize)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("Error reading body: %v", err)
		}
		if string(body) != expectedBody {
			t.Errorf("Expected body '%s', got '%s'", expectedBody, string(body))
		}

		return &HTTPResponse{
			StatusCode: 201,
			StatusText: "Created",
			Body:       strings.NewReader("Created successfully"),
		}
	}

	_, addr, cleanup := setupTestServer(t, handler)
	defer cleanup()

	request := fmt.Sprintf("POST /create HTTP/1.1\r\nHost: localhost\r\nContent-Length: %d\r\n\r\n%s",
		len(expectedBody), expectedBody)
	response := makeRequest(t, addr, request)

	if !strings.Contains(response, "HTTP/1.1 201 Created") {
		t.Errorf("Expected HTTP/1.1 201 Created in response, got: %s", response)
	}
}

func TestChunkedTransferEncoding(t *testing.T) {
	handler := func(req *HTTPRequest) *HTTPResponse {
		if !req.IsChunked {
			t.Errorf("Expected chunked request")
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("Error reading chunked body: %v", err)
		}

		expected := "Hello World"
		if string(body) != expected {
			t.Errorf("Expected body '%s', got '%s'", expected, string(body))
		}

		return &HTTPResponse{
			StatusCode: 200,
			StatusText: "OK",
			Body:       strings.NewReader("Chunked response received"),
		}
	}

	_, addr, cleanup := setupTestServer(t, handler)
	defer cleanup()

	request := "POST /chunked HTTP/1.1\r\n" +
		"Host: localhost\r\n" +
		"Transfer-Encoding: chunked\r\n\r\n" +
		"b\r\nHello World\r\n" +
		"0\r\n\r\n"

	response := makeRequest(t, addr, request)

	if !strings.Contains(response, "HTTP/1.1 200 OK") {
		t.Errorf("Expected HTTP/1.1 200 OK in response, got: %s", response)
	}
}

func TestKeepAliveHTTP11(t *testing.T) {
	requestCount := 0

	handler := func(req *HTTPRequest) *HTTPResponse {
		requestCount++
		return &HTTPResponse{
			StatusCode: 200,
			StatusText: "OK",
			Body:       strings.NewReader(fmt.Sprintf("Request %d", requestCount)),
		}
	}

	_, addr, cleanup := setupTestServer(t, handler)
	defer cleanup()

	conn := makeRawConnection(t, addr)
	defer conn.Close()

	request1 := "GET /test1 HTTP/1.1\r\nHost: localhost\r\n\r\n"
	_, err := conn.Write([]byte(request1))
	if err != nil {
		t.Fatalf("Failed to write first request: %v", err)
	}

	reader := bufio.NewReader(conn)
	response1, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read first response: %v", err)
	}

	if !strings.Contains(response1, "HTTP/1.1 200 OK") {
		t.Errorf("Expected HTTP/1.1 200 OK in first response, got: %s", response1)
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if line == "\r\n" {
			break
		}
	}

	reader.ReadString('\n')

	request2 := "GET /test2 HTTP/1.1\r\nHost: localhost\r\n\r\n"
	_, err = conn.Write([]byte(request2))
	if err != nil {
		t.Fatalf("Failed to write second request: %v", err)
	}

	response2, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read second response: %v", err)
	}

	if !strings.Contains(response2, "HTTP/1.1 200 OK") {
		t.Errorf("Expected HTTP/1.1 200 OK in second response, got: %s", response2)
	}

	if requestCount != 2 {
		t.Errorf("Expected 2 requests on keep-alive connection, got %d", requestCount)
	}
}

func TestConnectionClose(t *testing.T) {
	handler := func(req *HTTPRequest) *HTTPResponse {
		return &HTTPResponse{
			StatusCode: 200,
			StatusText: "OK",
			Body:       strings.NewReader("Response"),
		}
	}

	_, addr, cleanup := setupTestServer(t, handler)
	defer cleanup()

	conn := makeRawConnection(t, addr)
	defer conn.Close()

	request := "GET /test HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n"
	_, err := conn.Write([]byte(request))
	if err != nil {
		t.Fatalf("Failed to write request: %v", err)
	}

	response := make([]byte, 1024)
	n, err := conn.Read(response)
	if err != nil && err != io.EOF {
		t.Fatalf("Failed to read response: %v", err)
	}

	responseStr := string(response[:n])
	if !strings.Contains(responseStr, "Connection: close") {
		t.Errorf("Expected 'Connection: close' in response headers, got: %s", responseStr)
	}
}

func TestHTTP10KeepAlive(t *testing.T) {
	handler := func(req *HTTPRequest) *HTTPResponse {
		return &HTTPResponse{
			StatusCode: 200,
			StatusText: "OK",
			Body:       strings.NewReader("HTTP/1.0 Response"),
		}
	}

	_, addr, cleanup := setupTestServer(t, handler)
	defer cleanup()

	conn := makeRawConnection(t, addr)
	defer conn.Close()

	request := "GET /test HTTP/1.0\r\nConnection: keep-alive\r\n\r\n"
	_, err := conn.Write([]byte(request))
	if err != nil {
		t.Fatalf("Failed to write request: %v", err)
	}

	response := make([]byte, 1024)
	n, err := conn.Read(response)
	if err != nil && err != io.EOF {
		t.Fatalf("Failed to read response: %v", err)
	}

	responseStr := string(response[:n])
	if !strings.Contains(responseStr, "Connection: keep-alive") {
		t.Errorf("Expected 'Connection: keep-alive' in response headers, got: %s", responseStr)
	}
}

func TestLargeHeaders(t *testing.T) {
	handler := func(req *HTTPRequest) *HTTPResponse {
		return &HTTPResponse{StatusCode: 200, StatusText: "OK"}
	}

	server, addr, cleanup := setupTestServer(t, handler)
	defer cleanup()
	server.maxHeaderSize = 100

	largeHeader := strings.Repeat("a", 200)
	request := fmt.Sprintf("GET /test HTTP/1.1\r\nHost: localhost\r\nX-Large-Header: %s\r\n\r\n", largeHeader)

	response := makeRequest(t, addr, request)

	if !strings.Contains(response, "400") {
		t.Errorf("Expected 400 Bad Request for large headers, got: %s", response)
	}
}

func TestInvalidRequestFormat(t *testing.T) {
	handler := func(req *HTTPRequest) *HTTPResponse {
		return &HTTPResponse{StatusCode: 200, StatusText: "OK"}
	}

	_, addr, cleanup := setupTestServer(t, handler)
	defer cleanup()

	request := "GET /test\r\n\r\n"
	response := makeRequest(t, addr, request)

	if !strings.Contains(response, "400") {
		t.Errorf("Expected 400 Bad Request for invalid format, got: %s", response)
	}
}

func TestChunkedResponseWriting(t *testing.T) {
	largeBody := strings.Repeat("Hello World! ", 1000)

	handler := func(req *HTTPRequest) *HTTPResponse {
		return &HTTPResponse{
			StatusCode: 200,
			StatusText: "OK",
			Headers:    map[string]string{"content-type": "text/plain"},
			Body:       strings.NewReader(largeBody),
			bodySize:   -1,
		}
	}

	_, addr, cleanup := setupTestServer(t, handler)
	defer cleanup()

	request := "GET /large HTTP/1.1\r\nHost: localhost\r\n\r\n"

	conn := makeRawConnection(t, addr)
	defer conn.Close()

	_, err := conn.Write([]byte(request))
	if err != nil {
		t.Fatalf("Failed to write request: %v", err)
	}

	reader := bufio.NewReader(conn)

	statusLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read status line: %v", err)
	}

	if !strings.Contains(statusLine, "HTTP/1.1 200 OK") {
		t.Errorf("Expected HTTP/1.1 200 OK, got: %s", statusLine)
	}

	var chunkedFound bool
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("Failed to read header line: %v", err)
		}

		if strings.Contains(strings.ToLower(line), "transfer-encoding: chunked") {
			chunkedFound = true
		}

		if line == "\r\n" {
			break
		}
	}

	if !chunkedFound {
		t.Errorf("Expected Transfer-Encoding: chunked header")
	}

	chunkSizeLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read chunk size: %v", err)
	}

	chunkSizeStr := strings.TrimSpace(chunkSizeLine)
	if len(chunkSizeStr) == 0 {
		t.Errorf("Expected chunk size, got empty line")
	}
}

func TestMultipleHeaders(t *testing.T) {
	handler := func(req *HTTPRequest) *HTTPResponse {
		if userAgent, exists := req.Headers["user-agent"]; !exists || userAgent != "TestClient/1.0" {
			t.Errorf("Expected User-Agent header, got: %v", req.Headers)
		}
		if accept, exists := req.Headers["accept"]; !exists || accept != "text/html,application/json" {
			t.Errorf("Expected Accept header, got: %v", req.Headers)
		}

		return &HTTPResponse{
			StatusCode: 200,
			StatusText: "OK",
			Headers: map[string]string{
				"content-type":    "application/json",
				"cache-control":   "no-cache",
				"x-custom-header": "test-value",
			},
			Body: strings.NewReader(`{"status": "success"}`),
		}
	}

	_, addr, cleanup := setupTestServer(t, handler)
	defer cleanup()

	request := "GET /test HTTP/1.1\r\n" +
		"Host: localhost\r\n" +
		"User-Agent: TestClient/1.0\r\n" +
		"Accept: text/html,application/json\r\n\r\n"

	response := makeRequest(t, addr, request)

	if !strings.Contains(response, "content-type: application/json") {
		t.Errorf("Expected content-type header in response, got: %s", response)
	}
	if !strings.Contains(response, "x-custom-header: test-value") {
		t.Errorf("Expected custom header in response, got: %s", response)
	}
}

func TestEmptyBodyRequest(t *testing.T) {
	handler := func(req *HTTPRequest) *HTTPResponse {
		if req.BodySize != 0 {
			t.Errorf("Expected empty body size, got: %d", req.BodySize)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("Error reading empty body: %v", err)
		}
		if len(body) != 0 {
			t.Errorf("Expected empty body, got: %s", string(body))
		}

		return &HTTPResponse{
			StatusCode: 204,
			StatusText: "No Content",
		}
	}

	_, addr, cleanup := setupTestServer(t, handler)
	defer cleanup()

	request := "GET /empty HTTP/1.1\r\nHost: localhost\r\n\r\n"
	response := makeRequest(t, addr, request)

	if !strings.Contains(response, "HTTP/1.1 204 No Content") {
		t.Errorf("Expected HTTP/1.1 204 No Content, got: %s", response)
	}
}

func BenchmarkHTTPServer(b *testing.B) {
	handler := func(req *HTTPRequest) *HTTPResponse {
		return &HTTPResponse{
			StatusCode: 200,
			StatusText: "OK",
			Body:       strings.NewReader("OK"),
		}
	}

	_, addr, cleanup := setupTestServer(nil, handler)
	defer cleanup()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			b.Fatalf("Failed to connect: %v", err)
		}

		request := "GET /bench HTTP/1.1\r\nHost: localhost\r\n\r\n"
		conn.Write([]byte(request))

		response := make([]byte, 256)
		conn.Read(response)
		conn.Close()
	}
}
