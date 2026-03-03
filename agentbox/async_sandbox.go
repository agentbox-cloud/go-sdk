package agentbox

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// AsyncSandbox represents an asynchronous sandbox instance
type AsyncSandbox struct {
	sandboxID        string
	envdVersion      string
	envdAccessToken  string
	envdAPIURL       string
	connectionConfig *ConnectionConfig
	filesystem       Filesystem
	commands         Commands
	adbShell         ADBShell
	pty              Pty
	sandboxApi       SandboxApi
}

// asyncSandboxCreate is the internal implementation behind AsyncSandboxCreate.
func asyncSandboxCreate(ctx context.Context, opts *SandboxOptions) (*AsyncSandbox, error) {
	if opts == nil {
		opts = &SandboxOptions{}
	}

	config, sandboxApi, err := newSandboxInfra(opts)
	if err != nil {
		return nil, err
	}

	sandbox := &AsyncSandbox{
		connectionConfig: config,
		sandboxApi:       sandboxApi,
	}

	sandbox.sandboxID, sandbox.envdVersion, sandbox.envdAccessToken, err = resolveSandboxSession(ctx, opts, sandbox.sandboxApi, config)
	if err != nil {
		return nil, err
	}

	// Build envd API URL
	sandbox.envdAPIURL = buildEnvdAPIURL(config, sandbox.sandboxID)

	// Initialize filesystem and commands
	sandbox.filesystem = NewFilesystem(sandbox.envdAPIURL, sandbox.envdVersion, config)
	sandbox.commands = NewCommands(sandbox.envdAPIURL, config)

	// Initialize ADB shell (only for Android sandboxes)
	// ADB shell will be initialized lazily when Connect() is called
	sandbox.adbShell = NewADBShell(sandbox.sandboxID, config)

	// TODO: Initialize PTY if needed

	return sandbox, nil
}

// AsyncSandboxConnect connects to an existing sandbox (async version, class method)
// This matches the Python SDK's AsyncSandbox.connect() class method:
// AsyncSandbox.connect(sandbox_id, timeout=None, api_key=None, domain=None,
//
//	debug=None, request_timeout=None, proxy=None)
//
// Usage:
//
//	sandbox, err := AsyncSandboxConnect(ctx, sandboxID, nil, &apiKey, &domain, nil, nil, nil)
func AsyncSandboxConnect(
	ctx context.Context,
	sandboxID string,
	timeout *int,
	apiKey *string,
	domain *string,
	debug *bool,
	requestTimeout *time.Duration,
	proxy ProxyTypes,
) (*AsyncSandbox, error) {
	opts := &SandboxOptions{
		SandboxID:      sandboxID,
		Timeout:        getIntValue(timeout, 0),
		APIKey:         getStringValue(apiKey, ""),
		Domain:         getStringValue(domain, ""),
		Debug:          getBoolValue(debug, false),
		RequestTimeout: getDurationValue(requestTimeout, 0),
		Proxy:          proxy,
	}

	return asyncSandboxCreate(ctx, opts)
}

// AsyncSandboxList lists all running sandboxes (async version, class method)
// This matches the Python SDK's AsyncSandbox.list() class method:
// AsyncSandbox.list(api_key=None, query=None, domain=None, debug=None,
//
//	request_timeout=None, headers=None, proxy=None)
//
// Usage:
//
//	sandboxes, err := AsyncSandboxList(ctx, nil, &apiKey, &domain, nil, nil, nil, nil)
func AsyncSandboxList(
	ctx context.Context,
	query *SandboxQuery,
	apiKey *string,
	domain *string,
	debug *bool,
	requestTimeout *time.Duration,
	headers map[string]string,
	proxy ProxyTypes,
) ([]*ListedSandbox, error) {
	config := NewConnectionConfig(&ConnectionConfigOptions{
		APIKey:         getStringValue(apiKey, ""),
		Domain:         getStringValue(domain, ""),
		Debug:          debug,
		RequestTimeout: getDurationValue(requestTimeout, 0),
		Headers:        headers,
		Proxy:          proxy,
	})

	sandboxApi, err := NewSandboxApi(config)
	if err != nil {
		return nil, err
	}

	return sandboxApi.List(ctx, query)
}

// AsyncSandboxResume resumes a paused sandbox (async version, class method)
// This matches the Python SDK's AsyncSandbox.resume() class method:
// AsyncSandbox.resume(sandbox_id, timeout=None, api_key=None, domain=None,
//
//	debug=None, request_timeout=None)
//
// Usage:
//
//	sandbox, err := AsyncSandboxResume(ctx, sandboxID, &timeout, &apiKey, &domain, nil, nil)
func AsyncSandboxResume(
	ctx context.Context,
	sandboxID string,
	timeout *int,
	apiKey *string,
	domain *string,
	debug *bool,
	requestTimeout *time.Duration,
) (*AsyncSandbox, error) {
	config := NewConnectionConfig(&ConnectionConfigOptions{
		APIKey:         getStringValue(apiKey, ""),
		Domain:         getStringValue(domain, ""),
		Debug:          debug,
		RequestTimeout: getDurationValue(requestTimeout, 0),
	})

	sandboxApi, err := NewSandboxApi(config)
	if err != nil {
		return nil, err
	}

	resumeTimeout := getIntValue(timeout, DefaultSandboxTimeout)
	info, err := sandboxApi.Resume(ctx, sandboxID, &resumeTimeout)
	if err != nil {
		return nil, err
	}

	// Create sandbox instance with the resumed sandbox info
	opts := &SandboxOptions{
		SandboxID:      info.SandboxID,
		APIKey:         getStringValue(apiKey, ""),
		Domain:         getStringValue(domain, ""),
		Debug:          getBoolValue(debug, false),
		RequestTimeout: getDurationValue(requestTimeout, 0),
	}

	return asyncSandboxCreate(ctx, opts)
}

