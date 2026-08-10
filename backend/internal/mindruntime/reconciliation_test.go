package mindruntime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestReconciliationEngineStartScan(t *testing.T) {
	config := DefaultReconciliationConfig()
	engine := NewReconciliationEngine(config)
	if engine.Status() != ReconciliationStatusIdle {
		t.Fatalf("expected idle, got %s", engine.Status())
	}
	scan := engine.StartScan(ReconciliationSQLiteQdrant, StrategyAutoRebuild, "")
	if scan == nil {
		t.Fatal("expected non-nil scan")
	}
	if scan.Status != ReconciliationStatusRunning {
		t.Fatalf("expected running, got %s", scan.Status)
	}
	if scan.Target != ReconciliationSQLiteQdrant {
		t.Fatalf("expected sqlite_qdrant, got %s", scan.Target)
	}
	if engine.Status() != ReconciliationStatusRunning {
		t.Fatalf("expected running, got %s", engine.Status())
	}
	all := engine.AllScans()
	if len(all) != 1 {
		t.Fatalf("expected 1 scan, got %d", len(all))
	}
}

func TestReconciliationEngineProgress(t *testing.T) {
	config := DefaultReconciliationConfig()
	config.BatchSize = 50
	config.BudgetLimitMS = 1000
	engine := NewReconciliationEngine(config)
	scan := engine.StartScan(ReconciliationOutboxSideEffect, StrategyCompensate, "cursor-001")
	engine.UpdateScanProgress(scan.ID, 50, 3, 2, 1, 100)
	retrieved := engine.GetScan(scan.ID)
	if retrieved == nil {
		t.Fatal("expected non-nil scan")
	}
	if retrieved.TotalScanned != 50 {
		t.Fatalf("expected 50 scanned, got %d", retrieved.TotalScanned)
	}
	if retrieved.DiffsFound != 3 {
		t.Fatalf("expected 3 diffs, got %d", retrieved.DiffsFound)
	}
	if retrieved.DiffsRepaired != 2 {
		t.Fatalf("expected 2 repaired, got %d", retrieved.DiffsRepaired)
	}
	if retrieved.DiffsSkipped != 1 {
		t.Fatalf("expected 1 skipped, got %d", retrieved.DiffsSkipped)
	}
	if retrieved.BudgetUsedMS != 100 {
		t.Fatalf("expected 100ms budget, got %d", retrieved.BudgetUsedMS)
	}
}

func TestReconciliationEngineCursor(t *testing.T) {
	config := DefaultReconciliationConfig()
	engine := NewReconciliationEngine(config)
	scan := engine.StartScan(ReconciliationLeaseDelivery, StrategyReleaseLease, "cursor-000")
	engine.UpdateCursor(scan.ID, "cursor-050")
	engine.UpdateScanProgress(scan.ID, 50, 0, 0, 0, 50)
	retrieved := engine.GetScan(scan.ID)
	if retrieved.CursorID != "cursor-050" {
		t.Fatalf("expected cursor-050, got %s", retrieved.CursorID)
	}
}

func TestReconciliationEnginePauseResumeCancel(t *testing.T) {
	config := DefaultReconciliationConfig()
	engine := NewReconciliationEngine(config)
	scan := engine.StartScan(ReconciliationTombstoneDerivedData, StrategyLogicalInvalid, "")
	engine.PauseScan(scan.ID)
	retrieved := engine.GetScan(scan.ID)
	if retrieved.Status != ReconciliationStatusPaused {
		t.Fatalf("expected paused, got %s", retrieved.Status)
	}
	engine.ResumeScan(scan.ID)
	retrieved = engine.GetScan(scan.ID)
	if retrieved.Status != ReconciliationStatusRunning {
		t.Fatalf("expected running, got %s", retrieved.Status)
	}
	engine.CancelScan(scan.ID)
	retrieved = engine.GetScan(scan.ID)
	if retrieved.Status != ReconciliationStatusCancelled {
		t.Fatalf("expected cancelled, got %s", retrieved.Status)
	}
	if retrieved.EndedAt.IsZero() {
		t.Fatal("expected endedAt set for cancelled")
	}
}

