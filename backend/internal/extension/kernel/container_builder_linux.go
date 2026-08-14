//go:build linux && !android

package kernel

import (
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/androidlinux/shell"
	"github.com/u-ai/backend/internal/androidlinux/terminal"
	terminaltools "github.com/u-ai/backend/internal/androidlinux/terminal/tools"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimehost"
)

func (b *ContainerBuilder) WithAndroidLinuxProvider(
	provider terminal.AndroidLinuxProvider,
) *ContainerBuilder {
	b.androidLinuxProvider = provider
	return b
}

func registerTerminalTools(host runtimehost.RuntimeHost, provider interface{}, toolRegistry *capability.ToolRegistry) error {
	if _, ok := provider.(terminal.AndroidLinuxProvider); !ok {
		return nil
	}
	registrar := terminaltools.NewToolRegistrar()
	if err := registrar.RegisterTerminalTools(host, toolRegistry); err != nil {
		return fmt.Errorf("kernel: register terminal tools: %w", err)
	}

	if err := registerShellTools(toolRegistry); err != nil {
		return fmt.Errorf("kernel: register shell tools: %w", err)
	}

	return nil
}

func registerShellTools(toolRegistry *capability.ToolRegistry) error {
	execID := capability.BuildToolID(capability.ToolSourceBuiltin, "android_linux", "shell.exec")

	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"mode": {"type": "string", "enum": ["argv", "shell"]},
			"executable": {"type": "string"},
			"command": {"type": "string"},
			"args": {"type": "array", "items": {"type": "string"}},
			"workingDir": {"type": "string"},
			"environment": {"type": "object", "additionalProperties": {"type": "string"}},
			"stdin": {"type": "string"},
			"timeoutMs": {"type": "integer", "minimum": 1, "maximum": 300000},
			"maxOutputBytes": {"type": "integer", "minimum": 1}
		}
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"exitCode": {"type": "integer"},
			"stdout": {"type": "string"},
			"stderr": {"type": "string"},
			"stdoutTruncated": {"type": "boolean"},
			"stderrTruncated": {"type": "boolean"},
			"stdoutBytes": {"type": "integer"},
			"stderrBytes": {"type": "integer"},
			"durationMs": {"type": "integer"},
			"timedOut": {"type": "boolean"},
			"signal": {"type": "string"},
			"workingDir": {"type": "string"}
		}
	}`)

	tool := capability.ToolDefinition{
		ID:           string(execID),
		ModelName:    "android_linux__shell__exec",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Execute Shell Command",
		Description:  "Execute a one-shot shell command on Android Linux Guest without interactive terminal",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "runtime.linux.shell.execute", Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      30000,
		Runtime: capability.RuntimeBinding{
			RuntimeType: capability.RuntimeTypeAndroidLinux,
			RuntimeID:   shell.ShellRuntimeID,
			HandlerName: "shell.exec",
		},
		ToolVersion: capability.ToolVersion{SchemaVersion: 1, Revision: "b21.1"},
		Enabled:     true,
	}

	if err := toolRegistry.Register(nil, tool); err != nil {
		if err := toolRegistry.Replace(nil, tool); err != nil {
			return fmt.Errorf("register shell tool %s: %w", tool.ID, err)
		}
	}

	return nil
}
