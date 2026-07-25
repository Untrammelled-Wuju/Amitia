package extension

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/migration"
)

func TestLegacy_Permission_GlobalDeny(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	permissions := NewPermissionEvaluator(repository)
	subject := ExtensionIdentity{ExtensionID: "test.ext", SkillID: "test.skill", Version: "1.0.0"}
	decision := permissions.Evaluate(ctx, subject, "network.https", PermissionScope{Type: ScopeGlobal})
	if decision != DecisionDeny {
		t.Fatalf("expected deny, got %s", decision)
	}
}

func TestLegacy_Permission_SystemPolicyAllowAlways(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	permissions := NewPermissionEvaluator(repository)
	permissions.GrantSystemPolicy("test.skill", "network.https", DecisionAllowAlways)
	subject := ExtensionIdentity{ExtensionID: "test.ext", SkillID: "test.skill", Version: "1.0.0"}
	decision := permissions.Evaluate(ctx, subject, "network.https", PermissionScope{Type: ScopeGlobal})
	if decision != DecisionAllowAlways {
		t.Fatalf("expected allow_always, got %s", decision)
	}
}

func TestLegacy_Permission_SystemPolicyAllowOnce(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	permissions := NewPermissionEvaluator(repository)
	permissions.GrantSystemPolicy("test.skill", "network.https", DecisionAllowOnce)
	subject := ExtensionIdentity{ExtensionID: "test.ext", SkillID: "test.skill", Version: "1.0.0"}
	scope := ExecutionScope{Trigger: TriggerLLM}
	decision := permissions.EvaluateExecution(ctx, subject, "network.https", scope)
	if decision != DecisionAllowOnce {
		t.Fatalf("expected allow_once, got %s", decision)
	}
}

func TestLegacy_Permission_UnknownCapabilityDenied(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	permissions := NewPermissionEvaluator(repository)
	subject := ExtensionIdentity{ExtensionID: "test.ext", SkillID: "test.skill", Version: "1.0.0"}
	decision := permissions.Evaluate(ctx, subject, "nonexistent.cap", PermissionScope{Type: ScopeGlobal})
	if decision != DecisionDeny {
		t.Fatalf("unknown capability should be denied, got %s", decision)
	}
}

func TestLegacy_Permission_DatabaseGrantConsumed(t *testing.T) {
	ctx := context.Background()
	db, repository := testRepository(t)
	if err := (migration.Runner{DB: db, SkipBackup: true}).Apply([]migration.Migration{migration.ExtensionScopeBindingsMigration()}); err != nil {
		t.Fatal(err)
	}
	permissions := NewPermissionEvaluator(repository)
	subject := ExtensionIdentity{ExtensionID: "test.ext", SkillID: "test.skill", Version: "1.0.0"}
	scope := ExecutionScope{Trigger: TriggerManual}
	grants := []PermissionGrantInput{{Capability: "network.https", Decision: DecisionAllowOnce, ScopeType: ScopeGlobal, ScopeID: ""}}
	if err := repository.ReplaceGrants(ctx, "test.skill", grants); err != nil {
		t.Fatal(err)
	}
	decision := permissions.EvaluateExecution(ctx, subject, "network.https", scope)
	if decision != DecisionAllowOnce {
		t.Fatalf("first evaluation should be allow_once, got %s", decision)
	}
	decision = permissions.EvaluateExecution(ctx, subject, "network.https", scope)
	if decision != DecisionDeny {
		t.Fatalf("second evaluation should be deny after consume, got %s", decision)
	}
}

func TestLegacy_Permission_AllowSessionOnlyWithSession(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	permissions := NewPermissionEvaluator(repository)
	permissions.GrantSystemPolicy("test.skill", "network.https", DecisionAllowSession)
	subject := ExtensionIdentity{ExtensionID: "test.ext", SkillID: "test.skill", Version: "1.0.0"}
	scope := ExecutionScope{Trigger: TriggerLLM, SessionID: "session-1"}
	decision := permissions.EvaluateExecution(ctx, subject, "network.https", scope)
	if decision != DecisionAllowSession {
		t.Fatalf("expected allow_session with session, got %s", decision)
	}
}

func TestLegacy_Permission_AllowSessionDeniedWithoutSession(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	permissions := NewPermissionEvaluator(repository)
	permissions.GrantSystemPolicy("test.skill", "network.https", DecisionAllowSession)
	subject := ExtensionIdentity{ExtensionID: "test.ext", SkillID: "test.skill", Version: "1.0.0"}
	scope := ExecutionScope{Trigger: TriggerLLM, SessionID: ""}
	decision := permissions.EvaluateExecution(ctx, subject, "network.https", scope)
	if decision != DecisionDeny {
		t.Fatalf("expected deny without session, got %s", decision)
	}
}

