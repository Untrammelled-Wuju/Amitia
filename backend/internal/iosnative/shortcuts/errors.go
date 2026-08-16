package shortcuts

const (
	ErrShortcutsUnavailable       = "SHORTCUTS_UNAVAILABLE"
	ErrShortcutsIntentUnsupported = "SHORTCUTS_INTENT_UNSUPPORTED"
	ErrShortcutsActionUnsupported = "SHORTCUTS_ACTION_UNSUPPORTED"
	ErrShortcutsActionNotExposed  = "SHORTCUTS_ACTION_NOT_EXPOSED"

	ErrShortcutsEntityNotFound     = "SHORTCUTS_ENTITY_NOT_FOUND"
	ErrShortcutsEntityQueryFailed  = "SHORTCUTS_ENTITY_QUERY_FAILED"
	ErrShortcutsEntityAccessDenied = "SHORTCUTS_ENTITY_ACCESS_DENIED"

	ErrShortcutsParameterInvalid  = "SHORTCUTS_PARAMETER_INVALID"
	ErrShortcutsParameterRequired = "SHORTCUTS_PARAMETER_REQUIRED"

	ErrShortcutsRuntimeUnavailable = "SHORTCUTS_RUNTIME_UNAVAILABLE"
	ErrShortcutsRuntimeStartFailed = "SHORTCUTS_RUNTIME_START_FAILED"

	ErrShortcutsBackgroundNotAllowed = "SHORTCUTS_BACKGROUND_NOT_ALLOWED"
	ErrShortcutsForegroundRequired   = "SHORTCUTS_FOREGROUND_REQUIRED"

	ErrShortcutsPermissionDenied      = "SHORTCUTS_PERMISSION_DENIED"
	ErrShortcutsConfirmationRequired  = "SHORTCUTS_CONFIRMATION_REQUIRED"
	ErrShortcutsConfirmationCancelled = "SHORTCUTS_CONFIRMATION_CANCELLED"

	ErrShortcutsActionFailed    = "SHORTCUTS_ACTION_FAILED"
	ErrShortcutsActionTimeout   = "SHORTCUTS_ACTION_TIMEOUT"
	ErrShortcutsActionCancelled = "SHORTCUTS_ACTION_CANCELLED"

	ErrShortcutsResultTooLarge    = "SHORTCUTS_RESULT_TOO_LARGE"
	ErrShortcutsResultUnsupported = "SHORTCUTS_RESULT_UNSUPPORTED"

	ErrShortcutsNativeBridgeUnavailable = "SHORTCUTS_NATIVE_BRIDGE_UNAVAILABLE"
	ErrShortcutsInvalidResponse         = "SHORTCUTS_INVALID_RESPONSE"
	ErrShortcutsIntentDonationFailed    = "SHORTCUTS_INTENT_DONATION_FAILED"
	ErrShortcutsContributionRejected    = "SHORTCUTS_CONTRIBUTION_REJECTED"
	ErrShortcutsExposureInvalid         = "SHORTCUTS_EXPOSURE_INVALID"
	ErrShortcutsExecutionModeInvalid    = "SHORTCUTS_EXECUTION_MODE_INVALID"
	ErrShortcutsRiskLevelInvalid        = "SHORTCUTS_RISK_LEVEL_INVALID"
	ErrShortcutsCanonicalTargetInvalid  = "SHORTCUTS_CANONICAL_TARGET_INVALID"
	ErrShortcutsIdempotencyInvalid      = "SHORTCUTS_IDEMPOTENCY_INVALID"
	ErrShortcutsSnapshotUnavailable     = "SHORTCUTS_SNAPSHOT_UNAVAILABLE"
	ErrShortcutsAppShortcutInvalid      = "SHORTCUTS_APP_SHORTCUT_INVALID"
	ErrShortcutsPhraseInvalid           = "SHORTCUTS_PHRASE_INVALID"
	ErrShortcutsLocaleInvalid           = "SHORTCUTS_LOCALE_INVALID"
	ErrShortcutsMetadataInvalid         = "SHORTCUTS_METADATA_INVALID"
	ErrShortcutsResultRedactionFailed   = "SHORTCUTS_RESULT_REDACTION_FAILED"
	ErrShortcutsSecretExposureRisk      = "SHORTCUTS_SECRET_EXPOSURE_RISK"

	ErrOutcomeUnknown = "SHORTCUTS_OUTCOME_UNKNOWN"
	ErrTimeout        = "SHORTCUTS_TIMEOUT"
	ErrCancelled      = "SHORTCUTS_CANCELLED"
)
