package sandbox_sync

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/agentbox-cloud/go-sdk/agentbox"
)

// TestSandboxCreation tests creating a new sandbox
// This matches Python SDK test patterns
func TestSandboxCreation(t *testing.T) {
	ctx := context.Background()

	// Skip if no API key is set
	apiKey := os.Getenv("AGENTBOX_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping test: no API key provided (set AGENTBOX_API_KEY)")
	}

	opts := &agentbox.SandboxOptions{
		Template: "base",
		Timeout:  300,
		APIKey:   apiKey,
	}

	sandbox, err := NewSandbox(ctx, opts)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}

	if sandbox.SandboxID() == "" {
		t.Error("Sandbox ID should not be empty")
	}

	// Cleanup
	_, _ = sandbox.Kill(ctx, 0)
}

// TestSandboxConnect tests connecting to an existing sandbox
func TestSandboxConnect(t *testing.T) {
	ctx := context.Background()

	apiKey := os.Getenv("AGENTBOX_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping test: no API key provided")
	}

	// First create a sandbox
	sandbox, err := NewSandbox(ctx, &agentbox.SandboxOptions{
		Template: "base",
		Timeout:  300,
		APIKey:   apiKey,
	})
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}

	sandboxID := sandbox.SandboxID()

	// Then connect to it
	connectedSandbox, err := Connect(ctx, sandboxID, nil, &apiKey, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("Failed to connect to sandbox: %v", err)
	}

	if connectedSandbox.SandboxID() != sandboxID {
		t.Errorf("Connected sandbox ID mismatch: got %s, want %s",
			connectedSandbox.SandboxID(), sandboxID)
	}

	// Cleanup
	_, _ = sandbox.Kill(ctx, 0)
	_, _ = connectedSandbox.Kill(ctx, 0)
}

// TestSandboxIsRunning tests checking if sandbox is running
func TestSandboxIsRunning(t *testing.T) {
	ctx := context.Background()

	apiKey := os.Getenv("AGENTBOX_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping test: no API key provided")
	}

	sandbox, err := NewSandbox(ctx, &agentbox.SandboxOptions{
		Template: "base",
		Timeout:  300,
		APIKey:   apiKey,
	})
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer func() {
		_, _ = sandbox.Kill(ctx, 0)
	}()

	// Check if running
	isRunning, err := sandbox.IsRunning(ctx, 30*time.Second)
	if err != nil {
		t.Fatalf("Failed to check sandbox status: %v", err)
	}

	if !isRunning {
		t.Error("Newly created sandbox should be running")
	}
}

// TestSandboxSetTimeout tests setting sandbox timeout
func TestSandboxSetTimeout(t *testing.T) {
	ctx := context.Background()

	apiKey := os.Getenv("AGENTBOX_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping test: no API key provided")
	}

	sandbox, err := NewSandbox(ctx, &agentbox.SandboxOptions{
		Template: "base",
		Timeout:  300,
		APIKey:   apiKey,
	})
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer func() {
		_, _ = sandbox.Kill(ctx, 0)
	}()

	// Set timeout
	err = sandbox.SetTimeout(ctx, 600)
	if err != nil {
		t.Fatalf("Failed to set timeout: %v", err)
	}
}

// TestSandboxPauseResume tests pausing and resuming a sandbox
func TestSandboxPauseResume(t *testing.T) {
	ctx := context.Background()

	apiKey := os.Getenv("AGENTBOX_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping test: no API key provided")
	}

	sandbox, err := NewSandbox(ctx, &agentbox.SandboxOptions{
		Template: "base",
		Timeout:  300,
		APIKey:   apiKey,
	})
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}

	sandboxID := sandbox.SandboxID()

	// Pause
	err = sandbox.Pause(ctx)
	if err != nil {
		t.Fatalf("Failed to pause sandbox: %v", err)
	}

	// Resume
	timeout := 300
	_, err = Resume(ctx, sandboxID, &timeout, &apiKey, nil, nil, nil)
	if err != nil {
		t.Fatalf("Failed to resume sandbox: %v", err)
	}

	// Cleanup
	_, _ = sandbox.Kill(ctx, 0)
}

// TestSandboxKill tests killing a sandbox
func TestSandboxKill(t *testing.T) {
	ctx := context.Background()

	apiKey := os.Getenv("AGENTBOX_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping test: no API key provided")
	}

	sandbox, err := NewSandbox(ctx, &agentbox.SandboxOptions{
		Template: "base",
		Timeout:  300,
		APIKey:   apiKey,
	})
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}

	sandboxID := sandbox.SandboxID()

	// Kill
	killed, err := sandbox.Kill(ctx, 0)
	if err != nil {
		t.Fatalf("Failed to kill sandbox: %v", err)
	}

	if !killed {
		t.Error("Kill should return true for existing sandbox")
	}

	// Verify it's killed by trying to connect
	_, err = Connect(ctx, sandboxID, nil, &apiKey, nil, nil, nil, nil)
	if err == nil {
		t.Error("Should not be able to connect to killed sandbox")
	}
}

// TestSandboxValidation tests validation logic
func TestSandboxValidation(t *testing.T) {
	ctx := context.Background()

	// Test: Cannot set metadata when connecting to existing sandbox
	opts := &agentbox.SandboxOptions{
		SandboxID: "test-sandbox-id",
		Metadata:  map[string]string{"key": "value"},
	}

	_, err := NewSandbox(ctx, opts)
	if err == nil {
		t.Error("Should fail when setting metadata with existing sandbox_id")
	}
}
