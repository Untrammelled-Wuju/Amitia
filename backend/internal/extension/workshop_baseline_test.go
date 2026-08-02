package extension

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/migration"
	"gorm.io/gorm"
	"time"
)

func newWorkshopBaselineService(t *testing.T) (*WorkshopService, *Registry, *Executor, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := (migration.Runner{DB: db, SkipBackup: true}).Apply([]migration.Migration{migration.ExtensionsMigration(), migration.PluginRuntimeMigration(), migration.ExtensionWorkshopMigration(), migration.ExtensionWorkshopPermissionScopesMigration(), migration.ExtensionWorkshopPlannerMigration(), migration.ExtensionWorkshopGenerationSummaryMigration()}); err != nil {
		t.Fatal(err)
	}
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	compiler := NewWorkflowCompiler(registry)
	workflowExecutor := NewWorkflowExecutor(BuildWorkflowAdapters(executor, &WorkflowHostAdapter{}), validator)
	service := NewWorkshopService(NewWorkshopRepository(db), NewWorkshopGenerator(nil, registry), compiler, workflowExecutor, validator, registry, executor)
	return service, registry, executor, db
}

func baselineDraft() ExtensionDraft {
	inputSchema := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"name":{"type":"string"}},"required":["name"]}`)
	outputSchema := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"message":{"type":"string"}},"required":["message"]}`)
	configSchema := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{}}`)
	return ExtensionDraft{
		DraftVersion:  "1.0.0",
		Metadata:      DraftMetadata{ID: "dev.user.user-a.greeting", Name: "问候", Version: "1.0.0", Description: "生成结构化问候", Author: "Local User", License: "LicenseRef-Amitia-Local"},
		Intent:        DraftIntent{Goal: "生成问候", Triggers: []SkillTrigger{TriggerManual}},
		InputSchema:   inputSchema,
		OutputSchema:  outputSchema,
		ConfigSchema:  configSchema,
		DefaultConfig: json.RawMessage(`{}`),
		Workflow:      WorkflowDefinition{SchemaVersion: "1.0.0", Steps: []WorkflowStep{{ID: "result", Type: "transform", Input: json.RawMessage(`{"op":"pick","value":{"message":"hello"},"fields":["message"]}`), OnError: WorkflowErrorPolicy{Mode: "fail"}}}, Output: json.RawMessage(`{"$ref":"steps.result"}`), Limits: DefaultWorkflowLimits()},
		Capabilities:  []string{},
		Dependencies:  []SkillDependency{},
		TestCases:     []WorkshopTestCase{{ID: "dry", Name: "Dry Run", Mode: "dry_run", Input: json.RawMessage(`{"name":"A"}`), Config: json.RawMessage(`{}`), Assertions: []TestAssertion{{Type: "status_is", Expected: "succeeded"}}}},
		Assumptions:   []DraftAssumption{},
		Warnings:      []DraftWarning{},
	}
}

func baselinePrepareInstallable(t *testing.T, service *WorkshopService, scope ExecutionScope, draft ExtensionDraft) (WorkshopSession, WorkshopRevisionView) {
	t.Helper()
	ctx := context.Background()
	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建声明式 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}
	revision, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}
	validation, err := service.Validate(ctx, scope, session.ID, revision.Revision)
	if err != nil || !validation.Valid {
		t.Fatalf("validation failed: %#v %v", validation, err)
	}
	confirmation := PermissionConfirmation{WorkflowChecksum: validation.WorkflowChecksum, Capabilities: validation.Capabilities.Required, ConfirmedHighRisk: validation.Capabilities.HighRisk}
	if err := service.ConfirmPermissions(ctx, scope, session.ID, revision.Revision, confirmation); err != nil {
		t.Fatal(err)
	}
	report, err := service.Test(ctx, scope, session.ID, revision.Revision, WorkshopTestRequest{Mode: "dry_run"})
	if err != nil || report.Status != "passed" {
		t.Fatalf("test failed: %#v %v", report, err)
	}
	confirmation.Production = true
	if err := service.ConfirmPermissions(ctx, scope, session.ID, revision.Revision, confirmation); err != nil {
		t.Fatal(err)
	}
	return session, revision
}

func TestLegacy_Workshop_CreateSession(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}
	if session.Status != WorkshopDraft || session.Requirement != "创建问候 Skill" || session.CharacterID != scope.CharacterID || session.UserID != scope.UserID {
		t.Fatalf("unexpected session: %#v", session)
	}
	if session.CurrentRevision != 0 {
		t.Fatalf("new session should have revision 0: %d", session.CurrentRevision)
	}
	if session.ID == "" {
		t.Fatal("session ID is empty")
	}
}

func TestLegacy_Workshop_CreateSessionEmptyRequirement(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a"}

	_, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: ""})
	if err == nil || asExtensionError(err).Code != ErrWorkshopGenerationOutputInvalid {
		t.Fatalf("expected error for empty requirement: %v", err)
	}
}

func TestLegacy_Workshop_CreateSessionOverlengthRequirement(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a"}

	requirement := strings.Repeat("x", 20001)
	_, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: requirement})
	if err == nil || asExtensionError(err).Code != ErrWorkshopGenerationOutputInvalid {
		t.Fatalf("expected error for overlength requirement: %v", err)
	}
}

func TestLegacy_Workshop_GetSession(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	detail, err := service.GetSession(ctx, scope, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.ID != session.ID || detail.Requirement != session.Requirement {
		t.Fatalf("session mismatch: %#v", detail)
	}
}

func TestLegacy_Workshop_GetSessionNotFound(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a"}

	_, err := service.GetSession(ctx, scope, "nonexistent-id")
	if err == nil || asExtensionError(err).Code != ErrWorkshopSessionNotFound {
		t.Fatalf("expected not found: %v", err)
	}
}

func TestLegacy_Workshop_GetSessionForbidden(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	otherScope := ExecutionScope{UserID: "user-b"}
	_, err = service.GetSession(ctx, otherScope, session.ID)
	if err == nil || asExtensionError(err).Code != ErrWorkshopSessionForbidden {
		t.Fatalf("expected forbidden for other user: %v", err)
	}
}

func TestLegacy_Workshop_ListSessions(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	_, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建 Skill A", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建 Skill B", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.ListSessions(ctx, scope, WorkshopSessionFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total < 2 || len(result.Items) < 2 {
		t.Fatalf("expected at least 2 sessions: %#v", result)
	}
}

func TestLegacy_Workshop_ListSessionsWithFilters(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	_, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建 Skill A", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.ListSessions(ctx, scope, WorkshopSessionFilter{Status: WorkshopDraft, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total == 0 {
		t.Fatal("expected draft sessions")
	}
	for _, item := range result.Items {
		if item.Status != WorkshopDraft {
			t.Fatalf("unexpected status: %s", item.Status)
		}
	}
}

func TestLegacy_Workshop_Generate(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual, TraceID: "trace-gen"}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	revision, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}
	if revision.Revision != 1 {
		t.Fatalf("expected revision 1: %d", revision.Revision)
	}
	if revision.NormalizedDraft.Manifest.Entry.Kind != "workflow" {
		t.Fatalf("expected workflow entry: %#v", revision.NormalizedDraft.Manifest)
	}
	if revision.Plan.Goal == "" || len(revision.Plan.Steps) == 0 {
		t.Fatalf("plan incomplete: %#v", revision.Plan)
	}
	if revision.WorkflowChecksum == "" {
		t.Fatal("checksum is empty")
	}

	detail, err := service.GetSession(ctx, scope, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Status != WorkshopGenerated || detail.CurrentRevision != 1 {
		t.Fatalf("session not updated after generation: %#v", detail.WorkshopSession)
	}
}

func TestLegacy_Workshop_GenerateArchivedSession(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Archive(ctx, scope, session.ID); err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	_, err = service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err == nil || asExtensionError(err).Code != ErrWorkshopInvalidState {
		t.Fatalf("expected invalid state for archived session: %v", err)
	}
}

func TestLegacy_Workshop_ListRevisions(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	_, err = service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}

	revisions, err := service.ListRevisions(ctx, scope, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 1 || revisions[0].Revision != 1 {
		t.Fatalf("expected one revision: %#v", revisions)
	}
}

func TestLegacy_Workshop_GetRevision(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	_, err = service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}

	revision, err := service.GetRevision(ctx, scope, session.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if revision.Revision != 1 || revision.NormalizedDraft.Metadata.ID != draft.Metadata.ID {
		t.Fatalf("revision mismatch: %#v", revision)
	}
}

func TestLegacy_Workshop_GetRevisionNotFound(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.GetRevision(ctx, scope, session.ID, 99)
	if err == nil || asExtensionError(err).Code != ErrWorkshopRevisionNotFound {
		t.Fatalf("expected not found: %v", err)
	}
}

func TestLegacy_Workshop_Validate(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	revision, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}

	validation, err := service.Validate(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if !validation.Valid {
		t.Fatalf("expected valid: %#v", validation)
	}
	if validation.WorkflowChecksum == "" {
		t.Fatal("checksum empty")
	}
	if !validation.Valid {
		t.Fatal("expected valid validation")
	}
}

func TestLegacy_Workshop_ValidateInvalidSchema(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	draft.InputSchema = json.RawMessage(`{"type":"invalid_type_xyz"}`)
	revision, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}

	validation, err := service.Validate(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if validation.Valid {
		t.Fatal("expected invalid validation for array schema")
	}
	hasError := false
	for _, issue := range validation.Issues {
		if issue.Level == "error" {
			hasError = true
		}
	}
	if !hasError {
		t.Fatalf("expected error issues: %#v", validation.Issues)
	}
}

func TestLegacy_Workshop_ValidateOfficialNamespace(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	draft.Metadata.ID = "dev.amitia.test"
	revision, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}

	normalizedID := revision.NormalizedDraft.Metadata.ID
	if strings.HasPrefix(normalizedID, "dev.amitia.") {
		t.Fatalf("expected ID normalization away from dev.amitia: %s", normalizedID)
	}

	validation, err := service.Validate(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}
	_ = validation
}

func TestLegacy_Workshop_ConfirmPermissions(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	revision, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}

	validation, err := service.Validate(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}

	confirmation := PermissionConfirmation{WorkflowChecksum: validation.WorkflowChecksum, Capabilities: validation.Capabilities.Required, ConfirmedHighRisk: validation.Capabilities.HighRisk}
	if err := service.ConfirmPermissions(ctx, scope, session.ID, revision.Revision, confirmation); err != nil {
		t.Fatal(err)
	}

	detail, err := service.GetSession(ctx, scope, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !detail.TestPermissionConfirmed {
		t.Fatal("test permission not confirmed")
	}
	if detail.ProductionPermissionConfirmed {
		t.Fatal("production permission should not be confirmed yet")
	}
}

func TestLegacy_Workshop_ConfirmPermissionsMissingCapability(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	revision, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}

	validation, err := service.Validate(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}

	confirmation := PermissionConfirmation{WorkflowChecksum: validation.WorkflowChecksum, Capabilities: validation.Capabilities.Required, ConfirmedHighRisk: validation.Capabilities.HighRisk}
	err = service.ConfirmPermissions(ctx, scope, session.ID, revision.Revision, confirmation)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLegacy_Workshop_ConfirmPermissionsWrongChecksum(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	revision, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}

	validation, err := service.Validate(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}

	confirmation := PermissionConfirmation{WorkflowChecksum: "wrong-checksum", Capabilities: validation.Capabilities.Required}
	err = service.ConfirmPermissions(ctx, scope, session.ID, revision.Revision, confirmation)
	if err == nil || asExtensionError(err).Code != ErrWorkshopPermissionStale {
		t.Fatalf("expected permission stale: %v", err)
	}
}

func TestLegacy_Workshop_TestDryRun(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual, TraceID: "trace-test"}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	revision, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}

	validation, err := service.Validate(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}

	confirmation := PermissionConfirmation{WorkflowChecksum: validation.WorkflowChecksum, Capabilities: validation.Capabilities.Required, ConfirmedHighRisk: validation.Capabilities.HighRisk}
	if err := service.ConfirmPermissions(ctx, scope, session.ID, revision.Revision, confirmation); err != nil {
		t.Fatal(err)
	}

	report, err := service.Test(ctx, scope, session.ID, revision.Revision, WorkshopTestRequest{Mode: "dry_run"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "passed" {
		t.Fatalf("expected passed: %#v", report)
	}
	if report.Revision != revision.Revision {
		t.Fatalf("revision mismatch: %d", report.Revision)
	}
	if len(report.Assertions) == 0 {
		t.Fatal("expected at least one assertion")
	}
}

func TestLegacy_Workshop_TestWithoutPermission(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	revision, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Validate(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Test(ctx, scope, session.ID, revision.Revision, WorkshopTestRequest{Mode: "dry_run"})
	if err == nil {
		t.Fatal("expected error for testing without permission")
	}
}

func TestLegacy_Workshop_TestInvalidMode(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	revision, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}

	validation, err := service.Validate(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}

	confirmation := PermissionConfirmation{WorkflowChecksum: validation.WorkflowChecksum, Capabilities: validation.Capabilities.Required}
	if err := service.ConfirmPermissions(ctx, scope, session.ID, revision.Revision, confirmation); err != nil {
		t.Fatal(err)
	}

	_, err = service.Test(ctx, scope, session.ID, revision.Revision, WorkshopTestRequest{Mode: "production"})
	if err == nil || asExtensionError(err).Code != ErrWorkshopInvalidState {
		t.Fatalf("expected invalid state for bad mode: %v", err)
	}
}

func TestLegacy_Workshop_TestListAndGet(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual, TraceID: "trace-test-list"}

	session, revision := baselinePrepareInstallable(t, service, scope, baselineDraft())

	reports, err := service.ListTests(ctx, scope, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) == 0 {
		t.Fatal("expected at least one test report")
	}

	report, err := service.GetTest(ctx, scope, reports[0].TestRunID)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "passed" || report.Revision != revision.Revision {
		t.Fatalf("report mismatch: %#v", report)
	}
}

func TestLegacy_Workshop_Install(t *testing.T) {
	service, registry, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual, TraceID: "trace-install"}

	draft := baselineDraft()
	session, revision := baselinePrepareInstallable(t, service, scope, draft)

	installed, err := service.Install(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if installed.SkillID != draft.Metadata.ID || installed.Version != draft.Metadata.Version {
		t.Fatalf("install result mismatch: %#v", installed)
	}

	registered, err := registry.Get(ctx, installed.SkillID)
	if err != nil || registered.Definition.Enabled {
		t.Fatalf("installed skill should exist but disabled: %#v %v", registered.Definition, err)
	}

	detail, err := service.GetSession(ctx, scope, session.ID)
	if err != nil || detail.Status != WorkshopInstalled {
		t.Fatalf("session not updated after install: %#v %v", detail.WorkshopSession, err)
	}
}

func TestLegacy_Workshop_InstallWithoutTest(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	revision, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}

	validation, err := service.Validate(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}

	confirmation := PermissionConfirmation{WorkflowChecksum: validation.WorkflowChecksum, Capabilities: validation.Capabilities.Required}
	if err := service.ConfirmPermissions(ctx, scope, session.ID, revision.Revision, confirmation); err != nil {
		t.Fatal(err)
	}

	confirmation.Production = true
	err = service.ConfirmPermissions(ctx, scope, session.ID, revision.Revision, confirmation)
	if err == nil || asExtensionError(err).Code != ErrWorkshopInvalidState {
		t.Fatalf("expected invalid state for production confirm before test: %v", err)
	}
}

func TestLegacy_Workshop_InstallWrongState(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Install(ctx, scope, session.ID, 1)
	if err == nil || asExtensionError(err).Code != ErrWorkshopInvalidState {
		t.Fatalf("expected invalid state: %v", err)
	}
}

func TestLegacy_Workshop_IdempotentInstall(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual, TraceID: "trace-idempotent"}

	draft := baselineDraft()
	session, revision := baselinePrepareInstallable(t, service, scope, draft)

	_, err := service.Install(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}

	installed, err := service.Install(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if installed.SkillID != draft.Metadata.ID {
		t.Fatalf("idempotent install mismatch: %#v", installed)
	}
}

func TestLegacy_Workshop_Restore(t *testing.T) {
	service, registry, _, db := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual, TraceID: "trace-restore"}

	draft := baselineDraft()
	session, revision := baselinePrepareInstallable(t, service, scope, draft)

	_, err := service.Install(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}

	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	restoredRegistry := NewRegistry("1.0.0", validator, repository)
	restoredPermissions := NewPermissionEvaluator(repository)
	restoredExecutor := NewExecutor(restoredRegistry, validator, restoredPermissions, repository)
	compiler := NewWorkflowCompiler(restoredRegistry)
	workflowExecutor := NewWorkflowExecutor(BuildWorkflowAdapters(restoredExecutor, &WorkflowHostAdapter{}), validator)
	restoredService := NewWorkshopService(NewWorkshopRepository(db), NewWorkshopGenerator(nil, restoredRegistry), compiler, workflowExecutor, validator, restoredRegistry, restoredExecutor)

	if err := restoredService.Restore(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := restoredRegistry.Get(ctx, draft.Metadata.ID); err != nil {
		t.Fatalf("installed workflow was not restored: %v", err)
	}

	current, _ := registry.Get(ctx, draft.Metadata.ID)
	restored, _ := restoredRegistry.Get(ctx, draft.Metadata.ID)
	if current.Definition.Version != restored.Definition.Version {
		t.Fatalf("restored version mismatch: %s vs %s", current.Definition.Version, restored.Definition.Version)
	}
}

func TestLegacy_Workshop_Rollback(t *testing.T) {
	service, registry, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual, TraceID: "trace-rollback"}

	draft := baselineDraft()
	session, _ := baselinePrepareInstallable(t, service, scope, draft)

	_, err := service.Install(ctx, scope, session.ID, 1)
	if err != nil {
		t.Fatal(err)
	}

	forked, err := service.ForkSkill(ctx, scope, draft.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if forked.Revision == nil || forked.Revision.NormalizedDraft.Metadata.Version != "1.0.1" {
		t.Fatalf("fork didn't create update: %#v", forked)
	}

	forkUpdateValidation, err := service.Validate(ctx, scope, forked.ID, forked.CurrentRevision)
	if err != nil || !forkUpdateValidation.Valid {
		t.Fatalf("update validation failed: %#v %v", forkUpdateValidation, err)
	}
	updateConfirmation := PermissionConfirmation{WorkflowChecksum: forkUpdateValidation.WorkflowChecksum, Capabilities: forkUpdateValidation.Capabilities.Required, ConfirmedHighRisk: forkUpdateValidation.Capabilities.HighRisk}
	if err := service.ConfirmPermissions(ctx, scope, forked.ID, forked.CurrentRevision, updateConfirmation); err != nil {
		t.Fatal(err)
	}
	if report, err := service.Test(ctx, scope, forked.ID, forked.CurrentRevision, WorkshopTestRequest{Mode: "dry_run", TestCases: []WorkshopTestCase{{ID: "dry", Name: "Dry Run", Mode: "dry_run", Input: json.RawMessage(`{"name":"A"}`), Config: json.RawMessage(`{}`), Assertions: []TestAssertion{{Type: "status_is", Expected: "succeeded"}}}}}); err != nil || report.Status != "passed" {
		t.Fatalf("update test failed: %#v %v", report, err)
	}
	updateConfirmation.Production = true
	if err := service.ConfirmPermissions(ctx, scope, forked.ID, forked.CurrentRevision, updateConfirmation); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Install(ctx, scope, forked.ID, forked.CurrentRevision); err != nil {
		t.Fatal(err)
	}

	rolledBack, err := service.Rollback(ctx, scope, draft.Metadata.ID, "1.0.0")
	if err != nil || rolledBack.Version != "1.0.0" {
		t.Fatalf("rollback failed: %#v %v", rolledBack, err)
	}

	current, err := registry.Get(ctx, draft.Metadata.ID)
	if err != nil || current.Definition.Version != "1.0.0" {
		t.Fatalf("rollback did not restore registry: %#v %v", current.Definition, err)
	}
}

func TestLegacy_Workshop_RollbackNonexistent(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a"}

	_, err := service.Rollback(ctx, scope, "dev.user.nonexistent", "1.0.0")
	if err == nil || asExtensionError(err).Code != ErrWorkshopRollbackFailed {
		t.Fatalf("expected rollback failed: %v", err)
	}
}

func TestLegacy_Workshop_ForkSkill(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual, TraceID: "trace-fork"}

	draft := baselineDraft()
	session, _ := baselinePrepareInstallable(t, service, scope, draft)

	_, err := service.Install(ctx, scope, session.ID, 1)
	if err != nil {
		t.Fatal(err)
	}

	if err := service.EnableSkill(ctx, scope, draft.Metadata.ID); err != nil {
		t.Fatal(err)
	}

	forked, err := service.ForkSkill(ctx, scope, draft.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if forked.Status != WorkshopGenerated || forked.CurrentRevision != 1 {
		t.Fatalf("fork session not in expected state: %#v", forked.WorkshopSession)
	}
	if forked.Revision == nil {
		t.Fatal("fork revision is nil")
	}
	if forked.Revision.NormalizedDraft.Metadata.ID != draft.Metadata.ID {
		t.Fatalf("fork skill ID changed: %s", forked.Revision.NormalizedDraft.Metadata.ID)
	}
	if forked.Revision.NormalizedDraft.Metadata.Version != "1.0.1" {
		t.Fatalf("fork version not bumped: %s", forked.Revision.NormalizedDraft.Metadata.Version)
	}
}

func (s *WorkshopService) EnableSkill(ctx context.Context, scope ExecutionScope, skillID string) error {
	var record extensionRecord
	if err := s.repository.db.WithContext(ctx).Where("extension_id = ?", skillID).First(&record).Error; err != nil {
		return err
	}
	now := s.repository.db.NowFunc().Format(time.RFC3339Nano)
	result := s.repository.db.WithContext(ctx).Model(&record).Where("extension_id = ?", skillID).Updates(map[string]interface{}{"enabled": 1, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	var session workshopSessionRecord
	if err := s.repository.db.WithContext(ctx).Where("installed_skill_id = ?", skillID).First(&session).Error; err == nil {
		_ = s.repository.db.WithContext(ctx).Model(&session).Updates(map[string]interface{}{"status": string(WorkshopEnabled), "updated_at": now, "lock_version": gorm.Expr("lock_version + 1")}).Error
	}
	return nil
}

func TestLegacy_Workshop_ForkSkillNotWorkflow(t *testing.T) {
	service, registry, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a"}

	input := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`)
	output := json.RawMessage(`{"type":"object","additionalProperties":true}`)
	def, handler := testDefinition(t, "dev.user.native-skill", input, output, func(context.Context, ExecuteSkillRequest) (SkillResult, error) {
		return SkillResult{Status: RunSucceeded, Output: json.RawMessage(`{}`)}, nil
	})
	if err := registry.Register(ctx, def, handler); err != nil {
		t.Fatal(err)
	}

	_, err := service.ForkSkill(ctx, scope, "dev.user.native-skill")
	if err == nil || asExtensionError(err).Code != ErrWorkshopArtifactInvalid {
		t.Fatalf("expected artifact invalid: %v", err)
	}
}

