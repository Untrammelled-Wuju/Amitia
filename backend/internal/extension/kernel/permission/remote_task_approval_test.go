package permission

import (
	"context"
	"testing"
)

func TestRemoteTaskApprovalCannotOverrideSourceHardDenials(t *testing.T) {
	for _, scenario := range []string{"allowed", "unknown", "remote-deny", "background", "scope", "installation", "system"} {
		t.Run(scenario, func(t *testing.T) {
			registry := NewPermissionDefinitionRegistry()
			definition := PermissionDefinition{ID: "test.source.read", Category: CategoryFilesystem, AllowedScopes: []ScopeType{ScopeExtension}, BackgroundAllowed: true, RequiresPerUse: true, RemoteExecution: RemoteExecutionRequireApproval}
			if scenario == "scope" {
				definition.AllowedScopes = []ScopeType{ScopeConversation}
			}
			if scenario == "remote-deny" {
				definition.RemoteExecution = RemoteExecutionDeny
			}
			if scenario == "background" {
				definition.BackgroundAllowed = false
			}
			if scenario != "unknown" {
				registry.Register(definition)
			}
			broker := NewDefaultPermissionBroker(registry, NewMemoryPermissionStorage())
			t.Cleanup(func() { _ = broker.Close() })
			if scenario == "installation" {
				broker.InstallationPolicy = func(context.Context, PermissionSubject, PermissionRequirement, PermissionDefinition) (PermissionDecision, bool) {
					return DecisionDeny, true
				}
			}
			if scenario == "system" {
				broker.SystemPolicy = func(context.Context, PermissionSubject, string, PermissionScope) (PermissionDecision, bool) {
					return DecisionDeny, true
				}
			}
			request := PermissionEvaluationRequest{Subject: PermissionSubject{Type: SubjectModule, ID: "module", ExtensionID: "extension", ModuleID: "module"}, Requirements: []PermissionRequirement{{PermissionID: definition.ID, Scope: ScopeForExtension("extension")}}, IsBackground: true, ApprovalMode: string(ApprovalFullControl), ExecutionContext: PermissionExecutionContext{Placement: ExecutionPlacementDevice, SpaceID: "core", DeviceID: "device", RuntimeID: "runtime"}}
			if broker.CanApproveRemoteTask(t.Context(), request) != (scenario == "allowed") {
				t.Fatal("本机单次审批没有保留资源禁止策略")
			}
		})
	}
}
