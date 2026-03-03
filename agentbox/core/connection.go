package core

import "github.com/agentbox-cloud/go-sdk/agentbox"

// Core connection/config aliases.
type ConnectionConfig = agentbox.ConnectionConfig
type ConnectionConfigOptions = agentbox.ConnectionConfigOptions
type ProxyTypes = agentbox.ProxyTypes
type Username = agentbox.Username

const (
	DefaultRequestTimeout   = agentbox.DefaultRequestTimeout
	DefaultSandboxTimeout   = agentbox.DefaultSandboxTimeout
	DefaultConnectTimeout   = agentbox.DefaultConnectTimeout
	DefaultTemplate         = agentbox.DefaultTemplate
	KeepalivePingIntervalSec = agentbox.KeepalivePingIntervalSec
	KeepalivePingHeader     = agentbox.KeepalivePingHeader
	DefaultUsername         = agentbox.DefaultUsername
	UsernameRoot            = agentbox.UsernameRoot
	UsernameUser            = agentbox.UsernameUser
)

func NewConnectionConfig(opts *ConnectionConfigOptions) *ConnectionConfig {
	return agentbox.NewConnectionConfig(opts)
}

