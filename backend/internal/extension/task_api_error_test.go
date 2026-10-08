package extension

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

func TestTaskAPIReportsPermissionAndStateErrorsWithoutServerFailure(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		err    error
		status int
	}{
		{"provider_changed", coordination.ErrScopeExpired, http.StatusConflict},
		{"resource_changed", coordination.ErrResourceVersion, http.StatusConflict},
		{"confirmation_unknown", coordination.ErrAuthorityUnconfirmed, http.StatusConflict},
		{"wrong_owner", coordination.ErrWrongOwner, http.StatusForbidden},
		{"grant_revoked", coordination.ErrCapabilityGrant, http.StatusForbidden},
		{"missing_role", coordination.ErrRoleRequired, http.StatusConflict},
		{"multiple_roles", coordination.ErrRoleSelection, http.StatusConflict},
		{"resource_limit", coordination.ErrPendingLimit, http.StatusRequestEntityTooLarge},
		{"wrapped_not_found", fmt.Errorf("read task: %w", task_runtime.NewTaskError(task_runtime.ErrTaskNotFound, "任务不存在")), http.StatusNotFound},
		{"storage_failure", errors.New("storage unavailable"), http.StatusInternalServerError},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			writeTaskError(ctx, scenario.err)
			if recorder.Code != scenario.status || recorder.Body.Len() == 0 {
				t.Fatalf("任务接口错误分类不正确: status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestTaskArtifactResponseUsesAttachmentAndRejectsRevokedAuthority(t *testing.T) {
	for _, scenario := range []string{"valid", "unsafe_headers", "revoked"} {
		t.Run(scenario, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/task/artifact", nil)
			download := &task_runtime.TaskArtifactDownload{Content: []byte("private-artifact"), Name: "私有结果.bin", MimeType: "application/octet-stream", Hash: "confirmed-hash"}
			if scenario == "unsafe_headers" {
				download.Name, download.MimeType = "result\r\nInjected: true", "text/html\r\nInjected: true"
			}
			if scenario == "revoked" {
				ctx.Request = ctx.Request.WithContext(coordination.WithAdditionalGuard(ctx.Request.Context(), func(context.Context) error { return coordination.ErrScopeExpired }))
			}
			writeTaskArtifact(ctx, download)
			if scenario == "revoked" {
				if recorder.Code != http.StatusConflict || strings.Contains(recorder.Body.String(), "private-artifact") || recorder.Header().Get("Content-Disposition") != "" {
					t.Fatal("权限撤销后仍返回私有下载内容")
				}
				return
			}
			disposition, parameters, err := mime.ParseMediaType(recorder.Header().Get("Content-Disposition"))
			name := download.Name
			if scenario == "unsafe_headers" {
				name = "task-artifact"
			}
			if err != nil || disposition != "attachment" || parameters["filename"] != name || recorder.Header().Get("Content-Type") != "application/octet-stream" || recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("X-Content-Type-Options") != "nosniff" || recorder.Header().Get("Content-Security-Policy") == "" || recorder.Body.String() != string(download.Content) {
				t.Fatalf("下载响应的权限或浏览器隔离不正确: headers=%v err=%v", recorder.Header(), err)
			}
		})
	}
}
