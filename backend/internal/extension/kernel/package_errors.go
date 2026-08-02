package kernel

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/migration"
)

var (
	ErrPackageAlreadyInstalled                     = errors.New("kernel: package already installed")
	ErrPackageNotInstalled                         = errors.New("kernel: package not installed")
	ErrPackageOwnerMismatch                        = errors.New("kernel: package scope mismatch")
	ErrPackageUpdateIDMismatch                     = errors.New("kernel: package update id mismatch")
	ErrPackageUpdateTargetUnchanged                = errors.New("kernel: package update target unchanged")
	ErrPackageUninstallPreflightChanged            = errors.New("kernel: package uninstall preflight changed")
	ErrPackagePreviewSessionExpired                = errors.New("kernel: preview session expired")
	ErrPackagePreviewSessionStatus                 = errors.New("kernel: preview session status invalid")
	ErrPackageSecurityPolicyChanged                = errors.New("kernel: preview security policy changed")
	ErrPackageArtifactIdentityMismatch             = errors.New("kernel: artifact identity mismatch")
	ErrPackageConfirmationTokenInvalid             = errors.New("kernel: confirmation token invalid")
	ErrPackageConfirmationBindingMismatch          = errors.New("kernel: confirmation token binding mismatch")
	ErrPackageRollbackPointUnavailable             = errors.New("kernel: rollback point unavailable")
	ErrPackageRollbackPointCorrupt                 = errors.New("kernel: rollback point corrupt")
	ErrPackageRollbackPointExpired                 = errors.New("kernel: rollback point expired")
	ErrPackageRollbackPointHashMismatch            = errors.New("kernel: rollback point snapshot hash mismatch")
	ErrPackageMigrationPlanDrift                   = errors.New("kernel: package migration plan drift")
	ErrPackageMigrationPreflightMissing            = errors.New("kernel: package migration preflight evidence missing")
	ErrPackageMigrationManualRequired              = errors.New("kernel: package migration requires controlled manual recovery")
	ErrPackageServicesUnavailable                  = errors.New("kernel: package services unavailable")
	ErrPackageDependencyStateChanged               = errors.New("kernel: dependency or compatibility state changed after preview")
	ErrPackageUninstallEvidenceIncomplete          = errors.New("kernel: uninstall installation evidence incomplete")
	ErrPackageUninstallCurrentPointerMismatch      = errors.New("kernel: uninstall current pointer mismatch")
	ErrPackageUninstallTreeVerificationFailed      = errors.New("kernel: uninstall installed tree verification failed")
	ErrPackageForwardRecoveryPointFailed           = errors.New("kernel: forward recovery point creation failed")
	ErrPackageManualRecoveryRequired               = errors.New("kernel: requires_manual_recovery")
	ErrPackageIdempotencyKeyRequired               = errors.New("kernel: idempotency key required from caller")
	ErrPackageIdempotencyKeyReused                 = errors.New("kernel: idempotency key reused with different request")
	ErrPackageOperationConflict                    = errors.New("kernel: package operation conflict")
	ErrPackageLeaseAcquireFailed                   = errors.New("kernel: extension lease acquire failed")
	ErrPackageLeaseLost                            = errors.New("kernel: extension lease lost or expired")
	ErrPackageLeaseFenced                          = errors.New("kernel: extension lease fenced by newer token")
	ErrPackageMigrationFailed                      = errors.New("kernel: package migration execution failed")
	ErrPackageMigrationIrreversible                = errors.New("kernel: package migration is irreversible")
	ErrPackageRecoveryAmbiguous                    = errors.New("kernel: package recovery state ambiguous")
	ErrPackageArtifactMetadataMismatch             = errors.New("kernel: artifact metadata mismatch")
	ErrPackageQuarantineRestoreFailed              = errors.New("kernel: quarantine restore failed")
	ErrPackageSnapshotIncomplete                   = errors.New("kernel: rollback snapshot incomplete")
	ErrPackageVersionHistoryCorrupted              = errors.New("kernel: package version history corrupted")
	ErrPackageFinalGateFailed                      = errors.New("kernel: final gate validation failed")
	ErrPackageVersionActivateTargetNotFound        = errors.New("kernel: package version activate target not found")
	ErrPackageInstallationNotFound                 = errors.New("kernel: package installation not found")
	ErrPackageVersionRepositoryUnavailable         = errors.New("kernel: package version repository unavailable")
	ErrPackageInstallationGenerationIDMissing      = errors.New("kernel: package installation generation id missing")
	ErrPackageVersionDeactivateTargetNotFound      = errors.New("kernel: package version deactivate target not found")
	ErrPackageCompensationFailed                   = errors.New("kernel: package compensation failed")
	ErrPackageMigrationSandboxViolation            = errors.New("kernel: package migration sandbox violation")
	ErrPackageMigrationCompensationFailed          = errors.New("kernel: package migration compensation failed")
	ErrPackageQuarantineMetadataUnavailable        = errors.New("kernel: quarantine metadata repository unavailable")
	ErrPackageQuarantineMetadataMissing            = errors.New("kernel: quarantine metadata not found for operation")
	ErrPackageQuarantineMetadataIncomplete         = errors.New("kernel: quarantine metadata content incomplete")
	ErrPackageQuarantineReleaseFailed              = errors.New("kernel: quarantine metadata release failed")
	ErrPackageQuarantineStatePersistFailed         = errors.New("kernel: quarantine metadata state persistence failed")
	ErrPackageRecoveryStepPersistFailed            = errors.New("kernel: recovery step persistence failed")
	ErrPackageRollbackSnapshotCorrupted            = errors.New("kernel: rollback snapshot hash mismatch")
	ErrPackageResourceSnapshotInvalid              = errors.New("kernel: package resource snapshot invalid")
	ErrPackageUninstallArtifactPolicyUnproven      = errors.New("kernel: uninstall artifact retention policy unproven")
	ErrPackageUninstallArtifactMissing             = errors.New("kernel: uninstall artifact unexpectedly missing")
	ErrPackageUninstallArtifactReferenced          = errors.New("kernel: uninstall artifact still referenced")
	ErrPackageMigrationSQLUnparseable              = errors.New("kernel: migration sql statement unparseable")
	ErrPackageMigrationNamespaceViolation          = errors.New("kernel: migration namespace violation")
	ErrPackageLegacyRuntimeEnabled                 = errors.New("kernel: legacy runtime enabled in production")
	ErrPackageConfirmationStale                    = errors.New("kernel: confirmation token stale, re-preview required")
	ErrPackageRollbackRequirementChanged           = errors.New("kernel: rollback snapshot requirement changed after confirmation")
	ErrPackageUninstallArtifactRemovalFailed       = errors.New("kernel: uninstall artifact removal failed")
	ErrPackageConfirmationRequired                 = errors.New("kernel: confirmation required")
	ErrPackageConfirmationItemsMissing             = errors.New("kernel: confirmation items missing")
	ErrPackageConfirmationItemsMismatch            = errors.New("kernel: confirmation items mismatch")
	ErrPackageConfirmationPolicyMismatch           = errors.New("kernel: confirmation policy mismatch")
	ErrPackageConfirmationPolicyVersionStale       = errors.New("kernel: confirmation policy version stale")
	ErrPackageConfirmationPolicyVersionUnsupported = errors.New("kernel: confirmation policy version unsupported")
	ErrPackageArtifactPolicyMismatch               = errors.New("kernel: artifact policy mismatch")
	ErrPackageArtifactDeleteIncomplete             = errors.New("kernel: artifact delete incomplete")
	ErrPackageArtifactRetentionMismatch            = errors.New("kernel: artifact retention mismatch")
	ErrPackageArtifactReferenceRemains             = errors.New("kernel: artifact reference remains")
	ErrPackageSnapshotRequired                     = errors.New("kernel: snapshot required")
	ErrPackageSnapshotMissing                      = errors.New("kernel: snapshot missing")
	ErrPackageSnapshotInvalid                      = errors.New("kernel: snapshot invalid")
	ErrPackageSnapshotRequirementMismatch          = errors.New("kernel: snapshot requirement mismatch")
	ErrPackageSnapshotExemptionInvalid             = errors.New("kernel: snapshot exemption invalid")
	ErrPackageSnapshotEvidenceMissing              = errors.New("kernel: snapshot evidence missing")
	ErrPackageFinalGateEvidenceMissing             = errors.New("kernel: final gate evidence missing")
)

