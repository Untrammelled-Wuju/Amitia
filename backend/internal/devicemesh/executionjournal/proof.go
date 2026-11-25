package executionjournal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const EffectProofSchema = `CREATE TABLE IF NOT EXISTS kernel_invocation_effect_proof (
	scope_key TEXT NOT NULL,
	idempotency_key TEXT NOT NULL,
	proof_kind TEXT NOT NULL CHECK(proof_kind IN ('message','file_write','device_control')),
	capability_id TEXT NOT NULL,
	target TEXT NOT NULL,
	remote_id TEXT NOT NULL,
	payload_digest TEXT NOT NULL,
	bytes INTEGER NOT NULL DEFAULT 0,
	observed_at TEXT NOT NULL,
	recorded_at TEXT NOT NULL,
	PRIMARY KEY(scope_key,idempotency_key,proof_kind))`

const EffectProofImmutableSchema = `CREATE TRIGGER IF NOT EXISTS kernel_invocation_effect_proof_immutable
BEFORE UPDATE ON kernel_invocation_effect_proof
BEGIN
	SELECT RAISE(ABORT, 'effect proof row is immutable');
END`

const EffectProofRemovalSchema = `CREATE TRIGGER IF NOT EXISTS kernel_invocation_effect_proof_removal
BEFORE DELETE ON kernel_invocation_effect_proof
BEGIN
	SELECT RAISE(ABORT, 'effect proof row is immutable');
END`

const effectProofImmutableMessage = "effect proof row is immutable"

var (
	ErrEffectProofKindInvalid  = errors.New("执行账本：效果证明类型未知")
	ErrEffectProofIncomplete   = errors.New("执行账本：效果证明缺少该类型必需的字段")
	ErrEffectProofMismatch     = errors.New("执行账本：效果证明与账本记录的载荷不一致")
	ErrEffectProofForeign      = errors.New("执行账本：效果证明来自与账本记录不同的提供方")
	ErrEffectProofSurface      = errors.New("执行账本：该能力的对外效果面未定义，不能登记效果证明")
	ErrEffectProofKindMismatch = errors.New("执行账本：效果证明类型与该能力的对外效果面不符")
	ErrEffectProofOrphan       = errors.New("执行账本：没有对应的账本记录，不能凭空登记效果证明")
	ErrEffectProofConflict     = errors.New("执行账本：同一效果面已有不同证明，追加写不可改写")
)

type EffectProofKind string

const (
	ProofKindMessage       EffectProofKind = "message"
	ProofKindFileWrite     EffectProofKind = "file_write"
	ProofKindDeviceControl EffectProofKind = "device_control"
)

func (k EffectProofKind) Valid() bool {
	switch k {
	case ProofKindMessage, ProofKindFileWrite, ProofKindDeviceControl:
		return true
	}
	return false
}

var proofSurfaceByCapability = map[string]EffectProofKind{
	"host.message.send":   ProofKindMessage,
	"host.file.write":     ProofKindFileWrite,
	"host.device.control": ProofKindDeviceControl,
}

func ProofKindForCapability(capabilityID string) (EffectProofKind, bool) {
	kind, ok := proofSurfaceByCapability[strings.TrimSpace(capabilityID)]
	return kind, ok
}

type EffectProof struct {
	Kind         EffectProofKind
	CapabilityID string
	Target       string
	RemoteID     string
	Digest       string
	Bytes        int64
	ObservedAt   time.Time
}

