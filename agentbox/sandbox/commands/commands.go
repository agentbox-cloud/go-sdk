package commands

import "github.com/agentbox-cloud/go-sdk/agentbox"

// This package mirrors Python SDK layout:
// agentbox/sandbox_sync/commands and agentbox/sandbox_async/commands.
//
// In Go SDK we keep the runtime implementation in the root `agentbox` package
// for backward compatibility, and expose aliases here as the structured entry.

type Service = agentbox.Commands
type Handle = agentbox.CommandHandle
type AsyncHandle = agentbox.AsyncCommandHandle

type RunOptions = agentbox.RunCommandOptions
type ConnectOptions = agentbox.ConnectCommandOptions

type Result = agentbox.CommandResult
type ProcessInfo = agentbox.ProcessInfo
type OutputHandler = agentbox.OutputHandler