const (
	PackageErrCodeIdempotencyKeyRequired               = "PACKAGE_IDEMPOTENCY_KEY_REQUIRED"
	PackageErrCodeIdempotencyKeyReused                 = "PACKAGE_IDEMPOTENCY_KEY_REUSED"
	PackageErrCodeOperationConflict                    = "PACKAGE_OPERATION_CONFLICT"
	PackageErrCodeLeaseAcquireFailed                   = "PACKAGE_LEASE_ACQUIRE_FAILED"
	PackageErrCodeLeaseLost                            = "PACKAGE_LEASE_LOST"
	PackageErrCodeLeaseFenced                          = "PACKAGE_LEASE_FENCED"
	PackageErrCodeMigrationFailed                      = "PACKAGE_MIGRATION_FAILED"
	PackageErrCodeMigrationIrreversible                = "PACKAGE_MIGRATION_IRREVERSIBLE"
	PackageErrCodeRecoveryAmbiguous                    = "PACKAGE_RECOVERY_AMBIGUOUS"
	PackageErrCodeArtifactMetadataMismatch             = "PACKAGE_ARTIFACT_METADATA_MISMATCH"
	PackageErrCodeQuarantineRestoreFailed              = "PACKAGE_QUARANTINE_RESTORE_FAILED"
	PackageErrCodeSnapshotIncomplete                   = "PACKAGE_SNAPSHOT_INCOMPLETE"
	PackageErrCodeRollbackSnapshotIncomplete           = "PACKAGE_ROLLBACK_SNAPSHOT_INCOMPLETE"
	PackageErrCodeVersionHistoryCorrupted              = "PACKAGE_VERSION_HISTORY_CORRUPTED"
	PackageErrCodeFinalGateFailed                      = "PACKAGE_FINAL_GATE_FAILED"
	PackageErrCodeVersionActivateTargetNotFound        = "PACKAGE_VERSION_ACTIVATE_TARGET_NOT_FOUND"
	PackageErrCodeInstallationNotFound                 = "PACKAGE_INSTALLATION_NOT_FOUND"
	PackageErrCodeVersionRepositoryUnavailable         = "PACKAGE_VERSION_REPOSITORY_UNAVAILABLE"
	PackageErrCodeInstallationGenerationIDMissing      = "PACKAGE_INSTALLATION_GENERATION_ID_MISSING"
	PackageErrCodeVersionDeactivateTargetNotFound      = "PACKAGE_VERSION_DEACTIVATE_TARGET_NOT_FOUND"
	PackageErrCodeCompensationFailed                   = "PACKAGE_COMPENSATION_FAILED"
	PackageErrCodeMigrationSandboxViolation            = "PACKAGE_MIGRATION_SANDBOX_VIOLATION"
	PackageErrCodeMigrationCompensationFailed          = "PACKAGE_MIGRATION_COMPENSATION_FAILED"
	PackageErrCodeQuarantineMetadataUnavailable        = "PACKAGE_QUARANTINE_METADATA_UNAVAILABLE"
	PackageErrCodeQuarantineMetadataMissing            = "PACKAGE_QUARANTINE_METADATA_MISSING"
	PackageErrCodeQuarantineMetadataIncomplete         = "PACKAGE_QUARANTINE_METADATA_INCOMPLETE"
	PackageErrCodeQuarantineReleaseFailed              = "PACKAGE_QUARANTINE_RELEASE_FAILED"
	PackageErrCodeQuarantineStatePersistFailed         = "PACKAGE_QUARANTINE_STATE_PERSIST_FAILED"
	PackageErrCodeRecoveryStepPersistFailed            = "PACKAGE_RECOVERY_STEP_PERSIST_FAILED"
	PackageErrCodeRollbackSnapshotCorrupted            = "PACKAGE_ROLLBACK_SNAPSHOT_CORRUPTED"
	PackageErrCodeUninstallArtifactPolicyUnproven      = "PACKAGE_UNINSTALL_ARTIFACT_POLICY_UNPROVEN"
	PackageErrCodeUninstallArtifactMissing             = "PACKAGE_UNINSTALL_ARTIFACT_UNEXPECTEDLY_MISSING"
	PackageErrCodeUninstallArtifactReferenced          = "PACKAGE_UNINSTALL_ARTIFACT_STILL_REFERENCED"
	PackageErrCodeUninstallArtifactRemovalFailed       = "PACKAGE_UNINSTALL_ARTIFACT_REMOVAL_FAILED"
	PackageErrCodeResourceSnapshotInvalid              = "PACKAGE_RESOURCE_SNAPSHOT_INVALID"
	PackageErrCodeMigrationSQLUnparseable              = "PACKAGE_MIGRATION_SQL_UNPARSEABLE"
	PackageErrCodeMigrationNamespaceViolation          = "PACKAGE_MIGRATION_NAMESPACE_VIOLATION"
	PackageErrCodeLegacyRuntimeEnabled                 = "PACKAGE_LEGACY_RUNTIME_ENABLED"
	PackageErrCodeUserDataSnapshotInvalid              = "PACKAGE_USER_DATA_SNAPSHOT_INVALID"
	PackageErrCodeUserDataSnapshotStoreUnavailable     = "PACKAGE_USER_DATA_SNAPSHOT_STORE_UNAVAILABLE"
	PackageErrCodeConfirmationTokenInvalid             = "PACKAGE_CONFIRMATION_TOKEN_INVALID"
	PackageErrCodeConfirmationBindingMismatch          = "PACKAGE_CONFIRMATION_BINDING_MISMATCH"
	PackageErrCodeConfirmationStale                    = "PACKAGE_CONFIRMATION_STALE"
	PackageErrCodeRollbackRequirementChanged           = "PACKAGE_ROLLBACK_REQUIREMENT_CHANGED"
	PackageErrCodeQuarantinePathMismatch               = "PACKAGE_QUARANTINE_PATH_MISMATCH"
	PackageErrCodeQuarantineMetadataCorrupt            = "PACKAGE_QUARANTINE_METADATA_CORRUPT"
	PackageErrCodeRecoveryStepEvidenceInvalid          = "PACKAGE_RECOVERY_STEP_EVIDENCE_INVALID"
	PackageErrCodeArtifactReferenceConflict            = "PACKAGE_ARTIFACT_REFERENCE_CONFLICT"
	PackageErrCodeRestoredStateMismatch                = "PACKAGE_RESTORED_STATE_MISMATCH"
	PackageErrCodeFinalizationEvidenceMissing          = "PACKAGE_FINALIZATION_EVIDENCE_MISSING"
	PackageErrCodeLeaseStopFailed                      = "PACKAGE_LEASE_STOP_FAILED"
	PackageErrCodeConfirmationRequired                 = "PACKAGE_CONFIRMATION_REQUIRED"
	PackageErrCodeConfirmationInvalid                  = "PACKAGE_CONFIRMATION_INVALID"
	PackageErrCodeConfirmationExpired                  = "PACKAGE_CONFIRMATION_EXPIRED"
	PackageErrCodeConfirmationItemsMissing             = "PACKAGE_CONFIRMATION_ITEMS_MISSING"
	PackageErrCodeConfirmationItemsMismatch            = "PACKAGE_CONFIRMATION_ITEMS_MISMATCH"
	PackageErrCodeConfirmationPolicyMismatch           = "PACKAGE_CONFIRMATION_POLICY_MISMATCH"
	PackageErrCodeConfirmationPolicyVersionStale       = "PACKAGE_CONFIRMATION_POLICY_VERSION_STALE"
	PackageErrCodeConfirmationPolicyVersionUnsupported = "PACKAGE_CONFIRMATION_POLICY_VERSION_UNSUPPORTED"
	PackageErrCodeArtifactPolicyMismatch               = "PACKAGE_ARTIFACT_POLICY_MISMATCH"
	PackageErrCodeArtifactDeleteIncomplete             = "PACKAGE_ARTIFACT_DELETE_INCOMPLETE"
	PackageErrCodeArtifactRetentionMismatch            = "PACKAGE_ARTIFACT_RETENTION_MISMATCH"
	PackageErrCodeArtifactReferenceRemains             = "PACKAGE_ARTIFACT_REFERENCE_REMAINS"
	PackageErrCodeSnapshotRequired                     = "PACKAGE_SNAPSHOT_REQUIRED"
	PackageErrCodeSnapshotMissing                      = "PACKAGE_SNAPSHOT_MISSING"
	PackageErrCodeSnapshotInvalid                      = "PACKAGE_SNAPSHOT_INVALID"
	PackageErrCodeSnapshotRequirementMismatch          = "PACKAGE_SNAPSHOT_REQUIREMENT_MISMATCH"
	PackageErrCodeSnapshotExemptionInvalid             = "PACKAGE_SNAPSHOT_EXEMPTION_INVALID"
	PackageErrCodeSnapshotEvidenceMissing              = "PACKAGE_SNAPSHOT_EVIDENCE_MISSING"
	PackageErrCodeFinalGateEvidenceMissing             = "PACKAGE_FINAL_GATE_EVIDENCE_MISSING"
	PackageErrCodeUserDataEntityIDMissing              = "PACKAGE_USER_DATA_ENTITY_ID_MISSING"
	PackageErrCodeUserDataRecordInvalid                = "PACKAGE_USER_DATA_RECORD_INVALID"
	PackageErrCodeUserDataBatchHashMismatch            = "PACKAGE_USER_DATA_BATCH_HASH_MISMATCH"
	PackageErrCodeUserDataAggregateHashMismatch        = "PACKAGE_USER_DATA_AGGREGATE_HASH_MISMATCH"
	PackageErrCodeUserDataRestoreJournalConflict       = "PACKAGE_USER_DATA_RESTORE_JOURNAL_CONFLICT"
	PackageErrCodeResourceSnapshotPathInvalid          = "PACKAGE_RESOURCE_SNAPSHOT_PATH_INVALID"
	PackageErrCodeResourceSnapshotSymlinkForbidden     = "PACKAGE_RESOURCE_SNAPSHOT_SYMLINK_FORBIDDEN"
	PackageErrCodeResourceSnapshotHashComputeFailed    = "PACKAGE_RESOURCE_SNAPSHOT_HASH_COMPUTE_FAILED"
	PackageErrCodeResourceSnapshotHashMismatch         = "PACKAGE_RESOURCE_SNAPSHOT_HASH_MISMATCH"
	PackageErrCodeResourceSnapshotContentMissing       = "PACKAGE_RESOURCE_SNAPSHOT_CONTENT_MISSING"
	PackageErrCodeResourceRestoreIncomplete            = "PACKAGE_RESOURCE_RESTORE_INCOMPLETE"
	PackageErrCodeSnapshotSchemaUnsupported            = "PACKAGE_SNAPSHOT_SCHEMA_UNSUPPORTED"
	PackageErrCodeSnapshotIntegrityFailed              = "PACKAGE_SNAPSHOT_INTEGRITY_FAILED"
)

