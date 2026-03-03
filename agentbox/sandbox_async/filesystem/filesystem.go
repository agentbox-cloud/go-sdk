package filesystem

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/agentbox-cloud/go-sdk/agentbox"
)

const (
	// ENVD_API_FILES_ROUTE is the route for files API
	ENVD_API_FILES_ROUTE = "/files"
)

// filesystemImpl implements agentbox.Filesystem interface for async operations
// This matches Python SDK's Filesystem class in sandbox_async/filesystem/filesystem.py
// Note: In Go, async operations are handled via context.Context and goroutines
type filesystemImpl struct {
	envdAPIURL       string
	envdVersion      string
	connectionConfig *agentbox.ConnectionConfig
	httpClient       *http.Client
	mu               sync.Mutex // For thread safety
}

// NewFilesystem creates a new async filesystem implementation
// This matches Python SDK's Filesystem.__init__()
func NewFilesystem(envdAPIURL string, envdVersion string, config *agentbox.ConnectionConfig) agentbox.Filesystem {
	httpClient := &http.Client{
		Timeout: config.RequestTimeout,
	}

	return &filesystemImpl{
		envdAPIURL:       envdAPIURL,
		envdVersion:      envdVersion,
		connectionConfig: config,
		httpClient:       httpClient,
	}
}

