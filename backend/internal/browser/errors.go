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
