package extension

import (
	"context"
	"sync"
	"testing"
)

func TestSystemPolicyConcurrentGrantAndEvaluate(t *testing.T) {
	evaluator := NewPermissionEvaluator(nil)
	identity := ExtensionIdentity{SkillID: "dev.amitia.skill.concurrent"}
	scope := ExecutionScope{Trigger: TriggerLLM, SessionID: "session-1"}
	var group sync.WaitGroup
	for index := 0; index < 32; index++ {
		group.Add(2)
		go func() {
			defer group.Done()
			evaluator.GrantSystemPolicy(identity.SkillID, "memory.read", DecisionAllowSession)
		}()
		go func() {
			defer group.Done()
			decision := evaluator.EvaluateExecution(context.Background(), identity, "memory.read", scope)
			if decision != DecisionAllowSession && decision != DecisionDeny {
				t.Errorf("unexpected decision: %s", decision)
			}
		}()
	}
	group.Wait()
	if decision := evaluator.EvaluateExecution(context.Background(), identity, "memory.read", scope); decision != DecisionAllowSession {
		t.Fatalf("expected session permission after grants, got %s", decision)
	}
}
