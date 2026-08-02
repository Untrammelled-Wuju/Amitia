// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package taskstate

import (
	"errors"
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/contracts"
)

var (
	ErrTransitionConflict      = errors.New("taskstate: transition conflict, rows affected 0")
	ErrExecutionOwnershipLost  = errors.New("taskstate: execution ownership lost")
	ErrInvalidTransition       = errors.New("taskstate: invalid transition")
	ErrInvalidStatusStageCombo = errors.New("taskstate: invalid status stage combination")
	ErrEntityNotFound          = errors.New("taskstate: entity not found")
	ErrVersionConflict         = errors.New("taskstate: row version conflict")
	ErrIllegalTargetStatus     = errors.New("taskstate: target status not allowed for entity")
	ErrTerminalOverrideAttempt = errors.New("taskstate: terminal status cannot be overridden by worker")
	ErrArtifactGateFailed      = errors.New("taskstate: artifact gate check failed")
	ErrSnapshotInconsistent    = errors.New("taskstate: snapshot inconsistent with artifacts")
)

type TransitionError struct {
	Code       string
	EntityType contracts.EntityType
	EntityID   string
	FromStatus contracts.LifecycleStatus
	ToStatus   contracts.LifecycleStatus
	Reason     contracts.TransitionReason
	Err        error
}

func (e *TransitionError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("taskstate: %s [%s %s %s->%s reason=%s]: %v",
			e.Code, e.EntityType, e.EntityID, e.FromStatus, e.ToStatus, e.Reason, e.Err)
	}
	return fmt.Sprintf("taskstate: %s [%s %s %s->%s reason=%s]",
		e.Code, e.EntityType, e.EntityID, e.FromStatus, e.ToStatus, e.Reason)
}

func (e *TransitionError) Unwrap() error {
	if e.Err != nil {
		return e.Err
	}
	return nil
}

func (e *TransitionError) IsConflict() bool {
	return e.Code == "state_conflict" || e.Code == "version_conflict" || e.Code == "execution_ownership_lost"
}

func NewTransitionError(code string, et contracts.EntityType, entityID string, from, to contracts.LifecycleStatus, reason contracts.TransitionReason, err error) *TransitionError {
	return &TransitionError{
		Code:       code,
		EntityType: et,
		EntityID:   entityID,
		FromStatus: from,
		ToStatus:   to,
		Reason:     reason,
		Err:        err,
	}
}

const (
	CodeStateConflict          = "state_conflict"
	CodeVersionConflict        = "version_conflict"
	CodeExecutionOwnershipLost = "execution_ownership_lost"
	CodeInvalidTransition      = "invalid_transition"
	CodeInvalidCombo           = "invalid_status_stage_combo"
	CodeEntityNotFound         = "entity_not_found"
	CodeArtifactGateFailed     = "artifact_gate_failed"
)

func IsConflictError(err error) bool {
	var te *TransitionError
	if errors.As(err, &te) {
		return te.IsConflict()
	}
	return errors.Is(err, ErrTransitionConflict) ||
		errors.Is(err, ErrExecutionOwnershipLost) ||
		errors.Is(err, ErrVersionConflict)
}

func IsOwnershipLostError(err error) bool {
	var te *TransitionError
	if errors.As(err, &te) {
		return te.Code == CodeExecutionOwnershipLost
	}
	return errors.Is(err, ErrExecutionOwnershipLost)
}
