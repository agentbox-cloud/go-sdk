package httpapi

import "github.com/agentbox-cloud/go-sdk/agentbox"

// HTTP API client aliases and constructors.
type APIClient = agentbox.APIClient

func NewAPIClient(config *agentbox.ConnectionConfig, requireAPIKey bool, requireAccessToken bool) (*APIClient, error) {
	return agentbox.NewAPIClient(config, requireAPIKey, requireAccessToken)
}

