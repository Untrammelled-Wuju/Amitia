// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

const (
	ErrCodeDesktopPetNameRequired     = "DESKTOP_PET_NAME_REQUIRED"
	ErrCodeReferenceImageRequired     = "REFERENCE_IMAGE_REQUIRED"
	ErrCodeReferenceImageInvalid      = "REFERENCE_IMAGE_INVALID"
	ErrCodeReferenceImageTooLarge     = "REFERENCE_IMAGE_TOO_LARGE"
	ErrCodeCharacterNotFound          = "CHARACTER_NOT_FOUND"
	ErrCodeImageModelNotFound         = "IMAGE_MODEL_NOT_FOUND"
	ErrCodeImageModelDisabled         = "IMAGE_MODEL_DISABLED"
	ErrCodeImageModelTypeUnsupported  = "IMAGE_MODEL_TYPE_UNSUPPORTED"
	ErrCodeActionSelectionRequired    = "ACTION_SELECTION_REQUIRED"
	ErrCodeActionNotFound             = "ACTION_NOT_FOUND"
	ErrCodeActionDisabled             = "ACTION_DISABLED"
	ErrCodeDefaultIdleActionRequired  = "DEFAULT_IDLE_ACTION_REQUIRED"
	ErrCodeGenerationTaskCreateFailed = "GENERATION_TASK_CREATE_FAILED"
	ErrCodeGenerationTaskNotFound     = "GENERATION_TASK_NOT_FOUND"
	ErrCodeTaskStatusNotDeletable     = "TASK_STATUS_NOT_DELETABLE"
	ErrCodeTaskNotOwned               = "TASK_NOT_OWNED"

	ErrCodeImageModelUnavailable           = "IMAGE_MODEL_UNAVAILABLE"
	ErrCodeImageModelCapabilityUnsupported = "IMAGE_MODEL_CAPABILITY_UNSUPPORTED"
	ErrCodeImageModelCredentialMissing     = "IMAGE_MODEL_CREDENTIAL_MISSING"
	ErrCodeImageGenerationRequestInvalid   = "IMAGE_GENERATION_REQUEST_INVALID"
	ErrCodeImageGenerationProviderRejected = "IMAGE_GENERATION_PROVIDER_REJECTED"
	ErrCodeImageGenerationRateLimited      = "IMAGE_GENERATION_RATE_LIMITED"
	ErrCodeImageGenerationAuthFailed       = "IMAGE_GENERATION_AUTH_FAILED"
	ErrCodeImageGenerationTimeout          = "IMAGE_GENERATION_TIMEOUT"
	ErrCodeImageGenerationPollFailed       = "IMAGE_GENERATION_POLL_FAILED"
	ErrCodeImageGenerationCancelled        = "IMAGE_GENERATION_CANCELLED"
	ErrCodeImageGenerationEmptyResult      = "IMAGE_GENERATION_EMPTY_RESULT"
	ErrCodeImageResultDownloadFailed       = "IMAGE_RESULT_DOWNLOAD_FAILED"
	ErrCodeImageResultTooLarge             = "IMAGE_RESULT_TOO_LARGE"
	ErrCodeImageResultInvalidFormat        = "IMAGE_RESULT_INVALID_FORMAT"
	ErrCodeImageResultDecodeFailed         = "IMAGE_RESULT_DECODE_FAILED"
	ErrCodeImageResultSaveFailed           = "IMAGE_RESULT_SAVE_FAILED"
	ErrCodeGenerationWorkerError           = "GENERATION_WORKER_ERROR"
	ErrCodeGenerationStateConflict         = "GENERATION_STATE_CONFLICT"
	ErrCodeGenerationTaskAlreadyRunning    = "GENERATION_TASK_ALREADY_RUNNING"
	ErrCodeFrameNotFound                   = "FRAME_NOT_FOUND"
	ErrCodeArtifactUntrusted               = "DESKTOP_PET_ARTIFACT_UNTRUSTED"

	ErrCodeDataConflict              = "DESKTOP_PET_DATA_CONFLICT"
	ErrCodeDuplicateIdentity         = "DESKTOP_PET_DUPLICATE_IDENTITY"
	ErrCodeExecutionOwnershipLost    = "DESKTOP_PET_EXECUTION_OWNERSHIP_LOST"
	ErrCodeAttemptConflict           = "DESKTOP_PET_ATTEMPT_CONFLICT"
	ErrCodeMigrationUnresolved       = "DESKTOP_PET_MIGRATION_UNRESOLVED"
	ErrCodeVersionAllocationConflict = "DESKTOP_PET_VERSION_ALLOCATION_CONFLICT"
)

type BusinessError struct {
	Code    int
	Msg     string
	ErrCode string
}

func (e *BusinessError) Error() string { return e.Msg }

func NewBusinessError(code int, errCode, msg string) *BusinessError {
	return &BusinessError{Code: code, Msg: msg, ErrCode: errCode}
}
