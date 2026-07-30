package kernel

import (
	"context"
	"database/sql"
	"errors"
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
	VerifiedAt                string
	ExpiresAt                 string
	ConsumedAt                string
	CreatedAt                 string
}

type PackageOperationRecord struct {
	OperationID       string
	TraceID           string
	UserID            string
	ScopeType         string
	ScopeID           string
	ExtensionID       string
	TargetVersion     string
	OperationType     string
	Status            string
	CurrentStep       string
	ArtifactID        string
	PreviewSessionID  string
	ConfirmationsJSON string
	ErrorCode         string
	ErrorDetail       string
	StartedAt         string
	UpdatedAt         string
	CompletedAt       string
}

type PackageOperationStep struct {
	StepID       string
	OperationID  string
	StepName     string
	StepOrder    int
	Status       string
	AttemptCount int
	ResultJSON   string
	ErrorCode    string
	StartedAt    string
	CompletedAt  string
}

type PackageRollbackPoint struct {
	RollbackPointID          string
	ExtensionID              string
	SourceVersion            string
	SourceGeneration         int64
	ArtifactID               string
	DefinitionSnapshotJSON   string
	ModuleSnapshotJSON       string
	ContributionSnapshotJSON string
	PermissionSnapshotJSON   string
	ScopeSnapshotJSON        string
	ConfigSnapshotID         string
	InstalledPath            string
	CreatedAt                string
	ExpiresAt                string
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

func (r *PackageRepository) PutArtifact(ctx context.Context, a PackageArtifact) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO extension_package_artifacts (
		artifact_id, extension_id, version, archive_hash, manifest_hash, content_tree_hash,
		artifact_hash, archive_path, installed_path, size_bytes, signature_status, signer_key_id,
		publisher_id, trust_decision, verification_report_json, created_at, verified_at, quarantined_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT DO UPDATE SET extension_id=excluded.extension_id, version=excluded.version,
	manifest_hash=excluded.manifest_hash, content_tree_hash=excluded.content_tree_hash,
	artifact_hash=excluded.artifact_hash, archive_path=excluded.archive_path,
	installed_path=CASE WHEN excluded.installed_path = '' THEN extension_package_artifacts.installed_path ELSE excluded.installed_path END, signature_status=excluded.signature_status,
	signer_key_id=excluded.signer_key_id, publisher_id=excluded.publisher_id,
	trust_decision=excluded.trust_decision, verification_report_json=excluded.verification_report_json,
	verified_at=excluded.verified_at, quarantined_at=excluded.quarantined_at`,
		a.ArtifactID, a.ExtensionID, a.Version, a.ArchiveHash, a.ManifestHash, a.ContentTreeHash,
		a.ArtifactHash, a.ArchivePath, a.InstalledPath, a.SizeBytes, a.SignatureStatus, a.SignerKeyID,
		a.PublisherID, a.TrustDecision, a.VerificationReportJSON, a.CreatedAt, a.VerifiedAt, a.QuarantinedAt)
	return err
}

func (r *PackageRepository) GetArtifact(ctx context.Context, id string) (PackageArtifact, error) {
	var a PackageArtifact
	err := r.db.QueryRowContext(ctx, `SELECT artifact_id, extension_id, version, archive_hash,
		manifest_hash, content_tree_hash, artifact_hash, archive_path, installed_path, size_bytes,
		signature_status, signer_key_id, publisher_id, trust_decision, verification_report_json,
		created_at, verified_at, quarantined_at FROM extension_package_artifacts WHERE artifact_id = ?`, id).
		Scan(&a.ArtifactID, &a.ExtensionID, &a.Version, &a.ArchiveHash, &a.ManifestHash,
			&a.ContentTreeHash, &a.ArtifactHash, &a.ArchivePath, &a.InstalledPath, &a.SizeBytes,
			&a.SignatureStatus, &a.SignerKeyID, &a.PublisherID, &a.TrustDecision,
			&a.VerificationReportJSON, &a.CreatedAt, &a.VerifiedAt, &a.QuarantinedAt)
	return a, err
}

func (r *PackageRepository) GetArtifactByVersion(ctx context.Context, extensionID, version string) (PackageArtifact, error) {
	var id string
	if err := r.db.QueryRowContext(ctx, `SELECT artifact_id FROM extension_package_artifacts WHERE extension_id = ? AND version = ? AND quarantined_at = '' ORDER BY created_at DESC LIMIT 1`, extensionID, version).Scan(&id); err != nil {
		return PackageArtifact{}, err
	}
	return r.GetArtifact(ctx, id)
}

func (r *PackageRepository) GetArtifactByIdentity(ctx context.Context, extensionID, version, archiveHash string) (PackageArtifact, error) {
	var id string
	if err := r.db.QueryRowContext(ctx, `SELECT artifact_id FROM extension_package_artifacts WHERE extension_id = ? AND version = ? AND archive_hash = ? LIMIT 1`, extensionID, version, archiveHash).Scan(&id); err != nil {
		return PackageArtifact{}, err
	}
	return r.GetArtifact(ctx, id)
}

func (r *PackageRepository) PutPreview(ctx context.Context, s PackagePreviewSession) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO extension_package_preview_sessions (
		session_id, user_id, scope_type, scope_id, artifact_id, extension_id, version, status,
		archive_hash, manifest_hash, content_tree_hash, risk_flags_json, required_confirmations_json,
		dependency_result_json, preview_result_json, verification_report_json, policy_version,
		verified_at, expires_at, consumed_at, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.SessionID, s.UserID, s.ScopeType, s.ScopeID, s.ArtifactID, s.ExtensionID, s.Version,
		s.Status, s.ArchiveHash, s.ManifestHash, s.ContentTreeHash, s.RiskFlagsJSON,
		s.RequiredConfirmationsJSON, s.DependencyResultJSON, s.PreviewResultJSON,
		s.VerificationReportJSON, s.PolicyVersion, s.VerifiedAt, s.ExpiresAt, s.ConsumedAt, s.CreatedAt)
	return err
}

func (r *PackageRepository) GetPreview(ctx context.Context, id, userID, scopeType, scopeID string) (PackagePreviewSession, error) {
	var s PackagePreviewSession
	err := r.db.QueryRowContext(ctx, `SELECT session_id, user_id, scope_type, scope_id, artifact_id,
		extension_id, version, status, archive_hash, manifest_hash, content_tree_hash, risk_flags_json,
		required_confirmations_json, dependency_result_json, preview_result_json,
		verification_report_json, policy_version, verified_at, expires_at, consumed_at, created_at
		FROM extension_package_preview_sessions WHERE session_id = ? AND user_id = ? AND scope_type = ? AND scope_id = ?`,
		id, userID, scopeType, scopeID).Scan(&s.SessionID, &s.UserID, &s.ScopeType, &s.ScopeID,
		&s.ArtifactID, &s.ExtensionID, &s.Version, &s.Status, &s.ArchiveHash, &s.ManifestHash,
		&s.ContentTreeHash, &s.RiskFlagsJSON, &s.RequiredConfirmationsJSON, &s.DependencyResultJSON,
		&s.PreviewResultJSON, &s.VerificationReportJSON, &s.PolicyVersion, &s.VerifiedAt,
		&s.ExpiresAt, &s.ConsumedAt, &s.CreatedAt)
	return s, err
}

func (r *PackageRepository) ConsumePreview(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := r.db.ExecContext(ctx, `UPDATE extension_package_preview_sessions SET status = 'consumed', consumed_at = ? WHERE session_id = ? AND status IN ('ready','awaiting_confirmation')`, now, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("package preview session already consumed")
	}
	return nil
}

func (r *PackageRepository) CancelPreview(ctx context.Context, id, userID, scopeType, scopeID string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE extension_package_preview_sessions SET status = 'cancelled' WHERE session_id = ? AND user_id = ? AND scope_type = ? AND scope_id = ? AND status IN ('ready','awaiting_confirmation')`, id, userID, scopeType, scopeID)
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
	return nil
}

