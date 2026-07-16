package mindruntime

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

func DefaultReconciliationConfig() ReconciliationConfig {
	return ReconciliationConfig{
		BatchSize:       50,
		PauseAfterBatch: false,
		BudgetLimitMS:   5000,
		MaxConcurrency:  2,
		AutoRepair:      false,
		RetryCount:      3,
		RetryDelay:      100 * time.Millisecond,
	}
}

type ReconciliationEngine struct {
	config      ReconciliationConfig
	scans       map[string]*ReconciliationScan
	checkers    map[ReconciliationTarget]ReconciliationChecker
	scanCtr     int64
	status      ReconciliationStatus
	mu          sync.RWMutex
	activeScans int32
	control     chan struct{}
	done        chan struct{}
}

func NewReconciliationEngine(config ReconciliationConfig) *ReconciliationEngine {
	return &ReconciliationEngine{
		config:   config,
		scans:    make(map[string]*ReconciliationScan),
		checkers: make(map[ReconciliationTarget]ReconciliationChecker),
		status:   ReconciliationStatusIdle,
		control:  make(chan struct{}, 1),
		done:     make(chan struct{}, 1),
	}
}

func (e *ReconciliationEngine) RegisterChecker(target ReconciliationTarget, checker ReconciliationChecker) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if checker == nil {
		delete(e.checkers, target)
		return
	}
	e.checkers[target] = checker
}

func (e *ReconciliationEngine) RegisteredTargets() []ReconciliationTarget {
	e.mu.RLock()
	defer e.mu.RUnlock()
	targets := make([]ReconciliationTarget, 0, len(e.checkers))
	for target := range e.checkers {
		targets = append(targets, target)
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i] < targets[j]
	})
	return targets
}

func (e *ReconciliationEngine) RunScan(ctx context.Context, target ReconciliationTarget, strategy ReconciliationStrategy, startCursor string) (*ReconciliationScan, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	scan := e.StartScan(target, strategy, startCursor)
	e.mu.RLock()
	checker := e.checkers[target]
	e.mu.RUnlock()
	if checker == nil {
		e.FailScan(scan.ID, errors.New("reconciliation checker not registered"))
		return scan, errors.New("reconciliation checker not registered")
	}
	started := time.Now()
	diffs, err := checker.CheckReconciliation(ctx, ReconciliationCheckRequest{
		ScanID:    scan.ID,
		Target:    target,
		Strategy:  strategy,
		CursorID:  startCursor,
		BatchSize: scan.BatchSize,
		StartedAt: scan.StartedAt,
	})
	budgetUsed := time.Since(started).Milliseconds()
	if err != nil {
		e.FailScan(scan.ID, err)
		return scan, err
	}
	for i := range diffs {
		if diffs[i].ID == "" {
			diffs[i].ID = fmt.Sprintf("diff-%s-%d", scan.ID, i+1)
		}
		diffs[i].ScanID = scan.ID
		if diffs[i].FoundAt.IsZero() {
			diffs[i].FoundAt = time.Now().UTC()
		}
		e.AddDiff(scan.ID, diffs[i])
	}
	e.UpdateScanProgress(scan.ID, int64(len(diffs)), int64(len(diffs)), 0, 0, budgetUsed)
	e.CompleteScan(scan.ID)
	return e.GetScan(scan.ID), nil
}

func (e *ReconciliationEngine) StartScan(target ReconciliationTarget, strategy ReconciliationStrategy, startCursor string) *ReconciliationScan {
	e.mu.Lock()
	defer e.mu.Unlock()
	scan := &ReconciliationScan{
		ID:            fmt.Sprintf("scan-%s-%d", time.Now().UTC().Format("20060102T150405.000"), atomic.AddInt64(&e.scanCtr, 1)),
		Target:        target,
		Strategy:      strategy,
		Status:        ReconciliationStatusRunning,
		StartedAt:     time.Now().UTC(),
		CursorID:      startCursor,
		BatchSize:     e.config.BatchSize,
		BudgetLimitMS: e.config.BudgetLimitMS,
	}
	e.scans[scan.ID] = scan
	e.status = ReconciliationStatusRunning
	return scan
}

