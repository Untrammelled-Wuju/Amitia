package host_api

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

type PermissionMappingEntry struct {
	PermissionID string
	Resource     string
	Scope        permission.ScopeType
}

func DefaultPermissionMapping() map[Method][]PermissionMappingEntry {
	return map[Method][]PermissionMappingEntry{
		MethodStateGet:                     {{PermissionID: "storage.state.read", Resource: "state"}},
		MethodStateCAS:                     {{PermissionID: "storage.state.write", Resource: "state"}},
		MethodStateDelete:                  {{PermissionID: "storage.state.write", Resource: "state"}},
		MethodStateList:                    {{PermissionID: "storage.state.read", Resource: "state"}},
		MethodSecretGet:                    {{PermissionID: "secret.read", Resource: "secret"}},
		MethodResourceOpen:                 {{PermissionID: "resource.read", Resource: "resource"}},
		MethodResourceRead:                 {{PermissionID: "resource.read", Resource: "resource"}},
		MethodResourceWrite:                {{PermissionID: "resource.write", Resource: "resource"}},
		MethodResourceClose:                {{PermissionID: "resource.read", Resource: "resource"}},
		MethodResourceStat:                 {{PermissionID: "resource.read", Resource: "resource"}},
		MethodResourceLink:                 {{PermissionID: "resource.read", Resource: "resource"}},
		MethodResourceDelete:               {{PermissionID: "resource.write", Resource: "resource"}},
		MethodEventEmit:                    {{PermissionID: "event.emit", Resource: "event"}},
		MethodEventSubscribe:               {{PermissionID: "event.subscribe", Resource: "event"}},
		MethodEventUnsubscribe:             {{PermissionID: "event.subscribe", Resource: "event"}},
		MethodScheduleCreate:               {{PermissionID: "schedule.create", Resource: "schedule"}},
		MethodScheduleCancel:               {{PermissionID: "schedule.manage", Resource: "schedule"}},
		MethodScheduleList:                 {{PermissionID: "schedule.create", Resource: "schedule"}},
		MethodToolExecute:                  {{PermissionID: "tool.invoke", Resource: "tool"}},
		MethodPluginPreview:                {{PermissionID: "extensions.install", Resource: "extension", Scope: permission.ScopeGlobal}},
		MethodPluginInstall:                {{PermissionID: "extensions.install", Resource: "extension", Scope: permission.ScopeGlobal}},
		MethodPluginUninstall:              {{PermissionID: "extensions.uninstall", Resource: "extension", Scope: permission.ScopeGlobal}},
		MethodPluginList:                   {{PermissionID: "extensions.read", Resource: "extension", Scope: permission.ScopeGlobal}},
		MethodPluginContributionRegister:   {{PermissionID: "plugin.compose", Resource: "plugin_contribution"}},
		MethodPluginContributionUnregister: {{PermissionID: "plugin.compose", Resource: "plugin_contribution"}},
		MethodPluginServiceProvide:         {{PermissionID: "plugin.compose", Resource: "plugin_service"}},
		MethodPluginServiceUnprovide:       {{PermissionID: "plugin.compose", Resource: "plugin_service"}},
		MethodPluginServiceRequire:         {{PermissionID: "plugin.compose", Resource: "plugin_service"}},
		MethodPluginServiceRelease:         {{PermissionID: "plugin.compose", Resource: "plugin_service"}},
		MethodPluginAdapterInvoke:          {{PermissionID: "plugin.compose", Resource: "plugin_adapter"}},
		MethodPluginAdapterCancel:          {{PermissionID: "plugin.compose", Resource: "plugin_adapter"}},
		MethodCharacterRead:                {{PermissionID: "character.read", Resource: "character"}},
		MethodCharacterList:                {{PermissionID: "character.read", Resource: "character"}},
		MethodConversationRead:             {{PermissionID: "conversation.read", Resource: "conversation"}},
		MethodConversationMessageSend:      {{PermissionID: "message.send", Resource: "conversation"}},
		MethodConversationMessageAppend:    {{PermissionID: "message.append", Resource: "conversation"}},
		MethodMemoryQuery:                  {{PermissionID: "memory.read", Resource: "memory"}},
		MethodProviderInvoke:               {{PermissionID: "provider.invoke", Resource: "provider"}},
		MethodExtensionDataRead:            {{PermissionID: "extension.data.read", Resource: "extension_data"}},
		MethodVectorUpsert:                 {{PermissionID: "vector.write", Resource: "vector"}},
		MethodVectorSearch:                 {{PermissionID: "vector.read", Resource: "vector"}},
		MethodVectorDelete:                 {{PermissionID: "vector.write", Resource: "vector"}},
		MethodUINotify:                     {{PermissionID: "ui.notify", Resource: "ui"}},
		MethodUIDialog:                     {{PermissionID: "ui.dialog", Resource: "ui"}},
		MethodUINavigate:                   {{PermissionID: "ui.navigate", Resource: "ui"}},
		MethodClipboardWrite:               {{PermissionID: "clipboard.write", Resource: "clipboard"}},
		MethodClipboardRead:                {{PermissionID: "clipboard.read", Resource: "clipboard"}},
		MethodRuntimeHealth:                {{PermissionID: "runtime.health.read", Resource: "runtime"}},
		MethodRuntimeSnapshot:              {{PermissionID: "runtime.health.read", Resource: "runtime"}},
		MethodRuntimeMaintenanceStatus:     {{PermissionID: "runtime.maintenance.read", Resource: "runtime"}},
		MethodRuntimeMaintenanceOverview:   {{PermissionID: "runtime.maintenance.read", Resource: "runtime"}},
		MethodRuntimeMaintenanceConfig:     {{PermissionID: "runtime.maintenance.read", Resource: "runtime"}},
		MethodRuntimeMaintenanceUpdate:     {{PermissionID: "runtime.maintenance.write", Resource: "runtime"}},
		MethodRuntimeCleanupTemp:           {{PermissionID: "runtime.maintenance.write", Resource: "runtime"}},
		MethodRuntimeRotateLogs:            {{PermissionID: "runtime.maintenance.write", Resource: "runtime"}},
		MethodRuntimeCheckDBIntegrity:      {{PermissionID: "runtime.maintenance.write", Resource: "runtime"}},
		MethodWorkspaceRead:                {{PermissionID: "workspace.read", Resource: "workspace"}},
		MethodWorkspaceWrite:               {{PermissionID: "workspace.write", Resource: "workspace"}},
		MethodImportExecute:                {{PermissionID: "conversation.import", Resource: "import"}},
		MethodStorageCleanup:               {{PermissionID: "storage.cleanup", Resource: "storage"}},
		MethodStorageBackup:                {{PermissionID: "storage.backup", Resource: "storage"}},
		MethodStorageRestore:               {{PermissionID: "storage.restore", Resource: "storage"}},
		MethodConversationLogsRead:         {{PermissionID: "conversation.read", Resource: "conversation"}},
		MethodConversationLogsManage:       {{PermissionID: "conversation.manage", Resource: "conversation"}},
		MethodConversationLogsExport:       {{PermissionID: "conversation.export", Resource: "conversation"}},
		MethodNetworkRequest:               {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkTCPOpen:               {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkTCPRead:               {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkTCPWrite:              {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkTCPClose:              {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkUDPOpen:               {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkUDPReceive:            {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkUDPSend:               {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkUDPClose:              {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkWebSocketOpen:         {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkWebSocketReceive:      {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkWebSocketSend:         {{PermissionID: "service.network.request", Resource: "network"}},
		MethodNetworkWebSocketClose:        {{PermissionID: "service.network.request", Resource: "network"}},
		MethodMigrationSQLExecute:          {},
		MethodMigrationSQLQuery:            {},
	}
}

func RoutePermissionForMethod(method Method) []PermissionRequirement {
	if permission, _, ok := scheduledMethod(method); ok {
		return []PermissionRequirement{{Name: permission, Resource: "schedule"}}
	}
	entries, ok := DefaultPermissionMapping()[method]
	if !ok {
		return nil
	}
	out := make([]PermissionRequirement, 0, len(entries))
	for _, e := range entries {
		out = append(out, PermissionRequirement{Name: e.PermissionID, Resource: e.Resource, Scope: e.Scope})
	}
	return out
}

func RouteScopeForMethod(method Method) ScopePolicy {
	if _, _, ok := scheduledMethod(method); ok {
		return ScopePolicy{Namespaced: true}
	}
	switch method {
	case MethodStateGet, MethodStateCAS, MethodStateDelete, MethodStateList:
		return ScopePolicy{Namespaced: true}
	case MethodSecretGet:
		return ScopePolicy{Namespaced: true}
	case MethodResourceOpen, MethodResourceRead, MethodResourceClose, MethodResourceStat, MethodResourceLink, MethodResourceDelete:
		return ScopePolicy{Namespaced: true}
	case MethodResourceWrite:
		return ScopePolicy{Namespaced: true}
	case MethodCharacterRead:
		return ScopePolicy{RequireRoles: []string{"character"}}
	case MethodCharacterList:
		return ScopePolicy{Namespaced: true}
	case MethodConversationRead:
		return ScopePolicy{RequireRoles: []string{"conversation"}}
	case MethodConversationMessageSend, MethodConversationMessageAppend:
		return ScopePolicy{Namespaced: true}
	case MethodMemoryQuery:
		return ScopePolicy{RequireRoles: []string{"character", "conversation"}, AllowNarrowing: true}
	case MethodEventEmit, MethodEventSubscribe, MethodEventUnsubscribe:
		return ScopePolicy{}
	case MethodScheduleCreate, MethodScheduleCancel, MethodScheduleList:
		return ScopePolicy{Namespaced: true}
	case MethodToolExecute:
		return ScopePolicy{RequireRoles: []string{"invocation"}}
	case MethodExtensionDataRead:
		return ScopePolicy{Namespaced: true}
	case MethodVectorUpsert, MethodVectorSearch, MethodVectorDelete:
		return ScopePolicy{Namespaced: true}
	case MethodUINotify, MethodUIDialog, MethodUINavigate:
		return ScopePolicy{RequireRoles: []string{"session"}}
	case MethodClipboardWrite, MethodClipboardRead:
		return ScopePolicy{RequireRoles: []string{"session"}}
	case MethodRuntimeHealth:
		return ScopePolicy{Namespaced: true}
	case MethodWorkspaceRead, MethodWorkspaceWrite:
		return ScopePolicy{Namespaced: true}
	case MethodImportExecute:
		return ScopePolicy{Namespaced: true}
	case MethodStorageCleanup, MethodStorageBackup, MethodStorageRestore:
		return ScopePolicy{Namespaced: true}
	case MethodConversationLogsRead, MethodConversationLogsManage, MethodConversationLogsExport:
		return ScopePolicy{Namespaced: true}
	case MethodPluginContributionRegister, MethodPluginContributionUnregister,
		MethodPluginServiceProvide, MethodPluginServiceUnprovide, MethodPluginServiceRequire, MethodPluginServiceRelease,
		MethodPluginAdapterInvoke, MethodPluginAdapterCancel:
		return ScopePolicy{Namespaced: true}
	default:
		return ScopePolicy{}
	}
}

func IsDataRouteMethod(method Method) bool {
	if _, _, ok := scheduledMethod(method); ok {
		return true
	}
	switch method {
	case MethodStateGet, MethodStateCAS, MethodStateDelete, MethodStateList,
		MethodResourceOpen, MethodResourceRead, MethodResourceWrite, MethodResourceClose, MethodResourceStat, MethodResourceLink, MethodResourceDelete,
		MethodCharacterRead, MethodCharacterList, MethodConversationRead, MethodMemoryQuery, MethodRuntimeHealth, MethodWorkspaceRead, MethodWorkspaceWrite, MethodImportExecute, MethodStorageCleanup, MethodStorageBackup, MethodStorageRestore, MethodConversationLogsRead, MethodConversationLogsManage, MethodConversationLogsExport, MethodExtensionDataRead,
		MethodVectorUpsert, MethodVectorSearch, MethodVectorDelete:
		return true
	default:
		return false
	}
}

func RouteRiskForMethod(method Method) RiskLevel {
	if _, action, ok := scheduledMethod(method); ok {
		if action == "read" {
			return RiskLow
		}
		return RiskMedium
	}
	switch method {
	case MethodStateCAS, MethodStateDelete, MethodResourceWrite:
		return RiskMedium
	case MethodRuntimeMaintenanceUpdate, MethodRuntimeCleanupTemp, MethodRuntimeRotateLogs, MethodRuntimeCheckDBIntegrity:
		return RiskHigh
	case MethodWorkspaceWrite:
		return RiskHigh
	case MethodImportExecute:
		return RiskHigh
	case MethodStorageCleanup, MethodStorageBackup, MethodStorageRestore:
		return RiskHigh
	case MethodConversationLogsManage:
		return RiskHigh
	case MethodConversationLogsExport:
		return RiskMedium
	case MethodScheduleCreate, MethodScheduleCancel:
		return RiskMedium
	case MethodUIDialog:
		return RiskMedium
	case MethodToolExecute, MethodPluginPreview, MethodPluginInstall, MethodPluginUninstall,
		MethodPluginContributionRegister, MethodPluginContributionUnregister,
		MethodPluginServiceProvide, MethodPluginServiceUnprovide, MethodPluginServiceRequire, MethodPluginServiceRelease,
		MethodPluginAdapterInvoke, MethodPluginAdapterCancel,
		MethodConversationMessageSend, MethodConversationMessageAppend:
		return RiskHigh
	case MethodExtensionDataRead:
		return RiskMedium
	case MethodVectorUpsert, MethodVectorSearch, MethodVectorDelete:
		return RiskMedium
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
	if _, action, ok := scheduledMethod(method); ok {
		if action == "read" {
			return SideEffectReadOnly
		}
		if action == "execute" {
			return SideEffectExternal
		}
		return SideEffectWrite
	}
	switch method {
	case MethodStateGet, MethodStateList, MethodResourceOpen, MethodResourceRead, MethodResourceStat,
		MethodResourceLink, MethodCharacterRead, MethodCharacterList, MethodConversationRead, MethodMemoryQuery, MethodScheduleList, MethodExtensionDataRead, MethodPluginList, MethodVectorSearch:
		return SideEffectReadOnly
	case MethodStateCAS, MethodStateDelete, MethodResourceWrite, MethodResourceDelete, MethodScheduleCreate, MethodScheduleCancel, MethodVectorUpsert, MethodVectorDelete:
		return SideEffectWrite
	case MethodRuntimeMaintenanceUpdate, MethodRuntimeCleanupTemp, MethodRuntimeRotateLogs, MethodRuntimeCheckDBIntegrity:
		return SideEffectWrite
	case MethodWorkspaceRead:
		return SideEffectReadOnly
	case MethodWorkspaceWrite:
		return SideEffectWrite
	case MethodImportExecute:
		return SideEffectWrite
	case MethodStorageCleanup, MethodStorageBackup, MethodStorageRestore:
		return SideEffectWrite
	case MethodConversationLogsRead, MethodConversationLogsExport:
		return SideEffectReadOnly
	case MethodConversationLogsManage:
		return SideEffectWrite
	case MethodPluginContributionRegister, MethodPluginContributionUnregister,
		MethodPluginServiceProvide, MethodPluginServiceUnprovide, MethodPluginServiceRequire, MethodPluginServiceRelease,
		MethodPluginAdapterCancel:
		return SideEffectWrite
	case MethodPluginAdapterInvoke:
		return SideEffectExternal
	case MethodEventEmit:
		return SideEffectWrite
	case MethodToolExecute, MethodPluginPreview, MethodPluginInstall, MethodPluginUninstall, MethodConversationMessageSend, MethodConversationMessageAppend, MethodNetworkRequest, MethodNetworkTCPOpen, MethodNetworkTCPRead, MethodNetworkTCPWrite, MethodNetworkTCPClose,
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
		{ID: "extensions.install", Name: "Install Extension", Description: "Install a verified plugin package through the host package manager", Category: permission.CategoryExtension, RiskLevel: capability.RiskHigh, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: permission.ChildDeny, DefaultApproval: permission.ApprovalManual},
		{ID: "extensions.uninstall", Name: "Uninstall Extension", Description: "Uninstall a plugin package through the host package manager", Category: permission.CategoryExtension, RiskLevel: capability.RiskHigh, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: permission.ChildDeny, DefaultApproval: permission.ApprovalManual},
		{ID: "extensions.read", Name: "Read Extension Inventory", Description: "Read installed extension versions and enablement state", Category: permission.CategoryExtension, RiskLevel: capability.RiskLow, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: permission.ChildInherit, DefaultApproval: permission.ApprovalAuto},
		{ID: "character.read", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeCharacter}},
		{ID: "conversation.read", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeConversation, permission.ScopeCharacter}},
		{ID: "memory.read", AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeCharacter, permission.ScopeConversation}},
		{ID: "provider.invoke", Name: "Invoke Provider", Description: "Invoke an AI provider through the host provider boundary", Category: permission.CategoryProvider, RiskLevel: capability.RiskMedium, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: permission.ChildInherit, DefaultApproval: permission.ApprovalManual},
		{ID: "message.append", Name: "Append Conversation Message", Description: "Append explicit text or media messages to a conversation through the host chat store", Category: permission.CategoryHostData, RiskLevel: capability.RiskHigh, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeCharacter, permission.ScopeConversation}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: true, ChildInvocation: permission.ChildReevaluate, DefaultApproval: permission.ApprovalManual},
		{ID: "extension.data.read", Name: "Read Extension Data Source", Description: "Read a host-registered extension data source for migration or synchronization", Category: permission.CategoryHostData, RiskLevel: capability.RiskMedium, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: permission.ChildDeny, TrustedOnly: true, DefaultApproval: permission.ApprovalAuto},
		{ID: "vector.read", Name: "Search Extension Vectors", Description: "Search vectors in the current extension namespace", Category: permission.CategoryHostData, RiskLevel: capability.RiskLow, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: permission.ChildInherit, DefaultApproval: permission.ApprovalAuto},
		{ID: "vector.write", Name: "Write Extension Vectors", Description: "Create or delete vectors in the current extension namespace", Category: permission.CategoryHostData, RiskLevel: capability.RiskMedium, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: permission.ChildReevaluate, DefaultApproval: permission.ApprovalManual},
		{ID: "ui.notify", Name: "Show Notification", Description: "Display a host notification in the active user session", Category: permission.CategoryDesktop, RiskLevel: capability.RiskLow, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeSession}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: permission.ChildInherit, DefaultApproval: permission.ApprovalAuto},
		{ID: "ui.dialog", Name: "Show Dialog", Description: "Display an interactive host dialog in the active user session", Category: permission.CategoryDesktop, RiskLevel: capability.RiskMedium, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeSession}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: permission.ChildReevaluate, DefaultApproval: permission.ApprovalManual},
		{ID: "ui.navigate", Name: "Navigate UI", Description: "Navigate the host UI in the active user session", Category: permission.CategoryDesktop, RiskLevel: capability.RiskLow, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeSession}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: permission.ChildInherit, DefaultApproval: permission.ApprovalAuto},
		{ID: "clipboard.write", Name: "Write Clipboard", Description: "Write data to the user clipboard", Category: permission.CategoryDesktop, RiskLevel: capability.RiskMedium, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeSession}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: permission.ChildReevaluate, DefaultApproval: permission.ApprovalManual},
		{ID: "clipboard.read", Name: "Read Clipboard", Description: "Read data from the user clipboard", Category: permission.CategoryDesktop, RiskLevel: capability.RiskHigh, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeSession}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: permission.ChildDeny, DefaultApproval: permission.ApprovalManual},
		{ID: "runtime.health.read", Name: "Read Runtime Health", Description: "Read health state for the current runtime scope", Category: permission.CategoryService, RiskLevel: capability.RiskLow, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: permission.ChildInherit, DefaultApproval: permission.ApprovalAuto},
		{ID: "runtime.maintenance.read", Name: "Read Runtime Maintenance", Description: "Read task executor status and maintenance configuration", Category: permission.CategoryService, RiskLevel: capability.RiskLow, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: permission.ChildInherit, DefaultApproval: permission.ApprovalAuto},
		{ID: "runtime.maintenance.write", Name: "Manage Runtime Maintenance", Description: "Update task executor limits and run host maintenance actions", Category: permission.CategoryService, RiskLevel: capability.RiskHigh, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: permission.ChildReevaluate, TrustedOnly: true, DefaultApproval: permission.ApprovalFullControl},
		{ID: "service.network.request", AllowedScopes: []permission.ScopeType{permission.ScopeExtension, permission.ScopeModule, permission.ScopeGlobal}},
		{ID: "migration.sql.execute", Name: "Execute Migration SQL", Description: "Execute migration SQL through the host migration boundary", Category: permission.CategoryExtension, RiskLevel: capability.RiskHigh, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: permission.ChildDeny, TrustedOnly: true, DefaultApproval: permission.ApprovalManual},
		{ID: "migration.sql.query", Name: "Query Migration SQL", Description: "Query migration state through the host migration boundary", Category: permission.CategoryExtension, RiskLevel: capability.RiskMedium, AllowedScopes: []permission.ScopeType{permission.ScopeGlobal, permission.ScopeExtension, permission.ScopeModule}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: permission.ChildDeny, TrustedOnly: true, DefaultApproval: permission.ApprovalManual},
	}
	for _, d := range defs {
		registry.RegisterPreservingMetadata(d)
	}
}
