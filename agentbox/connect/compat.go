package connect

import (
	"time"

	envdconnect "github.com/agentbox-cloud/go-sdk/agentbox/envd/connect"
)

// Backward-compatible aliases for old import path:
// github.com/agentbox-cloud/go-sdk/agentbox/connect
//
// New implementation location:
// github.com/agentbox-cloud/go-sdk/agentbox/envd/connect

type (
	Codec              = envdconnect.Codec
	JSONCodec          = envdconnect.JSONCodec
	ProtobufCodec      = envdconnect.ProtobufCodec
	Client             = envdconnect.Client
	EnvelopeFlags      = envdconnect.EnvelopeFlags
	ServerStreamParser = envdconnect.ServerStreamParser

	ProcessConfig = envdconnect.ProcessConfig
	StartRequest  = envdconnect.StartRequest
	PTY           = envdconnect.PTY
	PTYSize       = envdconnect.PTYSize

	ProcessEvent   = envdconnect.ProcessEvent
	StartEvent     = envdconnect.StartEvent
	DataEvent      = envdconnect.DataEvent
	EndEvent       = envdconnect.EndEvent
	KeepAliveEvent = envdconnect.KeepAliveEvent

	StartResponse   = envdconnect.StartResponse
	ConnectRequest  = envdconnect.ConnectRequest
	ProcessSelector = envdconnect.ProcessSelector
	ConnectResponse = envdconnect.ConnectResponse
)

const (
	EnvelopeFlagCompressed EnvelopeFlags = envdconnect.EnvelopeFlagCompressed
	EnvelopeFlagEndStream  EnvelopeFlags = envdconnect.EnvelopeFlagEndStream
	EnvelopeHeaderLength                 = envdconnect.EnvelopeHeaderLength
)

func EncodeEnvelope(flags EnvelopeFlags, data []byte) []byte {
	return envdconnect.EncodeEnvelope(flags, data)
}

func DecodeEnvelopeHeader(header []byte) (EnvelopeFlags, uint32, error) {
	return envdconnect.DecodeEnvelopeHeader(header)
}

func NewServerStreamParser(codec Codec) *ServerStreamParser {
	return envdconnect.NewServerStreamParser(codec)
}

func NewClient(baseURL string, codec Codec, headers map[string]string, timeout time.Duration) *Client {
	return envdconnect.NewClient(baseURL, codec, headers, timeout)
}
