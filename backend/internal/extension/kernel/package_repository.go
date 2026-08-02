package kernel

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

type PackageArtifact struct {
	ArtifactID             string
	ExtensionID            string
	Version                string
	ArchiveHash            string
	ManifestHash           string
	ContentTreeHash        string
	ArtifactHash           string
	ArchivePath            string
	InstalledPath          string
	SizeBytes              int64
	SignatureStatus        string
	SignerKeyID            string
	PublisherID            string
	TrustDecision          string
	VerificationReportJSON string
	CreatedAt              string
	VerifiedAt             string
	QuarantinedAt          string
	ReferenceCount         int64
	RetentionState         string
	RetentionUntil         string
	LastVerifiedAt         string
	VerificationStatus     string
	GCError                string
	GCAttemptedAt          string
	DeletedAt              string
}

type PackagePreviewSession struct {
	SessionID                 string
	UserID                    string
	ScopeType                 string
	ScopeID                   string
	ArtifactID                string
	ExtensionID               string
	Version                   string
	Status                    string
	ArchiveHash               string
	ManifestHash              string
	ContentTreeHash           string
	RiskFlagsJSON             string
	RequiredConfirmationsJSON string
	DependencyResultJSON      string
	PreviewResultJSON         string
	VerificationReportJSON    string
	PolicyVersion             string
	SecurityPolicyHash        string
	VerifiedAt                string
	ExpiresAt                 string
	ConsumedAt                string
	CreatedAt                 string
}

type PackageOperationRecord struct {
	OperationID             string
	TraceID                 string
	UserID                  string
	ScopeType               string
	ScopeID                 string
	ExtensionID             string
	TargetVersion           string
	OperationType           string
	Status                  string
	CurrentStep             string
	ArtifactID              string
	PreviewSessionID        string
	ConfirmationsJSON       string
	ConfirmationClaimsJSON  string
	ErrorCode               string
	ErrorDetail             string
	StartedAt               string
	UpdatedAt               string
	CompletedAt             string
	StableGeneration        string
	TargetGeneration        string
	CurrentPointerJSON      string
	IdempotencyKey          string
	RequestHash             string
	FromVersion             string
	RecoveryRequired        bool
	CancelRequestedAt       string
	LeaseOwner              string
	LeaseExpiresAt          string
	AttemptCount            int
	FencingToken            int64
	OwnerInstanceID         string
	SnapshotRequirementHash string
}

type PackageOperationStep struct {
	StepID             string
	OperationID        string
	StepName           string
	StepOrder          int
	Status             string
	AttemptCount       int
	ResultJSON         string
	ErrorCode          string
	StartedAt          string
	CompletedAt        string
	StableGeneration   string
	TargetGeneration   string
	CurrentPointerJSON string
	InputHash          string
	ErrorDetail        string
	CompensationName   string
	CompensationStatus string
	SideEffectEvidence string
	UpdatedAt          string
	ResultHash         string
	CASVersion         int64
}

type PackageRollbackPoint struct {
	RollbackPointID            string
	ExtensionID                string
	SourceVersion              string
	SourceGeneration           int64
	ArtifactID                 string
	DefinitionSnapshotJSON     string
	ModuleSnapshotJSON         string
	ContributionSnapshotJSON   string
	PermissionSnapshotJSON     string
	ScopeSnapshotJSON          string
	ConfigSnapshotID           string
	ConfigSnapshotJSON         string
	SecretRefsJSON             string
	ResourceSnapshotJSON       string
	MigrationStateSnapshotJSON string
	UserDataMigrationStateJSON string
	SnapshotHash               string
	RetentionState             string
	RetentionUntil             string
	SourceOperationID          string
	InstalledPath              string
	CreatedAt                  string
	ExpiresAt                  string
}

type PackageExportTicket struct {
	ExportID    string
	UserID      string
	ExtensionID string
	ArtifactID  string
	FileName    string
	MIMEType    string
	ExpiresAt   string
	CreatedAt   string
}

type PackageRepository struct {
	db *sql.DB
}

func NewPackageRepository(db *sql.DB) *PackageRepository {
	return &PackageRepository{db: db}
}

const packageOperationSelectColumns = `operation_id, trace_id, user_id, scope_type, scope_id,
	extension_id, target_version, operation_type, status, current_step, artifact_id,
	preview_session_id, confirmations_json, confirmation_claims_json, error_code, error_detail, started_at, updated_at,
	completed_at, stable_generation, target_generation, current_pointer_json, snapshot_requirement_hash`

type sqlScanner interface {
	Scan(dest ...any) error
}

func scanPackageOperation(scanner sqlScanner) (*PackageOperationRecord, error) {
	var op PackageOperationRecord
	if err := scanner.Scan(
		&op.OperationID,
		&op.TraceID,
		&op.UserID,
		&op.ScopeType,
		&op.ScopeID,
		&op.ExtensionID,
		&op.TargetVersion,
		&op.OperationType,
		&op.Status,
		&op.CurrentStep,
		&op.ArtifactID,
		&op.PreviewSessionID,
		&op.ConfirmationsJSON,
		&op.ConfirmationClaimsJSON,
		&op.ErrorCode,
		&op.ErrorDetail,
		&op.StartedAt,
		&op.UpdatedAt,
		&op.CompletedAt,
		&op.StableGeneration,
		&op.TargetGeneration,
		&op.CurrentPointerJSON,
		&op.SnapshotRequirementHash,
	); err != nil {
		return nil, err
	}
	return &op, nil
}

