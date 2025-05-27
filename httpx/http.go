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
	StatusCode int
	StatusText string
	Headers    map[string]string
	Body       io.Reader
	bodySize   int64
	version    string
}

type HandlerFunc func(*HTTPRequest) *HTTPResponse

type HTTPServer struct {
	addr string
	port string

	maxRequestSize int64
	maxHeaderSize  int64

	readTimeout  time.Duration
	writeTimeout time.Duration

	keepAliveTimeout     time.Duration
	maxKeepAliveRequests int
	enableKeepAlive      bool

	Handler HandlerFunc
}

type HTTPServerConfig struct {
	Addr                 string
	Port                 string
	MaxRequestSize       int64
	MaxHeaderSize        int64
	ReadTimeout          time.Duration
	WriteTimeout         time.Duration
	KeepAliveTimeout     time.Duration
	MaxKeepAliveRequests int
	EnableKeepAlive      bool
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
	if cfg.KeepAliveTimeout == 0 {
		cfg.KeepAliveTimeout = DefaultKeepAliveTimeout
	}
	if cfg.MaxKeepAliveRequests == 0 {
		cfg.MaxKeepAliveRequests = DefaultMaxKeepAliveRequests
	}

	return &HTTPServer{
		addr:                 cfg.Addr,
		port:                 cfg.Port,
		maxRequestSize:       cfg.MaxRequestSize,
		maxHeaderSize:        cfg.MaxHeaderSize,
		readTimeout:          cfg.ReadTimeout,
		writeTimeout:         cfg.WriteTimeout,
		keepAliveTimeout:     cfg.KeepAliveTimeout,
		maxKeepAliveRequests: cfg.MaxKeepAliveRequests,
		enableKeepAlive:      cfg.EnableKeepAlive,
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

	if contentLength, exists := req.Headers[ContentLengthHeader]; exists {
		if length, err := strconv.ParseInt(contentLength, 10, 64); err == nil {
			req.BodySize = length

			req.Body = io.LimitReader(reader, length)
		} else {
			return nil, fmt.Errorf("invalid content-length: %s", contentLength)
		}
	} else if transferEncoding, exists := req.Headers[TransferEncodingHeader]; exists &&
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
	statusLine := fmt.Sprintf("%s %d %s\r\n", r.version, r.StatusCode, r.StatusText)
	if _, err := conn.Write([]byte(statusLine)); err != nil {
		return fmt.Errorf("error writing status line: %v", err)
	}

	if r.Headers == nil {
		r.Headers = make(map[string]string)
	}

	if r.bodySize >= 0 {
		r.Headers[ContentLengthHeader] = strconv.FormatInt(r.bodySize, 10)
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
		if r.bodySize >= 0 {
			// direct copy for fixed-length body
			_, err := io.Copy(conn, r.Body)
			if err != nil {
				return fmt.Errorf("error streaming body: %v", err)
			}
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

func (res *HTTPResponse) getContentLength() {
	if res.Body == nil {
		return
	}

	if seeker, ok := res.Body.(io.Seeker); ok {
		currentPos, err := seeker.Seek(0, io.SeekCurrent)
		if err != nil {
			res.bodySize = -1
			return
		}

		size, err := seeker.Seek(0, io.SeekEnd)
		if err != nil {
			res.bodySize = -1
			return
		}

		_, err = seeker.Seek(currentPos, io.SeekStart)
		if err != nil {
			fmt.Println("Error seeking to original position:", err)
			res.bodySize = -1
			return
		}

		res.bodySize = size - currentPos
		return
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println("Error reading body:", err)
		res.bodySize = -1
		return
	}

	res.bodySize = int64(len(data))
	res.Body = bytes.NewReader(data)
}

func (s *HTTPServer) shouldKeepConnectionAlive(req *HTTPRequest, res *HTTPResponse) bool {
	if !s.enableKeepAlive {
		return false
	}

	if req.Version == HTTP11Version {
		if connHeader, exists := req.Headers[ConnectionHeader]; exists {
			return strings.ToLower(connHeader) != "close"
		}
		return true
	} else if req.Version == HTTP10Version {
		if connHeader, exists := req.Headers[ConnectionHeader]; exists {
			return strings.ToLower(connHeader) == "keep-alive"
		}
		return false
	}

	return false
}

func (s *HTTPServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	fmt.Println("Client connected:", conn.RemoteAddr())

	requestCount := 0
	startTime := time.Now()

	for {
		if s.enableKeepAlive {
			if requestCount >= s.maxKeepAliveRequests {
				fmt.Printf("Max keep-alive requests reached for %s\n", conn.RemoteAddr())
				break
			}

			if time.Since(startTime) > s.keepAliveTimeout {
				fmt.Printf("Keep-alive timeout reached for %s\n", conn.RemoteAddr())
				break
			}
		}

		conn.SetReadDeadline(time.Now().Add(s.readTimeout))

		request, err := s.parseRequest(conn)
		if err != nil {
			if s.enableKeepAlive && requestCount > 0 {
				fmt.Printf("Connection closed by client %s after %d requests\n", conn.RemoteAddr(), requestCount)
				break
			}
			fmt.Printf("Error parsing request: %v\n", err)
			s.sendErrorResponse(conn, http.StatusBadRequest, "Bad Request", false)
			break
		}

		requestCount++

		if s.Handler == nil {
			s.sendErrorResponse(conn, http.StatusInternalServerError, "No handler defined", false)
			break
		}

		response := s.Handler(request)
		if response == nil {
			s.sendErrorResponse(conn, http.StatusInternalServerError, "Handler returned nil", false)
			break
		}

		shouldKeepAlive := s.shouldKeepConnectionAlive(request, response)

		response.version = request.Version
		response.getContentLength()

		if shouldKeepAlive {
			if response.Headers == nil {
				response.Headers = make(map[string]string)
			}
			response.Headers[ConnectionHeader] = "keep-alive"
			response.Headers[KeepAliveHeader] = fmt.Sprintf("timeout=%d, max=%d",
				int(s.keepAliveTimeout.Seconds()), s.maxKeepAliveRequests-requestCount)
		} else {
			if response.Headers == nil {
				response.Headers = make(map[string]string)
			}
			response.Headers[ConnectionHeader] = "close"
		}

		conn.SetWriteDeadline(time.Now().Add(s.writeTimeout))

		err = response.writeToConnection(conn)
		if err != nil {
			fmt.Printf("Error writing response: %v\n", err)
			break
		}

		if !shouldKeepAlive {
			break
		}

		startTime = time.Now()
	}

	fmt.Printf("Connection closed for %s after %d requests\n", conn.RemoteAddr(), requestCount)
}

func (s *HTTPServer) sendErrorResponse(conn net.Conn, statusCode int, statusText string, keepAlive bool) {
	body := strings.NewReader(statusText)

	headers := map[string]string{
		ContentTypeHeader: "text/plain",
	}

	if keepAlive {
		headers[ConnectionHeader] = "keep-alive"
	} else {
		headers[ConnectionHeader] = "close"
	}

	response := &HTTPResponse{
		version:    HTTP11Version,
		StatusCode: statusCode,
		StatusText: statusText,
		Headers:    headers,
		Body:       body,
		bodySize:   int64(len(statusText)),
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
