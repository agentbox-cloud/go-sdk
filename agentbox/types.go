package agentbox

import (
	"context"
	"time"
)

// SandboxInfo contains information about a sandbox
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
type SandboxQuery struct {
	Metadata map[string]string
}

// ProcessInfo contains information about a running process
type ProcessInfo struct {
	PID  int
	Tag  string
	Cmd  string
	Args []string
	Envs map[string]string
	Cwd  string
}

// CommandResult contains the result of a command execution
type CommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// CommandHandle represents a handle to a running command (for background execution)
type CommandHandle interface {
	// PID returns the process ID
	PID() int

	// Wait waits for the command to finish and returns the result
	Wait(ctx context.Context) (*CommandResult, error)

	// Kill kills the running command
	Kill(ctx context.Context) error

	// SendStdin sends data to the command's stdin
	SendStdin(ctx context.Context, data string) error
}

// AsyncCommandHandle represents a handle to a running command (async version)
type AsyncCommandHandle interface {
	// PID returns the process ID
	PID() int

	// Wait waits for the command to finish and returns the result
	Wait(ctx context.Context) (*CommandResult, error)

	// Kill kills the running command
	Kill(ctx context.Context) error

	// SendStdin sends data to the command's stdin
	SendStdin(ctx context.Context, data string) error
}

// OutputHandler is a callback function for handling command output
type OutputHandler func(data string)

// EntryInfo contains information about a filesystem entry
type EntryInfo struct {
	Name     string
	Path     string
	Type     FileType
	Size     int64
	Modified time.Time
}

// FileType represents the type of a filesystem entry
type FileType string

const (
	FileTypeFile      FileType = "file"
	FileTypeDirectory FileType = "directory"
	FileTypeSymlink   FileType = "symlink"
)

// FilesystemEvent represents a filesystem event
type FilesystemEvent struct {
	Type FilesystemEventType
	Path string
}

// FilesystemEventType represents the type of filesystem event
type FilesystemEventType string

const (
	FilesystemEventTypeCreate FilesystemEventType = "create"
	FilesystemEventTypeModify FilesystemEventType = "modify"
	FilesystemEventTypeDelete FilesystemEventType = "delete"
)

// WatchHandle represents a handle to a filesystem watch operation
type WatchHandle interface {
	// Close stops watching the filesystem
	Close() error
}

// AsyncWatchHandle represents a handle to an async filesystem watch operation
type AsyncWatchHandle interface {
	// Close stops watching the filesystem
	Close() error
}
