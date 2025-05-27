package httpx

import "time"

const (
	HTTP10Version = "HTTP/1.0"
	HTTP11Version = "HTTP/1.1"
	HTTP20Version = "HTTP/2.0"

	DefaultMaxRequestSize = 1024 * 1024 // 1MB
	DefaultMaxHeaderSize  = 8192        // 8KB

	DefaultKeepAliveTimeout     = 60 * time.Second 
	DefaultMaxKeepAliveRequests = 100

	DefaultChunkSize = 8192

	ContentTypeHeader      = "content-type"
	ContentLengthHeader    = "content-length"
	ConnectionHeader       = "connection"
	TransferEncodingHeader = "transfer-encoding"
	KeepAliveHeader        = "keep-alive"
	UpgradeHeader          = "upgrade"
	AcceptEncodingHeader   = "accept-encoding"
	AcceptLanguageHeader   = "accept-language"
	AcceptHeader           = "accept"
	CacheControlHeader     = "cache-control"
)
