package executionjournal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/execution"
)

const ReconciliationSchema = `CREATE TABLE IF NOT EXISTS kernel_irreversible_reconciliation (
	scope_key TEXT NOT NULL,
	operation_id TEXT NOT NULL,
	invocation_id TEXT NOT NULL,
	capability_id TEXT NOT NULL,
	provider_id TEXT NOT NULL,
	side_effect TEXT NOT NULL CHECK(side_effect IN ('write','external')),
	payload_hash TEXT NOT NULL,
	state TEXT NOT NULL CHECK(state IN ('attempting','needs_reconciliation','delivered','not_delivered')),
	attempts INTEGER NOT NULL,
	reason TEXT NOT NULL DEFAULT '',
	last_error TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	resolved_by TEXT NOT NULL DEFAULT '',
	resolution_note TEXT NOT NULL DEFAULT '',
	resolved_at TEXT NOT NULL DEFAULT '',
	PRIMARY KEY(scope_key,operation_id))`

var (
	ErrReconciliationKeyInvalid     = errors.New("人工核对：作用域、操作编号或调用标识非法")
	ErrReconciliationPending        = errors.New("人工核对：该操作已发起但结果未核实，禁止再次发送")
	ErrReconciliationDelivered      = errors.New("人工核对：该操作已核实为已送达，禁止再次发送")
	ErrReconciliationResolved       = errors.New("人工核对：该操作的核对状态不可再次改写")
	ErrReconciliationVerdictInvalid = errors.New("人工核对：结论、操作人或备注非法")
	ErrReconciliationNotFound       = errors.New("人工核对：找不到对应的操作记录")
)

type ReconciliationState string

const (
	ReconciliationAttempting ReconciliationState = "attempting"
	ReconciliationNeedsCheck ReconciliationState = "needs_reconciliation"
	ReconciliationDelivered  ReconciliationState = "delivered"
	ReconciliationNotSent    ReconciliationState = "not_delivered"
)

func (s ReconciliationState) Valid() bool {
	switch s {
	case ReconciliationAttempting, ReconciliationNeedsCheck, ReconciliationDelivered, ReconciliationNotSent:
		return true
	}
	return false
}

func (s ReconciliationState) Open() bool {
	return s == ReconciliationAttempting || s == ReconciliationNeedsCheck
}

func (s ReconciliationState) RetryAllowed() bool {
	return s == ReconciliationNotSent
}

type ReconciliationVerdict string

const (
	VerdictDelivered ReconciliationVerdict = "delivered"
	VerdictNotSent   ReconciliationVerdict = "not_delivered"
)

func (v ReconciliationVerdict) Valid() bool {
	return v == VerdictDelivered || v == VerdictNotSent
}

type IrreversibleRequest struct {
	Scope LedgerScope

	OperationID string

	Trace execution.InvocationTrace

	CapabilityID string

	ProviderID string

	SideEffect execution.SideEffectClass

	PayloadHash string
}

func (r IrreversibleRequest) validate() (string, error) {
	scopeKey, err := r.Scope.Key()
	if err != nil {
		return "", errors.Join(ErrReconciliationKeyInvalid, err)
	}
	operation := strings.TrimSpace(r.OperationID)
	if operation == "" || len(operation) > 512 {
		return "", fmt.Errorf("%w: operationId 长度必须在 1..512", ErrReconciliationKeyInvalid)
	}
	if strings.ContainsRune(operation, '\x1f') {
		return "", fmt.Errorf("%w: operationId 不能包含分隔符", ErrReconciliationKeyInvalid)
	}
	if err := r.Trace.Validate(); err != nil {
		return "", errors.Join(ErrReconciliationKeyInvalid, err)
	}
	if strings.TrimSpace(r.CapabilityID) == "" || len(strings.TrimSpace(r.CapabilityID)) > 256 {
		return "", fmt.Errorf("%w: capabilityId 长度必须在 1..256", ErrReconciliationKeyInvalid)
	}
	if strings.TrimSpace(r.ProviderID) == "" || len(strings.TrimSpace(r.ProviderID)) > 256 {
		return "", fmt.Errorf("%w: providerId 长度必须在 1..256", ErrReconciliationKeyInvalid)
	}
	if !r.SideEffect.Mutating() {
		return "", fmt.Errorf("%w: sideEffect=%q 不是不可逆写", ErrReconciliationKeyInvalid, r.SideEffect)
	}
	payload := strings.TrimSpace(r.PayloadHash)
	if payload == "" || len(payload) > 128 {
		return "", fmt.Errorf("%w: payloadHash 长度必须在 1..128", ErrReconciliationKeyInvalid)
	}
	return scopeKey, nil
}

