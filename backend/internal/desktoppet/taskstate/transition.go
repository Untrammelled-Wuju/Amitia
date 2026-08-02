// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package taskstate

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/contracts"
)

type TransitionRequest struct {
	EntityType      contracts.EntityType
	EntityID        string
	From            []contracts.LifecycleStatus
	To              contracts.LifecycleStatus
	Stage           contracts.Stage
	Reason          contracts.TransitionReason
	ActorType       contracts.ActorType
	ActorID         string
	ErrorCode       string
	ErrorMessage    string
	FailureStage    contracts.Stage
	ExpectedVersion int64
	ExecutionID     string
	WorkerID        string
	LeaseExpiresAt  string
	LastHeartbeatAt string
	Progress        *int
	Metadata        map[string]any
	ParentTaskID    string
	AttemptID       string
	NeedOwnership   bool
	OccurredAt      time.Time
}

type TransitionResult struct {
	Applied         bool
	PreviousStatus  contracts.LifecycleStatus
	PreviousStage   contracts.Stage
	CurrentStatus   contracts.LifecycleStatus
	PreviousVersion int64
	CurrentVersion  int64
	AuditID         string
}

type FieldUpdates struct {
	Status            contracts.LifecycleStatus
	Stage             contracts.Stage
	StatusReason      string
	FailureStage      string
	ErrorCode         string
	ErrorMessage      string
	Progress          *int
	StartedAt         string
	SetStartedIfEmpty bool
	CompletedAt       string
	ClearCompletedAt  bool
	SubmittedAt       string
	CancellingAt      string
	CancelledAt       string
	ClearCancelledAt  bool
	CancelRequestedAt string
	LastTransitionAt  string
	UpdatedAt         string
	ExecutionID       string
	ClearExecution    bool
	WorkerID          string
	ClearWorker       bool
	LeaseExpiresAt    string
	ClearLease        bool
	LastHeartbeatAt   string
	ClearHeartbeat    bool
	ClearError        bool
	BumpVersion       bool
}

type ApplyTransitionParams struct {
	EntityType       contracts.EntityType
	EntityID         string
	ExpectedStatuses []contracts.LifecycleStatus
	ExpectedVersion  int64
	ExecutionID      string
	NeedOwnership    bool
	Updates          FieldUpdates
	Audit            AuditRecord
}

type ApplyTransitionResult struct {
	Applied         bool
	ConflictReason  string
	PreviousStatus  contracts.LifecycleStatus
	PreviousStage   contracts.Stage
	PreviousVersion int64
	CurrentVersion  int64
}

type EntitySnapshot struct {
	Status            contracts.LifecycleStatus
	Stage             contracts.Stage
	RowVersion        int64
	ExecutionID       string
	WorkerID          string
	CancelRequested   bool
	CancelRequestedAt string
	LeaseExpiresAt    string
}

type Store interface {
	ApplyTransition(ctx context.Context, p ApplyTransitionParams) (*ApplyTransitionResult, error)
	GetSnapshot(ctx context.Context, et contracts.EntityType, id string) (*EntitySnapshot, error)
	WriteAudit(ctx context.Context, record AuditRecord) error
	ListAuditsByEntity(ctx context.Context, et contracts.EntityType, entityID string, limit int) ([]AuditRecord, error)
	ListAuditsByParent(ctx context.Context, parentTaskID string, limit int) ([]AuditRecord, error)
}

type Engine struct {
	store Store
	now   func() time.Time
}

func NewEngine(store Store) *Engine {
	return &Engine{store: store, now: time.Now}
}

func (e *Engine) WithClock(fn func() time.Time) *Engine {
	return &Engine{store: e.store, now: fn}
}

func (e *Engine) Transition(ctx context.Context, req TransitionRequest) (*TransitionResult, error) {
	if req.OccurredAt.IsZero() {
		req.OccurredAt = e.now()
	}
	if len(req.From) == 0 {
		return nil, NewTransitionError(CodeInvalidTransition, req.EntityType, req.EntityID, "", req.To, req.Reason, ErrInvalidTransition)
	}
	legal := false
	for _, from := range req.From {
		if IsLegalTransition(req.EntityType, from, req.To) {
			legal = true
			break
		}
	}
	if !legal {
		return nil, NewTransitionError(CodeInvalidTransition, req.EntityType, req.EntityID, req.From[0], req.To, req.Reason, ErrInvalidTransition)
	}
	if !contracts.ValidateStatusStageCombo(req.EntityType, req.To, req.Stage) {
		return nil, NewTransitionError(CodeInvalidCombo, req.EntityType, req.EntityID, req.From[0], req.To, req.Reason, ErrInvalidStatusStageCombo)
	}
	updates := e.buildFieldUpdates(req)
	audit := e.buildAudit(req, updates)
	result, err := e.store.ApplyTransition(ctx, ApplyTransitionParams{
		EntityType:       req.EntityType,
		EntityID:         req.EntityID,
		ExpectedStatuses: req.From,
		ExpectedVersion:  req.ExpectedVersion,
		ExecutionID:      req.ExecutionID,
		NeedOwnership:    req.NeedOwnership,
		Updates:          updates,
		Audit:            audit,
	})
	if err != nil {
		return nil, err
	}
	if !result.Applied {
		return nil, e.conflictError(req, result)
	}
	return &TransitionResult{
		Applied:         true,
		PreviousStatus:  result.PreviousStatus,
		PreviousStage:   result.PreviousStage,
		CurrentStatus:   req.To,
		PreviousVersion: result.PreviousVersion,
		CurrentVersion:  result.CurrentVersion,
		AuditID:         audit.ID,
	}, nil
}

