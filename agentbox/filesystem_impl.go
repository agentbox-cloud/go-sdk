package agentbox

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	envdconnect "github.com/agentbox-cloud/go-sdk/agentbox/envd/connect"
)

const (
	ENVD_API_FILES_ROUTE = "/files"
)

// filesystemImpl implements Filesystem
type filesystemImpl struct {
	envdAPIURL       string
	envdVersion      string
	connectionConfig *ConnectionConfig
}

type watchHandleImpl struct {
	ctx     context.Context
	cancel  context.CancelFunc
	closeFn func() error
}

func (w *watchHandleImpl) Close() error {
	w.cancel()
	if w.closeFn != nil {
		return w.closeFn()
	}
	return nil
}

// NewFilesystem creates a new Filesystem instance
func NewFilesystem(envdAPIURL string, envdVersion string, config *ConnectionConfig) Filesystem {
	return &filesystemImpl{
		envdAPIURL:       envdAPIURL,
		envdVersion:      envdVersion,
		connectionConfig: config,
	}
}

// Read reads file content
func (f *filesystemImpl) Read(ctx context.Context, path string, format ReadFormat, user Username, requestTimeout time.Duration) (interface{}, error) {
	fileURL := f.envdAPIURL + ENVD_API_FILES_ROUTE
	params := url.Values{}
	params.Set("path", path)
	params.Set("username", string(user))

	reqURL := fileURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	// Add auth header if available (would be set via connection config)
	timeout := f.connectionConfig.GetRequestTimeout(requestTimeout)
	req.Header.Set("X-Access-Token", f.connectionConfig.AccessToken)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := HandleEnvdAPIException(resp); err != nil {
		return nil, err
	}

	switch format {
	case ReadFormatText:
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return string(data), nil
	case ReadFormatBytes:
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return data, nil
	case ReadFormatStream:
		return resp.Body, nil
	default:
		return nil, fmt.Errorf("unsupported read format: %s", format)
	}
}

// Write writes content to a file
func (f *filesystemImpl) Write(ctx context.Context, path string, data interface{}, user Username, requestTimeout time.Duration) (*EntryInfo, error) {
	fileURL := f.envdAPIURL + ENVD_API_FILES_ROUTE

	// Prepare multipart form data
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add file
	var fileData []byte
	switch v := data.(type) {
	case string:
		fileData = []byte(v)
	case []byte:
		fileData = v
	case io.Reader:
		var err error
		fileData, err = io.ReadAll(v)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported data type: %T", data)
	}

	// Create form file with path as filename (matching Python SDK's httpx files format)
	// Python SDK uses: ("file", (file_path, file_data))
	// In Go multipart, the filename in CreateFormFile should be the file path
	part, err := writer.CreateFormFile("file", path)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(fileData); err != nil {
		return nil, err
	}

	// Note: username and path are sent as query parameters, not form fields
	// (see below where we set req.URL.RawQuery)

	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fileURL, &body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Add authentication header
	if f.connectionConfig.AccessToken != "" {
		req.Header.Set("X-Access-Token", f.connectionConfig.AccessToken)
	}

	// Add username and path as query parameters (matching Python SDK)
	// Python SDK sends params={"username": user, "path": path} as query string
	params := url.Values{}
	params.Set("username", string(user))
	params.Set("path", path)
	req.URL.RawQuery = params.Encode()

	timeout := f.connectionConfig.GetRequestTimeout(requestTimeout)
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := HandleEnvdAPIException(resp); err != nil {
		return nil, err
	}

	var files []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("expected to receive information about written file")
	}

	file := files[0]
	entry := &EntryInfo{
		Name: getStringFromMap(file, "name"),
		Path: getStringFromMap(file, "path"),
	}

	if fileTypeStr, ok := file["type"].(string); ok {
		entry.Type = FileType(fileTypeStr)
	}

	if size, ok := file["size"].(float64); ok {
		entry.Size = int64(size)
	}

	if modified, ok := file["modified"].(string); ok {
		if t, err := time.Parse(time.RFC3339, modified); err == nil {
			entry.Modified = t
		}
	}

	return entry, nil
}

// getStringFromMap extracts a string value from a map
func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// Remove removes a file or directory
func (f *filesystemImpl) Remove(ctx context.Context, path string, user Username, requestTimeout time.Duration) error {
	timeout := f.connectionConfig.GetRequestTimeout(requestTimeout)
	headers := map[string]string{
		"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(string(user)+":")),
	}
	client := envdconnect.NewClient(
		f.envdAPIURL+"/filesystem.Filesystem",
		&envdconnect.JSONCodec{},
		mergeHeaders(f.connectionConfig.Headers, headers),
		timeout,
	)

	_, err := client.CallUnary(ctx, "Remove", map[string]interface{}{
		"path": path,
	})
	return err
}

