package envd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/agentbox-cloud/go-sdk/agentbox"
)

const (
	// ENVDAPIFilesRoute is the route for file operations
	ENVDAPIFilesRoute = "/files"
	// ENVDAPIHealthRoute is the route for health checks
	ENVDAPIHealthRoute = "/health"
)

// HandleEnvdAPIException handles envd API exceptions
// This matches Python SDK's handle_envd_api_exception() function
func HandleEnvdAPIException(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return FormatEnvdAPIException(resp.StatusCode, resp.Status)
	}
	// Restore body
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// Try to parse as JSON
	var body map[string]interface{}
	message := resp.Status
	if err := json.Unmarshal(bodyBytes, &body); err == nil {
		if msg, ok := body["message"].(string); ok {
			message = msg
		}
	} else {
		message = string(bodyBytes)
	}

	return FormatEnvdAPIException(resp.StatusCode, message)
}

// FormatEnvdAPIException formats an envd API exception
// This matches Python SDK's format_envd_api_exception() function
func FormatEnvdAPIException(statusCode int, message string) error {
	switch statusCode {
	case 400:
		return agentbox.NewInvalidArgumentException(message, nil)
	case 401:
		return agentbox.NewAuthenticationException(message, nil)
	case 404:
		return agentbox.NewNotFoundException(message, nil)
	case 429:
		return agentbox.NewSandboxException(fmt.Sprintf("%s: The requests are being rate limited.", message), nil)
	case 502:
		return agentbox.FormatSandboxTimeoutException(message)
	case 507:
		return agentbox.NewNotEnoughSpaceException(message, nil)
	default:
		return agentbox.NewSandboxException(fmt.Sprintf("%d: %s", statusCode, message), nil)
	}
}
