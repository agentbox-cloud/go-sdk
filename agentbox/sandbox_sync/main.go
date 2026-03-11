package sandbox_sync

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/agentbox-cloud/go-sdk/agentbox"
	"github.com/agentbox-cloud/go-sdk/agentbox/sandbox_sync/adb_shell"
	"github.com/agentbox-cloud/go-sdk/agentbox/sandbox_sync/commands"
	"github.com/agentbox-cloud/go-sdk/agentbox/sandbox_sync/filesystem"
)

// NewADBShellImpl creates a new ADB shell implementation
// This is a helper function to create ADB shell from this package
func NewADBShellImpl(config *agentbox.ConnectionConfig, sandboxID string, host string, port int, rsaKeyPath string, authTimeoutS float64) agentbox.ADBShell {
	return adb_shell.NewADBShell(config, sandboxID, host, port, rsaKeyPath, authTimeoutS)
}

const (
	// DefaultTemplate is the default sandbox template
	DefaultTemplate = "base"

	// DefaultSandboxTimeout is the default timeout for sandbox (300 seconds)
	DefaultSandboxTimeout = 300

	// DefaultConnectTimeout is the default timeout for reconnecting to paused sandbox (3600 seconds)
	DefaultConnectTimeout = 3600

	// DefaultRequestTimeout is the default timeout for API requests (30 seconds)
	DefaultRequestTimeout = 30

	// EnvdPort is the default port for envd API
	EnvdPort = 49983
)

// Sandbox represents a synchronous sandbox instance
// This matches the Python SDK's Sandbox class in sandbox_sync/main.py
type Sandbox struct {
	sandboxID        string
	envdVersion      string
	envdAccessToken  string
	envdAPIURL       string
	connectionConfig *agentbox.ConnectionConfig
	filesystem       agentbox.Filesystem
	commands         agentbox.Commands
	adbShell         agentbox.ADBShell
	pty              agentbox.Pty
	sandboxApi       agentbox.SandboxApi
}

// NewSandbox creates a new sandbox instance
// This matches the Python SDK's Sandbox() constructor:
// Sandbox(template=None, timeout=None, metadata=None, envs=None, secure=None,
//
//	api_key=None, domain=None, debug=None, sandbox_id=None,
//	request_timeout=None, proxy=None, auto_pause=False)
func NewSandbox(ctx context.Context, opts *agentbox.SandboxOptions) (*Sandbox, error) {
	if opts == nil {
		opts = &agentbox.SandboxOptions{}
	}

	// Validate: Cannot set metadata or template when connecting to existing sandbox
	if opts.SandboxID != "" && (opts.Metadata != nil || opts.Template != "") {
		return nil, agentbox.NewSandboxException(
			"Cannot set metadata or template when connecting to an existing sandbox. Use Sandbox.Connect method instead.",
			nil,
		)
	}

	config, sandboxApi, err := newSandboxInfra(opts)
	if err != nil {
		return nil, err
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = DefaultRequestTimeout * time.Second
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

	// Initialize filesystem and commands based on sandbox type
	if isAndroidSandbox(sandbox.sandboxID) {
		// Android sandbox uses SSH-based filesystem and commands
		// This will be implemented separately
		sandbox.filesystem = nil // TODO: Initialize SSH filesystem
		sandbox.commands = nil   // TODO: Initialize SSH commands
	} else {
		// Regular sandbox uses envd API
		sandbox.filesystem = filesystem.NewFilesystem(sandbox.envdAPIURL, sandbox.envdVersion, config)
		sandbox.commands = commands.NewCommands(sandbox.envdAPIURL, config)
	}

	// Initialize ADB shell (only for Android sandboxes)
	// Use the implementation from sandbox_sync/adb_shell package
	sandbox.adbShell = NewADBShellImpl(config, sandbox.sandboxID, "", 0, "", 3.0)

	// TODO: Initialize PTY if needed

	return sandbox, nil
}

// SandboxID returns the unique identifier of the sandbox
func (s *Sandbox) SandboxID() string {
	return s.sandboxID
}

// Files returns the filesystem module
func (s *Sandbox) Files() agentbox.Filesystem {
	return s.filesystem
}

// Commands returns the commands module
func (s *Sandbox) Commands() agentbox.Commands {
	return s.commands
}

// ADBShell returns the ADB shell module
func (s *Sandbox) ADBShell() agentbox.ADBShell {
	return s.adbShell
}

// PTY returns the PTY module
func (s *Sandbox) PTY() agentbox.Pty {
	return s.pty
}

// ConnectionConfig returns the connection configuration
func (s *Sandbox) ConnectionConfig() *agentbox.ConnectionConfig {
	return s.connectionConfig
}

// IsRunning checks if the sandbox is running
// This matches Python SDK's sandbox.is_running()
func (s *Sandbox) IsRunning(ctx context.Context, requestTimeout time.Duration) (bool, error) {
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
		return false, agentbox.HandleEnvdAPIException(resp)
	}

	return true, nil
}

