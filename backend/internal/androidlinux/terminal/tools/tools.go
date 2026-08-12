//go:build linux && !android

package tools

import (
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/androidlinux/terminal"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimehost"
)

type ToolRegistrar interface {
	RegisterTerminalTools(
		host runtimehost.RuntimeHost,
		registry *capability.ToolRegistry,
	) error
}

type terminalToolRegistrar struct{}

func NewToolRegistrar() ToolRegistrar {
	return &terminalToolRegistrar{}
}

func (r *terminalToolRegistrar) RegisterTerminalTools(
	host runtimehost.RuntimeHost,
	registry *capability.ToolRegistry,
) error {
	if !terminal.IsAndroidLinuxRuntime(host) {
		return nil
	}

	tools := BuildTerminalTools()

	for _, tool := range tools {
		if err := registry.Register(nil, tool); err != nil {
			if err := registry.Replace(nil, tool); err != nil {
				return fmt.Errorf("register terminal tool %s: %w", tool.ID, err)
			}
		}
	}

	return nil
}

func BuildTerminalTools() []capability.ToolDefinition {
	providerID := "android_linux"
	namespace := "terminal"

	openID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".open")
	writeID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".write")
	readID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".read")
	resizeID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".resize")
	statusID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".status")
	closeID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".close")
	cancelID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, namespace+".cancel")

	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"shell": {"type": "string"},
			"cwd": {"type": "string"},
			"rows": {"type": "integer", "minimum": 1, "maximum": 1000},
			"cols": {"type": "integer", "minimum": 1, "maximum": 1000}
		}
	}`)

	writeInputSchema := json.RawMessage(`{
		"type": "object",
		"required": ["sessionId"],
		"properties": {
			"sessionId": {"type": "string"},
			"text": {"type": "string"},
			"data": {"type": "string"}
		}
	}`)

	readInputSchema := json.RawMessage(`{
		"type": "object",
		"required": ["sessionId"],
		"properties": {
			"sessionId": {"type": "string"},
			"afterSequence": {"type": "integer"},
			"maxBytes": {"type": "integer"},
			"waitMs": {"type": "integer", "minimum": 0, "maximum": 5000}
		}
	}`)

	resizeInputSchema := json.RawMessage(`{
		"type": "object",
		"required": ["sessionId", "rows", "cols"],
		"properties": {
			"sessionId": {"type": "string"},
			"rows": {"type": "integer", "minimum": 1, "maximum": 1000},
			"cols": {"type": "integer", "minimum": 1, "maximum": 1000}
		}
	}`)

	statusInputSchema := json.RawMessage(`{
		"type": "object",
		"required": ["sessionId"],
		"properties": {
			"sessionId": {"type": "string"}
		}
	}`)

	runtime := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeTypeAndroidLinux,
		RuntimeID:   terminal.RuntimeIDAndroidLinux,
		HandlerName: "terminal.open",
	}

	return []capability.ToolDefinition{
		{
			ID:          string(openID),
			ModelName:   "android_linux__terminal__open",
			Source:      capability.ToolSourceBuiltin,
			Name:        "Open Android Linux Terminal",
			Description: "Open an interactive terminal session on Android Linux Guest",
			InputSchema: inputSchema,
			OutputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"sessionId": {"type": "string"},
					"state": {"type": "string"},
					"rows": {"type": "integer"},
					"cols": {"type": "integer"}
				}
			}`),
			Permissions: []capability.PermissionRequirement{
				{Capability: "runtime.linux.terminal.control", Risk: "high"},
			},
			RiskLevel:         capability.RiskMedium,
			SideEffect:        capability.SideEffectSystem,
			HasSideEffects:    true,
			Idempotent:        false,
			Retryable:         false,
			TimeoutMS:         30000,
			Runtime:           capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "terminal.open"},
			ToolVersion:       capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:           true,
		},
		{
			ID:          string(writeID),
			ModelName:   "android_linux__terminal__write",
			Source:      capability.ToolSourceBuiltin,
			Name:        "Write to Terminal",
			Description: "Write data to an interactive terminal session",
			InputSchema: writeInputSchema,
			OutputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"accepted": {"type": "boolean"},
					"bytesWritten": {"type": "integer"}
				}
			}`),
			Permissions: []capability.PermissionRequirement{
				{Capability: "runtime.linux.terminal.control", Risk: "high"},
			},
			RiskLevel:         capability.RiskMedium,
			SideEffect:        capability.SideEffectSystem,
			HasSideEffects:    true,
			Idempotent:        false,
			Retryable:         false,
			TimeoutMS:         10000,
			Runtime:           capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "terminal.write"},
			ToolVersion:       capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:           true,
		},
		{
			ID:          string(readID),
			ModelName:   "android_linux__terminal__read",
			Source:      capability.ToolSourceBuiltin,
			Name:        "Read Terminal Output",
			Description: "Read output from an interactive terminal session",
			InputSchema: readInputSchema,
			OutputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"sessionId": {"type": "string"},
					"chunks": {"type": "array"},
					"nextSequence": {"type": "integer"},
					"truncated": {"type": "boolean"},
					"state": {"type": "string"}
				}
			}`),
			Permissions: []capability.PermissionRequirement{
				{Capability: "runtime.linux.terminal.read", Risk: "medium"},
			},
			RiskLevel:         capability.RiskLow,
			SideEffect:        capability.SideEffectReadOnly,
			HasSideEffects:    false,
			Idempotent:        true,
			Retryable:         true,
			TimeoutMS:         5000,
			Runtime:           capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "terminal.read"},
			ToolVersion:       capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:           true,
		},
		{
			ID:          string(resizeID),
			ModelName:   "android_linux__terminal__resize",
			Source:      capability.ToolSourceBuiltin,
			Name:        "Resize Terminal",
			Description: "Resize an interactive terminal session",
			InputSchema: resizeInputSchema,
			OutputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"rows": {"type": "integer"},
					"cols": {"type": "integer"}
				}
			}`),
			Permissions: []capability.PermissionRequirement{
				{Capability: "runtime.linux.terminal.control", Risk: "high"},
			},
			RiskLevel:         capability.RiskLow,
			SideEffect:        capability.SideEffectWrite,
			HasSideEffects:    true,
			Idempotent:        true,
			Retryable:         false,
			TimeoutMS:         5000,
			Runtime:           capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "terminal.resize"},
			ToolVersion:       capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:           true,
		},
		{
			ID:          string(statusID),
			ModelName:   "android_linux__terminal__status",
			Source:      capability.ToolSourceBuiltin,
			Name:        "Terminal Session Status",
			Description: "Get status of an interactive terminal session",
			InputSchema: statusInputSchema,
			OutputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"sessionId": {"type": "string"},
					"state": {"type": "string"},
					"exitCode": {"type": "integer"},
					"rows": {"type": "integer"},
					"cols": {"type": "integer"}
				}
			}`),
			Permissions: []capability.PermissionRequirement{
				{Capability: "runtime.linux.terminal.read", Risk: "medium"},
			},
			RiskLevel:         capability.RiskLow,
			SideEffect:        capability.SideEffectReadOnly,
			HasSideEffects:    false,
			Idempotent:        true,
			Retryable:         true,
			TimeoutMS:         5000,
			Runtime:           capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "terminal.status"},
			ToolVersion:       capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:           true,
		},
		{
			ID:          string(closeID),
			ModelName:   "android_linux__terminal__close",
			Source:      capability.ToolSourceBuiltin,
			Name:        "Close Terminal Session",
			Description: "Gracefully close an interactive terminal session",
			InputSchema: statusInputSchema,
			OutputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"state": {"type": "string"}
				}
			}`),
			Permissions: []capability.PermissionRequirement{
				{Capability: "runtime.linux.terminal.control", Risk: "high"},
			},
			RiskLevel:         capability.RiskLow,
			SideEffect:        capability.SideEffectWrite,
			HasSideEffects:    true,
			Idempotent:        true,
			Retryable:         false,
			TimeoutMS:         10000,
			Runtime:           capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "terminal.close"},
			ToolVersion:       capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:           true,
		},
		{
			ID:          string(cancelID),
			ModelName:   "android_linux__terminal__cancel",
			Source:      capability.ToolSourceBuiltin,
			Name:        "Cancel Terminal Session",
			Description: "Force cancel an interactive terminal session",
			InputSchema: statusInputSchema,
			OutputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"state": {"type": "string"}
				}
			}`),
			Permissions: []capability.PermissionRequirement{
				{Capability: "runtime.linux.terminal.control", Risk: "high"},
			},
			RiskLevel:         capability.RiskLow,
			SideEffect:        capability.SideEffectSystem,
			HasSideEffects:    true,
			Idempotent:        true,
			Retryable:         false,
			TimeoutMS:         10000,
			Runtime:           capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "terminal.cancel"},
			ToolVersion:       capability.ToolVersion{SchemaVersion: 1, Revision: "b20.1"},
			Enabled:           true,
		},
	}
}
