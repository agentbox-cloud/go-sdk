package agentbox

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	envdconnect "github.com/agentbox-cloud/go-sdk/agentbox/envd/connect"
)

// commandsImpl implements Commands
type commandsImpl struct {
	envdAPIURL       string
	connectionConfig *ConnectionConfig
}

func mergeHeaders(base map[string]string, extra map[string]string) map[string]string {
	headers := make(map[string]string)
	for k, v := range base {
		headers[k] = v
	}
	for k, v := range extra {
		headers[k] = v
	}
	return headers
}

func parsePID(v interface{}) (int, bool) {
	switch val := v.(type) {
	case float64:
		return int(val), true
	case int:
		return val, true
	case int32:
		return int(val), true
	case int64:
		return int(val), true
	default:
		return 0, false
	}
}

// NewCommands creates a new Commands instance
func NewCommands(envdAPIURL string, config *ConnectionConfig) Commands {
	return &commandsImpl{
		envdAPIURL:       envdAPIURL,
		connectionConfig: config,
	}
}

// List lists all running commands and PTY sessions
func (c *commandsImpl) List(ctx context.Context, requestTimeout time.Duration) ([]*ProcessInfo, error) {
	timeout := c.connectionConfig.GetRequestTimeout(requestTimeout)
	client := envdconnect.NewClient(
		c.envdAPIURL+"/process.Process",
		&envdconnect.JSONCodec{},
		c.connectionConfig.Headers,
		timeout,
	)

	resp, err := client.CallUnary(ctx, "List", map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	respMap, ok := resp.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid list response type: %T", resp)
	}

	processesRaw, ok := respMap["processes"].([]interface{})
	if !ok {
		return []*ProcessInfo{}, nil
	}

	processes := make([]*ProcessInfo, 0, len(processesRaw))
	for _, p := range processesRaw {
		pm, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		pid, _ := parsePID(pm["pid"])
		info := &ProcessInfo{
			PID:  pid,
			Tag:  getString(pm, "tag"),
			Envs: map[string]string{},
		}

		if cfg, ok := pm["config"].(map[string]interface{}); ok {
			info.Cmd = getString(cfg, "cmd")
			info.Cwd = getString(cfg, "cwd")
			if args, ok := cfg["args"].([]interface{}); ok {
				info.Args = make([]string, 0, len(args))
				for _, a := range args {
					if s, ok := a.(string); ok {
						info.Args = append(info.Args, s)
					}
				}
			}
			if envs, ok := cfg["envs"].(map[string]interface{}); ok {
				for k, v := range envs {
					if s, ok := v.(string); ok {
						info.Envs[k] = s
					}
				}
			}
		}

		processes = append(processes, info)
	}

	return processes, nil
}

// Kill kills a running command specified by its process ID
func (c *commandsImpl) Kill(ctx context.Context, pid int, requestTimeout time.Duration) (bool, error) {
	timeout := c.connectionConfig.GetRequestTimeout(requestTimeout)
	client := envdconnect.NewClient(
		c.envdAPIURL+"/process.Process",
		&envdconnect.JSONCodec{},
		c.connectionConfig.Headers,
		timeout,
	)

	req := map[string]interface{}{
		"process": map[string]interface{}{"pid": pid},
		"signal":  "SIGNAL_SIGKILL",
	}

	_, err := client.CallUnary(ctx, "SendSignal", req)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not_found") {
			return false, nil
		}
		// Fallback for services expecting enum number instead of enum name.
		req["signal"] = 9
		if _, retryErr := client.CallUnary(ctx, "SendSignal", req); retryErr != nil {
			if strings.Contains(strings.ToLower(retryErr.Error()), "not_found") {
				return false, nil
			}
			return false, retryErr
		}
	}

	return true, nil
}

// SendStdin sends data to command stdin
func (c *commandsImpl) SendStdin(ctx context.Context, pid int, data string, requestTimeout time.Duration) error {
	timeout := c.connectionConfig.GetRequestTimeout(requestTimeout)
	client := envdconnect.NewClient(
		c.envdAPIURL+"/process.Process",
		&envdconnect.JSONCodec{},
		c.connectionConfig.Headers,
		timeout,
	)

	req := map[string]interface{}{
		"process": map[string]interface{}{"pid": pid},
		"input": map[string]interface{}{
			"stdin": base64.StdEncoding.EncodeToString([]byte(data)),
		},
	}
	_, err := client.CallUnary(ctx, "SendInput", req)
	return err
}

// Run runs a command and waits for it to finish (foreground execution)
func (c *commandsImpl) Run(ctx context.Context, cmd string, opts *RunCommandOptions) (*CommandResult, error) {
	// Start the command in background mode
	handle, err := c.RunBackground(ctx, cmd, opts)
	if err != nil {
		return nil, err
	}
	
	// Wait for it to finish
	return handle.Wait(ctx)
}

