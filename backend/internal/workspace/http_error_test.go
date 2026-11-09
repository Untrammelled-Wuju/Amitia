package workspace

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestErrorStatusClassifiesWorkspaceFailures(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{fmt.Errorf("%w: %q", ErrMountNotFound, "probe"), http.StatusNotFound},
		{ErrWorkspaceNotFound, http.StatusNotFound},
		{ErrFileNotFound, http.StatusNotFound},
		{fmt.Errorf("%w: %v", ErrInvalidURI, "relative path"), http.StatusBadRequest},
		{ErrPathTraversal, http.StatusBadRequest},
		{ErrReadOnly, http.StatusForbidden},
		{ErrPermissionDenied, http.StatusForbidden},
		{ErrAlreadyExists, http.StatusConflict},
		{ErrOperationUnsupported, http.StatusNotImplemented},
		{ErrWorkspaceUnavailable, http.StatusServiceUnavailable},
		{errors.New("disk on fire"), http.StatusInternalServerError},
		{nil, http.StatusOK},
	}
	for _, tc := range cases {
		if got := ErrorStatus(tc.err); got != tc.want {
			t.Errorf("ErrorStatus(%v) = %d, want %d", tc.err, got, tc.want)
		}
	}
}
