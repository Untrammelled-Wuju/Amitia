package main

import (
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func validWorkerRole(role string) bool {
	switch strings.TrimSpace(role) {
	case "", "research_worker", "coding_worker", "review_worker", "operator_worker":
		return true
	default:
		return false
	}
}

func workerToolAllowed(role string, definition capability.ToolDefinition) bool {
	role = strings.TrimSpace(role)
	if role == "" {
		return true
	}
	name := strings.ToLower(definition.ModelName + " " + definition.Name + " " + definition.ID)
	readLike := containsAnyToolText(name, "read", "search", "list", "get", "inspect", "status", "diff", "query", "view", "snapshot")
	mutating := containsAnyToolText(name, "write", "edit", "apply", "patch", "delete", "remove", "create", "upload", "send", "execute", "shell", "terminal", "install", "update", "revoke", "approve")
	deviceOperation := containsAnyToolText(name, "android", "ios_native", "ios.", "device.", "device_", "camera", "microphone", "clipboard", "notification", "location", "bluetooth", "wifi", "nfc", "telephony", "contact", "calendar", "health", "usage_stats")
	switch role {
	case "research_worker":
		if definition.SideEffect == capability.SideEffectNone || definition.SideEffect == capability.SideEffectReadOnly {
			return !mutating
		}
		return definition.SideEffect == "" && readLike && !mutating
	case "coding_worker":
		if definition.SideEffect == capability.SideEffectDestructive ||
			definition.SideEffect == capability.SideEffectFinancial ||
			definition.SideEffect == capability.SideEffectSystem ||
			deviceOperation {
			return false
		}
		return definition.SideEffect != capability.SideEffectExternal || !deviceOperation
	case "review_worker":
		if definition.SideEffect == capability.SideEffectNone || definition.SideEffect == capability.SideEffectReadOnly {
			return !mutating || containsAnyToolText(name, "test", "check", "build")
		}
		return definition.SideEffect == "" && readLike && !mutating
	case "operator_worker":
		if containsAnyToolText(name, "workspace.write", "workspace_write", "workspace.edit", "workspace_edit", "workspace.apply", "workspace_patch", "git.commit", "git.push") {
			return false
		}
		return definition.SideEffect != capability.SideEffectDestructive &&
			definition.SideEffect != capability.SideEffectFinancial &&
			definition.SideEffect != capability.SideEffectWrite
	default:
		return false
	}
}

func containsAnyToolText(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
