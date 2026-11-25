package executionjournal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/execution"
)

const LedgerSchema = `CREATE TABLE IF NOT EXISTS kernel_invocation_ledger (
	scope_key TEXT NOT NULL,
	idempotency_key TEXT NOT NULL,
	invocation_id TEXT NOT NULL,
	capability_id TEXT NOT NULL,
	binding_revision TEXT NOT NULL,
	payload_hash TEXT NOT NULL,
	effect_state TEXT NOT NULL CHECK(effect_state IN ('pending','committed','rejected','unknown')),
	result BLOB,
	error_code TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL,
	PRIMARY KEY(scope_key,idempotency_key))`

const ledgerWaitTimeout = 5 * time.Second

var (
	ErrLedgerKeyInvalid      = errors.New("执行账本：作用域、幂等键或冻结版本非法")
	ErrLedgerPayloadConflict = errors.New("执行账本：同一幂等键的请求内容已变化，拒绝重复执行")
	ErrLedgerBindingFrozen   = errors.New("执行账本：同一幂等键绑定的版本已冻结，拒绝换版本重复执行")
	ErrLedgerEffectUnknown   = errors.New("执行账本：该幂等键的结果未知，必须先核查再决定是否重试")
	ErrLedgerRejected        = errors.New("执行账本：该幂等键已被确定拒绝")
)

type EffectState string

const (
	EffectPending   EffectState = "pending"
	EffectCommitted EffectState = "committed"
	EffectRejected  EffectState = "rejected"
	EffectUnknown   EffectState = "unknown"
)

func (s EffectState) Valid() bool {
	switch s {
	case EffectPending, EffectCommitted, EffectRejected, EffectUnknown:
		return true
	}
	return false
}

func (s EffectState) Terminal() bool {
	return s == EffectCommitted || s == EffectRejected
}

func (s EffectState) Ambiguous() bool {
	return s == EffectPending || s == EffectUnknown
}

type LedgerScope struct {
	SpaceID  string
	RoleID   string
	DeviceID string
	Surface  string
}

func (s LedgerScope) Key() (string, error) {
	parts := []string{s.SpaceID, s.RoleID, s.DeviceID, s.Surface}
	if strings.TrimSpace(s.SpaceID) == "" {
		return "", fmt.Errorf("%w: scope.spaceId 不能为空", ErrLedgerKeyInvalid)
	}
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
		if strings.ContainsRune(parts[index], '\x1f') || len(parts[index]) > 256 {
			return "", fmt.Errorf("%w: scope 第 %d 段非法", ErrLedgerKeyInvalid, index)
		}
	}
	return strings.Join(parts, "\x1f"), nil
}

type LedgerRequest struct {
	Scope LedgerScope

	IdempotencyKey string

	Trace execution.InvocationTrace

	CapabilityID string

	BindingRevision string

	PayloadHash string

	SideEffect execution.SideEffectClass
}

func (r LedgerRequest) validate() (string, execution.InvocationContract, error) {
	key := strings.TrimSpace(r.IdempotencyKey)
	if key == "" || len(key) > 512 {
		return "", execution.InvocationContract{}, fmt.Errorf("%w: idempotencyKey 长度必须在 1..512", ErrLedgerKeyInvalid)
	}
	if strings.TrimSpace(r.BindingRevision) == "" || len(r.BindingRevision) > 256 {
		return "", execution.InvocationContract{}, fmt.Errorf("%w: bindingRevision 必须冻结且不超过 256 字节", ErrLedgerKeyInvalid)
	}
	payload := strings.TrimSpace(r.PayloadHash)
	if payload == "" || len(payload) > 128 {
		return "", execution.InvocationContract{}, fmt.Errorf("%w: payloadHash 长度必须在 1..128", ErrLedgerKeyInvalid)
	}
	scopeKey, err := r.Scope.Key()
	if err != nil {
		return "", execution.InvocationContract{}, err
	}
	contract, err := execution.NewInvocationContract(r.Trace, r.CapabilityID, r.SideEffect, key)
	if err != nil {
		return "", execution.InvocationContract{}, errors.Join(ErrLedgerKeyInvalid, err)
	}
	return scopeKey, contract, nil
}