func (r *PackageRepository) CreateOperation(ctx context.Context, op PackageOperationRecord) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO extension_package_operations (
		operation_id, trace_id, user_id, scope_type, scope_id, extension_id, target_version,
		operation_type, status, current_step, artifact_id, preview_session_id, confirmations_json,
		error_code, error_detail, started_at, updated_at, completed_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, op.OperationID,
		op.TraceID, op.UserID, op.ScopeType, op.ScopeID, op.ExtensionID, op.TargetVersion,
		op.OperationType, op.Status, op.CurrentStep, op.ArtifactID, op.PreviewSessionID,
		op.ConfirmationsJSON, op.ErrorCode, op.ErrorDetail, op.StartedAt, op.UpdatedAt, op.CompletedAt)
	return err
}

func (r *PackageRepository) SetOperation(ctx context.Context, id, status, step, code, detail string, completed bool) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	completedAt := ""
	if completed {
		completedAt = now
	}
	_, err := r.db.ExecContext(ctx, `UPDATE extension_package_operations SET status=?, current_step=?, error_code=?, error_detail=?, updated_at=?, completed_at=CASE WHEN ? <> '' THEN ? ELSE completed_at END WHERE operation_id=?`, status, step, code, detail, now, completedAt, completedAt, id)
	return err
}

