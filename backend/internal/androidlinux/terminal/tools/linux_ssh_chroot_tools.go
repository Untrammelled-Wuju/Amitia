//go:build linux && !android

package tools

import (
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/androidlinux/terminal"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimehost"
)

func (r *terminalToolRegistrar) RegisterSSHTools(
	host runtimehost.RuntimeHost,
	registry *capability.ToolRegistry,
) error {
	if !terminal.IsAndroidLinuxRuntime(host) {
		return nil
	}

	tools := BuildSSHTools()

	for _, tool := range tools {
		if err := registry.Register(nil, tool); err != nil {
			if err := registry.Replace(nil, tool); err != nil {
				return fmt.Errorf("register ssh tool %s: %w", tool.ID, err)
			}
		}
	}

	return nil
}

func (r *terminalToolRegistrar) RegisterChrootTools(
	host runtimehost.RuntimeHost,
	registry *capability.ToolRegistry,
) error {
	if !terminal.IsAndroidLinuxRuntime(host) {
		return nil
	}

	tools := BuildChrootTools()

	for _, tool := range tools {
		if err := registry.Register(nil, tool); err != nil {
			if err := registry.Replace(nil, tool); err != nil {
				return fmt.Errorf("register chroot tool %s: %w", tool.ID, err)
			}
		}
	}

	return nil
}

func BuildSSHTools() []capability.ToolDefinition {
	readPerm := []capability.PermissionRequirement{
		{Capability: "runtime.linux.ssh.read", Risk: "low"},
	}
	execPerm := []capability.PermissionRequirement{
		{Capability: "runtime.linux.ssh.exec", Risk: "high"},
	}

	runtime := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeTypeAndroidLinux,
		RuntimeID:   terminal.RuntimeIDAndroidLinux,
	}

	return []capability.ToolDefinition{
		{
			ID:          "builtin:android_linux:ssh.status",
			ModelName:   "android_linux__ssh__status",
			Source:      capability.ToolSourceBuiltin,
			Name:        "SSH Status",
			Description: "Check SSH client availability and configuration",
			InputSchema: json.RawMessage(`{"type": "object", "properties": {}}`),
			OutputSchema: json.RawMessage(`{"type": "object", "properties": {"enabled": {"type": "boolean"}, "defaultUser": {"type": "string"}, "knownHostsCount": {"type": "integer"}, "maxSessions": {"type": "integer"}, "activeSessions": {"type": "integer"}}}`),
			Permissions: readPerm,
			RiskLevel:   capability.RiskLow,
			SideEffect:  capability.SideEffectReadOnly,
			Idempotent:  true,
			Retryable:   true,
			TimeoutMS:   5000,
			Runtime:     capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "ssh.status"},
			ToolVersion: capability.ToolVersion{SchemaVersion: 1, Revision: "b20.2"},
			Enabled:     true,
		},
		{
			ID:          "builtin:android_linux:ssh.exec",
			ModelName:   "android_linux__ssh__exec",
			Source:      capability.ToolSourceBuiltin,
			Name:        "SSH Execute",
			Description: "Execute a command on a remote host via SSH",
			InputSchema: json.RawMessage(`{"type": "object", "required": ["host", "command"], "properties": {"host": {"type": "string"}, "port": {"type": "integer", "minimum": 1, "maximum": 65535}, "user": {"type": "string"}, "command": {"type": "string"}, "stdin": {"type": "string"}, "timeoutMs": {"type": "integer", "minimum": 1}, "maxOutputBytes": {"type": "integer", "minimum": 1}, "environment": {"type": "object"}, "workingDir": {"type": "string"}, "hostKey": {"type": "string"}, "privateKey": {"type": "string"}, "password": {"type": "string"}, "hostKeyPolicy": {"type": "string", "enum": ["reject", "accept_new"]}, "agentAuth": {"type": "boolean"}}}`),
			OutputSchema: json.RawMessage(`{"type": "object", "properties": {"exitCode": {"type": "integer"}, "stdout": {"type": "string"}, "stderr": {"type": "string"}, "stdoutTruncated": {"type": "boolean"}, "stderrTruncated": {"type": "boolean"}, "stdoutBytes": {"type": "integer"}, "stderrBytes": {"type": "integer"}, "durationMs": {"type": "integer"}}}`),
			Permissions: execPerm,
			RiskLevel:   capability.RiskHigh,
			SideEffect:  capability.SideEffectWrite,
			Idempotent:  false,
			Retryable:   false,
			TimeoutMS:   120000,
			Runtime:     capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "ssh.exec"},
			ToolVersion: capability.ToolVersion{SchemaVersion: 1, Revision: "b20.2"},
			Enabled:     true,
		},
		{
			ID:          "builtin:android_linux:ssh.hostkey.scan",
			ModelName:   "android_linux__ssh__hostkey__scan",
			Source:      capability.ToolSourceBuiltin,
			Name:        "SSH Host Key Scan",
			Description: "Scan SSH host key algorithms and fingerprints without authentication",
			InputSchema: json.RawMessage(`{"type": "object", "required": ["host"], "properties": {"host": {"type": "string"}, "port": {"type": "integer", "minimum": 1, "maximum": 65535}, "timeoutMs": {"type": "integer", "minimum": 1}}}`),
			OutputSchema: json.RawMessage(`{"type": "object", "properties": {"host": {"type": "string"}, "port": {"type": "integer"}, "algorithms": {"type": "array", "items": {"type": "string"}}, "rawKeys": {"type": "array", "items": {"type": "string"}}, "fingerprints": {"type": "array", "items": {"type": "string"}}}}`),
			Permissions: readPerm,
			RiskLevel:   capability.RiskLow,
			SideEffect:  capability.SideEffectReadOnly,
			Idempotent:  true,
			Retryable:   true,
			TimeoutMS:   15000,
			Runtime:     capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "ssh.hostkey.scan"},
			ToolVersion: capability.ToolVersion{SchemaVersion: 1, Revision: "b20.2"},
			Enabled:     true,
		},
	}
}

