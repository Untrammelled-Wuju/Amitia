package kernel

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

type PackageArtifactReference struct {
	ReferenceID      string
	ArtifactID       string
	ReferenceType    string
	ReferenceOwnerID string
	ExpiresAt        string
	CreatedAt        string
	ReleasedAt       string
}

const (
	ArtifactReferencePreview         = "preview"
	ArtifactReferenceInstallation    = "installation"
	ArtifactReferenceRollbackPoint   = "rollback_point"
	ArtifactReferenceOperation       = "operation"
	ArtifactReferenceExportLease     = "export_lease"
	ArtifactReferenceManualRetention = "manual_retention"
	ArtifactReferenceLegacyMigration = "legacy_migration"
)

var artifactReferenceTypes = map[string]struct{}{
	ArtifactReferencePreview: {}, ArtifactReferenceInstallation: {}, ArtifactReferenceRollbackPoint: {},
	ArtifactReferenceOperation: {}, ArtifactReferenceExportLease: {}, ArtifactReferenceManualRetention: {},
	ArtifactReferenceLegacyMigration: {},
}

type PackageArtifactGCResult struct {
	Deleted []string
	Failed  map[string]string
}

type PackageArtifactVerificationResult struct {
	Verified  []string
	Corrupted map[string]string
}

type PackageArtifactLifecycle struct {
	repository *PackageRepository
	store      *PackageArtifactStore
}

func NewPackageArtifactLifecycle(repository *PackageRepository, store *PackageArtifactStore) *PackageArtifactLifecycle {
	return &PackageArtifactLifecycle{repository: repository, store: store}
}