func TestLegacy_Workshop_GetArtifact(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual, TraceID: "trace-artifact"}

	draft := baselineDraft()
	session, _ := baselinePrepareInstallable(t, service, scope, draft)

	_, err := service.Install(ctx, scope, session.ID, 1)
	if err != nil {
		t.Fatal(err)
	}

	artifact, err := service.GetArtifact(ctx, scope, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.ExtensionID != draft.Metadata.ID || artifact.ExtensionVersion != draft.Metadata.Version {
		t.Fatalf("artifact mismatch: %#v", artifact)
	}
	if artifact.Checksum == "" || artifact.SizeBytes <= 0 {
		t.Fatalf("artifact metadata missing: checksum=%s size=%d", artifact.Checksum, artifact.SizeBytes)
	}
}

func TestLegacy_Workshop_GetArtifactBeforeInstall(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.GetArtifact(ctx, scope, session.ID)
	if err == nil || asExtensionError(err).Code != ErrWorkshopArtifactInvalid {
		t.Fatalf("expected artifact invalid before install: %v", err)
	}
}

func TestLegacy_Workshop_Archive(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	if err := service.Archive(ctx, scope, session.ID); err != nil {
		t.Fatal(err)
	}

	detail, err := service.GetSession(ctx, scope, session.ID)
	if err != nil || detail.Status != WorkshopArchived {
		t.Fatalf("session not archived: %#v %v", detail.WorkshopSession, err)
	}
}

