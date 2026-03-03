package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/agentbox-cloud/go-sdk/agentbox"
	"github.com/agentbox-cloud/go-sdk/agentbox/connect"
)

const (
	// ProcessServiceName is the RPC service name for process
	ProcessServiceName = "process.Process"
)

// commandsImpl implements agentbox.Commands interface
// This matches Python SDK's Commands class in sandbox_sync/commands/command.py
type commandsImpl struct {
	envdAPIURL       string
	connectionConfig *agentbox.ConnectionConfig
	httpClient       *http.Client
	mu               sync.Mutex // For thread safety
}

// NewCommands creates a new commands implementation
// This matches Python SDK's Commands.__init__()
func NewCommands(envdAPIURL string, config *agentbox.ConnectionConfig) agentbox.Commands {
	httpClient := &http.Client{
		Timeout: config.RequestTimeout,
	}

	return &commandsImpl{
		envdAPIURL:       envdAPIURL,
		connectionConfig: config,
		httpClient:       httpClient,
	}
}

// List lists all running commands and PTY sessions
// This matches Python SDK's Commands.list()
func (c *commandsImpl) List(ctx context.Context, requestTimeout time.Duration) ([]*agentbox.ProcessInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Prepare RPC request (empty for List)
	req := map[string]interface{}{}

	// Prepare response
	var resp struct {
		Processes []map[string]interface{} `json:"processes"`
	}

	// Get timeout
	timeout := c.connectionConfig.GetRequestTimeout(requestTimeout)

	// Get headers (no auth needed for list, matching Python SDK)
	headers := make(map[string]string)
	if c.connectionConfig.AccessToken != "" {
		headers["X-Access-Token"] = c.connectionConfig.AccessToken
	}
	for k, v := range c.connectionConfig.Headers {
		headers[k] = v
	}

	// Make RPC call
	err := agentbox.CallRPC(
		ctx,
		c.envdAPIURL,
		ProcessServiceName,
		"List",
		req,
		&resp,
		headers,
		timeout,
		c.httpClient,
	)
	if err != nil {
		return nil, agentbox.HandleRPCException(err)
	}

	// Convert processes
	processes := make([]*agentbox.ProcessInfo, 0, len(resp.Processes))
	for _, procData := range resp.Processes {
		proc := parseRPCProcessInfo(procData)
		if proc != nil {
			processes = append(processes, proc)
		}
	}

	return processes, nil
}

// Kill kills a running command specified by its process ID
// This matches Python SDK's Commands.kill()
func (c *commandsImpl) Kill(ctx context.Context, pid int, requestTimeout time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Prepare RPC request
	req := map[string]interface{}{
		"process": map[string]interface{}{
			"pid": pid,
		},
		"signal": "SIGNAL_SIGKILL", // Try string first
	}

	// Prepare response (empty for SendSignal)
	var resp struct{}

	// Get timeout
	timeout := c.connectionConfig.GetRequestTimeout(requestTimeout)

	// Get headers
	headers := make(map[string]string)
	if c.connectionConfig.AccessToken != "" {
		headers["X-Access-Token"] = c.connectionConfig.AccessToken
	}
	for k, v := range c.connectionConfig.Headers {
		headers[k] = v
	}

	// Make RPC call
	err := agentbox.CallRPC(
		ctx,
		c.envdAPIURL,
		ProcessServiceName,
		"SendSignal",
		req,
		&resp,
		headers,
		timeout,
		c.httpClient,
	)
	if err != nil {
		// Check for not_found error
		if httpErr, ok := err.(*agentbox.HTTPRPCError); ok && httpErr.Code == "not_found" {
			return false, nil
		}
		// Try with numeric signal (9 = SIGKILL)
		req["signal"] = 9
		err2 := agentbox.CallRPC(
			ctx,
			c.envdAPIURL,
			ProcessServiceName,
			"SendSignal",
			req,
			&resp,
			headers,
			timeout,
			c.httpClient,
		)
		if err2 != nil {
			if httpErr, ok := err2.(*agentbox.HTTPRPCError); ok && httpErr.Code == "not_found" {
				return false, nil
			}
			return false, agentbox.HandleRPCException(err2)
		}
	}

	return true, nil
}