func (p EffectProof) missingFields() []string {
	missing := []string{}
	require := func(name, value string) {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	switch p.Kind {
	case ProofKindMessage:
		require("target", p.Target)
		require("remoteId", p.RemoteID)
		require("digest", p.Digest)
	case ProofKindFileWrite:
		require("target", p.Target)
		require("digest", p.Digest)
		if p.Bytes <= 0 {
			missing = append(missing, "bytes")
		}
	case ProofKindDeviceControl:
		require("target", p.Target)
		require("remoteId", p.RemoteID)
		require("digest", p.Digest)
	}
	return missing
}

type EffectProofRecord struct {
	ScopeKey       string
	IdempotencyKey string
	Kind           EffectProofKind
	CapabilityID   string
	Target         string
	RemoteID       string
	PayloadDigest  string
	Bytes          int64
	ObservedAt     time.Time
	RecordedAt     time.Time
}

func (l *InvocationLedger) RecordEffectProof(ctx context.Context, request LedgerRequest, proof EffectProof) (EffectProofRecord, bool, error) {
	scopeKey, _, err := request.validate()
	if err != nil {
		return EffectProofRecord{}, false, err
	}
	key := strings.TrimSpace(request.IdempotencyKey)
	entry, found, err := l.Lookup(ctx, request.Scope, key)
	if err != nil {
		return EffectProofRecord{}, false, err
	}
	if !found {
		return EffectProofRecord{}, false, fmt.Errorf("%w: scope=%q key=%q", ErrEffectProofOrphan, scopeKey, key)
	}
	if !proof.Kind.Valid() {
		return EffectProofRecord{}, false, fmt.Errorf("%w: kind=%q", ErrEffectProofKindInvalid, proof.Kind)
	}
	if missing := proof.missingFields(); len(missing) > 0 {
		return EffectProofRecord{}, false, fmt.Errorf("%w: kind=%s missing=%s", ErrEffectProofIncomplete, proof.Kind, strings.Join(missing, ","))
	}
	expected, defined := ProofKindForCapability(entry.CapabilityID)
	if !defined {
		return EffectProofRecord{}, false, fmt.Errorf("%w: capability=%q", ErrEffectProofSurface, entry.CapabilityID)
	}
	if expected != proof.Kind {
		return EffectProofRecord{}, false, fmt.Errorf("%w: capability=%q want=%s got=%s", ErrEffectProofKindMismatch, entry.CapabilityID, expected, proof.Kind)
	}
	if strings.TrimSpace(proof.CapabilityID) != entry.CapabilityID {
		return EffectProofRecord{}, false, fmt.Errorf("%w: recorded=%q proof=%q", ErrEffectProofForeign, entry.CapabilityID, strings.TrimSpace(proof.CapabilityID))
	}
	if strings.TrimSpace(proof.Digest) != entry.PayloadHash {
		return EffectProofRecord{}, false, fmt.Errorf("%w: recorded=%s proof=%s", ErrEffectProofMismatch, entry.PayloadHash, strings.TrimSpace(proof.Digest))
	}
	observed := proof.ObservedAt
	if observed.IsZero() {
		observed = time.Now().UTC()
	}
	recorded := time.Now().UTC()
	result, err := l.db.ExecContext(ctx,
		`INSERT INTO kernel_invocation_effect_proof (scope_key,idempotency_key,proof_kind,capability_id,target,remote_id,payload_digest,bytes,observed_at,recorded_at)
VALUES (?,?,?,?,?,?,?,?,?,?) ON CONFLICT(scope_key,idempotency_key,proof_kind) DO NOTHING`,
		scopeKey, key, string(proof.Kind), entry.CapabilityID, strings.TrimSpace(proof.Target), strings.TrimSpace(proof.RemoteID), entry.PayloadHash, proof.Bytes, observed.UTC().Format(time.RFC3339Nano), recorded.Format(time.RFC3339Nano))
	if err != nil {
		return EffectProofRecord{}, false, fmt.Errorf("执行账本：登记效果证明失败: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return EffectProofRecord{}, false, err
	}
	stored, found, err := l.proof(ctx, scopeKey, key, proof.Kind)
	if err != nil {
		return EffectProofRecord{}, false, err
	}
	if !found {
		return EffectProofRecord{}, false, fmt.Errorf("%w: scope=%q key=%q kind=%s", ErrEffectProofOrphan, scopeKey, key, proof.Kind)
	}
	if inserted == 0 {
		if stored.Target != strings.TrimSpace(proof.Target) || stored.RemoteID != strings.TrimSpace(proof.RemoteID) || stored.Bytes != proof.Bytes {
			return EffectProofRecord{}, false, fmt.Errorf("%w: scope=%q key=%q kind=%s recorded=%s/%s", ErrEffectProofConflict, scopeKey, key, proof.Kind, stored.Target, stored.RemoteID)
		}
		if stored.PayloadDigest != strings.TrimSpace(proof.Digest) {
			return EffectProofRecord{}, false, fmt.Errorf("%w: scope=%q key=%q kind=%s", ErrEffectProofConflict, scopeKey, key, proof.Kind)
		}
		return stored, false, nil
	}
	return stored, true, nil
}

func (l *InvocationLedger) proof(ctx context.Context, scopeKey, key string, kind EffectProofKind) (EffectProofRecord, bool, error) {
	row := l.db.QueryRowContext(ctx,
		`SELECT scope_key,idempotency_key,proof_kind,capability_id,target,remote_id,payload_digest,bytes,observed_at,recorded_at
FROM kernel_invocation_effect_proof WHERE scope_key=? AND idempotency_key=? AND proof_kind=?`,
		scopeKey, key, string(kind))
	record, err := scanEffectProof(row)
	if errors.Is(err, sql.ErrNoRows) {
		return EffectProofRecord{}, false, nil
	}
	if err != nil {
		return EffectProofRecord{}, false, err
	}
	return record, true, nil
}

func scanEffectProof(row interface{ Scan(...any) error }) (EffectProofRecord, error) {
	var record EffectProofRecord
	var kind, observed, recorded string
	var bytes int64
	if err := row.Scan(&record.ScopeKey, &record.IdempotencyKey, &kind, &record.CapabilityID, &record.Target, &record.RemoteID, &record.PayloadDigest, &bytes, &observed, &recorded); err != nil {
		return EffectProofRecord{}, err
	}
	record.Kind = EffectProofKind(kind)
	record.Bytes = bytes
	if parsed, err := time.Parse(time.RFC3339Nano, observed); err == nil {
		record.ObservedAt = parsed
	}
	if parsed, err := time.Parse(time.RFC3339Nano, recorded); err == nil {
		record.RecordedAt = parsed
	}
	return record, nil
}

func (l *InvocationLedger) InspectProofs(ctx context.Context, scope LedgerScope, idempotencyKey string) ([]EffectProofRecord, error) {
	scopeKey, err := scope.Key()
	if err != nil {
		return nil, err
	}
	key := strings.TrimSpace(idempotencyKey)
	if key == "" || len(key) > 512 {
		return nil, fmt.Errorf("%w: idempotencyKey 长度必须在 1..512", ErrLedgerKeyInvalid)
	}
	rows, err := l.db.QueryContext(ctx,
		`SELECT scope_key,idempotency_key,proof_kind,capability_id,target,remote_id,payload_digest,bytes,observed_at,recorded_at
FROM kernel_invocation_effect_proof WHERE scope_key=? AND idempotency_key=? ORDER BY proof_kind`, scopeKey, key)
	if err != nil {
		return nil, fmt.Errorf("执行账本：读取效果证明失败: %w", err)
	}
	defer rows.Close()
	records := []EffectProofRecord{}
	for rows.Next() {
		record, err := scanEffectProof(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

type ExactlyOnceClaim struct {
	Claimable   bool
	LedgerState EffectState
	Kind        EffectProofKind
	ProofFound  bool
	Reason      string
}

const (
	ClaimReasonOK             = "claimable"
	ClaimReasonAbsent         = "absent"
	ClaimReasonNotSettled     = "not_settled"
	ClaimReasonSurfaceUnknown = "unknown_effect_surface"
	ClaimReasonMissingProof   = "missing_effect_proof"
	ClaimReasonPayloadDiffers = "proof_payload_mismatch"
	ClaimReasonProofForeign   = "proof_capability_mismatch"
)

func (l *InvocationLedger) ExactlyOnceClaim(ctx context.Context, scope LedgerScope, idempotencyKey string) (ExactlyOnceClaim, error) {
	scopeKey, err := scope.Key()
	if err != nil {
		return ExactlyOnceClaim{}, err
	}
	key := strings.TrimSpace(idempotencyKey)
	if key == "" || len(key) > 512 {
		return ExactlyOnceClaim{}, fmt.Errorf("%w: idempotencyKey 长度必须在 1..512", ErrLedgerKeyInvalid)
	}
	entry, found, err := l.Lookup(ctx, scope, key)
	if err != nil {
		return ExactlyOnceClaim{}, err
	}
	if !found {
		return ExactlyOnceClaim{Reason: ClaimReasonAbsent}, nil
	}
	claim := ExactlyOnceClaim{LedgerState: entry.State}
	kind, defined := ProofKindForCapability(entry.CapabilityID)
	if !defined {
		claim.Reason = ClaimReasonSurfaceUnknown
		return claim, nil
	}
	claim.Kind = kind
	record, hasProof, err := l.proof(ctx, scopeKey, key, kind)
	if err != nil {
		return ExactlyOnceClaim{}, err
	}
	claim.ProofFound = hasProof
	if entry.State != EffectCommitted {
		claim.Reason = ClaimReasonNotSettled
		return claim, nil
	}
	if !hasProof {
		claim.Reason = ClaimReasonMissingProof
		return claim, nil
	}
	if record.PayloadDigest != entry.PayloadHash {
		claim.Reason = ClaimReasonPayloadDiffers
		return claim, nil
	}
	if record.CapabilityID != entry.CapabilityID {
		claim.Reason = ClaimReasonProofForeign
		return claim, nil
	}
	claim.Claimable = true
	claim.Reason = ClaimReasonOK
	return claim, nil
}

func IsEffectProofImmutableError(err error) bool {
	return err != nil && strings.Contains(err.Error(), effectProofImmutableMessage)
}
