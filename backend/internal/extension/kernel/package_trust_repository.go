package kernel

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"fmt"
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
	db *sql.DB
}

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
