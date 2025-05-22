package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

func handleConnection(conn net.Conn) {
	headers := make(map[string]string)
	contentLength := 0

	defer conn.Close()
	fmt.Println("Client connected:", conn.RemoteAddr())

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}

			fmt.Println("Error reading from client:", err)
			return
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}

		fmt.Println(line)

		headerParts := strings.SplitN(line, ":", 2)
		if len(headerParts) == 2 {
			headers[strings.TrimSpace(headerParts[0])] = strings.TrimSpace(headerParts[1])
		}
	}

	if val, ok := headers["Content-Length"]; ok {
		cl, err := strconv.Atoi(val)
		if err == nil {
			contentLength = cl
		}
	}

	var body []byte
	if contentLength > 0 {
		body = make([]byte, contentLength)
		_, err := io.ReadFull(reader, body)
		if err != nil {
			fmt.Println("Error reading body:", err)
			return
		}
		fmt.Println("Body:", string(body))
	}

	response := "HTTP/1.1 200 OK\r\n" +
		"Content-Type: application/json\r\n" +
		"Content-Length: 20\r\n" +
		"\r\n" +
		"{\"response\":\"hello\"}\n"

	_, err := conn.Write([]byte(response))
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
