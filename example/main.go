package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/Sanjar0126/go-simple-http/httpx"
)

func main() {
	server := httpx.NewHTTPServer(httpx.HTTPServerConfig{
		Addr:                 "localhost",
		Port:                 "8080",
		EnableKeepAlive:      false,
		KeepAliveTimeout:     20 * time.Second, // Keep connection open for 20 seconds
		MaxKeepAliveRequests: 100,              // Max 100 requests per connection
		ReadTimeout:          30 * time.Second,
		WriteTimeout:         30 * time.Second,
	})

	// Set up a simple handler
	server.Handler = func(req *httpx.HTTPRequest) *httpx.HTTPResponse {
		fmt.Printf("Received %s request for %s\n", req.Method, req.Path)

		// Simple routing
		switch req.Path {
		case "/":
			return &httpx.HTTPResponse{
				StatusCode: 200,
				StatusText: "OK",
				Headers: map[string]string{
					"Content-Type": "text/html",
				},
				Body: strings.NewReader("<h1>Hello, World!</h1><p>Keep-alive is working!</p>"),
			}
		case "/api/status":
			return &httpx.HTTPResponse{
				StatusCode: 200,
				StatusText: "OK",
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: strings.NewReader(`{"status": "OK", "keepalive": true}`),
			}
		case "/close":
			// Force connection close
			return &httpx.HTTPResponse{
				StatusCode: 200,
				StatusText: "OK",
				Headers: map[string]string{
					"Content-Type": "text/plain",
					"Connection":   "close",
				},
				Body: strings.NewReader("Connection will be closed after this response"),
			}
		default:
			return &httpx.HTTPResponse{
				StatusCode: 404,
				StatusText: "Not Found",
				Headers: map[string]string{
					"Content-Type": "text/plain",
				},
				Body: strings.NewReader("Page not found"),
			}
		}
	}

	fmt.Println("Starting HTTP server...")
	if err := server.Start(); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
