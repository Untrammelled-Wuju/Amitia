package management

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/u-ai/backend/internal/gamehost/control"
	ghdomain "github.com/u-ai/backend/internal/gamehost/domain"
	ghruntime "github.com/u-ai/backend/internal/gamehost/runtime"
)

func controlEngine(t *testing.T, takeoverErr, stopErr, rearmErr error) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := NewControlHandlerWithRearm(
		func(context.Context, string) (TakeoverResult, error) { return TakeoverResult{}, takeoverErr },
		func(context.Context, string, string, uint64) (ReleaseResult, error) { return ReleaseResult{}, nil },
		func(context.Context, string) (control.EmergencyStopResult, error) {
			return control.EmergencyStopResult{}, stopErr
		},
		func(context.Context, string) error { return rearmErr },
	)
	engine := gin.New()
	RegisterGameCenterControlRouter(engine.Group("/api"), handler)
	return engine
}

func TestControlRoutesClassifyServiceErrors(t *testing.T) {
	notFound := fmt.Errorf("topology snapshot: %w",
		ghruntime.NewTopologyError(ghruntime.ErrNotFound, "runtime not found"))
	invalid := fmt.Errorf("control: %w",
		ghruntime.NewTopologyError(ghruntime.ErrInvalidArgument, "target mode invalid"))
	broken := errors.New("disk on fire")
	authorityNotFound := fmt.Errorf("emergency: %w", &control.AuthorityError{Code: ghdomain.ErrNotFound, Message: "emergency stop: runtime not found: probe"})
	authorityConflict := &control.AuthorityError{Code: ghdomain.ErrInvalidState, Message: "emergency stop: state conflict"}

	cases := []struct {
		name   string
		engine *gin.Engine
		path   string
		want   int
	}{
		{"rearm unknown runtime", controlEngine(t, nil, nil, notFound), "/api/game-center/runtimes/probe/rearm", http.StatusNotFound},
		{"emergency stop unknown runtime", controlEngine(t, nil, notFound, nil), "/api/game-center/runtimes/probe/emergency-stop", http.StatusNotFound},
		{"takeover invalid argument", controlEngine(t, invalid, nil, nil), "/api/game-center/runtimes/probe/takeover", http.StatusBadRequest},
		{"emergency stop authority not found", controlEngine(t, nil, authorityNotFound, nil), "/api/game-center/runtimes/probe/emergency-stop", http.StatusNotFound},
		{"emergency stop state conflict", controlEngine(t, nil, authorityConflict, nil), "/api/game-center/runtimes/probe/emergency-stop", http.StatusConflict},
		{"rearm unclassified failure", controlEngine(t, nil, nil, broken), "/api/game-center/runtimes/fallback/rearm", http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, tc.path, nil)
			tc.engine.ServeHTTP(recorder, request)
			if recorder.Code != tc.want {
				t.Fatalf("status = %d, want %d, body = %s", recorder.Code, tc.want, recorder.Body.String())
			}
		})
	}
}

func TestHTTPStatusKeepsUnknownFailuresAsServerErrors(t *testing.T) {
	if got := HTTPStatus(errors.New("boom")); got != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", got, http.StatusInternalServerError)
	}
}