type PackageError struct {
	Code                string
	HTTPStatus          int
	Retryable           bool
	PreviewRequired     bool
	RecoveryRequired    bool
	ConflictOperationID string
	RecommendedAction   string
	Cause               error
}

func (e *PackageError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.Cause)
	}
	return e.Code
}

func (e *PackageError) Unwrap() error {
	return e.Cause
}

func NewPackageError(code string, httpStatus int, cause error) *PackageError {
	return &PackageError{Code: code, HTTPStatus: httpStatus, Cause: cause}
}

func NewPackageErrorWithRecovery(code string, httpStatus int, retryable, recoveryRequired bool, recommendedAction string, cause error) *PackageError {
	return &PackageError{
		Code:              code,
		HTTPStatus:        httpStatus,
		Retryable:         retryable,
		RecoveryRequired:  recoveryRequired,
		RecommendedAction: recommendedAction,
		Cause:             cause,
	}
}

func ToPackageMigrationError(err error) *PackageError {
	if errors.Is(err, migration.ErrSQLUnparseable) {
		return NewPackageError(PackageErrCodeMigrationSQLUnparseable, PackageErrorHTTPStatus(PackageErrCodeMigrationSQLUnparseable), err)
	}
	if errors.Is(err, migration.ErrNamespaceViolation) {
		return NewPackageError(PackageErrCodeMigrationNamespaceViolation, PackageErrorHTTPStatus(PackageErrCodeMigrationNamespaceViolation), err)
	}
	return nil
}

