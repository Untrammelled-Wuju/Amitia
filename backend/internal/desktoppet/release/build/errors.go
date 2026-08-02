package build

import "fmt"

const (
	ErrCodeQualityGateMissing          = "quality_gate_missing"
	ErrCodeQualityGatePending          = "quality_gate_pending"
	ErrCodeQualityGateReviewRequired   = "quality_gate_review_required"
	ErrCodeQualityGateFailed           = "quality_gate_failed"
	ErrCodeQualityGateError            = "quality_gate_error"
	ErrCodeQualityGateStale            = "quality_gate_stale"
	ErrCodeReleaseOwnershipDenied      = "release_ownership_denied"
	ErrCodeReleaseSourceMismatch       = "release_source_mismatch"
	ErrCodeReleaseDefaultActionInvalid = "release_default_action_invalid"
	ErrCodeReleaseFrameAssetMissing    = "release_frame_asset_missing"
	ErrCodeReleaseFrameSetIncomplete   = "release_frame_set_incomplete"
	ErrCodeReleaseFrameHashMismatch    = "release_frame_hash_mismatch"
	ErrCodeReleaseInputHashMismatch    = "release_input_hash_mismatch"
	ErrCodeReleaseLeaseExpired         = "release_lease_expired"
	ErrCodeReleaseOperationConflict    = "release_operation_conflict"
	ErrCodeReleasePublishFailed        = "release_publish_failed"
	ErrCodeReleaseValidationFailed     = "release_validation_failed"
	ErrCodeLegacyPackageWriteDisabled  = "legacy_package_write_disabled"
	ErrCodeReleaseCorrupted            = "release_corrupted"
)

type BuildError struct {
	Code    string
	Message string
	Err     error
}

func (e *BuildError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *BuildError) Unwrap() error { return e.Err }

func NewBuildError(code, message string, err error) *BuildError {
	return &BuildError{Code: code, Message: message, Err: err}
}
