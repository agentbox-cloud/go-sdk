package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ConnectionConfig interface to avoid circular imports
// This is a minimal interface that api package needs
type ConnectionConfig interface {
	GetAPIKey() string
	GetAccessToken() string
	GetRequestTimeout(time.Duration) time.Duration
	GetHeaders() map[string]string
	GetAPIURL() string
}

// connectionConfigWrapper wraps agentbox.ConnectionConfig to implement ConnectionConfig interface
type connectionConfigWrapper struct {
	apiKey         string
	accessToken    string
	requestTimeout time.Duration
	headers        map[string]string
	apiURL         string
}

func (c *connectionConfigWrapper) GetAPIKey() string {
	return c.apiKey
}

func (c *connectionConfigWrapper) GetAccessToken() string {
	return c.accessToken
}

func (c *connectionConfigWrapper) GetRequestTimeout(timeout time.Duration) time.Duration {
	if timeout > 0 {
		return timeout
	}
	if c.requestTimeout > 0 {
		return c.requestTimeout
	}
	return 0
}

func (c *connectionConfigWrapper) GetHeaders() map[string]string {
	return c.headers
}

func (c *connectionConfigWrapper) GetAPIURL() string {
	return c.apiURL
}

// ApiClient is the client for interacting with the AgentBox API
// This matches Python SDK's ApiClient class in agentbox/api/__init__.py
type ApiClient struct {
	config             ConnectionConfig
	httpClient         *http.Client
	baseURL            string
	headers            map[string]string
	requireAPIKey      bool
	requireAccessToken bool
}

// NewApiClient creates a new API client
// This matches Python SDK's ApiClient.__init__()
// Note: config parameter accepts interface to avoid circular imports
func NewApiClient(
	config ConnectionConfig,
	requireAPIKey bool,
	requireAccessToken bool,
) (*ApiClient, error) {
	if requireAPIKey && requireAccessToken {
		return nil, newApiAuthenticationException(
			"Only one of api_key or access_token can be required, not both",
			nil,
		)
	}

	if !requireAPIKey && !requireAccessToken {
		return nil, newApiAuthenticationException(
			"Either api_key or access_token is required",
			nil,
		)
	}

	var token string
	var authHeaderName string

	if requireAPIKey {
		if config.GetAPIKey() == "" {
			return nil, newApiAuthenticationException(
				"API key is required, please visit the Team tab at https://agentbox.cloud/dashboard to get your API key. "+
					"You can either set the environment variable `AGENTBOX_API_KEY` "+
					"or you can pass it directly to the sandbox like Sandbox(api_key=\"ab_...\")",
				nil,
			)
		}
		token = config.GetAPIKey()
		authHeaderName = "X-API-KEY"
	} else {
		if config.GetAccessToken() == "" {
			return nil, newApiAuthenticationException(
				"Access token is required, please visit the Personal tab at https://agentbox.cloud/dashboard to get your access token. "+
					"You can set the environment variable `AGENTBOX_ACCESS_TOKEN` or pass the `access_token` in options.",
				nil,
			)
		}
		token = config.GetAccessToken()
		authHeaderName = "Authorization"
	}

	// Build headers
	headers := DefaultHeaders()
	for k, v := range config.GetHeaders() {
		headers[k] = v
	}

	// Set auth header
	if requireAPIKey {
		headers[authHeaderName] = token
	} else {
		headers[authHeaderName] = "Bearer " + token
	}

	// Create HTTP client
	timeout := config.GetRequestTimeout(0)
	httpClient := &http.Client{
		Timeout: timeout,
		// TODO: Add proxy support
	}

	return &ApiClient{
		config:             config,
		httpClient:         httpClient,
		baseURL:            config.GetAPIURL(),
		headers:            headers,
		requireAPIKey:      requireAPIKey,
		requireAccessToken: requireAccessToken,
	}, nil
}

// Request makes an HTTP request
func (c *ApiClient) Request(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
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

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// Close closes the HTTP client
func (c *ApiClient) Close() {
	// HTTP client doesn't need explicit close in Go
}

// HandleAPIException handles API exceptions
// This matches Python SDK's handle_api_exception() function
// Note: This is implemented here to avoid circular imports
func HandleAPIException(resp *http.Response) error {
	var body map[string]interface{}
	if resp.Body != nil {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err == nil {
			json.Unmarshal(bodyBytes, &body)
			// Restore body for potential callers
			resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
	}

	if resp.StatusCode == 429 {
		return newApiRateLimitException(
			fmt.Sprintf("%d: Rate limit exceeded, please try again later.", resp.StatusCode),
			nil,
		)
	}

	message := ""
	if msg, ok := body["message"].(string); ok {
		message = msg
	} else {
		message = resp.Status
	}

	return newApiSandboxException(
		fmt.Sprintf("%d: %s", resp.StatusCode, message),
		nil,
	)
}

// SandboxCreateResponse contains the response from creating a sandbox
// This matches Python SDK's SandboxCreateResponse dataclass
type SandboxCreateResponse struct {
	SandboxID       string
	EnvdVersion     string
	EnvdAccessToken string
}

// SetHTTPClient sets a custom HTTP client
func (c *ApiClient) SetHTTPClient(client *http.Client) {
	c.httpClient = client
}

// GetHTTPClient returns the HTTP client
func (c *ApiClient) GetHTTPClient() *http.Client {
	return c.httpClient
}
