package executionjournal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const RecoverySchema = `CREATE TABLE IF NOT EXISTS kernel_invocation_recovery (
	scope_key TEXT NOT NULL,
	idempotency_key TEXT NOT NULL,
	invocation_id TEXT NOT NULL,
	state_before TEXT NOT NULL,
	decision TEXT NOT NULL CHECK(decision IN ('reconcile')),
	recovered_at TEXT NOT NULL,
	PRIMARY KEY(scope_key,idempotency_key))`

const RecoveryImmutableSchema = `CREATE TRIGGER IF NOT EXISTS kernel_invocation_recovery_immutable
BEFORE UPDATE ON kernel_invocation_recovery
BEGIN
	SELECT RAISE(ABORT, 'recovery record is immutable');
END`

const RecoveryRemovalSchema = `CREATE TRIGGER IF NOT EXISTS kernel_invocation_recovery_removal
BEFORE DELETE ON kernel_invocation_recovery
BEGIN
	SELECT RAISE(ABORT, 'recovery record is immutable');
END`

const recoveryImmutableMessage = "recovery record is immutable"

var ErrRecoveryRequired = errors.New("执行账本：该幂等键已被中断恢复接管，必须走人工核对，不得重新执行")

type RecoveryDecision string

const RecoveryReconcile RecoveryDecision = "reconcile"

func (d RecoveryDecision) Valid() bool {
	return d == RecoveryReconcile
}

type RecoveryRecord struct {
	ScopeKey       string
	IdempotencyKey string
	InvocationID   string
	StateBefore    EffectState
	Decision       RecoveryDecision
	RecoveredAt    time.Time
}

func (r RecoveryRecord) Reexecute() bool {
	return false
}

type RecoveryReport struct {
	Scanned    int
	Recovered  []RecoveryRecord
	Previously int
}

func (r RecoveryReport) ManualOnly() bool {
	for _, record := range r.Recovered {
		if record.Reexecute() {
			return false
		}
	}
	return true
}

type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func recoveryRecorded(ctx context.Context, q rowQuerier, scopeKey, key string) (bool, error) {
	var count int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(1) FROM kernel_invocation_recovery WHERE scope_key=? AND idempotency_key=?`, scopeKey, key).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (l *InvocationLedger) RecoverInFlight(ctx context.Context, scope LedgerScope, staleAfter time.Duration) (RecoveryReport, error) {
	scopeKey, err := scope.Key()
	if err != nil {
		return RecoveryReport{}, err
	}
	cutoff := time.Now().UTC()
	if staleAfter > 0 {
		cutoff = cutoff.Add(-staleAfter)
	}
	rows, err := l.db.QueryContext(ctx,
		`SELECT scope_key,idempotency_key,invocation_id,effect_state FROM kernel_invocation_ledger
WHERE scope_key=? AND effect_state IN ('pending','unknown') AND updated_at<=? ORDER BY idempotency_key`,
		scopeKey, cutoff.Format(time.RFC3339Nano))
	if err != nil {
		return RecoveryReport{}, fmt.Errorf("执行账本：扫描中断调用失败: %w", err)
	}
	report := RecoveryReport{}
	for rows.Next() {
		var record RecoveryRecord
		var state string
		if err := rows.Scan(&record.ScopeKey, &record.IdempotencyKey, &record.InvocationID, &state); err != nil {
			rows.Close()
			return RecoveryReport{}, err
		}
		record.StateBefore = EffectState(state)
		record.Decision = RecoveryReconcile
		report.Scanned++
		stored, inserted, err := l.recordRecovery(ctx, record)
		if err != nil {
			rows.Close()
			return RecoveryReport{}, err
		}
		if inserted {
			report.Recovered = append(report.Recovered, stored)
		} else {
			report.Previously++
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return RecoveryReport{}, err
	}
	rows.Close()
	return report, nil
}

func (l *InvocationLedger) recordRecovery(ctx context.Context, record RecoveryRecord) (RecoveryRecord, bool, error) {
	if !record.Decision.Valid() {
		return RecoveryRecord{}, false, fmt.Errorf("%w: decision=%q", ErrLedgerKeyInvalid, record.Decision)
	}
	now := time.Now().UTC()
	result, err := l.db.ExecContext(ctx,
		`INSERT INTO kernel_invocation_recovery(scope_key,idempotency_key,invocation_id,state_before,decision,recovered_at)
		 VALUES(?,?,?,?,?,?) ON CONFLICT(scope_key,idempotency_key) DO NOTHING`,
		record.ScopeKey, record.IdempotencyKey, record.InvocationID, string(record.StateBefore), string(record.Decision), now.Format(time.RFC3339Nano))
	if err != nil {
		return RecoveryRecord{}, false, fmt.Errorf("执行账本：登记中断恢复失败: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return RecoveryRecord{}, false, err
	}
	stored, found, err := l.recovery(ctx, record.ScopeKey, record.IdempotencyKey)
	if err != nil {
		return RecoveryRecord{}, false, err
	}
	if !found {
		return RecoveryRecord{}, false, fmt.Errorf("%w: scope=%q key=%q", ErrRecoveryRequired, record.ScopeKey, record.IdempotencyKey)
	}
	return stored, inserted == 1, nil
}

func (l *InvocationLedger) recovery(ctx context.Context, scopeKey, key string) (RecoveryRecord, bool, error) {
	var record RecoveryRecord
	var state, decision, recovered string
	err := l.db.QueryRowContext(ctx,
		`SELECT scope_key,idempotency_key,invocation_id,state_before,decision,recovered_at FROM kernel_invocation_recovery WHERE scope_key=? AND idempotency_key=?`,
		scopeKey, key).Scan(&record.ScopeKey, &record.IdempotencyKey, &record.InvocationID, &state, &decision, &recovered)
	if errors.Is(err, sql.ErrNoRows) {
		return RecoveryRecord{}, false, nil
	}
	if err != nil {
		return RecoveryRecord{}, false, err
	}
	record.StateBefore = EffectState(state)
	record.Decision = RecoveryDecision(decision)
	if parsed, err := time.Parse(time.RFC3339Nano, recovered); err == nil {
		record.RecoveredAt = parsed
	}
	return record, true, nil
}

func (l *InvocationLedger) InspectRecovery(ctx context.Context, scope LedgerScope, idempotencyKey string) (RecoveryRecord, bool, error) {
	scopeKey, err := scope.Key()
	if err != nil {
		return RecoveryRecord{}, false, err
	}
	key := strings.TrimSpace(idempotencyKey)
	if key == "" || len(key) > 512 {
		return RecoveryRecord{}, false, fmt.Errorf("%w: idempotencyKey 长度必须在 1..512", ErrLedgerKeyInvalid)
	}
	return l.recovery(ctx, scopeKey, key)
}

func IsRecoveryImmutableError(err error) bool {
	return err != nil && strings.Contains(err.Error(), recoveryImmutableMessage)
}
