package agentbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// ADBShell provides methods for interacting with Android devices via ADB
// This matches the Python SDK's ADBShell class
type ADBShell interface {
	// Connect connects to the ADB device
	Connect(ctx context.Context) error

	// Shell executes a shell command on the Android device
	Shell(ctx context.Context, command string, timeout *time.Duration) (string, error)

	// Push pushes a local file to the remote device
	Push(ctx context.Context, local string, remote string) error

	// Pull pulls a file from the remote device to local
	Pull(ctx context.Context, remote string, local string) error

	// Exists checks if a path exists on the device
	Exists(ctx context.Context, path string) (bool, error)

	// Remove removes a file or directory on the device
	Remove(ctx context.Context, path string) error

	// Rename renames a file or directory on the device
	Rename(ctx context.Context, src string, dst string) error

	// MakeDir creates a directory on the device
	MakeDir(ctx context.Context, path string) error

	// Install installs an APK on the device
	Install(ctx context.Context, apkPath string, reinstall bool) error

	// Uninstall uninstalls a package from the device
	Uninstall(ctx context.Context, packageName string) error

	// Close closes the ADB connection
	Close(ctx context.Context) error
}

// adbShellImpl implements ADBShell
// This will use an ADB client library (e.g., github.com/zach-klippenstein/goadb)
type adbShellImpl struct {
	sandboxID        string
	connectionConfig *ConnectionConfig
	host             string
	port             int
	device           interface{} // Will be the ADB device connection
	active           bool
}

// NewADBShell creates a new ADBShell instance
func NewADBShell(sandboxID string, config *ConnectionConfig) ADBShell {
	return &adbShellImpl{
		sandboxID:        sandboxID,
		connectionConfig: config,
		active:           false,
	}
}

// Connect connects to the ADB device
func (a *adbShellImpl) Connect(ctx context.Context) error {
	if a.active {
		return nil
	}

	// Get ADB public info from API
	info, err := a.getADBPublicInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get ADB public info: %w", err)
	}

	a.host = info.ADBIP
	a.port = info.ADBPort

	// TODO: Connect to ADB device using RSA keys
	// This requires an ADB client library for Go
	// Example: github.com/zach-klippenstein/goadb or similar
	//
	// For now, we'll mark it as a TODO that requires external library
	return fmt.Errorf("ADB connection not yet implemented: requires Go ADB client library (e.g., github.com/zach-klippenstein/goadb)")
}

// Shell executes a shell command on the Android device
func (a *adbShellImpl) Shell(ctx context.Context, command string, timeout *time.Duration) (string, error) {
	if !a.active {
		if err := a.Connect(ctx); err != nil {
			return "", err
		}
	}

	// TODO: Execute command via ADB
	return "", fmt.Errorf("ADB shell command execution not yet implemented: requires Go ADB client library")
}

// Push pushes a local file to the remote device
func (a *adbShellImpl) Push(ctx context.Context, local string, remote string) error {
	if !a.active {
		if err := a.Connect(ctx); err != nil {
			return err
		}
	}

	// TODO: Push file via ADB
	return fmt.Errorf("ADB push not yet implemented: requires Go ADB client library")
}

// Pull pulls a file from the remote device to local
func (a *adbShellImpl) Pull(ctx context.Context, remote string, local string) error {
	if !a.active {
		if err := a.Connect(ctx); err != nil {
			return err
		}
	}

	// TODO: Pull file via ADB
	return fmt.Errorf("ADB pull not yet implemented: requires Go ADB client library")
}

// Exists checks if a path exists on the device
func (a *adbShellImpl) Exists(ctx context.Context, path string) (bool, error) {
	output, err := a.Shell(ctx, fmt.Sprintf("ls %s", path), nil)
	if err != nil {
		return false, err
	}

	if output == "" || contains(output, "No such file") {
		return false, nil
	}
	return true, nil
}

// Remove removes a file or directory on the device
func (a *adbShellImpl) Remove(ctx context.Context, path string) error {
	_, err := a.Shell(ctx, fmt.Sprintf("rm -rf %s", path), nil)
	return err
}

// Rename renames a file or directory on the device
func (a *adbShellImpl) Rename(ctx context.Context, src string, dst string) error {
	_, err := a.Shell(ctx, fmt.Sprintf("mv %s %s", src, dst), nil)
	return err
}

// MakeDir creates a directory on the device
func (a *adbShellImpl) MakeDir(ctx context.Context, path string) error {
	_, err := a.Shell(ctx, fmt.Sprintf("mkdir -p %s", path), nil)
	return err
}

// Install installs an APK on the device
func (a *adbShellImpl) Install(ctx context.Context, apkPath string, reinstall bool) error {
	cmd := fmt.Sprintf("pm install %s%s", map[bool]string{true: "-r ", false: ""}[reinstall], apkPath)
	_, err := a.Shell(ctx, cmd, nil)
	return err
}

// Uninstall uninstalls a package from the device
func (a *adbShellImpl) Uninstall(ctx context.Context, packageName string) error {
	_, err := a.Shell(ctx, fmt.Sprintf("pm uninstall %s", packageName), nil)
	return err
}

// Close closes the ADB connection
func (a *adbShellImpl) Close(ctx context.Context) error {
	a.active = false
	// TODO: Close ADB device connection
	return nil
}

// ADBPublicInfo contains ADB connection information
type ADBPublicInfo struct {
	ADBIP      string
	ADBPort    int
	PublicKey  string
	PrivateKey string
}

// getADBPublicInfo retrieves ADB public info from the API
func (a *adbShellImpl) getADBPublicInfo(ctx context.Context) (*ADBPublicInfo, error) {
	// Create API client
	client, err := NewAPIClient(a.connectionConfig, true, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	// Call API: GET /sandboxes/{sandbox_id}/adb/public-info
	path := fmt.Sprintf("/sandboxes/%s/adb/public-info", a.sandboxID)
	resp, err := client.Request(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to request ADB public info: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read ADB public info response: %w", err)
	}

	// Check status code
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ADB public info request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse ADB public info response: %w", err)
	}

	// Extract fields (camelCase from API, fallback to snake_case)
	getString := func(key string) string {
		if val, ok := result[key].(string); ok {
			return val
		}
		// Try snake_case fallback
		snakeKey := toSnakeCase(key)
		if val, ok := result[snakeKey].(string); ok {
			return val
		}
		return ""
	}

	getInt := func(key string) int {
		if val, ok := result[key].(float64); ok {
			return int(val)
		}
		// Try snake_case fallback
		snakeKey := toSnakeCase(key)
		if val, ok := result[snakeKey].(float64); ok {
			return int(val)
		}
		return 0
	}

	info := &ADBPublicInfo{
		ADBIP:      getString("adbIp"),
		ADBPort:    getInt("adbPort"),
		PublicKey:  getString("publicKey"),
		PrivateKey: getString("privateKey"),
	}

	if info.ADBIP == "" || info.ADBPort == 0 || info.PublicKey == "" || info.PrivateKey == "" {
		return nil, fmt.Errorf("incomplete ADB public info: missing required fields")
	}

	return info, nil
}

// Helper function to convert camelCase to snake_case
func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		if r >= 'A' && r <= 'Z' {
			result = append(result, r+'a'-'A')
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
