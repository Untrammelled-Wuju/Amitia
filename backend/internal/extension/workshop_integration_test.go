package extension

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/migration"
	"gorm.io/gorm"
)

func newWorkshopIntegrationService(t *testing.T) (*WorkshopService, *Registry, *Executor, *gorm.DB) {
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

func integrationDraft() ExtensionDraft {
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

func prepareInstallableWorkshop(t *testing.T, service *WorkshopService, scope ExecutionScope, draft ExtensionDraft) (WorkshopSession, WorkshopRevisionView) {
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

func TestWorkshopEndToEndInstallExecuteAndRestore(t *testing.T) {
	service, registry, executor, db := newWorkshopIntegrationService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", ConversationID: "conv-a", Channel: "web", Trigger: TriggerManual, TraceID: "trace-a", RequestID: "request-a"}
	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建一个返回问候的声明式 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}
	draft := integrationDraft()
	revision, err := service.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}
	if revision.Revision != 1 || revision.NormalizedDraft.Manifest.Entry.Kind != "workflow" || revision.Plan.Goal == "" || len(revision.Plan.Steps) != 1 {
		t.Fatalf("unexpected revision: %#v", revision)
	}
	var revisionRecord workshopRevisionRecord
	if err := db.Where("session_id = ? AND revision = ?", session.ID, revision.Revision).First(&revisionRecord).Error; err != nil || !strings.Contains(revisionRecord.ModelInputSummaryJSON, `"sha256"`) || strings.Contains(revisionRecord.ModelInputSummaryJSON, "创建一个返回问候") || !strings.Contains(revisionRecord.ModelOutputSummaryJSON, `"plan"`) {
		t.Fatalf("generation summaries missing or unsafe: %#v %v", revisionRecord, err)
	}
	var audits []pluginAuditRecord
	if err := db.Where("extension_id = ?", "workshop:"+session.ID).Find(&audits).Error; err != nil || len(audits) < 3 {
		t.Fatalf("workshop transition audits missing: %d %v", len(audits), err)
	}
	for _, audit := range audits {
		if audit.TraceID != scope.TraceID || !strings.Contains(audit.DetailJSON, `"userId":"user-a"`) || !strings.Contains(audit.DetailJSON, `"operation":`) {
			t.Fatalf("incomplete workshop audit: %#v", audit)
		}
	}
	validation, err := service.Validate(ctx, scope, session.ID, revision.Revision)
	if err != nil || !validation.Valid {
		t.Fatalf("validation failed: %#v %v", validation, err)
	}
	confirmation := PermissionConfirmation{WorkflowChecksum: validation.WorkflowChecksum, Capabilities: validation.Capabilities.Required, ConfirmedHighRisk: validation.Capabilities.HighRisk}
	if err := service.ConfirmPermissions(ctx, scope, session.ID, revision.Revision, confirmation); err != nil {
		t.Fatal(err)
	}
	detail, err := service.GetSession(ctx, scope, session.ID)
	if err != nil || !detail.TestPermissionConfirmed || detail.ProductionPermissionConfirmed {
		t.Fatalf("permission scopes mixed: %#v %v", detail.WorkshopSession, err)
	}
	report, err := service.Test(ctx, scope, session.ID, revision.Revision, WorkshopTestRequest{Scope: scope, Mode: "dry_run"})
	if err != nil || report.Status != "passed" {
		t.Fatalf("test failed: %#v %v", report, err)
	}
	confirmation.Production = true
	if err := service.ConfirmPermissions(ctx, scope, session.ID, revision.Revision, confirmation); err != nil {
		t.Fatal(err)
	}
	installed, err := service.Install(ctx, scope, session.ID, revision.Revision)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := registry.Get(ctx, installed.SkillID)
	if err != nil || registered.Definition.Enabled || registered.Definition.Source != SkillSourceWorkflow {
		t.Fatalf("installed skill state invalid: %#v %v", registered.Definition, err)
	}
	runtimeValidator, _ := NewSchemaValidator()
	runtimeService := NewService(registry, executor, NewRepository(db), runtimeValidator)
	if err := runtimeService.EnableSkill(ctx, scope, installed.SkillID); err != nil {
		t.Fatal(err)
	}
	enabledSession, err := service.GetSession(ctx, scope, session.ID)
	if err != nil || enabledSession.Status != WorkshopEnabled {
		t.Fatalf("workshop session did not track enable: %#v %v", enabledSession.WorkshopSession, err)
	}
	if err := db.Model(&workshopSessionRecord{}).Where("id = ?", session.ID).Updates(map[string]interface{}{"status": string(WorkshopInstalled), "lock_version": gorm.Expr("lock_version + 1")}).Error; err != nil {
		t.Fatal(err)
	}
	if err := runtimeService.EnableSkill(ctx, scope, installed.SkillID); err != nil {
		t.Fatal(err)
	}
	resyncedSession, err := service.GetSession(ctx, scope, session.ID)
	if err != nil || resyncedSession.Status != WorkshopEnabled {
		t.Fatalf("idempotent enable did not repair workshop status: %#v %v", resyncedSession.WorkshopSession, err)
	}
	result, err := executor.Execute(ctx, ExecuteSkillRequest{SkillID: installed.SkillID, Input: json.RawMessage(`{"name":"A"}`), Scope: scope})
	if err != nil || result.Status != RunSucceeded || !json.Valid(result.Output) {
		t.Fatalf("workflow execution failed: %#v %v", result, err)
	}
	if err := runtimeService.DisableSkill(ctx, scope, installed.SkillID); err != nil {
		t.Fatal(err)
	}
	disabledSession, err := service.GetSession(ctx, scope, session.ID)
	if err != nil || disabledSession.Status != WorkshopDisabled {
		t.Fatalf("workshop session did not track disable: %#v %v", disabledSession.WorkshopSession, err)
	}
	forked, err := service.ForkSkill(ctx, scope, installed.SkillID)
	if err != nil || forked.Revision == nil || forked.Revision.NormalizedDraft.Metadata.ID != installed.SkillID || forked.Revision.NormalizedDraft.Metadata.Version != "1.0.1" {
		t.Fatalf("update revision was not created correctly: %#v %v", forked, err)
	}
	updateValidation, err := service.Validate(ctx, scope, forked.ID, forked.CurrentRevision)
	if err != nil || !updateValidation.Valid {
		t.Fatalf("update validation failed: %#v %v", updateValidation, err)
	}
	updateConfirmation := PermissionConfirmation{WorkflowChecksum: updateValidation.WorkflowChecksum, Capabilities: updateValidation.Capabilities.Required, ConfirmedHighRisk: updateValidation.Capabilities.HighRisk}
	if err := service.ConfirmPermissions(ctx, scope, forked.ID, forked.CurrentRevision, updateConfirmation); err != nil {
		t.Fatal(err)
	}
	updateCase := WorkshopTestCase{ID: "update-dry", Name: "Update Dry Run", Mode: "dry_run", Input: json.RawMessage(`{"name":"A"}`), Config: json.RawMessage(`{}`)}
	if report, err := service.Test(ctx, scope, forked.ID, forked.CurrentRevision, WorkshopTestRequest{Scope: scope, Mode: "dry_run", TestCases: []WorkshopTestCase{updateCase}}); err != nil || report.Status != "passed" {
		t.Fatalf("update test failed: %#v %v", report, err)
	}
	updateConfirmation.Production = true
	if err := service.ConfirmPermissions(ctx, scope, forked.ID, forked.CurrentRevision, updateConfirmation); err != nil {
		t.Fatal(err)
	}
	updated, err := service.Install(ctx, scope, forked.ID, forked.CurrentRevision)
	if err != nil || updated.Version != "1.0.1" {
		t.Fatalf("update install failed: %#v %v", updated, err)
	}
	rolledBack, err := service.Rollback(ctx, scope, installed.SkillID, "1.0.0")
	if err != nil || rolledBack.Version != "1.0.0" {
		t.Fatalf("rollback failed: %#v %v", rolledBack, err)
	}
	current, err := registry.Get(ctx, installed.SkillID)
	if err != nil || current.Definition.Version != "1.0.0" {
		t.Fatalf("rollback did not switch registry: %#v %v", current.Definition, err)
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
	if _, err := restoredRegistry.Get(ctx, installed.SkillID); err != nil {
		t.Fatalf("installed workflow was not restored: %v", err)
	}
}

func TestWorkshopRevisionInvalidatesPermissionScopes(t *testing.T) {
	service, _, _, _ := newWorkshopIntegrationService(t)
	ctx := context.Background()
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", Trigger: TriggerManual}
	session, err := service.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "创建问候 Skill"})
	if err != nil {
		t.Fatal(err)
	}
	draft := integrationDraft()
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
	detail, err := service.GetSession(ctx, scope, session.ID)
	if err != nil || second.Revision != 2 || detail.TestPermissionConfirmed || detail.ProductionPermissionConfirmed || len(detail.TestReports) != 0 {
		t.Fatalf("new revision retained stale state: %#v %v", detail, err)
	}
}