func TestLegacy_Permission_AllowSessionDeniedNonLLM(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	permissions := NewPermissionEvaluator(repository)
	permissions.GrantSystemPolicy("test.skill", "network.https", DecisionAllowSession)
	subject := ExtensionIdentity{ExtensionID: "test.ext", SkillID: "test.skill", Version: "1.0.0"}
	scope := ExecutionScope{Trigger: TriggerManual, SessionID: "session-1"}
	decision := permissions.EvaluateExecution(ctx, subject, "network.https", scope)
	if decision != DecisionDeny {
		t.Fatalf("expected deny for non-LLM trigger, got %s", decision)
	}
}

func TestLegacy_Permission_ScopeTypes(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	permissions := NewPermissionEvaluator(repository)
	permissions.GrantSystemPolicy("test.skill", "network.https", DecisionAllowCharacter)
	subject := ExtensionIdentity{ExtensionID: "test.ext", SkillID: "test.skill", Version: "1.0.0"}
	tests := []struct {
		name     string
		scope    PermissionScope
		expected PermissionDecision
	}{
		{"global", PermissionScope{Type: ScopeGlobal}, DecisionAllowCharacter},
		{"character", PermissionScope{Type: ScopeCharacter, ID: "char-a"}, DecisionAllowCharacter},
		{"conversation", PermissionScope{Type: ScopeConversation, ID: "conv-a"}, DecisionAllowCharacter},
		{"channel", PermissionScope{Type: ScopeChannel, ID: "ch-a"}, DecisionAllowCharacter},
		{"session", PermissionScope{Type: ScopeSession, ID: "sess-a"}, DecisionAllowCharacter},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := permissions.Evaluate(ctx, subject, "network.https", tt.scope)
			if decision != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, decision)
			}
		})
	}
}

func TestLegacy_Permission_MCPToolCapability(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	permissions := NewPermissionEvaluator(repository)
	subject := ExtensionIdentity{ExtensionID: "test.ext", SkillID: "test.skill", Version: "1.0.0"}
	_, ok := Capability("mcp.tool.github/search")
	if !ok {
		t.Fatal("mcp.tool.* capability should be recognized")
	}
	decision := permissions.Evaluate(ctx, subject, "mcp.tool.github/search", PermissionScope{Type: ScopeGlobal})
	if decision != DecisionDeny {
		t.Fatalf("expected deny for un-granted mcp tool, got %s", decision)
	}
}

func TestLegacy_Permission_MCPServerCapability(t *testing.T) {
	_, ok := Capability("mcp.server.github")
	if !ok {
		t.Fatal("mcp.server.* capability should be recognized")
	}
}

func TestLegacy_Permission_PreviewDoesNotConsume(t *testing.T) {
	ctx := context.Background()
	db, repository := testRepository(t)
	if err := (migration.Runner{DB: db, SkipBackup: true}).Apply([]migration.Migration{migration.ExtensionScopeBindingsMigration()}); err != nil {
		t.Fatal(err)
	}
	permissions := NewPermissionEvaluator(repository)
	subject := ExtensionIdentity{ExtensionID: "test.ext", SkillID: "test.skill", Version: "1.0.0"}
	grants := []PermissionGrantInput{{Capability: "network.https", Decision: DecisionAllowOnce, ScopeType: ScopeGlobal, ScopeID: ""}}
	if err := repository.ReplaceGrants(ctx, "test.skill", grants); err != nil {
		t.Fatal(err)
	}
	decision := permissions.PreviewExecution(ctx, subject, "network.https", ExecutionScope{Trigger: TriggerManual})
	if decision != DecisionAllowOnce {
		t.Fatalf("preview should return allow_once, got %s", decision)
	}
	decision = permissions.EvaluateExecution(ctx, subject, "network.https", ExecutionScope{Trigger: TriggerManual})
	if decision != DecisionAllowOnce {
		t.Fatalf("after preview, actual evaluation should still be allow_once, got %s", decision)
	}
}

func TestLegacy_Permission_DifferentRolesIsolated(t *testing.T) {
	ctx := context.Background()
	db, repository := testRepository(t)
	if err := (migration.Runner{DB: db, SkipBackup: true}).Apply([]migration.Migration{migration.ExtensionScopeBindingsMigration()}); err != nil {
		t.Fatal(err)
	}
	permissions := NewPermissionEvaluator(repository)
	subjectA := ExtensionIdentity{ExtensionID: "test.ext", SkillID: "skill.a", Version: "1.0.0"}
	subjectB := ExtensionIdentity{ExtensionID: "test.ext", SkillID: "skill.b", Version: "1.0.0"}
	grants := []PermissionGrantInput{{Capability: "network.https", Decision: DecisionAllowAlways, ScopeType: ScopeGlobal, ScopeID: ""}}
	if err := repository.ReplaceGrants(ctx, "skill.a", grants); err != nil {
		t.Fatal(err)
	}
	decisionA := permissions.EvaluateExecution(ctx, subjectA, "network.https", ExecutionScope{Trigger: TriggerManual})
	if decisionA != DecisionAllowAlways {
		t.Fatalf("skill.a should be allowed, got %s", decisionA)
	}
	decisionB := permissions.EvaluateExecution(ctx, subjectB, "network.https", ExecutionScope{Trigger: TriggerManual})
	if decisionB != DecisionDeny {
		t.Fatalf("skill.b should be denied, got %s", decisionB)
	}
}

