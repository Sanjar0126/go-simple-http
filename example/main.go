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
	server := httpx.NewHTTPServer(httpx.HTTPServerConfig{
		Addr: "localhost",
		Port: "8080",
	})

	fmt.Println("Starting simple HTTP server...")

	server.Handler = func(req *httpx.HTTPRequest) *httpx.HTTPResponse {
		fmt.Printf("Custom handler: %s %s\n", req.Method, req.Path)

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
