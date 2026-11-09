package git

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/u-ai/backend/internal/workspace"
)

func TestGitHandlerRejectsInvalidWorkspaceURIWithBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewGitHandler(&GitController{})
	engine := gin.New()
	handler.RegisterRoutes(engine.Group("/"))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/workspaces/git/status", strings.NewReader(`{"workspaceUri":"probe"}`))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}

func TestCreateIsolatedValidationIsClassifiedAsBadRequest(t *testing.T) {
	controller := &GitController{}

	if _, err := controller.CreateIsolated(t.Context(), IsolatedCreateRequest{}); err == nil {
		t.Fatal("expected a validation error for an empty name")
	} else if !errors.Is(err, ErrIsolatedInvalidRequest) {
		t.Fatalf("empty name error = %v, want ErrIsolatedInvalidRequest", err)
	} else if got := HTTPStatus(err); got != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", got, http.StatusBadRequest)
	}

	if _, err := controller.CreateIsolated(t.Context(), IsolatedCreateRequest{Name: "qa", Mode: "nope"}); !errors.Is(err, ErrIsolatedInvalidRequest) {
		t.Fatalf("unsupported mode error = %v, want ErrIsolatedInvalidRequest", err)
	}
}

func TestGitHTTPStatusKeepsStorageFailuresAsServerErrors(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{workspace.ErrMountNotFound, http.StatusNotFound},
		{ErrBranchNotFound, http.StatusNotFound},
		{workspace.ErrInvalidURI, http.StatusBadRequest},
		{ErrGitDirty, http.StatusConflict},
		{ErrGitRemoteAuthFailed, http.StatusForbidden},
		{ErrGitRemoteNetworkFailed, http.StatusBadGateway},
		{ErrIsolatedCreateFailed, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		if got := HTTPStatus(tc.err); got != tc.want {
			t.Errorf("HTTPStatus(%v) = %d, want %d", tc.err, got, tc.want)
		}
	}
}
