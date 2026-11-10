package ui_contribution_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
)

func TestClassifyFailureKeepsOriginalFailureClass(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want ui_contribution.UIErrorCode
	}{
		{
			name: "upstream",
			err:  fmt.Errorf("action command failed: %w", &host_api.Error{Code: host_api.ErrorCodeUpstream, Message: "upstream HTTP 500 Internal Server Error: 读取插件市场分类: dial tcp 127.0.0.1:3306"}),
			want: ui_contribution.UIErrUpstream,
		},
		{
			name: "permission",
			err:  fmt.Errorf("action command failed: %w", &host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: "permission denied: network.request"}),
			want: ui_contribution.UIErrPermissionDenied,
		},
		{
			name: "scope",
			err:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "scope mismatch"},
			want: ui_contribution.UIErrScopeDenied,
		},
		{
			name: "timeout coded",
			err:  &host_api.Error{Code: host_api.ErrorCodeTimeout, Message: "context deadline exceeded"},
			want: ui_contribution.UIErrTimeout,
		},
		{
			name: "cancelled coded",
			err:  &host_api.Error{Code: host_api.ErrorCodeCancelled, Message: "context canceled"},
			want: ui_contribution.UIErrCancelled,
		},
		{
			name: "deadline sentinel",
			err:  fmt.Errorf("host.network.request: %w", context.DeadlineExceeded),
			want: ui_contribution.UIErrTimeout,
		},
		{
			name: "cancel sentinel",
			err:  context.Canceled,
			want: ui_contribution.UIErrCancelled,
		},
		{
			name: "plugin host text",
			err:  errors.New("invoke error: jsonrpc: internal: host.call: upstream_error: upstream HTTP 500 Internal Server Error: 读取插件市场分类"),
			want: ui_contribution.UIErrUpstream,
		},
		{
			name: "unknown",
			err:  errors.New("boom"),
			want: ui_contribution.UIErrRuntimeUnavailable,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := ui_contribution.ClassifyFailure(testCase.err); got != testCase.want {
				t.Fatalf("ClassifyFailure(%v) = %s, want %s", testCase.err, got, testCase.want)
			}
		})
	}
}

func TestClassifyFailureKeepsFullOriginalText(t *testing.T) {
	original := `upstream HTTP 500 Internal Server Error: {"code":500,"msg":"读取插件市场分类: dial tcp 127.0.0.1:3306: connect: connection refused"}`
	err := fmt.Errorf("action command failed: %w", &host_api.Error{Code: host_api.ErrorCodeUpstream, Message: original})
	uiError := ui_contribution.NewUIError(ui_contribution.ClassifyFailure(err), err.Error(), nil)
	if uiError.Code != ui_contribution.UIErrUpstream {
		t.Fatalf("unexpected code: %s", uiError.Code)
	}
	if !strings.Contains(uiError.Message, "读取插件市场分类") || !strings.Contains(uiError.Message, "connection refused") {
		t.Fatalf("original error text lost: %s", uiError.Message)
	}
}
