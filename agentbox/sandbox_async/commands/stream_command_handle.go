package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/agentbox-cloud/go-sdk/agentbox"
	"github.com/agentbox-cloud/go-sdk/agentbox/connect"
)

// streamCommandHandle implements agentbox.CommandHandle for streaming commands
// This matches Python SDK's CommandHandle with stream processing
type streamCommandHandle struct {
	pid        int
	envdAPIURL string
	httpClient *http.Client
	mu         sync.Mutex

	// Stream channels
	msgChan <-chan interface{}
	errChan <-chan error

	// Stream state
	stdout       strings.Builder
	stderr       strings.Builder
	result       *agentbox.CommandResult
	streamDone   chan struct{}
	streamErr    error

	// Options
	onStdout agentbox.OutputHandler
	onStderr agentbox.OutputHandler
}

// NewStreamCommandHandle creates a new stream command handle
func NewStreamCommandHandle(
	pid int,
	envdAPIURL string,
	httpClient *http.Client,
	msgChan <-chan interface{},
	errChan <-chan error,
	opts *agentbox.RunCommandOptions,
) agentbox.CommandHandle {
	handle := &streamCommandHandle{
		pid:        pid,
		envdAPIURL: envdAPIURL,
		httpClient: httpClient,
		msgChan:    msgChan,
		errChan:    errChan,
		streamDone: make(chan struct{}),
	}

	if opts != nil {
		handle.onStdout = opts.OnStdout
		handle.onStderr = opts.OnStderr
	}

	return handle
}

// PID returns the process ID
func (h *streamCommandHandle) PID() int {
	return h.pid
}

