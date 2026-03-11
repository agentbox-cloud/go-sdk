package agentbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/agentbox-cloud/go-sdk/agentbox/api"
)

// SandboxApi provides methods for interacting with the AgentBox API
// This matches Python SDK's SandboxApi interface
type SandboxApi interface {
	// List lists all running sandboxes
	List(ctx context.Context, query *SandboxQuery) ([]*ListedSandbox, error)

	// GetInfo gets information about a specific sandbox
	GetInfo(ctx context.Context, sandboxID string) (*SandboxInfo, error)

	// Create creates a new sandbox
	Create(ctx context.Context, opts *CreateSandboxOptions) (*SandboxInfo, error)

	// Connect connects to an existing sandbox
	// If the sandbox is paused, it will be automatically resumed.
	// timeout: Timeout for the sandbox in seconds. For running sandboxes, the timeout will update only if the new timeout is longer than the existing one.
	Connect(ctx context.Context, sandboxID string, timeout *int) (*SandboxInfo, error)

	// Kill kills a sandbox
	Kill(ctx context.Context, sandboxID string) (bool, error)

	// SetTimeout sets the timeout for a sandbox
	SetTimeout(ctx context.Context, sandboxID string, timeout int) error

	// Pause pauses a sandbox
	Pause(ctx context.Context, sandboxID string) error

	// Resume resumes a paused sandbox
	Resume(ctx context.Context, sandboxID string, timeout *int) (*SandboxInfo, error)

	// GetADBPublicInfo gets ADB public information for a sandbox
	GetADBPublicInfo(ctx context.Context, sandboxID string) (*ADBPublicInfo, error)
}

// CreateSandboxOptions are options for creating a sandbox
type CreateSandboxOptions struct {
	Template       string
	Timeout        int
	Metadata       map[string]string
	Envs           map[string]string
	Secure         bool
	AutoPause      bool
	APIKey         string
	Domain         string
	Debug          bool
	RequestTimeout time.Duration
	Proxy          ProxyTypes
}

// ADBPublicInfo contains ADB public information
// This matches Python SDK's SandboxADBPublicInfo
type ADBPublicInfo struct {
	ADBIP      string
	ADBPort    int
	PublicKey  string
	PrivateKey string
}

// SandboxOptions are options for creating a sandbox
// This matches the Python SDK's Sandbox() constructor parameters
type SandboxOptions struct {
	Template       string            // Sandbox template name or ID
	Timeout        int               // Timeout for the sandbox in seconds
	Metadata       map[string]string // Custom metadata for the sandbox
	Envs           map[string]string // Custom environment variables (envs in Python SDK)
	Secure         bool              // Secure all system communication with sandbox
	AutoPause      bool              // Automatically pause sandbox after timeout
	APIKey         string            // API key (api_key in Python SDK)
	Domain         string            // Domain for the sandbox
	Debug          bool              // Enable debug mode
	SandboxID      string            // For connecting to existing sandbox (sandbox_id in Python SDK)
	RequestTimeout time.Duration     // Timeout for requests (request_timeout in Python SDK)
	Proxy          ProxyTypes        // Proxy configuration
}

// connectionConfigAdapter adapts ConnectionConfig to api.ConnectionConfig interface
type connectionConfigAdapter struct {
	*ConnectionConfig
}

func (a *connectionConfigAdapter) GetAPIKey() string {
	return a.APIKey
}

func (a *connectionConfigAdapter) GetAccessToken() string {
	return a.AccessToken
}

func (a *connectionConfigAdapter) GetRequestTimeout(timeout time.Duration) time.Duration {
	return a.ConnectionConfig.GetRequestTimeout(timeout)
}

func (a *connectionConfigAdapter) GetHeaders() map[string]string {
	return a.Headers
}

func (a *connectionConfigAdapter) GetAPIURL() string {
	return a.APIURL
}

// NewSandboxApi creates a new SandboxApi instance
func NewSandboxApi(config *ConnectionConfig) (SandboxApi, error) {
	// Implementation is in sandbox_api_impl.go
	return newSandboxApiImpl(config)
}

// ListSandboxes lists all running sandboxes
// This matches Python SDK's SandboxApi.list() class method
// It can be called directly without creating a SandboxApi instance
func ListSandboxes(
	ctx context.Context,
	query *SandboxQuery,
	apiKey *string,
	domain *string,
	debug *bool,
	requestTimeout *time.Duration,
	headers map[string]string,
	proxy ProxyTypes,
) ([]*ListedSandbox, error) {
	// Create connection config
	opts := &ConnectionConfigOptions{}
	if apiKey != nil {
		opts.APIKey = *apiKey
	}
	if domain != nil {
		opts.Domain = *domain
	}
	if debug != nil {
		opts.Debug = debug
	}
	if requestTimeout != nil {
		opts.RequestTimeout = *requestTimeout
	}
	if len(headers) > 0 {
		opts.Headers = headers
	}
	if proxy != nil {
		opts.Proxy = proxy
	}

	config := NewConnectionConfig(opts)

	// Create SandboxApi instance and call List
	sandboxApi, err := NewSandboxApi(config)
	if err != nil {
		return nil, err
	}

	return sandboxApi.List(ctx, query)
}

// newSandboxApiImpl is the internal function to create SandboxApi implementation
func newSandboxApiImpl(config *ConnectionConfig) (SandboxApi, error) {
	adapter := &connectionConfigAdapter{ConnectionConfig: config}
	client, err := api.NewApiClient(adapter, true, false)
	if err != nil {
		return nil, err
	}

	return &sandboxApiImpl{
		config: config,
		client: client,
	}, nil
}

// HandleEnvdAPIException handles envd API exceptions
func HandleEnvdAPIException(resp *http.Response) error {
	// Success responses are not errors
	if resp.StatusCode < 400 {
		return nil
	}

	// Read body for error detail
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return FormatEnvdAPIException(resp.StatusCode, resp.Status)
	}
	// Restore body
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// Try to parse as JSON
	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err == nil {
		if msg, ok := body["message"].(string); ok {
			return FormatEnvdAPIException(resp.StatusCode, msg)
		}
	}

	return FormatEnvdAPIException(resp.StatusCode, string(bodyBytes))
}

// FormatEnvdAPIException formats an envd API exception
func FormatEnvdAPIException(statusCode int, message string) error {
	if statusCode == 404 {
		return NewNotFoundException(message, nil)
	}
	if statusCode == 408 || statusCode == 504 {
		return FormatRequestTimeoutError()
	}
	return NewSandboxException(fmt.Sprintf("%d: %s", statusCode, message), nil)
}

// Pty interface (placeholder)
type Pty interface {
	// PTY methods
}