func TestWorkshopRegistryFailureCompensatesDatabaseInstall(t *testing.T) {
	service, registry, _, db := newWorkshopIntegrationService(t)
	input := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`)
	output := json.RawMessage(`{"type":"object","additionalProperties":true}`)
	blocker, handler := testDefinition(t, "dev.user.model-name-collision", input, output, func(context.Context, ExecuteSkillRequest) (SkillResult, error) {
		return SkillResult{Status: RunSucceeded, Output: json.RawMessage(`{}`)}, nil
	})
	blocker.ModelName = "dev_user_user_a_greeting"
	if err := registry.Register(context.Background(), blocker, handler); err != nil {
		t.Fatal(err)
	}
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", ConversationID: "conv-a", Channel: "web", Trigger: TriggerManual, TraceID: "trace-registry-failure"}
	session, revision := prepareInstallableWorkshop(t, service, scope, integrationDraft())
	if _, err := service.Install(context.Background(), scope, session.ID, revision.Revision); err == nil || asExtensionError(err).Code != ErrWorkshopInstallFailed {
		t.Fatalf("expected registry installation failure: %v", err)
	}
	detail, err := service.GetSession(context.Background(), scope, session.ID)
	if err != nil || detail.Status != WorkshopTestPassed || detail.InstalledSkillID != "" || detail.InstalledVersion != "" {
		t.Fatalf("session compensation failed: %#v %v", detail.WorkshopSession, err)
	}
	for _, table := range []string{"extensions", "extension_versions", "extension_artifacts"} {
		var count int64
		query := db.Table(table)
		if table == "extensions" || table == "extension_versions" || table == "extension_artifacts" {
			query = query.Where("extension_id = ?", integrationDraft().Metadata.ID)
		}
		if err := query.Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("half-installed row remained in %s: %d %v", table, count, err)
		}
	}
	if _, err := registry.Get(context.Background(), integrationDraft().Metadata.ID); asExtensionError(err).Code != ErrSkillNotFound {
		t.Fatalf("failed skill remained registered: %v", err)
	}
	if _, err := registry.Get(context.Background(), blocker.ID); err != nil {
		t.Fatalf("unrelated registry entry was damaged: %v", err)
	}
}

func TestWorkshopDatabaseFailureLeavesRegistryUntouched(t *testing.T) {
	service, registry, _, db := newWorkshopIntegrationService(t)
	scope := ExecutionScope{UserID: "user-a", CharacterID: "char-a", ConversationID: "conv-a", Channel: "web", Trigger: TriggerManual, TraceID: "trace-database-failure"}
	session, revision := prepareInstallableWorkshop(t, service, scope, integrationDraft())
	if err := db.Migrator().DropTable(&extensionVersionRecord{}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Install(context.Background(), scope, session.ID, revision.Revision); err == nil || asExtensionError(err).Code != ErrWorkshopInstallFailed {
		t.Fatalf("expected database installation failure: %v", err)
	}
	detail, err := service.GetSession(context.Background(), scope, session.ID)
	if err != nil || detail.Status != WorkshopTestPassed || detail.InstalledSkillID != "" {
		t.Fatalf("database failure changed session: %#v %v", detail.WorkshopSession, err)
	}
	if _, err := registry.Get(context.Background(), integrationDraft().Metadata.ID); asExtensionError(err).Code != ErrSkillNotFound {
		t.Fatalf("database failure changed registry: %v", err)
	}
	for _, table := range []string{"extensions", "extension_artifacts"} {
		var count int64
		if err := db.Table(table).Where("extension_id = ?", integrationDraft().Metadata.ID).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("database failure left row in %s: %d %v", table, count, err)
		}
	}
}