// Read reads file content
// This matches Python SDK's AsyncFilesystem.read()
func (f *filesystemImpl) Read(ctx context.Context, path string, format agentbox.ReadFormat, user agentbox.Username, requestTimeout time.Duration) (interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	fileURL := f.envdAPIURL + ENVD_API_FILES_ROUTE
	params := url.Values{}
	params.Set("path", path)
	// Use default username if not provided (matching Python SDK's default_username = "user")
	if user == "" {
		user = agentbox.DefaultUsername
	}
	params.Set("username", string(user))
	if format == agentbox.ReadFormatText {
		params.Set("format", "text")
	} else if format == agentbox.ReadFormatBytes {
		params.Set("format", "bytes")
	}

	reqURL := fileURL + "?" + params.Encode()

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	// Set headers
	headers := agentbox.AuthenticationHeader(user)
	if f.connectionConfig.AccessToken != "" {
		headers["X-Access-Token"] = f.connectionConfig.AccessToken
	}
	for k, v := range f.connectionConfig.Headers {
		headers[k] = v
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Set timeout
	timeout := f.connectionConfig.GetRequestTimeout(requestTimeout)
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	// Make request
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Handle errors
	if resp.StatusCode >= 400 {
		return nil, agentbox.HandleEnvdAPIException(resp)
	}

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse response based on format
	if format == agentbox.ReadFormatText {
		return string(body), nil
	} else if format == agentbox.ReadFormatBytes {
		return body, nil
	}

	return string(body), nil
}

// Write writes data to a file
// This matches Python SDK's AsyncFilesystem.write()
func (f *filesystemImpl) Write(ctx context.Context, path string, data interface{}, user agentbox.Username, requestTimeout time.Duration) (*agentbox.EntryInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Use default username if not provided (matching Python SDK's default_username = "user")
	if user == "" {
		user = agentbox.DefaultUsername
	}

	// Prepare multipart form
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add file data
	var part io.Writer
	var err error
	if strData, ok := data.(string); ok {
		part, err = writer.CreateFormFile("file", path)
		if err != nil {
			return nil, err
		}
		_, err = part.Write([]byte(strData))
		if err != nil {
			return nil, err
		}
	} else if byteData, ok := data.([]byte); ok {
		part, err = writer.CreateFormFile("file", path)
		if err != nil {
			return nil, err
		}
		_, err = part.Write(byteData)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("unsupported data type: %T", data)
	}

	writer.Close()

	// Create request
	// Add username and path as query parameters (matching Python SDK)
	// Python SDK sends params={"username": user, "path": path} as query string
	fileURL := f.envdAPIURL + ENVD_API_FILES_ROUTE
	params := url.Values{}
	params.Set("username", string(user))
	params.Set("path", path)

	req, err := http.NewRequestWithContext(ctx, "POST", fileURL+"?"+params.Encode(), &body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Add auth header if available
	if f.connectionConfig.AccessToken != "" {
		req.Header.Set("X-Access-Token", f.connectionConfig.AccessToken)
	}

	// Add custom headers
	for k, v := range f.connectionConfig.Headers {
		req.Header.Set(k, v)
	}

	// Set timeout
	timeout := f.connectionConfig.GetRequestTimeout(requestTimeout)
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	// Make request
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := agentbox.HandleEnvdAPIException(resp); err != nil {
		return nil, err
	}

	// Parse response
	// The API returns an array of file information, we need to extract the first element
	var writeFiles []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&writeFiles); err != nil {
		return nil, err
	}

	if len(writeFiles) == 0 {
		return nil, agentbox.NewSandboxException(
			"Expected to receive information about written file",
			nil,
		)
	}

	// Return first file info
	fileInfo := writeFiles[0]
	return parseEntryInfo(fileInfo), nil
}

// parseEntryInfo parses EntryInfo from a map
func parseEntryInfo(data map[string]interface{}) *agentbox.EntryInfo {
	info := &agentbox.EntryInfo{}

	if name, ok := data["name"].(string); ok {
		info.Name = name
	}

	if path, ok := data["path"].(string); ok {
		info.Path = path
	}

	if typ, ok := data["type"].(string); ok {
		info.Type = agentbox.FileType(typ)
	}

	if size, ok := data["size"].(float64); ok {
		info.Size = int64(size)
	}

	if modified, ok := data["modified"].(string); ok {
		if t, err := time.Parse(time.RFC3339, modified); err == nil {
			info.Modified = t
		}
	}

	return info
}

// Remove removes a file or directory
// This matches Python SDK's AsyncFilesystem.remove()
func (f *filesystemImpl) Remove(ctx context.Context, path string, user agentbox.Username, requestTimeout time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Prepare RPC request
	req := map[string]interface{}{
		"path": path,
	}

	// Prepare response (empty for Remove)
	var resp struct{}

	// Get timeout
	timeout := f.connectionConfig.GetRequestTimeout(requestTimeout)

	// Use default username if not provided (matching Python SDK's default_username = "user")
	if user == "" {
		user = agentbox.DefaultUsername
	}

	// Get auth headers
	headers := agentbox.AuthenticationHeader(user)
	if f.connectionConfig.AccessToken != "" {
		headers["X-Access-Token"] = f.connectionConfig.AccessToken
	}
	for k, v := range f.connectionConfig.Headers {
		headers[k] = v
	}

	// Make RPC call
	err := agentbox.CallRPC(
		ctx,
		f.envdAPIURL,
		agentbox.FilesystemServiceName,
		"Remove",
		req,
		&resp,
		headers,
		timeout,
		f.httpClient,
	)
	if err != nil {
		return agentbox.HandleRPCException(err)
	}

	return nil
}

// List lists files and directories
// This matches Python SDK's AsyncFilesystem.list()
func (f *filesystemImpl) List(ctx context.Context, path string, user agentbox.Username, requestTimeout time.Duration) ([]*agentbox.EntryInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Validate depth (default to 1, matching Python SDK)
	depth := 1

	// Prepare RPC request
	req := map[string]interface{}{
		"path":  path,
		"depth": depth,
	}

	// Prepare response
	var resp struct {
		Entries []map[string]interface{} `json:"entries"`
	}

	// Get timeout
	timeout := f.connectionConfig.GetRequestTimeout(requestTimeout)

	// Use default username if not provided (matching Python SDK's default_username = "user")
	if user == "" {
		user = agentbox.DefaultUsername
	}

	// Get auth headers
	headers := agentbox.AuthenticationHeader(user)
	if f.connectionConfig.AccessToken != "" {
		headers["X-Access-Token"] = f.connectionConfig.AccessToken
	}
	for k, v := range f.connectionConfig.Headers {
		headers[k] = v
	}

	// Make RPC call
	err := agentbox.CallRPC(
		ctx,
		f.envdAPIURL,
		agentbox.FilesystemServiceName,
		"ListDir",
		req,
		&resp,
		headers,
		timeout,
		f.httpClient,
	)
	if err != nil {
		return nil, agentbox.HandleRPCException(err)
	}

	// Convert entries
	entries := make([]*agentbox.EntryInfo, 0, len(resp.Entries))
	for _, entryData := range resp.Entries {
		entry := parseRPCEntryInfo(entryData)
		if entry != nil {
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

// parseRPCEntryInfo parses EntryInfo from RPC response
// This matches Python SDK's map_file_type() and EntryInfo conversion
func parseRPCEntryInfo(data map[string]interface{}) *agentbox.EntryInfo {
	if data == nil {
		return nil
	}

	info := &agentbox.EntryInfo{}

	if name, ok := data["name"].(string); ok {
		info.Name = name
	}

	if path, ok := data["path"].(string); ok {
		info.Path = path
	}

	// Map RPC file type to Go FileType
	// RPC uses enum: FILE_TYPE_FILE=1, FILE_TYPE_DIRECTORY=2
	if typeVal, ok := data["type"].(float64); ok {
		switch int(typeVal) {
		case 1: // FILE_TYPE_FILE
			info.Type = agentbox.FileTypeFile
		case 2: // FILE_TYPE_DIRECTORY
			info.Type = agentbox.FileTypeDirectory
		default:
			info.Type = agentbox.FileTypeFile // Default
		}
	} else if typeStr, ok := data["type"].(string); ok {
		// Also support string type (for compatibility)
		switch typeStr {
		case "file", "FILE":
			info.Type = agentbox.FileTypeFile
		case "directory", "DIRECTORY":
			info.Type = agentbox.FileTypeDirectory
		default:
			info.Type = agentbox.FileTypeFile
		}
	}

	if size, ok := data["size"].(float64); ok {
		info.Size = int64(size)
	}

	// Parse modified_time (protobuf timestamp format)
	if modifiedTime, ok := data["modified_time"].(map[string]interface{}); ok {
		if seconds, ok := modifiedTime["seconds"].(float64); ok {
			info.Modified = time.Unix(int64(seconds), 0)
		}
	} else if modified, ok := data["modified"].(string); ok {
		// Fallback to string format
		if t, err := time.Parse(time.RFC3339, modified); err == nil {
			info.Modified = t
		}
	}

	return info
}

// Stat gets file or directory information
// This matches Python SDK's AsyncFilesystem.stat()
func (f *filesystemImpl) Stat(ctx context.Context, path string, user agentbox.Username, requestTimeout time.Duration) (*agentbox.EntryInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Prepare RPC request
	req := map[string]interface{}{
		"path": path,
	}

	// Prepare response
	var resp struct {
		Entry map[string]interface{} `json:"entry"`
	}

	// Get timeout
	timeout := f.connectionConfig.GetRequestTimeout(requestTimeout)

	// Use default username if not provided (matching Python SDK's default_username = "user")
	if user == "" {
		user = agentbox.DefaultUsername
	}

	// Get auth headers
	headers := agentbox.AuthenticationHeader(user)
	if f.connectionConfig.AccessToken != "" {
		headers["X-Access-Token"] = f.connectionConfig.AccessToken
	}
	for k, v := range f.connectionConfig.Headers {
		headers[k] = v
	}

	// Make RPC call
	err := agentbox.CallRPC(
		ctx,
		f.envdAPIURL,
		agentbox.FilesystemServiceName,
		"Stat",
		req,
		&resp,
		headers,
		timeout,
		f.httpClient,
	)
	if err != nil {
		return nil, agentbox.HandleRPCException(err)
	}

	return parseRPCEntryInfo(resp.Entry), nil
}

// Exists checks whether a file or directory exists
// This matches Python SDK's AsyncFilesystem.exists()
func (f *filesystemImpl) Exists(ctx context.Context, path string, user agentbox.Username, requestTimeout time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Use Stat RPC to check if file exists
	req := map[string]interface{}{
		"path": path,
	}

	var resp struct {
		Entry map[string]interface{} `json:"entry"`
	}

	timeout := f.connectionConfig.GetRequestTimeout(requestTimeout)

	// Use default username if not provided
	if user == "" {
		user = agentbox.DefaultUsername
	}

	headers := agentbox.AuthenticationHeader(user)
	if f.connectionConfig.AccessToken != "" {
		headers["X-Access-Token"] = f.connectionConfig.AccessToken
	}
	for k, v := range f.connectionConfig.Headers {
		headers[k] = v
	}

	err := agentbox.CallRPC(
		ctx,
		f.envdAPIURL,
		agentbox.FilesystemServiceName,
		"Stat",
		req,
		&resp,
		headers,
		timeout,
		f.httpClient,
	)
	if err != nil {
		// Check if it's a not_found error
		if httpErr, ok := err.(*agentbox.HTTPRPCError); ok && httpErr.Code == "not_found" {
			return false, nil
		}
		// Handle other RPC errors
		rpcErr := agentbox.HandleRPCException(err)
		if _, ok := rpcErr.(*agentbox.NotFoundException); ok {
			return false, nil
		}
		return false, rpcErr
	}
	return true, nil
}

// Rename renames a file or directory
// This matches Python SDK's AsyncFilesystem.rename()
func (f *filesystemImpl) Rename(ctx context.Context, oldPath string, newPath string, user agentbox.Username, requestTimeout time.Duration) (*agentbox.EntryInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Prepare RPC request
	req := map[string]interface{}{
		"source":      oldPath,
		"destination": newPath,
	}

	// Prepare response
	var resp struct {
		Entry map[string]interface{} `json:"entry"`
	}

	// Get timeout
	timeout := f.connectionConfig.GetRequestTimeout(requestTimeout)

	// Use default username if not provided (matching Python SDK's default_username = "user")
	if user == "" {
		user = agentbox.DefaultUsername
	}

	// Get auth headers
	headers := agentbox.AuthenticationHeader(user)
	if f.connectionConfig.AccessToken != "" {
		headers["X-Access-Token"] = f.connectionConfig.AccessToken
	}
	for k, v := range f.connectionConfig.Headers {
		headers[k] = v
	}

	// Make RPC call
	err := agentbox.CallRPC(
		ctx,
		f.envdAPIURL,
		agentbox.FilesystemServiceName,
		"Move",
		req,
		&resp,
		headers,
		timeout,
		f.httpClient,
	)
	if err != nil {
		return nil, agentbox.HandleRPCException(err)
	}

	return parseRPCEntryInfo(resp.Entry), nil
}

// MakeDir creates a directory
// This matches Python SDK's AsyncFilesystem.make_dir()
func (f *filesystemImpl) MakeDir(ctx context.Context, path string, user agentbox.Username, requestTimeout time.Duration) (*agentbox.EntryInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Prepare RPC request
	req := map[string]interface{}{
		"path": path,
	}

	// Prepare response
	var resp struct {
		Entry map[string]interface{} `json:"entry"`
	}

	// Get timeout
	timeout := f.connectionConfig.GetRequestTimeout(requestTimeout)

	// Use default username if not provided (matching Python SDK's default_username = "user")
	if user == "" {
		user = agentbox.DefaultUsername
	}

	// Get auth headers
	headers := agentbox.AuthenticationHeader(user)
	if f.connectionConfig.AccessToken != "" {
		headers["X-Access-Token"] = f.connectionConfig.AccessToken
	}
	for k, v := range f.connectionConfig.Headers {
		headers[k] = v
	}

	// Make RPC call
	err := agentbox.CallRPC(
		ctx,
		f.envdAPIURL,
		agentbox.FilesystemServiceName,
		"MakeDir",
		req,
		&resp,
		headers,
		timeout,
		f.httpClient,
	)
	if err != nil {
		return nil, agentbox.HandleRPCException(err)
	}

	return parseRPCEntryInfo(resp.Entry), nil
}

// Watch watches a directory for changes
// This matches Python SDK's AsyncFilesystem.watch_dir()
func (f *filesystemImpl) Watch(ctx context.Context, path string, user agentbox.Username) (agentbox.WatchHandle, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Prepare RPC request (recursive defaults to false, matching Python SDK)
	req := map[string]interface{}{
		"path":      path,
		"recursive": false,
	}

	// Prepare response
	// Note: Connect Protocol JSON encoding may use camelCase (watcherId) or snake_case (watcher_id)
	// We need to support both formats
	var respData map[string]interface{}

	// Get timeout (use default request timeout)
	timeout := f.connectionConfig.GetRequestTimeout(0)

	// Use default username if not provided (matching Python SDK's default_username = "user")
	if user == "" {
		user = agentbox.DefaultUsername
	}

	// Get auth headers
	headers := agentbox.AuthenticationHeader(user)
	if f.connectionConfig.AccessToken != "" {
		headers["X-Access-Token"] = f.connectionConfig.AccessToken
	}
	// Add keepalive ping header (matching Python SDK)
	headers[agentbox.KeepalivePingHeader] = fmt.Sprintf("%d", agentbox.KeepalivePingIntervalSec)
	for k, v := range f.connectionConfig.Headers {
		headers[k] = v
	}

	// Make RPC call - parse as map first to handle both camelCase and snake_case
	err := agentbox.CallRPC(
		ctx,
		f.envdAPIURL,
		agentbox.FilesystemServiceName,
		"CreateWatcher",
		req,
		&respData,
		headers,
		timeout,
		f.httpClient,
	)
	if err != nil {
		return nil, agentbox.HandleRPCException(err)
	}

	// Extract watcher_id from response (support both camelCase and snake_case)
	watcherID := ""
	if id, ok := respData["watcherId"].(string); ok {
		watcherID = id
	} else if id, ok := respData["watcher_id"].(string); ok {
		watcherID = id
	}

	if watcherID == "" {
		return nil, agentbox.NewSandboxException(
			"CreateWatcher response missing watcher_id",
			nil,
		)
	}

	return NewWatchHandle(watcherID, f.envdAPIURL, f.connectionConfig, f.httpClient, user), nil
}
