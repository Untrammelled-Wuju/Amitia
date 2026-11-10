package kernel

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
)

func TestToolFailureStatusKeepsFailureClass(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{capability.ErrorCodeTimeout, host_api.StatusTimeout},
		{capability.ErrorCodeCancelled, host_api.StatusCancelled},
		{capability.ErrorCodePermissionDenied, host_api.StatusRejected},
		{capability.ErrorCodeScopeDenied, host_api.StatusRejected},
		{capability.ErrorCodeUpstream, host_api.StatusFailed},
		{capability.ErrorCodeExecutionFailed, host_api.StatusFailed},
	}
	for _, testCase := range cases {
		if got := toolFailureStatus(testCase.code); got != testCase.want {
			t.Fatalf("toolFailureStatus(%s) = %s, want %s", testCase.code, got, testCase.want)
		}
	}
}
