package browser

import "fmt"

type BrowserErrorCode string

const (
	ErrCodeInvalidRequest          BrowserErrorCode = "invalid_request"
	ErrCodeSessionNotFound         BrowserErrorCode = "session_not_found"
	ErrCodeTabNotFound             BrowserErrorCode = "tab_not_found"
	ErrCodeNavigationFailed        BrowserErrorCode = "navigation_failed"
	ErrCodeNavigationTimeout       BrowserErrorCode = "navigation_timeout"
	ErrCodeStaleElement            BrowserErrorCode = "stale_element"
	ErrCodeElementNotFound         BrowserErrorCode = "element_not_found"
	ErrCodeUnsupportedAction       BrowserErrorCode = "unsupported_operation"
	ErrCodeProviderUnavailable     BrowserErrorCode = "provider_unavailable"
	ErrCodeDownloadFailed          BrowserErrorCode = "download_failed"
	ErrCodeUploadFailed            BrowserErrorCode = "upload_failed"
	ErrCodeBrowserDisabled         BrowserErrorCode = "browser_disabled"
	ErrCodeBrowserExecNotFound     BrowserErrorCode = "browser_executable_not_found"
	ErrCodeBrowserStarting         BrowserErrorCode = "browser_starting"
	ErrCodeBrowserStartFailed      BrowserErrorCode = "browser_start_failed"
	ErrCodeBrowserConnFailed       BrowserErrorCode = "browser_connection_failed"
	ErrCodeBrowserHealthFailed     BrowserErrorCode = "browser_health_failed"
	ErrCodeBrowserProcessExited    BrowserErrorCode = "browser_process_exited"
	ErrCodeBrowserStopFailed       BrowserErrorCode = "browser_stop_failed"
	ErrCodeBrowserRuntimeNotReady  BrowserErrorCode = "browser_runtime_not_ready"
	ErrCodeSessionLimitReached     BrowserErrorCode = "session_limit_reached"
	ErrCodeSessionCreateFailed     BrowserErrorCode = "session_create_failed"
	ErrCodeSessionCloseFailed      BrowserErrorCode = "session_close_failed"
	ErrCodeSessionStale            BrowserErrorCode = "session_stale"
	ErrCodeTabQuotaReached         BrowserErrorCode = "tab_quota_reached"
	ErrCodeTabCreateFailed         BrowserErrorCode = "tab_create_failed"
	ErrCodeTabCloseFailed          BrowserErrorCode = "tab_close_failed"
	ErrCodeTabStale                BrowserErrorCode = "tab_stale"
	ErrCodeTabActivateFailed       BrowserErrorCode = "tab_activate_failed"
	ErrCodeInvalidSelector         BrowserErrorCode = "invalid_selector"
	ErrCodeElementNotInteractable  BrowserErrorCode = "element_not_interactable"
	ErrCodeElementOccluded         BrowserErrorCode = "element_occluded"
	ErrCodeDocumentNotReady        BrowserErrorCode = "document_not_ready"
	ErrCodeInteractionFailed       BrowserErrorCode = "interaction_failed"
	ErrCodeInteractionTimeout      BrowserErrorCode = "interaction_timeout"
	ErrCodeInteractionOutcomeUnknown BrowserErrorCode = "interaction_outcome_unknown"
	ErrCodeDOMSnapshotFailed       BrowserErrorCode = "dom_snapshot_failed"
	ErrCodeDOMSnapshotTooLarge    BrowserErrorCode = "dom_snapshot_too_large"

	ErrCodeDownloadNotStarted      BrowserErrorCode = "browser_download_not_started"
	ErrCodeDownloadAmbiguous       BrowserErrorCode = "browser_download_ambiguous"
	ErrCodeDownloadCancelled       BrowserErrorCode = "browser_download_cancelled"
	ErrCodeDownloadTooLarge        BrowserErrorCode = "browser_download_too_large"
	ErrCodeDownloadTimeout         BrowserErrorCode = "browser_download_timeout"
	ErrCodeDownloadStagingFailed   BrowserErrorCode = "browser_download_staging_failed"
	ErrCodeDownloadCommitFailed    BrowserErrorCode = "browser_download_commit_failed"
	ErrCodeDownloadOutcomeUnknown  BrowserErrorCode = "browser_download_outcome_unknown"

	ErrCodeUploadTargetNotFileInput BrowserErrorCode = "browser_upload_target_not_file_input"
	ErrCodeUploadResourceNotFound   BrowserErrorCode = "browser_upload_resource_not_found"
	ErrCodeUploadResourceUnavailable BrowserErrorCode = "browser_upload_resource_unavailable"
	ErrCodeUploadTooLarge           BrowserErrorCode = "browser_upload_too_large"
	ErrCodeUploadTypeNotAccepted    BrowserErrorCode = "browser_upload_type_not_accepted"
	ErrCodeUploadStagingFailed      BrowserErrorCode = "browser_upload_staging_failed"
	ErrCodeUploadOutcomeUnknown     BrowserErrorCode = "browser_upload_outcome_unknown"

	ErrCodeScreenshotInvalidFormat  BrowserErrorCode = "browser_screenshot_invalid_format"
	ErrCodeScreenshotInvalidQuality BrowserErrorCode = "browser_screenshot_invalid_quality"
	ErrCodeScreenshotTooLarge       BrowserErrorCode = "browser_screenshot_too_large"
	ErrCodeScreenshotFailed         BrowserErrorCode = "browser_screenshot_failed"

	ErrCodeRecoveryFailed           BrowserErrorCode = "browser_recovery_failed"
	ErrCodeTabCrashed               BrowserErrorCode = "browser_tab_crashed"
	ErrCodeRecoveryLimitReached     BrowserErrorCode = "browser_recovery_limit_reached"
)

type BrowserError struct {
	Code    BrowserErrorCode `json:"code"`
	Message string           `json:"message"`
	Cause   error            `json:"-"`
}

func (e *BrowserError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("browser: %s: %s (cause: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("browser: %s: %s", e.Code, e.Message)
}

func (e *BrowserError) Unwrap() error {
	return e.Cause
}

func IsSessionNotFound(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeSessionNotFound
}

func IsTabNotFound(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeTabNotFound
}

func IsStaleElement(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeStaleElement
}

func IsUnsupportedOperation(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeUnsupportedAction
}

func IsProviderUnavailable(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeProviderUnavailable
}

func IsBrowserDisabled(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeBrowserDisabled
}

func IsBrowserExecNotFound(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeBrowserExecNotFound
}

func IsBrowserRuntimeNotReady(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeBrowserRuntimeNotReady
}

func IsSessionLimitReached(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeSessionLimitReached
}

func IsSessionStale(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeSessionStale
}

func IsTabStale(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeTabStale
}

func IsTabQuotaReached(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeTabQuotaReached
}

func IsInvalidSelector(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeInvalidSelector
}

func IsElementNotInteractable(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeElementNotInteractable
}

func IsElementOccluded(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeElementOccluded
}

func IsDocumentNotReady(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeDocumentNotReady
}

func IsInteractionFailed(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeInteractionFailed
}

func IsDOMSnapshotFailed(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeDOMSnapshotFailed
}

func IsDownloadFailed(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeDownloadFailed
}

func IsUploadFailed(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeUploadFailed
}

func IsScreenshotFailed(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeScreenshotFailed
}

func IsRecoveryFailed(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeRecoveryFailed
}

func IsTabCrashed(err error) bool {
	be, ok := err.(*BrowserError)
	return ok && be.Code == ErrCodeTabCrashed
}