func (e *ReconciliationEngine) UpdateScanProgress(scanID string, scanned, diffs, repaired, skipped int64, budgetUsed int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	scan, ok := e.scans[scanID]
	if !ok {
		return
	}
	scan.TotalScanned += scanned
	scan.DiffsFound += diffs
	scan.DiffsRepaired += repaired
	scan.DiffsSkipped += skipped
	scan.BudgetUsedMS += budgetUsed
}

func (e *ReconciliationEngine) UpdateCursor(scanID string, cursor string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if scan, ok := e.scans[scanID]; ok {
		scan.CursorID = cursor
	}
}

func (e *ReconciliationEngine) AddDiff(scanID string, diff ReconciliationDiff) {
	e.mu.Lock()
	defer e.mu.Unlock()
	scan, ok := e.scans[scanID]
	if !ok {
		return
	}
	scan.Diffs = append(scan.Diffs, diff)
}

func (e *ReconciliationEngine) CompleteScan(scanID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	scan, ok := e.scans[scanID]
	if !ok {
		return
	}
	scan.Status = ReconciliationStatusCompleted
	scan.EndedAt = time.Now().UTC()
	e.status = ReconciliationStatusIdle
}

func (e *ReconciliationEngine) FailScan(scanID string, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	scan, ok := e.scans[scanID]
	if !ok {
		return
	}
	scan.Status = ReconciliationStatusCancelled
	scan.EndedAt = time.Now().UTC()
	if err != nil {
		scan.DiffsSkipped++
		scan.Diffs = append(scan.Diffs, ReconciliationDiff{
			ID:          fmt.Sprintf("diff-%s-error", scanID),
			ScanID:      scanID,
			Source:      "reconciliation",
			Target:      string(scan.Target),
			DiffType:    "scan_error",
			Description: err.Error(),
			Severity:    "critical",
			FoundAt:     time.Now().UTC(),
		})
	}
	e.status = ReconciliationStatusIdle
}

func (e *ReconciliationEngine) PauseScan(scanID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if scan, ok := e.scans[scanID]; ok {
		scan.Status = ReconciliationStatusPaused
	}
}

func (e *ReconciliationEngine) ResumeScan(scanID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if scan, ok := e.scans[scanID]; ok {
		scan.Status = ReconciliationStatusRunning
	}
}

func (e *ReconciliationEngine) CancelScan(scanID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if scan, ok := e.scans[scanID]; ok {
		scan.Status = ReconciliationStatusCancelled
		scan.EndedAt = time.Now().UTC()
	}
}

func (e *ReconciliationEngine) GetScan(scanID string) *ReconciliationScan {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.scans[scanID]
}

func (e *ReconciliationEngine) AllScans() []*ReconciliationScan {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*ReconciliationScan, 0, len(e.scans))
	for _, scan := range e.scans {
		result = append(result, scan)
	}
	return result
}

func (e *ReconciliationEngine) Status() ReconciliationStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.status
}

func (e *ReconciliationEngine) IsBudgetExhausted(scanID string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	scan, ok := e.scans[scanID]
	if !ok {
		return true
	}
	if scan.BudgetLimitMS <= 0 {
		return false
	}
	return scan.BudgetUsedMS >= scan.BudgetLimitMS
}

func (e *ReconciliationEngine) RunWorker(ctx context.Context, interval time.Duration, targets []ReconciliationWorkerTarget) {
	if e == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if interval <= 0 {
		interval = time.Minute
	}
	run := func() {
		for _, target := range targets {
			if target.Target == "" {
				continue
			}
			strategy := target.Strategy
			if strategy == "" {
				strategy = StrategyManualConfirm
			}
			_, _ = e.RunScan(ctx, target.Target, strategy, target.Cursor)
		}
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func DefaultReconciliationWorkerTargets() []ReconciliationWorkerTarget {
	return []ReconciliationWorkerTarget{
		{Target: ReconciliationTombstoneDerivedData, Strategy: StrategyLogicalInvalid},
		{Target: ReconciliationLeaseDelivery, Strategy: StrategyReleaseLease},
		{Target: ReconciliationOutboxSideEffect, Strategy: StrategyCompensate},
		{Target: ReconciliationInteractionRunMsg, Strategy: StrategyCompensate},
	}
}