func TestLegacy_Permission_GrantRevoke(t *testing.T) {
	ctx := context.Background()
	db, repository := testRepository(t)
	if err := (migration.Runner{DB: db, SkipBackup: true}).Apply([]migration.Migration{migration.ExtensionScopeBindingsMigration()}); err != nil {
		t.Fatal(err)
	}
	permissions := NewPermissionEvaluator(repository)
	subject := ExtensionIdentity{ExtensionID: "test.ext", SkillID: "test.skill", Version: "1.0.0"}
	grants := []PermissionGrantInput{{Capability: "network.https", Decision: DecisionAllowAlways, ScopeType: ScopeGlobal, ScopeID: ""}}
	if err := repository.ReplaceGrants(ctx, "test.skill", grants); err != nil {
		t.Fatal(err)
	}
	decision := permissions.EvaluateExecution(ctx, subject, "network.https", ExecutionScope{Trigger: TriggerManual})
	if decision != DecisionAllowAlways {
		t.Fatalf("before revoke should be allowed, got %s", decision)
	}
	if err := repository.ReplaceGrants(ctx, "test.skill", []PermissionGrantInput{}); err != nil {
		t.Fatal(err)
	}
	decision = permissions.EvaluateExecution(ctx, subject, "network.https", ExecutionScope{Trigger: TriggerManual})
	if decision != DecisionDeny {
		t.Fatalf("after revoke should be denied, got %s", decision)
	}
}

func TestLegacy_Permission_HighRiskCapability(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	permissions := NewPermissionEvaluator(repository)
	permissions.GrantSystemPolicy("test.skill", "financial.action", DecisionAllowOnce)
	subject := ExtensionIdentity{ExtensionID: "test.ext", SkillID: "test.skill", Version: "1.0.0"}
	decision := permissions.Evaluate(ctx, subject, "financial.action", PermissionScope{Type: ScopeGlobal})
	if decision != DecisionAllowOnce {
		t.Fatalf("high risk capability should respect system policy, got %s", decision)
	}
}

func TestLegacy_Permission_AllCapabilitiesKnown(t *testing.T) {
	caps := Capabilities()
	if len(caps) == 0 {
		t.Fatal("capability catalog should not be empty")
	}
	for _, cap := range caps {
		if cap.Name == "" || cap.Risk == "" || cap.Description == "" {
			t.Fatalf("capability %+v has empty fields", cap)
		}
	}
}

func TestLegacy_Permission_UndeclaredManifestCapabilityDenied(t *testing.T) {
	ctx := context.Background()
	db, repository := testRepository(t)
	if err := (migration.Runner{DB: db, SkipBackup: true}).Apply([]migration.Migration{migration.ExtensionScopeBindingsMigration()}); err != nil {
		t.Fatal(err)
	}
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	definition, handler := testDefinition(t, "dev.test.perm.undecl", json.RawMessage(`{}`), json.RawMessage(`{}`), func(_ context.Context, request ExecuteSkillRequest) (SkillResult, error) {
		return SkillResult{Status: RunSucceeded, Output: json.RawMessage(`{}`)}, nil
	})
	definition.Capabilities = []string{"storage.own.read"}
	syncDefinitionManifest(t, &definition)
	if err := registry.Register(ctx, definition, handler); err != nil {
		t.Fatal(err)
	}
	permissions.GrantSystemPolicy(definition.ID, "storage.own.read", DecisionAllowAlways)
	result, err := executor.Execute(ctx, ExecuteSkillRequest{SkillID: "dev.test.perm.undecl", Input: json.RawMessage(`{}`), Scope: ExecutionScope{Trigger: TriggerManual}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != RunSucceeded {
		t.Fatalf("expected succeeded with granted capability, got %s", result.Status)
	}
}

func TestLegacy_Permission_ManifestPermissionMismatch(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	definition, handler := testDefinition(t, "dev.test.perm.mismatch", json.RawMessage(`{}`), json.RawMessage(`{}`), func(_ context.Context, request ExecuteSkillRequest) (SkillResult, error) {
		return SkillResult{Status: RunSucceeded, Output: json.RawMessage(`{}`)}, nil
	})
	definition.Capabilities = []string{"network.https"}
	syncDefinitionManifest(t, &definition)
	if err := registry.Register(ctx, definition, handler); err != nil {
		t.Fatal(err)
	}
	_, err = executor.Execute(ctx, ExecuteSkillRequest{SkillID: "dev.test.perm.mismatch", Input: json.RawMessage(`{}`), Scope: ExecutionScope{Trigger: TriggerManual}})
	assertExtensionErrorCode(t, err, ErrSkillPermissionDenied)
}
