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
		MethodStateGet:         {{PermissionID: "storage.state.read", Resource: "state"}},
		MethodStateCAS:         {{PermissionID: "storage.state.write", Resource: "state"}},
		MethodStateDelete:      {{PermissionID: "storage.state.write", Resource: "state"}},
		MethodStateList:        {{PermissionID: "storage.state.read", Resource: "state"}},
		MethodSecretGet:        {{PermissionID: "secret.read", Resource: "secret"}},
		MethodResourceOpen:     {{PermissionID: "resource.read", Resource: "resource"}},
		MethodResourceRead:     {{PermissionID: "resource.read", Resource: "resource"}},
		MethodResourceWrite:    {{PermissionID: "resource.write", Resource: "resource"}},
		MethodResourceClose:    {{PermissionID: "resource.read", Resource: "resource"}},
		MethodResourceStat:     {{PermissionID: "resource.read", Resource: "resource"}},
		MethodEventEmit:        {{PermissionID: "event.emit", Resource: "event"}},
		MethodEventSubscribe:   {{PermissionID: "event.subscribe", Resource: "event"}},
		MethodEventUnsubscribe: {{PermissionID: "event.subscribe", Resource: "event"}},
		MethodScheduleCreate:   {{PermissionID: "schedule.create", Resource: "schedule"}},
		MethodScheduleCancel:   {{PermissionID: "schedule.manage", Resource: "schedule"}},
		MethodScheduleList:     {{PermissionID: "schedule.create", Resource: "schedule"}},
		MethodToolExecute:      {{PermissionID: "tool.invoke", Resource: "tool"}},
		MethodCharacterRead:    {{PermissionID: "character.read", Resource: "character"}},
		MethodConversationRead: {{PermissionID: "conversation.read", Resource: "conversation"}},
		MethodMemoryQuery:      {{PermissionID: "memory.read", Resource: "memory"}},
		MethodProviderInvoke:   {{PermissionID: "provider.invoke", Resource: "provider"}},
		MethodUINotify:         {{PermissionID: "ui.notify", Resource: "ui"}},
		MethodUIDialog:         {{PermissionID: "ui.dialog", Resource: "ui"}},
		MethodUINavigate:       {{PermissionID: "ui.navigate", Resource: "ui"}},
		MethodClipboardWrite:   {{PermissionID: "clipboard.write", Resource: "clipboard"}},
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

func RouteScopeForMethod(method Method) ScopePolicy {
	switch method {
	case MethodStateGet, MethodStateCAS, MethodStateDelete, MethodStateList:
		return ScopePolicy{Namespaced: true}
	case MethodSecretGet:
		return ScopePolicy{Namespaced: true}
	case MethodResourceOpen, MethodResourceRead, MethodResourceClose, MethodResourceStat:
		return ScopePolicy{Namespaced: true}
	case MethodResourceWrite:
		return ScopePolicy{Namespaced: true}
	case MethodCharacterRead:
		return ScopePolicy{RequireRoles: []string{"character"}}
	case MethodConversationRead:
		return ScopePolicy{RequireRoles: []string{"conversation"}}
	case MethodMemoryQuery:
		return ScopePolicy{RequireRoles: []string{"character", "conversation"}, AllowNarrowing: true}
	case MethodEventEmit, MethodEventSubscribe, MethodEventUnsubscribe:
		return ScopePolicy{}
	case MethodScheduleCreate, MethodScheduleCancel, MethodScheduleList:
		return ScopePolicy{Namespaced: true}
	case MethodToolExecute:
		return ScopePolicy{RequireRoles: []string{"invocation", "tool"}}
	case MethodUINotify, MethodUIDialog, MethodUINavigate:
		return ScopePolicy{RequireRoles: []string{"session"}}
	case MethodClipboardWrite:
		return ScopePolicy{RequireRoles: []string{"session"}}
	default:
		return ScopePolicy{}
	}
}

func IsDataRouteMethod(method Method) bool {
	switch method {
	case MethodStateGet, MethodStateCAS, MethodStateDelete, MethodStateList,
		MethodResourceOpen, MethodResourceRead, MethodResourceWrite, MethodResourceClose, MethodResourceStat,
		MethodCharacterRead, MethodConversationRead, MethodMemoryQuery:
		return true
	default:
		return false
	}
}

func RouteRiskForMethod(method Method) RiskLevel {
	switch method {
	case MethodStateCAS, MethodStateDelete, MethodResourceWrite:
		return RiskMedium
	case MethodScheduleCreate, MethodScheduleCancel:
		return RiskMedium
	case MethodUIDialog:
		return RiskMedium
	case MethodToolExecute:
		return RiskHigh
	default:
		return RiskLow
	}
}

func RouteSideEffectForMethod(method Method) SideEffectLevel {
	switch method {
	case MethodStateGet, MethodStateList, MethodResourceOpen, MethodResourceRead, MethodResourceStat,
		MethodCharacterRead, MethodConversationRead, MethodMemoryQuery, MethodScheduleList:
		return SideEffectReadOnly
	case MethodStateCAS, MethodStateDelete, MethodResourceWrite, MethodScheduleCreate, MethodScheduleCancel:
		return SideEffectWrite
	case MethodEventEmit:
		return SideEffectWrite
	case MethodToolExecute:
		return SideEffectExternal
	default:
		return SideEffectNone
	}
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
		{ID: "schedule.manage", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}},
		{ID: "tool.invoke", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule, permission.ScopeTool}},
		{ID: "character.read", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeCharacter}},
		{ID: "conversation.read", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeConversation, permission.ScopeCharacter}},
		{ID: "memory.read", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeCharacter, permission.ScopeConversation}},
		{ID: "provider.invoke", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension}},
		{ID: "ui.notify", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeSession}},
		{ID: "ui.dialog", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeSession}},
		{ID: "ui.navigate", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeSession}},
		{ID: "clipboard.write", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeSession}},
	}
	for _, d := range defs {
		registry.Register(d)
	}
}
