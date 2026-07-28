package host_api

import (
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

type PermissionMappingEntry struct {
	PermissionID string
	Resource     string
}

func DefaultPermissionMapping() map[Method][]PermissionMappingEntry {
	return map[Method][]PermissionMappingEntry{
		MethodStateGet:       {{PermissionID: "storage.state.read", Resource: "state"}},
		MethodStateCAS:       {{PermissionID: "storage.state.write", Resource: "state"}},
		MethodStateDelete:    {{PermissionID: "storage.state.write", Resource: "state"}},
		MethodStateList:      {{PermissionID: "storage.state.read", Resource: "state"}},
		MethodSecretGet:      {{PermissionID: "secret.read", Resource: "secret"}},
		MethodResourceOpen:   {{PermissionID: "resource.read", Resource: "resource"}},
		MethodResourceRead:   {{PermissionID: "resource.read", Resource: "resource"}},
		MethodResourceWrite:  {{PermissionID: "resource.write", Resource: "resource"}},
		MethodEventEmit:      {{PermissionID: "event.emit", Resource: "event"}},
		MethodEventSubscribe: {{PermissionID: "event.subscribe", Resource: "event"}},
		MethodScheduleCreate: {{PermissionID: "schedule.create", Resource: "schedule"}},
		MethodScheduleCancel: {{PermissionID: "schedule.cancel", Resource: "schedule"}},
		MethodToolExecute:    {{PermissionID: "tool.invoke", Resource: "tool"}},
		MethodCharacterRead:  {{PermissionID: "character.read", Resource: "character"}},
		MethodConversationRead: {{PermissionID: "conversation.read", Resource: "conversation"}},
		MethodMemoryQuery:    {{PermissionID: "memory.read", Resource: "memory"}},
		MethodProviderInvoke: {{PermissionID: "provider.invoke", Resource: "provider"}},
		MethodUINotify:       {{PermissionID: "ui.notify", Resource: "ui"}},
	}
}

func RoutePermissionForMethod(method Method) []PermissionRequirement {
	entries, ok := DefaultPermissionMapping()[method]
	if !ok {
		return nil
	}
	out := make([]PermissionRequirement, 0, len(entries))
	for _, e := range entries {
		out = append(out, PermissionRequirement{Name: e.PermissionID, Resource: e.Resource})
	}
	return out
}

func RegisterPermissionDefinitions(registry *permission.PermissionDefinitionRegistry) {
	defs := []permission.PermissionDefinition{
		{ID: "storage.state.read", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}},
		{ID: "storage.state.write", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}},
		{ID: "secret.read", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}},
		{ID: "resource.read", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule, permission.ScopeResource}},
		{ID: "resource.write", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule, permission.ScopeResource}},
		{ID: "event.emit", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}},
		{ID: "event.subscribe", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}},
		{ID: "schedule.create", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}},
		{ID: "schedule.cancel", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}},
		{ID: "tool.invoke", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule, permission.ScopeTool}},
		{ID: "character.read", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeCharacter}},
		{ID: "conversation.read", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeConversation, permission.ScopeCharacter}},
		{ID: "memory.read", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeCharacter, permission.ScopeConversation}},
		{ID: "provider.invoke", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension}},
		{ID: "ui.notify", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeSession}},
	}
	for _, d := range defs {
		registry.Register(d)
	}
}
