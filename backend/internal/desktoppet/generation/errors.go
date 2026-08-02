package generation

import "fmt"

const (
	ErrCodePlanInvalid                  = "GEN_PLAN_INVALID"
	ErrCodePlanCapabilityMismatch       = "GEN_PLAN_CAPABILITY_MISMATCH"
	ErrCodePlanModeNotSupported         = "GEN_PLAN_MODE_NOT_SUPPORTED"
	ErrCodePlanDimensionNotSupported    = "GEN_PLAN_DIMENSION_NOT_SUPPORTED"
	ErrCodePlanReferenceNotSupported    = "GEN_PLAN_REFERENCE_NOT_SUPPORTED"
	ErrCodePlanBudgetExceeded           = "GEN_PLAN_BUDGET_EXCEEDED"
	ErrCodePlanLayoutMissing            = "GEN_PLAN_LAYOUT_MISSING"
	ErrCodePlanPromptMissing            = "GEN_PLAN_PROMPT_MISSING"
	ErrCodePlanSegmentCountZero         = "GEN_PLAN_SEGMENT_COUNT_ZERO"
	ErrCodePlanReferenceAssetMissing    = "GEN_PLAN_REFERENCE_ASSET_MISSING"
	ErrCodeAttemptNotFound              = "GEN_ATTEMPT_NOT_FOUND"
	ErrCodeAttemptAlreadyActive         = "GEN_ATTEMPT_ALREADY_ACTIVE"
	ErrCodeAttemptConflict              = "GEN_ATTEMPT_CONFLICT"
	ErrCodeAttemptLeaseExpired          = "GEN_ATTEMPT_LEASE_EXPIRED"
	ErrCodeAttemptLockFailed            = "GEN_ATTEMPT_LOCK_FAILED"
	ErrCodeAttemptNumberExhausted       = "GEN_ATTEMPT_NUMBER_EXHAUSTED"
	ErrCodeAttemptNumberConflict        = "GEN_ATTEMPT_NUMBER_CONFLICT"
	ErrCodeArtifactNotFound             = "GEN_ARTIFACT_NOT_FOUND"
	ErrCodeArtifactPersistFailed        = "GEN_ARTIFACT_PERSIST_FAILED"
	ErrCodeArtifactPrimaryExists        = "GEN_ARTIFACT_PRIMARY_EXISTS"
	ErrCodeArtifactHashMismatch         = "GEN_ARTIFACT_HASH_MISMATCH"
	ErrCodeArtifactStagingFailed        = "GEN_ARTIFACT_STAGING_FAILED"
	ErrCodeArtifactRenameFailed         = "GEN_ARTIFACT_RENAME_FAILED"
	ErrCodeArtifactValidationFailed     = "GEN_ARTIFACT_VALIDATION_FAILED"
	ErrCodeArtifactBlankImage           = "GEN_ARTIFACT_BLANK_IMAGE"
	ErrCodeArtifactFullyTransparent     = "GEN_ARTIFACT_FULLY_TRANSPARENT"
	ErrCodeArtifactPublishFailed        = "GEN_ARTIFACT_PUBLISH_FAILED"
	ErrCodeRecoveryUnsupported          = "GEN_RECOVERY_UNSUPPORTED"
	ErrCodeRecoveryIdempotentNotSafe    = "GEN_RECOVERY_IDEMPOTENT_NOT_SAFE"
	ErrCodeRecoveryArtifactPersisted    = "GEN_RECOVERY_ARTIFACT_ALREADY_PERSISTED"
	ErrCodeRecoveryStatusUnknown        = "GEN_RECOVERY_STATUS_UNKNOWN"
	ErrCodeRecoveryParentMissing        = "GEN_RECOVERY_PARENT_MISSING"
	ErrCodeRecoveryLeaseConflict        = "GEN_RECOVERY_LEASE_CONFLICT"
	ErrCodeRecoveryPollingExhausted     = "GEN_RECOVERY_POLLING_EXHAUSTED"
	ErrCodeBudgetPrimaryRequestsExceed  = "GEN_BUDGET_PRIMARY_REQUESTS_EXCEEDED"
	ErrCodeBudgetProviderCallsExceed    = "GEN_BUDGET_PROVIDER_CALLS_EXCEEDED"
	ErrCodeBudgetOutputImagesExceed     = "GEN_BUDGET_OUTPUT_IMAGES_EXCEEDED"
	ErrCodeBudgetTotalPixelsExceed      = "GEN_BUDGET_TOTAL_PIXELS_EXCEEDED"
	ErrCodeBudgetEstimatedAmountExceed  = "GEN_BUDGET_ESTIMATED_AMOUNT_EXCEEDED"
	ErrCodeActiveBindingNotFound        = "GEN_ACTIVE_BINDING_NOT_FOUND"
	ErrCodeActiveBindingCASConflict     = "GEN_ACTIVE_BINDING_CAS_CONFLICT"
	ErrCodeActiveBindingArtifactInvalid = "GEN_ACTIVE_BINDING_ARTIFACT_INVALID"
	ErrCodeProviderReceiptNotFound      = "GEN_PROVIDER_RECEIPT_NOT_FOUND"
	ErrCodeProviderReceiptPersistFailed = "GEN_PROVIDER_RECEIPT_PERSIST_FAILED"
	ErrCodeFinalizeFailed               = "GEN_FINALIZE_FAILED"
	ErrCodeFinalizeRetryExhausted       = "GEN_FINALIZE_RETRY_EXHAUSTED"
	ErrCodeCancelNotSupported           = "GEN_CANCEL_NOT_SUPPORTED"
	ErrCodeCancelAlreadyTerminal        = "GEN_CANCEL_ALREADY_TERMINAL"
)

type GenerationError struct {
	Code    string
	Message string
	Cause   error
}

func (e *GenerationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *GenerationError) Unwrap() error {
	return e.Cause
}

func (e *GenerationError) ErrorCode() string {
	return e.Code
}

func NewGenerationError(code, message string, cause error) *GenerationError {
	return &GenerationError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

func IsGenerationError(err error) bool {
	_, ok := err.(*GenerationError)
	return ok
}

func ErrorCodeOf(err error) string {
	if ge, ok := err.(*GenerationError); ok {
		return ge.Code
	}
	return ErrCodePlanInvalid
}
