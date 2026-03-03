package agentbox

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const (
	// EnvdPort is the default port for envd API
	EnvdPort = 49983
)

// SandboxOptions are options for creating a sandbox
// This matches the Python SDK's Sandbox() constructor parameters:
// template, timeout, metadata, envs, secure, api_key, domain, debug,
// sandbox_id, request_timeout, proxy
type SandboxOptions struct {
	Template       string            // Sandbox template name or ID
	Timeout        int               // Timeout for the sandbox in seconds
	Metadata       map[string]string // Custom metadata for the sandbox
	Envs           map[string]string // Custom environment variables (envs in Python SDK)
	Secure         bool              // Secure all system communication with sandbox
	APIKey         string            // API key (api_key in Python SDK)
	Domain         string            // Domain for the sandbox
	Debug          bool              // Enable debug mode
	SandboxID      string            // For connecting to existing sandbox (sandbox_id in Python SDK)
	RequestTimeout time.Duration     // Timeout for requests (request_timeout in Python SDK)
	Proxy          ProxyTypes        // Proxy configuration
}

// Sandbox represents a synchronous sandbox instance
// This matches the Python SDK's Sandbox class
type Sandbox struct {
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

// newSandbox is the internal implementation for creating a sandbox
func newSandbox(ctx context.Context, opts *SandboxOptions) (*Sandbox, error) {
	if opts == nil {
		opts = &SandboxOptions{}
	}

	// Validate: Cannot set metadata or template when connecting to existing sandbox
	if opts.SandboxID != "" && (opts.Metadata != nil || opts.Template != "") {
		return nil, fmt.Errorf("cannot set metadata or template when connecting to existing sandbox. Use Sandbox.Connect method instead")
	}

	config, sandboxApi, err := newSandboxInfra(opts)
	if err != nil {
		return nil, err
	}

	sandbox := &Sandbox{
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

// SandboxConnect connects to an existing sandbox (class method)
// This matches the Python SDK's Sandbox.connect() class method:
// Sandbox.connect(sandbox_id, timeout=None, api_key=None, domain=None,
//
//	debug=None, request_timeout=None, proxy=None)
//
// Usage:
//
//	sandbox, err := SandboxConnect(ctx, sandboxID, nil, &apiKey, &domain, nil, nil, nil)
func SandboxConnect(
	ctx context.Context,
	sandboxID string,
	timeout *int,
	apiKey *string,
	domain *string,
	debug *bool,
	requestTimeout *time.Duration,
	proxy ProxyTypes,
) (*Sandbox, error) {
	opts := &SandboxOptions{
		SandboxID:      sandboxID,
		Timeout:        getIntValue(timeout, 0),
		APIKey:         getStringValue(apiKey, ""),
		Domain:         getStringValue(domain, ""),
		Debug:          getBoolValue(debug, false),
		RequestTimeout: getDurationValue(requestTimeout, 0),
		Proxy:          proxy,
	}
	return newSandbox(ctx, opts)
}

// SandboxList lists all running sandboxes (class method)
// This matches the Python SDK's Sandbox.list() class method:
// Sandbox.list(api_key=None, query=None, domain=None, debug=None,
//
//	request_timeout=None, headers=None, proxy=None)
//
// Usage:
//
//	sandboxes, err := SandboxList(ctx, nil, &apiKey, &domain, nil, nil, nil, nil)
func SandboxList(
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

// SandboxResume resumes a paused sandbox (class method)
// This matches the Python SDK's Sandbox.resume() class method:
// Sandbox.resume(sandbox_id, timeout=None, api_key=None, domain=None,
//
//	debug=None, request_timeout=None)
//
// Usage:
//
//	sandbox, err := SandboxResume(ctx, sandboxID, &timeout, &apiKey, &domain, nil, nil)
func SandboxResume(
	ctx context.Context,
	sandboxID string,
	timeout *int,
	apiKey *string,
	domain *string,
	debug *bool,
	requestTimeout *time.Duration,
) (*Sandbox, error) {
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

	return newSandbox(ctx, opts)
}

// NewSandbox creates a new sandbox instance
// This matches the Python SDK's Sandbox() constructor signature:
// Sandbox(template=None, timeout=None, metadata=None, envs=None, secure=None,
//
//	api_key=None, domain=None, debug=None, sandbox_id=None,
//	request_timeout=None, proxy=None)
//
// Usage (equivalent to Python's Sandbox()):
//
//	sandbox, err := NewSandbox(ctx, &SandboxOptions{
//	    Template: "base",
//	    Timeout:  300,
//	    APIKey:   "ab_...",
//	    Domain:   "agentbox.net.cn",
//	})
func NewSandbox(ctx context.Context, opts *SandboxOptions) (*Sandbox, error) {
	return newSandbox(ctx, opts)
}

// Helper functions for optional parameters
func getStringValue(ptr *string, defaultValue string) string {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

func getIntValue(ptr *int, defaultValue int) int {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

func getBoolValue(ptr *bool, defaultValue bool) bool {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

func getDurationValue(ptr *time.Duration, defaultValue time.Duration) time.Duration {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

// SandboxID returns the unique identifier of the sandbox
func (s *Sandbox) SandboxID() string {
	return s.sandboxID
}

// Files returns the filesystem module
func (s *Sandbox) Files() Filesystem {
	return s.filesystem
}

// Commands returns the commands module
func (s *Sandbox) Commands() Commands {
	return s.commands
}

// ADBShell returns the ADB shell module
func (s *Sandbox) ADBShell() ADBShell {
	return s.adbShell
}

// PTY returns the PTY module
func (s *Sandbox) PTY() Pty {
	return s.pty
}

// ConnectionConfig returns the connection configuration
func (s *Sandbox) ConnectionConfig() *ConnectionConfig {
	return s.connectionConfig
}

// GetHost returns the host address to connect to the sandbox
func (s *Sandbox) GetHost(port int) string {
	if s.connectionConfig.Debug {
		return fmt.Sprintf("localhost:%d", port)
	}
	return fmt.Sprintf("%d-%s.%s", port, s.sandboxID, s.connectionConfig.Domain)
}

// IsRunning checks if the sandbox is running
func (s *Sandbox) IsRunning(ctx context.Context, requestTimeout time.Duration) (bool, error) {
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
func (s *Sandbox) Kill(ctx context.Context, requestTimeout time.Duration) (bool, error) {
	return s.sandboxApi.Kill(ctx, s.sandboxID)
}

// SetTimeout sets the timeout for the sandbox
func (s *Sandbox) SetTimeout(ctx context.Context, timeout int) error {
	return s.sandboxApi.SetTimeout(ctx, s.sandboxID, timeout)
}

// Pause pauses the sandbox
func (s *Sandbox) Pause(ctx context.Context) error {
	return s.sandboxApi.Pause(ctx, s.sandboxID)
}

// Resume resumes a paused sandbox
func (s *Sandbox) Resume(ctx context.Context, timeout *int) (*SandboxInfo, error) {
	return s.sandboxApi.Resume(ctx, s.sandboxID, timeout)
}

// DownloadURL returns the URL to download a file from the sandbox
func (s *Sandbox) DownloadURL(path string, user Username, useSignature bool, signatureExpiration *int) string {
	return s.fileURL(path, user, OperationRead, useSignature, signatureExpiration)
}

// UploadURL returns the URL to upload a file to the sandbox
func (s *Sandbox) UploadURL(path string, user Username, useSignature bool, signatureExpiration *int) string {
	return s.fileURL(path, user, OperationWrite, useSignature, signatureExpiration)
}

// fileURL builds a file URL with optional signature
func (s *Sandbox) fileURL(path string, user Username, operation Operation, useSignature bool, signatureExpiration *int) string {
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

// ADBShell interface is defined in adb_shell.go

// Pty interface (placeholder)
type Pty interface {
	// PTY methods
}
