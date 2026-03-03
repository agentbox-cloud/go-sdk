package core

import "github.com/agentbox-cloud/go-sdk/agentbox"

type SandboxInfo = agentbox.SandboxInfo
type ListedSandbox = agentbox.ListedSandbox
type SandboxQuery = agentbox.SandboxQuery

type ProcessInfo = agentbox.ProcessInfo
type CommandResult = agentbox.CommandResult
type OutputHandler = agentbox.OutputHandler

type EntryInfo = agentbox.EntryInfo
type FileType = agentbox.FileType
type FilesystemEvent = agentbox.FilesystemEvent
type FilesystemEventType = agentbox.FilesystemEventType

const (
	FileTypeFile      = agentbox.FileTypeFile
	FileTypeDirectory = agentbox.FileTypeDirectory
	FileTypeSymlink   = agentbox.FileTypeSymlink

	FilesystemEventTypeCreate = agentbox.FilesystemEventTypeCreate
	FilesystemEventTypeModify = agentbox.FilesystemEventTypeModify
	FilesystemEventTypeDelete = agentbox.FilesystemEventTypeDelete
)

