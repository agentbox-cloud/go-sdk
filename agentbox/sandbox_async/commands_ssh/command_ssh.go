package commands_ssh

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/agentbox-cloud/go-sdk/agentbox"
)

// SSHCommands implements agentbox.Commands interface using SSH
// This matches Python SDK's SSHCommands class in sandbox_async/commands_ssh/command_ssh.py
type SSHCommands struct {
	sshHost          string
	sshPort          int
	sshUsername      string
	sshPassword      string
	connectionConfig *agentbox.ConnectionConfig
	processes        map[int]agentbox.CommandHandle
	mu               sync.Mutex
	client           *ssh.Client
}

// NewSSHCommands creates a new SSH commands implementation
// This matches Python SDK's SSHCommands.__init__()
func NewSSHCommands(
	sshHost string,
	sshPort int,
	sshUsername string,
	sshPassword string,
	connectionConfig *agentbox.ConnectionConfig,
) agentbox.Commands {
	return &SSHCommands{
		sshHost:          sshHost,
		sshPort:          sshPort,
		sshUsername:      sshUsername,
		sshPassword:      sshPassword,
		connectionConfig: connectionConfig,
		processes:        make(map[int]agentbox.CommandHandle),
	}
}

// getSSHClient gets or creates an SSH client connection
// This matches Python SDK's SSHCommands._get_ssh_client()
func (c *SSHCommands) getSSHClient(ctx context.Context) (*ssh.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if client exists and is active
	if c.client != nil {
		// Check if connection is still alive
		conn := c.client.Conn
		if conn != nil {
			// Try to send a keepalive to check if connection is alive
			_, _, err := conn.SendRequest("keepalive@openssh.com", false, nil)
			if err == nil {
				return c.client, nil
			}
		}
		// Connection is dead, close it
		c.client.Close()
		c.client = nil
	}

	// Create new SSH client
	config := &ssh.ClientConfig{
		User:            c.sshUsername,
		Auth:            []ssh.AuthMethod{ssh.Password(c.sshPassword)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Match Python's AutoAddPolicy
		Timeout:         60 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", c.sshHost, c.sshPort)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	c.client = client
	return client, nil
}

// List lists all running commands
// This matches Python SDK's SSHCommands.list()
func (c *SSHCommands) List(ctx context.Context, requestTimeout time.Duration) ([]*agentbox.ProcessInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	processes := make([]*agentbox.ProcessInfo, 0, len(c.processes))
	for _, handle := range c.processes {
		processes = append(processes, &agentbox.ProcessInfo{
			PID:  handle.PID(),
			Tag:  fmt.Sprintf("ssh-%d", handle.PID()),
			Cmd:  "",
			Args: []string{},
			Envs: make(map[string]string),
			Cwd:  "/",
		})
	}

	return processes, nil
}

// Kill kills a running command
// This matches Python SDK's SSHCommands.kill()
func (c *SSHCommands) Kill(ctx context.Context, pid int, requestTimeout time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	handle, exists := c.processes[pid]
	if exists {
		err := handle.Kill(ctx)
		delete(c.processes, pid)
		return err == nil, err
	}
	return false, nil
}

// SendStdin sends data to stdin of a running command
// This matches Python SDK's SSHCommands.send_stdin()
func (c *SSHCommands) SendStdin(ctx context.Context, pid int, data string, requestTimeout time.Duration) error {
	c.mu.Lock()
	handle, exists := c.processes[pid]
	c.mu.Unlock()

	if !exists {
		return agentbox.NewSandboxException(
			fmt.Sprintf("Process %d not found", pid),
			nil,
		)
	}
	return handle.SendStdin(ctx, data)
}

// Run runs a command and waits for it to complete
// This matches Python SDK's SSHCommands.run()
func (c *SSHCommands) Run(ctx context.Context, cmd string, opts *agentbox.RunCommandOptions) (*agentbox.CommandResult, error) {
	// Start command in background, then wait for it
	handle, err := c.RunBackground(ctx, cmd, opts)
	if err != nil {
		return nil, err
	}

	// Wait for command to finish
	return handle.Wait(ctx)
}

// RunBackground runs a command in the background
// This matches Python SDK's SSHCommands.run(background=True)
func (c *SSHCommands) RunBackground(ctx context.Context, cmd string, opts *agentbox.RunCommandOptions) (agentbox.CommandHandle, error) {
	if opts == nil {
		opts = &agentbox.RunCommandOptions{}
	}

	// Get SSH client
	client, err := c.getSSHClient(ctx)
	if err != nil {
		return nil, err
	}

	// Build command string with environment variables and working directory
	envStr := ""
	if opts.Envs != nil && len(opts.Envs) > 0 {
		for k, v := range opts.Envs {
			envStr += fmt.Sprintf("%s=%s ", k, v)
		}
	}

	var fullCmd string
	if opts.Cwd != "" {
		fullCmd = fmt.Sprintf("cd %s && %s%s; echo __CMD_DONE__$?; exit\n", opts.Cwd, envStr, cmd)
	} else {
		fullCmd = fmt.Sprintf("%s%s ; echo __CMD_DONE__$?; exit\n", envStr, cmd)
	}

	// Create SSH session (shell)
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH session: %w", err)
	}

	// Request PTY for interactive shell
	if err := session.RequestPty("xterm", 80, 40, ssh.TerminalModes{}); err != nil {
		session.Close()
		return nil, fmt.Errorf("failed to request PTY: %w", err)
	}

	// Start shell
	if err := session.Shell(); err != nil {
		session.Close()
		return nil, fmt.Errorf("failed to start shell: %w", err)
	}

	// Get stdin pipe
	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	// Send command
	if _, err := stdin.Write([]byte(fullCmd)); err != nil {
		session.Close()
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	// Generate PID (similar to Python SDK: len(processes) + 1000)
	c.mu.Lock()
	pid := len(c.processes) + 1000
	c.mu.Unlock()

	// Create command handle
	handle := NewSSHCommandHandle(
		pid,
		func(ctx context.Context) error {
			return c.Kill(ctx, pid, 0)
		},
		session,
		stdin,
		opts.OnStdout,
		opts.OnStderr,
	)

	// Store handle
	c.mu.Lock()
	c.processes[pid] = handle
	c.mu.Unlock()

	// Start reading output in background
	go handle.startReading(ctx)

	return handle, nil
}

// Connect connects to an existing command/PTY session
// This matches Python SDK's SSHCommands.connect()
func (c *SSHCommands) Connect(ctx context.Context, pid int, opts *agentbox.ConnectCommandOptions) (agentbox.CommandHandle, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	handle, exists := c.processes[pid]
	if !exists {
		return nil, agentbox.NewSandboxException(
			fmt.Sprintf("Process %d not found", pid),
			nil,
		)
	}
	return handle, nil
}
