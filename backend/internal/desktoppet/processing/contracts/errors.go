package contracts

import "fmt"

type ErrInvalidConfig string

func (e ErrInvalidConfig) Error() string {
	return string(e)
}

type ProcessingError struct {
	Code       string
	Message    string
	Stage      string
	Retryable  bool
	Degraded   bool
	Provider   string
	ArtifactID string
	FrameIndex int
	Cause      error
}

type ErrorOption func(*ProcessingError)

func WithStage(stage string) ErrorOption {
	return func(e *ProcessingError) { e.Stage = stage }
}

func WithRetryable(retryable bool) ErrorOption {
	return func(e *ProcessingError) { e.Retryable = retryable }
}

func WithDegraded(degraded bool) ErrorOption {
	return func(e *ProcessingError) { e.Degraded = degraded }
}

func WithProvider(provider string) ErrorOption {
	return func(e *ProcessingError) { e.Provider = provider }
}

func WithArtifactID(artifactID string) ErrorOption {
	return func(e *ProcessingError) { e.ArtifactID = artifactID }
}

func WithFrameIndex(frameIndex int) ErrorOption {
	return func(e *ProcessingError) { e.FrameIndex = frameIndex }
}

func WithCause(cause error) ErrorOption {
	return func(e *ProcessingError) { e.Cause = cause }
}

func NewProcessingError(code, message string, opts ...ErrorOption) *ProcessingError {
	e := &ProcessingError{
		Code:    code,
		Message: message,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(e)
		}
	}
	return e
}

func (e *ProcessingError) Error() string {
	if e == nil {
		return ""
	}
	msg := e.Message
	if e.Cause != nil {
		msg = msg + ": " + e.Cause.Error()
	}
	if e.Stage != "" {
		return fmt.Sprintf("processing error [%s] @ %s: %s", e.Code, e.Stage, msg)
	}
	return fmt.Sprintf("processing error [%s]: %s", e.Code, msg)
}

func (e *ProcessingError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *ProcessingError) IsRetryable() bool {
	if e == nil {
		return false
	}
	return e.Retryable
}