// Kill kills the sandbox
// This matches Python SDK's sandbox.kill()
func (s *Sandbox) Kill(ctx context.Context, requestTimeout time.Duration) (bool, error) {
	return s.sandboxApi.Kill(ctx, s.sandboxID)
}

// SetTimeout sets the timeout for the sandbox
// This matches Python SDK's sandbox.set_timeout()
func (s *Sandbox) SetTimeout(ctx context.Context, timeout int) error {
	return s.sandboxApi.SetTimeout(ctx, s.sandboxID, timeout)
}

// Pause pauses the sandbox
// This matches Python SDK's sandbox.pause()
func (s *Sandbox) Pause(ctx context.Context) error {
	return s.sandboxApi.Pause(ctx, s.sandboxID)
}

// Resume resumes a paused sandbox (instance method)
// This matches Python SDK's sandbox.resume()
// Returns a new Sandbox instance, matching Python SDK behavior
func (s *Sandbox) Resume(ctx context.Context, timeout *int) (*Sandbox, error) {
	// Get configuration from current instance
	apiKey := s.connectionConfig.APIKey
	domain := s.connectionConfig.Domain
	debug := s.connectionConfig.Debug
	requestTimeout := s.connectionConfig.RequestTimeout

	// Call package-level Resume function to resume and create new instance
	return Resume(
		ctx,
		s.sandboxID,
		timeout,
		&apiKey,
		&domain,
		&debug,
		&requestTimeout,
	)
}

// GetInfo gets information about the sandbox
// This matches Python SDK's sandbox.get_info()
func (s *Sandbox) GetInfo(ctx context.Context) (*agentbox.SandboxInfo, error) {
	return s.sandboxApi.GetInfo(ctx, s.sandboxID)
}

// Connect connects to an existing sandbox (instance method)
// This matches the Python SDK's sandbox.connect() method
// If the sandbox is paused, it will be automatically resumed.
func (s *Sandbox) Connect(ctx context.Context, timeout *int) error {
	info, err := s.sandboxApi.Connect(ctx, s.sandboxID, timeout)
	if err != nil {
		return err
	}

	// Update sandbox info
	s.sandboxID = info.SandboxID
	s.envdVersion = info.EnvdVersion
	s.envdAccessToken = info.EnvdAccessToken

	return nil
}

// Connect connects to an existing sandbox (class method)
// This matches the Python SDK's Sandbox.connect() class method:
// Sandbox.connect(sandbox_id, timeout=None, api_key=None, domain=None,
//
//	debug=None, request_timeout=None, proxy=None)
func Connect(
	ctx context.Context,
	sandboxID string,
	timeout *int,
	apiKey *string,
	domain *string,
	debug *bool,
	requestTimeout *time.Duration,
	proxy agentbox.ProxyTypes,
) (*Sandbox, error) {
	opts := &agentbox.SandboxOptions{
		SandboxID:      sandboxID,
		Timeout:        getIntValue(timeout, 0),
		APIKey:         getStringValue(apiKey, ""),
		Domain:         getStringValue(domain, ""),
		Debug:          getBoolValue(debug, false),
		RequestTimeout: getDurationValue(requestTimeout, 0),
		Proxy:          proxy,
	}
	return NewSandbox(ctx, opts)
}

