package adb_shell

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/agentbox-cloud/go-sdk/agentbox"

	goadb "github.com/zach-klippenstein/goadb"
)

// This package implements ADB Shell functionality
// This matches Python SDK's agentbox/sandbox_sync/adb_shell/adb_shell.py

// ADBShell represents an ADB shell connection
// This matches Python SDK's ADBShell class in sandbox_sync/adb_shell/adb_shell.py
type ADBShell struct {
	connectionConfig *agentbox.ConnectionConfig
	sandboxID        string
	host             string
	port             int
	authTimeoutS     float64
	publicKey        string     // RSA public key for authentication
	privateKey       string     // RSA private key for authentication
	adb              *goadb.Adb // Store ADB client for reuse
	device           *goadb.Device
	active           bool
}

// NewADBShell creates a new ADB shell instance
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
// This matches Python SDK's ADBShell.connect()
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

	// Create or reuse ADB client
	if a.adb == nil {
		adb, err := goadb.New()
		if err != nil {
			return fmt.Errorf("failed to create ADB client: %w", err)
		}
		a.adb = adb
	}

	// Start ADB server if not running (matching Python SDK behavior)
	// Note: goadb will try to start the server automatically, but we should ensure it's running
	if err := a.adb.StartServer(); err != nil {
		// Server might already be running, ignore the error
	}

	// Connect to remote device via ADB server
	// This matches Python SDK's behavior: it uses adb_shell library which directly connects via TCP
	// goadb requires going through the ADB server, so we use Connect() first
	deviceSerial := fmt.Sprintf("%s:%d", a.host, a.port)

	// Try to connect to remote device
	// Note: goadb.Connect() may fail if device is already connected, which is OK
	// However, goadb doesn't support RSA authentication like Python's adb_shell library
	// If the remote device requires RSA authentication, this will fail
	connectErr := a.adb.Connect(a.host, a.port)
	if connectErr != nil {
		// Log the error but continue - device might already be connected
		// We'll verify the connection by checking device state
	}

	// Get device using the serial
	device := a.adb.Device(goadb.DeviceWithSerial(deviceSerial))

	// Verify device is available
	// Retry a few times as the connection might take a moment
	maxRetries := 5
	var lastErr error
	var lastState goadb.DeviceState
	var connectErrorMsg string
	if connectErr != nil {
		connectErrorMsg = fmt.Sprintf(" (initial connect error: %v)", connectErr)
	}

	for i := 0; i < maxRetries; i++ {
		state, err := device.State()
		if err == nil && state == goadb.StateOnline {
			// Success!
			a.device = device
			a.active = true
			return nil
		}

		// Store error or create one if state is not online
		if err != nil {
			lastErr = err
		} else {
			lastState = state
			if state == goadb.StateUnauthorized {
				// Device requires authentication (RSA keys)
				// goadb doesn't support RSA authentication, so we can't proceed
				lastErr = fmt.Errorf("device requires RSA authentication (state: %v), but goadb library doesn't support RSA keys for remote connections. This is a known limitation - Python SDK uses adb_shell library which supports RSA authentication", state)
			} else {
				lastErr = fmt.Errorf("device state is %v, expected %v", state, goadb.StateOnline)
			}
		}

		if i < maxRetries-1 {
			// Wait before retry
			time.Sleep(time.Duration(500*(i+1)) * time.Millisecond) // Exponential backoff
			// Try to reconnect
			if reconnectErr := a.adb.Connect(a.host, a.port); reconnectErr != nil && i == 0 {
				// Store the first reconnect error for debugging
				connectErrorMsg = fmt.Sprintf(" (connect error: %v)", reconnectErr)
			}
		}
	}

	// All retries failed
	if lastErr == nil {
		lastErr = fmt.Errorf("device state is %v, expected %v", lastState, goadb.StateOnline)
	}
	return fmt.Errorf("failed to connect to ADB device %s after %d retries%s: %w", deviceSerial, maxRetries, connectErrorMsg, lastErr)
}

// Shell executes a shell command
// This matches Python SDK's ADBShell.shell()
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
// This matches Python SDK's ADBShell.push()
func (a *ADBShell) Push(ctx context.Context, local string, remote string) error {
	if !a.active || a.device == nil {
		if err := a.Connect(ctx); err != nil {
			return err
		}
	}

	// Open local file
	// Note: goadb doesn't have a direct Push method, we need to implement it
	// For now, we'll use shell command as a workaround
	// TODO: Implement proper file push using goadb sync protocol
	return fmt.Errorf("Push not yet implemented, use shell command as workaround")
}

// Pull pulls a file from the device
// This matches Python SDK's ADBShell.pull()
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
// This matches Python SDK's ADBShell.exists()
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
// This matches Python SDK's ADBShell.remove()
func (a *ADBShell) Remove(ctx context.Context, path string) error {
	_, err := a.Shell(ctx, fmt.Sprintf("rm -rf %s", path), nil)
	return err
}

// Rename renames a file or directory
// This matches Python SDK's ADBShell.rename()
func (a *ADBShell) Rename(ctx context.Context, src string, dst string) error {
	_, err := a.Shell(ctx, fmt.Sprintf("mv %s %s", src, dst), nil)
	return err
}

// MakeDir creates a directory
// This matches Python SDK's ADBShell.make_dir()
func (a *ADBShell) MakeDir(ctx context.Context, path string) error {
	_, err := a.Shell(ctx, fmt.Sprintf("mkdir -p %s", path), nil)
	return err
}

// Install installs an APK
// This matches Python SDK's ADBShell.install()
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
// This matches Python SDK's ADBShell.uninstall()
func (a *ADBShell) Uninstall(ctx context.Context, packageName string) error {
	_, err := a.Shell(ctx, fmt.Sprintf("pm uninstall %s", packageName), nil)
	return err
}

// Close closes the ADB connection
// This matches Python SDK's ADBShell.close()
func (a *ADBShell) Close() error {
	a.active = false
	// goadb doesn't require explicit close
	return nil
}

// getADBPublicInfo gets ADB public information from the API
// This matches Python SDK's ADBShell._get_adb_public_info()
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
	a.publicKey = info.PublicKey
	a.privateKey = info.PrivateKey
	// Note: goadb doesn't support RSA authentication like Python's adb_shell library
	// We store the keys but cannot use them directly with goadb
	// This is a known limitation - goadb requires going through ADB server
	// which may not support RSA authentication for remote devices
	return nil
}