type LedgerEntry struct {
	ScopeKey        string
	IdempotencyKey  string
	InvocationID    string
	CapabilityID    string
	BindingRevision string
	PayloadHash     string
	State           EffectState
	Result          json.RawMessage
	ErrorCode       string
	UpdatedAt       time.Time
}

type InvocationLedger struct{ db *sql.DB }

func NewLedger(db *sql.DB) (*InvocationLedger, error) {
	if db == nil {
		return nil, errors.New("执行账本：数据库连接不能为空")
	}
	if _, err := db.Exec(LedgerSchema); err != nil {
		return nil, fmt.Errorf("执行账本：创建表失败: %w", err)
	}
	if _, err := db.Exec(EffectTokenSchema); err != nil {
		return nil, fmt.Errorf("执行账本：创建效果凭据表失败: %w", err)
	}
	if _, err := db.Exec(EffectTokenImmutableIdentitySchema); err != nil {
		return nil, fmt.Errorf("执行账本：创建效果凭据身份不可篡改约束失败: %w", err)
	}
	if _, err := db.Exec(EffectTokenImmutableRemovalSchema); err != nil {
		return nil, fmt.Errorf("执行账本：创建效果凭据不可删除约束失败: %w", err)
	}
	if _, err := db.Exec(ReconciliationSchema); err != nil {
		return nil, fmt.Errorf("执行账本：创建人工核对表失败: %w", err)
	}
	if _, err := db.Exec(RecoverySchema); err != nil {
		return nil, fmt.Errorf("执行账本：创建中断恢复表失败: %w", err)
	}
	if _, err := db.Exec(RecoveryImmutableSchema); err != nil {
		return nil, fmt.Errorf("执行账本：创建中断恢复不可篡改约束失败: %w", err)
	}
	if _, err := db.Exec(RecoveryRemovalSchema); err != nil {
		return nil, fmt.Errorf("执行账本：创建中断恢复不可删除约束失败: %w", err)
	}
	if _, err := db.Exec(EffectProofSchema); err != nil {
		return nil, fmt.Errorf("执行账本：创建效果证明表失败: %w", err)
	}
	if _, err := db.Exec(EffectProofImmutableSchema); err != nil {
		return nil, fmt.Errorf("执行账本：创建效果证明不可篡改约束失败: %w", err)
	}
	if _, err := db.Exec(EffectProofRemovalSchema); err != nil {
		return nil, fmt.Errorf("执行账本：创建效果证明不可删除约束失败: %w", err)
	}
	return &InvocationLedger{db: db}, nil
}

func (l *InvocationLedger) Lookup(ctx context.Context, scope LedgerScope, idempotencyKey string) (LedgerEntry, bool, error) {
	scopeKey, err := scope.Key()
	if err != nil {
		return LedgerEntry{}, false, err
	}
	key := strings.TrimSpace(idempotencyKey)
	if key == "" || len(key) > 512 {
		return LedgerEntry{}, false, fmt.Errorf("%w: idempotencyKey 长度必须在 1..512", ErrLedgerKeyInvalid)
	}
	entry, err := scanLedgerEntry(l.db.QueryRowContext(ctx, ledgerSelect+` WHERE scope_key=? AND idempotency_key=?`, scopeKey, key))
	if errors.Is(err, sql.ErrNoRows) {
		return LedgerEntry{}, false, nil
	}
	if err != nil {
		return LedgerEntry{}, false, err
	}
	return entry, true, nil
}

