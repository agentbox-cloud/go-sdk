package agentbox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// HandleAPIException handles API exceptions
// This matches Python SDK's handle_api_exception() function
// Moved here from api package to avoid circular imports
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
		return NewRateLimitException(
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

	return NewSandboxException(
		fmt.Sprintf("%d: %s", resp.StatusCode, message),
		nil,
	)
}
