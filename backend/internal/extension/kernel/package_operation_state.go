package kernel

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type PackageOperationStatus string

const (
	PackageOperationPending          PackageOperationStatus = "pending"
	PackageOperationInProgress       PackageOperationStatus = "in_progress"
	PackageOperationCompleted        PackageOperationStatus = "completed"
	PackageOperationFailed           PackageOperationStatus = "failed"
	PackageOperationCancelled        PackageOperationStatus = "cancelled"
	PackageOperationRequiresRecovery PackageOperationStatus = "requires_recovery"
	PackageOperationFinalizing       PackageOperationStatus = "finalizing"
	PackageOperationReleasePending   PackageOperationStatus = "release_pending"
)

const (
	OperationErrIdempotencyConflict = "PACKAGE_OPERATION_IDEMPOTENCY_CONFLICT"
	OperationErrLeaseConflict       = "PACKAGE_OPERATION_LEASE_CONFLICT"
	OperationErrTransitionConflict  = "PACKAGE_OPERATION_TRANSITION_CONFLICT"
	OperationErrStepInputConflict   = "PACKAGE_OPERATION_STEP_INPUT_CONFLICT"
	OperationErrSideEffectConflict  = "PACKAGE_OPERATION_SIDE_EFFECT_CONFLICT"
	OperationErrCancelNotAllowed    = "PACKAGE_OPERATION_CANCEL_NOT_ALLOWED"
	OperationErrRetryNotAllowed     = "PACKAGE_OPERATION_RETRY_NOT_ALLOWED"
	OperationErrRecoveryNotAllowed  = "PACKAGE_OPERATION_RECOVERY_NOT_ALLOWED"
	OperationErrNotFound            = "PACKAGE_OPERATION_NOT_FOUND"
	OperationErrStorageFailure      = "PACKAGE_OPERATION_STORAGE_FAILURE"
	OperationErrTokenStale          = "PACKAGE_OPERATION_TOKEN_STALE"
	OperationErrProofUnavailable    = "PACKAGE_OPERATION_PROOF_UNAVAILABLE"
	OperationErrLeaseProofMismatch  = "PACKAGE_OPERATION_LEASE_PROOF_MISMATCH"
)

type PackageOperationStateError struct {
	Code   string
	Detail string
	Cause  error
}

func (e *PackageOperationStateError) Error() string {
	if e.Detail == "" {
		return e.Code
	}
	return e.Code + ": " + e.Detail
}

func (e *PackageOperationStateError) Unwrap() error {
	return e.Cause
}

type PackageOperationTransition struct {
	CurrentStep      string
	ErrorCode        string
	ErrorDetail      string
	RecoveryRequired bool
	Completed        bool
}

type PackageExtensionLease struct {
	ExtensionID    string
	OperationID    string
	LeaseOwner     string
	LeaseExpiresAt string
	UpdatedAt      string
	CASVersion     int64
	FencingToken   int64
}

