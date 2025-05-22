package main

import (
	"bufio"
	"net"
	"strings"
	"testing"
)

func TestHandleConnection(t *testing.T) {
	client, server := net.Pipe()

	go handleConnection(server)

	request := "POST / HTTP/1.1\r\n" +
		"Host: localhost\r\n" +
		"Content-Length: 15\r\n" +
		"Content-Type: application/json\r\n" +
		"\r\n" +
		"{\"name\":\"test\"}"

	_, err := client.Write([]byte(request))
	if err != nil {
		t.Fatalf("Failed to write request: %v", err)
	}

	reader := bufio.NewReader(client)

	statusLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read status line: %v", err)
	}
	if !strings.Contains(statusLine, "200 OK") {
		t.Errorf("Unexpected status line: %s", statusLine)
	}

	var response strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		response.WriteString(line)
	}

	expected := "{\"response\":\"hello\"}"
	if !strings.Contains(response.String(), expected) {
		t.Errorf("Expected response body to contain: %s\nGot:\n%s", expected, response.String())
	}
}