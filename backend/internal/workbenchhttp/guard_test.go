package workbenchhttp

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponseHasNoCacheAndNoSniff(t *testing.T) {
	w := httptest.NewRecorder()
	ProtectResponse(w.Header())
	if got := w.Header().Get("Cache-Control"); got != "no-store, private" {
		t.Fatalf("cache directive = %q", got)
	}
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("content type directive = %q", got)
	}
	if got := w.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("pragma directive = %q", got)
	}
}

func TestRejectsDeclaredOversizeWithoutReadingBody(t *testing.T) {
	body := io.NopCloser(strings.NewReader(strings.Repeat("x", 128)))
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.Body = body
	request.ContentLength = 128
	if LimitRequest(httptest.NewRecorder(), request, 127) {
		t.Fatal("declared oversize accepted")
	}
	if request.Body != body {
		t.Fatal("declared oversize should not consume or replace the body")
	}
}

func TestRejectsUnknownLengthStreamingOversize(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(strings.Repeat("x", 65)))
	request.ContentLength = -1
	if !LimitRequest(httptest.NewRecorder(), request, 64) {
		t.Fatal("streaming request should be limited during decoding")
	}
	data, err := io.ReadAll(request.Body)
	var tooLarge *http.MaxBytesError
	if !errors.As(err, &tooLarge) || len(data) > 64 {
		t.Fatalf("oversized stream was accepted: len=%d err=%v", len(data), err)
	}
}

func TestAllowsBoundedBodyAtLimit(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 64)))
	if !LimitRequest(httptest.NewRecorder(), request, 64) {
		t.Fatal("exact limit should be accepted")
	}
	data, err := io.ReadAll(request.Body)
	if err != nil || len(data) != 64 {
		t.Fatalf("valid body failed: len=%d err=%v", len(data), err)
	}
}

func TestDoesNotConsumeGetBody(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", strings.NewReader("get-body"))
	original := request.Body
	if !LimitRequest(httptest.NewRecorder(), request, 2) || request.Body != original {
		t.Fatal("read-only requests should not be altered")
	}
}

func TestRejectsNonPositiveLimit(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	for _, limit := range []int64{0, -1} {
		if LimitRequest(httptest.NewRecorder(), request, limit) {
			t.Fatalf("invalid limit %d accepted", limit)
		}
	}
}
