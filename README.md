# AgentBox Go SDK

AgentBox Go SDK provides a Go client library for interacting with AgentBox cloud sandbox environments. This SDK is designed to match the Python SDK's API semantics and structure, providing a familiar experience for developers who have used the Python SDK.

## Features

- ✅ **Synchronous and Asynchronous APIs** - Both `sandbox_sync` and `sandbox_async` packages
- ✅ **File System Operations** - Read, Write, List, Stat, Exists, Remove, Rename, MakeDir
- ✅ **File System Watching** - Real-time filesystem event monitoring
- ✅ **Command Execution** - Run commands in foreground or background with streaming output
- ✅ **Process Management** - List, kill, and connect to running processes
- ✅ **ADB Shell Support** - Full Android device interaction (connect, shell, push, pull, install, uninstall)
- ✅ **SSH Support** - SSH-based command execution and filesystem operations
- ✅ **Sandbox Lifecycle** - Create, connect, pause, resume, kill sandboxes
- ✅ **Error Handling** - Comprehensive error types matching Python SDK
- ✅ **Context Support** - Full `context.Context` integration for cancellation and timeouts

## Installation

```bash
go get github.com/agentbox-cloud/go-sdk
```

## Quick Start

### Synchronous API

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/agentbox-cloud/go-sdk/agentbox"
    "github.com/agentbox-cloud/go-sdk/agentbox/sandbox_sync"
)

