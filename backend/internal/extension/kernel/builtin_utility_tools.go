package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

// BuiltinUtilityToolSpec describes model-facing tool wrappers that compose
// existing Amitia capabilities into higher-level convenience operations.
type BuiltinUtilityToolSpec struct {
	id          string
	modelName   string
	name        string
	description string
	input       string
	permissions []capability.PermissionRequirement
	risk        capability.RiskLevel
	side        capability.SideEffectLevel
	approval    bool
	background  bool
	idempotent  bool
	timeout     time.Duration
	maxOutput   int
	category    string
}

func RegisterBuiltinUtilityTools(ctx context.Context, registry *capability.ToolRegistry, service *BuiltinUtilityService) error {
	if registry == nil || service == nil {
		return nil
	}
	const object = `{"type":"object","additionalProperties":true}`
	specs := []BuiltinUtilityToolSpec{
		{
			id: "builtin.read_file_part", modelName: "read_file_part", name: "Read File Part",
			description: "Read a text file by 1-based inclusive line range. Accepts a workspace URI directly or workspaceId + path/filePath.",
			input:       `{"type":"object","additionalProperties":false,"properties":{"uri":{"type":"string"},"path":{"type":"string"},"filePath":{"type":"string"},"workspaceId":{"type":"string"},"line_start":{"type":"integer","minimum":1},"line_end":{"type":"integer","minimum":1},"startLine":{"type":"integer","minimum":1},"endLine":{"type":"integer","minimum":1},"maxLines":{"type":"integer","minimum":1,"maximum":20000}}}`,
			permissions: []capability.PermissionRequirement{{Capability: "workspace.read", Risk: "low"}}, risk: capability.RiskLow, side: capability.SideEffectReadOnly, background: true, idempotent: true, timeout: 15 * time.Second, maxOutput: 512 * 1024, category: "workspace",
		},
		{
			id: "builtin.apply_file", modelName: "apply_file", name: "Apply File",
			description: "Create, overwrite, delete, apply a unified patch, or exact-match replace file content in one call. Uses Amitia workspace storage and rejects ambiguous exact replacements.",
			input:       `{"type":"object","additionalProperties":false,"properties":{"uri":{"type":"string"},"path":{"type":"string"},"filePath":{"type":"string"},"workspaceId":{"type":"string"},"operation":{"type":"string","enum":["replace","create","write","delete","patch"]},"oldText":{"type":"string"},"old_content":{"type":"string"},"newText":{"type":"string"},"new_content":{"type":"string"},"content":{"type":"string"},"expectedOccurrences":{"type":"integer","minimum":0},"recursive":{"type":"boolean"},"patch":{"type":"string"},"baseSha256":{"type":"string"}}}`,
			permissions: []capability.PermissionRequirement{{Capability: "workspace.write", Risk: "medium"}}, risk: capability.RiskMedium, side: capability.SideEffectWrite, approval: true, idempotent: false, timeout: 30 * time.Second, maxOutput: 32 * 1024, category: "workspace",
		},
		{
			id: "builtin.visit_web", modelName: "visit_web", name: "Visit Web",
			description: "Open a URL in Amitia browser automation and return extracted DOM text plus the created session/tab identifiers.",
			input:       `{"type":"object","required":["url"],"additionalProperties":false,"properties":{"url":{"type":"string","minLength":1,"maxLength":8192},"waitUntil":{"type":"string"},"timeoutMs":{"type":"integer","minimum":100,"maximum":120000},"maxDepth":{"type":"integer","minimum":1,"maximum":128},"closeAfter":{"type":"boolean"}}}`,
			permissions: []capability.PermissionRequirement{{Capability: "browser.navigate", Risk: "medium"}, {Capability: "browser.dom.read", Risk: "low"}}, risk: capability.RiskMedium, side: capability.SideEffectExternal, background: true, idempotent: false, timeout: 125 * time.Second, maxOutput: 1024 * 1024, category: "browser",
		},
		{
			id: "builtin.browser_close_all", modelName: "browser_close_all", name: "Browser Close All",
			description: "Close every Amitia browser automation session, returning per-session results without stopping after the first failure.",
			input:       `{"type":"object","additionalProperties":false,"properties":{}}`,
			permissions: []capability.PermissionRequirement{{Capability: "browser.session.manage", Risk: "medium"}}, risk: capability.RiskMedium, side: capability.SideEffectSystem, idempotent: true, timeout: 30 * time.Second, maxOutput: 64 * 1024, category: "browser",
		},
		{
			id: "builtin.browser_fill_form", modelName: "browser_fill_form", name: "Browser Fill Form",
			description: "Fill or activate multiple browser form controls in one call. Fields may target a selector or a previously returned browser element reference and can use input, select, click, checkbox, or radio actions.",
			input:       `{"type":"object","required":["sessionId","tabId","fields"],"additionalProperties":false,"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"},"fields":{"type":"array","minItems":1,"maxItems":100,"items":{"type":"object","additionalProperties":false,"properties":{"selector":{"type":"string"},"element":{"type":"object"},"action":{"type":"string","enum":["input","select","click","checkbox","radio"]},"type":{"type":"string","enum":["input","select","click","checkbox","radio"]},"value":{"type":"string"},"text":{"type":"string"}}}}}}`,
			permissions: []capability.PermissionRequirement{{Capability: "browser.interact", Risk: "high"}}, risk: capability.RiskHigh, side: capability.SideEffectExternal, approval: true, idempotent: false, timeout: 60 * time.Second, maxOutput: 256 * 1024, category: "browser",
		},
		{
			id: "builtin.bluetooth_send_and_read", modelName: "bluetooth_send_and_read", name: "Bluetooth Send And Read",
			description: "Write to an existing Bluetooth Classic RFCOMM session and immediately perform a bounded read in the same tool call.",
			input:       `{"type":"object","required":["sessionId"],"additionalProperties":false,"properties":{"sessionId":{"type":"string"},"valueBase64":{"type":"string","maxLength":100000},"valueText":{"type":"string","maxLength":65536},"maxBytes":{"type":"integer","minimum":1,"maximum":65536},"timeoutMs":{"type":"integer","minimum":100,"maximum":30000},"decodeUtf8":{"type":"boolean"}}}`,
			permissions: []capability.PermissionRequirement{{Capability: "android.bluetooth.connect", Risk: "high"}}, risk: capability.RiskHigh, side: capability.SideEffectExternal, approval: true, background: true, idempotent: false, timeout: 40 * time.Second, maxOutput: 192 * 1024, category: "android",
		},
		{
			id: "builtin.bluetooth_ble_write_and_read_characteristic", modelName: "bluetooth_ble_write_and_read_characteristic", name: "BLE Write And Read Characteristic",
			description: "Write one BLE characteristic and then read it back in the same call using an existing Amitia GATT session.",
			input:       `{"type":"object","required":["sessionId","serviceUuid","characteristicUuid"],"additionalProperties":false,"properties":{"sessionId":{"type":"string"},"serviceUuid":{"type":"string"},"characteristicUuid":{"type":"string"},"valueBase64":{"type":"string","maxLength":1024},"valueText":{"type":"string","maxLength":512},"withoutResponse":{"type":"boolean"},"timeoutMs":{"type":"integer","minimum":1000,"maximum":20000}}}`,
			permissions: []capability.PermissionRequirement{{Capability: "android.bluetooth.connect", Risk: "high"}}, risk: capability.RiskHigh, side: capability.SideEffectExternal, approval: true, background: true, idempotent: false, timeout: 45 * time.Second, maxOutput: 192 * 1024, category: "android",
		},
		{
			id: "builtin.press_key", modelName: "press_key", name: "Press Key",
			description: "Press an Android key by symbolic name or Android key code. Uses normal global/media actions where available and authorized Shizuku/Root input keyevent for arbitrary codes.",
			input:       `{"type":"object","additionalProperties":false,"properties":{"key":{"type":"string"},"keyCode":{"type":"integer","minimum":0,"maximum":10000},"key_code":{"type":"integer","minimum":0,"maximum":10000}}}`,
			permissions: []capability.PermissionRequirement{{Capability: "android.interaction.global", Risk: "high"}, {Capability: "android.root.execute", Risk: "high"}}, risk: capability.RiskHigh, side: capability.SideEffectSystem, approval: true, idempotent: false, timeout: 12 * time.Second, maxOutput: 16 * 1024, category: "android",
		},
		{
			id: "builtin.combined_operation", modelName: "combined_operation", name: "Combined UI Operation",
			description: "Execute a bounded sequence of Android UI operations (tap/click, input text, swipe, press key, wait, or an explicit safe interaction/device operation) in order.",
			input:       `{"type":"object","required":["operations"],"additionalProperties":false,"properties":{"operations":{"type":"array","minItems":1,"maxItems":64,"items":{"type":"object","additionalProperties":true}},"stopOnError":{"type":"boolean"}}}`,
			permissions: []capability.PermissionRequirement{{Capability: "android.interaction.global", Risk: "high"}, {Capability: "android.interaction.click", Risk: "medium"}, {Capability: "android.interaction.input", Risk: "high"}, {Capability: "android.interaction.gesture", Risk: "medium"}, {Capability: "android.interaction.coordinate", Risk: "medium"}, {Capability: "android.root.execute", Risk: "high"}}, risk: capability.RiskHigh, side: capability.SideEffectSystem, approval: true, idempotent: false, timeout: 120 * time.Second, maxOutput: 512 * 1024, category: "android",
		},
		{
			id: "builtin.execute_terminal", modelName: "execute_terminal", name: "Execute Terminal",
			description: "Execute a one-shot command in Amitia Android Linux. When ssh_login is active in the current scope and environment=linux/ssh, transparently routes the command through that SSH target.",
			input:       `{"type":"object","required":["command"],"additionalProperties":false,"properties":{"command":{"type":"string","minLength":1,"maxLength":131072},"environment":{"type":"string","enum":["linux","local","ssh"]},"cwd":{"type":"string"},"workingDir":{"type":"string"},"timeoutMs":{"type":"integer","minimum":100,"maximum":300000},"maxOutputBytes":{"type":"integer","minimum":1,"maximum":4194304}}}`,
			permissions: []capability.PermissionRequirement{{Capability: "runtime.linux.shell.execute", Risk: "high"}, {Capability: "runtime.linux.ssh.exec", Risk: "high"}}, risk: capability.RiskHigh, side: capability.SideEffectSystem, approval: true, idempotent: false, timeout: 305 * time.Second, maxOutput: 4 * 1024 * 1024, category: "terminal",
		},
		{
			id: "builtin.execute_in_terminal_session_streaming", modelName: "execute_in_terminal_session_streaming", name: "Execute In Terminal Session Streaming",
			description: "Write a command to an interactive Android Linux terminal session and collect bounded incremental output chunks; opens a session when sessionId is omitted.",
			input:       `{"type":"object","required":["command"],"additionalProperties":false,"properties":{"sessionId":{"type":"string"},"command":{"type":"string","minLength":1,"maxLength":65536},"afterSequence":{"type":"integer","minimum":0},"waitMs":{"type":"integer","minimum":0,"maximum":5000},"maxBytes":{"type":"integer","minimum":1,"maximum":1048576},"maxReads":{"type":"integer","minimum":1,"maximum":32},"shell":{"type":"string"},"cwd":{"type":"string"}}}`,
			permissions: []capability.PermissionRequirement{{Capability: "runtime.linux.terminal.control", Risk: "high"}, {Capability: "runtime.linux.terminal.read", Risk: "medium"}}, risk: capability.RiskHigh, side: capability.SideEffectSystem, approval: true, idempotent: false, timeout: 90 * time.Second, maxOutput: 2 * 1024 * 1024, category: "terminal",
		},
		{
			id: "builtin.get_terminal_session_screen", modelName: "get_terminal_session_screen", name: "Get Terminal Session Screen",
			description: "Replay buffered terminal output with common ANSI cursor/erase controls and return a rendered rows/cols screen snapshot plus session status.",
			input:       `{"type":"object","required":["sessionId"],"additionalProperties":false,"properties":{"sessionId":{"type":"string"},"afterSequence":{"type":"integer","minimum":0},"maxBytes":{"type":"integer","minimum":1,"maximum":4194304},"waitMs":{"type":"integer","minimum":0,"maximum":5000}}}`,
			permissions: []capability.PermissionRequirement{{Capability: "runtime.linux.terminal.read", Risk: "medium"}}, risk: capability.RiskMedium, side: capability.SideEffectReadOnly, background: true, idempotent: true, timeout: 15 * time.Second, maxOutput: 4 * 1024 * 1024, category: "terminal",
		},
		{
			id: "builtin.ssh_login", modelName: "ssh_login", name: "SSH Login",
			description: "Validate and save an SSH target for the current Amitia user/conversation scope. Subsequent execute_terminal calls with environment=linux or ssh route through it until ssh_exit.",
			input:       `{"type":"object","required":["host","user"],"additionalProperties":false,"properties":{"host":{"type":"string"},"port":{"type":"integer","minimum":1,"maximum":65535},"user":{"type":"string"},"password":{"type":"string"},"privateKey":{"type":"string"},"hostKey":{"type":"string"},"hostKeyPolicy":{"type":"string","enum":["reject","accept_new"]},"agentAuth":{"type":"boolean"},"timeoutMs":{"type":"integer","minimum":100,"maximum":120000}}}`,
			permissions: []capability.PermissionRequirement{{Capability: "runtime.linux.ssh.exec", Risk: "high"}}, risk: capability.RiskHigh, side: capability.SideEffectExternal, approval: true, idempotent: false, timeout: 125 * time.Second, maxOutput: 64 * 1024, category: "terminal",
		},
		{
			id: "builtin.ssh_exit", modelName: "ssh_exit", name: "SSH Exit",
			description: "Clear the SSH target saved by ssh_login for the current Amitia user/conversation scope and restore local Android Linux command routing.",
			input:       `{"type":"object","additionalProperties":false,"properties":{}}`,
			permissions: []capability.PermissionRequirement{{Capability: "runtime.linux.ssh.exec", Risk: "high"}}, risk: capability.RiskMedium, side: capability.SideEffectSystem, idempotent: true, timeout: 5 * time.Second, maxOutput: 8 * 1024, category: "terminal",
		},
	}

	for _, spec := range specs {
		if !service.Supports(spec.modelName) {
			continue
		}
		if _, exists := registry.GetByModelName(ctx, spec.modelName); exists {
			continue
		}
		def := capability.ToolDefinition{
			ID:             spec.id,
			ModelName:      spec.modelName,
			Source:         capability.ToolSourceBuiltin,
			Name:           spec.name,
			Description:    spec.description,
			InputSchema:    json.RawMessage(spec.input),
			OutputSchema:   json.RawMessage(object),
			Permissions:    spec.permissions,
			RiskLevel:      spec.risk,
			SideEffect:     spec.side,
			HasSideEffects: spec.side != capability.SideEffectReadOnly && spec.side != capability.SideEffectNone,
			Idempotent:     spec.idempotent,
			Retryable:      spec.idempotent,
			TimeoutMS:      spec.timeout.Milliseconds(),
			Enabled:        true,
			Compatible:     true,
			ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "builtin-v1"},
			ModelExposure: capability.ModelExposureRule{
				ExposedByDefault: true,
				Categories:       []string{spec.category},
				Priority:         46,
			},
			ExecutionPolicy: capability.ToolExecutionPolicy{
				Timeout:          spec.timeout,
				MaxConcurrency:   2,
				Idempotent:       spec.idempotent,
				ApprovalRequired: spec.approval,
				AllowBackground:  spec.background,
			},
			ResultPolicy: capability.ToolResultPolicy{SanitizeError: true, MaxOutputBytes: spec.maxOutput},
			Runtime: capability.RuntimeBinding{
				RuntimeType: capability.RuntimeTypeBuiltin,
				RuntimeID:   "builtin",
				HandlerName: spec.modelName,
			},
		}
		if err := registry.Register(ctx, def); err != nil {
			return fmt.Errorf("register builtin utility tool %s: %w", spec.modelName, err)
		}
	}
	return nil
}
