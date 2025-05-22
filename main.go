package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

type HTTPRequest struct {
	Method  string
	Path    string
	Version string
	Headers map[string]string
	Body    string
}

type HTTPResponse struct {
	Version    string
	StatusCode int
	StatusText string
	Headers    map[string]string
	Body       string
}

func parseRequest(data string) (*HTTPRequest, error) {
	lines := strings.Split(data, "\r\n")
	if len(lines) < 1 {
		return nil, fmt.Errorf("invalid request format")
	}

	requestLine := strings.Fields(lines[0])
	if len(requestLine) != 3 {
		return nil, fmt.Errorf("invalid request line")
	}

	req := &HTTPRequest{
		Method:  requestLine[0],
		Path:    requestLine[1],
		Version: requestLine[2],
		Headers: make(map[string]string),
	}

	bodyStart := -1
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			bodyStart = i + 1
			break
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			req.Headers[key] = value
		}
	}

	if bodyStart > 0 && bodyStart < len(lines) {
		req.Body = strings.Join(lines[bodyStart:], "\r\n")
	}

	return req, nil
}


func handleConnection(conn net.Conn) {
	defer conn.Close()
	fmt.Println("Client connected:", conn.RemoteAddr())

	reader := bufio.NewReader(conn)
	var requestData strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}

			fmt.Println("Error reading from client:", err)
			return
		}

		requestData.WriteString(line)

		if line == "\r\n" {
			contentLength := 0
			headerLines := strings.Split(requestData.String(), "\r\n")
			for _, headerLine := range headerLines {
				if strings.HasPrefix(headerLine, "Content-Length:") {
					parts := strings.SplitN(headerLine, ":", 2)
					if len(parts) == 2 {
						if length, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
							contentLength = length
						}
					}
					break
				}
			}

			if contentLength > 0 {
				body := make([]byte, contentLength)
				_, err := io.ReadFull(reader, body)
				if err != nil {
					fmt.Printf("Error reading request body: %v\n", err)
					return
				}
				requestData.Write(body)
			}

			break
		}
	}

	request, err := parseRequest(requestData.String())
	if err != nil {
		fmt.Printf("Error parsing request: %v\n", err)
		return
	}

	fmt.Println(request)

	response := "HTTP/1.1 200 OK\r\n" +
		"Content-Type: application/json\r\n" +
		"Content-Length: 20\r\n" +
		"\r\n" +
		"{\"response\":\"hello\"}\n"

	_, err = conn.Write([]byte(response))
	if err != nil {
		fmt.Println("Error writing to client:", err)
		return
	}
}

func main() {
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}

		go handleConnection(conn)
	}
}