func scanPackageOperations(rows *sql.Rows) ([]PackageOperationRecord, error) {
	defer rows.Close()
	operations := make([]PackageOperationRecord, 0)
	for rows.Next() {
		operation, err := scanPackageOperation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan package operation: %w", err)
		}
		operations = append(operations, *operation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate package operations: %w", err)
	}
	return operations, nil
}

func (r *PackageRepository) PutArtifact(ctx context.Context, a PackageArtifact) error {
	if a.RetentionState == "" {
		a.RetentionState = "active"
	}
	if a.VerificationStatus == "" {
		a.VerificationStatus = "pending"
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO extension_package_artifacts (
		artifact_id, extension_id, version, archive_hash, manifest_hash, content_tree_hash,
		artifact_hash, archive_path, installed_path, size_bytes, signature_status, signer_key_id,
		publisher_id, trust_decision, verification_report_json, created_at, verified_at, quarantined_at,
		reference_count, retention_state, retention_until, last_verified_at, verification_status,
		gc_error, gc_attempted_at, deleted_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT DO UPDATE SET extension_id=excluded.extension_id, version=excluded.version,
	manifest_hash=excluded.manifest_hash, content_tree_hash=excluded.content_tree_hash,
	artifact_hash=excluded.artifact_hash, archive_path=excluded.archive_path,
	installed_path=CASE WHEN excluded.installed_path = '' THEN extension_package_artifacts.installed_path ELSE excluded.installed_path END, signature_status=excluded.signature_status,
	signer_key_id=excluded.signer_key_id, publisher_id=excluded.publisher_id,
	trust_decision=excluded.trust_decision, verification_report_json=excluded.verification_report_json,
	verified_at=excluded.verified_at, quarantined_at=excluded.quarantined_at,
	last_verified_at=CASE WHEN excluded.last_verified_at = '' THEN extension_package_artifacts.last_verified_at ELSE excluded.last_verified_at END,
	verification_status=CASE WHEN excluded.verification_status = 'pending' AND extension_package_artifacts.verification_status <> '' THEN extension_package_artifacts.verification_status ELSE excluded.verification_status END`,
		a.ArtifactID, a.ExtensionID, a.Version, a.ArchiveHash, a.ManifestHash, a.ContentTreeHash,
		a.ArtifactHash, a.ArchivePath, a.InstalledPath, a.SizeBytes, a.SignatureStatus, a.SignerKeyID,
		a.PublisherID, a.TrustDecision, a.VerificationReportJSON, a.CreatedAt, a.VerifiedAt, a.QuarantinedAt,
		a.ReferenceCount, a.RetentionState, a.RetentionUntil, a.LastVerifiedAt, a.VerificationStatus,
		a.GCError, a.GCAttemptedAt, a.DeletedAt)
	return err
}

func (r *PackageRepository) SetArtifactInstalledPath(ctx context.Context, artifactID, installedPath string, guard PackageWriteGuard) error {
	if err := verifyFencingTokenDB(ctx, r.db, guard); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `UPDATE extension_package_artifacts SET installed_path=? WHERE artifact_id=?`, installedPath, artifactID)
	return err
}

func (r *PackageRepository) GetArtifact(ctx context.Context, id string) (PackageArtifact, error) {
	var a PackageArtifact
	err := r.db.QueryRowContext(ctx, `SELECT artifact_id, extension_id, version, archive_hash,
		manifest_hash, content_tree_hash, artifact_hash, archive_path, installed_path, size_bytes,
		signature_status, signer_key_id, publisher_id, trust_decision, verification_report_json,
		created_at, verified_at, quarantined_at, reference_count, retention_state, retention_until,
		last_verified_at, verification_status, gc_error, gc_attempted_at, deleted_at
		FROM extension_package_artifacts WHERE artifact_id = ?`, id).
		Scan(&a.ArtifactID, &a.ExtensionID, &a.Version, &a.ArchiveHash, &a.ManifestHash,
			&a.ContentTreeHash, &a.ArtifactHash, &a.ArchivePath, &a.InstalledPath, &a.SizeBytes,
			&a.SignatureStatus, &a.SignerKeyID, &a.PublisherID, &a.TrustDecision,
			&a.VerificationReportJSON, &a.CreatedAt, &a.VerifiedAt, &a.QuarantinedAt,
			&a.ReferenceCount, &a.RetentionState, &a.RetentionUntil, &a.LastVerifiedAt,
			&a.VerificationStatus, &a.GCError, &a.GCAttemptedAt, &a.DeletedAt)
	if err != nil {
		return a, ClassifyRepositoryError("get artifact", err)
	}
	return a, nil
}

func (r *PackageRepository) GetArtifactByVersion(ctx context.Context, extensionID, version string) (PackageArtifact, error) {
	var id string
	if err := r.db.QueryRowContext(ctx, `SELECT artifact_id FROM extension_package_artifacts WHERE extension_id = ? AND version = ? AND quarantined_at = '' AND deleted_at = '' ORDER BY created_at DESC LIMIT 1`, extensionID, version).Scan(&id); err != nil {
		return PackageArtifact{}, err
	}
	return r.GetArtifact(ctx, id)
}

func (r *PackageRepository) GetArtifactByIdentity(ctx context.Context, extensionID, version, archiveHash string) (PackageArtifact, error) {
	var id string
	if err := r.db.QueryRowContext(ctx, `SELECT artifact_id FROM extension_package_artifacts WHERE extension_id = ? AND version = ? AND archive_hash = ? AND deleted_at = '' LIMIT 1`, extensionID, version, archiveHash).Scan(&id); err != nil {
		return PackageArtifact{}, err
	}
	return r.GetArtifact(ctx, id)
}

func (r *PackageRepository) PutPreview(ctx context.Context, s PackagePreviewSession) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO extension_package_preview_sessions (
		session_id, user_id, scope_type, scope_id, artifact_id, extension_id, version, status,
		archive_hash, manifest_hash, content_tree_hash, risk_flags_json, required_confirmations_json,
		dependency_result_json, preview_result_json, verification_report_json, policy_version,
		security_policy_hash, verified_at, expires_at, consumed_at, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.SessionID, s.UserID, s.ScopeType, s.ScopeID, s.ArtifactID, s.ExtensionID, s.Version,
		s.Status, s.ArchiveHash, s.ManifestHash, s.ContentTreeHash, s.RiskFlagsJSON,
		s.RequiredConfirmationsJSON, s.DependencyResultJSON, s.PreviewResultJSON,
		s.VerificationReportJSON, s.PolicyVersion, s.SecurityPolicyHash, s.VerifiedAt, s.ExpiresAt, s.ConsumedAt, s.CreatedAt)
	if err != nil {
		return err
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, s.ExpiresAt)
	if err != nil {
		return err
	}
	if _, err := acquireArtifactReferenceTx(ctx, tx, s.ArtifactID, ArtifactReferencePreview, s.SessionID, expiresAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *PackageRepository) GetPreview(ctx context.Context, id, userID, scopeType, scopeID string) (PackagePreviewSession, error) {
	var s PackagePreviewSession
	err := r.db.QueryRowContext(ctx, `SELECT session_id, user_id, scope_type, scope_id, artifact_id,
		extension_id, version, status, archive_hash, manifest_hash, content_tree_hash, risk_flags_json,
		required_confirmations_json, dependency_result_json, preview_result_json,
		verification_report_json, policy_version, security_policy_hash, verified_at, expires_at, consumed_at, created_at
		FROM extension_package_preview_sessions WHERE session_id = ? AND user_id = ? AND scope_type = ? AND scope_id = ?`,
		id, userID, scopeType, scopeID).Scan(&s.SessionID, &s.UserID, &s.ScopeType, &s.ScopeID,
		&s.ArtifactID, &s.ExtensionID, &s.Version, &s.Status, &s.ArchiveHash, &s.ManifestHash,
		&s.ContentTreeHash, &s.RiskFlagsJSON, &s.RequiredConfirmationsJSON, &s.DependencyResultJSON,
		&s.PreviewResultJSON, &s.VerificationReportJSON, &s.PolicyVersion, &s.SecurityPolicyHash,
		&s.VerifiedAt, &s.ExpiresAt, &s.ConsumedAt, &s.CreatedAt)
	return s, err
}

func (r *PackageRepository) InvalidateTrustPreviews(ctx context.Context, publisherID, artifactID, packageHash string) ([]string, error) {
	if publisherID == "" && artifactID == "" && packageHash == "" {
		return nil, errors.New("trust preview invalidation target required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT preview.session_id, preview.artifact_id, preview.extension_id
		FROM extension_package_preview_sessions preview
		JOIN extension_package_artifacts artifact ON artifact.artifact_id = preview.artifact_id
		WHERE preview.status IN ('ready','awaiting_confirmation')
		AND (? = '' OR artifact.publisher_id = ?)
		AND (? = '' OR preview.artifact_id = ?)
		AND (? = '' OR artifact.archive_hash = ?)`, publisherID, publisherID, artifactID, artifactID, packageHash, packageHash)
	if err != nil {
		return nil, err
	}
	type target struct{ sessionID, artifactID, extensionID string }
	var targets []target
	for rows.Next() {
		var item target
		if err := rows.Scan(&item.sessionID, &item.artifactID, &item.extensionID); err != nil {
			_ = rows.Close()
			return nil, err
		}
		targets = append(targets, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	extensions := make([]string, 0, len(targets))
	seen := map[string]bool{}
	for _, item := range targets {
		result, err := tx.ExecContext(ctx, `UPDATE extension_package_preview_sessions
			SET status = 'invalidated' WHERE session_id = ? AND status IN ('ready','awaiting_confirmation')`, item.sessionID)
		if err != nil {
			return nil, err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if count == 1 {
			if err := releaseArtifactReferenceTx(ctx, tx, item.artifactID, ArtifactReferencePreview, item.sessionID); err != nil {
				return nil, err
			}
			if !seen[item.extensionID] {
				seen[item.extensionID] = true
				extensions = append(extensions, item.extensionID)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return extensions, nil
}

func (r *PackageRepository) ConsumePreview(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var artifactID, extensionID string
	if err := tx.QueryRowContext(ctx, `SELECT artifact_id, extension_id FROM extension_package_preview_sessions WHERE session_id=?`, id).Scan(&artifactID, &extensionID); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `UPDATE extension_package_preview_sessions SET status = 'consumed', consumed_at = ? WHERE session_id = ? AND status IN ('ready','awaiting_confirmation')`, now, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return NewRepositoryError(RepositoryErrorConflict, errors.New("package preview session already consumed"))
	}
	if _, err := acquireArtifactReferenceTx(ctx, tx, artifactID, ArtifactReferenceInstallation, extensionID, time.Time{}); err != nil {
		return err
	}
	if err := releaseArtifactReferenceTx(ctx, tx, artifactID, ArtifactReferencePreview, id); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT artifact_id FROM extension_package_artifact_references
		WHERE reference_type=? AND reference_owner_id=? AND artifact_id<>? AND released_at=''`, ArtifactReferenceInstallation, extensionID, artifactID)
	if err != nil {
		return err
	}
	var previous []string
	for rows.Next() {
		var previousArtifactID string
		if err := rows.Scan(&previousArtifactID); err != nil {
			rows.Close()
			return err
		}
		previous = append(previous, previousArtifactID)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, previousArtifactID := range previous {
		if err := releaseArtifactReferenceTx(ctx, tx, previousArtifactID, ArtifactReferenceInstallation, extensionID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *PackageRepository) CancelPreview(ctx context.Context, id, userID, scopeType, scopeID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var artifactID string
	if err := tx.QueryRowContext(ctx, `SELECT artifact_id FROM extension_package_preview_sessions WHERE session_id=? AND user_id=? AND scope_type=? AND scope_id=?`, id, userID, scopeType, scopeID).Scan(&artifactID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE extension_package_preview_sessions SET status = 'cancelled' WHERE session_id = ? AND user_id = ? AND scope_type = ? AND scope_id = ? AND status IN ('ready','awaiting_confirmation')`, id, userID, scopeType, scopeID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("package preview session cannot be cancelled")
	}
	if err := releaseArtifactReferenceTx(ctx, tx, artifactID, ArtifactReferencePreview, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *PackageRepository) CreateOperation(ctx context.Context, op PackageOperationRecord) error {
	if op.Status == "" || op.Status == "created" {
		op.Status = string(PackageOperationPending)
	}
	op.IdempotencyKey = "legacy:" + op.OperationID
	hash := sha256.Sum256([]byte(hashLegacyOperationRequest(op)))
	op.RequestHash = "sha256:" + hex.EncodeToString(hash[:])
	_, _, err := r.CreateOrGetOperation(ctx, op)
	return err
}

func (r *PackageRepository) SetOperationGenerationEvidence(ctx context.Context, operationID, stableGeneration, targetGeneration, currentPointerJSON string, guard PackageWriteGuard) error {
	if err := verifyFencingTokenDB(ctx, r.db, guard); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `UPDATE extension_package_operations SET stable_generation=?, target_generation=?, current_pointer_json=?, updated_at=? WHERE operation_id=?`, stableGeneration, targetGeneration, currentPointerJSON, time.Now().UTC().Format(time.RFC3339Nano), operationID)
	return err
}

func (r *PackageRepository) SetOperation(ctx context.Context, id, status, step, code, detail string, completed bool, guard PackageWriteGuard) error {
	op, err := r.getAuthoritativeOperationByID(ctx, id)
	if err != nil {
		return err
	}
	source := PackageOperationStatus(op.Status)
	if op.Status == "created" || op.Status == "compensating" {
		normalized := PackageOperationPending
		if op.Status == "compensating" {
			normalized = PackageOperationInProgress
		}
		result, updateErr := r.db.ExecContext(ctx, `UPDATE extension_package_operations SET status=?, updated_at=? WHERE operation_id=? AND status=?`,
			string(normalized), time.Now().UTC().Format(time.RFC3339Nano), id, op.Status)
		if updateErr != nil {
			return storageOperationError("normalize legacy operation status", updateErr)
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return storageOperationError("inspect legacy operation normalization", rowsErr)
		}
		if rows != 1 {
			return operationStateError(OperationErrTransitionConflict, "operation status changed", nil)
		}
		source = normalized
	}
	target := PackageOperationStatus(status)
	if status == "compensating" {
		target = PackageOperationInProgress
	}
	if source == PackageOperationPending && (target == PackageOperationCompleted || target == PackageOperationRequiresRecovery) {
		if err := r.TransitionOperation(ctx, id, []PackageOperationStatus{source}, PackageOperationInProgress,
			PackageOperationTransition{CurrentStep: step}, guard); err != nil {
			return err
		}
		source = PackageOperationInProgress
	}
	return r.TransitionOperation(ctx, id, []PackageOperationStatus{source}, target,
		PackageOperationTransition{CurrentStep: step, ErrorCode: code, ErrorDetail: detail,
			RecoveryRequired: status == string(PackageOperationRequiresRecovery), Completed: completed}, guard)
}

func (r *PackageRepository) PutStep(ctx context.Context, step PackageOperationStep, guard PackageWriteGuard) error {
	inputHash := step.InputHash
	if inputHash == "" {
		hash := sha256.Sum256([]byte(step.OperationID + ":" + step.StepName + ":" + step.StableGeneration + ":" + step.TargetGeneration))
		inputHash = "sha256:" + hex.EncodeToString(hash[:])
	}
	_, _, err := r.BeginStep(ctx, step.OperationID, step.StepName, step.StepOrder, inputHash, guard)
	if err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `UPDATE extension_package_operation_steps SET stable_generation=?,
		target_generation=?, current_pointer_json=? WHERE operation_id=? AND step_name=? AND input_hash=?`,
		step.StableGeneration, step.TargetGeneration, step.CurrentPointerJSON, step.OperationID, step.StepName, inputHash); err != nil {
		return ClassifyRepositoryError("update operation step metadata", err)
	}
	evidence := step.SideEffectEvidence
	if evidence == "" {
		evidence = step.CurrentPointerJSON
	}
	if step.Status == "failed" {
		_, err = r.FailStep(ctx, step.OperationID, step.StepName, inputHash, step.ErrorCode, step.ErrorDetail, evidence, guard)
		return err
	}
	if step.Status == "completed" {
		_, err = r.CompleteStep(ctx, step.OperationID, step.StepName, inputHash, step.ResultJSON, evidence, guard)
		return err
	}
	return nil
}

func (r *PackageRepository) ListIncompleteOperations(ctx context.Context) ([]PackageOperationRecord, error) {
	query := `
		SELECT ` + packageOperationSelectColumns + `
		FROM extension_package_operations
		WHERE status NOT IN ('completed','failed','cancelled')
		ORDER BY started_at`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	operations, err := scanPackageOperations(rows)
	if err != nil {
		return nil, err
	}
	return operations, nil
}

func (r *PackageRepository) ListOperations(ctx context.Context, userID string, limit int) ([]PackageOperationRecord, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	query := `
		SELECT ` + packageOperationSelectColumns + `
		FROM extension_package_operations
		WHERE user_id = ?
		ORDER BY started_at DESC
		LIMIT ?`
	rows, err := r.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	operations, err := scanPackageOperations(rows)
	if err != nil {
		return nil, err
	}
	return operations, nil
}

func (r *PackageRepository) GetOperation(ctx context.Context, userID, operationID string) (PackageOperationRecord, []PackageOperationStep, error) {
	var op PackageOperationRecord
	query := `
		SELECT ` + packageOperationSelectColumns + `
		FROM extension_package_operations
		WHERE user_id = ? AND operation_id = ?`
	err := r.db.QueryRowContext(ctx, query, userID, operationID).Scan(
		&op.OperationID, &op.TraceID, &op.UserID, &op.ScopeType, &op.ScopeID, &op.ExtensionID,
		&op.TargetVersion, &op.OperationType, &op.Status, &op.CurrentStep, &op.ArtifactID,
		&op.PreviewSessionID, &op.ConfirmationsJSON, &op.ConfirmationClaimsJSON, &op.ErrorCode, &op.ErrorDetail,
		&op.StartedAt, &op.UpdatedAt, &op.CompletedAt, &op.StableGeneration, &op.TargetGeneration,
		&op.CurrentPointerJSON, &op.SnapshotRequirementHash,
	)
	if err != nil {
		return op, nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT step_id, operation_id, step_name, step_order, status,
		attempt_count, result_json, error_code, started_at, completed_at, stable_generation, target_generation, current_pointer_json
		FROM extension_package_operation_steps WHERE operation_id = ? ORDER BY step_order`, operationID)
	if err != nil {
		return op, nil, err
	}
	defer rows.Close()
	var steps []PackageOperationStep
	for rows.Next() {
		var step PackageOperationStep
		if err := rows.Scan(&step.StepID, &step.OperationID, &step.StepName, &step.StepOrder,
			&step.Status, &step.AttemptCount, &step.ResultJSON, &step.ErrorCode,
			&step.StartedAt, &step.CompletedAt, &step.StableGeneration, &step.TargetGeneration, &step.CurrentPointerJSON); err != nil {
			return op, nil, err
		}
		steps = append(steps, step)
	}
	return op, steps, rows.Err()
}

func (r *PackageRepository) GetCompletedOperationByPreview(ctx context.Context, userID, sessionID string) (PackageOperationRecord, error) {
	query := `
		SELECT ` + packageOperationSelectColumns + `
		FROM extension_package_operations
		WHERE user_id = ? AND preview_session_id = ?
			AND status = 'completed'
		ORDER BY completed_at DESC
		LIMIT 1`
	row := r.db.QueryRowContext(ctx, query, userID, sessionID)
	operation, err := scanPackageOperation(row)
	if err != nil {
		return PackageOperationRecord{}, err
	}
	return *operation, nil
}

func (r *PackageRepository) PutRollbackPoint(ctx context.Context, p PackageRollbackPoint) error {
	if p.RetentionState == "" {
		p.RetentionState = "active"
	}
	if p.RetentionUntil == "" {
		p.RetentionUntil = p.ExpiresAt
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO extension_package_rollback_points (
		rollback_point_id, extension_id, source_version, source_generation, artifact_id,
		definition_snapshot_json, module_snapshot_json, contribution_snapshot_json,
		permission_snapshot_json, scope_snapshot_json, config_snapshot_id, config_snapshot_json,
		secret_refs_json, resource_snapshot_json, migration_state_snapshot_json,
		user_data_migration_state_json, snapshot_hash, retention_state, retention_until,
		source_operation_id, installed_path, created_at, expires_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, p.RollbackPointID, p.ExtensionID,
		p.SourceVersion, p.SourceGeneration, p.ArtifactID, p.DefinitionSnapshotJSON,
		p.ModuleSnapshotJSON, p.ContributionSnapshotJSON, p.PermissionSnapshotJSON,
		p.ScopeSnapshotJSON, p.ConfigSnapshotID, p.ConfigSnapshotJSON, p.SecretRefsJSON,
		p.ResourceSnapshotJSON, p.MigrationStateSnapshotJSON, p.UserDataMigrationStateJSON,
		p.SnapshotHash, p.RetentionState, p.RetentionUntil, p.SourceOperationID,
		p.InstalledPath, p.CreatedAt, p.ExpiresAt)
	if err != nil {
		return err
	}
	expiresAt := time.Time{}
	if p.ExpiresAt != "" {
		expiresAt, err = time.Parse(time.RFC3339Nano, p.ExpiresAt)
		if err != nil {
			return err
		}
	}
	if _, err := acquireArtifactReferenceTx(ctx, tx, p.ArtifactID, ArtifactReferenceRollbackPoint, p.RollbackPointID, expiresAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *PackageRepository) GetRollbackPoint(ctx context.Context, extensionID, version string) (PackageRollbackPoint, error) {
	var p PackageRollbackPoint
	err := r.db.QueryRowContext(ctx, `SELECT rollback_point_id, extension_id, source_version,
		source_generation, artifact_id, definition_snapshot_json, module_snapshot_json,
		contribution_snapshot_json, permission_snapshot_json, scope_snapshot_json,
		config_snapshot_id, config_snapshot_json, secret_refs_json, resource_snapshot_json,
		migration_state_snapshot_json, user_data_migration_state_json, snapshot_hash,
		retention_state, retention_until, source_operation_id, installed_path, created_at, expires_at
		FROM extension_package_rollback_points WHERE extension_id = ? AND source_version = ?
		AND retention_state IN ('active', 'forward_recovery')
		ORDER BY created_at DESC LIMIT 1`, extensionID, version).Scan(&p.RollbackPointID, &p.ExtensionID,
		&p.SourceVersion, &p.SourceGeneration, &p.ArtifactID, &p.DefinitionSnapshotJSON,
		&p.ModuleSnapshotJSON, &p.ContributionSnapshotJSON, &p.PermissionSnapshotJSON,
		&p.ScopeSnapshotJSON, &p.ConfigSnapshotID, &p.ConfigSnapshotJSON, &p.SecretRefsJSON,
		&p.ResourceSnapshotJSON, &p.MigrationStateSnapshotJSON, &p.UserDataMigrationStateJSON,
		&p.SnapshotHash, &p.RetentionState, &p.RetentionUntil, &p.SourceOperationID,
		&p.InstalledPath, &p.CreatedAt, &p.ExpiresAt)
	if err != nil {
		return p, ClassifyRepositoryError("get rollback point", err)
	}
	return p, nil
}

func (r *PackageRepository) PutExport(ctx context.Context, ticket PackageExportTicket) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO extension_package_exports
		(export_id, user_id, extension_id, artifact_id, file_name, mime_type, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, ticket.ExportID, ticket.UserID, ticket.ExtensionID,
		ticket.ArtifactID, ticket.FileName, ticket.MIMEType, ticket.ExpiresAt, ticket.CreatedAt)
	if err != nil {
		return err
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, ticket.ExpiresAt)
	if err != nil {
		return err
	}
	if _, err := acquireArtifactReferenceTx(ctx, tx, ticket.ArtifactID, ArtifactReferenceExportLease, ticket.ExportID, expiresAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *PackageRepository) FinalizeOperationAndReleaseLeaseTxWithStep(ctx context.Context, operationID, extensionID string, fencingToken int64, finalizeStep string, step PackageOperationStep) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return storageOperationError("begin finalize operation and release lease with step", err)
	}
	defer tx.Rollback()
	var currentStatus string
	var artifactID string
	if err := tx.QueryRowContext(ctx, `SELECT status, artifact_id FROM extension_package_operations WHERE operation_id=?`,
		operationID).Scan(&currentStatus, &artifactID); err != nil {
		return classifyOperationRead("read operation for finalize with step", err)
	}
	if currentStatus != string(PackageOperationFinalizing) {
		return operationStateError(OperationErrTransitionConflict,
			"operation is not in finalizing state, current: "+currentStatus, nil)
	}
	var leaseOpID string
	var leaseFencingToken int64
	var leaseOwnerID string
	var leaseExpiresAt string
	leaseErr := tx.QueryRowContext(ctx,
		`SELECT operation_id, fencing_token, owner_id, lease_expires_at FROM extension_package_operation_leases WHERE extension_id=?`,
		extensionID).Scan(&leaseOpID, &leaseFencingToken, &leaseOwnerID, &leaseExpiresAt)
	if errors.Is(leaseErr, sql.ErrNoRows) {
		return operationStateError(PackageErrCodeLeaseMissing,
			"extension lease missing: operation cannot finalize without lease", nil)
	}
	if leaseErr != nil {
		return storageOperationError("read lease during finalization with step", leaseErr)
	}
	if leaseOpID != operationID {
		return operationStateError(PackageErrCodeLeaseFenced,
			"lease taken over by another operation: "+leaseOpID, nil)
	}
	if leaseFencingToken != fencingToken {
		return operationStateError(PackageErrCodeLeaseFenced,
			"fencing token mismatch during finalization", nil)
	}
	if leaseExpiresAt != "" {
		expiresAt, parseErr := time.Parse(time.RFC3339Nano, leaseExpiresAt)
		if parseErr == nil && expiresAt.Before(time.Now().UTC()) {
			return operationStateError(PackageErrCodeLeaseExpired,
				"lease has expired: operation cannot finalize", nil)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM extension_package_operation_leases WHERE extension_id=? AND operation_id=?`,
		extensionID, operationID); err != nil {
		return storageOperationError("delete lease during finalization with step", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	stepID := step.StepID
	if stepID == "" {
		stepID = operationID + ":" + finalizeStep
	}
	resultJSON := step.ResultJSON
	if resultJSON == "" {
		resultJSON = `{"finalized":true,"atomic":true}`
	}
	errorCode := step.ErrorCode
	startedAt := step.StartedAt
	if startedAt == "" {
		startedAt = now
	}
	completedAt := step.CompletedAt
	if completedAt == "" {
		completedAt = now
	}
	attemptCount := step.AttemptCount
	if attemptCount < 1 {
		attemptCount = 1
	}
	stepOrder := step.StepOrder
	if stepOrder == 0 {
		stepOrder = 9999
	}
	resultHash := step.ResultHash
	if resultHash == "" && resultJSON != "" {
		resultHash = fmt.Sprintf("%x", sha256.Sum256([]byte(resultJSON)))
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO extension_package_operation_steps (
		step_id, operation_id, step_name, step_order, status, attempt_count, result_json, error_code,
		started_at, completed_at, stable_generation, target_generation, current_pointer_json, input_hash, result_hash, updated_at, cas_version
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)`,
		stepID, operationID, finalizeStep, stepOrder, step.Status, attemptCount, resultJSON, errorCode,
		startedAt, completedAt, step.StableGeneration, step.TargetGeneration, step.CurrentPointerJSON, step.InputHash, resultHash, now); err != nil {
		if isSQLiteConstraintViolation(err) {
			if _, updErr := tx.ExecContext(ctx, `UPDATE extension_package_operation_steps SET status=?,
				result_json=?, result_hash=?, completed_at=?, stable_generation=?, target_generation=?, current_pointer_json=?,
				updated_at=?, cas_version=cas_version+1
				WHERE operation_id=? AND step_name=? AND status!='completed'`,
				step.Status, resultJSON, resultHash, completedAt, step.StableGeneration, step.TargetGeneration,
				step.CurrentPointerJSON, now, operationID, finalizeStep); updErr != nil {
				return storageOperationError("upsert finalize step during finalization with step", updErr)
			}
		} else {
			return storageOperationError("record finalize step during finalization with step", err)
		}
	}
	result, err := tx.ExecContext(ctx,
		`UPDATE extension_package_operations SET status=?, current_step=?, error_code='', error_detail='',
		recovery_required=0, updated_at=?, completed_at=?, lease_owner='', lease_expires_at=''
		WHERE operation_id=? AND status=?`,
		string(PackageOperationCompleted), "completed", now, now, operationID, string(PackageOperationFinalizing))
	if err != nil {
		return storageOperationError("finalize operation to completed with step", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return storageOperationError("inspect finalize operation with step", err)
	}
	if rows != 1 {
		return operationStateError(OperationErrTransitionConflict,
			"operation status changed during finalization", nil)
	}
	if artifactID != "" {
		if err := releaseArtifactReferenceTx(ctx, tx, artifactID, ArtifactReferenceOperation, operationID); err != nil {
			return storageOperationError("release terminal operation artifact during finalization with step", err)
		}
	}
	return tx.Commit()
}

func (r *PackageRepository) ListSteps(ctx context.Context, operationID string) ([]PackageOperationStep, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT step_id, operation_id, step_name, step_order, status,
		attempt_count, result_json, error_code, started_at, completed_at, stable_generation, target_generation,
		current_pointer_json, input_hash, updated_at, result_hash, cas_version
		FROM extension_package_operation_steps WHERE operation_id = ? ORDER BY step_order`, operationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var steps []PackageOperationStep
	for rows.Next() {
		var step PackageOperationStep
		if err := rows.Scan(&step.StepID, &step.OperationID, &step.StepName, &step.StepOrder,
			&step.Status, &step.AttemptCount, &step.ResultJSON, &step.ErrorCode,
			&step.StartedAt, &step.CompletedAt, &step.StableGeneration, &step.TargetGeneration,
			&step.CurrentPointerJSON, &step.InputHash, &step.UpdatedAt, &step.ResultHash, &step.CASVersion); err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (r *PackageRepository) GetExport(ctx context.Context, exportID, userID, extensionID string) (PackageExportTicket, error) {
	var ticket PackageExportTicket
	err := r.db.QueryRowContext(ctx, `SELECT export_id, user_id, extension_id, artifact_id,
		file_name, mime_type, expires_at, created_at FROM extension_package_exports
		WHERE export_id = ? AND user_id = ? AND extension_id = ?`, exportID, userID, extensionID).
		Scan(&ticket.ExportID, &ticket.UserID, &ticket.ExtensionID, &ticket.ArtifactID,
			&ticket.FileName, &ticket.MIMEType, &ticket.ExpiresAt, &ticket.CreatedAt)
	return ticket, err
}

func (r *PackageRepository) DB() *sql.DB {
	return r.db
}

func (r *PackageRepository) FinalizeOperationAndReleaseLeaseTx(ctx context.Context, operationID, extensionID string, fencingToken int64, finalizeStep string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return storageOperationError("begin finalize operation and release lease", err)
	}
	defer tx.Rollback()
	var currentStatus string
	var artifactID string
	if err := tx.QueryRowContext(ctx, `SELECT status, artifact_id FROM extension_package_operations WHERE operation_id=?`,
		operationID).Scan(&currentStatus, &artifactID); err != nil {
		return classifyOperationRead("read operation for finalize", err)
	}
	if currentStatus != string(PackageOperationFinalizing) {
		return operationStateError(OperationErrTransitionConflict,
			"operation is not in finalizing state, current: "+currentStatus, nil)
	}
	var leaseOpID string
	var leaseFencingToken int64
	var leaseExpiresAt string
	leaseErr := tx.QueryRowContext(ctx,
		`SELECT operation_id, fencing_token, lease_expires_at FROM extension_package_operation_leases WHERE extension_id=?`,
		extensionID).Scan(&leaseOpID, &leaseFencingToken, &leaseExpiresAt)
	if errors.Is(leaseErr, sql.ErrNoRows) {
		return operationStateError(PackageErrCodeLeaseMissing,
			"extension lease missing: operation cannot finalize without lease", nil)
	}
	if leaseErr != nil {
		return storageOperationError("read lease during finalization", leaseErr)
	}
	if leaseOpID != operationID {
		return operationStateError(PackageErrCodeLeaseFenced,
			"lease taken over by another operation: "+leaseOpID, nil)
	}
	if leaseFencingToken != fencingToken {
		return operationStateError(PackageErrCodeLeaseFenced,
			"fencing token mismatch during finalization", nil)
	}
	if leaseExpiresAt != "" {
		expiresAt, parseErr := time.Parse(time.RFC3339Nano, leaseExpiresAt)
		if parseErr == nil && expiresAt.Before(time.Now().UTC()) {
			return operationStateError(PackageErrCodeLeaseExpired,
				"lease has expired: operation cannot finalize", nil)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM extension_package_operation_leases WHERE extension_id=? AND operation_id=?`,
		extensionID, operationID); err != nil {
		return storageOperationError("delete lease during finalization", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	stepID := operationID + ":" + finalizeStep
	inputHash := "sha256:" + fmt.Sprintf("%x", sha256.Sum256([]byte(stepID)))
	resultJSON := `{"finalized":true,"atomic":true}`
	resultHash := fmt.Sprintf("%x", sha256.Sum256([]byte(resultJSON)))
	if _, err := tx.ExecContext(ctx, `INSERT INTO extension_package_operation_steps (
		step_id, operation_id, step_name, step_order, status, attempt_count, result_json, error_code,
		started_at, completed_at, input_hash, result_hash, updated_at, cas_version
	) VALUES (?, ?, ?, ?, 'completed', 1, ?, '', ?, ?, ?, ?, ?, 1)`,
		stepID, operationID, finalizeStep, 9999, resultJSON, now, now, inputHash, resultHash, now); err != nil {
		if isSQLiteConstraintViolation(err) {
			if _, updErr := tx.ExecContext(ctx, `UPDATE extension_package_operation_steps SET status='completed',
				result_json=?, result_hash=?, completed_at=?, updated_at=?, cas_version=cas_version+1
				WHERE operation_id=? AND step_name=? AND status!='completed'`,
				resultJSON, resultHash, now, now, operationID, finalizeStep); updErr != nil {
				return storageOperationError("upsert finalize step during finalization", updErr)
			}
		} else {
			return storageOperationError("record finalize step during finalization", err)
		}
	}
	result, err := tx.ExecContext(ctx,
		`UPDATE extension_package_operations SET status=?, current_step=?, error_code='', error_detail='',
		recovery_required=0, updated_at=?, completed_at=?, lease_owner='', lease_expires_at=''
		WHERE operation_id=? AND status=?`,
		string(PackageOperationCompleted), "completed", now, now, operationID, string(PackageOperationFinalizing))
	if err != nil {
		return storageOperationError("finalize operation to completed", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return storageOperationError("inspect finalize operation", err)
	}
	if rows != 1 {
		return operationStateError(OperationErrTransitionConflict,
			"operation status changed during finalization", nil)
	}
	if artifactID != "" {
		if err := releaseArtifactReferenceTx(ctx, tx, artifactID, ArtifactReferenceOperation, operationID); err != nil {
			return storageOperationError("release terminal operation artifact during finalization", err)
		}
	}
	return tx.Commit()
}
