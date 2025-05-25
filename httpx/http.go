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

	Body     io.Reader
	BodySize int64
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

func (r *HTTPResponse) writeToConnection(conn net.Conn) error {
	if _, err := fmt.Fprintf(conn, "%s %d %s\r\n", r.Version, r.StatusCode, r.StatusText); err != nil {
		return err
	}

	for key, value := range r.Headers {
		if _, err := fmt.Fprintf(conn, "%s: %s\r\n", key, value); err != nil {
			return err
		}
	}

	if _, err := conn.Write([]byte("\r\n")); err != nil {
		return err
	}

	if r.Body != nil {
		_, err := io.Copy(conn, r.Body)
		return err
	}

	return nil
}

func (s *HTTPServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(s.readTimeout))
	conn.SetWriteDeadline(time.Now().Add(s.writeTimeout))

	fmt.Println("Client connected:", conn.RemoteAddr())

	reader := bufio.NewReader(conn)

	request, err := s.parseRequest(reader)
	if err != nil {
		fmt.Printf("Error parsing streaming request: %v\n", err)
		s.sendErrorResponse(conn, http.StatusBadRequest, "Bad Request")
		return
	}

	if request.BodySize > 0 {
		request.Body = io.LimitReader(request.Body, min(request.BodySize, s.maxRequestSize))
	}

	fmt.Printf("Received %s request for %s (body: %d bytes)\n",
		request.Method, request.Path, request.BodySize)

	response := s.Handler(request)
	if response == nil {
		s.sendErrorResponse(conn, http.StatusInternalServerError, "Handler returned nil")
		return
	}

	conn.SetWriteDeadline(time.Now().Add(s.writeTimeout))

	err = response.writeToConnection(conn)
	if err != nil {
		fmt.Println("Error writing streaming response:", err)
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
		Body: io.LimitReader(strings.NewReader(statusText), int64(len(statusText))),
	}

	response.writeToConnection(conn)
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
