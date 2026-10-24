package permission

import (
	"context"
	"testing"
)

func TestDefaultAutoPermissionAllowsWithoutStoredGrant(t *testing.T) {
	broker := NewDefaultPermissionBroker(NewPermissionDefinitionRegistry(), NewMemoryPermissionStorage())
	t.Cleanup(func() { _ = broker.Close() })
	result := broker.Evaluate(context.Background(), PermissionEvaluationRequest{
		Subject:      PermissionSubject{Type: SubjectSystem, ID: "core"},
		Requirements: []PermissionRequirement{{PermissionID: "workspace.read", Scope: ScopeGlobalOnly()}},
		InvocationID: "default-auto-read",
	})
	if result.Decision != DecisionAllow {
		t.Fatalf("decision=%s reasons=%+v", result.Decision, result.Reasons)
	}
	if len(result.Missing) != 0 {
		t.Fatalf("missing=%+v", result.Missing)
	}
}

func TestHostCommandPermissionIsRegistered(t *testing.T) {
	if _, ok := NewPermissionDefinitionRegistry().Get("runtime.host.command.execute"); !ok {
		t.Fatal("host command permission is missing")
	}
}
