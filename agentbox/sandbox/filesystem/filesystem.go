package filesystem

import "github.com/agentbox-cloud/go-sdk/agentbox"

// This package mirrors Python SDK layout:
// agentbox/sandbox_sync/filesystem and agentbox/sandbox_async/filesystem.
//
// Runtime implementation remains in root `agentbox` for compatibility.

type Service = agentbox.Filesystem
type WatchHandle = agentbox.WatchHandle
type AsyncWatchHandle = agentbox.AsyncWatchHandle

type EntryInfo = agentbox.EntryInfo
type Event = agentbox.FilesystemEvent
type EventType = agentbox.FilesystemEventType
type ReadFormat = agentbox.ReadFormat