const (
	ErrCodeSourceArtifactNotFound = "PROCESSING_SOURCE_ARTIFACT_NOT_FOUND"
	ErrCodeSourceHashMismatch     = "PROCESSING_SOURCE_HASH_MISMATCH"
	ErrCodeSourcePathUnsafe       = "PROCESSING_SOURCE_PATH_UNSAFE"
	ErrCodeLayoutMissing          = "PROCESSING_LAYOUT_MISSING"
	ErrCodeLayoutInvalid          = "PROCESSING_LAYOUT_INVALID"
	ErrCodeLayoutSizeMismatch     = "PROCESSING_LAYOUT_SIZE_MISMATCH"
	ErrCodeCellOutOfBounds        = "PROCESSING_CELL_OUT_OF_BOUNDS"
	ErrCodeFrameMappingConflict   = "PROCESSING_FRAME_MAPPING_CONFLICT"

	ErrCodeDecodeUnsupportedMIME    = "PROCESSING_DECODE_UNSUPPORTED_MIME"
	ErrCodeDecodeLimitExceeded      = "PROCESSING_DECODE_LIMIT_EXCEEDED"
	ErrCodeDecodeFailed             = "PROCESSING_DECODE_FAILED"
	ErrCodeOrientationFailed        = "PROCESSING_ORIENTATION_FAILED"
	ErrCodePixelNormalizationFailed = "PROCESSING_PIXEL_NORMALIZATION_FAILED"

	ErrCodeBackgroundProviderNotFound           = "BACKGROUND_PROVIDER_NOT_FOUND"
	ErrCodeBackgroundProviderCapabilityMismatch = "BACKGROUND_PROVIDER_CAPABILITY_MISMATCH"
	ErrCodeBackgroundProviderUnavailable        = "BACKGROUND_PROVIDER_UNAVAILABLE"
	ErrCodeBackgroundProviderAuthFailed         = "BACKGROUND_PROVIDER_AUTH_FAILED"
	ErrCodeBackgroundProviderTimeout            = "BACKGROUND_PROVIDER_TIMEOUT"
	ErrCodeBackgroundProviderResultInvalid      = "BACKGROUND_PROVIDER_RESULT_INVALID"
	ErrCodeBackgroundAlphaInvalid               = "BACKGROUND_ALPHA_INVALID"
	ErrCodeBackgroundMaskInvalid                = "BACKGROUND_MASK_INVALID"

	ErrCodeSubjectNotFound              = "SUBJECT_NOT_FOUND"
	ErrCodeSubjectGeometryFailed        = "SUBJECT_GEOMETRY_FAILED"
	ErrCodeScaleBaselineUnavailable     = "SCALE_BASELINE_UNAVAILABLE"
	ErrCodeScaleConstraintUnsatisfiable = "SCALE_CONSTRAINT_UNSATISFIABLE"
	ErrCodeResampleFailed               = "RESAMPLE_FAILED"
	ErrCodeAnchorPolicyMissing          = "ANCHOR_POLICY_MISSING"
	ErrCodeAnchorEstimationFailed       = "ANCHOR_ESTIMATION_FAILED"
	ErrCodeCanvasMappingFailed          = "CANVAS_MAPPING_FAILED"
	ErrCodeAlignmentProfileMissing      = "ALIGNMENT_PROFILE_MISSING"
	ErrCodeAlignmentFailed              = "ALIGNMENT_FAILED"

	ErrCodeEncodingFormatUnsupported = "ENCODING_FORMAT_UNSUPPORTED"
	ErrCodeEncodingFailed            = "ENCODING_FAILED"
	ErrCodeArtifactHashFailed        = "ARTIFACT_HASH_FAILED"
	ErrCodeArtifactValidationFailed  = "ARTIFACT_VALIDATION_FAILED"
	ErrCodeWorkdirCreateFailed       = "WORKDIR_CREATE_FAILED"
	ErrCodeRevisionPublishFailed     = "REVISION_PUBLISH_FAILED"
	ErrCodeRevisionDBCommitFailed    = "REVISION_DB_COMMIT_FAILED"
	ErrCodeRevisionIncomplete        = "REVISION_INCOMPLETE"

	ErrCodeProcessingCancelled            = "PROCESSING_CANCELLED"
	ErrCodeProcessingStageTimeout         = "PROCESSING_STAGE_TIMEOUT"
	ErrCodeProcessingMemoryBudgetExceeded = "PROCESSING_MEMORY_BUDGET_EXCEEDED"

	ErrCodeSourceManifestMissing        = "PROCESSING_SOURCE_MANIFEST_MISSING"
	ErrCodeSourceManifestHashMismatch   = "PROCESSING_SOURCE_MANIFEST_HASH_MISMATCH"
	ErrCodeSourceBindingInvalid         = "PROCESSING_SOURCE_BINDING_INVALID"
	ErrCodeSourceArtifactNotPersisted   = "PROCESSING_SOURCE_ARTIFACT_NOT_PERSISTED"
	ErrCodeSourceArtifactHashMismatch   = "PROCESSING_SOURCE_ARTIFACT_HASH_MISMATCH"
	ErrCodeSourceArtifactPathMismatch   = "PROCESSING_SOURCE_ARTIFACT_PATH_MISMATCH"
	ErrCodeSourceMIMEMismatch           = "PROCESSING_SOURCE_MIME_MISMATCH"
	ErrCodeSourceDimensionMismatch      = "PROCESSING_SOURCE_DIMENSION_MISMATCH"
	ErrCodeSourceFrameIndexInvalid      = "PROCESSING_SOURCE_FRAME_INDEX_INVALID"
	ErrCodeSourceActionSpecMismatch     = "PROCESSING_SOURCE_ACTION_SPEC_MISMATCH"
	ErrCodeSourceConfigHashMismatch     = "PROCESSING_SOURCE_CONFIG_HASH_MISMATCH"
	ErrCodeSourceStorageKeyMismatch     = "PROCESSING_SOURCE_STORAGE_KEY_MISMATCH"

	ErrCodeAttemptLeaseLost             = "PROCESSING_ATTEMPT_LEASE_LOST"
	ErrCodePipelineEmptyResult          = "PROCESSING_PIPELINE_EMPTY_RESULT"
	ErrCodeCommitConflict               = "PROCESSING_COMMIT_CONFLICT"
	ErrCodeRevisionNumberConflict       = "PROCESSING_REVISION_NUMBER_CONFLICT"
	ErrCodeRevisionPathConflict         = "PROCESSING_REVISION_PATH_CONFLICT"
	ErrCodeRevisionIntegrityFailed      = "PROCESSING_REVISION_INTEGRITY_FAILED"
	ErrCodeCommitRecoveryRequired       = "PROCESSING_COMMIT_RECOVERY_REQUIRED"
)