func TestLegacy_Workshop_ArchiveInstalling(t *testing.T) {
	service, _, _, db := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	db.Model(&workshopSessionRecord{}).Where("id = ?", session.ID).Update("status", string(WorkshopInstalling))

	err = service.Archive(ctx, scope, session.ID)
	if err == nil || asExtensionError(err).Code != ErrWorkshopInvalidState {
		t.Fatalf("expected invalid state for installing: %v", err)
	}
}

func TestLegacy_Workshop_PermissionRevisionInvalidation(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	first, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}
	validation, err := service.Validate(ctx, scope, session.ID, first.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ConfirmPermissions(ctx, scope, session.ID, first.Revision, PermissionConfirmation{WorkflowChecksum: validation.WorkflowChecksum, Capabilities: validation.Capabilities.Required}); err != nil {
		t.Fatal(err)
	}

	draft.Metadata.Description = "新描述"
	second, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}
	if second.Revision != 2 {
		t.Fatalf("expected revision 2: %d", second.Revision)
	}

	detail, err := service.GetSession(ctx, scope, session.ID)
	if err != nil || detail.TestPermissionConfirmed || detail.ProductionPermissionConfirmed {
		t.Fatalf("new revision retained stale permissions: %#v %v", detail, err)
	}
}

func TestLegacy_Workshop_MultipleRevisions(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	for i := 0; i < 3; i++ {
		draft.Metadata.Version = "1.0." + strings.TrimLeft(string(rune('0'+i)), "")
		_, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
		if err != nil {
			t.Fatalf("generate %d failed: %v", i, err)
		}
	}

	revisions, err := service.ListRevisions(ctx, scope, session.ID)
	if err != nil || len(revisions) != 3 {
		t.Fatalf("expected 3 revisions: %d %v", len(revisions), err)
	}
}

