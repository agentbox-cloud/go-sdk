package connect

import (
	"encoding/json"
	"fmt"
	"io"
)

// ServerStreamParser parses server streaming responses
// This matches Python SDK's ServerStreamParser
type ServerStreamParser struct {
	codec  Codec
	buffer []byte
	header *envelopeHeader
}

type envelopeHeader struct {
	flags   EnvelopeFlags
	dataLen uint32
}

// NewServerStreamParser creates a new server stream parser
func NewServerStreamParser(codec Codec) *ServerStreamParser {
	return &ServerStreamParser{
		codec:  codec,
		buffer: make([]byte, 0),
	}
}

// Parse parses incoming chunks and yields complete messages
// Returns (message, endOfStream, error)
func (p *ServerStreamParser) Parse(chunk []byte) ([]interface{}, bool, error) {
	p.buffer = append(p.buffer, chunk...)
	var messages []interface{}

	for len(p.buffer) >= EnvelopeHeaderLength {
		// Parse header if not already parsed
		if p.header == nil {
			flags, dataLen, err := DecodeEnvelopeHeader(p.buffer[:EnvelopeHeaderLength])
			if err != nil {
				return nil, false, err
			}
			p.header = &envelopeHeader{
				flags:   flags,
				dataLen: dataLen,
			}
		}

		// Check if we have enough data
		totalLen := EnvelopeHeaderLength + int(p.header.dataLen)
		if len(p.buffer) < totalLen {
			// Not enough data yet, wait for more
			break
		}

		// Extract message data
		data := p.buffer[EnvelopeHeaderLength:totalLen]

		// Check for end of stream
		if p.header.flags&EnvelopeFlagEndStream != 0 {
			// End of stream frame can carry connect error metadata.
			// Check explicit error first; otherwise treat payload as final message if decodable.
			var errorData map[string]interface{}
			if err := json.Unmarshal(data, &errorData); err == nil {
				if errMsg, ok := errorData["error"].(map[string]interface{}); ok {
					return nil, true, fmt.Errorf("connect error: %v", errMsg)
				}
			}

			var msg interface{}
			if err := p.codec.Decode(data, &msg); err == nil {
				messages = append(messages, msg)
			}
			// Clear buffer and return end of stream
			p.buffer = p.buffer[totalLen:]
			p.header = nil
			return messages, true, nil
		}

		// Decode message (using interface{} for now, will be typed later)
		var msg interface{}
		if err := p.codec.Decode(data, &msg); err != nil {
			return nil, false, fmt.Errorf("failed to decode message: %w", err)
		}

		messages = append(messages, msg)

		// Remove processed data from buffer
		p.buffer = p.buffer[totalLen:]
		p.header = nil
	}

	return messages, false, nil
}

// ReadFrom reads from an io.Reader and parses messages
func (p *ServerStreamParser) ReadFrom(reader io.Reader, msgChan chan<- interface{}, errChan chan<- error) {
	buffer := make([]byte, 4096)

	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			msgs, endOfStream, parseErr := p.Parse(buffer[:n])
			if parseErr != nil {
				errChan <- parseErr
				return
			}

			for _, msg := range msgs {
				select {
				case msgChan <- msg:
				default:
					// Channel full, skip (shouldn't happen with buffered channel)
				}
			}

			if endOfStream {
				// End of stream detected, but we should still process any remaining messages
				// Don't return immediately - let the loop continue to process remaining buffer
				// The endOfStream flag indicates the stream has ended, but there might be
				// more messages in the buffer that need to be parsed
				if len(p.buffer) == 0 {
					// No more data to process
					return
				}
				// Continue to process remaining buffer
			}
		}

		if err == io.EOF {
			// Try to parse any remaining buffer
			if len(p.buffer) > 0 {
				// Force parse remaining buffer by calling Parse with empty chunk
				// This will process any complete messages in the buffer
				msgs, _, parseErr := p.Parse([]byte{})
				if parseErr != nil {
					errChan <- parseErr
				} else {
					for _, msg := range msgs {
						select {
						case msgChan <- msg:
						default:
						}
					}
				}
			}
			return
		}

		if err != nil {
			errChan <- err
			return
		}
	}
}
