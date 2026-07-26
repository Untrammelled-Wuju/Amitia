package update

import (
	"context"
	"testing"
	"time"
)

func makeSnapshot(version string) DefinitionSnapshot {
	return DefinitionSnapshot{
		ExtensionID:    "com.example/weather",
		Version:        version,
		ManifestHash:   "sha256:manifest-" + version,
		ContentTreeHash: "sha256:tree-" + version,
		PackageHash:    "sha256:pkg-" + version,
		PublisherID:    "com.example",
		Modules: []ModuleSnapshot{
			{ID: "main", Type: "runtime", EnabledDefault: true, RuntimeID: "rt-main"},
		},
		Contributions: []ContributionSnapshot{
			{ID: "get_weather", Type: "tool", RuntimeID: "rt-main", EntryName: "get_weather", RiskLevel: "low"},
		},
		Runtimes: []RuntimeSnapshot{
			{ID: "rt-main", Type: "javascript", Entry: "modules/main/dist/index.js", MaxMemoryMB: 256},
		},
		Permissions: []PermissionSnapshot{
			{ID: "network.http.request", Reason: "weather API"},
		},
		Platforms:     []string{"windows", "linux"},
		SignatureKeyID: "k1",
		TrustLevel:     "trusted",
		GeneratedAt:    time.Now().UTC(),
	}
}

func TestComputeDiffPatchUpdate(t *testing.T) {
	old := makeSnapshot("1.0.0")
	new := makeSnapshot("1.0.1")
	diff := ComputeDiff(old, new)
	if diff.UpdateType != UpdateTypePatch {
		t.Fatalf("expected patch, got %s", diff.UpdateType)
	}
	if diff.HasBreakingChanges {
		t.Fatal("expected no breaking changes for patch")
	}
	if diff.PublisherChanged {
		t.Fatal("expected no publisher change")
	}
}

func TestComputeDiffMajorUpdate(t *testing.T) {
	old := makeSnapshot("1.0.0")
	new := makeSnapshot("2.0.0")
	diff := ComputeDiff(old, new)
	if diff.UpdateType != UpdateTypeMajor {
		t.Fatalf("expected major, got %s", diff.UpdateType)
	}
}

func TestComputeDiffModuleRemoved(t *testing.T) {
	old := makeSnapshot("1.0.0")
	new := makeSnapshot("1.1.0")
	new.Modules = []ModuleSnapshot{}
	diff := ComputeDiff(old, new)
	if len(diff.ModulesRemoved) != 1 {
		t.Fatalf("expected 1 removed module, got %d", len(diff.ModulesRemoved))
	}
	if !diff.HasBreakingChanges {
		t.Fatal("expected breaking changes")
	}
}

func TestComputeDiffPermissionExpansion(t *testing.T) {
	old := makeSnapshot("1.0.0")
	new := makeSnapshot("1.1.0")
	new.Permissions = append(new.Permissions, PermissionSnapshot{ID: "fs.read", Reason: "config"})
	diff := ComputeDiff(old, new)
	if !diff.PermissionExpanded {
		t.Fatal("expected permission expansion")
	}
	if len(diff.PermissionsAdded) != 1 {
		t.Fatalf("expected 1 added permission, got %d", len(diff.PermissionsAdded))
	}
}

func TestComputeDiffPublisherChange(t *testing.T) {
	old := makeSnapshot("1.0.0")
	new := makeSnapshot("1.1.0")
	new.PublisherID = "com.new"
	diff := ComputeDiff(old, new)
	if !diff.PublisherChanged {
		t.Fatal("expected publisher change")
	}
	if diff.OldPublisherID != "com.example" || diff.NewPublisherID != "com.new" {
		t.Fatalf("publisher mismatch: %s -> %s", diff.OldPublisherID, diff.NewPublisherID)
	}
}

func TestComputeDiffContributionSchemaChange(t *testing.T) {
	old := makeSnapshot("1.0.0")
	new := makeSnapshot("1.1.0")
	new.Contributions[0].InputSchema = `{"type":"object","properties":{"city":{"type":"string"}}}`
	old.Contributions[0].InputSchema = `{"type":"object","properties":{"city":{"type":"string"},"country":{"type":"string"}}}`
	diff := ComputeDiff(old, new)
	if !diff.HasBreakingChanges {
		t.Fatal("expected breaking changes due to schema change")
	}
}

func TestClassifyRiskNone(t *testing.T) {
	old := makeSnapshot("1.0.0")
	new := makeSnapshot("1.0.1")
	diff := ComputeDiff(old, new)
	risk := ClassifyRisk(diff)
	if risk.Level != RiskNone {
		t.Fatalf("expected none risk, got %s", risk.Level)
	}
}

