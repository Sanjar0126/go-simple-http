# go-simple-http

A simple HTTP server implementation in pure Go — no external routers or frameworks required.

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
