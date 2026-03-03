package core

import "github.com/agentbox-cloud/go-sdk/agentbox"

type SandboxException = agentbox.SandboxException
type InvalidArgumentException = agentbox.InvalidArgumentException
type AuthenticationException = agentbox.AuthenticationException
type NotFoundException = agentbox.NotFoundException
type TimeoutException = agentbox.TimeoutException
type TemplateException = agentbox.TemplateException
type NotEnoughSpaceException = agentbox.NotEnoughSpaceException

func FormatSandboxTimeoutException(message string) error {
	return agentbox.FormatSandboxTimeoutException(message)
}

