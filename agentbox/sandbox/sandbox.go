package sandbox

import (
	"context"
	"time"

	"github.com/agentbox-cloud/go-sdk/agentbox"
)

type Options = agentbox.SandboxOptions
type Sync = agentbox.Sandbox
type Async = agentbox.AsyncSandbox

func New(ctx context.Context, opts *Options) (*Sync, error) {
	return agentbox.NewSandbox(ctx, opts)
}

func NewAsync(ctx context.Context, opts *Options) (*Async, error) {
	return agentbox.AsyncSandboxCreate(ctx, opts)
}

func Connect(
	ctx context.Context,
	sandboxID string,
	timeout *int,
	apiKey *string,
	domain *string,
	debug *bool,
	requestTimeout *time.Duration,
	proxy agentbox.ProxyTypes,
) (*Sync, error) {
	return agentbox.SandboxConnect(ctx, sandboxID, timeout, apiKey, domain, debug, requestTimeout, proxy)
}

func AsyncConnect(
	ctx context.Context,
	sandboxID string,
	timeout *int,
	apiKey *string,
	domain *string,
	debug *bool,
	requestTimeout *time.Duration,
	proxy agentbox.ProxyTypes,
) (*Async, error) {
	return agentbox.AsyncSandboxConnect(ctx, sandboxID, timeout, apiKey, domain, debug, requestTimeout, proxy)
}

