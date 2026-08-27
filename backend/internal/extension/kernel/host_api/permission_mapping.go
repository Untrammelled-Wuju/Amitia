package host_api

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

type PermissionMappingEntry struct {
	PermissionID string
	Resource     string
}

func DefaultPermissionMapping() map[Method][]PermissionMappingEntry {
	return map[Method][]PermissionMappingEntry{
		MethodStateGet:                {{PermissionID: "storage.state.read", Resource: "state"}},
		MethodStateCAS:                {{PermissionID: "storage.state.write", Resource: "state"}},
		MethodStateDelete:             {{PermissionID: "storage.state.write", Resource: "state"}},
		MethodStateList:               {{PermissionID: "storage.state.read", Resource: "state"}},
		MethodSecretGet:               {{PermissionID: "secret.read", Resource: "secret"}},
		MethodResourceOpen:            {{PermissionID: "resource.read", Resource: "resource"}},
		MethodResourceRead:            {{PermissionID: "resource.read", Resource: "resource"}},
		MethodResourceWrite:           {{PermissionID: "resource.write", Resource: "resource"}},
		MethodResourceClose:           {{PermissionID: "resource.read", Resource: "resource"}},
		MethodResourceStat:            {{PermissionID: "resource.read", Resource: "resource"}},
		MethodEventEmit:               {{PermissionID: "event.emit", Resource: "event"}},
		MethodEventSubscribe:          {{PermissionID: "event.subscribe", Resource: "event"}},
		MethodEventUnsubscribe:        {{PermissionID: "event.subscribe", Resource: "event"}},
		MethodScheduleCreate:          {{PermissionID: "schedule.create", Resource: "schedule"}},
		MethodScheduleCancel:          {{PermissionID: "schedule.manage", Resource: "schedule"}},
		MethodScheduleList:            {{PermissionID: "schedule.create", Resource: "schedule"}},
		MethodToolExecute:             {{PermissionID: "tool.invoke", Resource: "tool"}},
		MethodCharacterRead:           {{PermissionID: "character.read", Resource: "character"}},
		MethodConversationRead:        {{PermissionID: "conversation.read", Resource: "conversation"}},
		MethodMemoryQuery:             {{PermissionID: "memory.read", Resource: "memory"}},
		MethodProviderInvoke:          {{PermissionID: "provider.invoke", Resource: "provider"}},
		MethodUINotify:                {{PermissionID: "ui.notify", Resource: "ui"}},
		MethodUIDialog:                {{PermissionID: "ui.dialog", Resource: "ui"}},
		MethodUINavigate:              {{PermissionID: "ui.navigate", Resource: "ui"}},
		MethodClipboardWrite:          {{PermissionID: "clipboard.write", Resource: "clipboard"}},
		MethodClipboardRead:           {{PermissionID: "clipboard.read", Resource: "clipboard"}},
		MethodRuntimeHealth:           {{PermissionID: "runtime.health.read", Resource: "runtime"}},
		MethodNetworkRequest:          {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkTCPOpen:          {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkTCPRead:          {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkTCPWrite:         {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkTCPClose:         {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkUDPOpen:          {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkUDPReceive:       {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkUDPSend:          {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkUDPClose:         {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkWebSocketOpen:    {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkWebSocketReceive: {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkWebSocketSend:    {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkWebSocketClose:   {{PermissionID: "service.network.request", Resource: "network"}},
		MethodMigrationSQLExecute:     {},
		MethodMigrationSQLQuery:       {},
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
	case MethodClipboardWrite, MethodClipboardRead:
		return ScopePolicy{RequireRoles: []string{"session"}}
	case MethodRuntimeHealth:
		return ScopePolicy{Namespaced: true}
	default:
		return ScopePolicy{}
	}
}

func IsDataRouteMethod(method Method) bool {
	switch method {
	case MethodStateGet, MethodStateCAS, MethodStateDelete, MethodStateList,
		MethodResourceOpen, MethodResourceRead, MethodResourceWrite, MethodResourceClose, MethodResourceStat,
		MethodCharacterRead, MethodConversationRead, MethodMemoryQuery, MethodRuntimeHealth:
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
	case MethodNetworkRequest, MethodNetworkTCPOpen, MethodNetworkTCPRead, MethodNetworkTCPWrite, MethodNetworkTCPClose,
		MethodNetworkUDPOpen, MethodNetworkUDPReceive, MethodNetworkUDPSend, MethodNetworkUDPClose,
		MethodNetworkWebSocketOpen, MethodNetworkWebSocketReceive, MethodNetworkWebSocketSend, MethodNetworkWebSocketClose:
		return RiskMedium
	case MethodMigrationSQLExecute:
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
	case MethodToolExecute, MethodNetworkRequest, MethodNetworkTCPOpen, MethodNetworkTCPRead, MethodNetworkTCPWrite, MethodNetworkTCPClose,
		MethodNetworkUDPOpen, MethodNetworkUDPReceive, MethodNetworkUDPSend, MethodNetworkUDPClose,
		MethodNetworkWebSocketOpen, MethodNetworkWebSocketReceive, MethodNetworkWebSocketSend, MethodNetworkWebSocketClose:
		return SideEffectExternal
	case MethodMigrationSQLExecute:
		return SideEffectWrite
	case MethodMigrationSQLQuery:
		return SideEffectReadOnly
	default:
		return SideEffectNone
	}
}

func RegisterPermissionDefinitions(registry *permission.PermissionDefinitionRegistry) {
	if registry == nil {
		return
	}
	defs := []permission.PermissionDefinition{
		{ID: "storage.state.read", Name: "Read Extension State", Description: "Read namespaced extension state", Category: permission.CategoryHostData, RiskLevel: capability.RiskLow, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: permission.ChildInherit, DefaultApproval: permission.ApprovalAuto},
		{ID: "storage.state.write", Name: "Write Extension State", Description: "Create, update, or delete namespaced extension state", Category: permission.CategoryHostData, RiskLevel: capability.RiskMedium, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: permission.ChildReevaluate, DefaultApproval: permission.ApprovalManual},
		{ID: "secret.read", Name: "Read Scoped Secret", Description: "Read a secret exposed to the current extension or module scope", Category: permission.CategoryExtension, RiskLevel: capability.RiskHigh, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: permission.ChildDeny, TrustedOnly: true, DefaultApproval: permission.ApprovalFullControl},
		{ID: "resource.read", Name: "Read Host Resource", Description: "Read a host-managed resource through a scoped resource handle", Category: permission.CategoryFilesystem, RiskLevel: capability.RiskMedium, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule, permission.ScopeResource}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: permission.ChildInherit, DefaultApproval: permission.ApprovalManual},
		{ID: "resource.write", Name: "Write Host Resource", Description: "Modify a host-managed resource through a scoped resource handle", Category: permission.CategoryFilesystem, RiskLevel: capability.RiskHigh, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule, permission.ScopeResource}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: permission.ChildReevaluate, DefaultApproval: permission.ApprovalManual},
		{ID: "event.emit", Name: "Emit Host Event", Description: "Publish an event through the host event bus", Category: permission.CategoryExtension, RiskLevel: capability.RiskMedium, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: permission.ChildReevaluate, DefaultApproval: permission.ApprovalManual},
		{ID: "event.subscribe", Name: "Subscribe Host Event", Description: "Subscribe to events visible to the current extension or module scope", Category: permission.CategoryExtension, RiskLevel: capability.RiskLow, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: permission.ChildInherit, DefaultApproval: permission.ApprovalAuto},
		{ID: "schedule.create", Name: "Create Schedule", Description: "Create a namespaced scheduled host task", Category: permission.CategoryWorkflow, RiskLevel: capability.RiskMedium, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: permission.ChildReevaluate, DefaultApproval: permission.ApprovalManual},
		{ID: "schedule.manage", Name: "Manage Schedule", Description: "Cancel or modify a namespaced scheduled host task", Category: permission.CategoryWorkflow, RiskLevel: capability.RiskMedium, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: permission.ChildReevaluate, DefaultApproval: permission.ApprovalManual},
		{ID: "tool.invoke", Name: "Invoke Host Tool", Description: "Invoke a host tool from an extension runtime", Category: permission.CategoryExtension, RiskLevel: capability.RiskHigh, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule, permission.ScopeTool}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: permission.ChildReevaluate, DefaultApproval: permission.ApprovalManual},
		{ID: "character.read", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeCharacter}},
		{ID: "conversation.read", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeConversation, permission.ScopeCharacter}},
		{ID: "memory.read", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeCharacter, permission.ScopeConversation}},
		{ID: "provider.invoke", Name: "Invoke Provider", Description: "Invoke an AI provider through the host provider boundary", Category: permission.CategoryProvider, RiskLevel: capability.RiskMedium, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: permission.ChildInherit, DefaultApproval: permission.ApprovalManual},
		{ID: "ui.notify", Name: "Show Notification", Description: "Display a host notification in the active user session", Category: permission.CategoryDesktop, RiskLevel: capability.RiskLow, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeSession}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: permission.ChildInherit, DefaultApproval: permission.ApprovalAuto},
		{ID: "ui.dialog", Name: "Show Dialog", Description: "Display an interactive host dialog in the active user session", Category: permission.CategoryDesktop, RiskLevel: capability.RiskMedium, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeSession}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: permission.ChildReevaluate, DefaultApproval: permission.ApprovalManual},
		{ID: "ui.navigate", Name: "Navigate UI", Description: "Navigate the host UI in the active user session", Category: permission.CategoryDesktop, RiskLevel: capability.RiskLow, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeSession}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: permission.ChildInherit, DefaultApproval: permission.ApprovalAuto},
		{ID: "clipboard.write", Name: "Write Clipboard", Description: "Write data to the user clipboard", Category: permission.CategoryDesktop, RiskLevel: capability.RiskMedium, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeSession}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: permission.ChildReevaluate, DefaultApproval: permission.ApprovalManual},
		{ID: "clipboard.read", Name: "Read Clipboard", Description: "Read data from the user clipboard", Category: permission.CategoryDesktop, RiskLevel: capability.RiskHigh, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeSession}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: permission.ChildDeny, DefaultApproval: permission.ApprovalManual},
		{ID: "runtime.health.read", Name: "Read Runtime Health", Description: "Read health state for the current runtime scope", Category: permission.CategoryService, RiskLevel: capability.RiskLow, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: permission.ChildInherit, DefaultApproval: permission.ApprovalAuto},
		{ID: "service.network.request", AllowedScopes: []permission.ScopeType{permission.ScopeExtension, permission.ScopeModule, permission.ScopeGlobal}},
		{ID: "migration.sql.execute", Name: "Execute Migration SQL", Description: "Execute migration SQL through the host migration boundary", Category: permission.CategoryExtension, RiskLevel: capability.RiskHigh, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: permission.ChildDeny, TrustedOnly: true, DefaultApproval: permission.ApprovalManual},
		{ID: "migration.sql.query", Name: "Query Migration SQL", Description: "Query migration state through the host migration boundary", Category: permission.CategoryExtension, RiskLevel: capability.RiskMedium, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: permission.ChildDeny, TrustedOnly: true, DefaultApproval: permission.ApprovalManual},
	}
	for _, d := range defs {
		registry.RegisterPreservingMetadata(d)
	}
}
