package agentbox

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	// FilesystemServiceName is the RPC service name for filesystem
	FilesystemServiceName = "filesystem.Filesystem"
)

// HandleRPCException handles RPC exceptions
// This matches Python SDK's handle_rpc_exception() function
func HandleRPCException(err error) error {
	// For now, check if it's an HTTP error and convert it
	// TODO: Parse Connect RPC error format
	if err == nil {
		return nil
	}

	// Check for specific error types
	if httpErr, ok := err.(*HTTPRPCError); ok {
		switch httpErr.Code {
		case "invalid_argument":
			return NewInvalidArgumentException(httpErr.Message, nil)
		case "unauthenticated":
			return NewAuthenticationException(httpErr.Message, nil)
		case "not_found":
			return NewNotFoundException(httpErr.Message, nil)
		case "unavailable":
			return FormatSandboxTimeoutException(httpErr.Message)
		case "canceled":
			return NewTimeoutException(
				fmt.Sprintf("%s: This error is likely due to exceeding 'requestTimeout'. You can pass the request timeout value as an option when making the request.", httpErr.Message),
				nil,
			)
		case "deadline_exceeded":
			return NewTimeoutException(
				fmt.Sprintf("%s: This error is likely due to exceeding 'timeout' — the total time a long running request (like process or directory watch) can be active. It can be modified by passing 'timeout' when making the request. Use '0' to disable the timeout.", httpErr.Message),
				nil,
			)
		default:
			return NewSandboxException(fmt.Sprintf("%s: %s", httpErr.Code, httpErr.Message), nil)
		}
	}

	return err
}

// HTTPRPCError represents an HTTP-based RPC error
type HTTPRPCError struct {
	Code    string
	Message string
	Status  int
}

func (e *HTTPRPCError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// AuthenticationHeader creates authentication header for RPC calls
// This matches Python SDK's authentication_header() function
func AuthenticationHeader(user Username) map[string]string {
	if user == "" {
		user = DefaultUsername
	}

	value := fmt.Sprintf("%s:", string(user))
	encoded := base64.StdEncoding.EncodeToString([]byte(value))

	return map[string]string{
		"Authorization": fmt.Sprintf("Basic %s", encoded),
	}
}

// CallRPC makes an RPC call using HTTP POST with JSON
// This is a helper function for making Connect RPC calls
func CallRPC(
	ctx context.Context,
	baseURL string,
	serviceName string,
	methodName string,
	request interface{},
	response interface{},
	headers map[string]string,
	timeout time.Duration,
	httpClient *http.Client,
) error {
	url := fmt.Sprintf("%s/%s/%s", baseURL, serviceName, methodName)

	// Marshal request to JSON
	reqBody, err := json.Marshal(request)
	if err != nil {
		return err
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Set timeout
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	// Make request
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check for errors
	if resp.StatusCode >= 400 {
		// Try to parse error response
		var rpcErr struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&rpcErr); err == nil {
			return &HTTPRPCError{
				Code:    rpcErr.Code,
				Message: rpcErr.Message,
				Status:  resp.StatusCode,
			}
		}
		return HandleEnvdAPIException(resp)
	}

	// Parse response
	if response != nil {
		if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
			return err
		}
	}

	return nil
}