// List lists files and directories
func (f *filesystemImpl) List(ctx context.Context, path string, user Username, requestTimeout time.Duration) ([]*EntryInfo, error) {
	timeout := f.connectionConfig.GetRequestTimeout(requestTimeout)
	headers := map[string]string{
		"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(string(user)+":")),
	}
	client := envdconnect.NewClient(
		f.envdAPIURL+"/filesystem.Filesystem",
		&envdconnect.JSONCodec{},
		mergeHeaders(f.connectionConfig.Headers, headers),
		timeout,
	)

	resp, err := client.CallUnary(ctx, "ListDir", map[string]interface{}{
		"path":  path,
		"depth": 1,
	})
	if err != nil {
		return nil, err
	}

	respMap, ok := resp.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid list response type: %T", resp)
	}

	entriesRaw, ok := respMap["entries"].([]interface{})
	if !ok {
		return []*EntryInfo{}, nil
	}

	entries := make([]*EntryInfo, 0, len(entriesRaw))
	for _, entryRaw := range entriesRaw {
		entryMap, ok := entryRaw.(map[string]interface{})
		if !ok {
			continue
		}
		entry := &EntryInfo{
			Name: getStringFromMap(entryMap, "name"),
			Path: getStringFromMap(entryMap, "path"),
		}

		switch t := entryMap["type"].(type) {
		case string:
			switch strings.ToUpper(t) {
			case "FILE_TYPE_FILE", "FILE":
				entry.Type = FileTypeFile
			case "FILE_TYPE_DIRECTORY", "DIRECTORY", "DIR":
				entry.Type = FileTypeDirectory
			default:
				entry.Type = FileType(strings.ToLower(t))
			}
		case float64:
			if int(t) == 1 {
				entry.Type = FileTypeFile
			} else if int(t) == 2 {
				entry.Type = FileTypeDirectory
			}
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// MakeDir creates a directory
func (f *filesystemImpl) MakeDir(ctx context.Context, path string, user Username, requestTimeout time.Duration) (*EntryInfo, error) {
	timeout := f.connectionConfig.GetRequestTimeout(requestTimeout)
	headers := map[string]string{
		"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(string(user)+":")),
	}
	client := envdconnect.NewClient(
		f.envdAPIURL+"/filesystem.Filesystem",
		&envdconnect.JSONCodec{},
		mergeHeaders(f.connectionConfig.Headers, headers),
		timeout,
	)

	resp, err := client.CallUnary(ctx, "MakeDir", map[string]interface{}{
		"path": path,
	})
	if err != nil {
		// Keep behavior practical: if directory already exists, still return directory info.
		if strings.Contains(strings.ToLower(err.Error()), "already_exists") {
			return &EntryInfo{
				Name: path[strings.LastIndex(path, "/")+1:],
				Path: path,
				Type: FileTypeDirectory,
			}, nil
		}
		return nil, err
	}

	respMap, ok := resp.(map[string]interface{})
	if !ok {
		return &EntryInfo{
			Name: path[strings.LastIndex(path, "/")+1:],
			Path: path,
			Type: FileTypeDirectory,
		}, nil
	}

	entryMap, ok := respMap["entry"].(map[string]interface{})
	if !ok {
		return &EntryInfo{
			Name: path[strings.LastIndex(path, "/")+1:],
			Path: path,
			Type: FileTypeDirectory,
		}, nil
	}

	entry := &EntryInfo{
		Name: getStringFromMap(entryMap, "name"),
		Path: getStringFromMap(entryMap, "path"),
		Type: FileTypeDirectory,
	}
	return entry, nil
}

// Watch watches filesystem for changes
func (f *filesystemImpl) Watch(ctx context.Context, path string, user Username) (WatchHandle, error) {
	timeout := f.connectionConfig.GetRequestTimeout(0)
	headers := map[string]string{
		"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(string(user)+":")),
	}
	client := envdconnect.NewClient(
		f.envdAPIURL+"/filesystem.Filesystem",
		&envdconnect.JSONCodec{},
		mergeHeaders(f.connectionConfig.Headers, headers),
		timeout,
	)

	createResp, err := client.CallUnary(ctx, "CreateWatcher", map[string]interface{}{
		"path":      path,
		"recursive": false,
	})
	if err != nil {
		return nil, err
	}

	respMap, ok := createResp.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid create watcher response type: %T", createResp)
	}

	watcherID := getString(respMap, "watcherId")
	if watcherID == "" {
		watcherID = getString(respMap, "watcher_id")
	}
	if watcherID == "" {
		return nil, fmt.Errorf("missing watcher_id in CreateWatcher response")
	}

	watchCtx, cancel := context.WithCancel(ctx)
	handle := &watchHandleImpl{
		ctx:    watchCtx,
		cancel: cancel,
		closeFn: func() error {
			_, removeErr := client.CallUnary(context.Background(), "RemoveWatcher", map[string]interface{}{
				"watcherId": watcherID,
			})
			if removeErr != nil && strings.Contains(strings.ToLower(removeErr.Error()), "invalid_argument") {
				_, removeErr = client.CallUnary(context.Background(), "RemoveWatcher", map[string]interface{}{
					"watcher_id": watcherID,
				})
			}
			return removeErr
		},
	}
	return handle, nil
}
