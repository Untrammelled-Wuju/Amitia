package permission

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestFullControlAllowsDeclaredAndroidCapabilities(t *testing.T) {
	ctx := context.Background()
	broker := NewDefaultPermissionBroker(NewPermissionDefinitionRegistry(), NewMemoryPermissionStorage())
	subject := PermissionSubject{Type: SubjectSystem, ID: "core"}
	permissionIDs := []string{
		"android.ui_tree.read",
		"android.interaction.read_visual",
		"android.interaction.click",
		"android.interaction.input",
		"android.interaction.gesture",
		"android.interaction.global",
		"android.app.launch",
		"runtime.linux.shell.execute",
		"runtime.linux.terminal.control",
		"runtime.linux.terminal.read",
		"runtime.linux.file.read",
		"runtime.linux.file.write",
		"runtime.linux.file.control",
		"runtime.linux.archive.read",
		"runtime.linux.archive.write",
		"runtime.linux.network.inspect",
		"runtime.linux.network.public",
		"runtime.linux.network.download",
		"runtime.linux.ssh.read",
		"runtime.linux.ssh.exec",
		"runtime.linux.chroot.read",
		"runtime.linux.chroot.exec",
	}
	requirements := make([]PermissionRequirement, 0, len(permissionIDs))
	for _, permissionID := range permissionIDs {
		requirements = append(requirements, PermissionRequirement{PermissionID: permissionID})
	}
	request := PermissionEvaluationRequest{
		Subject:      subject,
		Requirements: requirements,
		InvocationID: "inv-full-control",
		ApprovalMode: string(ApprovalFullControl),
		ExecutionContext: PermissionExecutionContext{
			Placement: ExecutionPlacementDevice,
			SpaceID:   runtimeidentity.ParseSpaceID("space-1"),
			DeviceID:  runtimeidentity.ParseDeviceID("device-1"),
			RuntimeID: runtimeidentity.ParseRuntimeID("runtime-1"),
			Source:    "model",
		},
	}

	result := broker.Evaluate(ctx, request)
	if result.Decision != DecisionAllow {
		t.Fatalf("decision = %s, want %s, reasons = %#v", result.Decision, DecisionAllow, result.Reasons)
	}
	allowed := 0
	for _, reason := range result.Reasons {
		if reason.Code == "full_control_allowed" {
			allowed++
		}
	}
	if allowed != len(permissionIDs) {
		t.Fatalf("full_control_allowed reasons = %d, want %d", allowed, len(permissionIDs))
	}
}

func TestFullControlDoesNotBypassHardDeny(t *testing.T) {
	ctx := context.Background()
	broker := NewDefaultPermissionBroker(NewPermissionDefinitionRegistry(), NewMemoryPermissionStorage())
	request := PermissionEvaluationRequest{
		Subject:      PermissionSubject{Type: SubjectSystem, ID: "core"},
		Requirements: []PermissionRequirement{{PermissionID: "android.root.execute"}},
		InvocationID: "inv-full-control-deny",
		ApprovalMode: string(ApprovalFullControl),
		ExecutionContext: PermissionExecutionContext{
			Placement: ExecutionPlacementDevice,
			SpaceID:   runtimeidentity.ParseSpaceID("space-1"),
			DeviceID:  runtimeidentity.ParseDeviceID("device-1"),
			RuntimeID: runtimeidentity.ParseRuntimeID("runtime-1"),
			Source:    "model",
		},
	}

	result := broker.Evaluate(ctx, request)
	if result.Decision == DecisionAllow {
		t.Fatalf("decision = %s, want non-allow, reasons = %#v", result.Decision, result.Reasons)
	}
	foundDeny := false
	for _, reason := range result.Reasons {
		if reason.Code == "remote_execution_denied" {
			foundDeny = true
			break
		}
	}
	if !foundDeny {
		t.Fatalf("expected remote_execution_denied, reasons = %#v", result.Reasons)
	}
}

func TestApprovalModesApplyToEveryCapability(t *testing.T) {
	ctx := context.Background()
	broker := NewDefaultPermissionBroker(NewPermissionDefinitionRegistry(), NewMemoryPermissionStorage())
	requirements := []PermissionRequirement{
		{PermissionID: "message.send"},
		{PermissionID: "network.request"},
		{PermissionID: "workspace.write"},
	}
	executionContext := PermissionExecutionContext{
		Placement: ExecutionPlacementLocal,
		SpaceID:   runtimeidentity.ParseSpaceID("space-1"),
		Source:    "model",
	}
	request := PermissionEvaluationRequest{
		Subject:          PermissionSubject{Type: SubjectSystem, ID: "core"},
		Requirements:     requirements,
		InvocationID:     "inv-all-capabilities",
		ScopeSnapshotID:  "scope-all-capabilities",
		ExecutionContext: executionContext,
	}

	request.ApprovalMode = string(ApprovalManual)
	gated := broker.Evaluate(ctx, request)
	if gated.Decision != DecisionRequireApproval {
		t.Fatalf("manual decision = %s, want %s, reasons = %#v", gated.Decision, DecisionRequireApproval, gated.Reasons)
	}

	request.ApprovalMode = string(ApprovalFullControl)
	allowed := broker.Evaluate(ctx, request)
	if allowed.Decision != DecisionAllow {
		t.Fatalf("full control decision = %s, want %s, reasons = %#v", allowed.Decision, DecisionAllow, allowed.Reasons)
	}
}