func (l *InvocationLedger) Commit(ctx context.Context, request LedgerRequest, run func() (json.RawMessage, error)) (LedgerEntry, bool, error) {
	scopeKey, contract, err := request.validate()
	if err != nil {
		return LedgerEntry{}, false, err
	}
	key := strings.TrimSpace(request.IdempotencyKey)
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return LedgerEntry{}, false, err
	}
	defer tx.Rollback()
	inserted, err := tx.ExecContext(ctx,
		`INSERT INTO kernel_invocation_ledger(scope_key,idempotency_key,invocation_id,capability_id,binding_revision,payload_hash,effect_state,result,error_code,updated_at)
		 VALUES(?,?,?,?,?,?,?,NULL,'',?) ON CONFLICT(scope_key,idempotency_key) DO NOTHING`,
		scopeKey, key, contract.Trace.InvocationID, contract.CapabilityID, strings.TrimSpace(request.BindingRevision), strings.TrimSpace(request.PayloadHash), string(EffectPending), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return LedgerEntry{}, false, err
	}
	entry, err := scanLedgerEntry(tx.QueryRowContext(ctx, ledgerSelect+` WHERE scope_key=? AND idempotency_key=?`, scopeKey, key))
	if err != nil {
		return LedgerEntry{}, false, err
	}
	if entry.InvocationID != strings.TrimSpace(contract.Trace.InvocationID) {
		return LedgerEntry{}, false, fmt.Errorf("%w: %s requestedInvocation=%q", ErrLedgerInvocationMismatch, entry.Binding(), strings.TrimSpace(contract.Trace.InvocationID))
	}
	if entry.PayloadHash != strings.TrimSpace(request.PayloadHash) {
		return LedgerEntry{}, false, fmt.Errorf("%w: scope=%q key=%q recorded=%s requested=%s", ErrLedgerPayloadConflict, scopeKey, key, entry.PayloadHash, request.PayloadHash)
	}
	if entry.BindingRevision != strings.TrimSpace(request.BindingRevision) {
		return LedgerEntry{}, false, fmt.Errorf("%w: scope=%q key=%q frozen=%s requested=%s", ErrLedgerBindingFrozen, scopeKey, key, entry.BindingRevision, request.BindingRevision)
	}
	if entry.State != EffectPending {
		return entry, false, ledgerStateError(entry)
	}
	recorded, err := recoveryRecorded(ctx, tx, scopeKey, key)
	if err != nil {
		return LedgerEntry{}, false, err
	}
	if recorded {
		return LedgerEntry{}, false, fmt.Errorf("%w: %s", ErrRecoveryRequired, entry.Binding())
	}
	changed, err := inserted.RowsAffected()
	if err != nil {
		return LedgerEntry{}, false, err
	}
	if changed == 0 {
		if err := tx.Commit(); err != nil {
			return LedgerEntry{}, false, err
		}
		return l.awaitTerminal(ctx, scopeKey, key)
	}
	if err := tx.Commit(); err != nil {
		return LedgerEntry{}, false, err
	}
	return l.runAndRecord(ctx, request, scopeKey, key, contract, run)
}