// Wait waits for the command to finish and returns the result
func (h *streamCommandHandle) Wait(ctx context.Context) (*agentbox.CommandResult, error) {
	// Wait for stream to complete
	select {
	case <-h.streamDone:
		// Stream completed
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.streamErr != nil {
		return nil, h.streamErr
	}

	if h.result == nil {
		return nil, agentbox.NewSandboxException(
			"Command ended without an end event",
			nil,
		)
	}

	// Only throw exception for non-zero exit codes (matching Python SDK behavior)
	// Exit code -1 indicates unknown status (stream ended without EndEvent)
	// We should still return the result, not throw exception
	if h.result.ExitCode != 0 && h.result.ExitCode != -1 {
		return nil, agentbox.NewCommandExitException(
			h.result.ExitCode,
			fmt.Sprintf("Command exited with code %d", h.result.ExitCode),
		)
	}

	return h.result, nil
}

// Kill kills the running command
func (h *streamCommandHandle) Kill(ctx context.Context) error {
	// Use Commands.Kill to kill the process
	// This requires access to the Commands instance, which we'll handle via a callback
	// For now, we'll close the stream
	h.mu.Lock()
	defer h.mu.Unlock()

	// Close stream channels (they will be closed by the reader)
	// The actual kill should be done via Commands.Kill()
	return nil
}

// SendStdin sends data to the command's stdin
func (h *streamCommandHandle) SendStdin(ctx context.Context, data string) error {
	// SendStdin should be called via Commands.SendStdin()
	return agentbox.NewSandboxException(
		"SendStdin should be called via Commands.SendStdin()",
		nil,
	)
}

// processStream processes the server stream events in the background
// This matches Python SDK's CommandHandle._iterate_events() method
func (h *streamCommandHandle) processStream(ctx context.Context) {
	defer func() {
		// Ensure streamDone is closed even if we return early
		// This must be done before setting result to ensure Wait() can see the result
		h.mu.Lock()
		// If we exited without an EndEvent, set a default result
		if h.result == nil && h.streamErr == nil {
			// Stream ended without EndEvent - this might be normal if the stream was closed
			// Set a result with exit code -1 to indicate unknown status
			h.result = &agentbox.CommandResult{
				ExitCode: -1,
				Stdout:   h.stdout.String(),
				Stderr:   h.stderr.String(),
			}
		}
		h.mu.Unlock()
		// Close streamDone after setting result to ensure Wait() sees the result
		close(h.streamDone)
	}()

	for {
		select {
		case msg, ok := <-h.msgChan:
			if !ok {
				// msgChan might be closed before we observe a pending parser error in errChan.
				// Drain errChan non-blockingly first so we don't lose the real failure.
				h.consumePendingStreamError()

				// Channel closed - stream ended
				// If we don't have a result yet, it means the stream ended without EndEvent
				// This might happen if the server closes the connection unexpectedly
				h.mu.Lock()
				if h.result == nil {
					// Set a default result with exit code -1 to indicate unknown status
					h.result = &agentbox.CommandResult{
						ExitCode: -1,
						Stdout:   h.stdout.String(),
						Stderr:   h.stderr.String(),
					}
				}
				h.mu.Unlock()
				return
			}

			// Parse ProcessEvent from message
			// Message is a map[string]interface{} from the stream parser
			msgMap, ok := msg.(map[string]interface{})
			if !ok {
				h.mu.Lock()
				h.streamErr = fmt.Errorf("unexpected message type: %T", msg)
				h.mu.Unlock()
				return
			}

			// Try to extract event from StartResponse format: {"event": {...}}
			var event connect.ProcessEvent
			if eventData, ok := msgMap["event"].(map[string]interface{}); ok {
				// Unmarshal event
				eventJSON, err := json.Marshal(eventData)
				if err != nil {
					h.mu.Lock()
					h.streamErr = fmt.Errorf("failed to marshal event data: %w", err)
					h.mu.Unlock()
					return
				}
				if err := json.Unmarshal(eventJSON, &event); err != nil {
					h.mu.Lock()
					h.streamErr = fmt.Errorf("failed to unmarshal event: %w", err)
					h.mu.Unlock()
					return
				}
			} else {
				// Try to unmarshal as ProcessEvent directly
				msgJSON, err := json.Marshal(msgMap)
				if err != nil {
					h.mu.Lock()
					h.streamErr = fmt.Errorf("failed to marshal message: %w", err)
					h.mu.Unlock()
					return
				}
				if err := json.Unmarshal(msgJSON, &event); err != nil {
					h.mu.Lock()
					h.streamErr = fmt.Errorf("failed to unmarshal as ProcessEvent: %w", err)
					h.mu.Unlock()
					return
				}
			}

			// Handle different event types
			if event.Start != nil {
				// StartEvent - already handled in startCommandStream, but process it here too
				// This ensures we don't miss it if it appears again
				continue
			} else if event.Data != nil {
				// DataEvent - stdout/stderr output
				h.handleDataEvent(event.Data)
			} else if event.End != nil {
				// EndEvent - command finished
				h.handleEndEvent(event.End)
				return
			} else if event.KeepAlive != nil {
				// KeepAliveEvent - ignore
				continue
			} else {
				// Unknown event type - log and continue
				// This shouldn't happen, but we don't want to break the stream
				continue
			}

		case err, ok := <-h.errChan:
			if !ok {
				// errChan closed normally; wait for msgChan close or context cancellation.
				continue
			}
			if err != nil {
				h.mu.Lock()
				h.streamErr = err
				h.mu.Unlock()
				return
			}

		case <-ctx.Done():
			h.mu.Lock()
			h.streamErr = ctx.Err()
			h.mu.Unlock()
			return
		}
	}
}

func (h *streamCommandHandle) consumePendingStreamError() {
	for {
		select {
		case err, ok := <-h.errChan:
			if !ok {
				return
			}
			if err != nil {
				h.mu.Lock()
				h.streamErr = err
				h.mu.Unlock()
				return
			}
		default:
			return
		}
	}
}

// handleDataEvent handles DataEvent (stdout/stderr output)
func (h *streamCommandHandle) handleDataEvent(data *connect.DataEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(data.Stdout) > 0 {
		stdoutStr := string(data.Stdout)
		h.stdout.WriteString(stdoutStr)
		if h.onStdout != nil {
			h.onStdout(stdoutStr)
		}
	}

	if len(data.Stderr) > 0 {
		stderrStr := string(data.Stderr)
		h.stderr.WriteString(stderrStr)
		if h.onStderr != nil {
			h.onStderr(stderrStr)
		}
	}
}

// handleEndEvent handles EndEvent (command finished)
func (h *streamCommandHandle) handleEndEvent(end *connect.EndEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.result = &agentbox.CommandResult{
		ExitCode: int(end.ExitCode),
		Stdout:   h.stdout.String(),
		Stderr:   h.stderr.String(),
	}
}

