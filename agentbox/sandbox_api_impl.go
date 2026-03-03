package agentbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/agentbox-cloud/go-sdk/agentbox/api"
)

// sandboxApiImpl implements SandboxApi
type sandboxApiImpl struct {
	config *ConnectionConfig
	client *api.ApiClient
}

// List lists all running sandboxes
// This matches Python SDK's SandboxApi.list()
func (api *sandboxApiImpl) List(ctx context.Context, query *SandboxQuery) ([]*ListedSandbox, error) {
	path := "/sandboxes"
	if query != nil && query.Metadata != nil && len(query.Metadata) > 0 {
		// Build query string - match Python SDK: URL encode both keys and values
		// Python SDK does: urllib.parse.quote(k): urllib.parse.quote(v) for each k, v
		// Then urllib.parse.urlencode(quoted_metadata)
		quotedMetadata := make(map[string]string)
		for k, v := range query.Metadata {
			quotedMetadata[url.QueryEscape(k)] = url.QueryEscape(v)
		}
		params := url.Values{}
		for k, v := range quotedMetadata {
			params.Add(k, v)
		}
		metadataStr := params.Encode()
		path += "?metadata=" + metadataStr
	}

	resp, err := api.client.Request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, HandleAPIException(resp)
	}

	var sandboxes []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&sandboxes); err != nil {
		return nil, err
	}

	result := make([]*ListedSandbox, 0, len(sandboxes))
	for _, s := range sandboxes {
		sandboxID := getString(s, "sandboxID")
		clientID := getString(s, "clientID")
		if sandboxID == "" {
			sandboxID = getString(s, "sandbox_id")
		}
		if clientID == "" {
			clientID = getString(s, "client_id")
		}

		combinedSandboxID := sandboxID
		if clientID != "" {
			combinedSandboxID = fmt.Sprintf("%s-%s", sandboxID, clientID)
		}

		sandbox := &ListedSandbox{
			SandboxID:  combinedSandboxID,
			TemplateID: getString(s, "templateID"),
			State:      getString(s, "state"),
			Metadata:   getMapString(s, "metadata"),
		}

		if name, ok := s["alias"].(string); ok {
			sandbox.Name = name
		}

		if cpu, ok := s["cpu_count"].(float64); ok {
			sandbox.CPUCount = int(cpu)
		}

		if mem, ok := s["memory_mb"].(float64); ok {
			sandbox.MemoryMB = int(mem)
		}

		// Parse timestamps
		if startedAt, ok := s["started_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, startedAt); err == nil {
				sandbox.StartedAt = t
			}
		}

		if endAt, ok := s["end_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, endAt); err == nil {
				sandbox.EndAt = &t
			}
		}

		result = append(result, sandbox)
	}

	return result, nil
}

// GetInfo gets information about a specific sandbox
// This matches Python SDK's SandboxApi.get_info()
func (api *sandboxApiImpl) GetInfo(ctx context.Context, sandboxID string) (*SandboxInfo, error) {
	path := fmt.Sprintf("/sandboxes/%s", sandboxID)

	resp, err := api.client.Request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, HandleAPIException(resp)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	info := &SandboxInfo{
		SandboxID:  sandboxID,
		TemplateID: getString(data, "templateID"),
		Metadata:   getMapString(data, "metadata"),
	}

	if name, ok := data["alias"].(string); ok {
		info.Name = name
	}

	if version, ok := data["envd_version"].(string); ok {
		info.EnvdVersion = version
	}

	if token, ok := data["envd_access_token"].(string); ok {
		info.EnvdAccessToken = token
	}

	// Parse timestamps
	if startedAt, ok := data["started_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, startedAt); err == nil {
			info.StartedAt = t
		}
	}

	if endAt, ok := data["end_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, endAt); err == nil {
			info.EndAt = &t
		}
	}

	return info, nil
}

// Create creates a new sandbox
// This matches Python SDK's SandboxApi._create_sandbox()
func (api *sandboxApiImpl) Create(ctx context.Context, opts *CreateSandboxOptions) (*SandboxInfo, error) {
	// Ensure metadata and env_vars are not nil (API doesn't accept null)
	metadata := opts.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}
	envVars := opts.Envs
	if envVars == nil {
		envVars = make(map[string]string)
	}

	body := map[string]interface{}{
		"templateID": opts.Template,
		"timeout":    opts.Timeout,
		"metadata":   metadata,
		"envVars":    envVars,
		"secure":     opts.Secure,
	}

	resp, err := api.client.Request(ctx, "POST", "/sandboxes", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, HandleAPIException(resp)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	sandboxID := getString(data, "sandboxID")
	clientID := getString(data, "clientID")
	if clientID != "" {
		sandboxID = fmt.Sprintf("%s-%s", sandboxID, clientID)
	}

	info := &SandboxInfo{
		SandboxID:   sandboxID,
		TemplateID:  getString(data, "templateID"),
		EnvdVersion: getString(data, "envd_version"),
		Metadata:    getMapString(data, "metadata"),
	}

	if token, ok := data["envd_access_token"].(string); ok {
		info.EnvdAccessToken = token
	}

	return info, nil
}

