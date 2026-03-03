package agentbox

import (
	"context"
	"time"
)

// NOTE:
// Root package contracts are kept for backward compatibility.
// Structured package entry (Python SDK aligned) is:
//   github.com/agentbox-cloud/go-sdk/agentbox/sandbox/filesystem

// Filesystem provides methods for interacting with the sandbox filesystem
type Filesystem interface {
	// Read reads file content
	Read(ctx context.Context, path string, format ReadFormat, user Username, requestTimeout time.Duration) (interface{}, error)

	// Write writes content to a file
	Write(ctx context.Context, path string, data interface{}, user Username, requestTimeout time.Duration) (*EntryInfo, error)

	// Remove removes a file or directory
	Remove(ctx context.Context, path string, user Username, requestTimeout time.Duration) error

	// List lists files and directories
	List(ctx context.Context, path string, user Username, requestTimeout time.Duration) ([]*EntryInfo, error)

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
