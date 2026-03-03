package agentbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// sandboxApiImpl implements SandboxApi
type sandboxApiImpl struct {
	config *ConnectionConfig
	client *APIClient
}

// NewSandboxApi creates a new SandboxApi instance
func NewSandboxApi(config *ConnectionConfig) (SandboxApi, error) {
	client, err := NewAPIClient(config, true, false)
	if err != nil {
		return nil, err
	}

	return &sandboxApiImpl{
		config: config,
		client: client,
	}, nil
}

// List lists all running sandboxes
func (api *sandboxApiImpl) List(ctx context.Context, query *SandboxQuery) ([]*ListedSandbox, error) {
	path := "/sandboxes"
	if query != nil && query.Metadata != nil && len(query.Metadata) > 0 {
		// Build query string - match Python SDK: URL encode both keys and values
		// Python SDK uses: urllib.parse.quote(k): urllib.parse.quote(v)
		// Then urlencode the result
		params := url.Values{}
		for k, v := range query.Metadata {
			// url.Values.Add() automatically URL encodes both key and value
			params.Add(k, v)
		}
		// The metadata parameter should be a single query string
		// According to OpenAPI spec: "user=abc&app=prod" (each key and value URL encoded)
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
		// API returns camelCase field names (sandboxID, clientID, templateID, etc.)
		// Match Python SDK: combine sandboxID and clientID as "{sandboxID}-{clientID}"
		sandboxID := getString(s, "sandboxID")
		clientID := getString(s, "clientID")

		// Fallback to snake_case for backward compatibility
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

		// Get templateID (camelCase)
		templateID := getString(s, "templateID")
		if templateID == "" {
			templateID = getString(s, "template_id") // Fallback
		}

		sandbox := &ListedSandbox{
			SandboxID:  combinedSandboxID,
			TemplateID: templateID,
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

		if startedAt, ok := s["started_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, startedAt); err == nil {
				sandbox.StartedAt = t
			}
		}

		if endAt, ok := s["end_at"].(string); ok && endAt != "" {
			if t, err := time.Parse(time.RFC3339, endAt); err == nil {
				sandbox.EndAt = &t
			}
		}

		result = append(result, sandbox)
	}

	return result, nil
}

// GetInfo gets information about a specific sandbox
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

	// API returns camelCase field names (sandboxID, clientID, templateID, etc.)
	// Match Python SDK: combine sandboxID and clientID as "{sandboxID}-{clientID}"
	rawSandboxID := getString(data, "sandboxID")
	rawClientID := getString(data, "clientID")

	// Fallback to snake_case for backward compatibility
	if rawSandboxID == "" {
		rawSandboxID = getString(data, "sandbox_id")
	}
	if rawClientID == "" {
		rawClientID = getString(data, "client_id")
	}

	combinedSandboxID := rawSandboxID
	if rawClientID != "" {
		combinedSandboxID = fmt.Sprintf("%s-%s", rawSandboxID, rawClientID)
	}

	// Get templateID (camelCase)
	templateID := getString(data, "templateID")
	if templateID == "" {
		templateID = getString(data, "template_id") // Fallback
	}

	// Get envdVersion (camelCase)
	envdVersion := getString(data, "envdVersion")
	if envdVersion == "" {
		envdVersion = getString(data, "envd_version") // Fallback
	}

	// Get envdAccessToken (camelCase)
	envdAccessToken := getString(data, "envdAccessToken")
	if envdAccessToken == "" {
		envdAccessToken = getString(data, "envd_access_token") // Fallback
	}

	info := &SandboxInfo{
		SandboxID:       combinedSandboxID,
		TemplateID:      templateID,
		EnvdVersion:     envdVersion,
		EnvdAccessToken: envdAccessToken,
		Metadata:        getMapString(data, "metadata"),
	}

	if name, ok := data["alias"].(string); ok {
		info.Name = name
	}

	if startedAt, ok := data["started_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, startedAt); err == nil {
			info.StartedAt = t
		}
	}

	if endAt, ok := data["end_at"].(string); ok && endAt != "" {
		if t, err := time.Parse(time.RFC3339, endAt); err == nil {
			info.EndAt = &t
		}
	}

	return info, nil
}