type ReconciliationCase struct {
	ScopeKey     string
	OperationID  string
	InvocationID string
	CapabilityID string
	ProviderID   string
	SideEffect   execution.SideEffectClass
	PayloadHash  string
	State        ReconciliationState
	Attempts     int
	Reason       string
	LastError    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ResolvedBy   string
	Resolution   string
	ResolvedAt   time.Time
}

func (c ReconciliationCase) NeedsHumanCheck() bool { return c.State.Open() }

func (c ReconciliationCase) RetryAllowed() bool { return c.State.RetryAllowed() }

const reconciliationSelect = `SELECT scope_key,operation_id,invocation_id,capability_id,provider_id,side_effect,payload_hash,state,attempts,reason,last_error,created_at,updated_at,resolved_by,resolution_note,resolved_at FROM kernel_irreversible_reconciliation`

func scanReconciliationCase(row rowScanner) (ReconciliationCase, error) {
	var record ReconciliationCase
	var sideEffect, state, createdAt, updatedAt, resolvedAt string
	if err := row.Scan(&record.ScopeKey, &record.OperationID, &record.InvocationID, &record.CapabilityID, &record.ProviderID, &sideEffect, &record.PayloadHash, &state, &record.Attempts, &record.Reason, &record.LastError, &createdAt, &updatedAt, &record.ResolvedBy, &record.Resolution, &resolvedAt); err != nil {
		return ReconciliationCase{}, err
	}
	record.SideEffect = execution.SideEffectClass(sideEffect)
	if !record.SideEffect.Mutating() {
		return ReconciliationCase{}, fmt.Errorf("%w: 记录 %q 的副作用 %q 非法", ErrReconciliationKeyInvalid, record.OperationID, sideEffect)
	}
	record.State = ReconciliationState(state)
	if !record.State.Valid() {
		return ReconciliationCase{}, fmt.Errorf("%w: 记录 %q 的核对状态 %q 非法", ErrReconciliationKeyInvalid, record.OperationID, state)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return ReconciliationCase{}, fmt.Errorf("人工核对：created_at %q 无法解析: %w", createdAt, err)
	}
	record.CreatedAt = parsedCreated
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return ReconciliationCase{}, fmt.Errorf("人工核对：updated_at %q 无法解析: %w", updatedAt, err)
	}
	record.UpdatedAt = parsedUpdated
	if resolvedAt != "" {
		parsedResolved, err := time.Parse(time.RFC3339Nano, resolvedAt)
		if err != nil {
			return ReconciliationCase{}, fmt.Errorf("人工核对：resolved_at %q 无法解析: %w", resolvedAt, err)
		}
		record.ResolvedAt = parsedResolved
	}
	return record, nil
}

type reconciliationQuery interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func lookupReconciliationCase(ctx context.Context, query reconciliationQuery, scopeKey, operationID string) (ReconciliationCase, bool, error) {
	record, err := scanReconciliationCase(query.QueryRowContext(ctx, reconciliationSelect+` WHERE scope_key=? AND operation_id=?`, scopeKey, operationID))
	if errors.Is(err, sql.ErrNoRows) {
		return ReconciliationCase{}, false, nil
	}
	if err != nil {
		return ReconciliationCase{}, false, err
	}
	return record, true, nil
}

func (l *InvocationLedger) Reconciliation(ctx context.Context, scope LedgerScope, operationID string) (ReconciliationCase, bool, error) {
	scopeKey, err := scope.Key()
	if err != nil {
		return ReconciliationCase{}, false, err
	}
	operation := strings.TrimSpace(operationID)
	if operation == "" || len(operation) > 512 {
		return ReconciliationCase{}, false, fmt.Errorf("%w: operationId 长度必须在 1..512", ErrReconciliationKeyInvalid)
	}
	return lookupReconciliationCase(ctx, l.db, scopeKey, operation)
}

