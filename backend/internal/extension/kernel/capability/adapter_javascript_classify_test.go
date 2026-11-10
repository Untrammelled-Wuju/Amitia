package capability

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestClassifyJavaScriptRuntimeError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "upstream token",
			err:  errors.New("invoke error: jsonrpc: internal: host.call: upstream_error: upstream HTTP 500 Internal Server Error: 读取插件市场分类"),
			want: ErrorCodeUpstream,
		},
		{
			name: "permission token",
			err:  errors.New("invoke error: jsonrpc: internal: host.call: permission_denied: permission denied: network.request"),
			want: ErrorCodePermissionDenied,
		},
		{
			name: "scope token",
			err:  errors.New("invoke error: jsonrpc: internal: host.call: scope_denied: scope mismatch"),
			want: ErrorCodeScopeDenied,
		},
		{
			name: "timeout token",
			err:  errors.New("invoke error: jsonrpc: internal: host.call: timeout: context deadline exceeded"),
			want: ErrorCodeTimeout,
		},
		{
			name: "cancel token",
			err:  errors.New("invoke error: jsonrpc: internal: host.call: cancelled: context canceled"),
			want: ErrorCodeCancelled,
		},
		{
			name: "deadline sentinel",
			err:  fmt.Errorf("invoke: %w", context.DeadlineExceeded),
			want: ErrorCodeTimeout,
		},
		{
			name: "cancel sentinel",
			err:  context.Canceled,
			want: ErrorCodeCancelled,
		},
		{
			name: "runtime missing",
			err:  errors.New("plugin runtime not found"),
			want: ErrorCodeRuntimeUnavailable,
		},
		{
			name: "unknown",
			err:  errors.New("boom"),
			want: ErrorCodeExecutionFailed,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := classifyJavaScriptRuntimeError(testCase.err); got != testCase.want {
				t.Fatalf("classifyJavaScriptRuntimeError(%v) = %s, want %s", testCase.err, got, testCase.want)
			}
		})
	}
}

func TestErrorCategoryForUpstream(t *testing.T) {
	if got := ErrorCategoryForCode(ErrorCodeUpstream); got != ToolErrorCategoryAvailability {
		t.Fatalf("upstream category = %s, want %s", got, ToolErrorCategoryAvailability)
	}
}
