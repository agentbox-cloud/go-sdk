package connect

import (
	"encoding/binary"
	"errors"
)

// EnvelopeFlags represents flags in the envelope header
type EnvelopeFlags uint8

const (
	// EnvelopeFlagCompressed indicates the message is compressed
	EnvelopeFlagCompressed EnvelopeFlags = 0b00000001
	// EnvelopeFlagEndStream indicates this is the end of the stream
	EnvelopeFlagEndStream EnvelopeFlags = 0b00000010
)

const (
	// EnvelopeHeaderLength is the length of the envelope header (1 byte flags + 4 bytes data length)
	EnvelopeHeaderLength = 5
)

// EncodeEnvelope encodes a message into Connect Protocol envelope format
// Format: [1 byte flags][4 bytes big-endian data length][data]
func EncodeEnvelope(flags EnvelopeFlags, data []byte) []byte {
	header := make([]byte, EnvelopeHeaderLength)
	header[0] = byte(flags)
	binary.BigEndian.PutUint32(header[1:5], uint32(len(data)))
	return append(header, data...)
}

// DecodeEnvelopeHeader decodes the envelope header
// Returns (flags, dataLength, error)
func DecodeEnvelopeHeader(header []byte) (EnvelopeFlags, uint32, error) {
	if len(header) < EnvelopeHeaderLength {
		return 0, 0, errors.New("envelope header too short")
	}
	flags := EnvelopeFlags(header[0])
	dataLength := binary.BigEndian.Uint32(header[1:5])
	return flags, dataLength, nil
}