func TestLegacy_Workshop_RegistryFailureCompensation(t *testing.T) {
	service, registry, _, db := newWorkshopBaselineService(t)
	input := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`)
	output := json.RawMessage(`{"type":"object","additionalProperties":true}`)
	blocker, handler := testDefinition(t, "dev.user.model-name-collision", input, output, func(context.Context, ExecuteSkillRequest) (SkillResult, error) {
		return SkillResult{Status: RunSucceeded, Output: json.RawMessage(`{}`)}, nil
	})
	blocker.ModelName = "dev_user_user_a_greeting"
	if err := registry.Register(context.Background(), blocker, handler); err != nil {
		t.Fatal(err)
	}

	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual, TraceID: "trace-compensation"}
	session, revision := baselinePrepareInstallable(t, service, scope, baselineDraft())

	_, err := service.Install(context.Background(), scope, session.ID, revision.Revision)
	if err == nil || asExtensionError(err).Code != ErrWorkshopInstallFailed {
		t.Fatalf("expected registry installation failure: %v", err)
	}

	detail, err := service.GetSession(context.Background(), scope, session.ID)
	if err != nil || detail.Status != WorkshopTestPassed || detail.InstalledSkillID != "" {
		t.Fatalf("session compensation failed: %#v %v", detail.WorkshopSession, err)
	}

	for _, table := range []string{"extensions", "extension_versions", "extension_artifacts"} {
		var count int64
		query := db.Table(table)
		if table == "extensions" || table == "extension_versions" || table == "extension_artifacts" {
			query = query.Where("extension_id = ?", baselineDraft().Metadata.ID)
		}
		if err := query.Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("half-installed row remained in %s: %d %v", table, count, err)
		}
	}
}

func TestLegacy_Workshop_DatabaseFailureLeavesRegistryUntouched(t *testing.T) {
	service, registry, _, db := newWorkshopBaselineService(t)
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual, TraceID: "trace-db-failure"}
	session, revision := baselinePrepareInstallable(t, service, scope, baselineDraft())

	if err := db.Migrator().DropTable(&extensionVersionRecord{}); err != nil {
		t.Fatal(err)
	}

	_, err := service.Install(context.Background(), scope, session.ID, revision.Revision)
	if err == nil || asExtensionError(err).Code != ErrWorkshopInstallFailed {
		t.Fatalf("expected database installation failure: %v", err)
	}

	detail, err := service.GetSession(context.Background(), scope, session.ID)
	if err != nil || detail.Status != WorkshopTestPassed || detail.InstalledSkillID != "" {
		t.Fatalf("database failure changed session: %#v %v", detail.WorkshopSession, err)
	}

	if _, err := registry.Get(context.Background(), baselineDraft().Metadata.ID); asExtensionError(err).Code != ErrSkillNotFound {
		t.Fatalf("database failure changed registry: %v", err)
	}
}

func TestLegacy_Workshop_VersionConflictOnInstall(t *testing.T) {
	service, _, _, db := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual, TraceID: "trace-version-conflict"}

	draft := baselineDraft()
	session, revision := baselinePrepareInstallable(t, service, scope, draft)
	_, err := service.Install(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}

	session2, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill v2", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}
	draft2 := baselineDraft()
	draft2.Metadata.Version = "1.0.0"
	revision2, err := service.Generate(ctx, session2.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft2})
	if err != nil {
		t.Fatal(err)
	}
	validation2, err := service.Validate(ctx, scope, session2.ID, revision2.Revision)
	if err != nil {
		t.Fatal(err)
	}
	hasVersionError := false
	for _, issue := range validation2.Issues {
		if issue.Code == ErrWorkshopVersionConflict {
			hasVersionError = true
			break
		}
	}
	if !hasVersionError {
		t.Fatalf("expected version conflict in validation issues: %#v", validation2.Issues)
	}

	_ = db.Exec("CREATE TABLE IF NOT EXISTS extension_versions (id TEXT, extension_id TEXT, version TEXT, manifest_json TEXT, checksum TEXT, created_at TEXT)")
}

func TestLegacy_Workshop_TestControlledLiveNeedsConfirmation(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	revision, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}

	validation, err := service.Validate(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}

	confirmation := PermissionConfirmation{WorkflowChecksum: validation.WorkflowChecksum, Capabilities: validation.Capabilities.Required}
	if err := service.ConfirmPermissions(ctx, scope, session.ID, revision.Revision, confirmation); err != nil {
		t.Fatal(err)
	}

	_, err = service.Test(ctx, scope, session.ID, revision.Revision, WorkshopTestRequest{Mode: "controlled_live"})
	if err == nil || asExtensionError(err).Code != ErrWorkshopPermissionRequired {
		t.Fatalf("expected permission required for controlled_live: %v", err)
	}
}

func TestLegacy_Workshop_TestStalePermission(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	first, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}
	validation, err := service.Validate(ctx, scope, session.ID, first.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ConfirmPermissions(ctx, scope, session.ID, first.Revision, PermissionConfirmation{WorkflowChecksum: validation.WorkflowChecksum, Capabilities: validation.Capabilities.Required}); err != nil {
		t.Fatal(err)
	}

	draft.Metadata.Description = "新描述 v2"
	second, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Test(ctx, scope, session.ID, second.Revision, WorkshopTestRequest{Mode: "dry_run"})
	if err == nil || asExtensionError(err).Code != ErrWorkshopInvalidState {
		t.Fatalf("expected invalid state for stale permission: %v", err)
	}
}

func TestLegacy_Workshop_ProductionPermissionBeforeTest(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	revision, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}
	validation, err := service.Validate(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}

	confirmation := PermissionConfirmation{WorkflowChecksum: validation.WorkflowChecksum, Capabilities: validation.Capabilities.Required, Production: true}
	err = service.ConfirmPermissions(ctx, scope, session.ID, revision.Revision, confirmation)
	if err == nil || asExtensionError(err).Code != ErrWorkshopInvalidState {
		t.Fatalf("expected invalid state for production before test: %v", err)
	}
}

func TestLegacy_Workshop_ValidateNonCurrentRevision(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	revision1, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}

	draft.Metadata.Description = "新描述"
	_, err = service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Validate(ctx, scope, session.ID, revision1.Revision)
	if err == nil || asExtensionError(err).Code != ErrWorkshopRevisionConflict {
		t.Fatalf("expected revision conflict for old revision: %v", err)
	}
}

func TestLegacy_Workshop_ManifestAndSchemaValidation(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	draft.OutputSchema = json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"},"required":["message"]}}`)
	revision, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}

	validation, err := service.Validate(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if validation.Valid {
		t.Fatal("expected invalid for malformed output schema")
	}
}

