package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Sanjar0126/go-simple-http/httpx"
)

func main() {
	server := httpx.NewHTTPServer(httpx.HTTPServerConfig{
		Addr: "localhost",
		Port: "8080",
	})

	fmt.Println("Starting simple HTTP server...")

	server.Handler = func(req *httpx.HTTPRequest) *httpx.HTTPResponse {
		fmt.Printf("Custom handler: %s %s\n", req.Method, req.Path)
		body := fmt.Sprintf("Hello, you requested %s %s", req.Path, req.Method)
		return &httpx.HTTPResponse{
			Version:    httpx.HTTP11Version,
			StatusCode: http.StatusOK,
			StatusText: http.StatusText(http.StatusOK),
			Headers: map[string]string{
				"Content-Type":   "text/plain",
				"Content-Length": fmt.Sprintf("%d", len(body)),
			},
			Body: io.LimitReader(strings.NewReader(body), int64(len(body))),
		}
	}

	if err := server.Start(); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
