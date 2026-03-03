# AgentBox Go SDK

[AgentBox](https://agentbox.cloud) provides AI sandbox infrastructure for enterprise-grade agents.

## Installation

```bash
go get github.com/agentbox-cloud/go-sdk
```

## Quick Start

### 1) Create an API key

1. Sign up at [AgentBox](https://agentbox.cloud)
2. Create an API key in [API Keys](https://agentbox.cloud/home/api-keys)
3. Export it to your environment:

```bash
export AGENTBOX_API_KEY=ab_******
```

### 2) Run your first sandbox command (sync)

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/agentbox-cloud/go-sdk/agentbox"
)

func main() {
	ctx := context.Background()

	sbx, err := agentbox.NewSandbox(ctx, &agentbox.SandboxOptions{
		APIKey:   "ab_******", // or rely on AGENTBOX_API_KEY
		Domain:   "agentbox.net.cn",
		Template: "your_template_id",
		Timeout:  120,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer sbx.Kill(ctx, 0)

	res, err := sbx.Commands().Run(ctx, "ls /", &agentbox.RunCommandOptions{
		User: agentbox.UsernameUser,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("exit:", res.ExitCode)
	fmt.Println(res.Stdout)
}
```

### 3) Async style sandbox (API-compatible async entry)

```go
sbx, err := agentbox.AsyncSandboxCreate(ctx, &agentbox.SandboxOptions{
	APIKey:   "ab_******",
	Domain:   "agentbox.net.cn",
	Template: "your_template_id",
	Timeout:  120,
})
```

## Common Usage

### Filesystem

```go
_, err = sbx.Files().Write(ctx, "/agentbox.txt", "hello", agentbox.UsernameUser, 0)
if err != nil {
	log.Fatal(err)
}

content, err := sbx.Files().Read(ctx, "/agentbox.txt", agentbox.ReadFormatText, agentbox.UsernameUser, 0)
if err != nil {
	log.Fatal(err)
}
fmt.Println(content.(string))
```

### Connect to existing sandbox

```go
apiKey := "ab_******"
domain := "agentbox.net.cn"
conn, err := agentbox.SandboxConnect(ctx, "sandbox_id", nil, &apiKey, &domain, nil, nil, nil)
if err != nil {
	log.Fatal(err)
}
defer conn.Kill(ctx, 0)
```

### List / Resume

```go
apiKey := "ab_******"
domain := "agentbox.net.cn"

list, err := agentbox.SandboxList(ctx, nil, &apiKey, &domain, nil, nil, nil, nil)
if err != nil {
	log.Fatal(err)
}
fmt.Println("running:", len(list))
```

## Package Layout (Python SDK aligned)

- `agentbox` (primary public entrypoint, backward-compatible)
- `agentbox/envd/connect`
- `agentbox/sandbox`
- `agentbox/sandbox/api`
- `agentbox/sandbox/commands`
- `agentbox/sandbox/filesystem`
- `agentbox/core`
- `agentbox/transport/httpapi`

## Notes

- Keep using root package `agentbox` for most integrations.
- Structured sub-packages are available for clearer module boundaries.
- Some advanced capabilities (for example parts of ADB/PTY) may still evolve.

## Documentation

See [AgentBox Docs](https://agentbox.cloud/docs).