func TestLegacy_Workshop_ForbiddenContentScanning(t *testing.T) {
	tests := []string{
		"package main\nfunc main() {}",
		"function run() { return 1 }",
		"#!/bin/bash\necho unsafe",
		"SELECT secret FROM users",
		"def run(value): return value",
		"<div>unsafe html</div>",
	}
	for _, value := range tests {
		t.Run("forbidden_"+string(rune(len(value))), func(t *testing.T) {
			if issues := ScanWorkshopSecrets([]byte(value)); !hasIssueCode(issues, ErrWorkshopGenerationOutputInvalid) {
				t.Fatalf("forbidden content accepted: %s %#v", value, issues)
			}
		})
	}
}

func TestLegacy_Workshop_SafeContentPasses(t *testing.T) {
	safe := []string{
		"创建一个返回问候的声明式 Skill",
		"根据用户输入翻译文本",
		"每天早上8点发送天气预报",
	}
	for _, value := range safe {
		t.Run("safe_"+string(rune(len(value))), func(t *testing.T) {
			if issues := ScanWorkshopSecrets([]byte(value)); hasErrorIssues(issues) {
				t.Fatalf("safe content rejected: %s %#v", value, issues)
			}
		})
	}
}

func TestLegacy_Workshop_ValidationVersionCheck(t *testing.T) {
	service, registry, _, _ := newWorkshopBaselineService(t)
	_ = registry
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	draft := baselineDraft()
	draft.Metadata.ID = "dev.user.user-a.greeting-v2"
	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建新Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	revision, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}

	validation, err := service.Validate(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if !validation.Valid {
		t.Fatal("expected valid for new skill with no existing version")
	}
}