var packageErrorHTTPStatus = map[string]int{
	PackageErrCodeIdempotencyKeyRequired:               400,
	PackageErrCodeIdempotencyKeyReused:                 409,
	PackageErrCodeOperationConflict:                    409,
	PackageErrCodeLeaseAcquireFailed:                   409,
	PackageErrCodeLeaseLost:                            409,
	PackageErrCodeLeaseFenced:                          409,
	PackageErrCodeMigrationFailed:                      500,
	PackageErrCodeMigrationIrreversible:                422,
	PackageErrCodeRecoveryAmbiguous:                    409,
	PackageErrCodeArtifactMetadataMismatch:             409,
	PackageErrCodeQuarantineRestoreFailed:              500,
	PackageErrCodeSnapshotIncomplete:                   409,
	PackageErrCodeRollbackSnapshotIncomplete:           409,
	PackageErrCodeVersionHistoryCorrupted:              409,
	PackageErrCodeFinalGateFailed:                      409,
	PackageErrCodeVersionActivateTargetNotFound:        409,
	PackageErrCodeInstallationNotFound:                 404,
	PackageErrCodeVersionRepositoryUnavailable:         503,
	PackageErrCodeInstallationGenerationIDMissing:      409,
	PackageErrCodeVersionDeactivateTargetNotFound:      409,
	PackageErrCodeCompensationFailed:                   500,
	PackageErrCodeMigrationSandboxViolation:            403,
	PackageErrCodeMigrationCompensationFailed:          500,
	PackageErrCodeQuarantineMetadataUnavailable:        503,
	PackageErrCodeQuarantineMetadataMissing:            409,
	PackageErrCodeQuarantineMetadataIncomplete:         409,
	PackageErrCodeQuarantineReleaseFailed:              500,
	PackageErrCodeQuarantineStatePersistFailed:         500,
	PackageErrCodeRecoveryStepPersistFailed:            500,
	PackageErrCodeRollbackSnapshotCorrupted:            409,
	PackageErrCodeUninstallArtifactPolicyUnproven:      409,
	PackageErrCodeUninstallArtifactMissing:             409,
	PackageErrCodeUninstallArtifactReferenced:          409,
	PackageErrCodeMigrationSQLUnparseable:              422,
	PackageErrCodeMigrationNamespaceViolation:          403,
	PackageErrCodeLegacyRuntimeEnabled:                 403,
	PackageErrCodeConfirmationTokenInvalid:             403,
	PackageErrCodeConfirmationBindingMismatch:          403,
	PackageErrCodeConfirmationStale:                    409,
	PackageErrCodeRollbackRequirementChanged:           409,
	PackageErrCodeQuarantinePathMismatch:               409,
	PackageErrCodeQuarantineMetadataCorrupt:            409,
	PackageErrCodeRecoveryStepEvidenceInvalid:          409,
	PackageErrCodeArtifactReferenceConflict:            409,
	PackageErrCodeRestoredStateMismatch:                409,
	PackageErrCodeFinalizationEvidenceMissing:          409,
	PackageErrCodeLeaseStopFailed:                      500,
	PackageErrCodeConfirmationRequired:                 403,
	PackageErrCodeConfirmationInvalid:                  403,
	PackageErrCodeConfirmationExpired:                  403,
	PackageErrCodeConfirmationItemsMissing:             403,
	PackageErrCodeConfirmationItemsMismatch:            403,
	PackageErrCodeConfirmationPolicyMismatch:           403,
	PackageErrCodeConfirmationPolicyVersionStale:       403,
	PackageErrCodeConfirmationPolicyVersionUnsupported: 403,
	PackageErrCodeArtifactPolicyMismatch:               409,
	PackageErrCodeArtifactDeleteIncomplete:             409,
	PackageErrCodeArtifactRetentionMismatch:            409,
	PackageErrCodeArtifactReferenceRemains:             409,
	PackageErrCodeSnapshotRequired:                     409,
	PackageErrCodeSnapshotMissing:                      409,
	PackageErrCodeSnapshotInvalid:                      409,
	PackageErrCodeSnapshotRequirementMismatch:          409,
	PackageErrCodeSnapshotExemptionInvalid:             409,
	PackageErrCodeSnapshotEvidenceMissing:              409,
	PackageErrCodeFinalGateEvidenceMissing:             409,
	PackageErrCodeUserDataEntityIDMissing:              422,
	PackageErrCodeUserDataRecordInvalid:                422,
	PackageErrCodeUserDataBatchHashMismatch:            409,
	PackageErrCodeUserDataAggregateHashMismatch:        409,
	PackageErrCodeUserDataRestoreJournalConflict:       409,
	PackageErrCodeResourceSnapshotPathInvalid:          400,
	PackageErrCodeResourceSnapshotSymlinkForbidden:     400,
	PackageErrCodeResourceSnapshotHashComputeFailed:    500,
	PackageErrCodeResourceSnapshotHashMismatch:         409,
	PackageErrCodeResourceSnapshotContentMissing:       409,
	PackageErrCodeResourceRestoreIncomplete:            409,
	PackageErrCodeSnapshotSchemaUnsupported:            422,
	PackageErrCodeSnapshotIntegrityFailed:              409,
}