func TestClassifyRiskCritical(t *testing.T) {
	old := makeSnapshot("1.0.0")
	new := makeSnapshot("2.0.0")
	new.PublisherID = "com.new"
	diff := ComputeDiff(old, new)
	risk := ClassifyRisk(diff)
	if risk.Level != RiskCritical {
		t.Fatalf("expected critical risk, got %s", risk.Level)
	}
	if !risk.HasOwnershipTransfer {
		t.Fatal("expected ownership transfer")
	}
}

func TestDetermineRollbackLevel(t *testing.T) {
	old := makeSnapshot("1.0.0")
	new := makeSnapshot("1.1.0")
	diff := ComputeDiff(old, new)
	level := DetermineRollbackLevel(diff, []MigrationSnapshot{})
	if level != RollbackLevelFull {
		t.Fatalf("expected full rollback, got %s", level)
	}

	new.PublisherID = "com.new"
	diff = ComputeDiff(old, new)
	level = DetermineRollbackLevel(diff, []MigrationSnapshot{})
	if level != RollbackLevelDataSnapshotRequired {
		t.Fatalf("expected data snapshot required, got %s", level)
	}
}

func TestBuildPlan(t *testing.T) {
	old := makeSnapshot("1.0.0")
	new := makeSnapshot("1.1.0")
	plan := BuildPlan("plan-1", old, new, []MigrationSnapshot{})
	if plan.PlanID != "plan-1" {
		t.Fatalf("expected plan-1, got %s", plan.PlanID)
	}
	if plan.OldVersion != "1.0.0" || plan.NewVersion != "1.1.0" {
		t.Fatalf("version mismatch: %s -> %s", plan.OldVersion, plan.NewVersion)
	}
	if plan.SwitchStrategy != SwitchStopThenStart {
		t.Fatalf("expected stop_then_start, got %s", plan.SwitchStrategy)
	}
	if !plan.AutoUpdateEligible {
		t.Fatal("expected auto update eligible")
	}
}

func TestBuildPlanRequiresConfirmation(t *testing.T) {
	old := makeSnapshot("1.0.0")
	new := makeSnapshot("1.1.0")
	new.PublisherID = "com.new"
	plan := BuildPlan("plan-1", old, new, []MigrationSnapshot{})
	if !plan.RequiresUserConfirm {
		t.Fatal("expected user confirmation required")
	}
	if plan.AutoUpdateEligible {
		t.Fatal("expected not auto update eligible")
	}
}

func TestGenerationManagerPrepareAndActivate(t *testing.T) {
	mgr := NewGenerationManager()
	ctx := context.Background()
	gen := mgr.Prepare(ctx, "ext-1", "1.0.0", "hash-1")
	if gen.State != GenerationStatePreparing {
		t.Fatalf("expected preparing, got %s", gen.State)
	}
	if err := mgr.Transition(ctx, "ext-1", gen.GenerationID, GenerationStateValidated); err != nil {
		t.Fatalf("transition to validated: %v", err)
	}
	if err := mgr.Transition(ctx, "ext-1", gen.GenerationID, GenerationStateRuntimeReady); err != nil {
		t.Fatalf("transition to runtime ready: %v", err)
	}
	if err := mgr.Transition(ctx, "ext-1", gen.GenerationID, GenerationStateActive); err != nil {
		t.Fatalf("transition to active: %v", err)
	}
	active := mgr.Active(ctx, "ext-1")
	if active == nil || active.GenerationID != gen.GenerationID {
		t.Fatal("expected active generation")
	}
}

func TestGenerationManagerInvalidTransition(t *testing.T) {
	mgr := NewGenerationManager()
	ctx := context.Background()
	gen := mgr.Prepare(ctx, "ext-1", "1.0.0", "hash-1")
	err := mgr.Transition(ctx, "ext-1", gen.GenerationID, GenerationStateActive)
	if err == nil {
		t.Fatal("expected invalid transition error")
	}
}

func TestGenerationManagerCanRollback(t *testing.T) {
	mgr := NewGenerationManager()
	ctx := context.Background()
	gen1 := mgr.Prepare(ctx, "ext-1", "1.0.0", "hash-1")
	mgr.Transition(ctx, "ext-1", gen1.GenerationID, GenerationStateValidated)
	mgr.Transition(ctx, "ext-1", gen1.GenerationID, GenerationStateRuntimeReady)
	mgr.Transition(ctx, "ext-1", gen1.GenerationID, GenerationStateActive)

	gen2 := mgr.Prepare(ctx, "ext-1", "1.1.0", "hash-2")
	mgr.Transition(ctx, "ext-1", gen2.GenerationID, GenerationStateValidated)
	mgr.Transition(ctx, "ext-1", gen2.GenerationID, GenerationStateRuntimeReady)
	mgr.Transition(ctx, "ext-1", gen2.GenerationID, GenerationStateActive)

	mgr.Transition(ctx, "ext-1", gen1.GenerationID, GenerationStateStopped)

	if !mgr.CanRollback(ctx, "ext-1") {
		t.Fatal("expected rollback possible")
	}
	target, err := mgr.RollbackTarget(ctx, "ext-1")
	if err != nil {
		t.Fatalf("rollback target: %v", err)
	}
	if target.GenerationID != gen1.GenerationID {
		t.Fatal("expected gen1 as rollback target")
	}
}

