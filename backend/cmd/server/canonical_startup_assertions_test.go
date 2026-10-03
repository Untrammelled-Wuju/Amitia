package main

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/hook"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

type mockPermissionBroker struct{}

func (m *mockPermissionBroker) Evaluate(_ context.Context, _ permission.PermissionEvaluationRequest) permission.PermissionEvaluationResult {
	return permission.PermissionEvaluationResult{}
}

func (m *mockPermissionBroker) Grant(_ context.Context, _ permission.PermissionGrantRequest) (permission.PermissionGrant, error) {
	return permission.PermissionGrant{}, nil
}

func (m *mockPermissionBroker) Revoke(_ context.Context, _ string) error { return nil }

func (m *mockPermissionBroker) RevokeBySubject(_ context.Context, _ permission.PermissionSubject) (int, error) {
	return 0, nil
}

func (m *mockPermissionBroker) RevokeByExtension(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (m *mockPermissionBroker) ListGrants(_ context.Context, _ permission.PermissionGrantFilter) ([]permission.PermissionGrant, error) {
	return nil, nil
}

func (m *mockPermissionBroker) Explain(_ context.Context, _ permission.PermissionEvaluationRequest) permission.PermissionExplanation {
	return permission.PermissionExplanation{}
}

func (m *mockPermissionBroker) DetectUpgrade(_ context.Context, _, _ []permission.PermissionRequirement) []permission.PermissionUpgrade {
	return nil
}

func (m *mockPermissionBroker) RecordApproval(_ context.Context, _ permission.PermissionApprovalRecordRequest) (permission.PermissionApprovalRecord, error) {
	return permission.PermissionApprovalRecord{}, nil
}

func (m *mockPermissionBroker) ValidateSnapshot(_ context.Context, _ string, _ permission.PermissionEvaluationRequest) error {
	return nil
}

func TestCanonicalStartupAssertions_Pass(t *testing.T) {
	services := &AppServices{
		KernelContainer: &kernel.Container{
			ToolFacade:         new(kernel.ToolFacade),
			PermissionBroker:   &mockPermissionBroker{},
			EventService:       new(event.Service),
			ScheduleService:    new(schedule.ScheduleService),
			TaskRuntimeService: new(task_runtime.TaskRuntimeService),
			HookService:        new(hook.Service),
		},
	}
	if err := runCanonicalStartupAssertions(services); err != nil {
		t.Fatalf("expected startup assertions to pass, got: %v", err)
	}
}

func TestCanonicalStartupAssertions_NilContainer(t *testing.T) {
	services := &AppServices{}
	err := runCanonicalStartupAssertions(services)
	if err == nil {
		t.Fatal("expected startup assertions to fail with nil KernelContainer")
	}
}

func TestCanonicalStartupAssertions_LegacyMCPAbsent(t *testing.T) {
	services := &AppServices{
		KernelContainer: &kernel.Container{
			ToolFacade:         new(kernel.ToolFacade),
			PermissionBroker:   &mockPermissionBroker{},
			EventService:       new(event.Service),
			ScheduleService:    new(schedule.ScheduleService),
			TaskRuntimeService: new(task_runtime.TaskRuntimeService),
			HookService:        new(hook.Service),
		},
	}
	if services.hasLegacyMCPManager() {
		t.Fatal("expected hasLegacyMCPManager to return false after Legacy MCP removal")
	}
	if services.hasMemoryRawWriter() {
		t.Fatal("expected hasMemoryRawWriter to return false after Legacy MCP removal")
	}
}
