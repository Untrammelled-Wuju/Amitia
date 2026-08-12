package deepsearch

import "errors"

const (
	ErrDeepSearchUnavailabled            = "DEEP_SEARCH_UNAVAILABLE"
	ErrDeepSearchTaskRuntimeUnavailabled = "DEEP_SEARCH_TASK_RUNTIME_UNAVAILABLE"
	ErrDeepSearchTaskDefinitionUnavailabled = "DEEP_SEARCH_TASK_DEFINITION_UNAVAILABLE"
	ErrDeepSearchGeneralSearchUnavailabled   = "DEEP_SEARCH_GENERAL_SEARCH_UNAVAILABLE"
	ErrDeepSearchHostToolExecuteUnavailabled = "DEEP_SEARCH_HOST_TOOL_EXECUTE_UNAVAILABLE"
	ErrDeepSearchInvalidQuery               = "DEEP_SEARCH_INVALID_QUERY"
	ErrDeepSearchInvalidFocusArea           = "DEEP_SEARCH_INVALID_FOCUS_AREA"
	ErrDeepSearchBudgetExceeded            = "DEEP_SEARCH_BUDGET_EXCEEDED"
	ErrDeepSearchPlanningFailed            = "DEEP_SEARCH_PLANNING_FAILED"
	ErrDeepSearchChildSearchFailed         = "DEEP_SEARCH_CHILD_SEARCH_FAILED"
	ErrDeepSearchRateLimited               = "DEEP_SEARCH_RATE_LIMITED"
	ErrDeepSearchResultTooLarge            = "DEEP_SEARCH_RESULT_TOO_LARGE"
	ErrDeepSearchCheckpointTooLarge        = "DEEP_SEARCH_CHECKPOINT_TOO_LARGE"
	ErrDeepSearchRecoveryFailed           = "DEEP_SEARCH_RECOVERY_FAILED"
	ErrDeepSearchCancelled                = "DEEP_SEARCH_CANCELLED"
	ErrDeepSearchTimeout                  = "DEEP_SEARCH_TIMEOUT"
)

type Error struct {
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return e.Code + ": " + e.Message + " (" + e.Cause.Error() + ")"
	}
	return e.Code + ": " + e.Message
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func NewError(code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

var (
	ErrB49NotEnabled      = errors.New("b49 general search provider is not enabled")
	ErrTaskRuntimeMissing = errors.New("task runtime service is not configured")
	ErrInvalidQuery       = errors.New("invalid deep search query")
	ErrInvalidFocusArea   = errors.New("invalid focus area")
	ErrBudgetExceeded     = errors.New("search call budget exceeded")
)
