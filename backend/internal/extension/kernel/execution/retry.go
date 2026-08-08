package execution

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const (
	defaultBaseBackoff = 100 * time.Millisecond
	defaultMaxBackoff  = 30 * time.Second
	maxBackoffExponent = 20
)

var nonRetryableCodes = map[string]bool{
	capability.ErrorCodeInvalidInput:     true,
	capability.ErrorCodePermissionDenied: true,
	capability.ErrorCodeScopeDenied:      true,
	capability.ErrorCodeNotAvailable:     true,
	capability.ErrorCodeCancelled:        true,
	capability.ErrorCodeTimeout:          true,
	capability.ErrorCodeConflict:         true,
	capability.ErrorCodeInvalidResult:    true,
	capability.ErrorCodeRateLimited:      true,
	capability.ErrorCodeInternalError:    true,
	capability.ErrorCodeDependencyMissing: true,
}

func isRetryableCode(code string) bool {
	return !nonRetryableCodes[code]
}

func hasToolBudget(tool capability.ToolDefinition) bool {
	return tool.ExecutionPolicy.RetryPolicy.MaxRetries > 0
}

func isDeadlineExhausted(inv capability.ToolInvocationContext) bool {
	if inv.ExpiresAt.IsZero() {
		return false
	}
	return !time.Now().Before(inv.ExpiresAt)
}

func backoffDelay(tool capability.ToolDefinition, attempt int) time.Duration {
	base := defaultBaseBackoff
	if tool.ExecutionPolicy.RetryPolicy.BackoffBase > 0 {
		base = tool.ExecutionPolicy.RetryPolicy.BackoffBase
	}
	exp := attempt - 1
	if exp < 0 {
		exp = 0
	}
	if exp > maxBackoffExponent {
		exp = maxBackoffExponent
	}
	delay := base << uint(exp)
	if delay <= 0 || delay > defaultMaxBackoff {
		delay = defaultMaxBackoff
	}
	return delay
}

func isFinalStatus(status capability.ToolResultStatus) bool {
	return status == capability.ToolResultStatusSuccess ||
		status == capability.ToolResultStatusCancelled ||
		status == capability.ToolResultStatusTimedOut
}

func RetryDecision(tool capability.ToolDefinition, inv capability.ToolInvocationContext, retErr *capability.ToolError, attempt int) (retry bool, delay time.Duration, reason string) {
	if !hasToolBudget(tool) {
		return false, 0, "no budget"
	}
	if attempt >= tool.ExecutionPolicy.RetryPolicy.MaxRetries {
		return false, 0, "budget exhausted"
	}
	if isDeadlineExhausted(inv) {
		return false, 0, "deadline exhausted"
	}
	if retErr == nil {
		return false, 0, "no error"
	}
	if !isRetryableCode(retErr.Code) {
		return false, 0, fmt.Sprintf("non-retryable: %s", retErr.Code)
	}
	if retErr.Retryable || tool.Retryable {
		return true, backoffDelay(tool, attempt+1), "explicitly retryable"
	}
	return false, 0, "not retryable"
}

func RespectBackoff(ctx context.Context, d time.Duration) (interrupted bool) {
	if d <= 0 {
		return false
	}
	if err := ctx.Err(); err != nil {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return true
	case <-timer.C:
		return false
	}
}