// AsyncSandboxCreate creates a new async sandbox instance (class method).
// Python parity: AsyncSandbox.create(template=None, timeout=None, metadata=None, envs=None, api_key=None, domain=None, debug=None, request_timeout=None, proxy=None, secure=None, auto_pause=False)
func AsyncSandboxCreate(ctx context.Context, opts *SandboxOptions) (*AsyncSandbox, error) {
	return asyncSandboxCreate(ctx, opts)
}

// SandboxID returns the unique identifier of the sandbox
func (s *AsyncSandbox) SandboxID() string {
	return s.sandboxID
}

// Files returns the filesystem module
func (s *AsyncSandbox) Files() Filesystem {
	return s.filesystem
}

// Commands returns the commands module
func (s *AsyncSandbox) Commands() Commands {
	return s.commands
}

// ADBShell returns the ADB shell module
func (s *AsyncSandbox) ADBShell() ADBShell {
	return s.adbShell
}

// PTY returns the PTY module
func (s *AsyncSandbox) PTY() Pty {
	return s.pty
}

// ConnectionConfig returns the connection configuration
func (s *AsyncSandbox) ConnectionConfig() *ConnectionConfig {
	return s.connectionConfig
}

// GetHost returns the host address to connect to the sandbox
func (s *AsyncSandbox) GetHost(port int) string {
	if s.connectionConfig.Debug {
		return fmt.Sprintf("localhost:%d", port)
	}
	return fmt.Sprintf("%d-%s.%s", port, s.sandboxID, s.connectionConfig.Domain)
}

// IsRunning checks if the sandbox is running
func (s *AsyncSandbox) IsRunning(ctx context.Context, requestTimeout time.Duration) (bool, error) {
	// Create HTTP client for health check
	client := &http.Client{
		Timeout: s.connectionConfig.GetRequestTimeout(requestTimeout),
	}

	healthURL := s.envdAPIURL + "/health"
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return false, err
	}

	// Add auth header if available
	if s.envdAccessToken != "" {
		req.Header.Set("X-Access-Token", s.envdAccessToken)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 502 {
		return false, nil
	}

	if resp.StatusCode >= 400 {
		return false, HandleEnvdAPIException(resp)
	}

	return true, nil
}

// Kill kills the sandbox
func (s *AsyncSandbox) Kill(ctx context.Context, requestTimeout time.Duration) (bool, error) {
	return s.sandboxApi.Kill(ctx, s.sandboxID)
}

// SetTimeout sets the timeout for the sandbox
func (s *AsyncSandbox) SetTimeout(ctx context.Context, timeout int) error {
	return s.sandboxApi.SetTimeout(ctx, s.sandboxID, timeout)
}

// Pause pauses the sandbox
func (s *AsyncSandbox) Pause(ctx context.Context) error {
	return s.sandboxApi.Pause(ctx, s.sandboxID)
}

// Resume resumes a paused sandbox
func (s *AsyncSandbox) Resume(ctx context.Context, timeout *int) (*SandboxInfo, error) {
	return s.sandboxApi.Resume(ctx, s.sandboxID, timeout)
}

// DownloadURL returns the URL to download a file from the sandbox
func (s *AsyncSandbox) DownloadURL(path string, user Username, useSignature bool, signatureExpiration *int) string {
	return s.fileURL(path, user, OperationRead, useSignature, signatureExpiration)
}

// UploadURL returns the URL to upload a file to the sandbox
func (s *AsyncSandbox) UploadURL(path string, user Username, useSignature bool, signatureExpiration *int) string {
	return s.fileURL(path, user, OperationWrite, useSignature, signatureExpiration)
}

// fileURL builds a file URL with optional signature
func (s *AsyncSandbox) fileURL(path string, user Username, operation Operation, useSignature bool, signatureExpiration *int) string {
	fileURL := s.envdAPIURL + "/files"

	params := make(map[string]string)
	if path != "" {
		params["path"] = path
	}
	params["username"] = string(user)

	if useSignature && s.envdAccessToken != "" {
		var exp *int
		if signatureExpiration != nil {
			exp = signatureExpiration
		}
		sig, err := GetSignature(path, operation, string(user), s.envdAccessToken, exp)
		if err == nil {
			params["signature"] = sig.Signature
			if sig.Expiration != nil {
				params["signature_expiration"] = fmt.Sprintf("%d", *sig.Expiration)
			}
		}
	}

	// Build query string
	query := ""
	first := true
	for k, v := range params {
		if first {
			query += "?"
			first = false
		} else {
			query += "&"
		}
		query += fmt.Sprintf("%s=%s", k, v)
	}

	return fileURL + query
}