func TestMigrationExecutorSnapshotAndRestore(t *testing.T) {
	exec := NewMigrationExecutor()
	ctx := context.Background()
	data := map[string][]byte{
		"settings": []byte(`{"key":"value"}`),
	}
	snap := exec.SnapshotNamespaces(ctx, "ext-1", []string{"settings"}, data)
	if snap.SnapshotID == "" {
		t.Fatal("expected snapshot id")
	}
	if snap.DataHash == "" {
		t.Fatal("expected data hash")
	}
	restored, err := exec.RestoreSnapshot(ctx, snap.SnapshotID)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if string(restored["settings"]) != `{"key":"value"}` {
		t.Fatalf("expected restored data, got %s", restored["settings"])
	}
}

func TestMigrationExecutorAcquireLock(t *testing.T) {
	exec := NewMigrationExecutor()
	ctx := context.Background()
	if err := exec.AcquireLock(ctx, "ext-1"); err != nil {
		t.Fatalf("acquire lock: %v", err)
	}
	if err := exec.AcquireLock(ctx, "ext-1"); err == nil {
		t.Fatal("expected lock already held error")
	}
	exec.ReleaseLock(ctx, "ext-1")
	if err := exec.AcquireLock(ctx, "ext-1"); err != nil {
		t.Fatalf("re-acquire lock: %v", err)
	}
}

