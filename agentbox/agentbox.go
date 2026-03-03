package agentbox

/*
Package agentbox provides a Go SDK for AgentBox cloud sandbox environments.

Secure sandboxed cloud environments made for AI agents and AI apps.

Check docs at https://agentbox.cloud/docs.

AgentBox Sandbox is a secure cloud sandbox environment made for AI agents and AI
apps. Sandboxes allow AI agents and apps to have long running cloud secure environments.
In these environments, large language models can use the same tools as humans do.

AgentBox Go SDK supports both sync and async API:

Sync usage:
	sandbox, err := sandbox_sync.NewSandbox(ctx, &SandboxOptions{
		Template: "base",
		APIKey:   "ab_...",
	})

Async usage:
	// Go uses goroutines and channels for async operations
	// See examples for async patterns
*/

// Package version
const (
	Version = "1.0.0"
)

// Re-export all public types and functions
// This matches the Python SDK's __init__.py structure

// All types are exported from their respective files:
// - ConnectionConfig, ProxyTypes from connection_config.go
// - All exception types from exceptions.go
// - SandboxInfo, ProcessInfo, CommandResult, etc. from types.go
// - Sandbox from sandbox_sync/main.go