// SendStdin sends data to command stdin
// This matches Python SDK's Commands.send_stdin()
func (c *commandsImpl) SendStdin(ctx context.Context, pid int, data string, requestTimeout time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Prepare RPC request
	// Note: Python SDK encodes stdin as bytes, but JSON will handle string
	req := map[string]interface{}{
		"process": map[string]interface{}{
			"pid": pid,
		},
		"input": map[string]interface{}{
			"stdin": data, // JSON will encode string as-is
		},
	}

	// Prepare response (empty for SendInput)
	var resp struct{}

	// Get timeout
	timeout := c.connectionConfig.GetRequestTimeout(requestTimeout)

	// Get headers
	headers := make(map[string]string)
	if c.connectionConfig.AccessToken != "" {
		headers["X-Access-Token"] = c.connectionConfig.AccessToken
	}
	for k, v := range c.connectionConfig.Headers {
		headers[k] = v
	}

	// Make RPC call
	err := agentbox.CallRPC(
		ctx,
		c.envdAPIURL,
		ProcessServiceName,
		"SendInput",
		req,
		&resp,
		headers,
		timeout,
		c.httpClient,
	)
	if err != nil {
		return agentbox.HandleRPCException(err)
	}

	return nil
}

// Run runs a command and waits for it to finish (foreground execution)
// This matches Python SDK's Commands.run() with background=False
func (c *commandsImpl) Run(ctx context.Context, cmd string, opts *agentbox.RunCommandOptions) (*agentbox.CommandResult, error) {
	// Start command in background, then wait for it
	handle, err := c.RunBackground(ctx, cmd, opts)
	if err != nil {
		return nil, err
	}

	// Wait for command to finish
	return handle.Wait(ctx)
}

// RunBackground runs a command in the background and returns a handle
// This matches Python SDK's Commands.run() with background=True
func (c *commandsImpl) RunBackground(ctx context.Context, cmd string, opts *agentbox.RunCommandOptions) (agentbox.CommandHandle, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if opts == nil {
		opts = &agentbox.RunCommandOptions{}
	}

	// Build process config (matching Python SDK: cmd="/bin/bash", args=["-l", "-c", cmd])
	processConfig := map[string]interface{}{
		"cmd":  "/bin/bash",
		"args": []string{"-l", "-c", cmd},
		"envs": opts.Envs,
	}
	if opts.Cwd != "" {
		processConfig["cwd"] = opts.Cwd
	}

	// Prepare RPC request
	req := map[string]interface{}{
		"process": processConfig,
	}

	// Get timeout
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second // Default 60 seconds, matching Python SDK
	}

	requestTimeout := c.connectionConfig.GetRequestTimeout(opts.RequestTimeout)

	// Get auth headers
	user := opts.User
	if user == "" {
		user = agentbox.DefaultUsername
	}
	headers := agentbox.AuthenticationHeader(user)
	headers[agentbox.KeepalivePingHeader] = fmt.Sprintf("%d", agentbox.KeepalivePingIntervalSec)
	if c.connectionConfig.AccessToken != "" {
		headers["X-Access-Token"] = c.connectionConfig.AccessToken
	}
	for k, v := range c.connectionConfig.Headers {
		headers[k] = v
	}

	// Start server stream RPC
	// Note: This requires server-sent events or HTTP streaming
	// For now, we'll use a simplified approach with polling or implement streaming
	handle, err := startCommandStream(ctx, c.envdAPIURL, req, headers, timeout, requestTimeout, c.httpClient, opts)
	if err != nil {
		return nil, agentbox.HandleRPCException(err)
	}

	return handle, nil
}

// Connect connects to a running command
// This matches Python SDK's Commands.connect()
func (c *commandsImpl) Connect(ctx context.Context, pid int, opts *agentbox.ConnectCommandOptions) (agentbox.CommandHandle, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if opts == nil {
		opts = &agentbox.ConnectCommandOptions{}
	}

	// Prepare RPC request
	req := map[string]interface{}{
		"process": map[string]interface{}{
			"pid": pid,
		},
	}

	// Get timeout
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second // Default 60 seconds
	}

	requestTimeout := c.connectionConfig.GetRequestTimeout(opts.RequestTimeout)

	// Get headers
	headers := make(map[string]string)
	headers[agentbox.KeepalivePingHeader] = fmt.Sprintf("%d", agentbox.KeepalivePingIntervalSec)
	if c.connectionConfig.AccessToken != "" {
		headers["X-Access-Token"] = c.connectionConfig.AccessToken
	}
	for k, v := range c.connectionConfig.Headers {
		headers[k] = v
	}

	// Connect to command stream
	handle, err := connectCommandStream(ctx, c.envdAPIURL, req, headers, timeout, requestTimeout, c.httpClient, opts)
	if err != nil {
		return nil, agentbox.HandleRPCException(err)
	}

	return handle, nil
}