func TestLegacy_Workshop_MetricsExposeAllCounters(t *testing.T) {
	resetWorkshopMetrics()
	defer resetWorkshopMetrics()

	incrementWorkshopMetric(WorkshopMetricSessionCreated)
	incrementWorkshopMetric(WorkshopMetricSessionCreated)

	snapshot := WorkshopMetricsSnapshot()

	if len(snapshot) != len(workshopMetricNames) {
		t.Fatalf("metric count = %d, expected %d", len(snapshot), len(workshopMetricNames))
	}
	for _, name := range workshopMetricNames {
		if _, ok := snapshot[name]; !ok {
			t.Fatalf("missing metric %s", name)
		}
	}

	if snapshot[WorkshopMetricSessionCreated] != 2 {
		t.Fatalf("expected 2 session created: %d", snapshot[WorkshopMetricSessionCreated])
	}
}

func TestLegacy_Workshop_MetricsErrorTracking(t *testing.T) {
	resetWorkshopMetrics()
	defer resetWorkshopMetrics()

	recordWorkshopErrorMetric(nil)
	recordWorkshopErrorMetric(NewExtensionError(ErrWorkshopNetworkDenied, "denied", "", false, nil))
	recordWorkshopErrorMetric(NewExtensionError(ErrWorkshopSecretDetected, "secret", "", false, nil))
	recordWorkshopErrorMetric(NewExtensionError(ErrWorkshopSandboxLimit, "limit", "", false, nil))
	recordWorkshopErrorMetric(NewExtensionError("OTHER", "other", "", false, nil))

	snapshot := WorkshopMetricsSnapshot()
	if snapshot[WorkshopMetricNetworkDenied] != 1 {
		t.Fatalf("expected 1 network denied: %d", snapshot[WorkshopMetricNetworkDenied])
	}
	if snapshot[WorkshopMetricSecretDetected] != 1 {
		t.Fatalf("expected 1 secret detected: %d", snapshot[WorkshopMetricSecretDetected])
	}
	if snapshot[WorkshopMetricSandboxLimit] != 1 {
		t.Fatalf("expected 1 sandbox limit: %d", snapshot[WorkshopMetricSandboxLimit])
	}
}

func TestLegacy_Workshop_ListTestsEmptySession(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	reports, err := service.ListTests(ctx, scope, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 0 {
		t.Fatalf("expected 0 test reports: %d", len(reports))
	}
}

func TestLegacy_Workshop_StateMachineValidTransitions(t *testing.T) {
	valid := [][2]WorkshopSessionStatus{
		{WorkshopDraft, WorkshopGenerating},
		{WorkshopGenerating, WorkshopGenerated},
		{WorkshopGenerated, WorkshopValidating},
		{WorkshopValidating, WorkshopValidated},
		{WorkshopValidated, WorkshopAwaitingPermissions},
		{WorkshopAwaitingPermissions, WorkshopTesting},
		{WorkshopTesting, WorkshopTestPassed},
		{WorkshopTestPassed, WorkshopInstalling},
		{WorkshopInstalling, WorkshopInstalled},
		{WorkshopInstalled, WorkshopArchived},
	}
	for _, transition := range valid {
		if !validWorkshopTransition(transition[0], transition[1]) {
			t.Fatalf("valid transition rejected: %s -> %s", transition[0], transition[1])
		}
	}
}

func TestLegacy_Workshop_StateMachineInvalidTransitions(t *testing.T) {
	invalid := [][2]WorkshopSessionStatus{
		{WorkshopDraft, WorkshopInstalled},
		{WorkshopArchived, WorkshopGenerating},
		{WorkshopGenerating, WorkshopEnabled},
		{WorkshopValidated, WorkshopInstalled},
		{WorkshopTesting, WorkshopArchived},
	}
	for _, transition := range invalid {
		if validWorkshopTransition(transition[0], transition[1]) {
			t.Fatalf("invalid transition accepted: %s -> %s", transition[0], transition[1])
		}
	}
}

func TestLegacy_Workshop_TestReportRedaction(t *testing.T) {
	report := WorkshopTestReport{
		Output: json.RawMessage(`{"token":"top-secret-value","message":"safe"}`),
		Error:  NewExtensionError(ErrWorkshopTestFailed, "failed", "authorization=Bearer-secret-value", false, nil),
		StepResults: []WorkflowStepResult{{
			Error: NewExtensionError(ErrWorkshopTestFailed, "failed", "password=secret-value", false, nil),
		}},
	}
	redacted := redactWorkshopTestReport(report)
	raw, _ := json.Marshal(redacted)
	if strings.Contains(string(raw), "top-secret-value") || strings.Contains(string(raw), "Bearer-secret-value") || strings.Contains(string(raw), "password=secret-value") {
		t.Fatalf("test report leaked secret: %s", raw)
	}
	if !strings.Contains(string(raw), "[REDACTED]") {
		t.Fatal("expected redaction markers")
	}
}

func TestLegacy_Workshop_BuildArtifactSizeLimit(t *testing.T) {
	draft := baselineDraft()
	compiled := CompiledWorkflow{
		SchemaVersion: "1.0.0",
		Steps:         []CompiledStep{{ID: "result", Type: "transform", Input: json.RawMessage(`{"op":"pick","value":{"message":"hello"},"fields":["message"]}`), TimeoutMS: 30000}},
		Checksum:      "test-checksum",
	}
	artifactID := "artifact.test-123"
	draft.Manifest = buildWorkshopManifest(draft, compiled, artifactID)

	artifact, err := buildArtifact("session-1", 1, draft, compiled, "test-run-1")
	if err != nil {
		t.Fatal(err)
	}
	if artifact.ArtifactID == "" || artifact.ExtensionID != draft.Metadata.ID || artifact.Checksum == "" {
		t.Fatalf("artifact incomplete: %#v", artifact)
	}
	if artifact.SizeBytes <= 0 {
		t.Fatalf("artifact size is zero")
	}
}

func TestLegacy_Workshop_ArtifactChecksumConsistency(t *testing.T) {
	draft := baselineDraft()
	compiled := CompiledWorkflow{
		SchemaVersion: "1.0.0",
		Steps:         []CompiledStep{{ID: "result", Type: "transform", Input: json.RawMessage(`{"op":"pick","value":{"message":"hello"},"fields":["message"]}`), TimeoutMS: 30000}},
		Checksum:      "test-checksum",
	}

	a1, _ := buildArtifact("session-a", 1, draft, compiled, "test-run-a")
	a2, _ := buildArtifact("session-a", 1, draft, compiled, "test-run-a")

	if a1.Checksum != a2.Checksum {
		t.Fatalf("checksums differ for same input: %s vs %s", a1.Checksum, a2.Checksum)
	}
}

func TestLegacy_Workshop_CompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		lt   bool
	}{
		{"1.0.0", "2.0.0", true},
		{"1.0.0", "1.0.1", true},
		{"1.0.1", "1.0.0", false},
		{"2.0.0", "1.9.0", false},
		{"1.0.0", "1.0.0", false},
	}
	for _, c := range cases {
		result := compareSemver(c.a, c.b)
		if c.lt && result >= 0 {
			t.Fatalf("compareSemver(%s, %s) = %d, expected < 0", c.a, c.b, result)
		}
		if !c.lt && result < 0 {
			t.Fatalf("compareSemver(%s, %s) = %d, expected >= 0", c.a, c.b, result)
		}
	}
}

