package native

import (
	"net/http"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/search"
)

func TestMapHTTPErrorPreservesRetryAfter(t *testing.T) {
	headers := make(http.Header)
	headers.Set("Retry-After", "5")
	err := mapHTTPError("engine", http.StatusTooManyRequests, headers, time.Unix(0, 0))
	searchErr, ok := err.(*search.Error)
	if !ok {
		t.Fatalf("expected *search.Error, got %T", err)
	}
	if searchErr.Code != search.SEARCH_PROVIDER_RATE_LIMITED || searchErr.RetryAfter != search.DurationMs(5000) {
		t.Fatalf("unexpected rate limit error: %+v", searchErr)
	}
}
