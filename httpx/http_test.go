package httpx

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseRequest_BasicGET(t *testing.T) {
	raw := "GET /hello HTTP/1.1\r\nHost: localhost\r\n\r\n"
	server := NewHTTPServer("127.0.0.1", "0")

	req, err := server.parseRequest(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.Method != "GET" || req.Path != "/hello" || req.Version != "HTTP/1.1" {
		t.Errorf("unexpected request fields: %+v", req)
	}

	if req.Headers["Host"] != "localhost" {
		t.Errorf("expected Host header to be localhost, got %s", req.Headers["Host"])
	}
}

func TestParseRequest_WithBody(t *testing.T) {
	raw := "POST /submit HTTP/1.1\r\nHost: localhost\r\nContent-Length: 11\r\n\r\nHello World"
	server := NewHTTPServer("127.0.0.1", "0")

	req, err := server.parseRequest(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.Body != "Hello World" {
		t.Errorf("expected body 'Hello World', got '%s'", req.Body)
	}
}

func TestFormatResponse(t *testing.T) {
	resp := &HTTPResponse{
		Version:    "HTTP/1.1",
		StatusCode: 200,
		StatusText: "OK",
		Headers: map[string]string{
			"Content-Type": "text/plain",
		},
		Body: "Hello",
	}

	raw := resp.formatResponse()

	if !strings.HasPrefix(raw, "HTTP/1.1 200 OK\r\n") {
		t.Errorf("unexpected response format: %s", raw)
	}
	if !strings.Contains(raw, "Content-Type: text/plain\r\n") {
		t.Error("missing Content-Type header")
	}
	if !strings.HasSuffix(raw, "\r\nHello") {
		t.Error("unexpected body content")
	}
}

func TestHTTPServer_HandlerInvocation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	server := NewHTTPServer("127.0.0.1", strconv.Itoa(port))

	handlerCalled := false
	server.Handler = func(req *HTTPRequest) *HTTPResponse {
		handlerCalled = true
		return &HTTPResponse{
			Version:    "HTTP/1.1",
			StatusCode: 200,
			StatusText: "OK",
			Headers: map[string]string{
				"Content-Type":   "text/plain",
				"Content-Length": "2",
				"Connection":     "close",
			},
			Body: "OK",
		}
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			t.Errorf("failed to accept connection: %v", err)
			return
		}
		server.handleConnection(conn)
	}()

	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("failed to connect to server: %v", err)
	}
	defer conn.Close()

	request := "GET /test HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n"
	if _, err := conn.Write([]byte(request)); err != nil {
		t.Fatalf("failed to write request: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(1 * time.Second)) // avoid blocking forever
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	resp := string(buf[:n])
	if !strings.Contains(resp, "200 OK") {
		t.Errorf("unexpected response: %s", resp)
	}
	if !handlerCalled {
		t.Error("handler was not called")
	}
}
