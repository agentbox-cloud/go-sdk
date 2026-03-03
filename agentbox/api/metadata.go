package api

import (
	"runtime"
)

const (
	// Version is the SDK version
	Version = "1.0.0"
)

// DefaultHeaders returns the default headers for API requests
// This matches Python SDK's default_headers in agentbox/api/metadata.py
func DefaultHeaders() map[string]string {
	return map[string]string{
		"lang":            "go",
		"lang_version":    runtime.Version(),
		"machine":         runtime.GOARCH,
		"os":              runtime.GOOS,
		"package_version": Version,
		"processor":       runtime.GOARCH,
		"publisher":       "agentbox",
		"release":         "", // Go doesn't have release info like Python
		"sdk_runtime":     "go",
		"system":          runtime.GOOS,
	}
}