func (l *InvocationLedger) awaitTerminal(ctx context.Context, scopeKey, key string) (LedgerEntry, bool, error) {
	deadline := time.Now().Add(ledgerWaitTimeout)
	ticker := time.NewTicker(2 * time.Millisecond)
	defer ticker.Stop()
	for {
		entry, found, err := l.lookupExact(ctx, scopeKey, key)
		if err != nil {
			return LedgerEntry{}, false, err
		}
		if !found {
			return LedgerEntry{}, false, fmt.Errorf("%w: scope=%q key=%q 预留已被释放", ErrLedgerEffectUnknown, scopeKey, key)
		}
		if entry.State != EffectPending {
			return entry, false, ledgerStateError(entry)
		}
		if time.Now().After(deadline) {
			return LedgerEntry{}, false, fmt.Errorf("%w: scope=%q key=%q 等待超时", ErrLedgerEffectUnknown, scopeKey, key)
		}
		select {
		case <-ctx.Done():
			return LedgerEntry{}, false, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (l *InvocationLedger) runAndRecord(ctx context.Context, request LedgerRequest, scopeKey, key string, contract execution.InvocationContract, run func() (json.RawMessage, error)) (LedgerEntry, bool, error) {
	bookkeeping := context.WithoutCancel(ctx)
	result, runErr := guardRun(run)
	if runErr == nil {
		if len(result) > 1<<20 {
			runErr = errors.New("执行账本：结果超过 1 MiB 上限")
		} else {
			updated, err := l.db.ExecContext(bookkeeping,
				`UPDATE kernel_invocation_ledger SET effect_state=?,result=?,error_code='',updated_at=? WHERE scope_key=? AND idempotency_key=? AND effect_state='pending'`,
				string(EffectCommitted), []byte(result), time.Now().UTC().Format(time.RFC3339Nano), scopeKey, key)
			if err != nil {
				return LedgerEntry{}, true, err
			}
			changed, err := updated.RowsAffected()
			if err != nil {
				return LedgerEntry{}, true, err
			}
			if changed != 1 {
				return LedgerEntry{}, true, fmt.Errorf("%w: scope=%q key=%q 提交状态被外部改动", ErrLedgerEffectUnknown, scopeKey, key)
			}
			entry, _, err := l.lookupExact(bookkeeping, scopeKey, key)
			return entry, true, err
		}
	}
	outcome, classified := execution.ClassifyInvocationOutcome(contract, true, runErr)
	if outcome == execution.InvocationOutcomeUnknown {
		if _, err := l.db.ExecContext(bookkeeping,
			`UPDATE kernel_invocation_ledger SET effect_state=?,error_code='unknown',updated_at=? WHERE scope_key=? AND idempotency_key=? AND effect_state='pending'`,
			string(EffectUnknown), time.Now().UTC().Format(time.RFC3339Nano), scopeKey, key); err != nil {
			return LedgerEntry{}, true, errors.Join(classified, err)
		}
		entry, _, lookupErr := l.lookupExact(bookkeeping, scopeKey, key)
		if lookupErr != nil {
			return LedgerEntry{}, true, errors.Join(classified, lookupErr)
		}
		return entry, true, classified
	}
	if _, err := l.db.ExecContext(bookkeeping, `DELETE FROM kernel_invocation_ledger WHERE scope_key=? AND idempotency_key=? AND effect_state='pending'`, scopeKey, key); err != nil {
		return LedgerEntry{}, true, errors.Join(runErr, err)
	}
	return LedgerEntry{}, true, runErr
}

func ledgerStateError(entry LedgerEntry) error {
	switch entry.State {
	case EffectPending:
		return fmt.Errorf("%w: scope=%q key=%q 仍处于执行中", ErrLedgerEffectUnknown, entry.ScopeKey, entry.IdempotencyKey)
	case EffectUnknown:
		return fmt.Errorf("%w: scope=%q key=%q invocation=%s", ErrLedgerEffectUnknown, entry.ScopeKey, entry.IdempotencyKey, entry.InvocationID)
	case EffectRejected:
		return fmt.Errorf("%w: scope=%q key=%q code=%s", ErrLedgerRejected, entry.ScopeKey, entry.IdempotencyKey, entry.ErrorCode)
	}
	return nil
}

const ledgerSelect = `SELECT scope_key,idempotency_key,invocation_id,capability_id,binding_revision,payload_hash,effect_state,result,error_code,updated_at FROM kernel_invocation_ledger`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanLedgerEntry(row rowScanner) (LedgerEntry, error) {
	var entry LedgerEntry
	var state string
	var result []byte
	var updatedAt string
	if err := row.Scan(&entry.ScopeKey, &entry.IdempotencyKey, &entry.InvocationID, &entry.CapabilityID, &entry.BindingRevision, &entry.PayloadHash, &state, &result, &entry.ErrorCode, &updatedAt); err != nil {
		return LedgerEntry{}, err
	}
	entry.State = EffectState(state)
	if !entry.State.Valid() {
		return LedgerEntry{}, fmt.Errorf("%w: 记录 %q 的效果状态 %q 非法", ErrLedgerEffectUnknown, entry.ScopeKey, state)
	}
	if result != nil {
		entry.Result = json.RawMessage(result)
	}
	parsed, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return LedgerEntry{}, fmt.Errorf("执行账本：updated_at %q 无法解析: %w", updatedAt, err)
	}
	entry.UpdatedAt = parsed
	return entry, nil
}

func (l *InvocationLedger) lookupExact(ctx context.Context, scopeKey, key string) (LedgerEntry, bool, error) {
	entry, err := scanLedgerEntry(l.db.QueryRowContext(ctx, ledgerSelect+` WHERE scope_key=? AND idempotency_key=?`, scopeKey, key))
	if errors.Is(err, sql.ErrNoRows) {
		return LedgerEntry{}, false, nil
	}
	if err != nil {
		return LedgerEntry{}, false, err
	}
	return entry, true, nil
}
