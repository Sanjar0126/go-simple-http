package custom_http

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
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

type HTTPServer struct {
	addr string
	port string
}

func NewHTTPServer(addr, port string) *HTTPServer {
	return &HTTPServer{
		addr: addr,
		port: port,
	}
}

func (s *HTTPServer) parseRequest(data string) (*HTTPRequest, error) {
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

func (r *HTTPResponse) formatResponse() string {
	var response strings.Builder

	response.WriteString(fmt.Sprintf("%s %d %s\r\n", r.Version, r.StatusCode, r.StatusText))

	for key, value := range r.Headers {
		response.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
	}

	response.WriteString("\r\n")
	response.WriteString(r.Body)

	return response.String()
}

func (s *HTTPServer) handleConnection(conn net.Conn) {
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

	request, err := s.parseRequest(requestData.String())
	if err != nil {
		fmt.Printf("Error parsing request: %v\n", err)
		return
	}

	fmt.Printf("Received %s request for %s\n", request.Method, request.Path)

	bodyText := "{\"response\":\"hello\"}\n"
	response := &HTTPResponse{
		Version:    "HTTP/1.1",
		StatusCode: http.StatusOK,
		StatusText: http.StatusText(http.StatusOK),
		Headers: map[string]string{
			"Content-Type":   "application/json",
			"Content-Length": strconv.Itoa(len(bodyText)),
			"Connection":     "close",
		},
		Body: bodyText,
	}

	_, err = conn.Write([]byte(response.formatResponse()))
	if err != nil {
		fmt.Println("Error writing to client:", err)
		return
	}
}

func (s *HTTPServer) sendErrorResponse(conn net.Conn, statusCode int, statusText string) {
	response := &HTTPResponse{
		Version:    "HTTP/1.1",
		StatusCode: statusCode,
		StatusText: statusText,
		Headers: map[string]string{
			"Content-Type":   "text/plain",
			"Content-Length": strconv.Itoa(len(statusText)),
			"Connection":     "close",
		},
		Body: statusText,
	}

	conn.Write([]byte(response.formatResponse()))
}

func (s *HTTPServer) Start() error {
	address := fmt.Sprintf("%s:%s", s.addr, s.port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Error accepting connection: %v\n", err)
			continue
		}

		go s.handleConnection(conn)
	}
}