// RunBackground runs a command in the background and returns a handle
func (c *commandsImpl) RunBackground(ctx context.Context, cmd string, opts *RunCommandOptions) (CommandHandle, error) {
	if opts == nil {
		opts = &RunCommandOptions{}
	}
	
	// Build process config
	processConfig := envdconnect.ProcessConfig{
		Cmd:  "/bin/bash",
		Args: []string{"-l", "-c", cmd},
		Envs: opts.Envs,
	}
	if opts.Cwd != "" {
		processConfig.Cwd = &opts.Cwd
	}
	
	// Build start request
	startReq := envdconnect.StartRequest{
		Process: processConfig,
	}
	
	// Build headers with authentication and keepalive (matching Python SDK behavior)
	headers := mergeHeaders(c.connectionConfig.Headers, map[string]string{
		KeepalivePingHeader: strconv.Itoa(KeepalivePingIntervalSec),
	})
	user := string(opts.User)
	if user == "" {
		user = string(UsernameUser)
	}
	// Python SDK uses Basic auth: base64("user:")
	// Add Authorization header
	authValue := user + ":"
	authEncoded := base64.StdEncoding.EncodeToString([]byte(authValue))
	headers["Authorization"] = "Basic " + authEncoded
	
	// Add timeout if specified
	var timeout *time.Duration
	if opts.Timeout > 0 {
		timeout = &opts.Timeout
	}
	
	// Get request timeout
	requestTimeout := c.connectionConfig.GetRequestTimeout(opts.RequestTimeout)
	
	// Create a new client with updated timeout
	// Connect Protocol URL format: {baseURL}/{ServiceName}/{MethodName}
	rpcClient := envdconnect.NewClient(
		c.envdAPIURL+"/process.Process",
		&envdconnect.JSONCodec{},
		headers,
		requestTimeout,
	)
	
	// Call server stream
	msgChan, errChan := rpcClient.CallServerStream(ctx, "Start", startReq, timeout)
	
	// Wait for first message (start event)
	select {
	case msg, ok := <-msgChan:
		if !ok {
			// Channel closed before receiving start event
			return nil, fmt.Errorf("stream closed before receiving start event")
		}
		
		// Parse StartResponse (msg is map[string]interface{} from JSON decoder)
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected message type: %T", msg)
		}
		
		// Extract event field
		eventData, ok := msgMap["event"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("missing event field in response")
		}
		
		// Check for start event
		startData, ok := eventData["start"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected start event, got: %+v", eventData)
		}
		
		// Extract PID
		pidVal, ok := startData["pid"].(float64)
		if !ok {
			return nil, fmt.Errorf("missing pid in start event")
		}
		pid := int(pidVal)
		
		// Create command handle
		handle := NewCommandHandle(
			pid,
			c.Kill,
			c.SendStdin,
			msgChan,
			errChan,
			opts.OnStdout,
			opts.OnStderr,
		)
		
		return handle, nil
		
	case err := <-errChan:
		return nil, fmt.Errorf("error starting process: %w", err)
		
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Connect connects to a running command
func (c *commandsImpl) Connect(ctx context.Context, pid int, opts *ConnectCommandOptions) (CommandHandle, error) {
	if opts == nil {
		opts = &ConnectCommandOptions{}
	}

	headers := mergeHeaders(c.connectionConfig.Headers, map[string]string{
		KeepalivePingHeader: strconv.Itoa(KeepalivePingIntervalSec),
	})

	requestTimeout := c.connectionConfig.GetRequestTimeout(opts.RequestTimeout)
	rpcClient := envdconnect.NewClient(
		c.envdAPIURL+"/process.Process",
		&envdconnect.JSONCodec{},
		headers,
		requestTimeout,
	)

	pidU32 := uint32(pid)
	req := envdconnect.ConnectRequest{
		Process: envdconnect.ProcessSelector{PID: &pidU32},
	}

	var timeout *time.Duration
	if opts.Timeout > 0 {
		timeout = &opts.Timeout
	}

	msgChan, errChan := rpcClient.CallServerStream(ctx, "Connect", req, timeout)

	select {
	case msg, ok := <-msgChan:
		if !ok {
			return nil, fmt.Errorf("stream closed before receiving connect event")
		}

		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected message type: %T", msg)
		}

		connectedPID := pid
		if eventData, ok := extractEventMap(msgMap); ok {
			if startData, ok := eventData["start"].(map[string]interface{}); ok {
				if parsed, ok := parsePID(startData["pid"]); ok {
					connectedPID = parsed
				}
			}
		}

		handle := NewCommandHandle(
			connectedPID,
			c.Kill,
			c.SendStdin,
			msgChan,
			errChan,
			opts.OnStdout,
			opts.OnStderr,
		)
		return handle, nil

	case err := <-errChan:
		return nil, fmt.Errorf("error connecting process: %w", err)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

