package dev_mode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ReloadStatus string

const (
	ReloadSucceeded                   ReloadStatus = "reload_succeeded"
	ReloadSucceededWithCleanupFailure ReloadStatus = "reload_succeeded_with_cleanup_failure"
	ReloadFailed                      ReloadStatus = "reload_failed"
	ReloadRequiresRecovery            ReloadStatus = "requires_recovery"
)

type ReloadEvent struct {
	WorkspaceID   WorkspaceID
	RevisionID    RevisionID
	Stage         ReloadStage
	Reason        string
	StartedAt     time.Time
	CompletedAt   time.Time
	Success       bool
	Status        ReloadStatus
	CleanupFailed bool
	Error         string
}

type ReloadStage string

const (
	ReloadStageUnload        ReloadStage = "unload"
	ReloadStageRebuild       ReloadStage = "rebuild"
	ReloadStageValidate      ReloadStage = "validate"
	ReloadStageLoad          ReloadStage = "load"
	ReloadStageActivate      ReloadStage = "activate"
	ReloadStageUIRefresh     ReloadStage = "ui_refresh"
	ReloadStageStatePreserve ReloadStage = "state_preserve"
)

type StateSnapshot struct {
	WorkspaceID WorkspaceID
	RevisionID  RevisionID
	State       map[string]any
	CapturedAt  time.Time
}

type StatePreserver struct {
	mu        sync.Mutex
	snapshots map[WorkspaceID]StateSnapshot
}

func NewStatePreserver() *StatePreserver {
	return &StatePreserver{
		snapshots: make(map[WorkspaceID]StateSnapshot),
	}
}

func (s *StatePreserver) Capture(id WorkspaceID, rev RevisionID, state map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[id] = StateSnapshot{
		WorkspaceID: id,
		RevisionID:  rev,
		State:       state,
		CapturedAt:  time.Now().UTC(),
	}
}

func (s *StatePreserver) Restore(id WorkspaceID) (StateSnapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, ok := s.snapshots[id]
	return snap, ok
}

func (s *StatePreserver) Clear(id WorkspaceID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.snapshots, id)
}

type CandidateRunner interface {
	StartCandidate(ctx context.Context, id WorkspaceID, rev *Revision) (string, error)
	HealthCheck(ctx context.Context, instanceID string) error
	DrainInstance(ctx context.Context, instanceID string, timeout time.Duration) error
	StopInstance(ctx context.Context, instanceID string) error
}

type RuntimeReloader struct {
	mu                sync.Mutex
	registry          *WorkspaceRegistry
	pipeline          *RebuildPipeline
	preserver         *StatePreserver
	reloadChans       map[WorkspaceID]chan ReloadEvent
	enabled           map[WorkspaceID]bool
	activeReloads     map[WorkspaceID]bool
	drainTimeout      time.Duration
	stopTimeout       time.Duration
	invMu             sync.Mutex
	activeInvocations map[WorkspaceID]int32
	runner            CandidateRunner
	watcher           *FileWatcher
	sessions          *SessionManager
	cleanupStore      CleanupFailureStore
	instanceMu        sync.Mutex
	currentInstance   map[WorkspaceID]string
	oldInstance       map[WorkspaceID]string
}

func NewRuntimeReloader(registry *WorkspaceRegistry, pipeline *RebuildPipeline, preserver *StatePreserver) *RuntimeReloader {
	return &RuntimeReloader{
		registry:          registry,
		pipeline:          pipeline,
		preserver:         preserver,
		reloadChans:       make(map[WorkspaceID]chan ReloadEvent),
		enabled:           make(map[WorkspaceID]bool),
		activeReloads:     make(map[WorkspaceID]bool),
		drainTimeout:      30 * time.Second,
		stopTimeout:       15 * time.Second,
		activeInvocations: make(map[WorkspaceID]int32),
		currentInstance:   make(map[WorkspaceID]string),
		oldInstance:       make(map[WorkspaceID]string),
		cleanupStore:      NoopCleanupFailureStore{},
	}
}

func (r *RuntimeReloader) SetCandidateRunner(runner CandidateRunner) *RuntimeReloader {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runner = runner
	return r
}

