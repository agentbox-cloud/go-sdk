package adb_shell

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/agentbox-cloud/go-sdk/agentbox"

	goadb "github.com/zach-klippenstein/goadb"
)

// This package implements async ADB Shell functionality
// This matches Python SDK's agentbox/sandbox_async/adb_shell/adb_shell.py
// Note: In Go, async operations are handled via context.Context and goroutines

// ADBShell represents an async ADB shell connection
// This matches Python SDK's ADBShell class in sandbox_async/adb_shell/adb_shell.py
type ADBShell struct {
	connectionConfig *agentbox.ConnectionConfig
	sandboxID        string
	host             string
	port             int
	authTimeoutS     float64
	device           *goadb.Device
	active           bool
}

// NewADBShell creates a new async ADB shell instance
// This matches Python SDK's ADBShell.__init__()
// Returns ADBShell interface to match agentbox.ADBShell
func NewADBShell(connectionConfig *agentbox.ConnectionConfig, sandboxID string, host string, port int, rsaKeyPath string, authTimeoutS float64) agentbox.ADBShell {
	if authTimeoutS == 0 {
		authTimeoutS = 3.0
	}
	return &ADBShell{
		connectionConfig: connectionConfig,
		sandboxID:        sandboxID,
		host:             host,
		port:             port,
		authTimeoutS:     authTimeoutS,
		active:           false,
	}
}

// Connect connects to the ADB device
// This matches Python SDK's AsyncADBShell.connect()
func (a *ADBShell) Connect(ctx context.Context) error {
	if a.active {
		return nil
	}

	// Get ADB public info if not already set
	if a.host == "" || a.port == 0 {
		if err := a.getADBPublicInfo(ctx); err != nil {
			return err
		}
		time.Sleep(1 * time.Second) // Match Python SDK delay
	}

	// Create ADB client
	adb, err := goadb.New()
	if err != nil {
		return fmt.Errorf("failed to create ADB client: %w", err)
	}

	// Connect to device
	device := adb.Device(goadb.DeviceWithSerial(fmt.Sprintf("%s:%d", a.host, a.port)))

	// Verify device is available
	state, err := device.State()
	if err != nil {
		return fmt.Errorf("failed to get device state: %w", err)
	}

	if state != goadb.StateOnline {
		return fmt.Errorf("ADB device not available, state: %v", state)
	}

	a.device = device
	a.active = true
	return nil
}

// Shell executes a shell command
// This matches Python SDK's AsyncADBShell.shell()
func (a *ADBShell) Shell(ctx context.Context, command string, timeout *time.Duration) (string, error) {
	if !a.active || a.device == nil {
		// Try to reconnect
		if err := a.Connect(ctx); err != nil {
			return "", err
		}
	}

	// Execute command
	result, err := a.device.RunCommand(command)
	if err != nil {
		// Try to reconnect on error
		if err := a.Connect(ctx); err != nil {
			return "", err
		}
		result, err = a.device.RunCommand(command)
		if err != nil {
			return "", fmt.Errorf("failed to execute shell command: %w", err)
		}
	}

	return result, nil
}

// Push pushes a file to the device
// This matches Python SDK's AsyncADBShell.push()
func (a *ADBShell) Push(ctx context.Context, local string, remote string) error {
	if !a.active || a.device == nil {
		if err := a.Connect(ctx); err != nil {
			return err
		}
	}

	// TODO: Implement proper file push using goadb sync protocol
	return fmt.Errorf("Push not yet implemented, use shell command as workaround")
}

// Pull pulls a file from the device
// This matches Python SDK's AsyncADBShell.pull()
func (a *ADBShell) Pull(ctx context.Context, remote string, local string) error {
	if !a.active || a.device == nil {
		if err := a.Connect(ctx); err != nil {
			return err
		}
	}

	// TODO: Implement proper file pull using goadb sync protocol
	return fmt.Errorf("Pull not yet implemented, use shell command as workaround")
}

// Exists checks if a path exists
// This matches Python SDK's AsyncADBShell.exists()
func (a *ADBShell) Exists(ctx context.Context, path string) (bool, error) {
	cmd := fmt.Sprintf("ls %s", path)
	output, err := a.Shell(ctx, cmd, nil)
	if err != nil {
		return false, err
	}

	if strings.Contains(output, "No such file") || strings.TrimSpace(output) == "" {
		return false, nil
	}
	return true, nil
}

// Remove removes a file or directory
// This matches Python SDK's AsyncADBShell.remove()
func (a *ADBShell) Remove(ctx context.Context, path string) error {
	_, err := a.Shell(ctx, fmt.Sprintf("rm -rf %s", path), nil)
	return err
}

// Rename renames a file or directory
// This matches Python SDK's AsyncADBShell.rename()
func (a *ADBShell) Rename(ctx context.Context, src string, dst string) error {
	_, err := a.Shell(ctx, fmt.Sprintf("mv %s %s", src, dst), nil)
	return err
}

// MakeDir creates a directory
// This matches Python SDK's AsyncADBShell.make_dir()
func (a *ADBShell) MakeDir(ctx context.Context, path string) error {
	_, err := a.Shell(ctx, fmt.Sprintf("mkdir -p %s", path), nil)
	return err
}

// Install installs an APK
// This matches Python SDK's AsyncADBShell.install()
func (a *ADBShell) Install(ctx context.Context, apkPath string, reinstall bool) error {
	cmd := "pm install"
	if reinstall {
		cmd += " -r"
	}
	cmd += " " + apkPath
	_, err := a.Shell(ctx, cmd, nil)
	return err
}

// Uninstall uninstalls a package
// This matches Python SDK's AsyncADBShell.uninstall()
func (a *ADBShell) Uninstall(ctx context.Context, packageName string) error {
	_, err := a.Shell(ctx, fmt.Sprintf("pm uninstall %s", packageName), nil)
	return err
}

// Close closes the ADB connection
// This matches Python SDK's AsyncADBShell.close()
func (a *ADBShell) Close() error {
	a.active = false
	// goadb doesn't require explicit close
	return nil
}

// getADBPublicInfo gets ADB public information from the API
// This matches Python SDK's AsyncADBShell._get_adb_public_info()
func (a *ADBShell) getADBPublicInfo(ctx context.Context) error {
	// Create sandbox API to get ADB info
	sandboxApi, err := agentbox.NewSandboxApi(a.connectionConfig)
	if err != nil {
		return err
	}

	// Get ADB public info
	info, err := sandboxApi.GetADBPublicInfo(ctx, a.sandboxID)
	if err != nil {
		return err
	}

	a.host = info.ADBIP
	a.port = info.ADBPort
	return nil
}
