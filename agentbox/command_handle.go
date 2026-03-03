package agentbox

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"
)

func extractEventMap(msgMap map[string]interface{}) (map[string]interface{}, bool) {
	if eventData, ok := msgMap["event"].(map[string]interface{}); ok {
		return eventData, true
	}
	if _, ok := msgMap["data"]; ok {
		return msgMap, true
	}
	if _, ok := msgMap["end"]; ok {
		return msgMap, true
	}
	if _, ok := msgMap["start"]; ok {
		return msgMap, true
	}
	if resultData, ok := msgMap["result"].(map[string]interface{}); ok {
		if eventData, ok := resultData["event"].(map[string]interface{}); ok {
			return eventData, true
		}
		if _, ok := resultData["data"]; ok {
			return resultData, true
		}
		if _, ok := resultData["end"]; ok {
			return resultData, true
		}
	}
	return nil, false
}

// commandHandleImpl implements CommandHandle
type commandHandleImpl struct {
	pid         int
	killFunc    func(ctx context.Context, pid int, requestTimeout time.Duration) (bool, error)
	stdinFunc   func(ctx context.Context, pid int, data string, requestTimeout time.Duration) error
	msgChan     <-chan interface{}
	errChan     <-chan error
	onStdout    OutputHandler
	onStderr    OutputHandler
	result      *CommandResult
	streamErr   error
	gotEndEvent bool
	resultMutex sync.Mutex
	done        chan struct{}
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewCommandHandle creates a new command handle
func NewCommandHandle(
	pid int,
	killFunc func(ctx context.Context, pid int, requestTimeout time.Duration) (bool, error),
	stdinFunc func(ctx context.Context, pid int, data string, requestTimeout time.Duration) error,
	msgChan <-chan interface{},
	errChan <-chan error,
	onStdout OutputHandler,
	onStderr OutputHandler,
) CommandHandle {
	ctx, cancel := context.WithCancel(context.Background())

	handle := &commandHandleImpl{
		pid:       pid,
		killFunc:  killFunc,
		stdinFunc: stdinFunc,
		msgChan:   msgChan,
		errChan:   errChan,
		onStdout:  onStdout,
		onStderr:  onStderr,
		done:      make(chan struct{}),
		ctx:       ctx,
		cancel:    cancel,
	}

	// Start processing events in background
	go handle.processEvents()

	return handle
}

// PID returns the process ID
func (h *commandHandleImpl) PID() int {
	return h.pid
}

// processEvents processes incoming events from the stream
func (h *commandHandleImpl) processEvents() {
	defer close(h.done)

	var stdout, stderr string
	msgChan := h.msgChan
	errChan := h.errChan

	for msgChan != nil || errChan != nil {
		select {
		case msg, ok := <-msgChan:
			if !ok {
				msgChan = nil
				continue
			}

			// Parse message (msg is map[string]interface{} from JSON decoder)
			msgMap, ok := msg.(map[string]interface{})
			if !ok {
				// Log unexpected message type for debugging
				// fmt.Printf("DEBUG: Unexpected message type: %T, value: %+v\n", msg, msg)
				continue
			}

			// Extract event field (support multiple response shapes)
			eventData, ok := extractEventMap(msgMap)
			if !ok {
				continue
			}

			// Handle data event (stdout/stderr)
			if dataData, ok := eventData["data"].(map[string]interface{}); ok {
				// stdout/stderr in JSON may be base64-encoded strings or plain strings
				if stdoutVal, ok := dataData["stdout"]; ok {
					var stdoutBytes []byte
					if stdoutStr, ok := stdoutVal.(string); ok {
						// Try to decode as base64 first (protobuf bytes in JSON are base64)
						if decoded, err := base64.StdEncoding.DecodeString(stdoutStr); err == nil {
							stdoutBytes = decoded
						} else {
							// Not base64, treat as plain string
							stdoutBytes = []byte(stdoutStr)
						}
					}
					if len(stdoutBytes) > 0 {
						text := string(stdoutBytes)
						stdout += text
						if h.onStdout != nil {
							h.onStdout(text)
						}
					}
				}

				if stderrVal, ok := dataData["stderr"]; ok {
					var stderrBytes []byte
					if stderrStr, ok := stderrVal.(string); ok {
						// Try to decode as base64 first
						if decoded, err := base64.StdEncoding.DecodeString(stderrStr); err == nil {
							stderrBytes = decoded
						} else {
							// Not base64, treat as plain string
							stderrBytes = []byte(stderrStr)
						}
					}
					if len(stderrBytes) > 0 {
						text := string(stderrBytes)
						stderr += text
						if h.onStderr != nil {
							h.onStderr(text)
						}
					}
				}
			}

			// Handle end event
			if endData, ok := eventData["end"].(map[string]interface{}); ok {
				// Match Python/protobuf semantics: when end event exists and exit_code is omitted,
				// exit_code defaults to 0 (proto3 scalar default), not -1.
				var exitCode int32 = 0
				// Support both snake_case and camelCase keys from different JSON codecs.
				exitCodeRaw, found := endData["exit_code"]
				if !found {
					exitCodeRaw = endData["exitCode"]
				}
				// Try multiple numeric types (JSON numbers can be float64 or int types).
				if exitCodeVal, ok := exitCodeRaw.(float64); ok {
					exitCode = int32(exitCodeVal)
				} else if exitCodeVal, ok := exitCodeRaw.(int); ok {
					exitCode = int32(exitCodeVal)
				} else if exitCodeVal, ok := exitCodeRaw.(int32); ok {
					exitCode = exitCodeVal
				} else if exitCodeVal, ok := exitCodeRaw.(int64); ok {
					exitCode = int32(exitCodeVal)
				} else if exitCodeVal, ok := exitCodeRaw.(json.Number); ok {
					if parsed, err := exitCodeVal.Int64(); err == nil {
						exitCode = int32(parsed)
					}
				} else if exitCodeVal, ok := exitCodeRaw.(string); ok {
					if parsed, err := strconv.Atoi(exitCodeVal); err == nil {
						exitCode = int32(parsed)
					}
				}

				h.resultMutex.Lock()
				h.gotEndEvent = true
				h.result = &CommandResult{
					ExitCode: int(exitCode),
					Stdout:   stdout,
					Stderr:   stderr,
				}
				h.resultMutex.Unlock()
				return
			}

		case err, ok := <-errChan:
			if !ok {
				errChan = nil
				continue
			}
			if err != nil {
				h.resultMutex.Lock()
				h.streamErr = err
				if h.result == nil {
					h.result = &CommandResult{
						ExitCode: -1,
						Stdout:   stdout,
						Stderr:   stderr,
					}
				}
				h.resultMutex.Unlock()
				return
			}

		case <-h.ctx.Done():
			return
		}
	}

	// Stream closed without explicit end event.
	h.resultMutex.Lock()
	if h.result == nil {
		h.result = &CommandResult{
			ExitCode: -1,
			Stdout:   stdout,
			Stderr:   stderr,
		}
	}
	h.resultMutex.Unlock()
}

// Wait waits for the command to finish and returns the result
func (h *commandHandleImpl) Wait(ctx context.Context) (*CommandResult, error) {
	// Wait for processEvents to finish or context to be cancelled
	select {
	case <-h.done:
		h.resultMutex.Lock()
		defer h.resultMutex.Unlock()
		if h.streamErr != nil {
			return nil, h.streamErr
		}
		if h.result == nil {
			return nil, fmt.Errorf("command handle closed without result")
		}
		if !h.gotEndEvent {
			return nil, fmt.Errorf("command ended without an end event")
		}
		return h.result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Kill kills the running command
func (h *commandHandleImpl) Kill(ctx context.Context) error {
	h.cancel()
	if h.killFunc != nil {
		_, err := h.killFunc(ctx, h.pid, 0)
		return err
	}
	return fmt.Errorf("kill function not available")
}

// SendStdin sends data to the command's stdin
func (h *commandHandleImpl) SendStdin(ctx context.Context, data string) error {
	if h.stdinFunc == nil {
		return fmt.Errorf("stdin function not available")
	}
	return h.stdinFunc(ctx, h.pid, data, 0)
}