func BuildChrootTools() []capability.ToolDefinition {
	readPerm := []capability.PermissionRequirement{
		{Capability: "runtime.linux.chroot.read", Risk: "low"},
	}
	execPerm := []capability.PermissionRequirement{
		{Capability: "runtime.linux.chroot.exec", Risk: "high"},
	}

	runtime := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeTypeAndroidLinux,
		RuntimeID:   terminal.RuntimeIDAndroidLinux,
	}

	return []capability.ToolDefinition{
		{
			ID:          "builtin:android_linux:chroot.status",
			ModelName:   "android_linux__chroot__status",
			Source:      capability.ToolSourceBuiltin,
			Name:        "Chroot Status",
			Description: "Check chroot/rootfs capabilities and available environments",
			InputSchema: json.RawMessage(`{"type": "object", "properties": {}}`),
			OutputSchema: json.RawMessage(`{"type": "object", "properties": {"enabled": {"type": "boolean"}, "defaultRootfsPath": {"type": "string"}, "knownRootfsPaths": {"type": "array", "items": {"type": "string"}}, "maxFsBytes": {"type": "integer"}, "maxEnvironments": {"type": "integer"}, "availableEnvironments": {"type": "array", "items": {"type": "string"}}, "execBackends": {"type": "array", "items": {"type": "string"}}}}`),
			Permissions: readPerm,
			RiskLevel:   capability.RiskLow,
			SideEffect:  capability.SideEffectReadOnly,
			Idempotent:  true,
			Retryable:   true,
			TimeoutMS:   5000,
			Runtime:     capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "chroot.status"},
			ToolVersion: capability.ToolVersion{SchemaVersion: 1, Revision: "b20.2"},
			Enabled:     true,
		},
		{
			ID:          "builtin:android_linux:chroot.inspect",
			ModelName:   "android_linux__chroot__inspect",
			Source:      capability.ToolSourceBuiltin,
			Name:        "Chroot Inspect",
			Description: "Inspect a rootfs directory for validity and contents",
			InputSchema: json.RawMessage(`{"type": "object", "required": ["rootfsPath"], "properties": {"rootfsPath": {"type": "string"}}}`),
			OutputSchema: json.RawMessage(`{"type": "object", "properties": {"rootfsPath": {"type": "string"}, "exists": {"type": "boolean"}, "valid": {"type": "boolean"}, "totalBytes": {"type": "integer"}, "fileCount": {"type": "integer"}, "hasBinSh": {"type": "boolean"}, "hasBinBash": {"type": "boolean"}, "error": {"type": "string"}}}`),
			Permissions: readPerm,
			RiskLevel:   capability.RiskLow,
			SideEffect:  capability.SideEffectReadOnly,
			Idempotent:  true,
			Retryable:   true,
			TimeoutMS:   10000,
			Runtime:     capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "chroot.inspect"},
			ToolVersion: capability.ToolVersion{SchemaVersion: 1, Revision: "b20.2"},
			Enabled:     true,
		},
		{
			ID:          "builtin:android_linux:chroot.exec",
			ModelName:   "android_linux__chroot__exec",
			Source:      capability.ToolSourceBuiltin,
			Name:        "Chroot Execute",
			Description: "Execute a command inside a chroot/proot environment",
			InputSchema: json.RawMessage(`{"type": "object", "required": ["rootfsPath", "command"], "properties": {"rootfsPath": {"type": "string"}, "command": {"type": "string"}, "args": {"type": "array", "items": {"type": "string"}}, "environment": {"type": "object"}, "stdin": {"type": "string"}, "timeoutMs": {"type": "integer", "minimum": 1}, "maxOutputBytes": {"type": "integer", "minimum": 1}, "workingDir": {"type": "string"}, "user": {"type": "string"}}}`),
			OutputSchema: json.RawMessage(`{"type": "object", "properties": {"rootfsPath": {"type": "string"}, "exitCode": {"type": "integer"}, "stdout": {"type": "string"}, "stderr": {"type": "string"}, "stdoutTruncated": {"type": "boolean"}, "stderrTruncated": {"type": "boolean"}, "stdoutBytes": {"type": "integer"}, "stderrBytes": {"type": "integer"}, "durationMs": {"type": "integer"}, "environment": {"type": "string"}}}`),
			Permissions: execPerm,
			RiskLevel:   capability.RiskHigh,
			SideEffect:  capability.SideEffectWrite,
			Idempotent:  false,
			Retryable:   false,
			TimeoutMS:   300000,
			Runtime:     capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "chroot.exec"},
			ToolVersion: capability.ToolVersion{SchemaVersion: 1, Revision: "b20.2"},
			Enabled:     true,
		},
	}
}
