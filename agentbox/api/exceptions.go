package api

import (
	"fmt"
)

// These are local exception types for the api package
// They match agentbox exception types but are defined here to avoid circular imports

// apiException is the base exception type for api package
type apiException struct {
	message string
	err     error
}

func (e *apiException) Error() string {
	if e.err != nil {
		return fmt.Sprintf("%s: %v", e.message, e.err)
	}
	return e.message
}

// apiAuthenticationException represents authentication errors
type apiAuthenticationException struct {
	*apiException
}

// apiRateLimitException represents rate limit errors
type apiRateLimitException struct {
	*apiException
}

// apiSandboxException represents general sandbox errors
type apiSandboxException struct {
	*apiException
}

// Helper functions to create exceptions
func newApiAuthenticationException(message string, err error) *apiAuthenticationException {
	return &apiAuthenticationException{
		apiException: &apiException{message: message, err: err},
	}
}

func newApiRateLimitException(message string, err error) *apiRateLimitException {
	return &apiRateLimitException{
		apiException: &apiException{message: message, err: err},
	}
}

func newApiSandboxException(message string, err error) *apiSandboxException {
	return &apiSandboxException{
		apiException: &apiException{message: message, err: err},
	}
}
