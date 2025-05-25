package httpx

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type emptyReader struct{}

func (e *emptyReader) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

type chunkedReader struct {
	reader    *bufio.Reader
	chunkLeft int
	finished  bool
}

func newChunkedReader(reader *bufio.Reader) *chunkedReader {
	return &chunkedReader{reader: reader}
}

func (c *chunkedReader) Read(p []byte) (n int, err error) {
	if c.finished {
		return 0, io.EOF
	}

	if c.chunkLeft == 0 {
		sizeLine, err := c.reader.ReadString('\n')
		if err != nil {
			return 0, err
		}

		sizeStr := strings.TrimSpace(sizeLine)
		if idx := strings.Index(sizeStr, ";"); idx != -1 {
			sizeStr = sizeStr[:idx]
		}

		chunkSize, err := strconv.ParseInt(sizeStr, 16, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid chunk size: %s", sizeStr)
		}

		c.chunkLeft = int(chunkSize)

		if c.chunkLeft == 0 { //final chunk
			for {
				line, err := c.reader.ReadString('\n')
				if err != nil {
					return 0, err
				}
				if strings.TrimSpace(line) == "" {
					break
				}
			}
			c.finished = true
			return 0, io.EOF
		}
	}

	toRead := len(p)
	if toRead > c.chunkLeft {
		toRead = c.chunkLeft
	}

	n, err = c.reader.Read(p[:toRead])
	c.chunkLeft -= n

	if c.chunkLeft == 0 && err == nil {
		c.reader.ReadString('\n')
	}

	return n, err
}
