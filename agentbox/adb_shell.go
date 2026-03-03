package agentbox

import (
	"context"
	"time"
)

// ADBShell provides methods for interacting with Android devices via ADB
// This matches Python SDK's ADBShell interface
type ADBShell interface {
	// Connect connects to the ADB device
	Connect(ctx context.Context) error

	// Shell executes a shell command
	Shell(ctx context.Context, command string, timeout *time.Duration) (string, error)

	// Push pushes a file to the device
	Push(ctx context.Context, local string, remote string) error

	// Pull pulls a file from the device
	Pull(ctx context.Context, remote string, local string) error

	// Exists checks if a path exists
	Exists(ctx context.Context, path string) (bool, error)

	// Remove removes a file or directory
	Remove(ctx context.Context, path string) error

	// Rename renames a file or directory
	Rename(ctx context.Context, src string, dst string) error

	// MakeDir creates a directory
	MakeDir(ctx context.Context, path string) error

	// Install installs an APK
	Install(ctx context.Context, apkPath string, reinstall bool) error

	// Uninstall uninstalls a package
	Uninstall(ctx context.Context, packageName string) error

	// Close closes the ADB connection
	Close() error
}

// NewADBShell creates a new ADB shell instance
// This matches Python SDK's ADBShell.__init__()
// Note: This is a factory function that creates the implementation from sandbox_sync/adb_shell
// The actual implementation is in sandbox_sync/adb_shell package
func NewADBShell(sandboxID string, config *ConnectionConfig) ADBShell {
	// Import and use the implementation from sandbox_sync/adb_shell
	// For now, return nil as placeholder
	// TODO: Import and return sandbox_sync/adb_shell.NewADBShell()
	return nil
}
