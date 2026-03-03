package connect

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// ServerStreamParser parses Connect Protocol server stream responses
// This matches Python SDK's stream parsing logic
type ServerStreamParser struct {
	codec Codec
	buf   []byte
	mu    sync.Mutex
}

// NewServerStreamParser creates a new server stream parser
func NewServerStreamParser(codec Codec) *ServerStreamParser {
	return &ServerStreamParser{
		codec: codec,
		buf:   make([]byte, 0, 4096),
	}
}

// ReadFrom reads from the stream and parses messages
// This matches Python SDK's stream reading logic
// Note: Caller is responsible for closing msgChan and errChan
func (p *ServerStreamParser) ReadFrom(reader io.Reader, msgChan chan<- interface{}, errChan chan<- error) {
	// Note: We don't close channels here - the caller (CallServerStream) is responsible

	bufReader := bufio.NewReader(reader)

	for {
		// Read envelope header
		header := make([]byte, EnvelopeHeaderLength)
		n, err := io.ReadFull(bufReader, header)
		if err != nil {
			if err == io.EOF {
				return // Stream ended normally
			}
			if err == io.ErrUnexpectedEOF {
				return // Stream ended unexpectedly
			}
			errChan <- fmt.Errorf("failed to read envelope header: %w", err)
			return
		}
		if n < EnvelopeHeaderLength {
			return // Incomplete header
		}

		// Decode header
		flags, dataLength, err := DecodeEnvelopeHeader(header)
		if err != nil {
			errChan <- fmt.Errorf("failed to decode envelope header: %w", err)
			return
		}

		// Check for end of stream
		if flags&EnvelopeFlagEndStream != 0 {
			return
		}

		// Read message data
		data := make([]byte, dataLength)
		n, err = io.ReadFull(bufReader, data)
		if err != nil {
			if err == io.EOF {
				return
			}
			errChan <- fmt.Errorf("failed to read message data: %w", err)
			return
		}
		if uint32(n) < dataLength {
			errChan <- fmt.Errorf("incomplete message data: expected %d bytes, got %d", dataLength, n)
			return
		}

		// Decompress if needed
		if flags&EnvelopeFlagCompressed != 0 {
			// TODO: Implement decompression if needed
			errChan <- fmt.Errorf("compressed messages not yet supported")
			return
		}

		// Decode message as map[string]interface{} (JSON object)
		var msg map[string]interface{}
		if err := p.codec.Decode(data, &msg); err != nil {
			errChan <- fmt.Errorf("failed to decode message: %w", err)
			return
		}

		// Send message to channel
		select {
		case msgChan <- msg:
		default:
			// Channel is full, skip this message (shouldn't happen with buffered channel)
		}
	}
}

// Codec interface for encoding/decoding messages
type Codec interface {
	Encode(v interface{}) ([]byte, error)
	Decode(data []byte, v interface{}) error
	ContentType() string
}

// JSONCodec implements Codec for JSON encoding
type JSONCodec struct{}

// NewJSONCodec creates a new JSON codec
func NewJSONCodec() Codec {
	return &JSONCodec{}
}

// Encode encodes a value to JSON
func (c *JSONCodec) Encode(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// Decode decodes JSON data to a value
func (c *JSONCodec) Decode(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// ContentType returns the content type for JSON
func (c *JSONCodec) ContentType() string {
	return "json"
}

