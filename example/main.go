package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Sanjar0126/go-simple-http/httpx"
)

func main() {
	err := os.Chdir("temp")
	if err != nil {
		os.Mkdir("temp", 0755)
		os.Chdir("temp")
	}

	server := httpx.NewHTTPServer(httpx.HTTPServerConfig{
		Addr:                 "localhost",
		Port:                 "8080",
		EnableKeepAlive:      true,
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
		case "/upload":
			safeName := strings.ReplaceAll(strings.Trim(req.Path, "/"), "/", "_")
			if safeName == "" {
				safeName = "root"
			}

			filename := fmt.Sprintf("%s_%d", safeName, time.Now().UnixNano())
			file, err := os.Create(filename)
			if err != nil {
				fmt.Printf("Error creating file: %v\n", err)
			}
			defer file.Close()

			written, err := io.Copy(file, req.Body)
			if err != nil {
				fmt.Printf("Error writing to file: %v\n", err)
			}

			body := fmt.Sprintf("Saved %d bytes to %s", written, filename)

			return &httpx.HTTPResponse{
				StatusCode: http.StatusOK,
				StatusText: http.StatusText(http.StatusOK),
				Headers: map[string]string{
					"content-Type": "text/plain",
				},
				Body: io.LimitReader(strings.NewReader(body), int64(len(body))),
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
