package desktop

import (
	"net/http"
	"testing"
)

func TestHTTPStatusClassifiesContributionFailures(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{ErrContributionNotFound, http.StatusNotFound},
		{ErrContractNotFound, http.StatusNotFound},
		{ErrInvalidDefinition, http.StatusBadRequest},
		{ErrForbiddenActionType, http.StatusBadRequest},
		{ErrPermissionDenied, http.StatusForbidden},
		{ErrScopeDenied, http.StatusForbidden},
		{ErrShortcutConflict, http.StatusConflict},
		{ErrHostNotReady, http.StatusServiceUnavailable},
		{ErrCircuitOpen, http.StatusServiceUnavailable},
		{ErrSnapshotBuildFailed, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		if got := HTTPStatus(tc.err); got != tc.want {
			t.Errorf("HTTPStatus(%v) = %d, want %d", tc.err, got, tc.want)
		}
	}
}
