package kernel

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

type PackageWriteGuard struct {
	ExtensionID  string
	FencingToken int64
}

func (g PackageWriteGuard) IsZero() bool {
	return g.FencingToken == 0 || g.ExtensionID == ""
}

func verifyFencingTokenTx(ctx context.Context, tx *sql.Tx, guard PackageWriteGuard) error {
	if guard.IsZero() {
		return nil
	}
	var currentToken int64
	var expiresAt string
	err := tx.QueryRowContext(ctx,
		`SELECT fencing_token, lease_expires_at FROM extension_package_operation_leases WHERE extension_id = ?`,
		guard.ExtensionID).Scan(&currentToken, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return operationStateError(PackageErrCodeLeaseFenced, "no active lease for extension", nil)
		}
		return storageOperationError("verify fencing token", err)
	}
	if currentToken != guard.FencingToken {
		return operationStateError(PackageErrCodeLeaseFenced, "fencing token mismatch: lease taken over by newer operation", nil)
	}
	now := time.Now().UTC()
	if expiresAt != "" {
		leaseExpiry, parseErr := time.Parse(time.RFC3339Nano, expiresAt)
		if parseErr == nil && leaseExpiry.Before(now) {
			return operationStateError(PackageErrCodeLeaseLost, "lease has expired", nil)
		}
	}
	return nil
}

func verifyFencingTokenDB(ctx context.Context, db *sql.DB, guard PackageWriteGuard) error {
	if guard.IsZero() {
		return nil
	}
	var currentToken int64
	var expiresAt string
	err := db.QueryRowContext(ctx,
		`SELECT fencing_token, lease_expires_at FROM extension_package_operation_leases WHERE extension_id = ?`,
		guard.ExtensionID).Scan(&currentToken, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return operationStateError(PackageErrCodeLeaseFenced, "no active lease for extension", nil)
		}
		return storageOperationError("verify fencing token", err)
	}
	if currentToken != guard.FencingToken {
		return operationStateError(PackageErrCodeLeaseFenced, "fencing token mismatch: lease taken over by newer operation", nil)
	}
	now := time.Now().UTC()
	if expiresAt != "" {
		leaseExpiry, parseErr := time.Parse(time.RFC3339Nano, expiresAt)
		if parseErr == nil && leaseExpiry.Before(now) {
			return operationStateError(PackageErrCodeLeaseLost, "lease has expired", nil)
		}
	}
	return nil
}

func (r *PackageRepository) VerifyFencingTokenInContext(ctx context.Context, guard PackageWriteGuard) error {
	if tx, ok := sqlite.TxFromContext(ctx); ok {
		return verifyFencingTokenTx(ctx, tx, guard)
	}
	return verifyFencingTokenDB(ctx, r.db, guard)
}

func lookupExtensionIDByOperationTx(ctx context.Context, tx *sql.Tx, operationID string) (string, error) {
	var extensionID string
	err := tx.QueryRowContext(ctx,
		`SELECT extension_id FROM extension_package_operations WHERE operation_id = ?`,
		operationID).Scan(&extensionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", operationStateError(OperationErrNotFound, "operation not found for fencing lookup", nil)
		}
		return "", storageOperationError("lookup extension id for fencing", err)
	}
	return extensionID, nil
}

type PackageQuarantineMetadata struct {
	QuarantineID             string
	OperationID              string
	ExtensionID              string
	GenerationQuarantinePath string
	CurrentQuarantinePath    string
	OriginalGenerationPath   string
	OriginalCurrentPath      string
	TreeHash                 string
	ArtifactID               string
	State                    string
	CreatedAt                string
	ReleasedAt               string
}

var validQuarantineStates = map[string]struct{}{
	"active": {}, "restoring": {}, "restored": {}, "finalizing": {}, "finalized": {}, "released": {},
}

