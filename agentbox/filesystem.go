package agentbox

import (
	"context"
	"time"
)

// Filesystem provides methods for interacting with the sandbox filesystem
// This matches Python SDK's Filesystem interface
type Filesystem interface {
	// Read reads file content
	Read(ctx context.Context, path string, format ReadFormat, user Username, requestTimeout time.Duration) (interface{}, error)

	// Write writes content to a file
	Write(ctx context.Context, path string, data interface{}, user Username, requestTimeout time.Duration) (*EntryInfo, error)

	// Remove removes a file or directory
	Remove(ctx context.Context, path string, user Username, requestTimeout time.Duration) error

	// List lists files and directories
	List(ctx context.Context, path string, user Username, requestTimeout time.Duration) ([]*EntryInfo, error)

	// Stat gets information about a file or directory
	Stat(ctx context.Context, path string, user Username, requestTimeout time.Duration) (*EntryInfo, error)

	// Exists checks whether a file or directory exists
	Exists(ctx context.Context, path string, user Username, requestTimeout time.Duration) (bool, error)

	// Rename renames a file or directory
	Rename(ctx context.Context, oldPath string, newPath string, user Username, requestTimeout time.Duration) (*EntryInfo, error)

	// MakeDir creates a directory
	MakeDir(ctx context.Context, path string, user Username, requestTimeout time.Duration) (*EntryInfo, error)

	// Watch watches filesystem for changes
	Watch(ctx context.Context, path string, user Username) (WatchHandle, error)
}

// ReadFormat represents the format for reading files
type ReadFormat string

const (
	ReadFormatText   ReadFormat = "text"
	ReadFormatBytes  ReadFormat = "bytes"
	ReadFormatStream ReadFormat = "stream"
)

// WatchHandle represents a handle to a filesystem watch operation
// This matches Python SDK's WatchHandle
type WatchHandle interface {
	// GetNewEvents gets new filesystem events since the last call
	GetNewEvents(ctx context.Context) ([]FilesystemEvent, error)

	// Close stops watching the filesystem
	Close() error
}

// NewFilesystem creates a new filesystem instance
// This is a factory function for creating filesystem implementations
// Implementation is in sandbox_sync/filesystem package
func NewFilesystem(envdAPIURL string, envdVersion string, config *ConnectionConfig) Filesystem {
	// Import and use the implementation from sandbox_sync/filesystem
	// This avoids circular imports
	return newFilesystemImpl(envdAPIURL, envdVersion, config)
}

// newFilesystemImpl is the internal function to create filesystem implementation
// This is a helper to avoid circular imports
func newFilesystemImpl(envdAPIURL string, envdVersion string, config *ConnectionConfig) Filesystem {
	// The actual implementation is in sandbox_sync/filesystem package
	// This function is kept for backward compatibility
	// The real implementation is created directly in sandbox_sync/main.go
	return nil
}
