package httpx

import (
	"bufio"
	"io"
)

type emptyReader struct{}

func (e *emptyReader) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

type chunkedReader struct {
	reader *bufio.Reader
}

func newChunkedReader(reader *bufio.Reader) *chunkedReader {
	return &chunkedReader{reader: reader}
}

func (c *chunkedReader) Read(p []byte) (n int, err error) {
	// TODO: Implement chunked reading logic
	return c.reader.Read(p)
}
