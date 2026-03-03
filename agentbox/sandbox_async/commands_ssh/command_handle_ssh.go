package commands_ssh

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"

	"github.com/agentbox-cloud/go-sdk/agentbox"
)

const (
	doneMarker = "__CMD_DONE__"
)

// SSHCommandHandle implements agentbox.CommandHandle interface for SSH commands
// This matches Python SDK's SSHAsyncCommandHandle class in sandbox_async/commands_ssh/command_handle_ssh.py
type SSHCommandHandle struct {
	pid         int
	killFunc    func(ctx context.Context) error
	session     *ssh.Session
	stdin       io.WriteCloser
	stdout      io.Reader
	stderr      io.Reader
	onStdout    agentbox.OutputHandler
	onStderr    agentbox.OutputHandler
	mu          sync.Mutex
	stdoutData  strings.Builder
	stderrData  strings.Builder
	result      *agentbox.CommandResult
	closed      bool
	readingDone chan struct{}
}

// NewSSHCommandHandle creates a new SSH command handle
// This matches Python SDK's SSHAsyncCommandHandle.__init__()
func NewSSHCommandHandle(
	pid int,
	killFunc func(ctx context.Context) error,
	session *ssh.Session,
	stdin io.WriteCloser,
	onStdout agentbox.OutputHandler,
	onStderr agentbox.OutputHandler,
) agentbox.CommandHandle {
	stdout, _ := session.StdoutPipe()
	stderr, _ := session.StderrPipe()

	return &SSHCommandHandle{
		pid:         pid,
		killFunc:    killFunc,
		session:     session,
		stdin:       stdin,
		stdout:      stdout,
		stderr:      stderr,
		onStdout:    onStdout,
		onStderr:    onStderr,
		readingDone: make(chan struct{}),
	}
}

// PID returns the process ID
func (h *SSHCommandHandle) PID() int {
	return h.pid
}

// Wait waits for the command to finish and returns the result
// This matches Python SDK's SSHAsyncCommandHandle.wait()
func (h *SSHCommandHandle) Wait(ctx context.Context) (*agentbox.CommandResult, error) {
	// Wait for reading to complete
	select {
	case <-h.readingDone:
		// Reading completed
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.result == nil {
		return nil, agentbox.NewSandboxException(
			"Command ended without an end event",
			nil,
		)
	}

	if h.result.ExitCode != 0 {
		return nil, agentbox.NewCommandExitException(
			h.result.ExitCode,
			fmt.Sprintf("Command exited with code %d", h.result.ExitCode),
		)
	}

	return h.result, nil
}

// Kill kills the running command
// This matches Python SDK's SSHAsyncCommandHandle.kill()
func (h *SSHCommandHandle) Kill(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return nil
	}

	h.closed = true

	// Close stdin to signal termination
	if h.stdin != nil {
		h.stdin.Close()
	}

	// Close session
	if h.session != nil {
		h.session.Close()
	}

	// Call kill function
	if h.killFunc != nil {
		return h.killFunc(ctx)
	}

	return nil
}

// SendStdin sends data to the command's stdin
// This matches Python SDK's SSHAsyncCommandHandle.send_stdin()
func (h *SSHCommandHandle) SendStdin(ctx context.Context, data string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed || h.stdin == nil {
		return agentbox.NewSandboxException(
			"Command handle is closed or stdin is not available",
			nil,
		)
	}

	_, err := h.stdin.Write([]byte(data))
	return err
}

// startReading starts reading command output in the background
// This matches Python SDK's SSHAsyncCommandHandle._handle_channel()
func (h *SSHCommandHandle) startReading(ctx context.Context) {
	defer close(h.readingDone)

	var wg sync.WaitGroup
	wg.Add(2)

	// Read stdout
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(h.stdout)
		for scanner.Scan() {
			h.mu.Lock()
			closed := h.closed
			h.mu.Unlock()
			if closed {
				break
			}
			chunk := scanner.Text() + "\n"
			h.mu.Lock()
			h.stdoutData.WriteString(chunk)
			stdoutStr := h.stdoutData.String()
			h.mu.Unlock()

			// Call onStdout handler if provided
			if h.onStdout != nil {
				h.onStdout(chunk)
			}

			// Check for done marker
			if strings.Contains(chunk, doneMarker) {
				exitCode := h.parseExitCode(stdoutStr)
				h.finishCommand(exitCode)
				break
			}
		}
		if err := scanner.Err(); err != nil && err != io.EOF {
			// Handle scan error
		}
	}()

	// Read stderr
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(h.stderr)
		for scanner.Scan() {
			h.mu.Lock()
			closed := h.closed
			h.mu.Unlock()
			if closed {
				break
			}
			chunk := scanner.Text() + "\n"
			h.mu.Lock()
			h.stderrData.WriteString(chunk)
			h.mu.Unlock()

			// Call onStderr handler if provided
			if h.onStderr != nil {
				h.onStderr(chunk)
			}
		}
		if err := scanner.Err(); err != nil && err != io.EOF {
			// Handle scan error
		}
	}()

	// Wait for both readers to finish
	wg.Wait()

	// Wait for session to complete
	h.session.Wait()
}

// parseExitCode parses exit code from command output
// This matches Python SDK's exit code parsing logic
func (h *SSHCommandHandle) parseExitCode(output string) int {
	// Look for __CMD_DONE__<exit_code> pattern
	re := regexp.MustCompile(doneMarker + `(\d+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) > 1 {
		var exitCode int
		if _, err := fmt.Sscanf(matches[1], "%d", &exitCode); err == nil {
			return exitCode
		}
	}
	return -1
}

// finishCommand finishes the command and creates the result
// This matches Python SDK's result creation logic
func (h *SSHCommandHandle) finishCommand(exitCode int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Strip echo and prompt from stdout (similar to Python SDK's OutputUtils.strip_echo_and_prompt)
	stdout := h.stripEchoAndPrompt(h.stdoutData.String())
	stderr := h.stderrData.String()

	h.result = &agentbox.CommandResult{
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
	}
}

// stripEchoAndPrompt strips echo commands and shell prompts from output
// This matches Python SDK's OutputUtils.strip_echo_and_prompt()
func (h *SSHCommandHandle) stripEchoAndPrompt(output string) string {
	// Remove common shell prompts and echo commands
	// This is a simplified version - Python SDK has more sophisticated logic
	lines := strings.Split(output, "\n")
	var filtered []string
	for _, line := range lines {
		// Skip lines that look like prompts or echo commands
		if strings.HasPrefix(line, "$ ") ||
			strings.HasPrefix(line, "# ") ||
			strings.HasPrefix(line, "> ") ||
			strings.Contains(line, "echo __CMD_DONE__") {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}