// Helper functions

// parseRPCProcessInfo parses ProcessInfo from RPC response
func parseRPCProcessInfo(data map[string]interface{}) *agentbox.ProcessInfo {
	if data == nil {
		return nil
	}

	info := &agentbox.ProcessInfo{}

	if pid, ok := data["pid"].(float64); ok {
		info.PID = int(pid)
	}

	if tag, ok := data["tag"].(string); ok {
		info.Tag = tag
	}

	// Parse config
	if config, ok := data["config"].(map[string]interface{}); ok {
		if cmd, ok := config["cmd"].(string); ok {
			info.Cmd = cmd
		}

		if args, ok := config["args"].([]interface{}); ok {
			info.Args = make([]string, 0, len(args))
			for _, arg := range args {
				if str, ok := arg.(string); ok {
					info.Args = append(info.Args, str)
				}
			}
		}

		if envs, ok := config["envs"].(map[string]interface{}); ok {
			info.Envs = make(map[string]string)
			for k, v := range envs {
				if str, ok := v.(string); ok {
					info.Envs[k] = str
				}
			}
		}

		if cwd, ok := config["cwd"].(string); ok {
			info.Cwd = cwd
		}
	}

	return info
}

// startCommandStream starts a command and returns a handle for the event stream
// This handles server stream RPC for Start method
func startCommandStream(
	ctx context.Context,
	envdAPIURL string,
	req map[string]interface{},
	headers map[string]string,
	timeout time.Duration,
	requestTimeout time.Duration,
	httpClient *http.Client,
	opts *agentbox.RunCommandOptions,
) (agentbox.CommandHandle, error) {
	// Convert request map to StartRequest
	var startReq connect.StartRequest
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	if err := json.Unmarshal(reqJSON, &startReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request: %w", err)
	}

	// Create Connect Protocol client
	codec := connect.NewJSONCodec()
	connectClient := connect.NewClient(
		envdAPIURL+"/"+ProcessServiceName,
		codec,
		headers,
		requestTimeout,
	)

	// Override HTTP client if provided
	if httpClient != nil {
		connectClient = connect.NewClientWithHTTPClient(
			envdAPIURL+"/"+ProcessServiceName,
			codec,
			headers,
			httpClient,
		)
	}

	// Call server stream
	var timeoutPtr *time.Duration
	if timeout > 0 {
		timeoutPtr = &timeout
	}
	msgChan, errChan := connectClient.CallServerStream(ctx, "Start", startReq, timeoutPtr)

	// Wait for first message (StartEvent) to get PID
	select {
	case msg, ok := <-msgChan:
		if !ok {
			// Channel closed, check error
			select {
			case err := <-errChan:
				return nil, err
			default:
				return nil, agentbox.NewSandboxException("stream closed without start event", nil)
			}
		}

		// Parse StartResponse
		var startResp connect.StartResponse
		msgJSON, err := json.Marshal(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal start response: %w", err)
		}
		if err := json.Unmarshal(msgJSON, &startResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal start response: %w", err)
		}

		// Get PID from StartEvent
		if startResp.Event.Start == nil {
			return nil, agentbox.NewSandboxException("start event missing PID", nil)
		}
		pid := int(startResp.Event.Start.PID)

		// Create command handle with stream
		handle := NewStreamCommandHandle(
			pid,
			envdAPIURL,
			httpClient,
			msgChan,
			errChan,
			opts,
		)

		// Start processing stream in background
		// The handle will process the stream automatically
		if streamHandle, ok := handle.(*streamCommandHandle); ok {
			go streamHandle.processStream(ctx)
		}

		return handle, nil

	case err := <-errChan:
		return nil, err

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// connectCommandStream connects to a running command stream
func connectCommandStream(
	ctx context.Context,
	envdAPIURL string,
	req map[string]interface{},
	headers map[string]string,
	timeout time.Duration,
	requestTimeout time.Duration,
	httpClient *http.Client,
	opts *agentbox.ConnectCommandOptions,
) (agentbox.CommandHandle, error) {
	// TODO: Implement server stream RPC
	return nil, agentbox.NewSandboxException(
		"Server stream RPC not yet implemented. Connect requires streaming support.",
		nil,
	)
}
