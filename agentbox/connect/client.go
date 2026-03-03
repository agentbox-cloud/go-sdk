package connect

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Client is a Connect Protocol client
// This matches Python SDK's Connect Protocol client
type Client struct {
	baseURL    string
	codec      Codec
	headers    map[string]string
	httpClient *http.Client
}

// NewClient creates a new Connect Protocol client
func NewClient(baseURL string, codec Codec, headers map[string]string, timeout time.Duration) *Client {
	httpClient := &http.Client{
		Timeout: timeout,
	}

	return &Client{
		baseURL:    baseURL,
		codec:      codec,
		headers:    headers,
		httpClient: httpClient,
	}
}

// NewClientWithHTTPClient creates a new Connect Protocol client with custom HTTP client
func NewClientWithHTTPClient(baseURL string, codec Codec, headers map[string]string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    baseURL,
		codec:      codec,
		headers:    headers,
		httpClient: httpClient,
	}
}

// CallServerStream makes a server stream RPC call
// This matches Python SDK's acall_server_stream() method
func (c *Client) CallServerStream(
	ctx context.Context,
	method string,
	req interface{},
	timeout *time.Duration,
) (<-chan interface{}, <-chan error) {
	msgChan := make(chan interface{}, 10)
	errChan := make(chan error, 1)

	go func() {
		// Use sync.Once to ensure channels are closed only once
		var closeOnce sync.Once
		closeChannels := func() {
			closeOnce.Do(func() {
				close(errChan)
				close(msgChan)
			})
		}

		// Encode request
		reqData, err := c.codec.Encode(req)
		if err != nil {
			select {
			case errChan <- fmt.Errorf("failed to encode request: %w", err):
			default:
			}
			closeChannels()
			return
		}

		// Encode as envelope
		envelopeData := EncodeEnvelope(0, reqData)

		// Create HTTP request
		url := c.baseURL + "/" + method
		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(envelopeData))
		if err != nil {
			select {
			case errChan <- err:
			default:
			}
			closeChannels()
			return
		}

		// Set headers
		for k, v := range c.headers {
			httpReq.Header.Set(k, v)
		}
		contentType := fmt.Sprintf("application/connect+%s", c.codec.ContentType())
		httpReq.Header.Set("Content-Type", contentType)
		httpReq.Header.Set("Connect-Protocol-Version", "1")
		httpReq.Header.Set("Connect-Content-Encoding", "identity")

		// Add timeout header if specified
		if timeout != nil && *timeout > 0 {
			httpReq.Header.Set("connect-timeout-ms", strconv.FormatInt(timeout.Milliseconds(), 10))
		}

		// Make request
		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			select {
			case errChan <- err:
			default:
			}
			closeChannels()
			return
		}
		defer resp.Body.Close()

		// Check status
		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			var errorData map[string]interface{}
			var errMsg error
			if err := c.codec.Decode(body, &errorData); err == nil {
				errMsg = fmt.Errorf("connect error: %v", errorData)
			} else {
				errMsg = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
			}
			select {
			case errChan <- errMsg:
			default:
			}
			closeChannels()
			return
		}

		// Parse stream - ReadFrom will process messages
		// We close channels after ReadFrom returns (whether success or error)
		parser := NewServerStreamParser(c.codec)
		parser.ReadFrom(resp.Body, msgChan, errChan)
		// Close channels after ReadFrom completes
		closeChannels()
	}()

	return msgChan, errChan
}