func (l *InvocationLedger) OpenReconciliations(ctx context.Context, scope *LedgerScope) ([]ReconciliationCase, error) {
	query := reconciliationSelect + ` WHERE state IN ('attempting','needs_reconciliation')`
	args := make([]any, 0, 1)
	if scope != nil {
		scopeKey, err := scope.Key()
		if err != nil {
			return nil, err
		}
		query += ` AND scope_key=?`
		args = append(args, scopeKey)
	}
	query += ` ORDER BY created_at,scope_key,operation_id`
	rows, err := l.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cases := make([]ReconciliationCase, 0, 4)
	for rows.Next() {
		record, err := scanReconciliationCase(rows)
		if err != nil {
			return nil, err
		}
		cases = append(cases, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cases, nil
}

func (l *InvocationLedger) ReserveIrreversible(ctx context.Context, request IrreversibleRequest) (ReconciliationCase, error) {
	scopeKey, err := request.validate()
	if err != nil {
		return ReconciliationCase{}, err
	}
	operation := strings.TrimSpace(request.OperationID)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return ReconciliationCase{}, err
	}
	defer tx.Rollback()
	existing, found, err := lookupReconciliationCase(ctx, tx, scopeKey, operation)
	if err != nil {
		return ReconciliationCase{}, err
	}
	if !found {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO kernel_irreversible_reconciliation(scope_key,operation_id,invocation_id,capability_id,provider_id,side_effect,payload_hash,state,attempts,reason,last_error,created_at,updated_at,resolved_by,resolution_note,resolved_at)
			 VALUES(?,?,?,?,?,?,?,?,1,'','',?,?,'','','')`,
			scopeKey, operation, request.Trace.InvocationID, strings.TrimSpace(request.CapabilityID), strings.TrimSpace(request.ProviderID), string(request.SideEffect), strings.TrimSpace(request.PayloadHash), string(ReconciliationAttempting), now, now); err != nil {
			current, present, lookupErr := lookupReconciliationCase(ctx, tx, scopeKey, operation)
			if lookupErr == nil && present && current.State.Open() {
				return current, fmt.Errorf("%w: scope=%q operation=%q state=%s", ErrReconciliationPending, scopeKey, operation, current.State)
			}
			return ReconciliationCase{}, err
		}
		if err := tx.Commit(); err != nil {
			return ReconciliationCase{}, err
		}
		record, _, err := lookupReconciliationCase(ctx, l.db, scopeKey, operation)
		if err != nil {
			return ReconciliationCase{}, err
		}
		return record, nil
	}
	switch existing.State {
	case ReconciliationAttempting, ReconciliationNeedsCheck:
		return existing, fmt.Errorf("%w: scope=%q operation=%q state=%s", ErrReconciliationPending, scopeKey, operation, existing.State)
	case ReconciliationDelivered:
		return existing, fmt.Errorf("%w: scope=%q operation=%q", ErrReconciliationDelivered, scopeKey, operation)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE kernel_irreversible_reconciliation SET state=?,attempts=attempts+1,reason='',last_error='',updated_at=?,resolved_by='',resolution_note='',resolved_at='' WHERE scope_key=? AND operation_id=? AND state=?`,
		string(ReconciliationAttempting), now, scopeKey, operation, string(ReconciliationNotSent)); err != nil {
		return ReconciliationCase{}, err
	}
	if err := tx.Commit(); err != nil {
		return ReconciliationCase{}, err
	}
	record, _, err := lookupReconciliationCase(ctx, l.db, scopeKey, operation)
	if err != nil {
		return ReconciliationCase{}, err
	}
	return record, nil
}

func (l *InvocationLedger) DispatchIrreversible(ctx context.Context, request IrreversibleRequest, dispatch func() error) (ReconciliationCase, error) {
	record, err := l.ReserveIrreversible(ctx, request)
	if err != nil {
		return record, err
	}
	dispatchErr := guardDispatch(dispatch)
	bookkeeping := context.WithoutCancel(ctx)
	scopeKey, _ := request.Scope.Key()
	operation := strings.TrimSpace(request.OperationID)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	state := ReconciliationDelivered
	reason := ""
	lastError := ""
	if dispatchErr != nil {
		lastError = dispatchErr.Error()
		state = ReconciliationNotSent
		reason = "已确认未送达，允许重新发送"
		if execution.IsAmbiguousInvocationFailure(dispatchErr) {
			state = ReconciliationNeedsCheck
			reason = "回执丢失或链路中断，需人工核对是否已送达"
		}
	}
	if _, err := l.db.ExecContext(bookkeeping,
		`UPDATE kernel_irreversible_reconciliation SET state=?,reason=?,last_error=?,updated_at=? WHERE scope_key=? AND operation_id=? AND state=?`,
		string(state), reason, lastError, now, scopeKey, operation, string(ReconciliationAttempting)); err != nil {
		return record, errors.Join(dispatchErr, err)
	}
	settled, _, lookupErr := lookupReconciliationCase(bookkeeping, l.db, scopeKey, operation)
	if lookupErr != nil {
		return record, errors.Join(dispatchErr, lookupErr)
	}
	if dispatchErr == nil {
		return settled, nil
	}
	if state == ReconciliationNeedsCheck {
		return settled, errors.Join(dispatchErr, fmt.Errorf("%w: scope=%q operation=%q", ErrReconciliationPending, scopeKey, operation))
	}
	return settled, dispatchErr
}

func (l *InvocationLedger) ResolveReconciliation(ctx context.Context, scope LedgerScope, operationID string, verdict ReconciliationVerdict, operator, note string) (ReconciliationCase, error) {
	scopeKey, err := scope.Key()
	if err != nil {
		return ReconciliationCase{}, err
	}
	operation := strings.TrimSpace(operationID)
	if operation == "" || len(operation) > 512 {
		return ReconciliationCase{}, fmt.Errorf("%w: operationId 长度必须在 1..512", ErrReconciliationKeyInvalid)
	}
	if !verdict.Valid() {
		return ReconciliationCase{}, fmt.Errorf("%w: verdict=%q", ErrReconciliationVerdictInvalid, verdict)
	}
	who := strings.TrimSpace(operator)
	if who == "" || len(who) > 128 {
		return ReconciliationCase{}, fmt.Errorf("%w: operator 长度必须在 1..128", ErrReconciliationVerdictInvalid)
	}
	if len(note) > 1024 {
		return ReconciliationCase{}, fmt.Errorf("%w: note 不能超过 1024 字节", ErrReconciliationVerdictInvalid)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	updated, err := l.db.ExecContext(ctx,
		`UPDATE kernel_irreversible_reconciliation SET state=?,resolved_by=?,resolution_note=?,resolved_at=?,updated_at=? WHERE scope_key=? AND operation_id=? AND state=?`,
		string(verdict), who, strings.TrimSpace(note), now, now, scopeKey, operation, string(ReconciliationNeedsCheck))
	if err != nil {
		return ReconciliationCase{}, err
	}
	changed, err := updated.RowsAffected()
	if err != nil {
		return ReconciliationCase{}, err
	}
	if changed != 1 {
		existing, found, lookupErr := lookupReconciliationCase(ctx, l.db, scopeKey, operation)
		if lookupErr != nil {
			return ReconciliationCase{}, lookupErr
		}
		if !found {
			return ReconciliationCase{}, fmt.Errorf("%w: scope=%q operation=%q", ErrReconciliationNotFound, scopeKey, operation)
		}
		return existing, fmt.Errorf("%w: scope=%q operation=%q state=%s", ErrReconciliationResolved, scopeKey, operation, existing.State)
	}
	record, _, err := lookupReconciliationCase(ctx, l.db, scopeKey, operation)
	if err != nil {
		return ReconciliationCase{}, err
	}
	return record, nil
}

func (l *InvocationLedger) RecoverInterrupted(ctx context.Context) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	updated, err := l.db.ExecContext(ctx,
		`UPDATE kernel_irreversible_reconciliation SET state=?,reason=?,updated_at=? WHERE state=?`,
		string(ReconciliationNeedsCheck), "进程重启前未取得回执，需人工核对是否已送达", now, string(ReconciliationAttempting))
	if err != nil {
		return 0, err
	}
	return updated.RowsAffected()
}
