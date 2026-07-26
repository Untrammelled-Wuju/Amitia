package dev_mode

import (
	"context"
	"errors"
	"fmt"
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
	ReloadStageUnload      ReloadStage = "unload"
	ReloadStageRebuild     ReloadStage = "rebuild"
	ReloadStageValidate    ReloadStage = "validate"
	ReloadStageLoad        ReloadStage = "load"
	ReloadStageActivate    ReloadStage = "activate"
	ReloadStageUIRefresh   ReloadStage = "ui_refresh"
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
	mu          sync.Mutex
	registry    *WorkspaceRegistry
	pipeline    *RebuildPipeline
	preserver   *StatePreserver
	reloadChans map[WorkspaceID]chan ReloadEvent
	enabled     map[WorkspaceID]bool
}

func NewRuntimeReloader(registry *WorkspaceRegistry, pipeline *RebuildPipeline, preserver *StatePreserver) *RuntimeReloader {
	return &RuntimeReloader{
		registry:    registry,
		pipeline:    pipeline,
		preserver:   preserver,
		reloadChans: make(map[WorkspaceID]chan ReloadEvent),
		enabled:     make(map[WorkspaceID]bool),
	}
}

var (
	ErrReloadDisabled      = errors.New("dev_mode: reload disabled")
	ErrReloadInProgress    = errors.New("dev_mode: reload in progress")
	ErrStateCaptureFailed  = errors.New("dev_mode: state capture failed")
)

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

func (r *RuntimeReloader) Reload(ctx context.Context, id WorkspaceID, reason string, stateProvider func() map[string]any) (*ReloadEvent, error) {
	r.mu.Lock()
	if !r.enabled[id] {
		r.mu.Unlock()
		return nil, ErrReloadDisabled
	}
	r.mu.Unlock()

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

	ev.Stage = ReloadStageUnload
	r.emit(id, ev)
	if err := r.registry.UpdateStatus(id, WorkspaceStatusReloading, ""); err != nil {
		ev.Success = false
		ev.Error = err.Error()
		ev.CompletedAt = time.Now().UTC()
		r.emit(id, ev)
		return &ev, err
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

	ev.Stage = ReloadStageValidate
	r.emit(id, ev)

	ev.Stage = ReloadStageLoad
	r.emit(id, ev)

	ev.Stage = ReloadStageActivate
	r.emit(id, ev)

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
