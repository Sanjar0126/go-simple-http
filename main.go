package main

import (
	"fmt"
	customHttp "go-simple-http/http"
)

func main() {
	server := customHttp.NewHTTPServer("0.0.0.0", "8080")

	fmt.Println("Starting simple HTTP server...")

	if err := server.Start(); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
