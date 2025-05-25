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

	Body      io.Reader
	BodySize  int64
	IsChunked bool
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

func (s *HTTPServer) parseRequest(conn net.Conn) (*HTTPRequest, error) {
	var headerBuf bytes.Buffer

	reader := bufio.NewReader(conn)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return nil, fmt.Errorf("error reading headers: %v", err)
		}

		headerBuf.Write(line)

		if bytes.Equal(line, []byte("\r\n")) {
			break
		}

		if headerBuf.Len() > int(s.maxHeaderSize) {
			return nil, fmt.Errorf("headers too large")
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
		Method:   requestLine[0],
		Path:     requestLine[1],
		Version:  requestLine[2],
		Headers:  make(map[string]string),
		BodySize: -1,
	}

	for i := 1; i < len(lines)-1; i++ {
		line := strings.TrimSpace(string(lines[i]))
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			value := strings.TrimSpace(parts[1])
			req.Headers[key] = value
		}
	}

	if contentLength, exists := req.Headers["content-length"]; exists {
		if length, err := strconv.ParseInt(contentLength, 10, 64); err == nil {
			req.BodySize = length

			req.Body = io.LimitReader(reader, length)
		} else {
			return nil, fmt.Errorf("invalid content-length: %s", contentLength)
		}
	} else if transferEncoding, exists := req.Headers["transfer-encoding"]; exists &&
		strings.ToLower(transferEncoding) == "chunked" {
		req.IsChunked = true
		req.Body = newChunkedReader(reader)
	} else {
		req.Body = &emptyReader{}
		req.BodySize = 0
	}

	return req, nil
}

func (r *HTTPResponse) writeToConnection(conn net.Conn) error {
	statusLine := fmt.Sprintf("%s %d %s\r\n", r.Version, r.StatusCode, r.StatusText)
	if _, err := conn.Write([]byte(statusLine)); err != nil {
		return fmt.Errorf("error writing status line: %v", err)
	}

	if r.Headers == nil {
		r.Headers = make(map[string]string)
	}

	if r.BodySize >= 0 {
		r.Headers[ContentLengthHeader] = strconv.FormatInt(r.BodySize, 10)
	} else if r.Body != nil {
		r.Headers[TransferEncodingHeader] = "chunked"
	}

	for key, value := range r.Headers {
		headerLine := fmt.Sprintf("%s: %s\r\n", key, value)
		if _, err := conn.Write([]byte(headerLine)); err != nil {
			return fmt.Errorf("error writing header: %v", err)
		}
	}

	if _, err := conn.Write([]byte("\r\n")); err != nil {
		return fmt.Errorf("error writing header terminator: %v", err)
	}

	if r.Body != nil {
		if r.BodySize >= 0 {
			// direct copy for fixed-length body
			written, err := io.Copy(conn, r.Body)
			if err != nil {
				return fmt.Errorf("error streaming body: %v", err)
			}
			fmt.Printf("Streamed %d bytes\n", written)
		} else {
			err := r.writeChunkedBody(conn)
			if err != nil {
				return fmt.Errorf("error writing chunked body: %v", err)
			}
		}
	}

	return nil
}

func (r *HTTPResponse) writeChunkedBody(conn net.Conn) error {
	buffer := make([]byte, DefaultChunkSize)

	for {
		n, err := r.Body.Read(buffer)
		if n > 0 {
			chunkSize := fmt.Sprintf("%x\r\n", n)
			if _, writeErr := conn.Write([]byte(chunkSize)); writeErr != nil {
				return writeErr
			}

			if _, writeErr := conn.Write(buffer[:n]); writeErr != nil {
				return writeErr
			}

			if _, writeErr := conn.Write([]byte("\r\n")); writeErr != nil {
				return writeErr
			}
		}

		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
	}

	// final size 0 chunk
	if _, err := conn.Write([]byte("0\r\n\r\n")); err != nil {
		return err
	}

	return nil
}

func (s *HTTPServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(s.readTimeout))
	conn.SetWriteDeadline(time.Now().Add(s.writeTimeout))

	fmt.Println("Client connected:", conn.RemoteAddr())

	request, err := s.parseRequest(conn)
	if err != nil {
		fmt.Printf("Error parsing streaming request: %v\n", err)
		s.sendErrorResponse(conn, http.StatusBadRequest, "Bad Request")
		return
	}

	if s.Handler == nil {
		s.sendErrorResponse(conn, http.StatusInternalServerError, "No handler defined")
		return
	}

	fmt.Printf("Received streaming %s request for %s (body length: %d)\n",
		request.Method, request.Path, request.BodySize)

	response := s.Handler(request)
	if response == nil {
		s.sendErrorResponse(conn, http.StatusInternalServerError, "Handler returned nil")
		return
	}

	conn.SetWriteDeadline(time.Now().Add(s.writeTimeout))

	err = response.writeToConnection(conn)
	if err != nil {
		fmt.Printf("Error writing streaming response: %v\n", err)
	}
}

func (s *HTTPServer) sendErrorResponse(conn net.Conn, statusCode int, statusText string) {
	body := strings.NewReader(statusText)
	response := &HTTPResponse{
		Version:    HTTP11Version,
		StatusCode: statusCode,
		StatusText: statusText,
		Headers: map[string]string{
			"Content-Type": "text/plain",
			"Connection":   "close",
		},
		Body:     body,
		BodySize: int64(len(statusText)),
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

	fmt.Printf("HTTP server listening on %s\n", address)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Error accepting connection: %v\n", err)
			continue
		}

		go s.handleConnection(conn)
	}
}