func (r *PackageRepository) PutQuarantineMetadata(ctx context.Context, qm PackageQuarantineMetadata, guard PackageWriteGuard) error {
	if qm.State != "" {
		if _, ok := validQuarantineStates[qm.State]; !ok {
			return errors.New("kernel: invalid quarantine state: " + qm.State)
		}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return storageOperationError("begin quarantine metadata", err)
	}
	defer tx.Rollback()
	if err := verifyFencingTokenTx(ctx, tx, guard); err != nil {
		return err
	}
	if qm.CreatedAt == "" {
		qm.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if qm.State == "" {
		qm.State = "active"
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO package_quarantine_metadata (
		quarantine_id, operation_id, extension_id, generation_quarantine_path,
		current_quarantine_path, original_generation_path, original_current_path,
		tree_hash, artifact_id, state, created_at, released_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(quarantine_id) DO UPDATE SET generation_quarantine_path=excluded.generation_quarantine_path,
		current_quarantine_path=excluded.current_quarantine_path,
		original_generation_path=excluded.original_generation_path,
		original_current_path=excluded.original_current_path,
		tree_hash=excluded.tree_hash, state=excluded.state,
		released_at=excluded.released_at`,
		qm.QuarantineID, qm.OperationID, qm.ExtensionID, qm.GenerationQuarantinePath,
		qm.CurrentQuarantinePath, qm.OriginalGenerationPath, qm.OriginalCurrentPath,
		qm.TreeHash, qm.ArtifactID, qm.State, qm.CreatedAt, qm.ReleasedAt)
	if err != nil {
		return storageOperationError("insert quarantine metadata", err)
	}
	return tx.Commit()
}

func (r *PackageRepository) GetQuarantineMetadata(ctx context.Context, extensionID string) (PackageQuarantineMetadata, error) {
	var qm PackageQuarantineMetadata
	err := r.db.QueryRowContext(ctx, `SELECT quarantine_id, operation_id, extension_id,
		generation_quarantine_path, current_quarantine_path, original_generation_path,
		original_current_path, tree_hash, artifact_id, state, created_at, released_at
		FROM package_quarantine_metadata WHERE extension_id = ? AND state = 'active' ORDER BY created_at DESC LIMIT 1`,
		extensionID).Scan(&qm.QuarantineID, &qm.OperationID, &qm.ExtensionID,
		&qm.GenerationQuarantinePath, &qm.CurrentQuarantinePath, &qm.OriginalGenerationPath,
		&qm.OriginalCurrentPath, &qm.TreeHash, &qm.ArtifactID, &qm.State, &qm.CreatedAt, &qm.ReleasedAt)
	if err != nil {
		return PackageQuarantineMetadata{}, classifyOperationRead("read quarantine metadata", err)
	}
	return qm, nil
}

func (r *PackageRepository) GetBlockingQuarantineMetadata(ctx context.Context, extensionID string) (PackageQuarantineMetadata, error) {
	var qm PackageQuarantineMetadata
	err := r.db.QueryRowContext(ctx, `SELECT quarantine_id, operation_id, extension_id,
		generation_quarantine_path, current_quarantine_path, original_generation_path,
		original_current_path, tree_hash, artifact_id, state, created_at, released_at
		FROM package_quarantine_metadata WHERE extension_id = ? AND state IN ('active', 'restoring', 'finalizing')
		ORDER BY created_at DESC LIMIT 1`,
		extensionID).Scan(&qm.QuarantineID, &qm.OperationID, &qm.ExtensionID,
		&qm.GenerationQuarantinePath, &qm.CurrentQuarantinePath, &qm.OriginalGenerationPath,
		&qm.OriginalCurrentPath, &qm.TreeHash, &qm.ArtifactID, &qm.State, &qm.CreatedAt, &qm.ReleasedAt)
	if err != nil {
		return PackageQuarantineMetadata{}, classifyOperationRead("read blocking quarantine metadata", err)
	}
	return qm, nil
}

func (r *PackageRepository) GetQuarantineMetadataByOperation(ctx context.Context, operationID string) (PackageQuarantineMetadata, error) {
	var qm PackageQuarantineMetadata
	err := r.db.QueryRowContext(ctx, `SELECT quarantine_id, operation_id, extension_id,
		generation_quarantine_path, current_quarantine_path, original_generation_path,
		original_current_path, tree_hash, artifact_id, state, created_at, released_at
		FROM package_quarantine_metadata WHERE operation_id = ? ORDER BY created_at DESC LIMIT 1`,
		operationID).Scan(&qm.QuarantineID, &qm.OperationID, &qm.ExtensionID,
		&qm.GenerationQuarantinePath, &qm.CurrentQuarantinePath, &qm.OriginalGenerationPath,
		&qm.OriginalCurrentPath, &qm.TreeHash, &qm.ArtifactID, &qm.State, &qm.CreatedAt, &qm.ReleasedAt)
	if err != nil {
		return PackageQuarantineMetadata{}, classifyOperationRead("read quarantine metadata by operation", err)
	}
	return qm, nil
}

func (r *PackageRepository) ReleaseQuarantineMetadata(ctx context.Context, quarantineID string, guard PackageWriteGuard) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return storageOperationError("begin release quarantine", err)
	}
	defer tx.Rollback()
	if err := verifyFencingTokenTx(ctx, tx, guard); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE package_quarantine_metadata SET state='released', released_at=? WHERE quarantine_id=? AND state IN ('active', 'finalized')`,
		time.Now().UTC().Format(time.RFC3339Nano), quarantineID)
	if err != nil {
		return storageOperationError("release quarantine metadata", err)
	}
	return tx.Commit()
}

type PackageConsistencyFinding struct {
	FindingID         string
	Metric            string
	Count             int64
	ResourceIDsJSON   string
	ErrorDetail       string
	RecommendedAction string
	CreatedAt         string
	ResolvedAt        string
}

func (r *PackageRepository) PutConsistencyFinding(ctx context.Context, f PackageConsistencyFinding) error {
	if f.CreatedAt == "" {
		f.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO package_consistency_findings (
		finding_id, metric, count, resource_ids_json, error_detail, recommended_action, created_at, resolved_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, f.FindingID, f.Metric, f.Count, f.ResourceIDsJSON,
		f.ErrorDetail, f.RecommendedAction, f.CreatedAt, f.ResolvedAt)
	return err
}

func (r *PackageRepository) ListUnresolvedConsistencyFindings(ctx context.Context) ([]PackageConsistencyFinding, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT finding_id, metric, count, resource_ids_json,
		error_detail, recommended_action, created_at, resolved_at
		FROM package_consistency_findings WHERE resolved_at = '' ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PackageConsistencyFinding
	for rows.Next() {
		var f PackageConsistencyFinding
		if err := rows.Scan(&f.FindingID, &f.Metric, &f.Count, &f.ResourceIDsJSON,
			&f.ErrorDetail, &f.RecommendedAction, &f.CreatedAt, &f.ResolvedAt); err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, rows.Err()
}

func (r *PackageRepository) ResolveConsistencyFindings(ctx context.Context, metric string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE package_consistency_findings SET resolved_at=? WHERE metric=? AND resolved_at=''`,
		time.Now().UTC().Format(time.RFC3339Nano), metric)
	return err
}
