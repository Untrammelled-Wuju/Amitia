package kernel

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/trust"
)

type PackagePublisherKeyRecord struct {
	KeyID            string
	Fingerprint      string
	PublicKey        []byte
	PublisherID      string
	TrustSource      string
	TrustLevel       string
	KeyState         string
	TrustedAt        string
	RevokedAt        string
	RevocationReason string
	CreatedAt        string
	UpdatedAt        string
}

type PackageTrustRepository struct {
	db         *sql.DB
	mutationMu sync.Mutex
}

var ErrTrustMutationConflict = errors.New("kernel: trust mutation conflict")

func NewPackageTrustRepository(db *sql.DB) *PackageTrustRepository {
	return &PackageTrustRepository{db: db}
}

func (r *PackageTrustRepository) Put(ctx context.Context, record PackagePublisherKeyRecord) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO extension_publisher_keys (
		key_id, fingerprint, public_key, publisher_id, trust_source, trust_level, key_state,
		trusted_at, revoked_at, revocation_reason, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(key_id) DO UPDATE SET fingerprint=excluded.fingerprint, public_key=excluded.public_key,
	publisher_id=excluded.publisher_id, trust_source=excluded.trust_source, trust_level=excluded.trust_level,
	key_state=excluded.key_state, trusted_at=excluded.trusted_at, revoked_at=excluded.revoked_at,
	revocation_reason=excluded.revocation_reason, updated_at=excluded.updated_at`, record.KeyID,
		record.Fingerprint, record.PublicKey, record.PublisherID, record.TrustSource, record.TrustLevel,
		record.KeyState, record.TrustedAt, record.RevokedAt, record.RevocationReason,
		record.CreatedAt, record.UpdatedAt)
	return err
}

func (r *PackageTrustRepository) CurrentPolicyVersion(ctx context.Context) (uint64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT version FROM extension_package_security_audit
		WHERE event_type IN ('trust_policy_pending', 'trust_policy_active')`)
	if err != nil {
		return 0, fmt.Errorf("kernel: query trust policy version: %w", err)
	}
	defer rows.Close()
	var maximum uint64
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return 0, fmt.Errorf("kernel: scan trust policy version: %w", err)
		}
		version, parseErr := strconv.ParseUint(raw, 10, 64)
		if parseErr != nil {
			return 0, fmt.Errorf("kernel: parse trust policy version %q: %w", raw, parseErr)
		}
		if version > maximum {
			maximum = version
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("kernel: iterate trust policy versions: %w", err)
	}
	return maximum, nil
}

func (r *PackageTrustRepository) ReservePending(ctx context.Context, mutation trust.PolicyMutation) (trust.PolicyMutation, error) {
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()
	for attempt := 0; attempt < 16; attempt++ {
		version, err := r.CurrentPolicyVersion(ctx)
		if err != nil {
			return trust.PolicyMutation{}, err
		}
		mutation.Version = version + 1
		mutation.MutationID = fmt.Sprintf("trust-policy-%020d", mutation.Version)
		mutation.State = trust.PolicyMutationPending
		if mutation.CreatedAt.IsZero() {
			mutation.CreatedAt = time.Now().UTC()
		}
		details, err := json.Marshal(mutation)
		if err != nil {
			return trust.PolicyMutation{}, fmt.Errorf("kernel: encode trust mutation: %w", err)
		}
		_, err = r.db.ExecContext(ctx, `INSERT INTO extension_package_security_audit (
			event_id, event_type, package_id, version, publisher_id, report_id, staging_id,
			snapshot_id, operation_id, details, success, created_at
		) VALUES (?, 'trust_policy_pending', ?, ?, ?, '', '', '', ?, ?, 0, ?)`,
			mutation.MutationID+":pending", trustMutationPackageID(mutation), strconv.FormatUint(mutation.Version, 10),
			mutation.PublisherID, mutation.Actor, string(details), mutation.CreatedAt.UTC().Format(time.RFC3339Nano))
		if err == nil {
			return mutation, nil
		}
		if !isTrustJournalConflict(err) {
			return trust.PolicyMutation{}, fmt.Errorf("kernel: persist pending trust mutation: %w", err)
		}
	}
	return trust.PolicyMutation{}, ErrTrustMutationConflict
}