func (r *PackageRepository) CreateOrGetOperation(ctx context.Context, op PackageOperationRecord) (PackageOperationRecord, bool, error) {
	if op.UserID == "" || op.IdempotencyKey == "" || op.RequestHash == "" || op.OperationID == "" || op.ExtensionID == "" {
		return PackageOperationRecord{}, false, operationStateError(OperationErrStorageFailure, "operation authority fields required", nil)
	}
	if op.Status == "" {
		op.Status = string(PackageOperationPending)
	}
	if op.AttemptCount < 1 {
		op.AttemptCount = 1
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if op.StartedAt == "" {
		op.StartedAt = now
	}
	if op.UpdatedAt == "" {
		op.UpdatedAt = now
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return PackageOperationRecord{}, false, storageOperationError("begin create operation", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO extension_package_operations (
		operation_id, trace_id, user_id, scope_type, scope_id, extension_id, target_version,
		operation_type, status, current_step, artifact_id, preview_session_id, confirmations_json,
		error_code, error_detail, started_at, updated_at, completed_at, stable_generation,
		target_generation, current_pointer_json, idempotency_key, request_hash, from_version,
		recovery_required, cancel_requested_at, lease_owner, lease_expires_at, attempt_count,
		fencing_token, owner_instance_id, confirmation_claims_json, snapshot_requirement_hash
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		op.OperationID, op.TraceID, op.UserID, op.ScopeType, op.ScopeID, op.ExtensionID, op.TargetVersion,
		op.OperationType, op.Status, op.CurrentStep, op.ArtifactID, op.PreviewSessionID,
		op.ConfirmationsJSON, op.ErrorCode, op.ErrorDetail, op.StartedAt, op.UpdatedAt, op.CompletedAt,
		op.StableGeneration, op.TargetGeneration, op.CurrentPointerJSON, op.IdempotencyKey,
		op.RequestHash, op.FromVersion, boolInteger(op.RecoveryRequired), op.CancelRequestedAt,
		op.LeaseOwner, op.LeaseExpiresAt, op.AttemptCount, op.FencingToken, op.OwnerInstanceID, op.ConfirmationClaimsJSON, op.SnapshotRequirementHash)
	if err != nil {
		return PackageOperationRecord{}, false, storageOperationError("insert operation", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return PackageOperationRecord{}, false, storageOperationError("inspect operation insert", err)
	}
	existing, err := getAuthoritativeOperationTx(ctx, tx, op.UserID, op.IdempotencyKey)
	if err != nil {
		return PackageOperationRecord{}, false, err
	}
	if existing.RequestHash != op.RequestHash {
		return PackageOperationRecord{}, false, operationStateError(OperationErrIdempotencyConflict, "idempotency key reused with different request", nil)
	}
	created := rows == 1 && existing.OperationID == op.OperationID
	if rows == 0 && existing.OperationID != op.OperationID && existing.IdempotencyKey != op.IdempotencyKey {
		return PackageOperationRecord{}, false, operationStateError(OperationErrIdempotencyConflict, "operation id collision", nil)
	}
	if created && op.ArtifactID != "" {
		if _, err := acquireArtifactReferenceTx(ctx, tx, op.ArtifactID, ArtifactReferenceOperation, op.OperationID, time.Time{}); err != nil {
			return PackageOperationRecord{}, false, storageOperationError("acquire operation artifact", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return PackageOperationRecord{}, false, storageOperationError("commit operation", err)
	}
	return existing, created, nil
}

func (r *PackageRepository) AcquireExtensionLease(ctx context.Context, extensionID, operationID, owner string, ttl time.Duration) (PackageExtensionLease, error) {
	if extensionID == "" || operationID == "" || owner == "" || ttl <= 0 {
		return PackageExtensionLease{}, operationStateError(OperationErrLeaseConflict, "valid lease binding required", nil)
	}
	now := time.Now().UTC()
	expires := now.Add(ttl)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return PackageExtensionLease{}, storageOperationError("begin extension lease", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO extension_package_operation_leases (
		extension_id, operation_id, lease_owner, lease_expires_at, updated_at, cas_version, fencing_token
	) VALUES (?, ?, ?, ?, ?, 1, 1)
	ON CONFLICT(extension_id) DO UPDATE SET operation_id=excluded.operation_id,
	lease_owner=excluded.lease_owner, lease_expires_at=excluded.lease_expires_at,
	updated_at=excluded.updated_at, cas_version=extension_package_operation_leases.cas_version+1,
	fencing_token=extension_package_operation_leases.fencing_token+1
	WHERE extension_package_operation_leases.lease_expires_at <= ?
	OR (extension_package_operation_leases.operation_id = excluded.operation_id
	AND extension_package_operation_leases.lease_owner = excluded.lease_owner)`, extensionID, operationID,
		owner, expires.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return PackageExtensionLease{}, storageOperationError("acquire extension lease", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return PackageExtensionLease{}, storageOperationError("inspect extension lease", err)
	}
	if rows != 1 {
		return PackageExtensionLease{}, operationStateError(OperationErrLeaseConflict, "extension has an active operation", nil)
	}
	bound, err := tx.ExecContext(ctx, `UPDATE extension_package_operations SET lease_owner=?, lease_expires_at=?, updated_at=? WHERE operation_id=? AND extension_id=?`,
		owner, expires.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), operationID, extensionID)
	if err != nil {
		return PackageExtensionLease{}, storageOperationError("bind operation lease", err)
	}
	boundRows, err := bound.RowsAffected()
	if err != nil || boundRows != 1 {
		return PackageExtensionLease{}, operationStateError(OperationErrNotFound, "lease operation binding unavailable", err)
	}
	lease, err := getExtensionLeaseTx(ctx, tx, extensionID)
	if err != nil {
		return PackageExtensionLease{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE extension_package_operations SET fencing_token=? WHERE operation_id=? AND extension_id=?`,
		lease.FencingToken, operationID, extensionID); err != nil {
		return PackageExtensionLease{}, storageOperationError("store operation fencing token", err)
	}
	if err := tx.Commit(); err != nil {
		return PackageExtensionLease{}, storageOperationError("commit extension lease", err)
	}
	return lease, nil
}

func (r *PackageRepository) RenewExtensionLease(ctx context.Context, extensionID, operationID, owner string, ttl time.Duration) (PackageExtensionLease, error) {
	if ttl <= 0 {
		return PackageExtensionLease{}, operationStateError(OperationErrLeaseConflict, "positive lease ttl required", nil)
	}
	now := time.Now().UTC()
	expires := now.Add(ttl)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return PackageExtensionLease{}, storageOperationError("begin lease renewal", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE extension_package_operation_leases
		SET lease_expires_at=?, updated_at=?, cas_version=cas_version+1
		WHERE extension_id=? AND operation_id=? AND lease_owner=? AND lease_expires_at>?`,
		expires.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), extensionID, operationID, owner, now.Format(time.RFC3339Nano))
	if err != nil {
		return PackageExtensionLease{}, storageOperationError("renew extension lease", err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return PackageExtensionLease{}, operationStateError(OperationErrLeaseConflict, "lease is not owned or has expired", err)
	}
	bound, err := tx.ExecContext(ctx, `UPDATE extension_package_operations SET lease_expires_at=?, updated_at=? WHERE operation_id=? AND lease_owner=?`,
		expires.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), operationID, owner)
	if err != nil {
		return PackageExtensionLease{}, storageOperationError("renew operation lease", err)
	}
	boundRows, err := bound.RowsAffected()
	if err != nil || boundRows != 1 {
		return PackageExtensionLease{}, operationStateError(OperationErrLeaseConflict, "operation lease binding changed", err)
	}
	lease, err := getExtensionLeaseTx(ctx, tx, extensionID)
	if err != nil {
		return PackageExtensionLease{}, err
	}
	if err := tx.Commit(); err != nil {
		return PackageExtensionLease{}, storageOperationError("commit lease renewal", err)
	}
	return lease, nil
}

func (r *PackageRepository) ReleaseExtensionLease(ctx context.Context, extensionID, operationID, owner string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return storageOperationError("begin lease release", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM extension_package_operation_leases WHERE extension_id=? AND operation_id=? AND lease_owner=?`, extensionID, operationID, owner)
	if err != nil {
		return storageOperationError("release extension lease", err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return operationStateError(OperationErrLeaseConflict, "lease is not owned", err)
	}
	_, err = tx.ExecContext(ctx, `UPDATE extension_package_operations SET lease_owner='', lease_expires_at='', updated_at=? WHERE operation_id=? AND lease_owner=?`,
		time.Now().UTC().Format(time.RFC3339Nano), operationID, owner)
	if err != nil {
		return storageOperationError("clear operation lease", err)
	}
	if err := tx.Commit(); err != nil {
		return storageOperationError("commit lease release", err)
	}
	return nil
}

func (r *PackageRepository) TransitionOperation(ctx context.Context, operationID string, expected []PackageOperationStatus, target PackageOperationStatus, change PackageOperationTransition, guard PackageWriteGuard) error {
	if len(expected) == 0 || !validOperationTransition(expected, target) {
		return operationStateError(OperationErrTransitionConflict, "illegal operation transition", nil)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(expected)), ",")
	args := []any{string(target), change.CurrentStep, change.ErrorCode, change.ErrorDetail, boolInteger(change.RecoveryRequired), time.Now().UTC().Format(time.RFC3339Nano)}
	completedAt := ""
	if change.Completed || target == PackageOperationCompleted || target == PackageOperationFailed || target == PackageOperationCancelled {
		completedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	args = append(args, completedAt, completedAt, operationID)
	for _, status := range expected {
		args = append(args, string(status))
	}
	query := `UPDATE extension_package_operations SET status=?, current_step=?, error_code=?, error_detail=?,
		recovery_required=?, updated_at=?, completed_at=CASE WHEN ?<>'' THEN ? ELSE completed_at END
		WHERE operation_id=? AND status IN (` + placeholders + `)`
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return storageOperationError("begin operation transition", err)
	}
	defer tx.Rollback()
	if err := verifyFencingTokenTx(ctx, tx, guard); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return storageOperationError("transition operation", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return storageOperationError("inspect operation transition", err)
	}
	if rows != 1 {
		return operationStateError(OperationErrTransitionConflict, "operation status changed", nil)
	}
	if target == PackageOperationCompleted || target == PackageOperationFailed || target == PackageOperationCancelled {
		var artifactID string
		if err := tx.QueryRowContext(ctx, `SELECT artifact_id FROM extension_package_operations WHERE operation_id=?`, operationID).Scan(&artifactID); err != nil {
			return classifyOperationRead("read terminal operation artifact", err)
		}
		if artifactID != "" {
			if err := releaseArtifactReferenceTx(ctx, tx, artifactID, ArtifactReferenceOperation, operationID); err != nil {
				return storageOperationError("release terminal operation artifact", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return storageOperationError("commit operation transition", err)
	}
	return nil
}

func (r *PackageRepository) BeginStep(ctx context.Context, operationID, stepName string, stepOrder int, inputHash string, guard PackageWriteGuard) (PackageOperationStep, bool, error) {
	if operationID == "" || stepName == "" || inputHash == "" {
		return PackageOperationStep{}, false, operationStateError(OperationErrStepInputConflict, "step input binding required", nil)
	}
	if err := verifyFencingTokenDB(ctx, r.db, guard); err != nil {
		return PackageOperationStep{}, false, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	stepID := operationID + ":" + stepName
	result, err := r.db.ExecContext(ctx, `INSERT OR IGNORE INTO extension_package_operation_steps (
		step_id, operation_id, step_name, step_order, status, attempt_count, result_json, error_code,
		started_at, completed_at, stable_generation, target_generation, current_pointer_json,
		input_hash, error_detail, compensation_name, compensation_status, side_effect_evidence, updated_at, cas_version
	) VALUES (?, ?, ?, ?, 'in_progress', 1, '{}', '', ?, '', '', '', '', ?, '', '', '', '', ?, 1)`,
		stepID, operationID, stepName, stepOrder, now, inputHash, now)
	if err != nil {
		return PackageOperationStep{}, false, storageOperationError("begin operation step", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return PackageOperationStep{}, false, storageOperationError("inspect operation step", err)
	}
	step, err := r.getOperationStep(ctx, operationID, stepName)
	if err != nil {
		return PackageOperationStep{}, false, err
	}
	if step.InputHash != inputHash {
		return PackageOperationStep{}, false, operationStateError(OperationErrStepInputConflict, "step name reused with different input", nil)
	}
	return step, rows == 1, nil
}

func (r *PackageRepository) CompleteStep(ctx context.Context, operationID, stepName, inputHash, resultJSON, sideEffectEvidence string, guard PackageWriteGuard) (PackageOperationStep, error) {
	return r.finishStep(ctx, operationID, stepName, inputHash, "completed", resultJSON, "", "", sideEffectEvidence, guard)
}

func (r *PackageRepository) FailStep(ctx context.Context, operationID, stepName, inputHash, errorCode, errorDetail, sideEffectEvidence string, guard PackageWriteGuard) (PackageOperationStep, error) {
	return r.finishStep(ctx, operationID, stepName, inputHash, "failed", "{}", errorCode, errorDetail, sideEffectEvidence, guard)
}

func (r *PackageRepository) BeginCompensation(ctx context.Context, operationID, stepName, compensationName string, guard PackageWriteGuard) (PackageOperationStep, error) {
	if err := verifyFencingTokenDB(ctx, r.db, guard); err != nil {
		return PackageOperationStep{}, err
	}
	step, err := r.getOperationStep(ctx, operationID, stepName)
	if err != nil {
		return PackageOperationStep{}, err
	}
	if step.CompensationStatus == "completed" || step.CompensationStatus == "in_progress" && step.CompensationName == compensationName {
		return step, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := r.db.ExecContext(ctx, `UPDATE extension_package_operation_steps SET compensation_name=?,
		compensation_status='in_progress', updated_at=?, cas_version=cas_version+1
		WHERE operation_id=? AND step_name=? AND cas_version=? AND compensation_status IN ('','failed')`,
		compensationName, now, operationID, stepName, step.CASVersion)
	if err != nil {
		return PackageOperationStep{}, storageOperationError("begin compensation", err)
	}
	return r.requireStepCAS(ctx, result, operationID, stepName)
}

func (r *PackageRepository) CompleteCompensation(ctx context.Context, operationID, stepName, compensationName, evidence string, guard PackageWriteGuard) (PackageOperationStep, error) {
	if err := verifyFencingTokenDB(ctx, r.db, guard); err != nil {
		return PackageOperationStep{}, err
	}
	step, err := r.getOperationStep(ctx, operationID, stepName)
	if err != nil {
		return PackageOperationStep{}, err
	}
	if step.CompensationStatus == "completed" && step.CompensationName == compensationName && step.SideEffectEvidence == evidence {
		return step, nil
	}
	if step.CompensationName != compensationName || step.CompensationStatus != "in_progress" {
		return PackageOperationStep{}, operationStateError(OperationErrTransitionConflict, "compensation is not active", nil)
	}
	result, err := r.db.ExecContext(ctx, `UPDATE extension_package_operation_steps SET compensation_status='completed',
		side_effect_evidence=?, updated_at=?, cas_version=cas_version+1
		WHERE operation_id=? AND step_name=? AND cas_version=? AND compensation_status='in_progress'`,
		evidence, time.Now().UTC().Format(time.RFC3339Nano), operationID, stepName, step.CASVersion)
	if err != nil {
		return PackageOperationStep{}, storageOperationError("complete compensation", err)
	}
	return r.requireStepCAS(ctx, result, operationID, stepName)
}

func (r *PackageRepository) RequestCancel(ctx context.Context, operationID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := r.db.ExecContext(ctx, `UPDATE extension_package_operations SET cancel_requested_at=?, updated_at=?
		WHERE operation_id=? AND status IN ('pending','in_progress') AND cancel_requested_at=''`, now, now, operationID)
	if err != nil {
		return storageOperationError("request operation cancel", err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return operationStateError(OperationErrCancelNotAllowed, "operation cannot be cancelled", err)
	}
	return nil
}

func (r *PackageRepository) RetryOperation(ctx context.Context, operationID, leaseOwner string) (PackageOperationRecord, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := r.db.ExecContext(ctx, `UPDATE extension_package_operations SET status='pending',
		attempt_count=attempt_count+1, error_code='', error_detail='', cancel_requested_at='',
		updated_at=? WHERE operation_id=? AND status IN ('failed','requires_recovery')
		AND recovery_required=1 AND lease_owner=? AND lease_expires_at>?`, now, operationID, leaseOwner, now)
	if err != nil {
		return PackageOperationRecord{}, storageOperationError("retry operation", err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return PackageOperationRecord{}, operationStateError(OperationErrRetryNotAllowed, "operation is not recoverable or lease is invalid", err)
	}
	return r.getAuthoritativeOperationByID(ctx, operationID)
}

func (r *PackageRepository) finishStep(ctx context.Context, operationID, stepName, inputHash, status, resultJSON, errorCode, errorDetail, evidence string, guard PackageWriteGuard) (PackageOperationStep, error) {
	if err := verifyFencingTokenDB(ctx, r.db, guard); err != nil {
		return PackageOperationStep{}, err
	}
	step, err := r.getOperationStep(ctx, operationID, stepName)
	if err != nil {
		return PackageOperationStep{}, err
	}
	if step.InputHash != inputHash {
		return PackageOperationStep{}, operationStateError(OperationErrStepInputConflict, "step input hash changed", nil)
	}
	if step.Status == status {
		if step.ResultJSON == resultJSON && step.ErrorCode == errorCode && step.ErrorDetail == errorDetail && step.SideEffectEvidence == evidence {
			return step, nil
		}
		return PackageOperationStep{}, operationStateError(OperationErrSideEffectConflict, "step already finalized with different evidence", nil)
	}
	if step.Status != "in_progress" {
		return PackageOperationStep{}, operationStateError(OperationErrTransitionConflict, "step is not in progress", nil)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := r.db.ExecContext(ctx, `UPDATE extension_package_operation_steps SET status=?, result_json=?,
		error_code=?, error_detail=?, side_effect_evidence=?, completed_at=?, updated_at=?, cas_version=cas_version+1
		WHERE operation_id=? AND step_name=? AND input_hash=? AND status='in_progress' AND cas_version=?`,
		status, resultJSON, errorCode, errorDetail, evidence, now, now, operationID, stepName, inputHash, step.CASVersion)
	if err != nil {
		return PackageOperationStep{}, storageOperationError("finish operation step", err)
	}
	return r.requireStepCAS(ctx, result, operationID, stepName)
}

func (r *PackageRepository) requireStepCAS(ctx context.Context, result sql.Result, operationID, stepName string) (PackageOperationStep, error) {
	rows, err := result.RowsAffected()
	if err != nil {
		return PackageOperationStep{}, storageOperationError("inspect step CAS", err)
	}
	if rows != 1 {
		return PackageOperationStep{}, operationStateError(OperationErrTransitionConflict, "step changed concurrently", nil)
	}
	return r.getOperationStep(ctx, operationID, stepName)
}

func (r *PackageRepository) getOperationStep(ctx context.Context, operationID, stepName string) (PackageOperationStep, error) {
	var step PackageOperationStep
	err := r.db.QueryRowContext(ctx, `SELECT step_id, operation_id, step_name, step_order, status,
		attempt_count, result_json, error_code, started_at, completed_at, stable_generation,
		target_generation, current_pointer_json, input_hash, error_detail, compensation_name,
		compensation_status, side_effect_evidence, updated_at, cas_version
		FROM extension_package_operation_steps WHERE operation_id=? AND step_name=?`, operationID, stepName).
		Scan(&step.StepID, &step.OperationID, &step.StepName, &step.StepOrder, &step.Status,
			&step.AttemptCount, &step.ResultJSON, &step.ErrorCode, &step.StartedAt, &step.CompletedAt,
			&step.StableGeneration, &step.TargetGeneration, &step.CurrentPointerJSON, &step.InputHash,
			&step.ErrorDetail, &step.CompensationName, &step.CompensationStatus,
			&step.SideEffectEvidence, &step.UpdatedAt, &step.CASVersion)
	if err != nil {
		return PackageOperationStep{}, classifyOperationRead("read operation step", err)
	}
	return step, nil
}

func getAuthoritativeOperationTx(ctx context.Context, tx *sql.Tx, userID, idempotencyKey string) (PackageOperationRecord, error) {
	return scanAuthoritativeOperation(tx.QueryRowContext(ctx, authoritativeOperationSelect+` WHERE user_id=? AND idempotency_key=?`, userID, idempotencyKey))
}

func (r *PackageRepository) getAuthoritativeOperationByID(ctx context.Context, operationID string) (PackageOperationRecord, error) {
	return scanAuthoritativeOperation(r.db.QueryRowContext(ctx, authoritativeOperationSelect+` WHERE operation_id=?`, operationID))
}

const authoritativeOperationSelect = `SELECT operation_id, trace_id, user_id, scope_type, scope_id,
	extension_id, target_version, operation_type, status, current_step, artifact_id, preview_session_id,
	confirmations_json, error_code, error_detail, started_at, updated_at, completed_at,
	stable_generation, target_generation, current_pointer_json, idempotency_key, request_hash,
	from_version, recovery_required, cancel_requested_at, lease_owner, lease_expires_at, attempt_count,
	fencing_token, owner_instance_id, confirmation_claims_json, snapshot_requirement_hash
	FROM extension_package_operations`

type operationRow interface {
	Scan(...any) error
}

func scanAuthoritativeOperation(row operationRow) (PackageOperationRecord, error) {
	var op PackageOperationRecord
	err := row.Scan(&op.OperationID, &op.TraceID, &op.UserID, &op.ScopeType, &op.ScopeID,
		&op.ExtensionID, &op.TargetVersion, &op.OperationType, &op.Status, &op.CurrentStep,
		&op.ArtifactID, &op.PreviewSessionID, &op.ConfirmationsJSON, &op.ErrorCode, &op.ErrorDetail,
		&op.StartedAt, &op.UpdatedAt, &op.CompletedAt, &op.StableGeneration, &op.TargetGeneration,
		&op.CurrentPointerJSON, &op.IdempotencyKey, &op.RequestHash, &op.FromVersion,
		&op.RecoveryRequired, &op.CancelRequestedAt, &op.LeaseOwner, &op.LeaseExpiresAt, &op.AttemptCount,
		&op.FencingToken, &op.OwnerInstanceID, &op.ConfirmationClaimsJSON, &op.SnapshotRequirementHash)
	if err != nil {
		return PackageOperationRecord{}, classifyOperationRead("read operation", err)
	}
	return op, nil
}

func (r *PackageRepository) getExtensionLease(ctx context.Context, extensionID string) (PackageExtensionLease, error) {
	return scanExtensionLease(r.db.QueryRowContext(ctx, `SELECT extension_id, operation_id, lease_owner,
		lease_expires_at, updated_at, cas_version, fencing_token FROM extension_package_operation_leases WHERE extension_id=?`, extensionID))
}

func getExtensionLeaseTx(ctx context.Context, tx *sql.Tx, extensionID string) (PackageExtensionLease, error) {
	return scanExtensionLease(tx.QueryRowContext(ctx, `SELECT extension_id, operation_id, lease_owner,
		lease_expires_at, updated_at, cas_version, fencing_token FROM extension_package_operation_leases WHERE extension_id=?`, extensionID))
}

func scanExtensionLease(row operationRow) (PackageExtensionLease, error) {
	var lease PackageExtensionLease
	err := row.Scan(&lease.ExtensionID, &lease.OperationID, &lease.LeaseOwner, &lease.LeaseExpiresAt, &lease.UpdatedAt, &lease.CASVersion, &lease.FencingToken)
	if err != nil {
		return PackageExtensionLease{}, classifyOperationRead("read extension lease", err)
	}
	return lease, nil
}

func validOperationTransition(expected []PackageOperationStatus, target PackageOperationStatus) bool {
	allowed := map[PackageOperationStatus]map[PackageOperationStatus]bool{
		PackageOperationPending: {PackageOperationPending: true, PackageOperationInProgress: true,
			PackageOperationFailed: true, PackageOperationCancelled: true},
		PackageOperationInProgress: {PackageOperationInProgress: true, PackageOperationCompleted: true,
			PackageOperationFailed: true, PackageOperationCancelled: true, PackageOperationRequiresRecovery: true,
			PackageOperationFinalizing: true},
		PackageOperationRequiresRecovery: {PackageOperationRequiresRecovery: true, PackageOperationCompleted: true,
			PackageOperationFailed: true, PackageOperationCancelled: true},
		PackageOperationFinalizing: {PackageOperationFinalizing: true, PackageOperationCompleted: true,
			PackageOperationFailed: true, PackageOperationCancelled: true, PackageOperationRequiresRecovery: true,
			PackageOperationReleasePending: true},
		PackageOperationReleasePending: {PackageOperationReleasePending: true, PackageOperationFinalizing: true,
			PackageOperationRequiresRecovery: true, PackageOperationFailed: true},
	}
	for _, source := range expected {
		if allowed[source][target] {
			return true
		}
	}
	return false
}

func classifyOperationRead(action string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return operationStateError(OperationErrNotFound, action, err)
	}
	return storageOperationError(action, err)
}

func storageOperationError(action string, err error) error {
	return operationStateError(OperationErrStorageFailure, action, err)
}

func operationStateError(code, detail string, cause error) error {
	return &PackageOperationStateError{Code: code, Detail: detail, Cause: cause}
}

func boolInteger(value bool) int {
	if value {
		return 1
	}
	return 0
}

func IsPackageOperationError(err error, code string) bool {
	var stateErr *PackageOperationStateError
	return errors.As(err, &stateErr) && stateErr.Code == code
}

func hashLegacyOperationRequest(op PackageOperationRecord) string {
	return fmt.Sprintf("legacy:%s:%s:%s:%s:%s", op.OperationType, op.ExtensionID, op.TargetVersion, op.ArtifactID, op.PreviewSessionID)
}
