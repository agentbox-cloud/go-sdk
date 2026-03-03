package agentbox

import (
	"context"
	"time"
)

// Commands provides methods for executing commands in the sandbox
// This matches Python SDK's Commands interface
type Commands interface {
	// List lists all running commands and PTY sessions
	List(ctx context.Context, requestTimeout time.Duration) ([]*ProcessInfo, error)

	// Kill kills a running command specified by its process ID
	Kill(ctx context.Context, pid int, requestTimeout time.Duration) (bool, error)

	// SendStdin sends data to command stdin
	SendStdin(ctx context.Context, pid int, data string, requestTimeout time.Duration) error

	// Run runs a command and waits for it to finish (foreground execution)
	Run(ctx context.Context, cmd string, opts *RunCommandOptions) (*CommandResult, error)

	// RunBackground runs a command in the background and returns a handle
	RunBackground(ctx context.Context, cmd string, opts *RunCommandOptions) (CommandHandle, error)

	// Connect connects to a running command
	Connect(ctx context.Context, pid int, opts *ConnectCommandOptions) (CommandHandle, error)
}

// RunCommandOptions are options for running a command
// This matches Python SDK's command options
type RunCommandOptions struct {
	Envs           map[string]string
	User           Username
	Cwd            string
	OnStdout       OutputHandler
	OnStderr       OutputHandler
	Timeout        time.Duration // Timeout for command execution (0 = no limit)
	RequestTimeout time.Duration
}

// ConnectCommandOptions are options for connecting to a running command
type ConnectCommandOptions struct {
	Timeout        time.Duration
	RequestTimeout time.Duration
	OnStdout       OutputHandler
	OnStderr       OutputHandler
}

// CommandHandle represents a handle to a running command (for background execution)
// This matches Python SDK's CommandHandle
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

// NewCommands creates a new commands instance
// This is a factory function for creating commands implementations
// Implementation is in sandbox_sync/commands package
func NewCommands(envdAPIURL string, config *ConnectionConfig) Commands {
	// Import and use the implementation from sandbox_sync/commands
	return newCommandsImpl(envdAPIURL, config)
}

// newCommandsImpl is the internal function to create commands implementation
func newCommandsImpl(envdAPIURL string, config *ConnectionConfig) Commands {
	// The actual implementation is in sandbox_sync/commands package
	// This function is kept for backward compatibility
	// The real implementation is created directly in sandbox_sync/main.go
	return nil
}
