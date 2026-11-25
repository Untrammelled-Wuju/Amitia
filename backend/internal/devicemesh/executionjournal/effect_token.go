package executionjournal

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const EffectTokenSchema = `CREATE TABLE IF NOT EXISTS kernel_invocation_effect (
	scope_key TEXT NOT NULL,
	idempotency_key TEXT NOT NULL,
	attempt INTEGER NOT NULL,
	salt TEXT NOT NULL,
	token TEXT NOT NULL,
	state TEXT NOT NULL CHECK(state IN ('issued','redeemed','abandoned')),
	created_at TEXT NOT NULL,
	redeemed_at TEXT NOT NULL DEFAULT '',
	PRIMARY KEY(scope_key,idempotency_key,attempt))`

const (
	effectTokenPrefix      = "v1"
	effectTokenSaltBytes   = 32
	effectTokenIssueTries  = 5
	effectTokenBusinessSep = "\x1e"
)

var (
	ErrEffectTokenInvalid   = errors.New("执行账本：效果凭据非法或与本次调用不匹配")
	ErrEffectRecordConflict = errors.New("执行账本：权威效果记录与本请求不一致，拒绝复用")
)

type EffectToken struct {
	Value   string
	Attempt int
}

func (t EffectToken) String() string { return t.Value }

type EffectRecord struct {
	BusinessKey string
	PayloadHash string
	Token       string
	Result      json.RawMessage
}

type EffectAuthority interface {
	LookupEffect(ctx context.Context, businessKey string) (EffectRecord, bool, error)
}

type EffectResolution string

const (
	EffectResolvedCommitted    EffectResolution = "committed"
	EffectResolvedAbsent       EffectResolution = "absent"
	EffectResolvedRejected     EffectResolution = "rejected"
	EffectResolvedUndetermined EffectResolution = "unknown"
)

type Reconciliation struct {
	Resolution EffectResolution
	Entry      LedgerEntry
	Record     EffectRecord
}

func (r Reconciliation) RetrySafe() bool {
	return r.Resolution == EffectResolvedAbsent || r.Resolution == EffectResolvedRejected
}

func (r Reconciliation) replayForbidden() bool {
	return r.Resolution == EffectResolvedCommitted || r.Resolution == EffectResolvedUndetermined
}

func BusinessKey(scope LedgerScope, idempotencyKey string) (string, error) {
	scopeKey, err := scope.Key()
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(idempotencyKey)
	if key == "" || len(key) > 512 {
		return "", fmt.Errorf("%w: idempotencyKey 长度必须在 1..512", ErrLedgerKeyInvalid)
	}
	return scopeKey + effectTokenBusinessSep + key, nil
}

func deriveEffectToken(salt, scopeKey, key, payloadHash, bindingRevision string, attempt int) string {
	digest := sha256.New()
	digest.Write([]byte("amitia-effect-token-v1"))
	for _, part := range []string{salt, scopeKey, key, payloadHash, bindingRevision, strconv.Itoa(attempt)} {
		digest.Write([]byte{0x1f})
		digest.Write([]byte(part))
	}
	return effectTokenPrefix + "." + strconv.Itoa(attempt) + "." + hex.EncodeToString(digest.Sum(nil))
}

func parseEffectToken(value string) (int, error) {
	parts := strings.Split(strings.TrimSpace(value), ".")
	if len(parts) != 3 || parts[0] != effectTokenPrefix || len(parts[2]) != 64 {
		return 0, fmt.Errorf("%w: token=%q 结构非法", ErrEffectTokenInvalid, value)
	}
	attempt, err := strconv.Atoi(parts[1])
	if err != nil || attempt < 1 {
		return 0, fmt.Errorf("%w: token=%q 的尝试序号非法", ErrEffectTokenInvalid, value)
	}
	if _, err := hex.DecodeString(parts[2]); err != nil {
		return 0, fmt.Errorf("%w: token=%q 的摘要不是十六进制", ErrEffectTokenInvalid, value)
	}
	return attempt, nil
}

