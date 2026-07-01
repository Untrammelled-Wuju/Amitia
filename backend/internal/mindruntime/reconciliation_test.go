package mindruntime

import (
	"testing"
	"time"
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
		ID:            "diff-001",
		ScanID:        scan.ID,
		Source:        "sqlite",
		Target:        "qdrant",
		DiffType:      "missing",
		SourceKey:     "doc-001",
		TargetKey:     "qt-001",
		Description:   "Document exists in SQLite but missing from Qdrant",
		Severity:      "warning",
		AutoRepairable: true,
		RepairAction:  "reindex",
		FoundAt:       time.Now().UTC(),
	}
	diff2 := ReconciliationDiff{
		ID:            "diff-002",
		ScanID:        scan.ID,
		Source:        "sqlite",
		Target:        "surrealdb",
		DiffType:      "stale",
		SourceKey:     "rec-002",
		TargetKey:     "sr-002",
		Description:   "Record version mismatch",
		Severity:      "critical",
		AutoRepairable: false,
		FoundAt:       time.Now().UTC(),
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
	data := BuildDebugPanelData()
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
	export := BuildSanitizedExport()
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
		ID:            "diff-sev",
		Severity:      "critical",
		AutoRepairable: false,
		DiffType:      "orphan",
		Description:   "Orphan record with no source reference",
		FoundAt:       time.Now().UTC(),
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