func TestMigrationExecutorExecuteMigration(t *testing.T) {
	exec := NewMigrationExecutor()
	ctx := context.Background()
	plan := MigrationPlan{
		MigrationID: "m1",
		FromRange:   "<1.1.0",
		ToRange:     ">=1.1.0",
		RuntimeType: "task",
		Entry:       "migrations/m1.js",
		Reversible:  true,
	}
	called := false
	run, err := exec.ExecuteMigration(ctx, plan, "ext-1", "1.0.0", "1.1.0", "", func(ctx context.Context) (string, error) {
		called = true
		return "sha256:output", nil
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !called {
		t.Fatal("expected handler called")
	}
	if run.Status != MigrationStatusSucceeded {
		t.Fatalf("expected succeeded, got %s", run.Status)
	}
	if run.OutputHash != "sha256:output" {
		t.Fatalf("expected output hash, got %s", run.OutputHash)
	}
}

func TestMigrationExecutorSkipsIdempotent(t *testing.T) {
	exec := NewMigrationExecutor()
	ctx := context.Background()
	plan := MigrationPlan{
		MigrationID: "m1",
		FromRange:   "<1.1.0",
		ToRange:     ">=1.1.0",
		RuntimeType: "task",
		Entry:       "migrations/m1.js",
		Reversible:  true,
	}
	callCount := 0
	handler := func(ctx context.Context) (string, error) {
		callCount++
		return "sha256:output", nil
	}
	exec.ExecuteMigration(ctx, plan, "ext-1", "1.0.0", "1.1.0", "", handler)
	exec.ExecuteMigration(ctx, plan, "ext-1", "1.0.0", "1.1.0", "", handler)
	if callCount != 1 {
		t.Fatalf("expected handler called once, got %d", callCount)
	}
}

func TestRollbackPointStoreSaveAndGet(t *testing.T) {
	store := NewRollbackPointStore()
	ctx := context.Background()
	point := RollbackPoint{
		PointID:       "rb-1",
		ExtensionID:   "ext-1",
		Version:       "1.0.0",
		GenerationID:  "gen-1",
		DefinitionHash: "hash-1",
		CreatedAt:     time.Now().UTC(),
	}
	if err := store.Save(point); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := store.Get(ctx, "rb-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Version != "1.0.0" {
		t.Fatalf("expected version 1.0.0, got %s", got.Version)
	}
}

func TestRollbackPointStoreCannotDeletePinned(t *testing.T) {
	store := NewRollbackPointStore()
	ctx := context.Background()
	point := RollbackPoint{
		PointID:     "rb-1",
		ExtensionID: "ext-1",
		Version:     "1.0.0",
		CreatedAt:   time.Now().UTC(),
	}
	store.Save(point)
	store.Pin(ctx, "rb-1")
	if err := store.Delete(ctx, "rb-1"); err == nil {
		t.Fatal("expected error deleting pinned point")
	}
	store.Unpin(ctx, "rb-1")
	if err := store.Delete(ctx, "rb-1"); err != nil {
		t.Fatalf("delete after unpin: %v", err)
	}
}

func TestRetentionPolicyKeepsLastN(t *testing.T) {
	store := NewRollbackPointStore()
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		store.Save(RollbackPoint{
			PointID:     "rb-" + string(rune('1'+i)),
			ExtensionID: "ext-1",
			Version:     "1.0.0",
			CreatedAt:   time.Now().Add(time.Duration(i) * time.Minute),
		})
	}
	policy := RetentionPolicy{
		KeepLastN: 2,
		KeepCurrent: true,
		MaxPoints: 3,
	}
	deleted := store.ApplyRetentionPolicy(ctx, "ext-1", policy)
	if len(deleted) < 1 {
		t.Fatalf("expected at least 1 deleted, got %d", len(deleted))
	}
	remaining := store.List(ctx, "ext-1")
	if len(remaining) > 3 {
		t.Fatalf("expected at most 3 remaining, got %d", len(remaining))
	}
}

func TestConflictRegistryDetectConflicts(t *testing.T) {
	registry := NewConflictRegistry()
	registry.RegisterAsset(UserAsset{
		AssetID:    "asset-1",
		ExtensionID: "ext-1",
		AssetType:  UserAssetForkWorkflow,
		ResourceID: "workflow/daily",
		Hash:       "sha256:old",
	})
	newAssets := []UserAsset{
		{
			AssetID:    "asset-1-new",
			ExtensionID: "ext-1",
			AssetType:  UserAssetForkWorkflow,
			ResourceID: "workflow/daily",
			Hash:       "sha256:new",
		},
	}
	conflicts := registry.DetectConflicts("ext-1", newAssets)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if !registry.HasUnresolvedConflicts("ext-1") {
		t.Fatal("expected unresolved conflicts")
	}
	registry.ResolveConflict(conflicts[0].ConflictID, ConflictKeepUser)
	if registry.HasUnresolvedConflicts("ext-1") {
		t.Fatal("expected no unresolved conflicts after resolution")
	}
}

func TestUpdateExecutorCreatePlan(t *testing.T) {
	exec := NewUpdateExecutor()
	ctx := context.Background()
	old := makeSnapshot("1.0.0")
	new := makeSnapshot("1.1.0")
	plan := exec.CreatePlan(ctx, "plan-1", old, new, []MigrationSnapshot{})
	if plan.PlanID != "plan-1" {
		t.Fatalf("expected plan-1, got %s", plan.PlanID)
	}
	got, err := exec.GetPlan(ctx, "plan-1")
	if err != nil {
		t.Fatalf("get plan: %v", err)
	}
	if got.NewVersion != "1.1.0" {
		t.Fatalf("expected version 1.1.0, got %s", got.NewVersion)
	}
}

func TestUpdateExecutorExecuteSuccess(t *testing.T) {
	exec := NewUpdateExecutor()
	ctx := context.Background()

	old := makeSnapshot("1.0.0")
	exec.CreatePlan(ctx, "plan-1", old, makeSnapshot("1.1.0"), []MigrationSnapshot{})

	activateGen := exec.Generations().Prepare(ctx, "com.example/weather", "1.0.0", "old-hash")
	exec.Generations().Transition(ctx, "com.example/weather", activateGen.GenerationID, GenerationStateValidated)
	exec.Generations().Transition(ctx, "com.example/weather", activateGen.GenerationID, GenerationStateRuntimeReady)
	exec.Generations().Transition(ctx, "com.example/weather", activateGen.GenerationID, GenerationStateActive)

	result := exec.Execute(ctx, ExecuteRequest{
		PlanID:        "plan-1",
		ExtensionID:   "com.example/weather",
		UserConfirmed: true,
	})
	if !result.Success {
		t.Fatalf("expected success, got reason: %s", result.Reason)
	}
	if result.NewGenerationID == "" {
		t.Fatal("expected new generation id")
	}
	if result.RollbackPointID == "" {
		t.Fatal("expected rollback point id")
	}
	active := exec.Generations().Active(ctx, "com.example/weather")
	if active == nil || active.Version != "1.1.0" {
		t.Fatal("expected active version 1.1.0")
	}
}

func TestUpdateExecutorRequiresConfirmation(t *testing.T) {
	exec := NewUpdateExecutor()
	ctx := context.Background()
	old := makeSnapshot("1.0.0")
	new := makeSnapshot("1.1.0")
	new.PublisherID = "com.new"
	exec.CreatePlan(ctx, "plan-1", old, new, []MigrationSnapshot{})
	result := exec.Execute(ctx, ExecuteRequest{
		PlanID:        "plan-1",
		ExtensionID:   "com.example/weather",
		UserConfirmed: false,
	})
	if result.Success {
		t.Fatal("expected failure without confirmation")
	}
}