func (l *InvocationLedger) IssueEffectToken(ctx context.Context, request LedgerRequest) (EffectToken, error) {
	scopeKey, _, err := request.validate()
	if err != nil {
		return EffectToken{}, err
	}
	if !request.SideEffect.Mutating() {
		return EffectToken{}, fmt.Errorf("%w: sideEffect=%s 不产生需要核对的业务效果", ErrEffectTokenInvalid, request.SideEffect)
	}
	key := strings.TrimSpace(request.IdempotencyKey)
	salt := make([]byte, effectTokenSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return EffectToken{}, fmt.Errorf("执行账本：生成效果凭据盐失败: %w", err)
	}
	encodedSalt := hex.EncodeToString(salt)
	for try := 0; try < effectTokenIssueTries; try++ {
		var attempt int
		if err := l.db.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(attempt),0)+1 FROM kernel_invocation_effect WHERE scope_key=? AND idempotency_key=?`,
			scopeKey, key).Scan(&attempt); err != nil {
			return EffectToken{}, fmt.Errorf("执行账本：分配效果凭据序号失败: %w", err)
		}
		value := deriveEffectToken(encodedSalt, scopeKey, key, strings.TrimSpace(request.PayloadHash), strings.TrimSpace(request.BindingRevision), attempt)
		inserted, err := l.db.ExecContext(ctx,
			`INSERT INTO kernel_invocation_effect(scope_key,idempotency_key,attempt,salt,token,state,created_at) VALUES(?,?,?,?,?,'issued',?) ON CONFLICT(scope_key,idempotency_key,attempt) DO NOTHING`,
			scopeKey, key, attempt, encodedSalt, value, time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			return EffectToken{}, fmt.Errorf("执行账本：登记效果凭据失败: %w", err)
		}
		changed, err := inserted.RowsAffected()
		if err != nil {
			return EffectToken{}, err
		}
		if changed == 1 {
			return EffectToken{Value: value, Attempt: attempt}, nil
		}
	}
	return EffectToken{}, fmt.Errorf("%w: scope=%q key=%q 无法分配尝试序号", ErrEffectTokenInvalid, scopeKey, key)
}

func (l *InvocationLedger) loadEffectToken(ctx context.Context, scopeKey, key string, attempt int) (salt, token, state string, err error) {
	err = l.db.QueryRowContext(ctx,
		`SELECT salt,token,state FROM kernel_invocation_effect WHERE scope_key=? AND idempotency_key=? AND attempt=?`,
		scopeKey, key, attempt).Scan(&salt, &token, &state)
	return salt, token, state, err
}

func (l *InvocationLedger) VerifyEffectToken(ctx context.Context, request LedgerRequest, token EffectToken) error {
	scopeKey, _, err := request.validate()
	if err != nil {
		return err
	}
	key := strings.TrimSpace(request.IdempotencyKey)
	attempt, err := parseEffectToken(token.Value)
	if err != nil {
		return err
	}
	if token.Attempt != 0 && token.Attempt != attempt {
		return fmt.Errorf("%w: token 序号=%d 与声明=%d 不一致", ErrEffectTokenInvalid, attempt, token.Attempt)
	}
	salt, stored, state, err := l.loadEffectToken(ctx, scopeKey, key, attempt)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: scope=%q key=%q attempt=%d 未签发", ErrEffectTokenInvalid, scopeKey, key, attempt)
	}
	if err != nil {
		return err
	}
	if state != "issued" {
		return fmt.Errorf("%w: scope=%q key=%q attempt=%d 凭据状态=%s 不可再用", ErrEffectTokenInvalid, scopeKey, key, attempt, state)
	}
	expected := deriveEffectToken(salt, scopeKey, key, strings.TrimSpace(request.PayloadHash), strings.TrimSpace(request.BindingRevision), attempt)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(token.Value)) != 1 || subtle.ConstantTimeCompare([]byte(stored), []byte(token.Value)) != 1 {
		return fmt.Errorf("%w: scope=%q key=%q attempt=%d 摘要不匹配", ErrEffectTokenInvalid, scopeKey, key, attempt)
	}
	return nil
}

func (l *InvocationLedger) verifyEffectEvidence(ctx context.Context, request LedgerRequest, value string) (int, error) {
	scopeKey, _, err := request.validate()
	if err != nil {
		return 0, err
	}
	key := strings.TrimSpace(request.IdempotencyKey)
	attempt, err := parseEffectToken(value)
	if err != nil {
		return 0, err
	}
	salt, stored, _, err := l.loadEffectToken(ctx, scopeKey, key, attempt)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("%w: 权威记录携带的凭据 scope=%q key=%q attempt=%d 非本账本签发", ErrEffectRecordConflict, scopeKey, key, attempt)
	}
	if err != nil {
		return 0, err
	}
	expected := deriveEffectToken(salt, scopeKey, key, strings.TrimSpace(request.PayloadHash), strings.TrimSpace(request.BindingRevision), attempt)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(value)) != 1 || subtle.ConstantTimeCompare([]byte(stored), []byte(value)) != 1 {
		return 0, fmt.Errorf("%w: 权威记录携带的凭据 attempt=%d 摘要不匹配", ErrEffectRecordConflict, attempt)
	}
	return attempt, nil
}

func (l *InvocationLedger) markEffectToken(ctx context.Context, scopeKey, key string, attempt int, from, to string) error {
	if attempt < 1 {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := l.db.ExecContext(ctx,
		`UPDATE kernel_invocation_effect SET state=?,redeemed_at=? WHERE scope_key=? AND idempotency_key=? AND attempt=? AND state=?`,
		to, now, scopeKey, key, attempt, from)
	if err != nil {
		return fmt.Errorf("执行账本：更新凭据状态 %s→%s 失败: %w", from, to, err)
	}
	if changed, err := result.RowsAffected(); err != nil {
		return err
	} else if changed != 1 {
		return fmt.Errorf("%w: scope=%q key=%q attempt=%d 凭据状态不是 %s", ErrEffectTokenInvalid, scopeKey, key, attempt, from)
	}
	return nil
}

func (l *InvocationLedger) Reconcile(ctx context.Context, request LedgerRequest, authority EffectAuthority) (Reconciliation, error) {
	scopeKey, _, err := request.validate()
	if err != nil {
		return Reconciliation{}, err
	}
	if authority == nil {
		return Reconciliation{}, fmt.Errorf("%w: 未提供权威效果来源，无法核对", ErrEffectTokenInvalid)
	}
	key := strings.TrimSpace(request.IdempotencyKey)
	businessKey, err := BusinessKey(request.Scope, key)
	if err != nil {
		return Reconciliation{}, err
	}
	pending, found, err := l.lookupExact(ctx, scopeKey, key)
	if err != nil {
		return Reconciliation{}, err
	}
	result := Reconciliation{}
	if found {
		result.Entry = pending
		switch pending.State {
		case EffectCommitted:
			result.Resolution = EffectResolvedCommitted
			return result, nil
		case EffectRejected:
			result.Resolution = EffectResolvedRejected
			return result, nil
		}
	}
	record, present, err := authority.LookupEffect(ctx, businessKey)
	if err != nil {
		return result, errors.Join(fmt.Errorf("%w: 权威核对失败 businessKey=%q", ErrLedgerEffectUnknown, businessKey), err)
	}
	if !present {
		if found && pending.State == EffectPending {
			result.Resolution = EffectResolvedUndetermined
			return result, fmt.Errorf("%w: scope=%q key=%q 仍在执行中且权威侧无记录，不能判定未生效", ErrLedgerEffectUnknown, scopeKey, key)
		}
		result.Resolution = EffectResolvedAbsent
		if found {
			if _, err := l.db.ExecContext(ctx,
				`DELETE FROM kernel_invocation_ledger WHERE scope_key=? AND idempotency_key=? AND effect_state='unknown'`,
				scopeKey, key); err != nil {
				return result, errors.Join(fmt.Errorf("%w: 释放未生效的未知记录失败", ErrLedgerEffectUnknown), err)
			}
			result.Entry = LedgerEntry{}
		}
		return result, nil
	}
	if record.BusinessKey != "" && record.BusinessKey != businessKey {
		return result, fmt.Errorf("%w: 权威记录业务键=%q 与请求=%q 不一致", ErrEffectRecordConflict, record.BusinessKey, businessKey)
	}
	if strings.TrimSpace(record.PayloadHash) != strings.TrimSpace(request.PayloadHash) {
		return result, fmt.Errorf("%w: 权威记录 payloadHash=%q 与请求=%q 不一致", ErrEffectRecordConflict, record.PayloadHash, request.PayloadHash)
	}
	attempt, err := l.verifyEffectEvidence(ctx, request, record.Token)
	if err != nil {
		return result, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	updated, err := l.db.ExecContext(ctx,
		`UPDATE kernel_invocation_ledger SET effect_state=?,result=?,error_code='',updated_at=? WHERE scope_key=? AND idempotency_key=? AND effect_state IN ('pending','unknown')`,
		string(EffectCommitted), []byte(record.Result), now, scopeKey, key)
	if err != nil {
		return result, err
	}
	if changed, err := updated.RowsAffected(); err != nil {
		return result, err
	} else if changed == 0 && !found {
		if _, err := l.db.ExecContext(ctx,
			`INSERT INTO kernel_invocation_ledger(scope_key,idempotency_key,invocation_id,capability_id,binding_revision,payload_hash,effect_state,result,error_code,updated_at) VALUES(?,?,?,?,?,?,?,?,'',?)`,
			scopeKey, key, strings.TrimSpace(request.Trace.InvocationID), strings.TrimSpace(request.CapabilityID), strings.TrimSpace(request.BindingRevision), strings.TrimSpace(request.PayloadHash), string(EffectCommitted), []byte(record.Result), now); err != nil {
			return result, err
		}
	}
	entry, _, err := l.lookupExact(ctx, scopeKey, key)
	if err != nil {
		return result, err
	}
	_ = l.markEffectToken(ctx, scopeKey, key, attempt, "issued", "redeemed")
	result.Resolution = EffectResolvedCommitted
	result.Entry = entry
	result.Record = record
	return result, nil
}

func (l *InvocationLedger) CommitWithEffect(ctx context.Context, request LedgerRequest, authority EffectAuthority, run func(EffectToken) (json.RawMessage, error)) (LedgerEntry, bool, error) {
	scopeKey, _, err := request.validate()
	if err != nil {
		return LedgerEntry{}, false, err
	}
	if !request.SideEffect.Mutating() {
		return LedgerEntry{}, false, fmt.Errorf("%w: sideEffect=%s 不产生需要核对的业务效果", ErrEffectTokenInvalid, request.SideEffect)
	}
	if run == nil {
		return LedgerEntry{}, false, fmt.Errorf("%w: 缺少写执行回调", ErrEffectTokenInvalid)
	}
	if authority == nil {
		return LedgerEntry{}, false, fmt.Errorf("%w: 未提供权威效果来源，写操作不能进入执行", ErrEffectTokenInvalid)
	}
	key := strings.TrimSpace(request.IdempotencyKey)
	existing, found, err := l.lookupExact(ctx, scopeKey, key)
	if err != nil {
		return LedgerEntry{}, false, err
	}
	if found {
		if existing.PayloadHash != strings.TrimSpace(request.PayloadHash) {
			return LedgerEntry{}, false, fmt.Errorf("%w: scope=%q key=%q recorded=%s requested=%s", ErrLedgerPayloadConflict, scopeKey, key, existing.PayloadHash, request.PayloadHash)
		}
		if existing.BindingRevision != strings.TrimSpace(request.BindingRevision) {
			return LedgerEntry{}, false, fmt.Errorf("%w: scope=%q key=%q frozen=%s requested=%s", ErrLedgerBindingFrozen, scopeKey, key, existing.BindingRevision, request.BindingRevision)
		}
		switch existing.State {
		case EffectCommitted:
			return existing, false, nil
		case EffectRejected:
			return existing, false, ledgerStateError(existing)
		case EffectUnknown:
			reconciled, err := l.Reconcile(ctx, request, authority)
			if err != nil {
				return reconciled.Entry, false, err
			}
			if reconciled.Resolution == EffectResolvedCommitted {
				return reconciled.Entry, false, nil
			}
			if !reconciled.RetrySafe() {
				return reconciled.Entry, false, fmt.Errorf("%w: scope=%q key=%q 核对结论=%s", ErrLedgerEffectUnknown, scopeKey, key, reconciled.Resolution)
			}
		}
	}
	token, err := l.IssueEffectToken(ctx, request)
	if err != nil {
		return LedgerEntry{}, false, err
	}
	executed := false
	entry, ran, err := l.Commit(ctx, request, func() (json.RawMessage, error) {
		executed = true
		return run(token)
	})
	_ = ran
	if executed {
		if err == nil {
			if markErr := l.markEffectToken(ctx, scopeKey, key, token.Attempt, "issued", "redeemed"); markErr != nil {
				return entry, executed, markErr
			}
		} else if markErr := l.markEffectToken(context.WithoutCancel(ctx), scopeKey, key, token.Attempt, "issued", "abandoned"); markErr != nil {
			return entry, executed, errors.Join(err, markErr)
		}
		return entry, executed, err
	}
	if markErr := l.markEffectToken(ctx, scopeKey, key, token.Attempt, "issued", "abandoned"); markErr != nil {
		return entry, executed, markErr
	}
	return entry, executed, err
}

func (r Reconciliation) String() string {
	return fmt.Sprintf("resolution=%s state=%s retrySafe=%t", r.Resolution, r.Entry.State, r.RetrySafe())
}