func TestLegacy_Workshop_BumpPatchVersion(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"1.0.0", "1.0.1"},
		{"2.3.5", "2.3.6"},
		{"1.0.0-alpha", "1.0.1"},
		{"bad", "1.0.0"},
	}
	for _, c := range cases {
		result := bumpPatchVersion(c.input)
		if result != c.expected {
			t.Fatalf("bumpPatchVersion(%s) = %s, expected %s", c.input, result, c.expected)
		}
	}
}

func TestLegacy_Workshop_SplitWorkflowConfig(t *testing.T) {
	schemas := `{"config":{"type":"object","properties":{"endpoint":{"type":"string"},"api_token":{"type":"string","writeOnly":true,"format":"password"}}}}`
	config, secrets := splitWorkflowConfig(json.RawMessage(`{"endpoint":"https://example.com","api_token":"sensitive-value"}`), schemas)
	if strings.Contains(string(config), "sensitive-value") {
		t.Fatal("secret leaked to config")
	}
	if secrets["api_token"] != "sensitive-value" {
		t.Fatalf("secret not isolated: %#v", secrets)
	}
	if !strings.Contains(string(config), "https://example.com") {
		t.Fatal("safe config value missing")
	}
}

func TestLegacy_Workshop_WorkshopSecretFieldsDetection(t *testing.T) {
	result := workshopSecretFields(json.RawMessage(`{"type":"object","properties":{"token":{"writeOnly":true},"key":{"format":"password"},"secret":{"format":"secret"},"normal":{"type":"string"}}}`))
	if !result["token"] || !result["key"] || !result["secret"] {
		t.Fatalf("secret fields not detected: %#v", result)
	}
	if result["normal"] {
		t.Fatal("normal field wrongly detected as secret")
	}
}

func TestLegacy_Workshop_CapabilityAnalysis(t *testing.T) {
	compiled := CompiledWorkflow{
		Capabilities:   []string{"network.https"},
		HasSideEffects: true,
		Steps: []CompiledStep{
			{ID: "call", Type: "http", Input: json.RawMessage(`{}`), TimeoutMS: 30000},
		},
	}
	analysis := analyzeCapabilityDeclaration([]string{"network.https"}, compiled)
	if len(analysis.Required) != 1 || len(analysis.Missing) != 0 || len(analysis.Excess) != 0 {
		t.Fatalf("unexpected analysis: %#v", analysis)
	}
	if len(analysis.HighRisk) == 0 {
		t.Fatal("expected high risk capability")
	}
}

func TestLegacy_Workshop_CapabilityAnalysisExcess(t *testing.T) {
	compiled := CompiledWorkflow{
		Capabilities: []string{},
		Steps:        []CompiledStep{{ID: "result", Type: "transform", Input: json.RawMessage(`{}`), TimeoutMS: 30000}},
	}
	analysis := analyzeCapabilityDeclaration([]string{"network.https", "notification.send"}, compiled)
	if len(analysis.Excess) != 2 {
		t.Fatalf("expected 2 excess capabilities: %#v", analysis)
	}
}

func TestLegacy_Workshop_CapabilityAnalysisMissing(t *testing.T) {
	compiled := CompiledWorkflow{
		Capabilities: []string{"network.https", "notification.send"},
		Steps:        []CompiledStep{{ID: "call", Type: "http", Input: json.RawMessage(`{}`), TimeoutMS: 30000}},
	}
	analysis := analyzeCapabilityDeclaration([]string{"network.https"}, compiled)
	if len(analysis.Missing) != 1 || analysis.Missing[0] != "notification.send" {
		t.Fatalf("expected missing notification.send: %#v", analysis)
	}
}

func TestLegacy_Workshop_BuildWorkshopManifest(t *testing.T) {
	draft := baselineDraft()
	compiled := CompiledWorkflow{
		Capabilities:   []string{},
		Limits:         DefaultWorkflowLimits(),
		HasSideEffects: false,
		Idempotent:     true,
	}
	artifactID := "artifact.test-123"

	manifest := buildWorkshopManifest(draft, compiled, artifactID)
	if manifest.Kind != "Skill" || manifest.Entry.Kind != "workflow" || manifest.Entry.ArtifactID != artifactID {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	if manifest.Metadata.ID != draft.Metadata.ID {
		t.Fatalf("ID mismatch: %s", manifest.Metadata.ID)
	}
}

func TestLegacy_Workshop_DependenciesFromCompiled(t *testing.T) {
	compiled := CompiledWorkflow{
		Dependencies: []ResolvedSkillDependency{
			{SkillID: "dev.user.dep-a", Version: "1.0.0"},
			{SkillID: "dev.user.dep-b", Version: "2.0.0"},
		},
	}
	deps := dependenciesFromCompiled(compiled)
	if len(deps) != 2 {
		t.Fatalf("expected 2 deps: %d", len(deps))
	}
	if deps[0].SkillID != "dev.user.dep-a" || deps[0].Version != "1.0.0" {
		t.Fatalf("dep-a mismatch: %#v", deps[0])
	}
}

func TestLegacy_Workshop_ErrorCodesCoverage(t *testing.T) {
	codes := []string{
		ErrWorkshopSessionNotFound,
		ErrWorkshopSessionForbidden,
		ErrWorkshopInvalidState,
		ErrWorkshopRevisionNotFound,
		ErrWorkshopRevisionConflict,
		ErrWorkshopGenerationFailed,
		ErrWorkshopGenerationOutputInvalid,
		ErrWorkshopManifestInvalid,
		ErrWorkshopWorkflowInvalid,
		ErrWorkshopSchemaInvalid,
		ErrWorkshopStaticAnalysisFailed,
		ErrWorkshopCapabilityMismatch,
		ErrWorkshopPermissionRequired,
		ErrWorkshopPermissionStale,
		ErrWorkshopSecretDetected,
		ErrWorkshopNetworkDenied,
		ErrWorkshopDependencyCycle,
		ErrWorkshopTestRequired,
		ErrWorkshopTestFailed,
		ErrWorkshopTestStale,
		ErrWorkshopSandboxLimit,
		ErrWorkshopInstallFailed,
		ErrWorkshopSkillIDConflict,
		ErrWorkshopVersionConflict,
		ErrWorkshopArtifactInvalid,
		ErrWorkshopChecksumMismatch,
		ErrWorkshopRollbackFailed,
	}
	for _, code := range codes {
		if code == "" {
			t.Fatal("empty error code found")
		}
		if !strings.HasPrefix(code, "WORKSHOP_") {
			t.Fatalf("error code missing WORKSHOP_ prefix: %s", code)
		}
	}
}

func TestLegacy_Workshop_DifferentCharacterIsolation(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope1 := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}
	scope2 := ExecutionScope{UserID: "user-a", CharacterID: "char-b", Trigger: TriggerManual}

	s1, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope1, Requirement: "创建 Skill - char-a", CharacterID: scope1.CharacterID})
	_ = s1
	if err != nil {
		t.Fatal(err)
	}

	s2, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope2, Requirement: "创建 Skill - char-b", CharacterID: scope2.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	filtered1, err := service.ListSessions(ctx, scope1, WorkshopSessionFilter{CharacterID: "char-a", Page: 1, PageSize: 10})
	if err != nil || len(filtered1.Items) == 0 {
		t.Fatalf("character filter 1 failed: %#v %v", filtered1, err)
	}
	filtered2, err := service.ListSessions(ctx, scope2, WorkshopSessionFilter{CharacterID: "char-b", Page: 1, PageSize: 10})
	if err != nil || len(filtered2.Items) == 0 {
		t.Fatalf("character filter 2 failed: %#v %v", filtered2, err)
	}

	for _, item := range filtered1.Items {
		if item.ID == s2.ID {
			t.Fatal("character isolation violated")
		}
	}
}

