package dev_mode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

type ReloadEvent struct {
	WorkspaceID WorkspaceID
	RevisionID  RevisionID
	Stage       ReloadStage
	Reason      string
	StartedAt   time.Time
	CompletedAt time.Time
	Success     bool
	Error       string
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

type RuntimeReloader struct {
	mu                sync.Mutex
	registry          *WorkspaceRegistry
	pipeline          *RebuildPipeline
	preserver         *StatePreserver
	reloadChans       map[WorkspaceID]chan ReloadEvent
	enabled           map[WorkspaceID]bool
	activeReloads     map[WorkspaceID]bool
	drainTimeout      time.Duration
	invMu             sync.Mutex
	activeInvocations map[WorkspaceID]int32
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
		activeInvocations: make(map[WorkspaceID]int32),
	}
}

var (
	ErrReloadDisabled        = errors.New("dev_mode: reload disabled")
	ErrReloadInProgress      = errors.New("dev_mode: reload in progress")
	ErrStateCaptureFailed    = errors.New("dev_mode: state capture failed")
	ErrDrainTimeout          = errors.New("dev_mode: drain timeout")
	ErrReloadValidationFailed = errors.New("dev_mode: reload validation failed")
	ErrReloadLoadFailed      = errors.New("dev_mode: reload load failed")
	ErrReloadActivateFailed  = errors.New("dev_mode: reload activate failed")
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

	ev := ReloadEvent{
		WorkspaceID: id,
		Stage:       ReloadStageStatePreserve,
		Reason:      reason,
		StartedAt:   time.Now().UTC(),
	}

	if stateProvider != nil {
		if state := stateProvider(); state != nil {
			r.preserver.Capture(id, ws.CurrentRevision, state)
			r.emit(id, ev)
		}
	}

	if err := r.drain(ctx, id); err != nil {
		ev.Stage = ReloadStageUnload
		ev.Success = false
		ev.Error = err.Error()
		ev.CompletedAt = time.Now().UTC()
		r.emit(id, ev)
		_ = r.registry.UpdateStatus(id, WorkspaceStatusFailed, err.Error())
		return &ev, err
	}

	ev.Stage = ReloadStageUnload
	r.emit(id, ev)
	if err := r.registry.UpdateStatus(id, WorkspaceStatusReloading, ""); err != nil {
		ev.Success = false
		ev.Error = err.Error()
		ev.CompletedAt = time.Now().UTC()
		r.emit(id, ev)
		return &ev, err
	}

	ev.Stage = ReloadStageValidate
	r.emit(id, ev)
	if vErrs := r.validateManifest(ws); len(vErrs) > 0 {
		msg := vErrs[0]
		ev.Success = false
		ev.Error = msg
		ev.CompletedAt = time.Now().UTC()
		r.emit(id, ev)
		_ = r.registry.UpdateStatus(id, WorkspaceStatusFailed, msg)
		return &ev, fmt.Errorf("%w: %s", ErrReloadValidationFailed, msg)
	}

	ev.Stage = ReloadStageRebuild
	rev, err := r.pipeline.Build(ctx, id, BuildOptions{
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
	ev.RevisionID = rev.RevisionID
	r.emit(id, ev)

	ev.Stage = ReloadStageLoad
	r.emit(id, ev)
	if err := r.loadDefinitions(id, rev); err != nil {
		ev.Success = false
		ev.Error = err.Error()
		ev.CompletedAt = time.Now().UTC()
		r.emit(id, ev)
		_ = r.registry.UpdateStatus(id, WorkspaceStatusFailed, err.Error())
		return &ev, fmt.Errorf("%w: %v", ErrReloadLoadFailed, err)
	}

	ev.Stage = ReloadStageActivate
	r.emit(id, ev)
	if err := r.activateContributions(id, rev); err != nil {
		ev.Success = false
		ev.Error = err.Error()
		ev.CompletedAt = time.Now().UTC()
		r.emit(id, ev)
		_ = r.registry.UpdateStatus(id, WorkspaceStatusFailed, err.Error())
		return &ev, fmt.Errorf("%w: %v", ErrReloadActivateFailed, err)
	}

	if snap, ok := r.preserver.Restore(id); ok {
		ev.Stage = ReloadStageStatePreserve
		ev.Reason = fmt.Sprintf("%s; restored state from %s", ev.Reason, snap.RevisionID)
		r.emit(id, ev)
	}

	ev.Stage = ReloadStageUIRefresh
	ev.Success = true
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

func (r *RuntimeReloader) loadDefinitions(id WorkspaceID, rev *Revision) error {
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
	return r.registry.SetCurrentRevision(id, rev.RevisionID)
}

func (r *RuntimeReloader) activateContributions(id WorkspaceID, rev *Revision) error {
	if rev == nil {
		return fmt.Errorf("cannot activate nil revision")
	}
	if rev.Status != RevisionStatusSucceeded {
		return fmt.Errorf("cannot activate revision with status %s", rev.Status)
	}
	return r.registry.UpdateStatus(id, WorkspaceStatusReady, "")
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