// Connect connects to an existing sandbox
// This matches Python SDK's SandboxApi._cls_connect()
// If the sandbox is paused, it will be automatically resumed.
// timeout: Timeout for the sandbox in seconds. For running sandboxes, the timeout will update only if the new timeout is longer than the existing one.
func (api *sandboxApiImpl) Connect(ctx context.Context, sandboxID string, timeout *int) (*SandboxInfo, error) {
	// Build request body with timeout (matching Python SDK's ConnectSandbox)
	body := map[string]interface{}{}
	if timeout != nil {
		body["timeout"] = *timeout
	}

	resp, err := api.client.Request(ctx, "POST", fmt.Sprintf("/sandboxes/%s/connect", sandboxID), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, NewSandboxException(
			fmt.Sprintf("Sandbox %s not found", sandboxID),
			nil,
		)
	}

	if resp.StatusCode >= 300 {
		return nil, HandleAPIException(resp)
	}

	return api.GetInfo(ctx, sandboxID)
}

// Kill kills a sandbox
func (api *sandboxApiImpl) Kill(ctx context.Context, sandboxID string) (bool, error) {
	resp, err := api.client.Request(ctx, "DELETE", fmt.Sprintf("/sandboxes/%s", sandboxID), nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return false, nil
	}

	if resp.StatusCode >= 300 {
		return false, HandleAPIException(resp)
	}

	return true, nil
}

// SetTimeout sets the timeout for a sandbox
func (api *sandboxApiImpl) SetTimeout(ctx context.Context, sandboxID string, timeout int) error {
	body := map[string]interface{}{
		"timeout": timeout,
	}

	resp, err := api.client.Request(ctx, "POST", fmt.Sprintf("/sandboxes/%s/timeout", sandboxID), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return HandleAPIException(resp)
	}

	return nil
}

// Pause pauses a sandbox
func (api *sandboxApiImpl) Pause(ctx context.Context, sandboxID string) error {
	resp, err := api.client.Request(ctx, "POST", fmt.Sprintf("/sandboxes/%s/pause", sandboxID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return HandleAPIException(resp)
	}

	return nil
}

// Resume resumes a paused sandbox
func (api *sandboxApiImpl) Resume(ctx context.Context, sandboxID string, timeout *int) (*SandboxInfo, error) {
	body := map[string]interface{}{}
	if timeout != nil {
		body["timeout"] = *timeout
	}

	resp, err := api.client.Request(ctx, "POST", fmt.Sprintf("/sandboxes/%s/resume", sandboxID), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, HandleAPIException(resp)
	}

	return api.GetInfo(ctx, sandboxID)
}

// GetADBPublicInfo gets ADB public information for a sandbox
// This matches Python SDK's SandboxApi._get_adb_public_info()
func (api *sandboxApiImpl) GetADBPublicInfo(ctx context.Context, sandboxID string) (*ADBPublicInfo, error) {
	// Python SDK uses: /sandboxes/{sandbox_id}/adb-public-info (with hyphen, not slash)
	path := fmt.Sprintf("/sandboxes/%s/adb-public-info", sandboxID)

	resp, err := api.client.Request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, HandleAPIException(resp)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	// Extract fields (support both camelCase and snake_case, matching Python SDK)
	info := &ADBPublicInfo{
		ADBIP:      getStringWithFallback(data, "adbIp", "adb_ip"),
		ADBPort:    getIntWithFallback(data, "adbPort", "adb_port"),
		PublicKey:  getStringWithFallback(data, "publicKey", "public_key"),
		PrivateKey: getStringWithFallback(data, "privateKey", "private_key"),
	}

	return info, nil
}

// Helper functions

func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if val, ok := m[key].(float64); ok {
		return int(val)
	}
	return 0
}

func getMapString(m map[string]interface{}, key string) map[string]string {
	result := make(map[string]string)
	if val, ok := m[key].(map[string]interface{}); ok {
		for k, v := range val {
			if str, ok := v.(string); ok {
				result[k] = str
			}
		}
	}
	return result
}

// getStringWithFallback gets a string value, trying multiple keys
func getStringWithFallback(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := m[key].(string); ok {
			return val
		}
	}
	return ""
}

// getIntWithFallback gets an int value, trying multiple keys
func getIntWithFallback(m map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		if val, ok := m[key].(float64); ok {
			return int(val)
		}
	}
	return 0
}
