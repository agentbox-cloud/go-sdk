package agentbox

import "time"

// SandboxInfo contains information about a sandbox
// This matches Python SDK's SandboxInfo
type SandboxInfo struct {
	SandboxID       string
	TemplateID      string
	Name            string
	Metadata        map[string]string
	StartedAt       time.Time
	EndAt           *time.Time
	EnvdVersion     string
	EnvdAccessToken string
}

// ListedSandbox represents a sandbox in a list
// This matches Python SDK's ListedSandbox
type ListedSandbox struct {
	SandboxID  string
	TemplateID string
	Name       string
	Metadata   map[string]string
	State      string
	CPUCount   int
	MemoryMB   int
	StartedAt  time.Time
	EndAt      *time.Time
}

// SandboxQuery is used to filter sandboxes when listing
// This matches Python SDK's SandboxQuery
type SandboxQuery struct {
	Metadata map[string]string
}

// ProcessInfo contains information about a running process
// This matches Python SDK's ProcessInfo
type ProcessInfo struct {
	PID  int
	Tag  string
	Cmd  string
	Args []string
	Envs map[string]string
	Cwd  string
}

// CommandResult contains the result of a command execution
// This matches Python SDK's CommandResult
type CommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// EntryInfo contains information about a filesystem entry
// This matches Python SDK's EntryInfo
type EntryInfo struct {
	Name     string
	Path     string
	Type     FileType
	Size     int64
	Modified time.Time
}

// FileType represents the type of a filesystem entry
// This matches Python SDK's FileType
type FileType string

const (
	FileTypeFile      FileType = "file"
	FileTypeDirectory FileType = "directory"
	FileTypeSymlink   FileType = "symlink"
)

// FilesystemEvent represents a filesystem event
// This matches Python SDK's FilesystemEvent
type FilesystemEvent struct {
	Type FilesystemEventType
	Name string
	Path string
}

// FilesystemEventType represents the type of filesystem event
// This matches Python SDK's FilesystemEventType
type FilesystemEventType string

const (
	FilesystemEventTypeCreate FilesystemEventType = "create"
	FilesystemEventTypeModify FilesystemEventType = "modify"
	FilesystemEventTypeDelete FilesystemEventType = "delete"
)

// OutputHandler is a callback function for handling command output
// This matches Python SDK's OutputHandler
type OutputHandler func(data string)

// Stderr represents stderr output (type alias for OutputHandler)
type Stderr = OutputHandler

// Stdout represents stdout output (type alias for OutputHandler)
type Stdout = OutputHandler

// PtyOutput represents PTY output (type alias for OutputHandler)
type PtyOutput = OutputHandler

// PtySize represents PTY size
type PtySize struct {
	Rows int
	Cols int
}

// CommandExitException represents a command exit exception
// This matches Python SDK's CommandExitException
type CommandExitException struct {
	*SandboxException
	ExitCode int
}

// NewCommandExitException creates a new CommandExitException
func NewCommandExitException(exitCode int, message string) *CommandExitException {
	return &CommandExitException{
		SandboxException: NewSandboxException(message, nil),
		ExitCode:         exitCode,
	}
}