func (r *RuntimeReloader) SetFileWatcher(w *FileWatcher) *RuntimeReloader {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.watcher = w
	return r
}

func (r *RuntimeReloader) SetSessionManager(m *SessionManager) *RuntimeReloader {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions = m
	return r
}

func (r *RuntimeReloader) SetCleanupFailureStore(store CleanupFailureStore) *RuntimeReloader {
	r.mu.Lock()
	defer r.mu.Unlock()
	if store != nil {
		r.cleanupStore = store
	}
	return r
}

func (r *RuntimeReloader) SetStopTimeout(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if d > 0 {
		r.stopTimeout = d
	}
}

func (r *RuntimeReloader) PendingCleanupFailures(ctx context.Context) (int64, error) {
	r.mu.Lock()
	store := r.cleanupStore
	r.mu.Unlock()
	if store == nil {
		return 0, nil
	}
	return store.Count(ctx)
}

var (
	ErrReloadDisabled         = errors.New("dev_mode: reload disabled")
	ErrReloadInProgress       = errors.New("dev_mode: reload in progress")
	ErrStateCaptureFailed     = errors.New("dev_mode: state capture failed")
	ErrDrainTimeout           = errors.New("dev_mode: drain timeout")
	ErrReloadValidationFailed = errors.New("dev_mode: reload validation failed")
	ErrReloadLoadFailed       = errors.New("dev_mode: reload load failed")
	ErrReloadActivateFailed   = errors.New("dev_mode: reload activate failed")
	ErrReloadHealthCheckFailed = errors.New("dev_mode: reload health check failed")
)

func (r *RuntimeReloader) SetDrainTimeout(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if d > 0 {
		r.drainTimeout = d
	}
}

func (r *RuntimeReloader) Enable(id WorkspaceID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled[id] = true
	if _, ok := r.reloadChans[id]; !ok {
		r.reloadChans[id] = make(chan ReloadEvent, 16)
	}
}

func (r *RuntimeReloader) Disable(id WorkspaceID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled[id] = false
}

func (r *RuntimeReloader) IsEnabled(id WorkspaceID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.enabled[id]
}

func (r *RuntimeReloader) Subscribe(id WorkspaceID) (<-chan ReloadEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch, ok := r.reloadChans[id]
	if !ok {
		ch = make(chan ReloadEvent, 16)
		r.reloadChans[id] = ch
	}
	return ch, nil
}

func (r *RuntimeReloader) IncrementInvocation(workspaceID WorkspaceID) {
	r.invMu.Lock()
	defer r.invMu.Unlock()
	r.activeInvocations[workspaceID]++
}

func (r *RuntimeReloader) DecrementInvocation(workspaceID WorkspaceID) {
	r.invMu.Lock()
	defer r.invMu.Unlock()
	v := r.activeInvocations[workspaceID]
	v--
	if v < 0 {
		v = 0
	}
	r.activeInvocations[workspaceID] = v
}

func (r *RuntimeReloader) ActiveInvocations(workspaceID WorkspaceID) int32 {
	r.invMu.Lock()
	defer r.invMu.Unlock()
	return r.activeInvocations[workspaceID]
}