func (r *PackageRepository) PutStep(ctx context.Context, step PackageOperationStep) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO extension_package_operation_steps (
		step_id, operation_id, step_name, step_order, status, attempt_count, result_json,
		error_code, started_at, completed_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(operation_id, step_name) DO UPDATE SET status=excluded.status,
	attempt_count=extension_package_operation_steps.attempt_count+1, result_json=excluded.result_json,
	error_code=excluded.error_code, started_at=CASE WHEN extension_package_operation_steps.started_at='' THEN excluded.started_at ELSE extension_package_operation_steps.started_at END,
	completed_at=excluded.completed_at`, step.StepID, step.OperationID, step.StepName, step.StepOrder,
		step.Status, step.AttemptCount, step.ResultJSON, step.ErrorCode, step.StartedAt, step.CompletedAt)
	return err
}

func (r *PackageRepository) ListIncompleteOperations(ctx context.Context) ([]PackageOperationRecord, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT operation_id, trace_id, user_id, scope_type, scope_id,
		extension_id, target_version, operation_type, status, current_step, artifact_id,
		preview_session_id, confirmations_json, error_code, error_detail, started_at, updated_at,
		completed_at FROM extension_package_operations WHERE status NOT IN ('completed','failed') ORDER BY started_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PackageOperationRecord
	for rows.Next() {
		var op PackageOperationRecord
		if err := rows.Scan(&op.OperationID, &op.TraceID, &op.UserID, &op.ScopeType, &op.ScopeID,
			&op.ExtensionID, &op.TargetVersion, &op.OperationType, &op.Status, &op.CurrentStep,
			&op.ArtifactID, &op.PreviewSessionID, &op.ConfirmationsJSON, &op.ErrorCode,
			&op.ErrorDetail, &op.StartedAt, &op.UpdatedAt, &op.CompletedAt); err != nil {
			return nil, err
		}
		result = append(result, op)
	}
	return result, rows.Err()
}

func (r *PackageRepository) ListOperations(ctx context.Context, userID string, limit int) ([]PackageOperationRecord, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT operation_id, trace_id, user_id, scope_type, scope_id,
		extension_id, target_version, operation_type, status, current_step, artifact_id,
		preview_session_id, confirmations_json, error_code, error_detail, started_at, updated_at,
		completed_at FROM extension_package_operations WHERE user_id = ? ORDER BY started_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPackageOperations(rows)
}

func (r *PackageRepository) GetOperation(ctx context.Context, userID, operationID string) (PackageOperationRecord, []PackageOperationStep, error) {
	var op PackageOperationRecord
	err := r.db.QueryRowContext(ctx, `SELECT operation_id, trace_id, user_id, scope_type, scope_id,
		extension_id, target_version, operation_type, status, current_step, artifact_id,
		preview_session_id, confirmations_json, error_code, error_detail, started_at, updated_at,
		completed_at FROM extension_package_operations WHERE user_id = ? AND operation_id = ?`, userID, operationID).
		Scan(&op.OperationID, &op.TraceID, &op.UserID, &op.ScopeType, &op.ScopeID, &op.ExtensionID,
			&op.TargetVersion, &op.OperationType, &op.Status, &op.CurrentStep, &op.ArtifactID,
			&op.PreviewSessionID, &op.ConfirmationsJSON, &op.ErrorCode, &op.ErrorDetail,
			&op.StartedAt, &op.UpdatedAt, &op.CompletedAt)
	if err != nil {
		return op, nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT step_id, operation_id, step_name, step_order, status,
		attempt_count, result_json, error_code, started_at, completed_at
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
			&step.StartedAt, &step.CompletedAt); err != nil {
			return op, nil, err
		}
		steps = append(steps, step)
	}
	return op, steps, rows.Err()
}

func (r *PackageRepository) GetCompletedOperationByPreview(ctx context.Context, userID, sessionID string) (PackageOperationRecord, error) {
	var op PackageOperationRecord
	err := r.db.QueryRowContext(ctx, `SELECT operation_id, trace_id, user_id, scope_type, scope_id,
		extension_id, target_version, operation_type, status, current_step, artifact_id,
		preview_session_id, confirmations_json, error_code, error_detail, started_at, updated_at,
		completed_at FROM extension_package_operations WHERE user_id = ? AND preview_session_id = ?
		AND status = 'completed' ORDER BY completed_at DESC LIMIT 1`, userID, sessionID).
		Scan(&op.OperationID, &op.TraceID, &op.UserID, &op.ScopeType, &op.ScopeID, &op.ExtensionID,
			&op.TargetVersion, &op.OperationType, &op.Status, &op.CurrentStep, &op.ArtifactID,
			&op.PreviewSessionID, &op.ConfirmationsJSON, &op.ErrorCode, &op.ErrorDetail,
			&op.StartedAt, &op.UpdatedAt, &op.CompletedAt)
	return op, err
}

func scanPackageOperations(rows *sql.Rows) ([]PackageOperationRecord, error) {
	var result []PackageOperationRecord
	for rows.Next() {
		var op PackageOperationRecord
		if err := rows.Scan(&op.OperationID, &op.TraceID, &op.UserID, &op.ScopeType, &op.ScopeID,
			&op.ExtensionID, &op.TargetVersion, &op.OperationType, &op.Status, &op.CurrentStep,
			&op.ArtifactID, &op.PreviewSessionID, &op.ConfirmationsJSON, &op.ErrorCode,
			&op.ErrorDetail, &op.StartedAt, &op.UpdatedAt, &op.CompletedAt); err != nil {
			return nil, err
		}
		result = append(result, op)
	}
	return result, rows.Err()
}

func (r *PackageRepository) PutRollbackPoint(ctx context.Context, p PackageRollbackPoint) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO extension_package_rollback_points (
		rollback_point_id, extension_id, source_version, source_generation, artifact_id,
		definition_snapshot_json, module_snapshot_json, contribution_snapshot_json,
		permission_snapshot_json, scope_snapshot_json, config_snapshot_id, installed_path,
		created_at, expires_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, p.RollbackPointID, p.ExtensionID,
		p.SourceVersion, p.SourceGeneration, p.ArtifactID, p.DefinitionSnapshotJSON,
		p.ModuleSnapshotJSON, p.ContributionSnapshotJSON, p.PermissionSnapshotJSON,
		p.ScopeSnapshotJSON, p.ConfigSnapshotID, p.InstalledPath, p.CreatedAt, p.ExpiresAt)
	return err
}

func (r *PackageRepository) GetRollbackPoint(ctx context.Context, extensionID, version string) (PackageRollbackPoint, error) {
	var p PackageRollbackPoint
	err := r.db.QueryRowContext(ctx, `SELECT rollback_point_id, extension_id, source_version,
		source_generation, artifact_id, definition_snapshot_json, module_snapshot_json,
		contribution_snapshot_json, permission_snapshot_json, scope_snapshot_json,
		config_snapshot_id, installed_path, created_at, expires_at
		FROM extension_package_rollback_points WHERE extension_id = ? AND source_version = ?
		ORDER BY created_at DESC LIMIT 1`, extensionID, version).Scan(&p.RollbackPointID, &p.ExtensionID,
		&p.SourceVersion, &p.SourceGeneration, &p.ArtifactID, &p.DefinitionSnapshotJSON,
		&p.ModuleSnapshotJSON, &p.ContributionSnapshotJSON, &p.PermissionSnapshotJSON,
		&p.ScopeSnapshotJSON, &p.ConfigSnapshotID, &p.InstalledPath, &p.CreatedAt, &p.ExpiresAt)
	return p, err
}

func (r *PackageRepository) PutExport(ctx context.Context, ticket PackageExportTicket) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO extension_package_exports
		(export_id, user_id, extension_id, artifact_id, file_name, mime_type, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, ticket.ExportID, ticket.UserID, ticket.ExtensionID,
		ticket.ArtifactID, ticket.FileName, ticket.MIMEType, ticket.ExpiresAt, ticket.CreatedAt)
	return err
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
