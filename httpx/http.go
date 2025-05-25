package httpx

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type HTTPRequest struct {
	Method  string
	Path    string
	Version string
	Headers map[string]string

	Body     io.Reader
	BodySize int64
}

type HTTPResponse struct {
	Version    string
	StatusCode int
	StatusText string
	Headers    map[string]string

	Body       io.Reader 
	BodySize   int64
}

type HandlerFunc func(*HTTPRequest) *HTTPResponse

type HTTPServer struct {
	addr string
	port string

	maxRequestSize int64
	maxHeaderSize  int64

	readTimeout  time.Duration
	writeTimeout time.Duration

	Handler HandlerFunc
}

type HTTPServerConfig struct {
	Addr           string
	Port           string
	MaxRequestSize int64
	MaxHeaderSize  int64
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
}

func NewHTTPServer(cfg HTTPServerConfig) *HTTPServer {
	if cfg.MaxRequestSize == 0 {
		cfg.MaxRequestSize = DefaultMaxRequestSize
	}
	if cfg.MaxHeaderSize == 0 {
		cfg.MaxHeaderSize = DefaultMaxHeaderSize
	}

	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 30 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 30 * time.Second
	}

	return &HTTPServer{
		addr:           cfg.Addr,
		port:           cfg.Port,
		maxRequestSize: cfg.MaxRequestSize,
		maxHeaderSize:  cfg.MaxHeaderSize,
		readTimeout:    cfg.ReadTimeout,
		writeTimeout:   cfg.WriteTimeout,
	}
}

func (s *HTTPServer) parseRequest(reader *bufio.Reader) (*HTTPRequest, error) {
	var headerBuf bytes.Buffer
	
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return nil, fmt.Errorf("error reading headers: %v", err)
		}
		
		headerBuf.Write(line)
		
		if bytes.Equal(line, []byte("\r\n")) {
			break
		}
	}
	
	headerData := headerBuf.Bytes()
	lines := bytes.Split(headerData, []byte("\r\n"))
	
	if len(lines) < 1 {
		return nil, fmt.Errorf("invalid request format")
	}

	requestLine := strings.Fields(string(lines[0]))
	if len(requestLine) != 3 {
		return nil, fmt.Errorf("invalid request line")
	}

	req := &HTTPRequest{
		Method:  requestLine[0],
		Path:    requestLine[1],
		Version: requestLine[2],
		Headers: make(map[string]string),
		Body:    reader, 
	}

	for i := 1; i < len(lines)-1; i++ { 
		line := string(lines[i])
		if line == "" {
			break
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			value := strings.TrimSpace(parts[1])
			req.Headers[key] = value
			
			if key == "content-length" {
				if length, err := strconv.ParseInt(value, 10, 64); err == nil {
					req.BodySize = length
				}
			}
		}
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

	conn.SetReadDeadline(time.Now().Add(s.readTimeout))
	conn.SetWriteDeadline(time.Now().Add(s.writeTimeout))

	fmt.Println("Client connected:", conn.RemoteAddr())

	reader := bufio.NewReader(conn)
	var requestData strings.Builder

	headerSize := 0

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}

			fmt.Println("Error reading from client:", err)
			return
		}

		headerSize += len(line)
		if int64(headerSize) > s.maxHeaderSize {
			fmt.Println("Header size exceeded limit")
			s.sendErrorResponse(conn, http.StatusRequestHeaderFieldsTooLarge, "Request header too large")
			return
		}

		requestData.WriteString(line)

		if int64(requestData.Len()) > s.maxRequestSize {
			fmt.Println("Request size exceeded limit")
			s.sendErrorResponse(conn, http.StatusRequestEntityTooLarge, "Request too large")
			return
		}

		if line == "\r\n" {
			contentLength := 0
			headerLines := strings.Split(requestData.String(), "\r\n")
			for _, headerLine := range headerLines {
				if strings.HasPrefix(strings.ToLower(headerLine), "content-length:") {
					parts := strings.SplitN(headerLine, ":", 2)
					if len(parts) == 2 {
						if length, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
							contentLength = length
						}
					}
					break
				}
			}

			if int64(contentLength) > s.maxRequestSize-int64(requestData.Len()) {
				fmt.Println("Content-Length exceeds remaining request size limit")
				s.sendErrorResponse(conn, http.StatusRequestEntityTooLarge, "Request body too large")
				return
			}

			if contentLength > 0 {
				conn.SetReadDeadline(time.Now().Add(s.readTimeout + time.Duration(contentLength/1024)*time.Second))

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

	if int64(requestData.Len()) > s.maxRequestSize {
		fmt.Println("Final request size exceeded limit")
		s.sendErrorResponse(conn, http.StatusRequestEntityTooLarge, "Request too large")
		return
	}

	request, err := s.parseRequest(requestData.String())
	if err != nil {
		fmt.Printf("Error parsing request: %v\n", err)
		s.sendErrorResponse(conn, http.StatusBadRequest, "Bad Request")
		return
	}

	if s.Handler == nil {
		s.sendErrorResponse(conn, http.StatusInternalServerError, "No handler defined")
		return
	}

	fmt.Printf("Received %s request for %s\n", request.Method, request.Path)

	response := s.Handler(request)
	if response == nil {
		s.sendErrorResponse(conn, http.StatusInternalServerError, "Handler returned nil")
		return
	}

	conn.SetWriteDeadline(time.Now().Add(s.writeTimeout))

	_, err = conn.Write([]byte(response.formatResponse()))
	if err != nil {
		fmt.Println("Error writing to client:", err)
		return
	}
}

func (s *HTTPServer) sendErrorResponse(conn net.Conn, statusCode int, statusText string) {
	response := &HTTPResponse{
		Version:    HTTP11Version,
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