// Kill kills a sandbox by ID (class method)
// This matches the Python SDK's Sandbox.kill() class method
func Kill(
	ctx context.Context,
	sandboxID string,
	apiKey *string,
	domain *string,
	debug *bool,
	requestTimeout *time.Duration,
	proxy agentbox.ProxyTypes,
) (bool, error) {
	config := agentbox.NewConnectionConfig(&agentbox.ConnectionConfigOptions{
		APIKey:         getStringValue(apiKey, ""),
		Domain:         getStringValue(domain, ""),
		Debug:          debug,
		RequestTimeout: getDurationValue(requestTimeout, 0),
		Proxy:          proxy,
	})

	sandboxApi, err := agentbox.NewSandboxApi(config)
	if err != nil {
		return false, err
	}

	return sandboxApi.Kill(ctx, sandboxID)
}

// SetTimeout sets the timeout for a sandbox by ID (class method)
// This matches the Python SDK's Sandbox.set_timeout() class method
func SetTimeout(
	ctx context.Context,
	sandboxID string,
	timeout int,
	apiKey *string,
	domain *string,
	debug *bool,
	requestTimeout *time.Duration,
	proxy agentbox.ProxyTypes,
) error {
	config := agentbox.NewConnectionConfig(&agentbox.ConnectionConfigOptions{
		APIKey:         getStringValue(apiKey, ""),
		Domain:         getStringValue(domain, ""),
		Debug:          debug,
		RequestTimeout: getDurationValue(requestTimeout, 0),
		Proxy:          proxy,
	})

	sandboxApi, err := agentbox.NewSandboxApi(config)
	if err != nil {
		return err
	}

	return sandboxApi.SetTimeout(ctx, sandboxID, timeout)
}

// Resume resumes a paused sandbox (class method)
// This matches the Python SDK's Sandbox.resume() class method
func Resume(
	ctx context.Context,
	sandboxID string,
	timeout *int,
	apiKey *string,
	domain *string,
	debug *bool,
	requestTimeout *time.Duration,
) (*Sandbox, error) {
	config := agentbox.NewConnectionConfig(&agentbox.ConnectionConfigOptions{
		APIKey:         getStringValue(apiKey, ""),
		Domain:         getStringValue(domain, ""),
		Debug:          debug,
		RequestTimeout: getDurationValue(requestTimeout, 0),
	})

	sandboxApi, err := agentbox.NewSandboxApi(config)
	if err != nil {
		return nil, err
	}

	resumeTimeout := getIntValue(timeout, DefaultSandboxTimeout)
	info, err := sandboxApi.Resume(ctx, sandboxID, &resumeTimeout)
	if err != nil {
		return nil, err
	}

	// Create sandbox instance with the resumed sandbox info
	opts := &agentbox.SandboxOptions{
		SandboxID:      info.SandboxID,
		APIKey:         getStringValue(apiKey, ""),
		Domain:         getStringValue(domain, ""),
		Debug:          getBoolValue(debug, false),
		RequestTimeout: getDurationValue(requestTimeout, 0),
	}

	return NewSandbox(ctx, opts)
}

// Helper functions

func newSandboxInfra(opts *agentbox.SandboxOptions) (*agentbox.ConnectionConfig, agentbox.SandboxApi, error) {
	debug := opts.Debug
	config := agentbox.NewConnectionConfig(&agentbox.ConnectionConfigOptions{
		APIKey:         opts.APIKey,
		Domain:         opts.Domain,
		Debug:          &debug,
		RequestTimeout: opts.RequestTimeout,
		Proxy:          opts.Proxy,
	})

	sandboxApi, err := agentbox.NewSandboxApi(config)
	if err != nil {
		return nil, nil, err
	}
	return config, sandboxApi, nil
}

