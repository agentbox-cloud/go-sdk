package filesystem

import (
	"context"
	"net/http"
	"sync"

	"github.com/agentbox-cloud/go-sdk/agentbox"
)

// watchHandleImpl implements agentbox.WatchHandle interface for async operations
// This matches Python SDK's AsyncWatchHandle class in sandbox_async/filesystem/watch_handle.py
// Note: In Go, async operations are handled via context.Context and goroutines
type watchHandleImpl struct {
	watcherID        string
	envdAPIURL       string
	connectionConfig *agentbox.ConnectionConfig
	httpClient       *http.Client
	user             agentbox.Username // Store user for authentication
	closed           bool
	mu               sync.Mutex
}

// NewWatchHandle creates a new async watch handle
func NewWatchHandle(watcherID string, envdAPIURL string, config *agentbox.ConnectionConfig, httpClient *http.Client, user agentbox.Username) agentbox.WatchHandle {
	return &watchHandleImpl{
		watcherID:        watcherID,
		envdAPIURL:       envdAPIURL,
		connectionConfig: config,
		httpClient:       httpClient,
		user:             user,
		closed:           false,
	}
}

// GetNewEvents gets new filesystem events since the last call
// This matches Python SDK's AsyncWatchHandle.get_new_events()
func (w *watchHandleImpl) GetNewEvents(ctx context.Context) ([]agentbox.FilesystemEvent, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil, agentbox.NewSandboxException(
			"The watcher is already stopped",
			nil,
		)
	}

	// Prepare RPC request
	req := map[string]interface{}{
		"watcher_id": w.watcherID,
	}

	// Prepare response
	var resp struct {
		Events []map[string]interface{} `json:"events"`
	}

	// Get timeout
	timeout := w.connectionConfig.GetRequestTimeout(0)

	// Get auth headers (matching Python SDK - RPC client includes authentication)
	headers := agentbox.AuthenticationHeader(w.user)
	if w.connectionConfig.AccessToken != "" {
		headers["X-Access-Token"] = w.connectionConfig.AccessToken
	}
	for k, v := range w.connectionConfig.Headers {
		headers[k] = v
	}

	// Make RPC call
	err := agentbox.CallRPC(
		ctx,
		w.envdAPIURL,
		agentbox.FilesystemServiceName,
		"GetWatcherEvents",
		req,
		&resp,
		headers,
		timeout,
		w.httpClient,
	)
	if err != nil {
		return nil, agentbox.HandleRPCException(err)
	}

	// Convert events
	events := make([]agentbox.FilesystemEvent, 0, len(resp.Events))
	for _, eventData := range resp.Events {
		event := parseRPCFilesystemEvent(eventData)
		if event != nil {
			events = append(events, *event)
		}
		// Note: Events that return nil from parseRPCFilesystemEvent are filtered out
		// This is expected behavior for unknown event types
	}

	return events, nil
}

// Close stops watching the filesystem
// This matches Python SDK's AsyncWatchHandle.stop()
func (w *watchHandleImpl) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	// Prepare RPC request
	req := map[string]interface{}{
		"watcher_id": w.watcherID,
	}

	// Prepare response (empty for RemoveWatcher)
	var resp struct{}

	// Get timeout
	timeout := w.connectionConfig.GetRequestTimeout(0)

	// Get auth headers (matching Python SDK - RPC client includes authentication)
	headers := agentbox.AuthenticationHeader(w.user)
	if w.connectionConfig.AccessToken != "" {
		headers["X-Access-Token"] = w.connectionConfig.AccessToken
	}
	for k, v := range w.connectionConfig.Headers {
		headers[k] = v
	}

	// Make RPC call
	err := agentbox.CallRPC(
		context.Background(),
		w.envdAPIURL,
		agentbox.FilesystemServiceName,
		"RemoveWatcher",
		req,
		&resp,
		headers,
		timeout,
		w.httpClient,
	)
	if err != nil {
		// Even if RPC fails, mark as closed
		w.closed = true
		return agentbox.HandleRPCException(err)
	}

	w.closed = true
	return nil
}

// parseRPCFilesystemEvent parses FilesystemEvent from RPC response
// This matches Python SDK's map_event_type() and FilesystemEvent conversion
func parseRPCFilesystemEvent(data map[string]interface{}) *agentbox.FilesystemEvent {
	if data == nil {
		return nil
	}

	event := &agentbox.FilesystemEvent{}

	if name, ok := data["name"].(string); ok {
		event.Name = name
	}

	if path, ok := data["path"].(string); ok {
		event.Path = path
	}

	// Map RPC event type to FilesystemEventType
	// RPC uses enum or string
	if typeVal, ok := data["type"].(float64); ok {
		// Numeric enum (matching protobuf enum values)
		switch int(typeVal) {
		case 1: // EVENT_TYPE_CREATE
			event.Type = agentbox.FilesystemEventTypeCreate
		case 2: // EVENT_TYPE_WRITE
			event.Type = agentbox.FilesystemEventTypeModify // Map WRITE to MODIFY
		case 3: // EVENT_TYPE_REMOVE
			event.Type = agentbox.FilesystemEventTypeDelete
		case 4: // EVENT_TYPE_RENAME
			event.Type = agentbox.FilesystemEventTypeModify // Map RENAME to MODIFY
		case 5: // EVENT_TYPE_CHMOD
			event.Type = agentbox.FilesystemEventTypeModify // Map CHMOD to MODIFY
		default:
			return nil // Unknown type, skip
		}
	} else if typeStr, ok := data["type"].(string); ok {
		// String enum
		switch typeStr {
		case "create", "CREATE":
			event.Type = agentbox.FilesystemEventTypeCreate
		case "write", "WRITE":
			event.Type = agentbox.FilesystemEventTypeModify
		case "remove", "REMOVE":
			event.Type = agentbox.FilesystemEventTypeDelete
		case "rename", "RENAME":
			event.Type = agentbox.FilesystemEventTypeModify
		case "chmod", "CHMOD":
			event.Type = agentbox.FilesystemEventTypeModify
		default:
			return nil // Unknown type, skip
		}
	} else {
		return nil // No type, skip
	}

	return event
}

