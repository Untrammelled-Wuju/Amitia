//go:build linux && !android

package tools

import (
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/androidlinux/terminal"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimehost"
)

func (r *terminalToolRegistrar) RegisterLinuxFileTools(
	host runtimehost.RuntimeHost,
	registry *capability.ToolRegistry,
) error {
	if !terminal.IsAndroidLinuxRuntime(host) {
		return nil
	}

	tools := BuildLinuxFileTools()

	for _, tool := range tools {
		if err := registry.Register(nil, tool); err != nil {
			if err := registry.Replace(nil, tool); err != nil {
				return fmt.Errorf("register linux file tool %s: %w", tool.ID, err)
			}
		}
	}

	return nil
}

func BuildLinuxFileTools() []capability.ToolDefinition {
	providerID := "android_linux"
	namespace := "file"

	statID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".stat")
	listID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".list")
	readID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".read")
	writeID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".write")
	appendID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".append")
	mkdirID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".mkdir")
	touchID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".touch")
	copyID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".copy")
	moveID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".move")
	deleteID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".delete")
	searchID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".search")
	chmodID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".chmod")
	readlinkID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".readlink")
	symlinkID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".symlink")

	statSchema := json.RawMessage(`{
		"type": "object",
		"required": ["path"],
		"properties": {
			"path": {"type": "string"}
		}
	}`)

	listSchema := json.RawMessage(`{
		"type": "object",
		"required": ["path"],
		"properties": {
			"path": {"type": "string"},
			"limit": {"type": "integer", "minimum": 1, "maximum": 1000},
			"includeHidden": {"type": "boolean"},
			"followSymlinks": {"type": "boolean"}
		}
	}`)

	readSchema := json.RawMessage(`{
		"type": "object",
		"required": ["path"],
		"properties": {
			"path": {"type": "string"},
			"offset": {"type": "integer", "minimum": 0},
			"maxBytes": {"type": "integer", "minimum": 1, "maximum": 10485760}
		}
	}`)

	writeSchema := json.RawMessage(`{
		"type": "object",
		"required": ["path", "data"],
		"properties": {
			"path": {"type": "string"},
			"data": {"type": "string"},
			"encoding": {"type": "string", "enum": ["utf8", "base64"]},
			"overwrite": {"type": "boolean"},
			"createParents": {"type": "boolean"},
			"mode": {"type": "integer", "minimum": 0, "maximum": 511}
		}
	}`)

	appendSchema := json.RawMessage(`{
		"type": "object",
		"required": ["path", "data"],
		"properties": {
			"path": {"type": "string"},
			"data": {"type": "string"},
			"encoding": {"type": "string", "enum": ["utf8", "base64"]}
		}
	}`)

	mkdirSchema := json.RawMessage(`{
		"type": "object",
		"required": ["path"],
		"properties": {
			"path": {"type": "string"},
			"recursive": {"type": "boolean"},
			"mode": {"type": "integer", "minimum": 0, "maximum": 511}
		}
	}`)

	touchSchema := json.RawMessage(`{
		"type": "object",
		"required": ["path"],
		"properties": {
			"path": {"type": "string"}
		}
	}`)

	copySchema := json.RawMessage(`{
		"type": "object",
		"required": ["source", "destination"],
		"properties": {
			"source": {"type": "string"},
			"destination": {"type": "string"},
			"recursive": {"type": "boolean"},
			"overwrite": {"type": "boolean"}
		}
	}`)

	moveSchema := json.RawMessage(`{
		"type": "object",
		"required": ["source", "destination"],
		"properties": {
			"source": {"type": "string"},
			"destination": {"type": "string"},
			"overwrite": {"type": "boolean"}
		}
	}`)

	deleteSchema := json.RawMessage(`{
		"type": "object",
		"required": ["path"],
		"properties": {
			"path": {"type": "string"},
			"recursive": {"type": "boolean"}
		}
	}`)

	searchSchema := json.RawMessage(`{
		"type": "object",
		"required": ["root", "query"],
		"properties": {
			"root": {"type": "string"},
			"path": {"type": "string"},
			"query": {"type": "string"},
			"maxDepth": {"type": "integer", "minimum": 1, "maximum": 20},
			"limit": {"type": "integer", "minimum": 1, "maximum": 500},
			"includeHidden": {"type": "boolean"},
			"followSymlinks": {"type": "boolean"}
		}
	}`)

	chmodSchema := json.RawMessage(`{
		"type": "object",
		"required": ["path", "mode"],
		"properties": {
			"path": {"type": "string"},
			"mode": {"type": "integer", "minimum": 0, "maximum": 511}
		}
	}`)

	readlinkSchema := json.RawMessage(`{
		"type": "object",
		"required": ["path"],
		"properties": {
			"path": {"type": "string"}
		}
	}`)

	symlinkSchema := json.RawMessage(`{
		"type": "object",
		"required": ["target", "linkPath"],
		"properties": {
			"target": {"type": "string"},
			"linkPath": {"type": "string"}
		}
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string"}
		}
	}`)

	readOutputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string"},
			"offset": {"type": "integer"},
			"bytesRead": {"type": "integer"},
			"content": {},
			"eof": {"type": "boolean"},
			"isBinary": {"type": "boolean"}
		}
	}`)

	listOutputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string"},
			"entries": {"type": "array"},
			"count": {"type": "integer"}
		}
	}`)

	searchOutputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"root": {"type": "string"},
			"query": {"type": "string"},
			"results": {"type": "array"},
			"count": {"type": "integer"}
		}
	}`)

	deleteOutputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string"},
			"deleted": {"type": "boolean"}
		}
	}`)

	readlinkOutputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string"},
			"target": {"type": "string"}
		}
	}`)

	runtime := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeTypeAndroidLinux,
		RuntimeID:   terminal.RuntimeIDAndroidLinux,
		HandlerName: "file.stat",
	}

	readPerm := []capability.PermissionRequirement{
		{Capability: "runtime.linux.file.read", Risk: "low"},
	}

	writePerm := []capability.PermissionRequirement{
		{Capability: "runtime.linux.file.write", Risk: "high"},
	}

	controlPerm := []capability.PermissionRequirement{
		{Capability: "runtime.linux.file.control", Risk: "high"},
	}

	return []capability.ToolDefinition{
		{
			ID:            string(statID),
			ModelName:     "android_linux__file__stat",
			Source:        capability.ToolSourceBuiltin,
			Name:          "Stat File",
			Description:   "Get file metadata (size, mode, mod time, type)",
			InputSchema:   statSchema,
			OutputSchema:  outputSchema,
			Permissions:   readPerm,
			RiskLevel:     capability.RiskLow,
			SideEffect:    capability.SideEffectReadOnly,
			HasSideEffects: false,
			Idempotent:    true,
			Retryable:     true,
			TimeoutMS:     5000,
			Runtime:       capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "file.stat"},
			ToolVersion:   capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:       true,
		},
		{
			ID:            string(listID),
			ModelName:     "android_linux__file__list",
			Source:        capability.ToolSourceBuiltin,
			Name:          "List Directory",
			Description:   "List entries in a directory",
			InputSchema:   listSchema,
			OutputSchema:  listOutputSchema,
			Permissions:   readPerm,
			RiskLevel:     capability.RiskLow,
			SideEffect:    capability.SideEffectReadOnly,
			HasSideEffects: false,
			Idempotent:    true,
			Retryable:     true,
			TimeoutMS:     10000,
			Runtime:       capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "file.list"},
			ToolVersion:   capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:       true,
		},
		{
			ID:            string(readID),
			ModelName:     "android_linux__file__read",
			Source:        capability.ToolSourceBuiltin,
			Name:          "Read File",
			Description:   "Read file content with optional offset and byte limit",
			InputSchema:   readSchema,
			OutputSchema:  readOutputSchema,
			Permissions:   readPerm,
			RiskLevel:     capability.RiskLow,
			SideEffect:    capability.SideEffectReadOnly,
			HasSideEffects: false,
			Idempotent:    true,
			Retryable:     true,
			TimeoutMS:     10000,
			Runtime:       capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "file.read"},
			ToolVersion:   capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:       true,
		},
		{
			ID:            string(writeID),
			ModelName:     "android_linux__file__write",
			Source:        capability.ToolSourceBuiltin,
			Name:          "Write File",
			Description:   "Write data to a file (atomic replace via temp file + rename)",
			InputSchema:   writeSchema,
			OutputSchema:  outputSchema,
			Permissions:   writePerm,
			RiskLevel:     capability.RiskHigh,
			SideEffect:    capability.SideEffectDestructive,
			HasSideEffects: true,
			Idempotent:    false,
			Retryable:     false,
			TimeoutMS:     15000,
			Runtime:       capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "file.write"},
			ToolVersion:   capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:       true,
		},
		{
			ID:            string(appendID),
			ModelName:     "android_linux__file__append",
			Source:        capability.ToolSourceBuiltin,
			Name:          "Append to File",
			Description:   "Append data to an existing file",
			InputSchema:   appendSchema,
			OutputSchema:  outputSchema,
			Permissions:   writePerm,
			RiskLevel:     capability.RiskHigh,
			SideEffect:    capability.SideEffectWrite,
			HasSideEffects: true,
			Idempotent:    false,
			Retryable:     false,
			TimeoutMS:     15000,
			Runtime:       capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "file.append"},
			ToolVersion:   capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:       true,
		},
		{
			ID:            string(mkdirID),
			ModelName:     "android_linux__file__mkdir",
			Source:        capability.ToolSourceBuiltin,
			Name:          "Create Directory",
			Description:   "Create a directory",
			InputSchema:   mkdirSchema,
			OutputSchema:  outputSchema,
			Permissions:   writePerm,
			RiskLevel:     capability.RiskHigh,
			SideEffect:    capability.SideEffectWrite,
			HasSideEffects: true,
			Idempotent:    true,
			Retryable:     false,
			TimeoutMS:     5000,
			Runtime:       capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "file.mkdir"},
			ToolVersion:   capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:       true,
		},
		{
			ID:            string(touchID),
			ModelName:     "android_linux__file__touch",
			Source:        capability.ToolSourceBuiltin,
			Name:          "Touch File",
			Description:   "Create an empty file or update its modification time",
			InputSchema:   touchSchema,
			OutputSchema:  outputSchema,
			Permissions:   writePerm,
			RiskLevel:     capability.RiskHigh,
			SideEffect:    capability.SideEffectWrite,
			HasSideEffects: true,
			Idempotent:    true,
			Retryable:     false,
			TimeoutMS:     5000,
			Runtime:       capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "file.touch"},
			ToolVersion:   capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:       true,
		},
		{
			ID:            string(copyID),
			ModelName:     "android_linux__file__copy",
			Source:        capability.ToolSourceBuiltin,
			Name:          "Copy File or Directory",
			Description:   "Copy a file or directory recursively",
			InputSchema:   copySchema,
			OutputSchema:  outputSchema,
			Permissions:   writePerm,
			RiskLevel:     capability.RiskHigh,
			SideEffect:    capability.SideEffectWrite,
			HasSideEffects: true,
			Idempotent:    false,
			Retryable:     false,
			TimeoutMS:     30000,
			Runtime:       capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "file.copy"},
			ToolVersion:   capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:       true,
		},
		{
			ID:            string(moveID),
			ModelName:     "android_linux__file__move",
			Source:        capability.ToolSourceBuiltin,
			Name:          "Move File or Directory",
			Description:   "Move or rename a file or directory",
			InputSchema:   moveSchema,
			OutputSchema:  outputSchema,
			Permissions:   writePerm,
			RiskLevel:     capability.RiskHigh,
			SideEffect:    capability.SideEffectDestructive,
			HasSideEffects: true,
			Idempotent:    false,
			Retryable:     false,
			TimeoutMS:     15000,
			Runtime:       capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "file.move"},
			ToolVersion:   capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:       true,
		},
		{
			ID:            string(deleteID),
			ModelName:     "android_linux__file__delete",
			Source:        capability.ToolSourceBuiltin,
			Name:          "Delete File or Directory",
			Description:   "Delete a file or directory",
			InputSchema:   deleteSchema,
			OutputSchema:  deleteOutputSchema,
			Permissions:   writePerm,
			RiskLevel:     capability.RiskHigh,
			SideEffect:    capability.SideEffectDestructive,
			HasSideEffects: true,
			Idempotent:    false,
			Retryable:     false,
			TimeoutMS:     15000,
			Runtime:       capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "file.delete"},
			ToolVersion:   capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:       true,
		},
		{
			ID:            string(searchID),
			ModelName:     "android_linux__file__search",
			Source:        capability.ToolSourceBuiltin,
			Name:          "Search Files",
			Description:   "Search for files by name pattern in a directory tree",
			InputSchema:   searchSchema,
			OutputSchema:  searchOutputSchema,
			Permissions:   readPerm,
			RiskLevel:     capability.RiskLow,
			SideEffect:    capability.SideEffectReadOnly,
			HasSideEffects: false,
			Idempotent:    true,
			Retryable:     true,
			TimeoutMS:     30000,
			Runtime:       capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "file.search"},
			ToolVersion:   capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:       true,
		},
		{
			ID:            string(chmodID),
			ModelName:     "android_linux__file__chmod",
			Source:        capability.ToolSourceBuiltin,
			Name:          "Change File Mode",
			Description:   "Change file permissions (chmod)",
			InputSchema:   chmodSchema,
			OutputSchema:  outputSchema,
			Permissions:   controlPerm,
			RiskLevel:     capability.RiskHigh,
			SideEffect:    capability.SideEffectWrite,
			HasSideEffects: true,
			Idempotent:    true,
			Retryable:     false,
			TimeoutMS:     5000,
			Runtime:       capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "file.chmod"},
			ToolVersion:   capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:       true,
		},
		{
			ID:            string(readlinkID),
			ModelName:     "android_linux__file__readlink",
			Source:        capability.ToolSourceBuiltin,
			Name:          "Read Symlink Target",
			Description:   "Read the target of a symbolic link",
			InputSchema:   readlinkSchema,
			OutputSchema:  readlinkOutputSchema,
			Permissions:   readPerm,
			RiskLevel:     capability.RiskLow,
			SideEffect:    capability.SideEffectReadOnly,
			HasSideEffects: false,
			Idempotent:    true,
			Retryable:     true,
			TimeoutMS:     5000,
			Runtime:       capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "file.readlink"},
			ToolVersion:   capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:       true,
		},
		{
			ID:            string(symlinkID),
			ModelName:     "android_linux__file__symlink",
			Source:        capability.ToolSourceBuiltin,
			Name:          "Create Symlink",
			Description:   "Create a symbolic link",
			InputSchema:   symlinkSchema,
			OutputSchema:  outputSchema,
			Permissions:   controlPerm,
			RiskLevel:     capability.RiskHigh,
			SideEffect:    capability.SideEffectWrite,
			HasSideEffects: true,
			Idempotent:    false,
			Retryable:     false,
			TimeoutMS:     5000,
			Runtime:       capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "file.symlink"},
			ToolVersion:   capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:       true,
		},
	}
}