func resolveSandboxSession(
	ctx context.Context,
	opts *agentbox.SandboxOptions,
	sandboxApi agentbox.SandboxApi,
	config *agentbox.ConnectionConfig,
) (sandboxID string, envdVersion string, envdAccessToken string, err error) {
	if config.Debug {
		return "debug_sandbox_id", "", "", nil
	}

	if opts.SandboxID != "" {
		// When connecting to existing sandbox, use Connect API instead of GetInfo
		// This matches Python SDK's _cls_connect() behavior:
		// - If sandbox is paused, it will be automatically resumed
		// - TTL (timeout) is extended if provided
		// - For "brd" sandboxes, skip the Connect API call (handled in Python SDK)
		sandboxIDLower := strings.ToLower(opts.SandboxID)
		if strings.Contains(sandboxIDLower, "brd") {
			// For "brd" sandboxes, just get info without calling Connect API
			// This matches Python SDK's behavior
			info, err := sandboxApi.GetInfo(ctx, opts.SandboxID)
			if err != nil {
				return "", "", "", err
			}
			sandboxID = info.SandboxID
			envdVersion = info.EnvdVersion
			envdAccessToken = info.EnvdAccessToken
		} else {
			// For regular sandboxes, call Connect API with timeout
			// Use default connect timeout if not specified
			timeout := opts.Timeout
			if timeout == 0 {
				timeout = DefaultConnectTimeout
			}
			var timeoutPtr *int
			if timeout > 0 {
				timeoutPtr = &timeout
			}
			info, err := sandboxApi.Connect(ctx, opts.SandboxID, timeoutPtr)
			if err != nil {
				return "", "", "", err
			}
			sandboxID = info.SandboxID
			envdVersion = info.EnvdVersion
			envdAccessToken = info.EnvdAccessToken
		}
	} else {
		template := opts.Template
		if template == "" {
			template = DefaultTemplate
		}
		timeout := opts.Timeout
		if timeout == 0 {
			timeout = DefaultSandboxTimeout
		}

		info, err := sandboxApi.Create(ctx, &agentbox.CreateSandboxOptions{
			Template:       template,
			Timeout:        timeout,
			Metadata:       opts.Metadata,
			Envs:           opts.Envs,
			Secure:         opts.Secure,
			AutoPause:      opts.AutoPause,
			APIKey:         opts.APIKey,
			Domain:         opts.Domain,
			Debug:          opts.Debug,
			RequestTimeout: opts.RequestTimeout,
			Proxy:          opts.Proxy,
		})
		if err != nil {
			return "", "", "", err
		}
		sandboxID = info.SandboxID
		envdVersion = info.EnvdVersion
		envdAccessToken = info.EnvdAccessToken
	}

	// Preserve existing headers and append access token when available
	headers := make(map[string]string)
	for k, v := range config.Headers {
		headers[k] = v
	}
	if envdAccessToken != "" {
		headers["X-Access-Token"] = envdAccessToken
	}
	config.Headers = headers

	return sandboxID, envdVersion, envdAccessToken, nil
}

func buildEnvdAPIURL(config *agentbox.ConnectionConfig, sandboxID string) string {
	if config.Debug {
		return fmt.Sprintf("http://localhost:%d", EnvdPort)
	}
	return fmt.Sprintf("https://%d-%s.%s", EnvdPort, sandboxID, config.Domain)
}

func isAndroidSandbox(sandboxID string) bool {
	// Check if sandbox ID contains "brd" (Android sandbox indicator)
	// This matches Python SDK behavior
	return strings.Contains(strings.ToLower(sandboxID), "brd")
}

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