func (r *PackageTrustRepository) MarkActive(ctx context.Context, mutation trust.PolicyMutation) error {
	mutation.State = trust.PolicyMutationActive
	if mutation.ActivatedAt == nil {
		now := time.Now().UTC()
		mutation.ActivatedAt = &now
	}
	details, err := json.Marshal(mutation)
	if err != nil {
		return fmt.Errorf("kernel: encode active trust mutation: %w", err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("kernel: begin trust activation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var existing int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM extension_package_security_audit
		WHERE event_id = ? AND event_type = 'trust_policy_active'`, mutation.MutationID+":active").Scan(&existing); err != nil {
		return fmt.Errorf("kernel: check active trust mutation: %w", err)
	}
	if existing > 0 {
		return nil
	}
	if mutation.Kind == trust.PolicyMutationPublisherTrust && len(mutation.OldValue) > 0 && len(mutation.NewValue) > 0 {
		var before, after PackagePublisherKeyRecord
		if err := json.Unmarshal(mutation.OldValue, &before); err != nil {
			return fmt.Errorf("kernel: decode previous publisher key: %w", err)
		}
		if err := json.Unmarshal(mutation.NewValue, &after); err != nil {
			return fmt.Errorf("kernel: decode next publisher key: %w", err)
		}
		result, err := tx.ExecContext(ctx, `UPDATE extension_publisher_keys SET fingerprint = ?, public_key = ?,
			publisher_id = ?, trust_source = ?, trust_level = ?, key_state = ?, trusted_at = ?, revoked_at = ?,
			revocation_reason = ?, updated_at = ? WHERE key_id = ? AND updated_at = ?`, after.Fingerprint,
			after.PublicKey, after.PublisherID, after.TrustSource, after.TrustLevel, after.KeyState,
			after.TrustedAt, after.RevokedAt, after.RevocationReason, after.UpdatedAt, before.KeyID, before.UpdatedAt)
		if err != nil {
			return fmt.Errorf("kernel: activate publisher key mutation: %w", err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("kernel: inspect publisher key activation: %w", err)
		}
		if rows != 1 {
			return ErrTrustMutationConflict
		}
	} else if mutation.Kind == trust.PolicyMutationPublisherTrust && len(mutation.NewValue) > 0 {
		var after PackagePublisherKeyRecord
		if err := json.Unmarshal(mutation.NewValue, &after); err != nil {
			return fmt.Errorf("kernel: decode new publisher key: %w", err)
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO extension_publisher_keys (
			key_id, fingerprint, public_key, publisher_id, trust_source, trust_level, key_state,
			trusted_at, revoked_at, revocation_reason, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, after.KeyID, after.Fingerprint,
			after.PublicKey, after.PublisherID, after.TrustSource, after.TrustLevel, after.KeyState,
			after.TrustedAt, after.RevokedAt, after.RevocationReason, after.CreatedAt, after.UpdatedAt)
		if err != nil {
			if isTrustJournalConflict(err) {
				return ErrTrustMutationConflict
			}
			return fmt.Errorf("kernel: activate new publisher key: %w", err)
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO extension_package_security_audit (
		event_id, event_type, package_id, version, publisher_id, report_id, staging_id,
		snapshot_id, operation_id, details, success, created_at
	) VALUES (?, 'trust_policy_active', ?, ?, ?, '', '', '', ?, ?, 1, ?)`,
		mutation.MutationID+":active", trustMutationPackageID(mutation), strconv.FormatUint(mutation.Version, 10),
		mutation.PublisherID, mutation.Actor, string(details), mutation.ActivatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		if isTrustJournalConflict(err) {
			return nil
		}
		return fmt.Errorf("kernel: persist active trust mutation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("kernel: commit active trust mutation: %w", err)
	}
	return nil
}

func (r *PackageTrustRepository) Pending(ctx context.Context) ([]trust.PolicyMutation, error) {
	return r.listPolicyMutations(ctx, `SELECT pending.details
		FROM extension_package_security_audit pending
		WHERE pending.event_type = 'trust_policy_pending'
		AND NOT EXISTS (
			SELECT 1 FROM extension_package_security_audit active
			WHERE active.event_type = 'trust_policy_active'
			AND active.version = pending.version
		)
		ORDER BY CAST(pending.version AS INTEGER), pending.created_at`, trust.PolicyMutationPending)
}

func (r *PackageTrustRepository) ActivePolicyMutations(ctx context.Context) ([]trust.PolicyMutation, error) {
	return r.listPolicyMutations(ctx, `SELECT details FROM extension_package_security_audit
		WHERE event_type = 'trust_policy_active'
		ORDER BY CAST(version AS INTEGER), created_at`, trust.PolicyMutationActive)
}

func (r *PackageTrustRepository) PendingRestrictionReason(ctx context.Context, publisherID, artifactID, packageHash string) (string, error) {
	pending, err := r.Pending(ctx)
	if err != nil {
		return "", err
	}
	for _, mutation := range pending {
		if !mutation.Restrictive {
			continue
		}
		matches := mutation.PublisherID == "" && mutation.ArtifactID == "" && mutation.PackageHash == ""
		matches = matches || mutation.PublisherID != "" && mutation.PublisherID == publisherID
		matches = matches || mutation.ArtifactID != "" && mutation.ArtifactID == artifactID
		matches = matches || mutation.PackageHash != "" && mutation.PackageHash == packageHash
		if matches {
			return mutation.Reason, nil
		}
	}
	return "", nil
}

func (r *PackageTrustRepository) listPolicyMutations(ctx context.Context, query string, expected trust.PolicyMutationState) ([]trust.PolicyMutation, error) {
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("kernel: query pending trust mutations: %w", err)
	}
	defer rows.Close()
	var result []trust.PolicyMutation
	for rows.Next() {
		var details string
		if err := rows.Scan(&details); err != nil {
			return nil, fmt.Errorf("kernel: scan pending trust mutation: %w", err)
		}
		var mutation trust.PolicyMutation
		if err := json.Unmarshal([]byte(details), &mutation); err != nil {
			return nil, fmt.Errorf("kernel: decode pending trust mutation: %w", err)
		}
		if mutation.State != expected || mutation.Version == 0 || mutation.MutationID == "" {
			return nil, fmt.Errorf("kernel: invalid %s trust mutation %q", expected, mutation.MutationID)
		}
		result = append(result, mutation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("kernel: iterate pending trust mutations: %w", err)
	}
	return result, nil
}

func (r *PackageTrustRepository) CompareAndSwap(ctx context.Context, before, after PackagePublisherKeyRecord) error {
	result, err := r.db.ExecContext(ctx, `UPDATE extension_publisher_keys SET fingerprint = ?, public_key = ?,
		publisher_id = ?, trust_source = ?, trust_level = ?, key_state = ?, trusted_at = ?, revoked_at = ?,
		revocation_reason = ?, updated_at = ? WHERE key_id = ? AND updated_at = ?`, after.Fingerprint,
		after.PublicKey, after.PublisherID, after.TrustSource, after.TrustLevel, after.KeyState,
		after.TrustedAt, after.RevokedAt, after.RevocationReason, after.UpdatedAt, before.KeyID, before.UpdatedAt)
	if err != nil {
		return fmt.Errorf("kernel: compare and swap publisher key: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("kernel: inspect publisher key update: %w", err)
	}
	if rows != 1 {
		return ErrTrustMutationConflict
	}
	return nil
}

func trustMutationPackageID(mutation trust.PolicyMutation) string {
	if mutation.ArtifactID != "" {
		return mutation.ArtifactID
	}
	return mutation.PackageHash
}

func isTrustJournalConflict(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "constraint")
}

var _ trust.PolicyMutationJournal = (*PackageTrustRepository)(nil)

func (r *PackageTrustRepository) GetByFingerprint(ctx context.Context, fingerprint string) (PackagePublisherKeyRecord, error) {
	var record PackagePublisherKeyRecord
	err := r.db.QueryRowContext(ctx, `SELECT key_id, fingerprint, public_key, publisher_id,
		trust_source, trust_level, key_state, trusted_at, revoked_at, revocation_reason,
		created_at, updated_at FROM extension_publisher_keys WHERE fingerprint = ?`, fingerprint).
		Scan(&record.KeyID, &record.Fingerprint, &record.PublicKey, &record.PublisherID,
			&record.TrustSource, &record.TrustLevel, &record.KeyState, &record.TrustedAt,
			&record.RevokedAt, &record.RevocationReason, &record.CreatedAt, &record.UpdatedAt)
	return record, err
}

func (r *PackageTrustRepository) List(ctx context.Context) ([]PackagePublisherKeyRecord, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT key_id, fingerprint, public_key, publisher_id,
		trust_source, trust_level, key_state, trusted_at, revoked_at, revocation_reason,
		created_at, updated_at FROM extension_publisher_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PackagePublisherKeyRecord
	for rows.Next() {
		var record PackagePublisherKeyRecord
		if err := rows.Scan(&record.KeyID, &record.Fingerprint, &record.PublicKey, &record.PublisherID,
			&record.TrustSource, &record.TrustLevel, &record.KeyState, &record.TrustedAt,
			&record.RevokedAt, &record.RevocationReason, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	return result, rows.Err()
}

func (r *PackageTrustRepository) SetTrusted(ctx context.Context, fingerprint string, trusted bool) (PackagePublisherKeyRecord, error) {
	record, err := r.GetByFingerprint(ctx, fingerprint)
	if err != nil {
		return record, err
	}
	if len(record.PublicKey) != ed25519.PublicKeySize || record.TrustSource == "legacy_fingerprint_only" {
		return record, fmt.Errorf("kernel: legacy fingerprint cannot become a trusted verification key")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if trusted {
		record.TrustLevel = string(trust.TrustLevelUserTrusted)
		record.KeyState = string(trust.KeyStateActive)
		record.TrustedAt = now
		record.RevokedAt = ""
		record.RevocationReason = ""
	} else {
		record.TrustLevel = string(trust.TrustLevelRevoked)
		record.KeyState = string(trust.KeyStateRevoked)
		record.RevokedAt = now
		record.RevocationReason = "user decision"
	}
	record.UpdatedAt = now
	return record, r.Put(ctx, record)
}

func (r *PackageTrustRepository) Restore(ctx context.Context, service *trust.TrustService) error {
	records, err := r.List(ctx)
	if err != nil {
		return err
	}
	grouped := map[string][]PackagePublisherKeyRecord{}
	for _, record := range records {
		if len(record.PublicKey) == ed25519.PublicKeySize && record.TrustSource != "legacy_fingerprint_only" {
			grouped[record.PublisherID] = append(grouped[record.PublisherID], record)
		}
	}
	for publisherID, records := range grouped {
		identity := trust.PublisherIdentity{PublisherID: publisherID, DisplayName: publisherID,
			TrustLevel: trust.TrustLevel(records[0].TrustLevel), Source: trust.TrustSourceUserDecision}
		for _, record := range records {
			createdAt, _ := time.Parse(time.RFC3339Nano, record.CreatedAt)
			key := trust.PublisherKey{KeyID: record.KeyID, PublisherID: publisherID,
				PublicKey: append([]byte(nil), record.PublicKey...), Algorithm: trust.AlgorithmEd25519,
				State: trust.KeyState(record.KeyState), CreatedAt: createdAt, RevokedReason: record.RevocationReason}
			if record.RevokedAt != "" {
				revokedAt, parseErr := time.Parse(time.RFC3339Nano, record.RevokedAt)
				if parseErr == nil {
					key.RevokedAt = &revokedAt
				}
			}
			identity.Keys = append(identity.Keys, key)
		}
		if err := service.Store().RegisterUserDecision(identity); err != nil {
			return err
		}
	}
	return nil
}
