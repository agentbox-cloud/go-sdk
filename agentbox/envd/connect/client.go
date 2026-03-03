package connect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Client is a Connect Protocol client for making RPC calls
// This matches Python SDK's agentbox_connect.Client
type Client struct {
	baseURL    string
	httpClient *http.Client
	codec      Codec
	headers    map[string]string
}

// NewClient creates a new Connect Protocol client
func NewClient(baseURL string, codec Codec, headers map[string]string, timeout time.Duration) *Client {
	if headers == nil {
		headers = make(map[string]string)
	}

	// Add default headers
	defaultHeaders := map[string]string{
		"user-agent":               "connect-go",
		"connect-protocol-version": "1",
	}

	for k, v := range defaultHeaders {
		if _, exists := headers[k]; !exists {
			headers[k] = v
		}
	}

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		codec:   codec,
		headers: headers,
	}
}

// CallUnary makes a unary RPC call
func (c *Client) CallUnary(ctx context.Context, method string, req interface{}) (interface{}, error) {
	// Encode request
	reqData, err := c.codec.Encode(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	// Create HTTP request
	url := c.baseURL + "/" + method
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqData))
	if err != nil {
		return nil, err
	}

	// Set headers
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}
	httpReq.Header.Set("content-type", fmt.Sprintf("application/%s", c.codec.ContentType()))

	// Make request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		var errorData map[string]interface{}
		if err := json.Unmarshal(body, &errorData); err == nil {
			return nil, fmt.Errorf("connect error: %v", errorData)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Read and decode response
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result interface{}
	if err := c.codec.Decode(respData, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// CallServerStream makes a server streaming RPC call
// Returns a channel that receives messages and an error channel
func (c *Client) CallServerStream(
	ctx context.Context,
	method string,
	req interface{},
	timeout *time.Duration,
) (<-chan interface{}, <-chan error) {
	msgChan := make(chan interface{}, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(msgChan)
		defer close(errChan)

		// Encode request
		reqData, err := c.codec.Encode(req)
		if err != nil {
			errChan <- fmt.Errorf("failed to encode request: %w", err)
			return
		}

		// Encode as envelope
		envelopeData := EncodeEnvelope(0, reqData)

		// Create HTTP request
		url := c.baseURL + "/" + method
		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(envelopeData))
		if err != nil {
			errChan <- err
			return
		}

		// Set headers
		for k, v := range c.headers {
			httpReq.Header.Set(k, v)
		}
		contentType := fmt.Sprintf("application/connect+%s", c.codec.ContentType())
		httpReq.Header.Set("content-type", contentType)

		// Add timeout header if specified
		if timeout != nil && *timeout > 0 {
			httpReq.Header.Set("connect-timeout-ms", strconv.FormatInt(timeout.Milliseconds(), 10))
		}

		// Make request
		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			errChan <- err
			return
		}
		defer resp.Body.Close()

		// Check status
		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			var errorData map[string]interface{}
			if err := json.Unmarshal(body, &errorData); err == nil {
				errChan <- fmt.Errorf("connect error: %v", errorData)
			} else {
				errChan <- fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
			}
			return
		}

		// Parse stream
		parser := NewServerStreamParser(c.codec)
		parser.ReadFrom(resp.Body, msgChan, errChan)
	}()

	return msgChan, errChan
}
