package background

const (
	ErrBackgroundUnavailable         = "IOS_BACKGROUND_UNAVAILABLE"
	ErrBackgroundRefreshUnavailable  = "IOS_BACKGROUND_REFRESH_UNAVAILABLE"
	ErrBackgroundProcessingUnavailable = "IOS_BACKGROUND_PROCESSING_UNAVAILABLE"
	ErrBackgroundContinuedUnavailable = "IOS_BACKGROUND_CONTINUED_UNAVAILABLE"

	ErrBackgroundIdentifierInvalid    = "IOS_BACKGROUND_IDENTIFIER_INVALID"
	ErrBackgroundIdentifierNotPermitted = "IOS_BACKGROUND_IDENTIFIER_NOT_PERMITTED"
	ErrBackgroundRegistrationFailed   = "IOS_BACKGROUND_REGISTRATION_FAILED"

	ErrBackgroundSubmissionFailed     = "IOS_BACKGROUND_SUBMISSION_FAILED"
	ErrBackgroundSubmissionNotImmediate = "IOS_BACKGROUND_SUBMISSION_NOT_IMMEDIATE"
	ErrBackgroundTooManyPending       = "IOS_BACKGROUND_TOO_MANY_PENDING"

	ErrBackgroundNetworkRequirementUnavailable = "IOS_BACKGROUND_NETWORK_REQUIREMENT_UNAVAILABLE"
	ErrBackgroundPowerRequirementUnavailable   = "IOS_BACKGROUND_POWER_REQUIREMENT_UNAVAILABLE"
	ErrBackgroundResourceUnsupported  = "IOS_BACKGROUND_RESOURCE_UNSUPPORTED"
	ErrBackgroundGPUEntitlementRequired = "IOS_BACKGROUND_GPU_ENTITLEMENT_REQUIRED"

	ErrBackgroundTaskNotFound        = "IOS_BACKGROUND_TASK_NOT_FOUND"
	ErrBackgroundTaskBindingInvalid  = "IOS_BACKGROUND_TASK_BINDING_INVALID"

	ErrBackgroundRuntimeUnavailable  = "IOS_BACKGROUND_RUNTIME_UNAVAILABLE"
	ErrBackgroundRuntimeStartFailed  = "IOS_BACKGROUND_RUNTIME_START_FAILED"

	ErrBackgroundExpired             = "IOS_BACKGROUND_EXPIRED"
	ErrBackgroundCancelled           = "IOS_BACKGROUND_CANCELLED"
	ErrBackgroundInterrupted         = "IOS_BACKGROUND_INTERRUPTED"
	ErrBackgroundProgressInvalid     = "IOS_BACKGROUND_PROGRESS_INVALID"
	ErrBackgroundCompletionFailed    = "IOS_BACKGROUND_COMPLETION_FAILED"

	ErrBackgroundNotUserInitiated    = "IOS_BACKGROUND_NOT_USER_INITIATED"
	ErrBackgroundUIRequired          = "IOS_BACKGROUND_UI_REQUIRED"

	ErrFileUnavailable               = "IOS_FILE_UNAVAILABLE"
	ErrFilePickerUIRequired          = "IOS_FILE_PICKER_UI_REQUIRED"
	ErrFileSelectionCancelled        = "IOS_FILE_SELECTION_CANCELLED"

	ErrFileGrantInvalid              = "IOS_FILE_GRANT_INVALID"
	ErrFileGrantNotFound             = "IOS_FILE_GRANT_NOT_FOUND"
	ErrFileGrantStale                = "IOS_FILE_GRANT_STALE"
	ErrFilePermissionRevoked         = "IOS_FILE_PERMISSION_REVOKED"

	ErrFileSecurityScopeFailed       = "IOS_FILE_SECURITY_SCOPE_FAILED"
	ErrFileCoordinationFailed        = "IOS_FILE_COORDINATION_FAILED"
	ErrFileCoordinationCancelled     = "IOS_FILE_COORDINATION_CANCELLED"

	ErrFileNotFound                  = "IOS_FILE_NOT_FOUND"
	ErrFileProviderUnavailable       = "IOS_FILE_PROVIDER_UNAVAILABLE"
	ErrFileProviderOffline           = "IOS_FILE_PROVIDER_OFFLINE"

	ErrFileReadFailed                = "IOS_FILE_READ_FAILED"
	ErrFileWriteFailed               = "IOS_FILE_WRITE_FAILED"
	ErrFileMoveFailed                = "IOS_FILE_MOVE_FAILED"
	ErrFileCopyFailed                = "IOS_FILE_COPY_FAILED"
	ErrFileDeleteFailed              = "IOS_FILE_DELETE_FAILED"

	ErrFileImportFailed              = "IOS_FILE_IMPORT_FAILED"
	ErrFileExportFailed              = "IOS_FILE_EXPORT_FAILED"
	ErrFileMaterializeFailed         = "IOS_FILE_MATERIALIZE_FAILED"

	ErrFilePathInvalid               = "IOS_FILE_PATH_INVALID"
	ErrFileSizeLimitExceeded         = "IOS_FILE_SIZE_LIMIT_EXCEEDED"
	ErrFileOutcomeUnknown            = "IOS_FILE_OUTCOME_UNKNOWN"

	ErrOutcomeUnknown                = "IOS_OUTCOME_UNKNOWN"
	ErrNativeBridgeUnavailable       = "IOS_NATIVE_BRIDGE_UNAVAILABLE"
	ErrInvalidResponse               = "IOS_INVALID_RESPONSE"
	ErrTimeout                       = "IOS_TIMEOUT"
	ErrCancelled                     = "IOS_CANCELLED"
	ErrTaskRuntimeError              = "IOS_TASK_RUNTIME_ERROR"
)