// Create creates a new sandbox
func (api *sandboxApiImpl) Create(ctx context.Context, opts *CreateSandboxOptions) (*SandboxInfo, error) {
	// Match OpenAPI spec: templateID (camelCase), envVars (camelCase)
	body := map[string]interface{}{
		"templateID": opts.Template,
		"timeout":    opts.Timeout,
		"secure":     opts.Secure,
	}

	if opts.Metadata != nil {
		body["metadata"] = opts.Metadata
	}

	if opts.Envs != nil {
		body["envVars"] = opts.Envs
	}

	resp, err := api.client.Request(ctx, "POST", "/sandboxes", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// OpenAPI spec says POST /sandboxes returns 201 on success
	if resp.StatusCode != 201 {
		if resp.StatusCode >= 300 {
			return nil, HandleAPIException(resp)
		}
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	// API returns camelCase field names (sandboxID, clientID, templateID, etc.)
	// Match Python SDK: combine sandboxID and clientID as "{sandboxID}-{clientID}"
	rawSandboxID := getString(data, "sandboxID")
	rawClientID := getString(data, "clientID")

	// Fallback to snake_case for backward compatibility
	if rawSandboxID == "" {
		rawSandboxID = getString(data, "sandbox_id")
	}
	if rawClientID == "" {
		rawClientID = getString(data, "client_id")
	}

	// Check if we got the required fields
	if rawSandboxID == "" {
		return nil, fmt.Errorf("missing sandboxID in response: %v", data)
	}

	combinedSandboxID := rawSandboxID
	if rawClientID != "" {
		combinedSandboxID = fmt.Sprintf("%s-%s", rawSandboxID, rawClientID)
	}

	// Get templateID (camelCase)
	templateID := getString(data, "templateID")
	if templateID == "" {
		templateID = getString(data, "template_id") // Fallback
	}

	// Get envdVersion (camelCase)
	envdVersion := getString(data, "envdVersion")
	if envdVersion == "" {
		envdVersion = getString(data, "envd_version") // Fallback
	}

	// Get envdAccessToken (camelCase)
	envdAccessToken := getString(data, "envdAccessToken")
	if envdAccessToken == "" {
		envdAccessToken = getString(data, "envd_access_token") // Fallback
	}

	info := &SandboxInfo{
		SandboxID:       combinedSandboxID,
		TemplateID:      templateID,
		EnvdVersion:     envdVersion,
		EnvdAccessToken: envdAccessToken,
		Metadata:        getMapString(data, "metadata"),
	}

	if name, ok := data["alias"].(string); ok {
		info.Name = name
	}

	if startedAt, ok := data["started_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, startedAt); err == nil {
			info.StartedAt = t
		}
	}

	return info, nil
}

// Connect connects to an existing sandbox
func (api *sandboxApiImpl) Connect(ctx context.Context, sandboxID string) (*SandboxInfo, error) {
	return api.GetInfo(ctx, sandboxID)
}

// Kill kills a sandbox
func (api *sandboxApiImpl) Kill(ctx context.Context, sandboxID string) (bool, error) {
	path := fmt.Sprintf("/sandboxes/%s", sandboxID)

	resp, err := api.client.Request(ctx, "DELETE", path, nil)
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
	path := fmt.Sprintf("/sandboxes/%s/timeout", sandboxID)
	body := map[string]interface{}{
		"timeout": timeout,
	}

	resp, err := api.client.Request(ctx, "POST", path, body)
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
	path := fmt.Sprintf("/sandboxes/%s/pause", sandboxID)

	resp, err := api.client.Request(ctx, "POST", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return NewNotFoundException(fmt.Sprintf("Sandbox %s not found", sandboxID), nil)
	}

	if resp.StatusCode == 409 {
		return fmt.Errorf("sandbox is already paused or cannot be paused")
	}

	if resp.StatusCode >= 300 {
		return HandleAPIException(resp)
	}

	return nil
}

// Resume resumes a paused sandbox
func (api *sandboxApiImpl) Resume(ctx context.Context, sandboxID string, timeout *int) (*SandboxInfo, error) {
	path := fmt.Sprintf("/sandboxes/%s/resume", sandboxID)

	// OpenAPI spec says requestBody is required, but Python SDK allows optional timeout
	// We'll always send a body, using default timeout if not provided
	body := map[string]interface{}{
		"timeout": 15, // Default from OpenAPI spec
	}
	if timeout != nil {
		body["timeout"] = *timeout
	}

	resp, err := api.client.Request(ctx, "POST", path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, NewNotFoundException(fmt.Sprintf("Paused sandbox %s not found", sandboxID), nil)
	}

	if resp.StatusCode == 409 {
		return nil, fmt.Errorf("sandbox is already running or cannot be resumed")
	}

	if resp.StatusCode >= 300 {
		return nil, HandleAPIException(resp)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	// API returns camelCase field names (sandboxID, clientID, templateID, etc.)
	// Match Python SDK: combine sandboxID and clientID as "{sandboxID}-{clientID}"
	rawSandboxID := getString(data, "sandboxID")
	rawClientID := getString(data, "clientID")

	// Fallback to snake_case for backward compatibility
	if rawSandboxID == "" {
		rawSandboxID = getString(data, "sandbox_id")
	}
	if rawClientID == "" {
		rawClientID = getString(data, "client_id")
	}

	combinedSandboxID := rawSandboxID
	if rawClientID != "" {
		combinedSandboxID = fmt.Sprintf("%s-%s", rawSandboxID, rawClientID)
	}

	// Get templateID (camelCase)
	templateID := getString(data, "templateID")
	if templateID == "" {
		templateID = getString(data, "template_id") // Fallback
	}

	// Get envdVersion (camelCase)
	envdVersion := getString(data, "envdVersion")
	if envdVersion == "" {
		envdVersion = getString(data, "envd_version") // Fallback
	}

	// Get envdAccessToken (camelCase)
	envdAccessToken := getString(data, "envdAccessToken")
	if envdAccessToken == "" {
		envdAccessToken = getString(data, "envd_access_token") // Fallback
	}

	info := &SandboxInfo{
		SandboxID:       combinedSandboxID,
		TemplateID:      templateID,
		EnvdVersion:     envdVersion,
		EnvdAccessToken: envdAccessToken,
		Metadata:        getMapString(data, "metadata"),
	}

	if name, ok := data["alias"].(string); ok {
		info.Name = name
	}

	if startedAt, ok := data["started_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, startedAt); err == nil {
			info.StartedAt = t
		}
	}

	return info, nil
}

// Helper functions

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getMapString(m map[string]interface{}, key string) map[string]string {
	if v, ok := m[key].(map[string]interface{}); ok {
		result := make(map[string]string)
		for k, val := range v {
			if s, ok := val.(string); ok {
				result[k] = s
			}
		}
		return result
	}
	return make(map[string]string)
}
