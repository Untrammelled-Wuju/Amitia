package execution

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const (
	defaultBaseBackoff = 100 * time.Millisecond
	defaultMaxBackoff  = 30 * time.Second
	maxBackoffExponent = 20
)

var retryableCanonicalCodes = map[string]bool{
	capability.ErrorCodeRuntimeUnavailable: true,
	capability.ErrorCodeConnectionLost:      true,
}

func isRetryableResult(result capability.UnifiedToolResult) bool {
	if result.Error == nil {
		return false
	}
	code := result.Error.Code
	if code == "" {
		code = capability.ErrorCodeExecutionFailed
		result.Error.Code = code
	}
	if !retryableCanonicalCodes[code] {
		return false
	}
	return result.Error.Retryable
}

func isRetrySafe(tool capability.ToolDefinition) bool {
	if !tool.HasSideEffects {
		return true
	}
	if tool.Idempotent {
		return true
	}
	if tool.ExecutionPolicy.Idempotent {
		return true
	}
	return false
}

func ComputeRetryBackoff(policy capability.RetryPolicy, retryIndex int) time.Duration {
	base := defaultBaseBackoff
	if policy.BackoffBase > 0 {
		base = policy.BackoffBase
	}
	exp := retryIndex - 1
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
