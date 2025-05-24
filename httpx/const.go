package httpx

const (
	HTTP10Version = "HTTP/1.0"
	HTTP11Version = "HTTP/1.1"
	HTTP20Version = "HTTP/2.0"

	DefaultMaxRequestSize = 1024 * 1024 // 1MB
	DefaultMaxHeaderSize  = 8192        // 8KB
)