func TestReconciliationEngineComplete(t *testing.T) {
	config := DefaultReconciliationConfig()
	engine := NewReconciliationEngine(config)
	scan := engine.StartScan(ReconciliationSQLiteSurrealDB, StrategyReindex, "")
	engine.UpdateScanProgress(scan.ID, 100, 0, 0, 0, 200)
	engine.CompleteScan(scan.ID)
	retrieved := engine.GetScan(scan.ID)
	if retrieved.Status != ReconciliationStatusCompleted {
		t.Fatalf("expected completed, got %s", retrieved.Status)
	}
	if retrieved.EndedAt.Before(retrieved.StartedAt) {
		t.Fatal("expected endedAt not before startedAt")
	}
	if engine.Status() != ReconciliationStatusIdle {
		t.Fatalf("expected idle after completion, got %s", engine.Status())
	}
}

func TestReconciliationEngineBudgetExhausted(t *testing.T) {
	config := DefaultReconciliationConfig()
	config.BudgetLimitMS = 500
	engine := NewReconciliationEngine(config)
	scan := engine.StartScan(ReconciliationInteractionRunMsg, StrategyRetry, "")
	if engine.IsBudgetExhausted(scan.ID) {
		t.Fatal("budget should not be exhausted initially")
	}
	engine.UpdateScanProgress(scan.ID, 100, 5, 3, 2, 500)
	if !engine.IsBudgetExhausted(scan.ID) {
		t.Fatal("budget should be exhausted")
	}
}

func TestReconciliationEngineUnlimitedBudget(t *testing.T) {
	config := DefaultReconciliationConfig()
	config.BudgetLimitMS = 0
	engine := NewReconciliationEngine(config)
	scan := engine.StartScan(ReconciliationSQLiteQdrant, StrategyAutoRebuild, "")
	engine.UpdateScanProgress(scan.ID, 99999, 0, 0, 0, 99999)
	if engine.IsBudgetExhausted(scan.ID) {
		t.Fatal("unlimited budget should never be exhausted")
	}
}

func TestReconciliationEngineAddDiff(t *testing.T) {
	config := DefaultReconciliationConfig()
	engine := NewReconciliationEngine(config)
	scan := engine.StartScan(ReconciliationOutboxSideEffect, StrategyCompensate, "")
	diff1 := ReconciliationDiff{
		ID:             "diff-001",
		ScanID:         scan.ID,
		Source:         "sqlite",
		Target:         "qdrant",
		DiffType:       "missing",
		SourceKey:      "doc-001",
		TargetKey:      "qt-001",
		Description:    "Document exists in SQLite but missing from Qdrant",
		Severity:       "warning",
		AutoRepairable: true,
		RepairAction:   "reindex",
		FoundAt:        time.Now().UTC(),
	}
	diff2 := ReconciliationDiff{
		ID:             "diff-002",
		ScanID:         scan.ID,
		Source:         "sqlite",
		Target:         "surrealdb",
		DiffType:       "stale",
		SourceKey:      "rec-002",
		TargetKey:      "sr-002",
		Description:    "Record version mismatch",
		Severity:       "critical",
		AutoRepairable: false,
		FoundAt:        time.Now().UTC(),
	}
	engine.AddDiff(scan.ID, diff1)
	engine.AddDiff(scan.ID, diff2)
	engine.UpdateScanProgress(scan.ID, 0, 2, 0, 0, 0)
	retrieved := engine.GetScan(scan.ID)
	if len(retrieved.Diffs) != 2 {
		t.Fatalf("expected 2 diffs, got %d", len(retrieved.Diffs))
	}
	if retrieved.DiffsFound != 2 {
		t.Fatalf("expected 2 diffs found, got %d", retrieved.DiffsFound)
	}
}