func (r *PackageRepository) AcquireArtifactReference(ctx context.Context, artifactID, referenceType, ownerID string, expiresAt time.Time) (PackageArtifactReference, error) {
	if _, ok := artifactReferenceTypes[referenceType]; !ok {
		return PackageArtifactReference{}, fmt.Errorf("invalid artifact reference type")
	}
	if strings.TrimSpace(artifactID) == "" || strings.TrimSpace(ownerID) == "" {
		return PackageArtifactReference{}, fmt.Errorf("artifact reference identity required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return PackageArtifactReference{}, err
	}
	defer tx.Rollback()
	reference, err := acquireArtifactReferenceTx(ctx, tx, artifactID, referenceType, ownerID, expiresAt)
	if err != nil {
		return PackageArtifactReference{}, err
	}
	if err := tx.Commit(); err != nil {
		return PackageArtifactReference{}, err
	}
	return reference, nil
}

func acquireArtifactReferenceTx(ctx context.Context, tx *sql.Tx, artifactID, referenceType, ownerID string, expiresAt time.Time) (PackageArtifactReference, error) {
	var retentionState, deletedAt string
	if err := tx.QueryRowContext(ctx, `SELECT retention_state, deleted_at FROM extension_package_artifacts WHERE artifact_id = ?`, artifactID).Scan(&retentionState, &deletedAt); err != nil {
		return PackageArtifactReference{}, err
	}
	if deletedAt != "" || retentionState == "gc_pending" || retentionState == "deleted" {
		return PackageArtifactReference{}, fmt.Errorf("artifact unavailable for reference")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	expires := ""
	if !expiresAt.IsZero() {
		expires = expiresAt.UTC().Format(time.RFC3339Nano)
	}
	reference := PackageArtifactReference{ReferenceID: "artifact-reference-" + uuid.NewString(), ArtifactID: artifactID,
		ReferenceType: referenceType, ReferenceOwnerID: ownerID, ExpiresAt: expires, CreatedAt: now}
	_, err := tx.ExecContext(ctx, `INSERT INTO extension_package_artifact_references
		(reference_id, artifact_id, reference_type, reference_owner_id, expires_at, created_at, released_at)
		VALUES (?, ?, ?, ?, ?, ?, '') ON CONFLICT(artifact_id, reference_type, reference_owner_id)
		DO UPDATE SET expires_at=excluded.expires_at, released_at='', created_at=CASE
		WHEN extension_package_artifact_references.released_at <> '' THEN excluded.created_at
		ELSE extension_package_artifact_references.created_at END`, reference.ReferenceID, artifactID,
		referenceType, ownerID, expires, now)
	if err != nil {
		return PackageArtifactReference{}, err
	}
	if err := syncArtifactReferenceCountTx(ctx, tx, artifactID, false); err != nil {
		return PackageArtifactReference{}, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT reference_id, expires_at, created_at, released_at
		FROM extension_package_artifact_references WHERE artifact_id=? AND reference_type=? AND reference_owner_id=?`,
		artifactID, referenceType, ownerID).Scan(&reference.ReferenceID, &reference.ExpiresAt, &reference.CreatedAt, &reference.ReleasedAt); err != nil {
		return PackageArtifactReference{}, err
	}
	return reference, nil
}

func (r *PackageRepository) HasArtifactReference(ctx context.Context, artifactID, referenceType, ownerID string) (bool, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM extension_package_artifact_references WHERE artifact_id=? AND reference_type=? AND reference_owner_id=? AND released_at=''`, artifactID, referenceType, ownerID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *PackageRepository) ReleaseArtifactReference(ctx context.Context, artifactID, referenceType, ownerID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := releaseArtifactReferenceTx(ctx, tx, artifactID, referenceType, ownerID); err != nil {
		return err
	}
	return tx.Commit()
}

func releaseArtifactReferenceTx(ctx context.Context, tx *sql.Tx, artifactID, referenceType, ownerID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `UPDATE extension_package_artifact_references SET released_at=?
		WHERE artifact_id=? AND reference_type=? AND reference_owner_id=? AND released_at=''`, now, artifactID, referenceType, ownerID); err != nil {
		return err
	}
	return syncArtifactReferenceCountTx(ctx, tx, artifactID, true)
}

func syncArtifactReferenceCountTx(ctx context.Context, tx *sql.Tx, artifactID string, released bool) error {
	var count int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM extension_package_artifact_references WHERE artifact_id=? AND released_at=''`, artifactID).Scan(&count); err != nil {
		return err
	}
	if count < 0 {
		return fmt.Errorf("artifact reference count invalid")
	}
	if released && count == 0 {
		_, err := tx.ExecContext(ctx, `UPDATE extension_package_artifacts SET reference_count=0,
			retention_until=?, retention_state=CASE WHEN retention_state='retained' THEN retention_state ELSE 'active' END
			WHERE artifact_id=? AND deleted_at=''`, time.Now().UTC().Format(time.RFC3339Nano), artifactID)
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE extension_package_artifacts SET reference_count=?, retention_state='active',
		retention_until='', gc_error='' WHERE artifact_id=? AND deleted_at=''`, count, artifactID)
	return err
}

func (r *PackageRepository) CountActiveArtifactReferences(ctx context.Context, artifactID string) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM extension_package_artifact_references WHERE artifact_id=? AND released_at=''`, artifactID).Scan(&count)
	return count, err
}

func (r *PackageRepository) ExpirePackagePreviews(ctx context.Context, now time.Time) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT session_id, artifact_id FROM extension_package_preview_sessions
		WHERE status IN ('ready','awaiting_confirmation','created','verifying') AND expires_at <> '' AND expires_at <= ?`, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	type expiredPreview struct{ sessionID, artifactID string }
	var previews []expiredPreview
	for rows.Next() {
		var preview expiredPreview
		if err := rows.Scan(&preview.sessionID, &preview.artifactID); err != nil {
			rows.Close()
			return 0, err
		}
		previews = append(previews, preview)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, preview := range previews {
		if _, err := tx.ExecContext(ctx, `UPDATE extension_package_preview_sessions SET status='expired' WHERE session_id=?`, preview.sessionID); err != nil {
			return 0, err
		}
		if err := releaseArtifactReferenceTx(ctx, tx, preview.artifactID, ArtifactReferencePreview, preview.sessionID); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int64(len(previews)), nil
}

func (r *PackageRepository) ReleaseExpiredArtifactReferences(ctx context.Context, now time.Time) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT artifact_id, reference_type, reference_owner_id
		FROM extension_package_artifact_references WHERE released_at='' AND expires_at<>'' AND expires_at<=?
		AND reference_type<>?`, now.UTC().Format(time.RFC3339Nano), ArtifactReferencePreview)
	if err != nil {
		return 0, err
	}
	type expiredReference struct{ artifactID, referenceType, ownerID string }
	var references []expiredReference
	for rows.Next() {
		var reference expiredReference
		if err := rows.Scan(&reference.artifactID, &reference.referenceType, &reference.ownerID); err != nil {
			rows.Close()
			return 0, err
		}
		references = append(references, reference)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, reference := range references {
		if err := releaseArtifactReferenceTx(ctx, tx, reference.artifactID, reference.referenceType, reference.ownerID); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int64(len(references)), nil
}

func (r *PackageRepository) ListArtifactGCCandidates(ctx context.Context, now time.Time, retention time.Duration, limit int) ([]PackageArtifact, error) {
	if limit < 1 || limit > 1000 {
		limit = 100
	}
	cutoff := now.UTC().Add(-retention).Format(time.RFC3339Nano)
	rows, err := r.db.QueryContext(ctx, `SELECT artifact_id FROM extension_package_artifacts
		WHERE deleted_at='' AND retention_state IN ('active','gc_failed') AND reference_count=0
		AND COALESCE(NULLIF(retention_until,''), created_at) <= ? ORDER BY created_at LIMIT ?`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]PackageArtifact, 0, len(ids))
	for _, id := range ids {
		artifact, err := r.GetArtifact(ctx, id)
		if err != nil {
			return nil, err
		}
		result = append(result, artifact)
	}
	return result, nil
}

func (r *PackageRepository) MarkArtifactGCPending(ctx context.Context, artifactID string) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var count int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM extension_package_artifact_references WHERE artifact_id=? AND released_at=''`, artifactID).Scan(&count); err != nil {
		return false, err
	}
	if count != 0 {
		if err := syncArtifactReferenceCountTx(ctx, tx, artifactID, false); err != nil {
			return false, err
		}
		if err := tx.Commit(); err != nil {
			return false, err
		}
		return false, nil
	}
	result, err := tx.ExecContext(ctx, `UPDATE extension_package_artifacts SET retention_state='gc_pending',
		gc_attempted_at=?, gc_error='' WHERE artifact_id=? AND deleted_at='' AND retention_state IN ('active','gc_failed')`,
		time.Now().UTC().Format(time.RFC3339Nano), artifactID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return affected == 1, nil
}

func (r *PackageRepository) CompleteArtifactGC(ctx context.Context, artifactID string, removeErr error) error {
	if removeErr != nil {
		_, err := r.db.ExecContext(ctx, `UPDATE extension_package_artifacts SET retention_state='gc_failed', gc_error=?
			WHERE artifact_id=? AND retention_state='gc_pending'`, removeErr.Error(), artifactID)
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `UPDATE extension_package_artifacts SET retention_state='deleted', deleted_at=?,
		gc_error='', reference_count=0 WHERE artifact_id=? AND retention_state='gc_pending'`, now, artifactID)
	return err
}

func (r *PackageRepository) ListArtifactsDueVerification(ctx context.Context, before time.Time, limit int) ([]PackageArtifact, error) {
	if limit < 1 || limit > 1000 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `SELECT artifact_id FROM extension_package_artifacts WHERE deleted_at=''
		AND retention_state NOT IN ('gc_pending','deleted') AND COALESCE(NULLIF(last_verified_at,''), NULLIF(verified_at,''), created_at) <= ?
		ORDER BY COALESCE(NULLIF(last_verified_at,''), NULLIF(verified_at,''), created_at) LIMIT ?`, before.UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	artifacts := make([]PackageArtifact, 0, len(ids))
	for _, id := range ids {
		artifact, err := r.GetArtifact(ctx, id)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, nil
}

func (r *PackageRepository) RecordArtifactVerification(ctx context.Context, artifactID string, verificationErr error) error {
	status := "valid"
	detail := ""
	if verificationErr != nil {
		status = "corrupted"
		detail = verificationErr.Error()
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `UPDATE extension_package_artifacts SET last_verified_at=?, verified_at=?,
		verification_status=?, gc_error=CASE WHEN ?='' THEN gc_error ELSE ? END WHERE artifact_id=? AND deleted_at=''`,
		now, now, status, detail, detail, artifactID)
	return err
}

func (l *PackageArtifactLifecycle) CollectGarbage(ctx context.Context, now time.Time, retention time.Duration, limit int) (PackageArtifactGCResult, error) {
	result := PackageArtifactGCResult{Failed: map[string]string{}}
	if l == nil || l.repository == nil || l.store == nil {
		return result, fmt.Errorf("artifact lifecycle unavailable")
	}
	candidates, err := l.repository.ListArtifactGCCandidates(ctx, now, retention, limit)
	if err != nil {
		return result, err
	}
	for _, artifact := range candidates {
		pending, err := l.repository.MarkArtifactGCPending(ctx, artifact.ArtifactID)
		if err != nil {
			result.Failed[artifact.ArtifactID] = err.Error()
			continue
		}
		if !pending {
			continue
		}
		removeErr := l.store.RemoveArchive(artifact)
		if err := l.repository.CompleteArtifactGC(ctx, artifact.ArtifactID, removeErr); err != nil {
			result.Failed[artifact.ArtifactID] = err.Error()
			continue
		}
		if removeErr != nil {
			result.Failed[artifact.ArtifactID] = removeErr.Error()
			continue
		}
		result.Deleted = append(result.Deleted, artifact.ArtifactID)
	}
	return result, nil
}

func (l *PackageArtifactLifecycle) VerifyDueArtifacts(ctx context.Context, before time.Time, limit int) (PackageArtifactVerificationResult, error) {
	result := PackageArtifactVerificationResult{Corrupted: map[string]string{}}
	if l == nil || l.repository == nil || l.store == nil {
		return result, fmt.Errorf("artifact lifecycle unavailable")
	}
	artifacts, err := l.repository.ListArtifactsDueVerification(ctx, before, limit)
	if err != nil {
		return result, err
	}
	for _, artifact := range artifacts {
		verifyErr := l.store.VerifyArchive(artifact)
		if err := l.repository.RecordArtifactVerification(ctx, artifact.ArtifactID, verifyErr); err != nil {
			return result, err
		}
		if verifyErr != nil {
			if errors.Is(verifyErr, os.ErrNotExist) {
				result.Corrupted[artifact.ArtifactID] = "artifact missing"
			} else {
				result.Corrupted[artifact.ArtifactID] = verifyErr.Error()
			}
			continue
		}
		result.Verified = append(result.Verified, artifact.ArtifactID)
	}
	return result, nil
}