func main() {
    ctx := context.Background()
    
    // Create a new sandbox
    sandbox, err := sandbox_sync.NewSandbox(ctx, &agentbox.SandboxOptions{
        Template: "base",
        Timeout:  300,
        APIKey:   "ab_...", // Your API key
        Domain:   "agentbox.cloud",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer sandbox.Kill(ctx, 0)
    
    // Run a command
    result, err := sandbox.Commands().Run(ctx, "echo 'Hello, World!'", &agentbox.RunCommandOptions{
        User: agentbox.UsernameUser,
    })
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println(result.Stdout)
}
```

### Asynchronous API

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/agentbox-cloud/go-sdk/agentbox"
    "github.com/agentbox-cloud/go-sdk/agentbox/sandbox_async"
)

func main() {
    ctx := context.Background()
    
    // Create a new async sandbox
    sandbox, err := sandbox_async.Create(ctx, &agentbox.SandboxOptions{
        Template: "base",
        Timeout:  300,
        APIKey:   "ab_...",
        Domain:   "agentbox.cloud",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer sandbox.Kill(ctx, 0)
    
    // Run a command
    result, err := sandbox.Commands().Run(ctx, "echo 'Hello, Async World!'", &agentbox.RunCommandOptions{
        User: agentbox.UsernameUser,
    })
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println(result.Stdout)
}
```

**Note:** In Go, both synchronous and asynchronous versions use the same underlying implementation. The distinction is maintained for API consistency with the Python SDK. You can use goroutines for concurrent operations.

## API Reference

### Creating a Sandbox

```go
// Create a new sandbox (synchronous)
sandbox, err := sandbox_sync.NewSandbox(ctx, &agentbox.SandboxOptions{
    Template:       "base",           // Sandbox template ID
    Timeout:        300,              // Timeout in seconds
    APIKey:         "ab_...",         // API key
    Domain:         "agentbox.cloud",  // Domain (optional)
    Debug:          false,            // Debug mode (optional)
    RequestTimeout: 30 * time.Second, // Request timeout (optional)
    Metadata:       map[string]string{"env": "dev"}, // Metadata (optional)
    Envs:           map[string]string{"MY_VAR": "value"}, // Environment variables (optional)
    Secure:         false,            // Secure mode (optional)
})

// Create a new sandbox (asynchronous)
sandbox, err := sandbox_async.Create(ctx, &agentbox.SandboxOptions{
    // Same options as above
})

// Connect to existing sandbox
sandbox, err := sandbox_sync.Connect(ctx, sandboxID, nil, &apiKey, &domain, nil, nil, nil)

// Resume paused sandbox
sandbox, err := sandbox_sync.Resume(ctx, sandboxID, &timeout, &apiKey, &domain, nil, nil)
```

### File Operations

```go
// Write a file (supports string, []byte, or io.Reader)
entry, err := sandbox.Files().Write(ctx, "/tmp/test.txt", "Hello, World!", 
    agentbox.UsernameUser, 0)

// Read a file (text format)
content, err := sandbox.Files().Read(ctx, "/tmp/test.txt", 
    agentbox.ReadFormatText, agentbox.UsernameUser, 0)
fmt.Println(content.(string))

// Read a file (bytes format)
content, err := sandbox.Files().Read(ctx, "/tmp/test.txt", 
    agentbox.ReadFormatBytes, agentbox.UsernameUser, 0)
fmt.Println(content.([]byte))

// List files and directories
entries, err := sandbox.Files().List(ctx, "/tmp", agentbox.UsernameUser, 0)
for _, entry := range entries {
    fmt.Printf("%s: %s\n", entry.Type, entry.Name)
}

// Get file information
info, err := sandbox.Files().Stat(ctx, "/tmp/test.txt", agentbox.UsernameUser, 0)

// Check if file exists
exists, err := sandbox.Files().Exists(ctx, "/tmp/test.txt", agentbox.UsernameUser, 0)

// Remove a file or directory
err := sandbox.Files().Remove(ctx, "/tmp/test.txt", agentbox.UsernameUser, 0)

// Rename a file or directory
entry, err := sandbox.Files().Rename(ctx, "/tmp/old.txt", "/tmp/new.txt", 
    agentbox.UsernameUser, 0)

// Create a directory
entry, err := sandbox.Files().MakeDir(ctx, "/tmp/mydir", agentbox.UsernameUser, 0)
```

### File System Watching

```go
// Watch a directory for changes
handle, err := sandbox.Files().Watch(ctx, "/tmp", agentbox.UsernameUser)
if err != nil {
    log.Fatal(err)
}
defer handle.Close()

// Write a file to trigger an event
sandbox.Files().Write(ctx, "/tmp/test.txt", "content", agentbox.UsernameUser, 0)

// Wait for events to propagate
time.Sleep(2 * time.Second)

// Get new events
events, err := handle.GetNewEvents(ctx)
for _, event := range events {
    fmt.Printf("Event: %s - %s\n", event.Type, event.Name)
}
```

### Command Execution

```go
// Run a command and wait for result (foreground)
result, err := sandbox.Commands().Run(ctx, "ls -la", &agentbox.RunCommandOptions{
    User: agentbox.UsernameUser,
    Cwd:  "/tmp",
    Envs: map[string]string{"VAR": "value"},
})
fmt.Printf("Exit code: %d\n", result.ExitCode)
fmt.Printf("Stdout: %s\n", result.Stdout)
fmt.Printf("Stderr: %s\n", result.Stderr)

// Run a command with streaming output
result, err := sandbox.Commands().Run(ctx, "python script.py", &agentbox.RunCommandOptions{
    User: agentbox.UsernameUser,
    OnStdout: func(data string) {
        fmt.Print(data) // Print stdout in real-time
    },
    OnStderr: func(data string) {
        fmt.Fprint(os.Stderr, data) // Print stderr in real-time
    },
})

// Run a command in background
handle, err := sandbox.Commands().RunBackground(ctx, "python long_script.py", &agentbox.RunCommandOptions{
    User:    agentbox.UsernameUser,
    Timeout: 300 * time.Second,
})
defer handle.Kill(ctx)

// Wait for background command to finish
result, err := handle.Wait(ctx)

// Send input to running command
err := handle.SendStdin(ctx, "input data\n")

// List running processes
processes, err := sandbox.Commands().List(ctx, 0)
for _, proc := range processes {
    fmt.Printf("PID: %d, Cmd: %s\n", proc.PID, proc.Cmd)
}

// Kill a process
killed, err := sandbox.Commands().Kill(ctx, pid, 0)

// Connect to a running process
handle, err := sandbox.Commands().Connect(ctx, pid, &agentbox.ConnectCommandOptions{
    OnStdout: func(data string) { fmt.Print(data) },
})
```

### Sandbox Management

```go
// Get sandbox information
info, err := sandbox.GetInfo(ctx)
fmt.Printf("Sandbox ID: %s\n", info.SandboxID)
fmt.Printf("Template: %s\n", info.TemplateID)

// Check if sandbox is running
isRunning, err := sandbox.IsRunning(ctx, 0)

// Set timeout
err := sandbox.SetTimeout(ctx, 600)

// Pause sandbox
err := sandbox.Pause(ctx)

// Resume sandbox (returns new sandbox instance)
resumedSandbox, err := sandbox.Resume(ctx, &timeout)

// Kill sandbox
killed, err := sandbox.Kill(ctx, 0)

// List all sandboxes
query := &agentbox.SandboxQuery{
    Metadata: map[string]string{"env": "dev"},
}
sandboxes, err := agentbox.ListSandboxes(ctx, query, &apiKey, &domain, nil, nil, nil)
```

### ADB Shell (Android Sandboxes)

```go
// Get ADB shell instance
adbShell := sandbox.ADBShell()
if adbShell == nil {
    log.Fatal("ADB shell not available (not an Android sandbox)")
}

// Connect to ADB
err := adbShell.Connect(ctx)

// Execute shell command
output, err := adbShell.Shell(ctx, "ls /sdcard", nil)
fmt.Println(output)

// Push file to device
err := adbShell.Push(ctx, "/local/path/file.apk", "/sdcard/file.apk")

// Pull file from device
err := adbShell.Pull(ctx, "/sdcard/file.apk", "/local/path/file.apk")

// Check if file exists on device
exists, err := adbShell.Exists(ctx, "/sdcard/file.apk")

// Install APK
err := adbShell.Install(ctx, "/sdcard/app.apk", false)

// Uninstall package
err := adbShell.Uninstall(ctx, "com.example.app")

// Close ADB connection
err := adbShell.Close(ctx)
```

## Configuration

### Environment Variables

The SDK supports the following environment variables:

- `AGENTBOX_API_KEY`: Your API key (default authentication method)
- `AGENTBOX_ACCESS_TOKEN`: Access token (alternative authentication)
- `AGENTBOX_DOMAIN`: Domain for the API (default: "agentbox.cloud")
- `AGENTBOX_DEBUG`: Enable debug mode (default: false)

### Connection Config

```go
config := agentbox.NewConnectionConfig(&agentbox.ConnectionConfigOptions{
    APIKey:         "ab_...",
    Domain:         "agentbox.cloud",
    Debug:          false,
    RequestTimeout: 30 * time.Second,
    Headers:        map[string]string{"Custom-Header": "value"},
    Proxy:          nil, // Proxy configuration
})
```

## Error Handling

The SDK uses Go's standard error handling. All errors implement the `error` interface, and specific error types are available:

```go
import "github.com/agentbox-cloud/go-sdk/agentbox"

// Check for specific error types
if err != nil {
    switch e := err.(type) {
    case *agentbox.TimeoutException:
        // Handle timeout
        fmt.Printf("Request timed out: %v\n", e)
    case *agentbox.NotFoundException:
        // Handle not found
        fmt.Printf("Resource not found: %v\n", e)
    case *agentbox.AuthenticationException:
        // Handle authentication error
        fmt.Printf("Authentication failed: %v\n", e)
    case *agentbox.CommandExitException:
        // Handle command exit error
        fmt.Printf("Command exited with code %d: %v\n", e.ExitCode, e)
    case *agentbox.SandboxException:
        // Handle general sandbox error
        fmt.Printf("Sandbox error: %v\n", e)
    default:
        // Handle other errors
        fmt.Printf("Unknown error: %v\n", err)
    }
}
```

## Synchronous vs Asynchronous

The Go SDK provides both `sandbox_sync` and `sandbox_async` packages for API consistency with the Python SDK. However, in Go:

- **Both versions use the same underlying implementation** - Go's `net/http.Client` is used for both
- **Concurrency is handled via goroutines** - You can use goroutines for concurrent operations
- **Context support** - Both versions fully support `context.Context` for cancellation and timeouts

Example of concurrent operations:

```go
// Run multiple operations concurrently
var wg sync.WaitGroup
results := make([]string, 2)

wg.Add(1)
go func() {
    defer wg.Done()
    content, _ := sandbox.Files().Read(ctx, "/file1.txt", agentbox.ReadFormatText, agentbox.UsernameUser, 0)
    results[0] = content.(string)
}()

wg.Add(1)
go func() {
    defer wg.Done()
    content, _ := sandbox.Files().Read(ctx, "/file2.txt", agentbox.ReadFormatText, agentbox.UsernameUser, 0)
    results[1] = content.(string)
}()

wg.Wait()
```

## Comparison with Python SDK

The Go SDK is designed to match the Python SDK's API semantics:

| Python SDK | Go SDK |
|------------|--------|
| `Sandbox()` | `sandbox_sync.NewSandbox()` |
| `AsyncSandbox.create()` | `sandbox_async.Create()` |
| `sandbox.commands.run()` | `sandbox.Commands().Run()` |
| `sandbox.commands.run_background()` | `sandbox.Commands().RunBackground()` |
| `sandbox.files.write()` | `sandbox.Files().Write()` |
| `sandbox.files.read()` | `sandbox.Files().Read()` |
| `sandbox.files.list()` | `sandbox.Files().List()` |
| `sandbox.files.watch_dir()` | `sandbox.Files().Watch()` |
| `sandbox.is_running()` | `sandbox.IsRunning()` |
| `Sandbox.connect()` | `sandbox_sync.Connect()` |
| `sandbox.adb_shell.shell()` | `sandbox.ADBShell().Shell()` |
| `sandbox.pause()` | `sandbox.Pause()` |
| `Sandbox.resume()` | `sandbox_sync.Resume()` |

## Requirements

- Go 1.22 or later
- Network access to AgentBox API

## Dependencies

The SDK uses minimal dependencies:

- Standard library (`net/http`, `context`, `encoding/json`, etc.)
- `github.com/zach-klippenstein/goadb` - For ADB operations (Android sandboxes only)
- `golang.org/x/crypto` - For SSH operations (optional)

## Examples

See the `examples/` directory for comprehensive examples including:

- Basic file operations
- Command execution with streaming
- Filesystem watching
- ADB shell operations
- Sandbox lifecycle management
- Concurrent operations

Run examples:

```bash
cd examples
go test -v -run TestSyncBaseReadAndWrite
```

## License

MIT

## Documentation

For more detailed documentation, visit [https://agentbox.cloud/docs](https://agentbox.cloud/docs)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