func (r *RuntimeReloader) drain(ctx context.Context, workspaceID WorkspaceID) error {
	r.mu.Lock()
	timeout := r.drainTimeout
	r.mu.Unlock()
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		if r.ActiveInvocations(workspaceID) <= 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%w: %s (active=%d)", ErrDrainTimeout, workspaceID, r.ActiveInvocations(workspaceID))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (r *RuntimeReloader) Reload(ctx context.Context, id WorkspaceID, reason string, stateProvider func() map[string]any) (*ReloadEvent, error) {
	r.mu.Lock()
	if !r.enabled[id] {
		r.mu.Unlock()
		return nil, ErrReloadDisabled
	}
	if r.activeReloads[id] {
		r.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrReloadInProgress, id)
	}
	r.activeReloads[id] = true
	drainTimeout := r.drainTimeout
	runner := r.runner
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		delete(r.activeReloads, id)
		r.mu.Unlock()
	}()

	ws, err := r.registry.Get(id)
	if err != nil {
		return nil, err
	}
	if !ws.DevTrust {
		return nil, fmt.Errorf("%w: %s", ErrDevTrustNotGranted, id)
	}

	oldRev := ws.CurrentRevision

	ev := ReloadEvent{
		WorkspaceID: id,
		Stage:       ReloadStageStatePreserve,
		Reason:      reason,
		StartedAt:   time.Now().UTC(),
		Status:      ReloadFailed,
	}

	if stateProvider != nil {
		if state := stateProvider(); state != nil {
			r.preserver.Capture(id, oldRev, state)
			r.emit(id, ev)
		}
	}

	ev.Stage = ReloadStageRebuild
	r.emit(id, ev)
	candidateRev, err := r.pipeline.BuildCandidate(ctx, id, BuildOptions{
		Watch:            ws.WatchEnabled,
		SourceMap:        true,
		Deterministic:    true,
		IncludeResources: true,
	})
	if err != nil {
		ev.Success = false
		ev.Error = err.Error()
		ev.CompletedAt = time.Now().UTC()
		r.emit(id, ev)
		_ = r.registry.UpdateStatus(id, WorkspaceStatusFailed, err.Error())
		return &ev, err
	}
	ev.RevisionID = candidateRev.RevisionID
	r.emit(id, ev)

	ev.Stage = ReloadStageValidate
	r.emit(id, ev)
	if vErrs := r.validateManifest(ws); len(vErrs) > 0 {
		msg := vErrs[0]
		ev.Success = false
		ev.Error = msg
		ev.CompletedAt = time.Now().UTC()
		r.emit(id, ev)
		r.cleanupCandidateArtifact(candidateRev)
		_ = r.registry.UpdateStatus(id, WorkspaceStatusFailed, msg)
		return &ev, fmt.Errorf("%w: %s", ErrReloadValidationFailed, msg)
	}
	if err := r.validateCandidateArtifact(candidateRev); err != nil {
		ev.Success = false
		ev.Error = err.Error()
		ev.CompletedAt = time.Now().UTC()
		r.emit(id, ev)
		r.cleanupCandidateArtifact(candidateRev)
		_ = r.registry.UpdateStatus(id, WorkspaceStatusFailed, err.Error())
		return &ev, fmt.Errorf("%w: %v", ErrReloadValidationFailed, err)
	}

	ev.Stage = ReloadStageLoad
	r.emit(id, ev)
	candidateInstanceID, healthErr := r.startAndHealthCheckCandidate(ctx, id, candidateRev, runner)
	if healthErr != nil {
		ev.Success = false
		ev.Error = healthErr.Error()
		ev.CompletedAt = time.Now().UTC()
		r.emit(id, ev)
		r.cleanupCandidateArtifact(candidateRev)
		if candidateInstanceID != "" && runner != nil {
			_ = runner.StopInstance(ctx, candidateInstanceID)
		}
		_ = r.registry.UpdateStatus(id, WorkspaceStatusFailed, healthErr.Error())
		_ = r.registry.UpdateStatus(id, WorkspaceStatusReady, "")
		return &ev, fmt.Errorf("%w: %v", ErrReloadHealthCheckFailed, healthErr)
	}

	ev.Stage = ReloadStageActivate
	r.emit(id, ev)
	if err := r.registry.SetCurrentRevision(id, candidateRev.RevisionID); err != nil {
		ev.Success = false
		ev.Error = err.Error()
		ev.CompletedAt = time.Now().UTC()
		r.emit(id, ev)
		r.cleanupCandidateArtifact(candidateRev)
		if candidateInstanceID != "" && runner != nil {
			_ = runner.StopInstance(ctx, candidateInstanceID)
		}
		return &ev, err
	}
	r.pipeline.PromoteRevision(id, candidateRev.RevisionID)
	if err := r.registry.UpdateStatus(id, WorkspaceStatusReady, ""); err != nil {
		ev.Success = false
		ev.Error = err.Error()
		ev.CompletedAt = time.Now().UTC()
		r.emit(id, ev)
		return &ev, err
	}

	r.instanceMu.Lock()
	oldInstanceID := r.currentInstance[id]
	r.currentInstance[id] = candidateInstanceID
	r.oldInstance[id] = oldInstanceID
	r.instanceMu.Unlock()

	ev.Stage = ReloadStageUnload
	r.emit(id, ev)
	_ = r.drain(ctx, id)
	cleanupFailed := false
	if oldInstanceID != "" && runner != nil {
		if drainErr := runner.DrainInstance(ctx, oldInstanceID, drainTimeout); drainErr != nil {
			ev.Error = fmt.Sprintf("drain warning: %v", drainErr)
			r.emit(id, ev)
		}
		stopTimeout := r.stopTimeout
		if stopTimeout <= 0 {
			stopTimeout = 15 * time.Second
		}
		stopCtx, stopCancel := context.WithTimeout(ctx, stopTimeout)
		if stopErr := runner.StopInstance(stopCtx, oldInstanceID); stopErr != nil {
			stopCancel()
			cleanupFailed = true
			ev.CleanupFailed = true
			ev.Status = ReloadSucceededWithCleanupFailure
			ev.Error = fmt.Sprintf("RUNTIME_STOP_FAILED: %v; requires_recovery", stopErr)
			r.emit(id, ev)
			if r.cleanupStore != nil {
				failureRecord := &CleanupFailureRecord{
					WorkspaceID:   id,
					ExtensionID:   string(ws.ExtensionID),
					OldInstanceID: oldInstanceID,
					NewInstanceID: candidateInstanceID,
					ErrorCode:     "RUNTIME_STOP_FAILED",
					ErrorMessage:  stopErr.Error(),
					NextRetryAt:   time.Now().UTC().Add(30 * time.Second),
					Status:        CleanupFailurePending,
				}
				_ = r.cleanupStore.Save(ctx, failureRecord)
			}
		} else {
			stopCancel()
			r.instanceMu.Lock()
			delete(r.oldInstance, id)
			r.instanceMu.Unlock()
		}
	} else {
		r.instanceMu.Lock()
		delete(r.oldInstance, id)
		r.instanceMu.Unlock()
	}

	if snap, ok := r.preserver.Restore(id); ok {
		ev.Stage = ReloadStageStatePreserve
		ev.Reason = fmt.Sprintf("%s; restored state from %s", ev.Reason, snap.RevisionID)
		r.emit(id, ev)
	}

	ev.Stage = ReloadStageUIRefresh
	ev.Success = true
	if !cleanupFailed {
		ev.Status = ReloadSucceeded
	}
	ev.CompletedAt = time.Now().UTC()
	r.emit(id, ev)
	return &ev, nil
}

