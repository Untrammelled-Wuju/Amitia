package search

import (
	"net/http"
)

const (
	SEARCH_DISABLED                    = "SEARCH_DISABLED"
	SEARCH_PROVIDER_NOT_CONFIGURED     = "SEARCH_PROVIDER_NOT_CONFIGURED"
	SEARCH_PROVIDER_UNAVAILABLE        = "SEARCH_PROVIDER_UNAVAILABLE"
	SEARCH_PROVIDER_AUTH_FAILED        = "SEARCH_PROVIDER_AUTH_FAILED"
	SEARCH_PROVIDER_RATE_LIMITED       = "SEARCH_PROVIDER_RATE_LIMITED"
	SEARCH_PROVIDER_TIMEOUT            = "SEARCH_PROVIDER_TIMEOUT"
	SEARCH_PROVIDER_REQUEST_FAILED     = "SEARCH_PROVIDER_REQUEST_FAILED"
	SEARCH_PROVIDER_REQUEST_REJECTED   = "SEARCH_PROVIDER_REQUEST_REJECTED"
	SEARCH_PROVIDER_INVALID_RESPONSE   = "SEARCH_PROVIDER_INVALID_RESPONSE"
	SEARCH_PROVIDER_RESPONSE_TOO_LARGE = "SEARCH_PROVIDER_RESPONSE_TOO_LARGE"
	SEARCH_INVALID_QUERY               = "SEARCH_INVALID_QUERY"
	SEARCH_INVALID_LIMIT               = "SEARCH_INVALID_LIMIT"
	SEARCH_INVALID_OFFSET              = "SEARCH_INVALID_OFFSET"
	SEARCH_INVALID_LANGUAGE            = "SEARCH_INVALID_LANGUAGE"
	SEARCH_INVALID_COUNTRY             = "SEARCH_INVALID_COUNTRY"
	SEARCH_INVALID_SAFE_SEARCH         = "SEARCH_INVALID_SAFE_SEARCH"
	SEARCH_FILTER_UNSUPPORTED          = "SEARCH_FILTER_UNSUPPORTED"
	SEARCH_RESULT_URL_INVALID          = "SEARCH_RESULT_URL_INVALID"
	SEARCH_CANCELLED                   = "SEARCH_CANCELLED"
	SEARCH_BLOCKED_BY_NETWORK          = "SEARCH_BLOCKED_BY_NETWORK"
)

type Error struct {
	Code       string        `json:"-"`
	Provider   string        `json:"-"`
	HTTPStatus int           `json:"-"`
	RetryAfter DurationMs    `json:"retryAfterMs,omitempty"`
	Retryable  bool          `json:"retryable"`
	Cause      error         `json:"-"`
}

type DurationMs int64

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	msg := e.Code
	if e.Provider != "" {
		msg = msg + " (" + e.Provider + ")"
	}
	if e.Cause != nil {
		msg = msg + ": " + e.Cause.Error()
	}
	return msg
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *Error) Is(target error) bool {
	if e == nil || target == nil {
		return false
	}
	if e == target {
		return true
	}
	if t, ok := target.(*Error); ok {
		return t.Code == e.Code
	}
	return false
}

func NewError(code, provider string, retryable bool, cause error) *Error {
	return &Error{
		Code:      code,
		Provider:  provider,
		Retryable: retryable,
		Cause:     cause,
	}
}

func WrapHTTPError(code, provider string, status int, cause error) *Error {
	var retryable bool
	switch {
	case status >= 500 && status != 501:
		retryable = true
	}
	return &Error{
		Code:       code,
		Provider:   provider,
		HTTPStatus: status,
		Retryable:  retryable,
		Cause:      cause,
	}
}

func IsRetryableStatus(status int) bool {
	switch {
	case http.StatusServiceUnavailable == status,
		http.StatusBadGateway == status,
		http.StatusGatewayTimeout == status,
		http.StatusInternalServerError == status,
		http.StatusTooManyRequests == status:
		return false
	case status >= 500:
		return true
	}
	return false
}

func IsAuthError(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden
}

func IsRateLimited(status int) bool {
	return status == http.StatusTooManyRequests
}

func IsClientError(status int) bool {
	return status >= 400 && status < 500 && !IsAuthError(status) && !IsRateLimited(status)
}

func IsServerError(status int) bool {
	return status >= 500 && status != 501
}

