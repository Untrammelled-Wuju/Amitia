package kernel

import (
	"errors"
	"fmt"
)

var (
	ErrPackageAlreadyInstalled                = errors.New("kernel: package already installed")
	ErrPackageNotInstalled                    = errors.New("kernel: package not installed")
	ErrPackageOwnerMismatch                   = errors.New("kernel: package scope mismatch")
	ErrPackageUpdateIDMismatch                = errors.New("kernel: package update id mismatch")
	ErrPackageUpdateTargetUnchanged           = errors.New("kernel: package update target unchanged")
	ErrPackageUninstallPreflightChanged       = errors.New("kernel: package uninstall preflight changed")
	ErrPackagePreviewSessionExpired           = errors.New("kernel: preview session expired")
	ErrPackagePreviewSessionStatus            = errors.New("kernel: preview session status invalid")
	ErrPackageSecurityPolicyChanged           = errors.New("kernel: preview security policy changed")
	ErrPackageArtifactIdentityMismatch        = errors.New("kernel: artifact identity mismatch")
	ErrPackageConfirmationTokenInvalid        = errors.New("kernel: confirmation token invalid")
	ErrPackageConfirmationBindingMismatch     = errors.New("kernel: confirmation token binding mismatch")
	ErrPackageRollbackPointUnavailable        = errors.New("kernel: rollback point unavailable")
	ErrPackageRollbackPointCorrupt            = errors.New("kernel: rollback point corrupt")
	ErrPackageRollbackPointExpired            = errors.New("kernel: rollback point expired")
	ErrPackageRollbackPointHashMismatch       = errors.New("kernel: rollback point snapshot hash mismatch")
	ErrPackageMigrationPlanDrift              = errors.New("kernel: package migration plan drift")
	ErrPackageMigrationPreflightMissing       = errors.New("kernel: package migration preflight evidence missing")
	ErrPackageMigrationManualRequired         = errors.New("kernel: package migration requires controlled manual recovery")
	ErrPackageServicesUnavailable             = errors.New("kernel: package services unavailable")
	ErrPackageDependencyStateChanged          = errors.New("kernel: dependency or compatibility state changed after preview")
	ErrPackageUninstallEvidenceIncomplete     = errors.New("kernel: uninstall installation evidence incomplete")
	ErrPackageUninstallCurrentPointerMismatch = errors.New("kernel: uninstall current pointer mismatch")
	ErrPackageUninstallTreeVerificationFailed = errors.New("kernel: uninstall installed tree verification failed")
	ErrPackageForwardRecoveryPointFailed      = errors.New("kernel: forward recovery point creation failed")
	ErrPackageManualRecoveryRequired          = errors.New("kernel: requires_manual_recovery")
	ErrPackageIdempotencyKeyRequired          = errors.New("kernel: idempotency key required from caller")
	ErrPackageIdempotencyKeyReused            = errors.New("kernel: idempotency key reused with different request")
	ErrPackageOperationConflict               = errors.New("kernel: package operation conflict")
	ErrPackageLeaseAcquireFailed              = errors.New("kernel: extension lease acquire failed")
	ErrPackageLeaseLost                       = errors.New("kernel: extension lease lost or expired")
	ErrPackageLeaseFenced                     = errors.New("kernel: extension lease fenced by newer token")
	ErrPackageMigrationFailed                 = errors.New("kernel: package migration execution failed")
	ErrPackageMigrationIrreversible           = errors.New("kernel: package migration is irreversible")
	ErrPackageRecoveryAmbiguous               = errors.New("kernel: package recovery state ambiguous")
	ErrPackageArtifactMetadataMismatch        = errors.New("kernel: artifact metadata mismatch")
	ErrPackageQuarantineRestoreFailed         = errors.New("kernel: quarantine restore failed")
	ErrPackageSnapshotIncomplete              = errors.New("kernel: rollback snapshot incomplete")
	ErrPackageVersionHistoryCorrupted         = errors.New("kernel: package version history corrupted")
	ErrPackageFinalGateFailed                 = errors.New("kernel: final gate validation failed")
)

const (
	PackageErrCodeIdempotencyKeyRequired      = "PACKAGE_IDEMPOTENCY_KEY_REQUIRED"
	PackageErrCodeIdempotencyKeyReused        = "PACKAGE_IDEMPOTENCY_KEY_REUSED"
	PackageErrCodeOperationConflict           = "PACKAGE_OPERATION_CONFLICT"
	PackageErrCodeLeaseAcquireFailed         = "PACKAGE_LEASE_ACQUIRE_FAILED"
	PackageErrCodeLeaseLost                   = "PACKAGE_LEASE_LOST"
	PackageErrCodeLeaseFenced                 = "PACKAGE_LEASE_FENCED"
	PackageErrCodeMigrationFailed             = "PACKAGE_MIGRATION_FAILED"
	PackageErrCodeMigrationIrreversible       = "PACKAGE_MIGRATION_IRREVERSIBLE"
	PackageErrCodeRecoveryAmbiguous           = "PACKAGE_RECOVERY_AMBIGUOUS"
	PackageErrCodeArtifactMetadataMismatch   = "PACKAGE_ARTIFACT_METADATA_MISMATCH"
	PackageErrCodeQuarantineRestoreFailed     = "PACKAGE_QUARANTINE_RESTORE_FAILED"
	PackageErrCodeSnapshotIncomplete          = "PACKAGE_SNAPSHOT_INCOMPLETE"
	PackageErrCodeVersionHistoryCorrupted    = "PACKAGE_VERSION_HISTORY_CORRUPTED"
	PackageErrCodeFinalGateFailed            = "PACKAGE_FINAL_GATE_FAILED"
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

var packageErrorHTTPStatus = map[string]int{
	PackageErrCodeIdempotencyKeyRequired:    400,
	PackageErrCodeIdempotencyKeyReused:      409,
	PackageErrCodeOperationConflict:         409,
	PackageErrCodeLeaseAcquireFailed:        409,
	PackageErrCodeLeaseLost:                 409,
	PackageErrCodeLeaseFenced:               409,
	PackageErrCodeMigrationFailed:           500,
	PackageErrCodeMigrationIrreversible:     422,
	PackageErrCodeRecoveryAmbiguous:         409,
	PackageErrCodeArtifactMetadataMismatch: 409,
	PackageErrCodeQuarantineRestoreFailed:  500,
	PackageErrCodeSnapshotIncomplete:       409,
	PackageErrCodeVersionHistoryCorrupted:  409,
	PackageErrCodeFinalGateFailed:          409,
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