func TestReconciliationEngineRunScanUsesRegisteredChecker(t *testing.T) {
	config := DefaultReconciliationConfig()
	engine := NewReconciliationEngine(config)
	engine.RegisterChecker(ReconciliationTombstoneDerivedData, ReconciliationCheckerFunc(func(ctx context.Context, req ReconciliationCheckRequest) ([]ReconciliationDiff, error) {
		if req.Target != ReconciliationTombstoneDerivedData {
			t.Fatalf("unexpected target %s", req.Target)
		}
		if req.BatchSize != config.BatchSize {
			t.Fatalf("unexpected batch size %d", req.BatchSize)
		}
		return []ReconciliationDiff{
			{
				Source:         "deletion_tombstones",
				Target:         "data_lifecycle_outbox_cleanup_items",
				DiffType:       "missing_derived_cleanup",
				SourceKey:      "tombstone-1",
				TargetKey:      "outbox-tombstone-1",
				Description:    "tombstone has no derived cleanup item",
				Severity:       "critical",
				AutoRepairable: true,
				RepairAction:   string(StrategyLogicalInvalid),
			},
		}, nil
	}))

	scan, err := engine.RunScan(context.Background(), ReconciliationTombstoneDerivedData, StrategyLogicalInvalid, "")
	if err != nil {
		t.Fatal(err)
	}
	if scan.Status != ReconciliationStatusCompleted {
		t.Fatalf("expected completed, got %s", scan.Status)
	}
	if scan.TotalScanned != 1 || scan.DiffsFound != 1 {
		t.Fatalf("expected one scanned diff, got scanned=%d diffs=%d", scan.TotalScanned, scan.DiffsFound)
	}
	if len(scan.Diffs) != 1 {
		t.Fatalf("expected one diff, got %d", len(scan.Diffs))
	}
	if scan.Diffs[0].ScanID != scan.ID || scan.Diffs[0].ID == "" || scan.Diffs[0].FoundAt.IsZero() {
		t.Fatalf("diff metadata was not populated: %#v", scan.Diffs[0])
	}
	if engine.Status() != ReconciliationStatusIdle {
		t.Fatalf("expected idle after run, got %s", engine.Status())
	}
}

func TestReconciliationEngineRunScanRecordsCheckerErrors(t *testing.T) {
	engine := NewReconciliationEngine(DefaultReconciliationConfig())
	wantErr := errors.New("source unavailable")
	engine.RegisterChecker(ReconciliationOutboxSideEffect, ReconciliationCheckerFunc(func(ctx context.Context, req ReconciliationCheckRequest) ([]ReconciliationDiff, error) {
		return nil, wantErr
	}))

	scan, err := engine.RunScan(context.Background(), ReconciliationOutboxSideEffect, StrategyCompensate, "")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected checker error, got %v", err)
	}
	if scan.Status != ReconciliationStatusCancelled {
		t.Fatalf("expected cancelled failed scan, got %s", scan.Status)
	}
	if scan.DiffsSkipped != 1 || len(scan.Diffs) != 1 {
		t.Fatalf("expected recorded error diff, skipped=%d diffs=%d", scan.DiffsSkipped, len(scan.Diffs))
	}
	if scan.Diffs[0].DiffType != "scan_error" || scan.Diffs[0].Description != wantErr.Error() {
		t.Fatalf("unexpected error diff: %#v", scan.Diffs[0])
	}
}

func TestReconciliationEngineRunScanRequiresChecker(t *testing.T) {
	engine := NewReconciliationEngine(DefaultReconciliationConfig())
	scan, err := engine.RunScan(context.Background(), ReconciliationLeaseDelivery, StrategyReleaseLease, "")
	if err == nil {
		t.Fatal("expected missing checker error")
	}
	if scan.Status != ReconciliationStatusCancelled {
		t.Fatalf("expected cancelled scan without checker, got %s", scan.Status)
	}
}

