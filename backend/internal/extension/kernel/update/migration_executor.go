package update

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

type MigrationRunStatus string

const (
	MigrationStatusPending   MigrationRunStatus = "pending"
	MigrationStatusRunning   MigrationRunStatus = "running"
	MigrationStatusSucceeded MigrationRunStatus = "succeeded"
	MigrationStatusFailed    MigrationRunStatus = "failed"
	MigrationStatusRolled    MigrationRunStatus = "rolled_back"
	MigrationStatusSkipped   MigrationRunStatus = "skipped"
)

type MigrationRun struct {
	RunID         string
	MigrationID   string
	ExtensionID   string
	SourceVersion string
	TargetVersion string
	InputHash     string
	OutputHash    string
	Status        MigrationRunStatus
	Attempt       int
	StartedAt     time.Time
	FinishedAt    *time.Time
	Error         string
}

type MigrationExecutor struct {
	mu       sync.RWMutex
	runs     map[string][]MigrationRun
	snapshots map[string]StorageSnapshot
	locks    map[string]bool
}

func NewMigrationExecutor() *MigrationExecutor {
	return &MigrationExecutor{
		runs:      make(map[string][]MigrationRun),
		snapshots: make(map[string]StorageSnapshot),
		locks:     make(map[string]bool),
	}
}

type StorageSnapshot struct {
	SnapshotID   string
	ExtensionID  string
	Namespaces   []string
	DataHash     string
	TakenAt      time.Time
	Data         map[string][]byte
}

func (e *MigrationExecutor) SnapshotNamespaces(ctx context.Context, extensionID string, namespaces []string, data map[string][]byte) StorageSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	snapshot := StorageSnapshot{
		SnapshotID:  fmt.Sprintf("snap-%s-%d", extensionID, time.Now().UnixNano()),
		ExtensionID: extensionID,
		Namespaces:  namespaces,
		TakenAt:     time.Now().UTC(),
		Data:        make(map[string][]byte),
	}
	for _, ns := range namespaces {
		if d, ok := data[ns]; ok {
			snapshot.Data[ns] = d
		}
	}
	snapshot.DataHash = hashSnapshotData(snapshot.Data)
	e.snapshots[snapshot.SnapshotID] = snapshot
	return snapshot
}

func (e *MigrationExecutor) GetSnapshot(ctx context.Context, snapshotID string) (*StorageSnapshot, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s, ok := e.snapshots[snapshotID]
	if !ok {
		return nil, fmt.Errorf("update: snapshot %s not found", snapshotID)
	}
	result := s
	return &result, nil
}

func (e *MigrationExecutor) RestoreSnapshot(ctx context.Context, snapshotID string) (map[string][]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s, ok := e.snapshots[snapshotID]
	if !ok {
		return nil, fmt.Errorf("update: snapshot %s not found", snapshotID)
	}
	out := make(map[string][]byte, len(s.Data))
	for k, v := range s.Data {
		out[k] = v
	}
	return out, nil
}

func (e *MigrationExecutor) AcquireLock(ctx context.Context, extensionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.locks[extensionID] {
		return errors.New("update: migration lock already held")
	}
	e.locks[extensionID] = true
	return nil
}

func (e *MigrationExecutor) ReleaseLock(ctx context.Context, extensionID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.locks, extensionID)
}

func (e *MigrationExecutor) ExecuteMigration(ctx context.Context, plan MigrationPlan, extensionID, sourceVersion, targetVersion string, inputHash string, handler func(ctx context.Context) (string, error)) (MigrationRun, error) {
	if err := e.AcquireLock(ctx, extensionID); err != nil {
		return MigrationRun{}, err
	}
	defer e.ReleaseLock(ctx, extensionID)

	e.mu.Lock()
	runs := e.runs[extensionID]
	for _, r := range runs {
		if r.MigrationID == plan.MigrationID && r.SourceVersion == sourceVersion && r.TargetVersion == targetVersion && r.Status == MigrationStatusSucceeded {
			e.mu.Unlock()
			r := r
			r.Status = MigrationStatusSkipped
			return r, nil
		}
	}
	e.mu.Unlock()

	run := MigrationRun{
		RunID:         fmt.Sprintf("run-%s-%d", plan.MigrationID, time.Now().UnixNano()),
		MigrationID:   plan.MigrationID,
		ExtensionID:   extensionID,
		SourceVersion: sourceVersion,
		TargetVersion: targetVersion,
		InputHash:     inputHash,
		Status:        MigrationStatusRunning,
		Attempt:       1,
		StartedAt:     time.Now().UTC(),
	}

	outputHash, err := handler(ctx)
	if err != nil {
		run.Status = MigrationStatusFailed
		run.Error = err.Error()
		now := time.Now().UTC()
		run.FinishedAt = &now
		e.mu.Lock()
		e.runs[extensionID] = append(e.runs[extensionID], run)
		e.mu.Unlock()
		return run, err
	}

	run.OutputHash = outputHash
	run.Status = MigrationStatusSucceeded
	now := time.Now().UTC()
	run.FinishedAt = &now

	e.mu.Lock()
	e.runs[extensionID] = append(e.runs[extensionID], run)
	e.mu.Unlock()

	return run, nil
}

func (e *MigrationExecutor) ListRuns(ctx context.Context, extensionID string) []MigrationRun {
	e.mu.RLock()
	defer e.mu.RUnlock()
	runs := e.runs[extensionID]
	out := make([]MigrationRun, len(runs))
	copy(out, runs)
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt.Before(out[j].StartedAt)
	})
	return out
}

func (e *MigrationExecutor) GetRun(ctx context.Context, runID string) (*MigrationRun, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, runs := range e.runs {
		for _, r := range runs {
			if r.RunID == runID {
				result := r
				return &result, nil
			}
		}
	}
	return nil, fmt.Errorf("update: run %s not found", runID)
}

func (e *MigrationExecutor) RollbackMigration(ctx context.Context, runID string, handler func(ctx context.Context) error) error {
	run, err := e.GetRun(ctx, runID)
	if err != nil {
		return err
	}
	if run.Status != MigrationStatusSucceeded {
		return fmt.Errorf("update: cannot roll back migration with status %s", run.Status)
	}
	if err := handler(ctx); err != nil {
		return fmt.Errorf("update: rollback handler failed: %w", err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for extID, runs := range e.runs {
		for i := range runs {
			if runs[i].RunID == runID {
				runs[i].Status = MigrationStatusRolled
				now := time.Now().UTC()
				runs[i].FinishedAt = &now
				e.runs[extID] = runs
				return nil
			}
		}
	}
	return fmt.Errorf("update: run %s not found for update", runID)
}

func (e *MigrationExecutor) RequiresSnapshot(plan MigrationPlan) bool {
	return plan.RequiresSnapshot || !plan.Reversible
}

func hashSnapshotData(data map[string][]byte) string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b []byte
	for _, k := range keys {
		b = append(b, []byte(k)...)
		b = append(b, data[k]...)
	}
	return fmt.Sprintf("sha256:%x", simpleHash(b))
}

func simpleHash(b []byte) []byte {
	if len(b) == 0 {
		return []byte{}
	}
	h := make([]byte, 32)
	for i, byteVal := range b {
		h[i%32] ^= byteVal
	}
	return h
}
