package extension

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/u-ai/backend/internal/extension/kernel/hook"
)

func TestHookErrorStatusClassifiesHookFailures(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{hook.NewHookError(hook.ErrCodeHookNotFound, "get contribution: hook: contribution not found"), http.StatusNotFound},
		{hook.NewHookError(hook.ErrCodeHookPointNotFound, "point"), http.StatusNotFound},
		{hook.NewHookError(hook.ErrCodePermissionDenied, "denied"), http.StatusForbidden},
		{hook.NewHookError(hook.ErrCodeContributionDisabled, "disabled"), http.StatusConflict},
		{hook.NewHookError(hook.ErrCodeRuntimeNotReady, "not ready"), http.StatusServiceUnavailable},
		{hook.NewHookError(hook.ErrCodeHookTimeout, "timeout"), http.StatusGatewayTimeout},
		{hook.NewHookError(hook.ErrCodeHookResultInvalid, "bad result"), http.StatusInternalServerError},
		{hook.ErrContributionMissing, http.StatusNotFound},
		{fmt.Errorf("get contribution: %w", hook.ErrContributionMissing), http.StatusNotFound},
		{hook.ErrMaxHandlers, http.StatusTooManyRequests},
	}
	for _, tc := range cases {
		if got := hookErrorStatus(tc.err); got != tc.want {
			t.Errorf("hookErrorStatus(%v) = %d, want %d", tc.err, got, tc.want)
		}
	}
}

func TestWriteWorkflowAIErrorClassifiesInvalidModelOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name string
		err  error
		want int
	}{
		{"model unavailable", errWorkflowAIUnavailable, http.StatusServiceUnavailable},
		{"model output unusable", fmt.Errorf("AI could not explain workflow: %w", fmt.Errorf("%w: bad json", errWorkflowAIInvalidOutput)), http.StatusBadGateway},
		{"instruction invalid", errors.New("AI workflow instruction is required"), http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine := gin.New()
			engine.POST("/explain", func(c *gin.Context) { writeWorkflowAIError(c, tc.err) })
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/explain", nil))
			if recorder.Code != tc.want {
				t.Fatalf("status = %d, want %d, body = %s", recorder.Code, tc.want, recorder.Body.String())
			}
		})
	}
}
