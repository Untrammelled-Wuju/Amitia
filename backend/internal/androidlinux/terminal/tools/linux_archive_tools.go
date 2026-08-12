//go:build linux && !android

package tools

import (
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/androidlinux/terminal"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimehost"
)

func (r *terminalToolRegistrar) RegisterArchiveTools(
	host runtimehost.RuntimeHost,
	registry *capability.ToolRegistry,
) error {
	if !terminal.IsAndroidLinuxRuntime(host) {
		return nil
	}

	tools := BuildArchiveTools()

	for _, tool := range tools {
		if err := registry.Register(nil, tool); err != nil {
			if err := registry.Replace(nil, tool); err != nil {
				return fmt.Errorf("register archive tool %s: %w", tool.ID, err)
			}
		}
	}

	return nil
}

func BuildArchiveTools() []capability.ToolDefinition {
	providerID := "android_linux"
	ns := "archive"

	detectID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, ns+".detect")
	listID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, ns+".list")
	extractID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, ns+".extract")
	createID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, ns+".create")
	verifyID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, ns+".verify")

	readPerm := []capability.PermissionRequirement{
		{Capability: "runtime.linux.archive.read", Risk: "low"},
	}
	writePerm := []capability.PermissionRequirement{
		{Capability: "runtime.linux.archive.write", Risk: "medium"},
	}

	runtime := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeTypeAndroidLinux,
		RuntimeID:   terminal.RuntimeIDAndroidLinux,
		HandlerName: "archive.detect",
	}

	return []capability.ToolDefinition{
		{
			ID:             string(detectID),
			ModelName:      "android_linux__archive__detect",
			Source:         capability.ToolSourceBuiltin,
			Name:           "Archive Detect",
			Description:    "Detect archive format and metadata from file contents",
			InputSchema:    json.RawMessage(`{"type": "object", "required": ["path"], "properties": {"path": {"type": "string"}}}`),
			OutputSchema:   json.RawMessage(`{"type": "object", "properties": {"path": {"type": "string"}, "format": {"type": "string"}, "mimeType": {"type": "string"}, "archive": {"type": "boolean"}, "compressed": {"type": "boolean"}, "sizeBytes": {"type": "integer"}, "entryCount": {"type": "integer"}, "encrypted": {"type": "boolean"}}}`),
			Permissions:    readPerm,
			RiskLevel:      capability.RiskLow,
			SideEffect:     capability.SideEffectReadOnly,
			HasSideEffects: false,
			Idempotent:     true,
			Retryable:      true,
			TimeoutMS:      10000,
			Runtime:        capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "archive.detect"},
			ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b20.2"},
			Enabled:        true,
		},
		{
			ID:             string(listID),
			ModelName:      "android_linux__archive__list",
			Source:         capability.ToolSourceBuiltin,
			Name:           "Archive List",
			Description:    "List entries inside an archive with pagination",
			InputSchema:    json.RawMessage(`{"type": "object", "required": ["path"], "properties": {"path": {"type": "string"}, "limit": {"type": "integer", "minimum": 1}, "offset": {"type": "integer", "minimum": 0}, "includeDirectories": {"type": "boolean"}}}`),
			OutputSchema:   json.RawMessage(`{"type": "object", "properties": {"path": {"type": "string"}, "entries": {"type": "array"}, "count": {"type": "integer"}, "totalCount": {"type": "integer"}, "limit": {"type": "integer"}, "offset": {"type": "integer"}}}`),
			Permissions:    readPerm,
			RiskLevel:      capability.RiskLow,
			SideEffect:     capability.SideEffectReadOnly,
			HasSideEffects: false,
			Idempotent:     true,
			Retryable:      true,
			TimeoutMS:      30000,
			Runtime:        capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "archive.list"},
			ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b20.2"},
			Enabled:        true,
		},
		{
			ID:             string(extractID),
			ModelName:      "android_linux__archive__extract",
			Source:         capability.ToolSourceBuiltin,
			Name:           "Archive Extract",
			Description:    "Extract archive contents to a target directory with safety controls",
			InputSchema:    json.RawMessage(`{"type": "object", "required": ["path", "target"], "properties": {"path": {"type": "string"}, "target": {"type": "string"}, "overwrite": {"type": "boolean"}, "stripComponents": {"type": "integer", "minimum": 0}, "include": {"type": "array", "items": {"type": "string"}}, "exclude": {"type": "array", "items": {"type": "string"}}, "allowSymlinks": {"type": "boolean"}, "maxEntries": {"type": "integer", "minimum": 1}, "maxBytes": {"type": "integer", "minimum": 1}}}`),
			OutputSchema:   json.RawMessage(`{"type": "object", "properties": {"path": {"type": "string"}, "target": {"type": "string"}, "entryCount": {"type": "integer"}, "totalBytes": {"type": "integer"}}}`),
			Permissions:    writePerm,
			RiskLevel:      capability.RiskMedium,
			SideEffect:     capability.SideEffectWrite,
			HasSideEffects: true,
			Idempotent:     false,
			Retryable:      false,
			TimeoutMS:      300000,
			Runtime:        capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "archive.extract"},
			ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b20.2"},
			Enabled:        true,
		},
		{
			ID:             string(createID),
			ModelName:      "android_linux__archive__create",
			Source:         capability.ToolSourceBuiltin,
			Name:           "Archive Create",
			Description:    "Create a new archive from files and directories",
			InputSchema:    json.RawMessage(`{"type": "object", "required": ["sources", "target"], "properties": {"sources": {"type": "array", "items": {"type": "string"}}, "target": {"type": "string"}, "format": {"type": "string", "enum": ["zip", "tar", "tar.gz", "tar.bz2", "tar.xz", "tar.zst"]}, "compressionLevel": {"type": "integer", "minimum": 1, "maximum": 9}, "basePath": {"type": "string"}, "includeHidden": {"type": "boolean"}, "followSymlinks": {"type": "boolean"}, "overwrite": {"type": "boolean"}}}`),
			OutputSchema:   json.RawMessage(`{"type": "object", "properties": {"target": {"type": "string"}, "format": {"type": "string"}, "entryCount": {"type": "integer"}, "totalBytes": {"type": "integer"}}}`),
			Permissions:    writePerm,
			RiskLevel:      capability.RiskMedium,
			SideEffect:     capability.SideEffectWrite,
			HasSideEffects: true,
			Idempotent:     false,
			Retryable:      false,
			TimeoutMS:      300000,
			Runtime:        capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "archive.create"},
			ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b20.2"},
			Enabled:        true,
		},
		{
			ID:             string(verifyID),
			ModelName:      "android_linux__archive__verify",
			Source:         capability.ToolSourceBuiltin,
			Name:           "Archive Verify",
			Description:    "Verify archive integrity and safety without extracting",
			InputSchema:    json.RawMessage(`{"type": "object", "required": ["path"], "properties": {"path": {"type": "string"}}}`),
			OutputSchema:   json.RawMessage(`{"type": "object", "properties": {"valid": {"type": "boolean"}, "format": {"type": "string"}, "entryCount": {"type": "integer"}, "totalUncompressedBytes": {"type": "integer"}, "unsafeEntries": {"type": "integer"}, "corruptEntries": {"type": "integer"}, "encryptedEntries": {"type": "integer"}, "warnings": {"type": "array", "items": {"type": "string"}}}}`),
			Permissions:    readPerm,
			RiskLevel:      capability.RiskLow,
			SideEffect:     capability.SideEffectReadOnly,
			HasSideEffects: false,
			Idempotent:     true,
			Retryable:      true,
			TimeoutMS:      60000,
			Runtime:        capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "archive.verify"},
			ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b20.2"},
			Enabled:        true,
		},
	}
}