func (e *Engine) buildFieldUpdates(req TransitionRequest) FieldUpdates {
	ts := req.OccurredAt.Format("2006-01-02 15:04:05")
	u := FieldUpdates{
		Status:           req.To,
		Stage:            req.Stage,
		StatusReason:     string(req.Reason),
		FailureStage:     string(req.FailureStage),
		ErrorCode:        req.ErrorCode,
		ErrorMessage:     req.ErrorMessage,
		Progress:         req.Progress,
		LastTransitionAt: ts,
		UpdatedAt:        ts,
		BumpVersion:      true,
	}
	switch req.To {
	case contracts.StatusQueued:
		u.SubmittedAt = ts
		u.ClearCompletedAt = true
		u.ClearCancelledAt = true
		u.ClearError = true
		u.ExecutionID = ""
		u.ClearExecution = true
		u.ClearWorker = true
		u.ClearLease = true
		u.ClearHeartbeat = true
	case contracts.StatusProcessing:
		u.ExecutionID = req.ExecutionID
		u.WorkerID = req.WorkerID
		u.LeaseExpiresAt = req.LeaseExpiresAt
		u.LastHeartbeatAt = req.LastHeartbeatAt
		u.StartedAt = ts
		u.SetStartedIfEmpty = true
	case contracts.StatusCancelling:
		u.CancellingAt = ts
		u.CancelRequestedAt = ts
	case contracts.StatusCancelled:
		u.CancelledAt = ts
		u.CompletedAt = ts
		u.CancelRequestedAt = ts
		u.ClearExecution = true
		u.ClearWorker = true
		u.ClearLease = true
		u.ClearHeartbeat = true
	case contracts.StatusSucceeded, contracts.StatusPartiallySucceeded, contracts.StatusFailed:
		u.CompletedAt = ts
		if req.To == contracts.StatusFailed {
			u.ClearExecution = true
			u.ClearWorker = true
			u.ClearLease = true
			u.ClearHeartbeat = true
		}
		if req.To != contracts.StatusFailed {
			u.ClearExecution = true
			u.ClearWorker = true
			u.ClearLease = true
			u.ClearHeartbeat = true
		}
	}
	return u
}

func (e *Engine) buildAudit(req TransitionRequest, u FieldUpdates) AuditRecord {
	ts := req.OccurredAt.Format("2006-01-02 15:04:05")
	a := AuditRecord{
		ID:           NewAuditID(),
		EntityType:   req.EntityType,
		EntityID:     req.EntityID,
		ParentTaskID: req.ParentTaskID,
		ExecutionID:  req.ExecutionID,
		AttemptID:    req.AttemptID,
		ToStatus:     req.To,
		ToStage:      req.Stage,
		ReasonCode:   req.Reason,
		ErrorCode:    req.ErrorCode,
		ErrorMessage: req.ErrorMessage,
		FailureStage: req.FailureStage,
		ActorType:    req.ActorType,
		ActorID:      req.ActorID,
		Metadata:     req.Metadata,
		CreatedAt:    ts,
	}
	if len(req.From) > 0 {
		a.FromStatus = req.From[0]
	}
	a.Sanitize()
	return a
}

func (e *Engine) conflictError(req TransitionRequest, result *ApplyTransitionResult) error {
	var from contracts.LifecycleStatus
	if len(req.From) > 0 {
		from = req.From[0]
	}
	code := CodeStateConflict
	var baseErr = ErrTransitionConflict
	switch result.ConflictReason {
	case "not_found":
		code = CodeEntityNotFound
		baseErr = ErrEntityNotFound
	case "version_mismatch":
		code = CodeVersionConflict
		baseErr = ErrVersionConflict
	case "ownership_lost":
		code = CodeExecutionOwnershipLost
		baseErr = ErrExecutionOwnershipLost
	}
	return NewTransitionError(code, req.EntityType, req.EntityID, from, req.To, req.Reason, baseErr)
}

func (e *Engine) GetSnapshot(ctx context.Context, et contracts.EntityType, id string) (*EntitySnapshot, error) {
	return e.store.GetSnapshot(ctx, et, id)
}
