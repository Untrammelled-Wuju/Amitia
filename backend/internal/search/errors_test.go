package search

import (
	"errors"
	"net/http"
	"testing"
)

func TestNewError_Basic(t *testing.T) {
	cause := errors.New("root cause")
	e := NewError(SEARCH_DISABLED, "fake", false, cause)
	if e.Code != SEARCH_DISABLED {
		t.Fatalf("code mismatch: %s", e.Code)
	}
	if e.Provider != "fake" {
		t.Fatalf("provider mismatch: %s", e.Provider)
	}
	if e.Retryable {
		t.Fatal("should not be retryable")
	}
	if e.Cause != cause {
		t.Fatal("cause mismatch")
	}
}

func TestError_ErrorString(t *testing.T) {
	e := NewError(SEARCH_DISABLED, "fake", false, errors.New("x"))
	got := e.Error()
	want := "SEARCH_DISABLED (fake): x"
	if got != want {
		t.Fatalf("error string: got %q, want %q", got, want)
	}
}

func TestError_ErrorStringNoCause(t *testing.T) {
	e := NewError(SEARCH_DISABLED, "", false, nil)
	got := e.Error()
	if got != "SEARCH_DISABLED" {
		t.Fatalf("error string: %q", got)
	}
}

func TestError_NilError(t *testing.T) {
	var e *Error
	if e.Error() != "" {
		t.Fatal("nil error should return empty string")
	}
	if e.Unwrap() != nil {
		t.Fatal("nil unwrap should return nil")
	}
	if e.Is(errors.New("x")) {
		t.Fatal("nil should not match any target")
	}
}

func TestError_Unwrap(t *testing.T) {
	cause := errors.New("root")
	e := NewError(SEARCH_DISABLED, "", false, cause)
	if !errors.Is(e, cause) {
		t.Fatal("errors.Is should traverse to cause")
	}
}

func TestError_Is_SameCode(t *testing.T) {
	e1 := NewError(SEARCH_DISABLED, "a", false, nil)
	e2 := &Error{Code: SEARCH_DISABLED}
	if !e1.Is(e2) {
		t.Fatal("should match by code")
	}
}

func TestError_Is_DifferentCode(t *testing.T) {
	e1 := NewError(SEARCH_DISABLED, "a", false, nil)
	e2 := &Error{Code: SEARCH_CANCELLED}
	if e1.Is(e2) {
		t.Fatal("should not match different code")
	}
}

func TestWrapHTTPError_Retryable(t *testing.T) {
	e := WrapHTTPError(SEARCH_PROVIDER_UNAVAILABLE, "p", http.StatusBadGateway, nil)
	if !e.Retryable {
		t.Fatal("502 should be retryable")
	}
	if e.HTTPStatus != http.StatusBadGateway {
		t.Fatal("HTTPStatus not set")
	}
}

func TestWrapHTTPError_NonRetryable(t *testing.T) {
	e := WrapHTTPError(SEARCH_PROVIDER_TIMEOUT, "p", http.StatusGatewayTimeout, nil)
	if e.Retryable {
		t.Fatal("504 should not be retryable per spec")
	}
}

func TestIsRetryableStatus(t *testing.T) {
	cases := map[int]bool{
		200: false,
		400: false,
		403: false,
		429: false,
		500: false,
		502: false,
		503: false,
		504: false,
		505: true,
	}
	for status, want := range cases {
		if got := IsRetryableStatus(status); got != want {
			t.Fatalf("status %d: got %v, want %v", status, got, want)
		}
	}
}

func TestIsAuthError(t *testing.T) {
	if !IsAuthError(401) || !IsAuthError(403) {
		t.Fatal("401/403 should be auth errors")
	}
	if IsAuthError(400) || IsAuthError(200) {
		t.Fatal("non-auth codes should not be auth errors")
	}
}

func TestIsRateLimited(t *testing.T) {
	if !IsRateLimited(429) {
		t.Fatal("429 should be rate limited")
	}
	if IsRateLimited(200) {
		t.Fatal("200 should not be rate limited")
	}
}

func TestIsClientError(t *testing.T) {
	if !IsClientError(400) || !IsClientError(404) || !IsClientError(422) {
		t.Fatal("4xx non-auth non-429 should be client errors")
	}
	if IsClientError(401) || IsClientError(403) || IsClientError(429) || IsClientError(500) {
		t.Fatal("auth/rate-limit/server should NOT be client errors")
	}
}

func TestIsServerError(t *testing.T) {
	if !IsServerError(500) || !IsServerError(502) {
		t.Fatal("5xx should be server errors")
	}
	if IsServerError(501) {
		t.Fatal("501 should NOT be server error per spec")
	}
	if IsServerError(400) || IsServerError(200) {
		t.Fatal("non-5xx should not be server errors")
	}
}
