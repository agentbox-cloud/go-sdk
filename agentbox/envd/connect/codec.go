package connect

import (
	"encoding/json"
	"fmt"
)

// Codec handles encoding and decoding of messages
type Codec interface {
	ContentType() string
	Encode(msg interface{}) ([]byte, error)
	Decode(data []byte, msgType interface{}) error
}

// JSONCodec implements JSON encoding/decoding
// This matches Python SDK's JSONCodec when json=True
type JSONCodec struct{}

func (c *JSONCodec) ContentType() string {
	return "json"
}

func (c *JSONCodec) Encode(msg interface{}) ([]byte, error) {
	return json.Marshal(msg)
}

func (c *JSONCodec) Decode(data []byte, msgType interface{}) error {
	// msgType is a pointer to the target type
	return json.Unmarshal(data, msgType)
}

// ProtobufCodec would implement protobuf encoding/decoding
// Currently not implemented as Python SDK uses json=True
type ProtobufCodec struct{}

func (c *ProtobufCodec) ContentType() string {
	return "proto"
}

func (c *ProtobufCodec) Encode(msg interface{}) ([]byte, error) {
	return nil, fmt.Errorf("protobuf encoding not yet implemented")
}

func (c *ProtobufCodec) Decode(data []byte, msgType interface{}) error {
	return fmt.Errorf("protobuf decoding not yet implemented")
}