func TestRuntimeReconciliationCheckerDetectsRealStateDiffs(t *testing.T) {
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)
	req := ReconciliationCheckRequest{
		Target:   ReconciliationSQLiteQdrant,
		Strategy: StrategyReindex,
	}
	source := ReconciliationStateSourceFunc(func(ctx context.Context, req ReconciliationCheckRequest) ([]ReconciliationEntity, error) {
		return []ReconciliationEntity{
			{Store: "sqlite", Kind: "memory", Key: "missing", Version: "1", Hash: "a", Status: "committed"},
			{Store: "sqlite", Kind: "memory", Key: "changed", Version: "2", Hash: "b", Status: "committed", References: map[string]string{"run": "run-1"}},
			{Store: "sqlite", Kind: "memory", Key: "deleted", Deleted: true, Status: "completed"},
			{Store: "sqlite", Kind: "outbox", Key: "lease", Status: "processing", LeasedUntil: now.Add(-time.Minute)},
		}, nil
	})
	target := ReconciliationStateSourceFunc(func(ctx context.Context, req ReconciliationCheckRequest) ([]ReconciliationEntity, error) {
		return []ReconciliationEntity{
			{Store: "qdrant", Kind: "memory", Key: "changed", Version: "3", Hash: "c", Status: "indexed", References: map[string]string{"run": "run-2"}},
			{Store: "qdrant", Kind: "memory", Key: "deleted", Deleted: false, Status: "indexed"},
			{Store: "qdrant", Kind: "memory", Key: "orphan", Version: "1", Hash: "z", Status: "indexed"},
			{Store: "dispatcher", Kind: "outbox", Key: "lease", Status: "processing", LeasedUntil: now.Add(-time.Second)},
		}, nil
	})
	checker := NewRuntimeReconciliationChecker(source, target)
	checker.Now = func() time.Time { return now }

	diffs, err := checker.CheckReconciliation(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	assertDiffTypes(t, diffs, []string{
		"expired_source_lease",
		"expired_target_lease",
		"hash_mismatch",
		"missing_target",
		"orphan_target",
		"reference_mismatch",
		"status_mismatch",
		"status_mismatch",
		"tombstone_target_present",
		"version_mismatch",
	})
	for _, diff := range diffs {
		if diff.FoundAt.IsZero() {
			t.Fatalf("expected found time on diff %#v", diff)
		}
		if diff.Description == "" {
			t.Fatalf("expected description on diff %#v", diff)
		}
	}
}

func TestRuntimeReconciliationCheckerPropagatesSourceErrors(t *testing.T) {
	wantErr := errors.New("sqlite unavailable")
	checker := NewRuntimeReconciliationChecker(
		ReconciliationStateSourceFunc(func(ctx context.Context, req ReconciliationCheckRequest) ([]ReconciliationEntity, error) {
			return nil, wantErr
		}),
		ReconciliationStateSourceFunc(func(ctx context.Context, req ReconciliationCheckRequest) ([]ReconciliationEntity, error) {
			return nil, nil
		}),
	)
	_, err := checker.CheckReconciliation(context.Background(), ReconciliationCheckRequest{Target: ReconciliationSQLiteQdrant})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected source error, got %v", err)
	}
}

func TestGormReconciliationSourceScansSQLiteRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&DeletionTombstoneModel{}, &OutboxCleanupItemModel{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)
	cleanedAt := now.Add(-time.Minute)
	if err := db.Create(&DeletionTombstoneModel{
		ID:               "tombstone-1",
		TargetID:         "target-1",
		TargetType:       "memory",
		Scope:            string(DeletionScopeAll),
		Status:           string(DeletionStatusCompleted),
		ItemsCount:       6,
		CleanedCount:     6,
		FailedCount:      0,
		RequestedAt:      now.Add(-time.Hour),
		BlockedUntil:     now.Add(-time.Hour),
		CompletedAt:      &cleanedAt,
		RetrievalBlocked: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&OutboxCleanupItemModel{
		ID:         "outbox-1",
		Storage:    "qdrant",
		TargetID:   "target-1",
		TargetKind: "memory",
		Status:     "completed",
		Attempts:   1,
		CleanedAt:  &cleanedAt,
	}).Error; err != nil {
		t.Fatal(err)
	}

	source := GormReconciliationSource{
		DB:    db,
		Store: "sqlite",
		Tables: []GormReconciliationTable{
			{
				Table:         "deletion_tombstones",
				Kind:          "tombstone",
				KeyColumns:    []string{"target_id"},
				StatusColumn:  "status",
				DeletedColumn: "retrieval_blocked",
				HashColumns:   []string{"status", "items_count", "cleaned_count", "failed_count"},
				FieldColumns:  []string{"target_type", "scope", "status"},
			},
			{
				Table:        "data_lifecycle_outbox_cleanup_items",
				Kind:         "cleanup",
				KeyColumns:   []string{"target_id", "storage"},
				StatusColumn: "status",
				HashColumns:  []string{"status", "attempts"},
				FieldColumns: []string{"target_kind", "storage", "status"},
				ReferenceColumns: map[string]string{
					"target": "target_id",
				},
			},
		},
	}
	entities, err := source.ListReconciliationEntities(context.Background(), ReconciliationCheckRequest{BatchSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 2 {
		t.Fatalf("expected 2 scanned entities, got %d", len(entities))
	}
	if entities[0].Store != "sqlite" || entities[0].Key == "" || entities[0].Hash == "" {
		t.Fatalf("unexpected first entity: %#v", entities[0])
	}
	var cleanup ReconciliationEntity
	for _, entity := range entities {
		if entity.Kind == "cleanup" {
			cleanup = entity
			break
		}
	}
	if cleanup.References["target"] != "target-1" {
		t.Fatalf("expected scanned reference, got %#v", cleanup.References)
	}
}

func TestRunScanWithRuntimeCheckerUpdatesRealDiffCounts(t *testing.T) {
	engine := NewReconciliationEngine(DefaultReconciliationConfig())
	engine.RegisterChecker(ReconciliationInteractionRunMsg, NewRuntimeReconciliationChecker(
		ReconciliationStateSourceFunc(func(ctx context.Context, req ReconciliationCheckRequest) ([]ReconciliationEntity, error) {
			return []ReconciliationEntity{{Store: "interaction_runs", Kind: "run", Key: "run-1", Status: "committed", Version: "4"}}, nil
		}),
		ReconciliationStateSourceFunc(func(ctx context.Context, req ReconciliationCheckRequest) ([]ReconciliationEntity, error) {
			return []ReconciliationEntity{{Store: "messages", Kind: "run", Key: "run-1", Status: "persisted", Version: "3"}}, nil
		}),
	))

	scan, err := engine.RunScan(context.Background(), ReconciliationInteractionRunMsg, StrategyCompensate, "")
	if err != nil {
		t.Fatal(err)
	}
	if scan.Status != ReconciliationStatusCompleted {
		t.Fatalf("expected completed, got %s", scan.Status)
	}
	if scan.DiffsFound != 2 || len(scan.Diffs) != 2 {
		t.Fatalf("expected two real diffs, found=%d len=%d %#v", scan.DiffsFound, len(scan.Diffs), scan.Diffs)
	}
	assertDiffTypes(t, scan.Diffs, []string{"status_mismatch", "version_mismatch"})
}

func TestReconciliationEngineMultipleScans(t *testing.T) {
	config := DefaultReconciliationConfig()
	engine := NewReconciliationEngine(config)
	targets := []ReconciliationTarget{
		ReconciliationSQLiteQdrant,
		ReconciliationSQLiteSurrealDB,
		ReconciliationInteractionRunMsg,
		ReconciliationOutboxSideEffect,
		ReconciliationLeaseDelivery,
		ReconciliationTombstoneDerivedData,
	}
	for i, target := range targets {
		scan := engine.StartScan(target, StrategyAutoRebuild, "")
		engine.UpdateScanProgress(scan.ID, int64((i+1)*10), 0, 0, 0, 0)
		engine.CompleteScan(scan.ID)
	}
	all := engine.AllScans()
	if len(all) != len(targets) {
		t.Fatalf("expected %d scans, got %d", len(targets), len(all))
	}
	for i, scan := range all {
		if scan.Status != ReconciliationStatusCompleted {
			t.Fatalf("scan %d: expected completed, got %s", i, scan.Status)
		}
	}
}

func TestBuildDebugPanelData(t *testing.T) {
	data := BuildDebugPanelData(NewReconciliationEngine(DefaultReconciliationConfig()))
	if data.Consistency.ID == "" {
		t.Fatal("expected non-empty consistency ID")
	}
	if data.Consistency.Version.AppVersion == "" {
		t.Fatal("expected non-empty app version")
	}
	if data.Consistency.Version.SchemaVersion != 1 {
		t.Fatalf("expected schema version 1, got %d", data.Consistency.Version.SchemaVersion)
	}
	if data.GeneratedAt.IsZero() {
		t.Fatal("expected non-zero generatedAt")
	}
	if len(data.RuntimeMetrics) != 4 {
		t.Fatalf("expected 4 runtime metrics, got %d", len(data.RuntimeMetrics))
	}
}

func TestBuildSanitizedExport(t *testing.T) {
	export := BuildSanitizedExport(NewReconciliationEngine(DefaultReconciliationConfig()))
	if export.ExportID == "" {
		t.Fatal("expected non-empty export ID")
	}
	if !export.Sanitized {
		t.Fatal("expected sanitized true")
	}
	if export.GeneratedAt.IsZero() {
		t.Fatal("expected non-zero generatedAt")
	}
	if export.PanelData.Consistency.ID == "" {
		t.Fatal("expected non-empty consistency ID in panel data")
	}
}

func TestDefaultReconciliationConfig(t *testing.T) {
	config := DefaultReconciliationConfig()
	if config.BatchSize != 50 {
		t.Fatalf("expected batch 50, got %d", config.BatchSize)
	}
	if config.BudgetLimitMS != 5000 {
		t.Fatalf("expected budget 5000, got %d", config.BudgetLimitMS)
	}
	if config.MaxConcurrency != 2 {
		t.Fatalf("expected concurrency 2, got %d", config.MaxConcurrency)
	}
	if config.AutoRepair {
		t.Fatal("auto repair should default to false")
	}
	if config.RetryCount != 3 {
		t.Fatalf("expected retry 3, got %d", config.RetryCount)
	}
}

func TestReconciliationDiffSeverity(t *testing.T) {
	diff := ReconciliationDiff{
		ID:             "diff-sev",
		Severity:       "critical",
		AutoRepairable: false,
		DiffType:       "orphan",
		Description:    "Orphan record with no source reference",
		FoundAt:        time.Now().UTC(),
	}
	if diff.Severity != "critical" {
		t.Fatalf("expected critical, got %s", diff.Severity)
	}
	if diff.Repaired {
		t.Fatal("new diff should not be repaired")
	}
	diff.Repaired = true
	diff.RepairedAt = time.Now().UTC()
	if !diff.Repaired {
		t.Fatal("diff should be repaired after setting")
	}
}

func TestRegisterDefaultRuntimeReconciliationCheckersRegistersAvailableTargets(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&DeletionTombstoneModel{}, &OutboxCleanupItemModel{}, &RecalculationTaskModel{}); err != nil {
		t.Fatal(err)
	}
	statements := []string{
		"CREATE TABLE outbox_records (id TEXT PRIMARY KEY, aggregate_id TEXT, event_type TEXT, payload TEXT, status TEXT, retry_count INTEGER, last_error TEXT, leased_until DATETIME)",
		"CREATE TABLE interaction_records (id TEXT PRIMARY KEY, user_id TEXT, character_id TEXT, conversation_id TEXT, request_id TEXT, status TEXT, status_version INTEGER, result_ref TEXT, error_code TEXT, error_message TEXT)",
		"CREATE TABLE messages (id TEXT PRIMARY KEY, conversation_id TEXT, character_id TEXT, request_id TEXT, status TEXT, content TEXT, updated_at DATETIME)",
		"CREATE TABLE delivery_intents (id TEXT PRIMARY KEY, request_id TEXT, channel TEXT, status TEXT, retry_count INTEGER, last_error TEXT)",
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}

	engine := NewReconciliationEngine(DefaultReconciliationConfig())
	if err := RegisterDefaultRuntimeReconciliationCheckers(engine, db); err != nil {
		t.Fatal(err)
	}
	targets := engine.RegisteredTargets()
	want := map[ReconciliationTarget]bool{
		ReconciliationTombstoneDerivedData: false,
		ReconciliationLeaseDelivery:        false,
		ReconciliationOutboxSideEffect:     false,
		ReconciliationInteractionRunMsg:    false,
	}
	for _, target := range targets {
		if _, ok := want[target]; ok {
			want[target] = true
		}
	}
	for target, found := range want {
		if !found {
			t.Fatalf("expected registered target %s in %#v", target, targets)
		}
	}
}

func TestTombstoneDerivedDataReconciliationCheckerDetectsMissingDerivedRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&DeletionTombstoneModel{}, &OutboxCleanupItemModel{}, &RecalculationTaskModel{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)
	if err := db.Create(&DeletionTombstoneModel{
		ID:               "tombstone-missing",
		TargetID:         "target-missing",
		TargetType:       "memory",
		Scope:            string(DeletionScopeAll),
		Status:           string(DeletionStatusBlocked),
		RequestedAt:      now.Add(-time.Hour),
		BlockedUntil:     now.Add(-time.Hour),
		RetrievalBlocked: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&OutboxCleanupItemModel{
		ID:         "outbox-existing",
		Storage:    "qdrant",
		TargetID:   "target-missing",
		TargetKind: "memory",
		Status:     "queued",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&RecalculationTaskModel{
		ID:           "recalc-existing",
		TriggerType:  "deletion",
		TargetID:     "target-missing",
		AffectedZone: "belief",
		Priority:     1,
		CreatedAt:    now,
		Status:       "pending",
		Description:  "test",
	}).Error; err != nil {
		t.Fatal(err)
	}

	checker := NewTombstoneDerivedDataReconciliationChecker(db)
	checker.Now = func() time.Time { return now }
	diffs, err := checker.CheckReconciliation(context.Background(), ReconciliationCheckRequest{
		Target:    ReconciliationTombstoneDerivedData,
		Strategy:  StrategyLogicalInvalid,
		BatchSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertDiffTypes(t, diffs, []string{
		"missing_cleanup_item",
		"missing_cleanup_item",
		"missing_cleanup_item",
		"missing_cleanup_item",
		"missing_cleanup_item",
		"missing_recalculation_task",
		"missing_recalculation_task",
	})
}

func TestTombstoneDerivedDataReconciliationCheckerPassesCompleteDerivedRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&DeletionTombstoneModel{}, &OutboxCleanupItemModel{}, &RecalculationTaskModel{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)
	if err := db.Create(&DeletionTombstoneModel{
		ID:               "tombstone-complete",
		TargetID:         "target-complete",
		TargetType:       "memory",
		Scope:            string(DeletionScopeAll),
		Status:           string(DeletionStatusBlocked),
		RequestedAt:      now.Add(-time.Hour),
		BlockedUntil:     now.Add(-time.Hour),
		RetrievalBlocked: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	for _, storage := range []string{"qdrant", "surrealdb", "cache", "summaries", "reflections", "traces"} {
		if err := db.Create(&OutboxCleanupItemModel{
			ID:         "outbox-complete-" + storage,
			Storage:    storage,
			TargetID:   "target-complete",
			TargetKind: "memory",
			Status:     "queued",
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i, zone := range []string{"belief", "relationship", "memory"} {
		if err := db.Create(&RecalculationTaskModel{
			ID:           "recalc-complete-" + zone,
			TriggerType:  "deletion",
			TargetID:     "target-complete",
			AffectedZone: zone,
			Priority:     i + 1,
			CreatedAt:    now,
			Status:       "pending",
			Description:  "test",
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	checker := NewTombstoneDerivedDataReconciliationChecker(db)
	diffs, err := checker.CheckReconciliation(context.Background(), ReconciliationCheckRequest{
		Target:    ReconciliationTombstoneDerivedData,
		Strategy:  StrategyLogicalInvalid,
		BatchSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 0 {
		t.Fatalf("expected no diffs for complete derived rows, got %#v", diffs)
	}
}

func assertDiffTypes(t *testing.T, diffs []ReconciliationDiff, expected []string) {
	t.Helper()
	got := make(map[string]int)
	for _, diff := range diffs {
		got[diff.DiffType]++
	}
	for _, diffType := range expected {
		if got[diffType] == 0 {
			t.Fatalf("expected diff type %s in %#v", diffType, diffs)
		}
	}
	if len(diffs) != len(expected) {
		t.Fatalf("expected %d diffs, got %d: %#v", len(expected), len(diffs), diffs)
	}
}
