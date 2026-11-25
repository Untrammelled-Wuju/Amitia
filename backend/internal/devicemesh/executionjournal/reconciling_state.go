package executionjournal

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrReconcileQueryInvalid = errors.New("核对查询：作用域、幂等键或操作编号非法")

type ReconcilingState string

const (
	ReconcilingAbsent    ReconcilingState = "absent"
	ReconcilingRetryable ReconcilingState = "retryable"
	ReconcilingSettled   ReconcilingState = "settled"
	ReconcilingInFlight  ReconcilingState = "in_flight"
	ReconcilingAmbiguous ReconcilingState = "ambiguous"
	ReconcilingBlocked   ReconcilingState = "blocked"
)

func (s ReconcilingState) Valid() bool {
	switch s {
	case ReconcilingAbsent, ReconcilingRetryable, ReconcilingSettled, ReconcilingInFlight, ReconcilingAmbiguous, ReconcilingBlocked:
		return true
	}
	return false
}

func (s ReconcilingState) AutoRetryAllowed() bool {
	return s == ReconcilingRetryable
}

func (s ReconcilingState) RequiresManualReview() bool {
	return s == ReconcilingInFlight || s == ReconcilingAmbiguous || s == ReconcilingBlocked
}

func (s ReconcilingState) NextAction() string {
	switch s {
	case ReconcilingRetryable:
		return "retry"
	case ReconcilingInFlight, ReconcilingAmbiguous, ReconcilingBlocked:
		return "manual_review"
	}
	return "none"
}

func (s ReconcilingState) severity() int {
	switch s {
	case ReconcilingRetryable:
		return 1
	case ReconcilingSettled:
		return 2
	case ReconcilingInFlight:
		return 3
	case ReconcilingAmbiguous:
		return 4
	case ReconcilingBlocked:
		return 5
	}
	return 0
}

type ReconcileSnapshot struct {
	ScopeKey            string
	IdempotencyKey      string
	OperationID         string
	State               ReconcilingState
	Effect              EffectState
	Reconciliation      ReconciliationState
	Attempts            int
	LedgerFound         bool
	ReconciliationFound bool
	AutoRetry           bool
	ManualReview        bool
	Conflict            bool
	NextAction          string
	Reason              string
}

func ledgerReconcilingState(state EffectState) ReconcilingState {
	switch state {
	case EffectPending:
		return ReconcilingInFlight
	case EffectCommitted:
		return ReconcilingSettled
	case EffectRejected:
		return ReconcilingBlocked
	case EffectUnknown:
		return ReconcilingAmbiguous
	}
	return ReconcilingAbsent
}

func reconciliationReconcilingState(state ReconciliationState) ReconcilingState {
	switch state {
	case ReconciliationAttempting:
		return ReconcilingInFlight
	case ReconciliationNeedsCheck:
		return ReconcilingAmbiguous
	case ReconciliationDelivered:
		return ReconcilingSettled
	case ReconciliationNotSent:
		return ReconcilingRetryable
	}
	return ReconcilingAbsent
}

func classifyReconciling(effect EffectState, ledgerFound bool, reconciliation ReconciliationState, reconciliationFound bool) (ReconcilingState, bool, string) {
	ledgerState := ReconcilingAbsent
	reconciliationState := ReconcilingAbsent
	effectLabel := "none"
	reconciliationLabel := "none"
	if ledgerFound {
		ledgerState = ledgerReconcilingState(effect)
		effectLabel = string(effect)
	}
	if reconciliationFound {
		reconciliationState = reconciliationReconcilingState(reconciliation)
		reconciliationLabel = string(reconciliation)
	}
	conflict := ledgerFound && reconciliationFound &&
		((ledgerState == ReconcilingSettled && reconciliationState == ReconcilingRetryable) ||
			(ledgerState == ReconcilingRetryable && reconciliationState == ReconcilingSettled))
	state := ledgerState
	if reconciliationState.severity() > state.severity() {
		state = reconciliationState
	}
	if conflict {
		state = ReconcilingBlocked
	}
	reason := fmt.Sprintf("ledger=%s reconciliation=%s", effectLabel, reconciliationLabel)
	if conflict {
		reason = fmt.Sprintf("%s conflict=ledger 与核对账结论相反", reason)
	}
	return state, conflict, reason
}

func (l *InvocationLedger) DescribeReconciliation(ctx context.Context, scope LedgerScope, idempotencyKey, operationID string) (ReconcileSnapshot, error) {
	scopeKey, err := scope.Key()
	if err != nil {
		return ReconcileSnapshot{}, errors.Join(ErrReconcileQueryInvalid, err)
	}
	key := strings.TrimSpace(idempotencyKey)
	operation := strings.TrimSpace(operationID)
	if key == "" && operation == "" {
		return ReconcileSnapshot{}, fmt.Errorf("%w: idempotencyKey 与 operationId 至少要提供一个", ErrReconcileQueryInvalid)
	}
	if len(key) > 512 {
		return ReconcileSnapshot{}, fmt.Errorf("%w: idempotencyKey 不能超过 512 字节", ErrReconcileQueryInvalid)
	}
	if len(operation) > 512 {
		return ReconcileSnapshot{}, fmt.Errorf("%w: operationId 不能超过 512 字节", ErrReconcileQueryInvalid)
	}
	if err := ctx.Err(); err != nil {
		return ReconcileSnapshot{}, err
	}
	snapshot := ReconcileSnapshot{
		ScopeKey:       scopeKey,
		IdempotencyKey: key,
		OperationID:    operation,
	}
	if key != "" {
		entry, found, lookupErr := l.Lookup(ctx, scope, key)
		if lookupErr != nil {
			return ReconcileSnapshot{}, lookupErr
		}
		snapshot.LedgerFound = found
		if found {
			snapshot.Effect = entry.State
		}
	}
	if operation != "" {
		record, found, lookupErr := l.Reconciliation(ctx, scope, operation)
		if lookupErr != nil {
			return ReconcileSnapshot{}, lookupErr
		}
		snapshot.ReconciliationFound = found
		if found {
			snapshot.Reconciliation = record.State
			snapshot.Attempts = record.Attempts
		}
	}
	state, conflict, reason := classifyReconciling(snapshot.Effect, snapshot.LedgerFound, snapshot.Reconciliation, snapshot.ReconciliationFound)
	snapshot.State = state
	snapshot.Conflict = conflict
	snapshot.Reason = reason
	snapshot.AutoRetry = state.AutoRetryAllowed()
	snapshot.ManualReview = state.RequiresManualReview()
	snapshot.NextAction = state.NextAction()
	return snapshot, nil
}
