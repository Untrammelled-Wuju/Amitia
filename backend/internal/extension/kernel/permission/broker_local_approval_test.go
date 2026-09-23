package permission

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestLocalApprovalRecordAuthorizesBoundInvocation(t *testing.T) {
	ctx := context.Background()
	broker := NewDefaultPermissionBroker(NewPermissionDefinitionRegistry(), NewMemoryPermissionStorage())
	subject := PermissionSubject{Type: SubjectSystem, ID: "core"}
	requirements := []PermissionRequirement{{PermissionID: "service.runtime.execute"}}
	executionContext := PermissionExecutionContext{
		Placement: ExecutionPlacementLocal,
		SpaceID:   runtimeidentity.ParseSpaceID("space-1"),
		Source:    "model",
	}
	request := PermissionEvaluationRequest{
		Subject:          subject,
		Requirements:     requirements,
		InvocationID:     "inv-1",
		ScopeSnapshotID:  "scope-1",
		ExecutionContext: executionContext,
	}

	initial := broker.Evaluate(ctx, request)
	if initial.Decision != DecisionRequireApproval {
		t.Fatalf("initial decision = %s, want %s", initial.Decision, DecisionRequireApproval)
	}

	record, err := broker.RecordApproval(ctx, PermissionApprovalRecordRequest{
		InvocationID:        request.InvocationID,
		PermissionIDs:       []string{"service.runtime.execute"},
		ScopeSnapshotID:     request.ScopeSnapshotID,
		Decision:            ApprovalDecisionApproved,
		ExecutionContext:    executionContext,
		ExecutionBindingKey: executionContext.BindingKey(),
		RiskLevel:           "high",
	})
	if err != nil {
		t.Fatal(err)
	}

	request.ApprovalRecordID = record.RecordID
	resolved := broker.Evaluate(ctx, request)
	if resolved.Decision != DecisionAllow {
		t.Fatalf("resolved decision = %s, want %s, reasons = %#v", resolved.Decision, DecisionAllow, resolved.Reasons)
	}
}
