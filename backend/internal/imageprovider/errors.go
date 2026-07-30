package imageprovider

import "fmt"

type RetryClass string

const (
	RetryNever      RetryClass = "never"
	RetrySameCall   RetryClass = "same_call"
	RetryQueryFirst RetryClass = "query_first"
	RetryUserAction RetryClass = "user_action"
	RetryAfterDelay RetryClass = "after_delay"
)

const (
	ErrCodeConfigMissing            = "IMAGE_PROVIDER_CONFIG_MISSING"
	ErrCodeProviderNotRegistered    = "IMAGE_PROVIDER_NOT_REGISTERED"
	ErrCodeModelNotSupported        = "IMAGE_MODEL_NOT_SUPPORTED"
	ErrCodeReferenceImageNotSupported = "IMAGE_REFERENCE_IMAGE_NOT_SUPPORTED"
	ErrCodeDimensionNotSupported    = "IMAGE_DIMENSION_NOT_SUPPORTED"
	ErrCodePromptTooLong            = "IMAGE_PROMPT_TOO_LONG"
	ErrCodeAuthFailed               = "IMAGE_GENERATION_AUTH_FAILED"
	ErrCodeInsufficientBalance      = "IMAGE_GENERATION_INSUFFICIENT_BALANCE"
	ErrCodeRateLimited              = "IMAGE_GENERATION_RATE_LIMITED"
	ErrCodeContentPolicyRejected    = "IMAGE_GENERATION_CONTENT_POLICY_REJECTED"
	ErrCodeRequestInvalid           = "IMAGE_GENERATION_REQUEST_INVALID"
	ErrCodeServiceError             = "IMAGE_GENERATION_SERVICE_ERROR"
	ErrCodeNetworkTimeout           = "IMAGE_GENERATION_NETWORK_TIMEOUT"
	ErrCodeStatusUnknown            = "IMAGE_GENERATION_STATUS_UNKNOWN"
	ErrCodeAsyncQueryFailed         = "IMAGE_GENERATION_ASYNC_QUERY_FAILED"
	ErrCodeCancellationNotSupported = "IMAGE_GENERATION_CANCELLATION_NOT_SUPPORTED"
	ErrCodeEmptyResult              = "IMAGE_GENERATION_EMPTY_RESULT"
	ErrCodeDownloadFailed           = "IMAGE_RESULT_DOWNLOAD_FAILED"
	ErrCodeInvalidFormat            = "IMAGE_RESULT_INVALID_FORMAT"
	ErrCodePixelLimitExceeded       = "IMAGE_RESULT_PIXEL_LIMIT_EXCEEDED"
	ErrCodeSaveFailed               = "IMAGE_RESULT_SAVE_FAILED"
	ErrCodeOutputCountExceeded      = "IMAGE_OUTPUT_COUNT_EXCEEDED"
	ErrCodeReferenceImageExceeded   = "IMAGE_REFERENCE_IMAGE_EXCEEDED"
)

type ProviderError struct {
	Code       string
	Message    string
	RetryClass RetryClass
	HTTPStatus int
	Cause      error
}

func (e *ProviderError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *ProviderError) Unwrap() error {
	return e.Cause
}

func (e *ProviderError) ErrorCode() string {
	return e.Code
}

func NewProviderError(code, message string, retryClass RetryClass, httpStatus int, cause error) *ProviderError {
	return &ProviderError{
		Code:       code,
		Message:    message,
		RetryClass: retryClass,
		HTTPStatus: httpStatus,
		Cause:      cause,
	}
}

func ClassifyHTTPError(status int, body string) *ProviderError {
	switch {
	case status == 401 || status == 403:
		return NewProviderError(ErrCodeAuthFailed, "authentication failed", RetryNever, status, nil)
	case status == 402:
		return NewProviderError(ErrCodeInsufficientBalance, "insufficient balance", RetryNever, status, nil)
	case status == 429:
		return NewProviderError(ErrCodeRateLimited, "rate limited", RetryAfterDelay, status, nil)
	case status == 400:
		return NewProviderError(ErrCodeRequestInvalid, "request invalid", RetryNever, status, nil)
	case status >= 500:
		return NewProviderError(ErrCodeServiceError, "provider service error", RetryAfterDelay, status, nil)
	default:
		return NewProviderError(ErrCodeServiceError, fmt.Sprintf("unexpected status %d", status), RetryAfterDelay, status, nil)
	}
}

func IsRetriable(err error) bool {
	if pe, ok := err.(*ProviderError); ok {
		return pe.RetryClass == RetrySameCall || pe.RetryClass == RetryAfterDelay
	}
	return false
}

func IsNeverRetry(err error) bool {
	if pe, ok := err.(*ProviderError); ok {
		return pe.RetryClass == RetryNever
	}
	return false
}

func ErrorCodeOf(err error) string {
	if pe, ok := err.(*ProviderError); ok {
		return pe.Code
	}
	return ErrCodeServiceError
}
