package api

import (
	"context"

	"github.com/agentbox-cloud/go-sdk/agentbox"
)

type Service = agentbox.SandboxApi
type Query = agentbox.SandboxQuery

type CreateOptions = agentbox.CreateSandboxOptions

func New(config *agentbox.ConnectionConfig) (Service, error) {
	return agentbox.NewSandboxApi(config)
}

func List(ctx context.Context, svc Service, query *Query) ([]*agentbox.ListedSandbox, error) {
	return svc.List(ctx, query)
}