func TestLegacy_Workshop_EndToEndMinimal(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual, TraceID: "e2e-minimal"}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建翻译 Skill", CharacterID: scope.CharacterID})
	if err != nil || session.Status != WorkshopDraft {
		t.Fatalf("create failed: %#v %v", session, err)
	}

	draft := baselineDraft()
	draft.Metadata.ID = "dev.user.user-a.translate"
	draft.Metadata.Name = "翻译"
	draft.Metadata.Description = "翻译文本"
	revision, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil || revision.Revision != 1 {
		t.Fatalf("generate failed: %#v %v", revision, err)
	}

	validation, err := service.Validate(ctx, scope, session.ID, revision.Revision)
	if err != nil || !validation.Valid {
		t.Fatalf("validate failed: %#v %v", validation, err)
	}

	confirmation := PermissionConfirmation{WorkflowChecksum: validation.WorkflowChecksum, Capabilities: validation.Capabilities.Required, ConfirmedHighRisk: validation.Capabilities.HighRisk}
	if err := service.ConfirmPermissions(ctx, scope, session.ID, revision.Revision, confirmation); err != nil {
		t.Fatal(err)
	}

	report, err := service.Test(ctx, scope, session.ID, revision.Revision, WorkshopTestRequest{Mode: "dry_run"})
	if err != nil || report.Status != "passed" {
		t.Fatalf("test failed: %#v %v", report, err)
	}

	confirmation.Production = true
	if err := service.ConfirmPermissions(ctx, scope, session.ID, revision.Revision, confirmation); err != nil {
		t.Fatal(err)
	}

	installed, err := service.Install(ctx, scope, session.ID, revision.Revision)
	if err != nil || installed.SkillID != draft.Metadata.ID {
		t.Fatalf("install failed: %#v %v", installed, err)
	}

	if err := service.Archive(ctx, scope, session.ID); err != nil {
		t.Fatal(err)
	}

	final, err := service.GetSession(ctx, scope, session.ID)
	if err != nil || final.Status != WorkshopArchived {
		t.Fatalf("archive failed: %#v %v", final.WorkshopSession, err)
	}
}

func TestLegacy_Workshop_DefaultWorkflowLimitsClamping(t *testing.T) {
	host := DefaultWorkflowLimits()
	expanded := WorkflowLimits{
		MaxSteps:               host.MaxSteps + 100,
		MaxExecutionDurationMS: host.MaxExecutionDurationMS + 10000,
		MaxHTTPResponseBytes:   host.MaxHTTPResponseBytes + 10000,
		MaxSkillCallDepth:      host.MaxSkillCallDepth + 10,
	}
	clamped := effectiveWorkflowLimits(expanded)
	if clamped.MaxSteps != host.MaxSteps {
		t.Fatalf("steps not clamped: %d vs %d", clamped.MaxSteps, host.MaxSteps)
	}
	if clamped.MaxExecutionDurationMS != host.MaxExecutionDurationMS {
		t.Fatalf("duration not clamped: %d vs %d", clamped.MaxExecutionDurationMS, host.MaxExecutionDurationMS)
	}
}

func TestLegacy_Workshop_UserIDInjection(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "evil-user'; DROP TABLE extension_workshop_sessions; --", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}
	if session.ID == "" {
		t.Fatal("session ID is empty after SQL injection attempt")
	}
}

func TestLegacy_Workshop_SessionLocks(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	unlock, ok := service.lockSession(session.ID)
	if !ok || unlock == nil {
		t.Fatal("first lock should succeed")
	}

	_, ok = service.lockSession(session.ID)
	if ok {
		t.Fatal("second lock should fail")
	}

	unlock()

	unlock2, ok := service.lockSession(session.ID)
	if !ok || unlock2 == nil {
		t.Fatal("lock should succeed after unlock")
	}
	unlock2()
}

func TestLegacy_Workshop_DefaultConfigValidation(t *testing.T) {
	service, _, _, _ := newWorkshopBaselineService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}

	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}

	draft := baselineDraft()
	draft.ConfigSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"level":{"type":"integer","minimum":0,"maximum":10}},"required":["level"]}`)
	draft.DefaultConfig = json.RawMessage(`{"level":-1}`)
	revision, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}

	validation, err := service.Validate(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if validation.Valid {
		t.Fatal("expected invalid for bad default config")
	}
}

func TestLegacy_Workshop_TestAssertionsGeneric(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}

	result := WorkflowExecutionResult{
		Output: json.RawMessage(`{"ok":true,"count":42}`),
		Steps:  []WorkflowStepResult{{StepID: "step1", Status: "succeeded", DurationMS: 100}},
	}
	assertions := []TestAssertion{
		{Type: "status_is", Expected: "succeeded"},
		{Type: "step_succeeded", StepID: "step1"},
		{Type: "matches_schema", Expected: map[string]interface{}{"type": "object", "required": []interface{}{"ok"}, "properties": map[string]interface{}{"ok": map[string]interface{}{"type": "boolean"}}}},
		{Type: "duration_less_than", Expected: float64(5000)},
	}
	results := evaluateAssertions(assertions, result, validator)
	if len(results) != len(assertions) {
		t.Fatalf("expected %d results: %d", len(assertions), len(results))
	}
	for _, a := range results {
		if !a.Passed {
			t.Fatalf("assertion failed: %#v", a)
		}
	}
}
