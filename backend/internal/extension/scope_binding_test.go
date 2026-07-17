package extension

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/migration"
)

func TestCharacterScopeBindingConfigAndGrantIsolation(t *testing.T) {
	ctx := context.Background()
	db, repository := testRepository(t)
	if err := (migration.Runner{DB: db, SkipBackup: true}).Apply([]migration.Migration{migration.ExtensionScopeBindingsMigration()}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE characters (id TEXT PRIMARY KEY)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO characters (id) VALUES ('char-a'), ('char-b')").Error; err != nil {
		t.Fatal(err)
	}
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	definition, handler := testDefinition(t, "dev.amitia.scope.test", json.RawMessage(`{"type":"object"}`), json.RawMessage(`{"type":"object"}`), func(_ context.Context, request ExecuteSkillRequest) (SkillResult, error) {
		return SkillResult{Status: RunSucceeded, Output: request.Config}, nil
	})
	definition.ConfigSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"value":{"type":"string"}}}`)
	definition.DefaultConfig = json.RawMessage(`{"value":"default"}`)
	syncDefinitionManifest(t, &definition)
	registry := NewRegistry("1.0.0", validator, repository)
	if err := registry.Register(ctx, definition, handler); err != nil {
		t.Fatal(err)
	}
	if err := registry.SetScopeEnabled(ctx, definition.ID, ExecutionScope{}, true); err != nil {
		t.Fatal(err)
	}
	if err := registry.SetScopeEnabled(ctx, definition.ID, ExecutionScope{CharacterID: "char-a"}, false); err != nil {
		t.Fatal(err)
	}
	a, err := registry.GetScoped(ctx, definition.ID, ExecutionScope{CharacterID: "char-a"})
	if err != nil || a.Definition.Enabled || a.Definition.EffectiveScopeType != ScopeCharacter {
		t.Fatalf("unexpected char-a binding: %+v %v", a.Definition, err)
	}
	b, err := registry.GetScoped(ctx, definition.ID, ExecutionScope{CharacterID: "char-b"})
	if err != nil || !b.Definition.Enabled || b.Definition.EffectiveScopeType != ScopeGlobal {
		t.Fatalf("unexpected char-b fallback: %+v %v", b.Definition, err)
	}
	if err := repository.UpdateConfig(ctx, definition.ID, PermissionScope{Type: ScopeGlobal}, json.RawMessage(`{"value":"global"}`)); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpdateConfig(ctx, definition.ID, PermissionScope{Type: ScopeCharacter, ID: "char-a"}, json.RawMessage(`{"value":"character"}`)); err != nil {
		t.Fatal(err)
	}
	configA, err := repository.GetEffectiveConfig(ctx, definition.ID, ExecutionScope{CharacterID: "char-a"}, definition.DefaultConfig)
	if err != nil || string(configA) != `{"value":"character"}` {
		t.Fatalf("unexpected char-a config: %s %v", configA, err)
	}
	configB, err := repository.GetEffectiveConfig(ctx, definition.ID, ExecutionScope{CharacterID: "char-b"}, definition.DefaultConfig)
	if err != nil || string(configB) != `{"value":"global"}` {
		t.Fatalf("unexpected char-b config: %s %v", configB, err)
	}
	if err := repository.ReplaceGrants(ctx, definition.ID, []PermissionGrantInput{{Capability: "runtime.character.read", Decision: DecisionAllowAlways, ScopeType: ScopeGlobal}, {Capability: "runtime.character.read", Decision: DecisionDeny, ScopeType: ScopeCharacter, ScopeID: "char-a"}}); err != nil {
		t.Fatal(err)
	}
	identity := ExtensionIdentity{ExtensionID: definition.ID, SkillID: definition.ID, Version: definition.Version}
	decisionA, foundA, err := repository.ResolveGrant(ctx, identity, "runtime.character.read", ExecutionScope{CharacterID: "char-a"}, false)
	if err != nil || !foundA || decisionA != DecisionDeny {
		t.Fatalf("unexpected char-a grant: %s %v %v", decisionA, foundA, err)
	}
	decisionB, foundB, err := repository.ResolveGrant(ctx, identity, "runtime.character.read", ExecutionScope{CharacterID: "char-b"}, false)
	if err != nil || !foundB || decisionB != DecisionAllowAlways {
		t.Fatalf("unexpected char-b grant: %s %v %v", decisionB, foundB, err)
	}
	service := NewService(registry, nil, repository, validator)
	if _, err := service.GetSkill(ctx, ExecutionScope{CharacterID: "missing-character"}, definition.ID); asExtensionError(err).Code != ErrSkillPermissionDenied {
		t.Fatalf("unknown character bypassed backend validation: %v", err)
	}
}

func TestOwnedScheduleAttributionAndCleanupRetryState(t *testing.T) {
	ctx := context.Background()
	db, repository := testRepository(t)
	if err := db.Exec("CREATE TABLE schedules (id TEXT PRIMARY KEY, title TEXT, status TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := (migration.Runner{DB: db, SkipBackup: true}).Apply([]migration.Migration{migration.ExtensionOwnedResourcesMigration(), migration.ExtensionScheduleSourceMigration()}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO schedules (id, title, status) VALUES ('schedule-1', 'test', 'pending'), ('user-schedule', 'user', 'pending'), ('other-extension', 'other', 'pending'), ('other-character', 'other role', 'pending')").Error; err != nil {
		t.Fatal(err)
	}
	scope := ExecutionScope{CharacterID: "char-a", ExtensionID: "dev.amitia.owned.test", ExtensionVersion: "1.0.0", RunID: "run-1"}
	if err := repository.RegisterOwnedSideEffects(ctx, scope, []SideEffectRecord{{Type: "schedule_create", TargetID: "schedule-1", Confirmed: true}}); err != nil {
		t.Fatal(err)
	}
	if err := repository.RegisterOwnedSideEffects(ctx, ExecutionScope{CharacterID: "char-a", ExtensionID: "dev.amitia.other", ExtensionVersion: "1.0.0", RunID: "run-2"}, []SideEffectRecord{{Type: "schedule_create", TargetID: "other-extension", Confirmed: true}}); err != nil {
		t.Fatal(err)
	}
	if err := repository.RegisterOwnedSideEffects(ctx, ExecutionScope{CharacterID: "char-b", ExtensionID: scope.ExtensionID, ExtensionVersion: "1.0.0", RunID: "run-3"}, []SideEffectRecord{{Type: "schedule_create", TargetID: "other-character", Confirmed: true}}); err != nil {
		t.Fatal(err)
	}
	var attribution struct {
		ExtensionID string `gorm:"column:source_extension_id"`
		RunID       string `gorm:"column:source_run_id"`
		ScopeID     string `gorm:"column:owner_scope_id"`
		SourceType  string `gorm:"column:source_type"`
	}
	if err := db.Table("schedules").Where("id = ?", "schedule-1").Take(&attribution).Error; err != nil || attribution.ExtensionID != scope.ExtensionID || attribution.RunID != scope.RunID || attribution.ScopeID != scope.CharacterID || attribution.SourceType != "extension" {
		t.Fatalf("unexpected schedule attribution: %+v %v", attribution, err)
	}
	if err := repository.CleanupOwnedResources(ctx, scope.ExtensionID, string(ScopeCharacter), scope.CharacterID); err != nil {
		t.Fatal(err)
	}
	var schedules int64
	if err := db.Table("schedules").Where("id = ?", "schedule-1").Count(&schedules).Error; err != nil || schedules != 0 {
		t.Fatalf("schedule was not cleaned: %d %v", schedules, err)
	}
	if err := db.Table("schedules").Where("id IN ?", []string{"user-schedule", "other-extension", "other-character"}).Count(&schedules).Error; err != nil || schedules != 3 {
		t.Fatalf("cleanup crossed ownership boundary: %d %v", schedules, err)
	}
	var resource ownedResourceRecord
	if err := db.Where("extension_id = ? AND resource_id = ?", scope.ExtensionID, "schedule-1").First(&resource).Error; err != nil || resource.Status != "cleaned" || resource.CleanupAttempts != 1 {
		t.Fatalf("unexpected cleanup state: %+v %v", resource, err)
	}
	if err := repository.CleanupOwnedResources(ctx, scope.ExtensionID, string(ScopeCharacter), scope.CharacterID); err != nil {
		t.Fatal(err)
	}
}