func (r *RuntimeReloader) validateManifest(ws *DevelopmentWorkspace) []string {
	data, err := os.ReadFile(ws.ManifestPath)
	if err != nil {
		return []string{fmt.Sprintf("read manifest: %v", err)}
	}
	if len(data) == 0 {
		return []string{"manifest is empty"}
	}
	var probe map[string]any
	if err := json.Unmarshal(data, &probe); err != nil {
		return []string{fmt.Sprintf("parse manifest: %v", err)}
	}
	if ext, ok := probe["extension"].(map[string]any); ok {
		if id, _ := ext["id"].(string); id != "" {
			return nil
		}
	}
	if id, ok := probe["id"].(string); ok && id != "" {
		return nil
	}
	return []string{"manifest missing required field: extension.id"}
}

func (r *RuntimeReloader) validateCandidateArtifact(rev *Revision) error {
	if rev == nil || rev.ArtifactPath == "" {
		return fmt.Errorf("missing build artifact")
	}
	info, err := os.Stat(rev.ArtifactPath)
	if err != nil {
		return fmt.Errorf("artifact stat: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("artifact is not a directory: %s", rev.ArtifactPath)
	}
	entries, err := os.ReadDir(rev.ArtifactPath)
	if err != nil {
		return fmt.Errorf("artifact readdir: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("artifact directory is empty: %s", rev.ArtifactPath)
	}
	return nil
}

func (r *RuntimeReloader) startAndHealthCheckCandidate(ctx context.Context, id WorkspaceID, rev *Revision, runner CandidateRunner) (string, error) {
	if runner == nil {
		if rev == nil || rev.ArtifactPath == "" {
			return "", fmt.Errorf("missing build artifact for health check")
		}
		if _, err := os.Stat(rev.ArtifactPath); err != nil {
			return "", fmt.Errorf("artifact stat: %w", err)
		}
		return "", nil
	}
	instanceID, err := runner.StartCandidate(ctx, id, rev)
	if err != nil {
		return instanceID, fmt.Errorf("start candidate: %w", err)
	}
	if err := runner.HealthCheck(ctx, instanceID); err != nil {
		return instanceID, fmt.Errorf("health check: %w", err)
	}
	return instanceID, nil
}

func (r *RuntimeReloader) cleanupCandidateArtifact(rev *Revision) {
	if rev == nil || rev.ArtifactPath == "" {
		return
	}
	if filepath.Base(rev.ArtifactPath) == "dist" {
		return
	}
	_ = os.RemoveAll(rev.ArtifactPath)
}

func (r *RuntimeReloader) CloseDevMode(ctx context.Context, workspaceID WorkspaceID) error {
	var firstErr error
	if r.watcher != nil {
		if r.watcher.IsRunning(workspaceID) {
			if err := r.watcher.Stop(workspaceID); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	r.instanceMu.Lock()
	instanceID := r.currentInstance[workspaceID]
	delete(r.currentInstance, workspaceID)
	delete(r.oldInstance, workspaceID)
	r.instanceMu.Unlock()
	if instanceID != "" && r.runner != nil {
		if err := r.runner.StopInstance(ctx, instanceID); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if r.sessions != nil {
		if sess, err := r.sessions.GetByWorkspace(workspaceID); err == nil {
			_ = r.sessions.Revoke(sess.SessionID)
		}
	}
	if r.preserver != nil {
		r.preserver.Clear(workspaceID)
	}
	if err := r.registry.Remove(workspaceID); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (r *RuntimeReloader) RecoverStaleInstances(ctx context.Context) int {
	cleaned := 0

	r.instanceMu.Lock()
	stale := make(map[WorkspaceID]string, len(r.oldInstance))
	for id, instID := range r.oldInstance {
		stale[id] = instID
	}
	for id := range stale {
		delete(r.oldInstance, id)
	}
	runner := r.runner
	stopTimeout := r.stopTimeout
	if stopTimeout <= 0 {
		stopTimeout = 15 * time.Second
	}
	r.instanceMu.Unlock()

	if runner != nil {
		for _, instID := range stale {
			if instID == "" {
				continue
			}
			if err := runner.StopInstance(ctx, instID); err == nil {
				cleaned++
			}
		}
	}

	r.mu.Lock()
	store := r.cleanupStore
	r.mu.Unlock()
	if store == nil {
		return cleaned
	}

	failures, err := store.ListPending(ctx)
	if err != nil {
		return cleaned
	}

	for _, failure := range failures {
		if failure.Status == CleanupFailureExhausted {
			continue
		}
		if time.Now().UTC().Before(failure.NextRetryAt) {
			continue
		}

		newRetryCount := failure.RetryCount + 1
		if newRetryCount > failure.MaxRetries {
			_ = store.UpdateRetry(ctx, failure.FailureID, failure.RetryCount, time.Now().UTC().Add(1*time.Hour), CleanupFailureExhausted)
			continue
		}

		if runner != nil && failure.OldInstanceID != "" {
			stopCtx, stopCancel := context.WithTimeout(ctx, stopTimeout)
			stopErr := runner.StopInstance(stopCtx, failure.OldInstanceID)
			stopCancel()
			if stopErr == nil {
				_ = store.Delete(ctx, failure.FailureID)
				cleaned++
			} else {
				backoff := time.Duration(30*(newRetryCount+1)) * time.Second
				_ = store.UpdateRetry(ctx, failure.FailureID, newRetryCount, time.Now().UTC().Add(backoff), CleanupFailureRetrying)
			}
		} else {
			_ = store.Delete(ctx, failure.FailureID)
			cleaned++
		}
	}

	return cleaned
}

func (r *RuntimeReloader) emit(id WorkspaceID, ev ReloadEvent) {
	r.mu.Lock()
	ch, ok := r.reloadChans[id]
	r.mu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- ev:
	default:
	}
}
