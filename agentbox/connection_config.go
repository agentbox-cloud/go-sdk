package agentbox

import (
	"os"
	"strconv"
	"time"
)

const (
	// DefaultRequestTimeout is the default timeout for API requests (30 seconds)
	DefaultRequestTimeout = 30 * time.Second

	// DefaultSandboxTimeout is the default timeout for sandbox (300 seconds)
	DefaultSandboxTimeout = 300

	// DefaultConnectTimeout is the default timeout for reconnecting to paused sandbox (3600 seconds)
	DefaultConnectTimeout = 3600

	// DefaultTemplate is the default sandbox template
	DefaultTemplate = "base"

	// KeepalivePingIntervalSec is the keepalive ping interval in seconds
	KeepalivePingIntervalSec = 50

	// KeepalivePingHeader is the HTTP header for keepalive ping interval
	KeepalivePingHeader = "Keepalive-Ping-Interval"
)

// ProxyTypes represents proxy configuration types
type ProxyTypes interface {
	// Proxy configuration interface
}

// ConnectionConfig represents the configuration for connecting to the AgentBox API
type ConnectionConfig struct {
	// Domain is the AgentBox domain (default: "agentbox.cloud")
	Domain string

	// Debug enables debug mode (default: false)
	// When enabled, requests are sent to localhost
	Debug bool

	// APIKey is the AgentBox API key
	APIKey string

	// AccessToken is the access token for authentication
	AccessToken string

	// RequestTimeout is the timeout for API requests
	RequestTimeout time.Duration

	// Headers are additional headers to send with requests
	Headers map[string]string

	// Proxy is the proxy configuration
	Proxy ProxyTypes
}

// NewConnectionConfig creates a new ConnectionConfig with default values
// Values are loaded from environment variables if not provided:
// - AGENTBOX_DOMAIN (default: "agentbox.cloud")
// - AGENTBOX_DEBUG (default: false)
// - AGENTBOX_API_KEY
// - AGENTBOX_ACCESS_TOKEN
func NewConnectionConfig(opts *ConnectionConfigOptions) *ConnectionConfig {
	config := &ConnectionConfig{
		Domain:         getEnvOrDefault("AGENTBOX_DOMAIN", "agentbox.cloud"),
		Debug:          getEnvBoolOrDefault("AGENTBOX_DEBUG", false),
		APIKey:         os.Getenv("AGENTBOX_API_KEY"),
		AccessToken:    os.Getenv("AGENTBOX_ACCESS_TOKEN"),
		RequestTimeout: DefaultRequestTimeout,
		Headers:        make(map[string]string),
	}

	if opts != nil {
		if opts.Domain != "" {
			config.Domain = opts.Domain
		}
		if opts.Debug != nil {
			config.Debug = *opts.Debug
		}
		if opts.APIKey != "" {
			config.APIKey = opts.APIKey
		}
		if opts.AccessToken != "" {
			config.AccessToken = opts.AccessToken
		}
		if opts.RequestTimeout > 0 {
			config.RequestTimeout = opts.RequestTimeout
		}
		if opts.Headers != nil {
			config.Headers = opts.Headers
		}
		if opts.Proxy != nil {
			config.Proxy = opts.Proxy
		}
	}

	// Set API URL based on debug mode
	if config.Debug {
		// In debug mode, API URL is localhost
		// This is handled in the API client
	} else {
		// Normal mode: https://api.{domain}
		// This is handled in the API client
	}

	return config
}

// ConnectionConfigOptions are options for creating a ConnectionConfig
type ConnectionConfigOptions struct {
	Domain         string
	Debug          *bool
	APIKey         string
	AccessToken    string
	RequestTimeout time.Duration
	Headers        map[string]string
	Proxy          ProxyTypes
}

// GetRequestTimeout returns the request timeout, using the provided timeout if not zero,
// otherwise using the configured timeout
func (c *ConnectionConfig) GetRequestTimeout(timeout time.Duration) time.Duration {
	if timeout > 0 {
		return timeout
	}
	if c.RequestTimeout > 0 {
		return c.RequestTimeout
	}
	return DefaultRequestTimeout
}

// APIURL returns the API URL based on the configuration
func (c *ConnectionConfig) APIURL() string {
	if c.Debug {
		return "http://localhost:3000"
	}
	return "https://api." + c.Domain
}

// Username represents the user for operations in the sandbox
type Username string

const (
	// UsernameRoot represents the root user
	UsernameRoot Username = "root"

	// UsernameUser represents the regular user (default)
	UsernameUser Username = "user"
)

// DefaultUsername is the default username for sandbox operations
const DefaultUsername Username = UsernameUser

// Helper functions

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}