func PackageErrorHTTPStatus(code string) int {
	if status, ok := packageErrorHTTPStatus[code]; ok {
		return status
	}
	return 422
}

type PackageOperationError struct {
	Code        string
	Step        string
	Cause       error
	Recoverable bool
}

func (e *PackageOperationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Step, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Step)
}

func (e *PackageOperationError) Unwrap() error {
	return e.Cause
}

func NewPackageOperationError(code, step string, cause error, recoverable bool) *PackageOperationError {
	return &PackageOperationError{Code: code, Step: step, Cause: cause, Recoverable: recoverable}
}

func PackageErrorToHTTPStatus(err error) int {
	var pkgErr *PackageError
	if errors.As(err, &pkgErr) {
		return pkgErr.HTTPStatus
	}
	var opErr *PackageOperationError
	if errors.As(err, &opErr) {
		return PackageErrorHTTPStatus(opErr.Code)
	}
	return 422
}

func PackageErrorResponse(err error) (int, string, string) {
	var pkgErr *PackageError
	if errors.As(err, &pkgErr) {
		return pkgErr.HTTPStatus, pkgErr.Code, err.Error()
	}
	var opErr *PackageOperationError
	if errors.As(err, &opErr) {
		return PackageErrorHTTPStatus(opErr.Code), opErr.Code, err.Error()
	}
	return 422, "", err.Error()
}

