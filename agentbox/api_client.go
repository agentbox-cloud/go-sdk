package agentbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
)

const (
	// DefaultUserAgent is the default user agent for API requests
	DefaultUserAgent = "agentbox-go-sdk"
	// SDKVersion is the SDK version
	SDKVersion = "1.0.0"
)

// APIClient is the HTTP client for interacting with the AgentBox API
type APIClient struct {
	baseURL     string
	httpClient  *http.Client
	headers     map[string]string
	apiKey      string
	accessToken string
	useAPIKey   bool
}

// NewAPIClient creates a new API client
func NewAPIClient(config *ConnectionConfig, requireAPIKey bool, requireAccessToken bool) (*APIClient, error) {
	if requireAPIKey && requireAccessToken {
		return nil, NewAuthenticationException(
			"Only one of api_key or access_token can be required, not both",
			nil,
		)
	}

	if !requireAPIKey && !requireAccessToken {
		return nil, NewAuthenticationException(
			"Either api_key or access_token is required",
			nil,
		)
	}

	var token string
	var useAPIKey bool
	var authHeaderName string

	if requireAPIKey {
		if config.APIKey == "" {
			return nil, NewAuthenticationException(
				"API key is required, please visit the Team tab at https://agentbox.cloud/dashboard to get your API key. "+
					"You can either set the environment variable `AGENTBOX_API_KEY` "+
					"or you can pass it directly to the sandbox like Sandbox(api_key=\"ab_...\")",
				nil,
			)
		}
		token = config.APIKey
		useAPIKey = true
		authHeaderName = "X-API-KEY"
	} else {
		if config.AccessToken == "" {
			return nil, NewAuthenticationException(
				"Access token is required, please visit the Personal tab at https://agentbox.cloud/dashboard to get your access token. "+
					"You can set the environment variable `AGENTBOX_ACCESS_TOKEN` or pass the `access_token` in options.",
				nil,
			)
		}
		token = config.AccessToken
		useAPIKey = false
		authHeaderName = "Authorization"
	}

	// Build default headers
	headers := make(map[string]string)
	headers["User-Agent"] = DefaultUserAgent
	headers["lang"] = "go"
	headers["lang_version"] = runtime.Version()
	headers["machine"] = runtime.GOARCH
	headers["os"] = runtime.GOOS
	headers["package_version"] = SDKVersion
	headers["publisher"] = "agentbox"
	headers["sdk_runtime"] = "go"
	headers["system"] = runtime.GOOS

	// Add custom headers
	for k, v := range config.Headers {
		headers[k] = v
	}

	// Set auth header
	if useAPIKey {
		headers[authHeaderName] = token
	} else {
		headers[authHeaderName] = "Bearer " + token
	}

	// Create HTTP client
	httpClient := &http.Client{
		Timeout: config.RequestTimeout,
		// TODO: Add proxy support
	}

	return &APIClient{
		baseURL:     config.APIURL(),
		httpClient:  httpClient,
		headers:     headers,
		apiKey:      token,
		accessToken: token,
		useAPIKey:   useAPIKey,
	}, nil
}

// Request makes an HTTP request
func (c *APIClient) Request(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	url := c.baseURL + path

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}

	// Set headers
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Log request
	// TODO: Add logging

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Log response
	// TODO: Add logging

	return resp, nil
}

// HandleAPIException handles API exceptions
func HandleAPIException(resp *http.Response) error {
	var body map[string]interface{}
	if resp.Body != nil {
		decoder := json.NewDecoder(resp.Body)
		decoder.Decode(&body)
	}

	message := ""
	if msg, ok := body["message"].(string); ok {
		message = msg
	} else {
		message = resp.Status
	}

	if resp.StatusCode == 429 {
		return NewRateLimitException(
			fmt.Sprintf("%d: Rate limit exceeded, please try again later.", resp.StatusCode),
			nil,
		)
	}

	return NewSandboxException(
		fmt.Sprintf("%d: %s", resp.StatusCode, message),
		nil,
	)
}

// HandleEnvdAPIException handles envd API exceptions
func HandleEnvdAPIException(resp *http.Response) error {
	// Success responses are not errors. Keep body readable by downstream decoders.
	if resp.StatusCode < 400 {
		return nil
	}

	// Read body for error detail.
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		// If we can't read body, just use status
		return FormatEnvdAPIException(resp.StatusCode, resp.Status)
	}
	// Restore body for potential callers that still want to inspect it.
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// Try to parse as JSON
	var body map[string]interface{}
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &body); err == nil {
			message := ""
			if msg, ok := body["message"].(string); ok {
				message = msg
			} else if errMsg, ok := body["error"].(string); ok {
				message = errMsg
			} else {
				message = string(bodyBytes)
			}
			return FormatEnvdAPIException(resp.StatusCode, message)
		}
	}

	// If not JSON or empty, use status and body as message
	message := resp.Status
	if len(bodyBytes) > 0 {
		message = string(bodyBytes)
	}
	return FormatEnvdAPIException(resp.StatusCode, message)
}

// FormatEnvdAPIException formats envd API exception
func FormatEnvdAPIException(statusCode int, message string) error {
	switch statusCode {
	case 400:
		return NewInvalidArgumentException(message, nil)
	case 401:
		return NewAuthenticationException(message, nil)
	case 404:
		return NewNotFoundException(message, nil)
	case 429:
		return NewSandboxException(fmt.Sprintf("%s: The requests are being rate limited.", message), nil)
	case 502:
		return FormatSandboxTimeoutException(message)
	case 507:
		return NewNotEnoughSpaceException(message, nil)
	default:
		return NewSandboxException(fmt.Sprintf("%d: %s", statusCode, message), nil)
	}
}
