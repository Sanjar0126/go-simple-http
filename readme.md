# go-simple-http

A simple HTTP server implementation in pure Go — no external routers or frameworks required.

## ✅ Minimal HTTP/1.0 Server (Done)
- Accept TCP connections.
- Parse the HTTP request line (`GET /path HTTP/1.0`).
- Send static responses with status line, headers, and body.

---

## 🔜 Basic HTTP/1.1 Features
- ✅ **Header Parsing**
  - Parse request headers into a normalized map (case-insensitive).
- [ ] **Persistent Connections**
  - Support `Connection: keep-alive` and multiple requests per connection.
- [ ] **Chunked Transfer-Encoding (Optional)**
  - Decode chunked request bodies.
  - Encode responses in chunked format if body length is unknown.
- [ ] **Content-Length**
  - Correctly read request bodies and set `Content-Length` in responses.

---

## 🔒 Routing and Dynamic Responses
- [ ] **Request Router**
  - Map URL paths and methods to handler functions.
- [ ] **Dynamic Parameters**
  - Support parameters like `/user/:id` or query strings (`?q=go`).
- [ ] **Status Codes**
  - Send appropriate HTTP status codes (200, 404, 500, etc.).

---

## 🔒 Advanced HTTP/1.1 Features
- [ ] **Form and JSON Body Parsing**
  - Parse `application/x-www-form-urlencoded` and `application/json` bodies.
- [ ] **File Serving**
  - Serve static files with correct MIME types.
- [ ] **Cookie Handling**
  - Parse `Cookie:` headers from requests.
  - Support `Set-Cookie` in responses.
- [ ] **Important Headers**
  - Implement: `Host`, `User-Agent`.
  - Optional: `ETag`, `If-Modified-Since`, `Cache-Control`.

---

## 🔧 Middleware & Error Handling
- [ ] **Middleware System**
  - Support wrapping handlers (e.g., for logging, auth).
- [ ] **Custom Error Pages**
  - Return custom pages for 404, 500, etc.
- [ ] **Request Logging**
  - Log method, path, response code, and duration.

---

## 🔒 HTTPS & HTTP/2 (Optional, Advanced)
- [ ] **TLS (HTTPS) Support**
  - Use TLS with certificates (via `crypto/tls` in Go or equivalent).
- [ ] **HTTP/2 Support (Optional)**
  - Requires multiplexed streams, HPACK header compression.
  - Consider using existing libraries unless implementing from scratch.

---

## 📦 Installation

```bash
go get github.com/Sanjar0126/go-simple-http
```

---

## 🧑‍💻 Simple Usage

Create a basic HTTP server with a custom handler:

```go
package main

import (
	"fmt"

	"github.com/Sanjar0126/go-simple-http/httpx"
)

func main() {
	server := httpx.NewHTTPServer("0.0.0.0", "8080")

	fmt.Println("Starting simple HTTP server...")

	server.Handler = func(req *httpx.HTTPRequest) *httpx.HTTPResponse {
		fmt.Printf("Custom handler: %s %s\n", req.Method, req.Path)
		body := fmt.Sprintf("Hello, you requested %s", req.Path)
		return &httpx.HTTPResponse{
			Version:    "HTTP/1.1",
			StatusCode: 200,
			StatusText: "OK",
			Headers: map[string]string{
				"Content-Type":   "text/plain",
				"Content-Length": fmt.Sprintf("%d", len(body)),
			},
			Body: body,
		}
	}

	if err := server.Start(); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
```

Run it:

```bash
go run main.go
```

Then test it with:

```bash
curl http://localhost:8080/test
```

---

## 🧾 Request & Response Structures

### `HTTPRequest`

```go
type HTTPRequest struct {
	Method  string
	Path    string
	Version string
	Headers map[string]string
	Body    string
}
```

### `HTTPResponse`

```go
type HTTPResponse struct {
	Version    string
	StatusCode int
	StatusText string
	Headers    map[string]string
	Body       string
}
```