type RepositoryErrorKind string

const (
	RepositoryErrorNotFound    RepositoryErrorKind = "not_found"
	RepositoryErrorUnavailable RepositoryErrorKind = "unavailable"
	RepositoryErrorCorrupt     RepositoryErrorKind = "corrupt"
	RepositoryErrorConflict    RepositoryErrorKind = "conflict"
)

type RepositoryError struct {
	Kind  RepositoryErrorKind
	Cause error
}

func (e *RepositoryError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("repository %s: %v", e.Kind, e.Cause)
	}
	return fmt.Sprintf("repository %s", e.Kind)
}

func (e *RepositoryError) Unwrap() error {
	return e.Cause
}

func NewRepositoryError(kind RepositoryErrorKind, cause error) *RepositoryError {
	return &RepositoryError{Kind: kind, Cause: cause}
}

func IsRepositoryErrorKind(err error, kind RepositoryErrorKind) bool {
	return RepositoryErrorKindOf(err) == kind
}

func IsRepositoryError(err error) bool {
	var repoErr *RepositoryError
	return errors.As(err, &repoErr)
}

func RepositoryErrorKindOf(err error) RepositoryErrorKind {
	if err == nil {
		return ""
	}
	var repoErr *RepositoryError
	if errors.As(err, &repoErr) {
		return repoErr.Kind
	}
	if IsPackageOperationError(err, OperationErrNotFound) {
		return RepositoryErrorNotFound
	}
	if IsPackageOperationError(err, OperationErrStorageFailure) {
		return RepositoryErrorUnavailable
	}
	if IsPackageOperationError(err, OperationErrTransitionConflict) ||
		IsPackageOperationError(err, OperationErrStepInputConflict) ||
		IsPackageOperationError(err, OperationErrSideEffectConflict) ||
		IsPackageOperationError(err, OperationErrIdempotencyConflict) ||
		IsPackageOperationError(err, OperationErrLeaseConflict) {
		return RepositoryErrorConflict
	}
	if errors.Is(err, sql.ErrNoRows) {
		return RepositoryErrorNotFound
	}
	return RepositoryErrorUnavailable
}

func ClassifyRepositoryError(action string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return NewRepositoryError(RepositoryErrorNotFound, err)
	}
	if isSQLiteConstraintViolation(err) {
		return NewRepositoryError(RepositoryErrorConflict, err)
	}
	return NewRepositoryError(RepositoryErrorUnavailable, err)
}

func isSQLiteConstraintViolation(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "UNIQUE constraint failed") ||
		strings.Contains(errStr, "constraint failed")
}
