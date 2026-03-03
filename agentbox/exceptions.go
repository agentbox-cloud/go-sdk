package agentbox

import "fmt"

// SandboxException is the base class for all sandbox errors
type SandboxException struct {
	Message string
	Err     error
}

func (e *SandboxException) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *SandboxException) Unwrap() error {
	return e.Err
}

// TimeoutException is raised when a timeout occurs
type TimeoutException struct {
	*SandboxException
}

// InvalidArgumentException is raised when an invalid argument is provided
type InvalidArgumentException struct {
	*SandboxException
}

// NotEnoughSpaceException is raised when there is not enough disk space
type NotEnoughSpaceException struct {
	*SandboxException
}

// NotFoundException is raised when a resource is not found
type NotFoundException struct {
	*SandboxException
}

// AuthenticationException is raised when authentication fails
type AuthenticationException struct {
	*SandboxException
}

// TemplateException is raised when the template uses old envd version
type TemplateException struct {
	*SandboxException
}

// RateLimitException is raised when the API rate limit is exceeded
type RateLimitException struct {
	*SandboxException
}

// Helper functions for creating exceptions

func NewSandboxException(message string, err error) *SandboxException {
	return &SandboxException{
		Message: message,
		Err:     err,
	}
}

func NewTimeoutException(message string, err error) *TimeoutException {
	return &TimeoutException{
		SandboxException: NewSandboxException(message, err),
	}
}

func NewInvalidArgumentException(message string, err error) *InvalidArgumentException {
	return &InvalidArgumentException{
		SandboxException: NewSandboxException(message, err),
	}
}

func NewNotEnoughSpaceException(message string, err error) *NotEnoughSpaceException {
	return &NotEnoughSpaceException{
		SandboxException: NewSandboxException(message, err),
	}
}

func NewNotFoundException(message string, err error) *NotFoundException {
	return &NotFoundException{
		SandboxException: NewSandboxException(message, err),
	}
}

func NewAuthenticationException(message string, err error) *AuthenticationException {
	return &AuthenticationException{
		SandboxException: NewSandboxException(message, err),
	}
}

func NewTemplateException(message string, err error) *TemplateException {
	return &TemplateException{
		SandboxException: NewSandboxException(message, err),
	}
}

func NewRateLimitException(message string, err error) *RateLimitException {
	return &RateLimitException{
		SandboxException: NewSandboxException(message, err),
	}
}

// Format functions (matching Python SDK)

func FormatSandboxTimeoutException(message string) *TimeoutException {
	return NewTimeoutException(
		fmt.Sprintf("%s: This error is likely due to sandbox timeout. You can modify the sandbox timeout by passing 'timeout' when starting the sandbox or calling '.SetTimeout' on the sandbox with the desired timeout.", message),
		nil,
	)
}

func FormatRequestTimeoutError() *TimeoutException {
	return NewTimeoutException(
		"Request timed out — the 'request_timeout' option can be used to increase this timeout",
		nil,
	)
}

func FormatExecutionTimeoutError() *TimeoutException {
	return NewTimeoutException(
		"Execution timed out — the 'timeout' option can be used to increase this timeout",
		nil,
	)
}
