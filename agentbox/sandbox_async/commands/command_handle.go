package commands

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/agentbox-cloud/go-sdk/agentbox"
)

// commandHandleImpl implements agentbox.CommandHandle interface for async operations
// This matches Python SDK's CommandHandle class in sandbox_async/commands/command_handle.py
type commandHandleImpl struct {
	pid              int
	envdAPIURL       string
	connectionConfig *agentbox.ConnectionConfig
	httpClient       *http.Client
	killFunc         func(ctx context.Context, pid int) error
	mu               sync.Mutex

	// Stream state
	stdout       string
	stderr       string
	result       *agentbox.CommandResult
	streamReader io.ReadCloser
	streamClosed bool
	streamErr    error
}

// NewCommandHandle creates a new async command handle
func NewCommandHandle(
	pid int,
	envdAPIURL string,
	config *agentbox.ConnectionConfig,
	httpClient *http.Client,
	killFunc func(ctx context.Context, pid int) error,
) agentbox.CommandHandle {
	return &commandHandleImpl{
		pid:              pid,
		envdAPIURL:       envdAPIURL,
		connectionConfig: config,
		httpClient:       httpClient,
		killFunc:         killFunc,
		streamClosed:     false,
	}
}

// PID returns the process ID
func (c *commandHandleImpl) PID() int {
	return c.pid
}

// Wait waits for the command to finish and returns the result
// This matches Python SDK's AsyncCommandHandle.wait()
func (c *commandHandleImpl) Wait(ctx context.Context) (*agentbox.CommandResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If we already have a result, return it
	if c.result != nil {
		if c.result.ExitCode != 0 {
			return nil, agentbox.NewCommandExitException(
				c.result.ExitCode,
				fmt.Sprintf("Command exited with code %d", c.result.ExitCode),
			)
		}
		return c.result, nil
	}

	// Process stream events
	// TODO: Implement proper server stream processing
	// For now, poll for result or wait for stream to complete
	if c.streamReader != nil {
		// Process stream
		err := c.processStream(ctx)
		if err != nil {
			return nil, err
		}
	}

	if c.result == nil {
		return nil, agentbox.NewSandboxException(
			"Command ended without an end event",
			nil,
		)
	}

	if c.result.ExitCode != 0 {
		return nil, agentbox.NewCommandExitException(
			c.result.ExitCode,
			fmt.Sprintf("Command exited with code %d", c.result.ExitCode),
		)
	}

	return c.result, nil
}

// Kill kills the running command
// This matches Python SDK's AsyncCommandHandle.kill()
func (c *commandHandleImpl) Kill(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.killFunc != nil {
		err := c.killFunc(ctx, c.pid)
		return err
	}

	// Fallback: close stream
	if c.streamReader != nil {
		c.streamReader.Close()
		c.streamClosed = true
	}

	return nil
}

// SendStdin sends data to the command's stdin
// This matches Python SDK's AsyncCommandHandle.send_stdin() (via Commands.send_stdin)
func (c *commandHandleImpl) SendStdin(ctx context.Context, data string) error {
	// This should use the Commands instance to send stdin
	// For now, return error indicating it needs to be called via Commands
	return agentbox.NewSandboxException(
		"SendStdin should be called via Commands.SendStdin()",
		nil,
	)
}

// processStream processes the server stream events
// This handles stdout, stderr, and end events from the stream
func (c *commandHandleImpl) processStream(ctx context.Context) error {
	// TODO: Implement server stream processing
	// This requires parsing server-sent events or HTTP streaming
	// For now, return error
	return agentbox.NewSandboxException(
		"Server stream processing not yet implemented",
		nil,
	)
}

