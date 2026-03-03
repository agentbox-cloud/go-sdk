package agentbox

import (
	"context"
	"time"
)

// SandboxApi provides methods for interacting with the AgentBox API
type SandboxApi interface {
	// List lists all running sandboxes
	List(ctx context.Context, query *SandboxQuery) ([]*ListedSandbox, error)
	
	// GetInfo gets information about a specific sandbox
	GetInfo(ctx context.Context, sandboxID string) (*SandboxInfo, error)
	
	// Create creates a new sandbox
	Create(ctx context.Context, opts *CreateSandboxOptions) (*SandboxInfo, error)
	
	// Connect connects to an existing sandbox
	Connect(ctx context.Context, sandboxID string) (*SandboxInfo, error)
	
	// Kill kills a sandbox
	Kill(ctx context.Context, sandboxID string) (bool, error)
	
	// SetTimeout sets the timeout for a sandbox
	SetTimeout(ctx context.Context, sandboxID string, timeout int) error
	
	// Pause pauses a sandbox
	Pause(ctx context.Context, sandboxID string) error
	
	// Resume resumes a paused sandbox
	Resume(ctx context.Context, sandboxID string, timeout *int) (*SandboxInfo, error)
}

// CreateSandboxOptions are options for creating a sandbox
type CreateSandboxOptions struct {
	Template      string
	Timeout       int
	Metadata      map[string]string
	Envs          map[string]string
	Secure        bool
	APIKey        string
	Domain        string
	Debug         bool
	RequestTimeout time.Duration
	Proxy         ProxyTypes
}

// SandboxApi interface is defined above
// Implementation is in sandbox_api_impl.go

