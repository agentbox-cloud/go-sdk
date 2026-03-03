package agentbox

import (
	"context"
	"time"
)

// NOTE:
// Root package contracts are kept for backward compatibility.
// Structured package entry (Python SDK aligned) is:
//   github.com/agentbox-cloud/go-sdk/agentbox/sandbox/commands

// Commands provides methods for executing commands in the sandbox
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
