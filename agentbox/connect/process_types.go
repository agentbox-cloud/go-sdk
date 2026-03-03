package connect

// Process message types for JSON encoding
// These match the protobuf definitions but use JSON-compatible Go structs
// This matches Python SDK's process.proto definitions

// ProcessConfig represents a process configuration
type ProcessConfig struct {
	Cmd  string            `json:"cmd"`
	Args []string          `json:"args,omitempty"`
	Envs map[string]string `json:"envs,omitempty"`
	Cwd  *string           `json:"cwd,omitempty"`
}

// StartRequest represents a request to start a process
type StartRequest struct {
	Process ProcessConfig `json:"process"`
	Pty     *PTY           `json:"pty,omitempty"`
	Tag     *string        `json:"tag,omitempty"`
}

// PTY represents PTY configuration
type PTY struct {
	Size *PTYSize `json:"size,omitempty"`
}

// PTYSize represents PTY size
type PTYSize struct {
	Cols uint32 `json:"cols"`
	Rows uint32 `json:"rows"`
}

// ProcessEvent represents a process event
type ProcessEvent struct {
	Start     *StartEvent     `json:"start,omitempty"`
	Data      *DataEvent      `json:"data,omitempty"`
	End       *EndEvent       `json:"end,omitempty"`
	KeepAlive *KeepAliveEvent `json:"keepalive,omitempty"`
}

// StartEvent represents a process start event
type StartEvent struct {
	PID uint32 `json:"pid"`
}

// DataEvent represents process output data
type DataEvent struct {
	Stdout []byte `json:"stdout,omitempty"`
	Stderr []byte `json:"stderr,omitempty"`
	Pty    []byte `json:"pty,omitempty"`
}

// EndEvent represents a process end event
type EndEvent struct {
	ExitCode int32   `json:"exit_code"`
	Exited   bool    `json:"exited"`
	Status   string  `json:"status"`
	Error    *string `json:"error,omitempty"`
}

// KeepAliveEvent represents a keepalive event
type KeepAliveEvent struct{}

// StartResponse represents a start response
type StartResponse struct {
	Event ProcessEvent `json:"event"`
}

// ConnectRequest represents a request to connect to a process
type ConnectRequest struct {
	Process ProcessSelector `json:"process"`
}

// ProcessSelector selects a process by PID or tag
type ProcessSelector struct {
	PID *uint32 `json:"pid,omitempty"`
	Tag *string `json:"tag,omitempty"`
}

// ConnectResponse represents a connect response
type ConnectResponse struct {
	Event ProcessEvent `json:"event"`
}

