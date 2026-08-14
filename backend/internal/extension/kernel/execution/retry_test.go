package execution

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestRetryController_SuccessNoRetry(t *testing.T) {
	ctrl := NewRetryController()
	decision := ctrl.Decide(context.Background(), RetryDecisionInput{
		Tool: newTestTool("test/tool"),
		Result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusSuccess,
		},
		AttemptNumber: 1,
	})
	if decision.Retry {
		t.Fatalf("success should not retry, got reason=%s", decision.Reason)
	}
}

func TestRetryController_MaxRetriesZero(t *testing.T) {
	ctrl := NewRetryController()
	tool := newTestTool("test/tool")
	tool.ExecutionPolicy.RetryPolicy.MaxRetries = 0
	decision := ctrl.Decide(context.Background(), RetryDecisionInput{
		Tool: tool,
		Result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:      capability.ErrorCodeConnectionLost,
				Retryable: true,
			},
		},
		AttemptNumber: 1,
	})
	if decision.Retry {
		t.Fatal("MaxRetries=0 should not retry")
	}
	if decision.Reason != RetryReasonNoBudget {
		t.Fatalf("expected no_retry_budget, got %s", decision.Reason)
	}
}

func TestRetryController_MaxRetriesExactSemantics(t *testing.T) {
	ctrl := NewRetryController()
	tool := newTestTool("test/tool")
	tool.HasSideEffects = false
	tool.ExecutionPolicy.RetryPolicy.MaxRetries = 2

	for i := 0; i < 2; i++ {
		decision := ctrl.Decide(context.Background(), RetryDecisionInput{
			Tool: tool,
			Result: capability.UnifiedToolResult{
				Status: capability.ToolResultStatusFailed,
				Error: &capability.ToolError{
					Code:      capability.ErrorCodeConnectionLost,
					Retryable: true,
				},
			},
			RetryIndex:      i,
			AttemptNumber:   i + 1,
			RemainingBudget: 10 * time.Second,
		})
		if !decision.Retry {
			t.Fatalf("iteration %d: expected retry, got reason=%s", i, decision.Reason)
		}
		if decision.NextAttemptNumber != i+2 {
			t.Fatalf("iteration %d: expected next_attempt=%d, got %d", i, i+2, decision.NextAttemptNumber)
		}
	}

	decision := ctrl.Decide(context.Background(), RetryDecisionInput{
		Tool: tool,
		Result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:      capability.ErrorCodeConnectionLost,
				Retryable: true,
			},
		},
		RetryIndex:      2,
		AttemptNumber:   3,
		RemainingBudget: 10 * time.Second,
	})
	if decision.Retry {
		t.Fatal("should not retry after MaxRetries exhausted")
	}
	if decision.Reason != RetryReasonBudgetExhausted {
		t.Fatalf("expected retry_budget_exhausted, got %s", decision.Reason)
	}
}

func TestRetryController_ConnectionLostRetries(t *testing.T) {
	ctrl := NewRetryController()
	tool := newTestTool("test/tool")
	tool.HasSideEffects = false

	decision := ctrl.Decide(context.Background(), RetryDecisionInput{
		Tool: tool,
		Result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:      capability.ErrorCodeConnectionLost,
				Retryable: true,
			},
		},
		RetryIndex:      0,
		AttemptNumber:   1,
		RemainingBudget: 10 * time.Second,
	})
	if !decision.Retry {
		t.Fatalf("connection_lost with Retryable=true should retry, got reason=%s", decision.Reason)
	}
	if decision.Reason != RetryReasonRetryableRuntimeFailure {
		t.Fatalf("expected retryable_runtime_failure, got %s", decision.Reason)
	}
}

func TestRetryController_RuntimeUnavailableRetries(t *testing.T) {
	ctrl := NewRetryController()
	tool := newTestTool("test/tool")
	tool.HasSideEffects = false

	decision := ctrl.Decide(context.Background(), RetryDecisionInput{
		Tool: tool,
		Result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:      capability.ErrorCodeRuntimeUnavailable,
				Retryable: true,
			},
		},
		RetryIndex:      0,
		AttemptNumber:   1,
		RemainingBudget: 10 * time.Second,
	})
	if !decision.Retry {
		t.Fatal("runtime_unavailable should retry")
	}
}

func TestRetryController_UnknownErrorNotRetried(t *testing.T) {
	ctrl := NewRetryController()
	tool := newTestTool("test/tool")
	tool.Retryable = true
	tool.HasSideEffects = false

	decision := ctrl.Decide(context.Background(), RetryDecisionInput{
		Tool: tool,
		Result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:      "provider_weird_error",
				Retryable: true,
			},
		},
		RetryIndex:      0,
		AttemptNumber:   1,
		RemainingBudget: 10 * time.Second,
	})
	if decision.Retry {
		t.Fatal("unknown error should NOT retry even with tool.Retryable=true and Retryable=true")
	}
}

func TestRetryController_PermissionDeniedNoRetry(t *testing.T) {
	ctrl := NewRetryController()
	tool := newTestTool("test/tool")
	tool.Retryable = true

	decision := ctrl.Decide(context.Background(), RetryDecisionInput{
		Tool: tool,
		Result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:      capability.ErrorCodePermissionDenied,
				Retryable: true,
			},
		},
		RetryIndex:    0,
		AttemptNumber: 1,
	})
	if decision.Retry {
		t.Fatal("permission_denied should not retry")
	}
}

func TestRetryController_ScopeDeniedNoRetry(t *testing.T) {
	ctrl := NewRetryController()
	tool := newTestTool("test/tool")

	decision := ctrl.Decide(context.Background(), RetryDecisionInput{
		Tool: tool,
		Result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code: capability.ErrorCodeScopeDenied,
			},
		},
		RetryIndex:    0,
		AttemptNumber: 1,
	})
	if decision.Retry {
		t.Fatal("scope_denied should not retry")
	}
}

func TestRetryController_TimeoutNoRetry(t *testing.T) {
	ctrl := NewRetryController()
	tool := newTestTool("test/tool")

	decision := ctrl.Decide(context.Background(), RetryDecisionInput{
		Tool: tool,
		Result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code: capability.ErrorCodeTimeout,
			},
		},
		RetryIndex:    0,
		AttemptNumber: 1,
	})
	if decision.Retry {
		t.Fatal("timeout should not retry")
	}
}

func TestRetryController_CancelledNoRetry(t *testing.T) {
	ctrl := NewRetryController()
	tool := newTestTool("test/tool")

	decision := ctrl.Decide(context.Background(), RetryDecisionInput{
		Tool: tool,
		Result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code: capability.ErrorCodeCancelled,
			},
		},
		RetryIndex:    0,
		AttemptNumber: 1,
	})
	if decision.Retry {
		t.Fatal("cancelled should not retry")
	}
}

func TestRetryController_RateLimitedNoRetry(t *testing.T) {
	ctrl := NewRetryController()
	tool := newTestTool("test/tool")

	decision := ctrl.Decide(context.Background(), RetryDecisionInput{
		Tool: tool,
		Result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code: capability.ErrorCodeRateLimited,
			},
		},
		RetryIndex:    0,
		AttemptNumber: 1,
	})
	if decision.Retry {
		t.Fatal("rate_limited should not retry")
	}
}

func TestRetryController_ResourceErrorsNoRetry(t *testing.T) {
	ctrl := NewRetryController()
	codes := []string{
		capability.ErrorCodeResourceLimitInvalid,
		capability.ErrorCodeResourceLimitUnavailable,
		capability.ErrorCodeResourceLimitExceeded,
		capability.ErrorCodeResourceUsageUnavailable,
	}
	for _, code := range codes {
		tool := newTestTool("test/tool")
		tool.HasSideEffects = false
		decision := ctrl.Decide(context.Background(), RetryDecisionInput{
			Tool: tool,
			Result: capability.UnifiedToolResult{
				Status: capability.ToolResultStatusFailed,
				Error: &capability.ToolError{
					Code:      code,
					Retryable: true,
				},
			},
			RetryIndex:    0,
			AttemptNumber: 1,
		})
		if decision.Retry {
			t.Fatalf("%s should not retry", code)
		}
	}
}

func TestRetryController_StreamErrorsNoRetry(t *testing.T) {
	ctrl := NewRetryController()
	codes := []string{
		capability.ErrorCodeStreamDeliveryFailed,
		capability.ErrorCodeStreamProtocol,
		capability.ErrorCodeStreamLimitExceeded,
	}
	for _, code := range codes {
		tool := newTestTool("test/tool")
		decision := ctrl.Decide(context.Background(), RetryDecisionInput{
			Tool: tool,
			Result: capability.UnifiedToolResult{
				Status: capability.ToolResultStatusFailed,
				Error: &capability.ToolError{
					Code: code,
				},
			},
			RetryIndex:    0,
			AttemptNumber: 1,
		})
		if decision.Retry {
			t.Fatalf("%s should not retry", code)
		}
	}
}

func TestRetryController_NonSideEffectToolAllowsRetry(t *testing.T) {
	ctrl := NewRetryController()
	tool := newTestTool("test/tool")
	tool.HasSideEffects = false

	decision := ctrl.Decide(context.Background(), RetryDecisionInput{
		Tool: tool,
		Result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:      capability.ErrorCodeConnectionLost,
				Retryable: true,
			},
		},
		RetryIndex:    0,
		AttemptNumber: 1,
	})
	if !decision.Retry {
		t.Fatal("non-side-effect tool should allow retry")
	}
}

func TestRetryController_IdempotentSideEffectAllowsRetry(t *testing.T) {
	ctrl := NewRetryController()
	tool := newTestTool("test/tool")
	tool.HasSideEffects = true
	tool.Idempotent = true

	decision := ctrl.Decide(context.Background(), RetryDecisionInput{
		Tool: tool,
		Result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:      capability.ErrorCodeConnectionLost,
				Retryable: true,
			},
		},
		RetryIndex:    0,
		AttemptNumber: 1,
	})
	if !decision.Retry {
		t.Fatal("idempotent side-effect tool should allow retry")
	}
}

func TestRetryController_ExecutionIdempotentAllowsRetry(t *testing.T) {
	ctrl := NewRetryController()
	tool := newTestTool("test/tool")
	tool.HasSideEffects = true
	tool.Idempotent = false
	tool.ExecutionPolicy.Idempotent = true

	decision := ctrl.Decide(context.Background(), RetryDecisionInput{
		Tool: tool,
		Result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:      capability.ErrorCodeConnectionLost,
				Retryable: true,
			},
		},
		RetryIndex:    0,
		AttemptNumber: 1,
	})
	if !decision.Retry {
		t.Fatal("execution-policy idempotent tool should allow retry")
	}
}

func TestRetryController_NonIdempotentSideEffectBlocksRetry(t *testing.T) {
	ctrl := NewRetryController()
	tool := newTestTool("test/tool")
	tool.HasSideEffects = true
	tool.Idempotent = false
	tool.ExecutionPolicy.Idempotent = false

	decision := ctrl.Decide(context.Background(), RetryDecisionInput{
		Tool: tool,
		Result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:      capability.ErrorCodeConnectionLost,
				Retryable: true,
			},
		},
		RetryIndex:    0,
		AttemptNumber: 1,
	})
	if decision.Retry {
		t.Fatal("non-idempotent side-effect tool should block retry")
	}
	if decision.Reason != RetryReasonUnsafeSideEffect {
		t.Fatalf("expected unsafe_side_effect, got %s", decision.Reason)
	}
}

func TestRetryController_StreamVisibleBlocksRetry(t *testing.T) {
	ctrl := NewRetryController()
	tool := newTestTool("test/tool")
	tool.HasSideEffects = false

	decision := ctrl.Decide(context.Background(), RetryDecisionInput{
		Tool: tool,
		Result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:      capability.ErrorCodeConnectionLost,
				Retryable: true,
			},
		},
		RetryIndex:    0,
		AttemptNumber: 1,
		StreamVisible: true,
	})
	if decision.Retry {
		t.Fatal("visible stream output should block retry")
	}
	if decision.Reason != RetryReasonStreamVisible {
		t.Fatalf("expected stream_visible, got %s", decision.Reason)
	}
}

func TestRetryController_StreamFailureBlocksRetry(t *testing.T) {
	ctrl := NewRetryController()
	tool := newTestTool("test/tool")

	decision := ctrl.Decide(context.Background(), RetryDecisionInput{
		Tool: tool,
		Result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:      capability.ErrorCodeConnectionLost,
				Retryable: true,
			},
		},
		RetryIndex:    0,
		AttemptNumber: 1,
		StreamFailed:  true,
	})
	if decision.Retry {
		t.Fatal("stream failure should block retry")
	}
	if decision.Reason != RetryReasonStreamFailure {
		t.Fatalf("expected stream_failure, got %s", decision.Reason)
	}
}

func TestRetryController_DeadlineInsufficient(t *testing.T) {
	ctrl := NewRetryController()
	tool := newTestTool("test/tool")
	tool.HasSideEffects = false
	tool.ExecutionPolicy.RetryPolicy.MaxRetries = 10
	tool.ExecutionPolicy.RetryPolicy.BackoffBase = 100 * time.Millisecond

	decision := ctrl.Decide(context.Background(), RetryDecisionInput{
		Tool: tool,
		Result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:      capability.ErrorCodeConnectionLost,
				Retryable: true,
			},
		},
		RetryIndex:      5,
		AttemptNumber:   6,
		RemainingBudget: 50 * time.Millisecond,
	})
	if decision.Retry {
		t.Fatal("insufficient deadline should block retry")
	}
	if decision.Reason != RetryReasonDeadlineInsufficient {
		t.Fatalf("expected deadline_budget_insufficient, got %s", decision.Reason)
	}
}

func TestRetryController_ContextCancelled(t *testing.T) {
	ctrl := NewRetryController()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	decision := ctrl.Decide(ctx, RetryDecisionInput{
		Tool:          newTestTool("test/tool"),
		Result:        capability.UnifiedToolResult{Status: capability.ToolResultStatusFailed},
		RetryIndex:    0,
		AttemptNumber: 1,
	})
	if decision.Retry {
		t.Fatal("cancelled context should not retry")
	}
	if decision.Reason != RetryReasonCancelled {
		t.Fatalf("expected cancelled, got %s", decision.Reason)
	}
}

func TestRetryController_ContextDeadline(t *testing.T) {
	ctrl := NewRetryController()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond)

	decision := ctrl.Decide(ctx, RetryDecisionInput{
		Tool:          newTestTool("test/tool"),
		Result:        capability.UnifiedToolResult{Status: capability.ToolResultStatusFailed},
		RetryIndex:    0,
		AttemptNumber: 1,
	})
	if decision.Retry {
		t.Fatal("deadline ctx should not retry")
	}
	if decision.Reason != RetryReasonTimedOut {
		t.Fatalf("expected timed_out, got %s", decision.Reason)
	}
}

func TestComputeRetryBackoff_Exponential(t *testing.T) {
	policy := capability.RetryPolicy{
		BackoffBase: 100 * time.Millisecond,
	}
	d1 := ComputeRetryBackoff(policy, 1)
	d2 := ComputeRetryBackoff(policy, 2)
	d3 := ComputeRetryBackoff(policy, 3)

	if d1 != 100*time.Millisecond {
		t.Fatalf("retry 1: expected 100ms, got %v", d1)
	}
	if d2 != 200*time.Millisecond {
		t.Fatalf("retry 2: expected 200ms, got %v", d2)
	}
	if d3 != 400*time.Millisecond {
		t.Fatalf("retry 3: expected 400ms, got %v", d3)
	}
}

func TestComputeRetryBackoff_Cap(t *testing.T) {
	policy := capability.RetryPolicy{
		BackoffBase: 1 * time.Second,
	}
	d := ComputeRetryBackoff(policy, 100)
	if d > defaultMaxBackoff {
		t.Fatalf("backoff should cap at %v, got %v", defaultMaxBackoff, d)
	}
}

func TestRetryableResult_EmptyCodeBecomesExecutionFailed(t *testing.T) {
	result := capability.UnifiedToolResult{
		Status: capability.ToolResultStatusFailed,
		Error: &capability.ToolError{
			Code:      "",
			Retryable: true,
		},
	}
	if isRetryableResult(result) {
		t.Fatal("empty code normalizes to execution_failed which is not in allowlist")
	}
}

func TestRetryableResult_CanonicalAllowlistOnly(t *testing.T) {
	cases := map[string]bool{
		capability.ErrorCodeRuntimeUnavailable: true,
		capability.ErrorCodeConnectionLost:     true,
		capability.ErrorCodeExecutionFailed:    false,
		capability.ErrorCodePermissionDenied:   false,
		capability.ErrorCodeTimeout:            false,
		"custom_provider_error":                false,
	}
	for code, expected := range cases {
		result := capability.UnifiedToolResult{
			Status: capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:      code,
				Retryable: true,
			},
		}
		if got := isRetryableResult(result); got != expected {
			t.Fatalf("code %s: expected isRetryable=%v, got %v", code, expected, got)
		}
	}
}

func TestRetryableResult_RetryableFalse_Rejected(t *testing.T) {
	result := capability.UnifiedToolResult{
		Status: capability.ToolResultStatusFailed,
		Error: &capability.ToolError{
			Code:      capability.ErrorCodeConnectionLost,
			Retryable: false,
		},
	}
	if isRetryableResult(result) {
		t.Fatal("Retryable=false should reject even for allowlist code")
	}
}

func TestIsRetrySafe_NoSideEffects(t *testing.T) {
	tool := newTestTool("test/tool")
	tool.HasSideEffects = false
	if !isRetrySafe(tool) {
		t.Fatal("no side effects should be safe")
	}
}

func TestIsRetrySafe_IdempotentTool(t *testing.T) {
	tool := newTestTool("test/tool")
	tool.HasSideEffects = true
	tool.Idempotent = true
	if !isRetrySafe(tool) {
		t.Fatal("idempotent tool should be safe")
	}
}

func TestIsRetrySafe_IdempotentPolicy(t *testing.T) {
	tool := newTestTool("test/tool")
	tool.HasSideEffects = true
	tool.Idempotent = false
	tool.ExecutionPolicy.Idempotent = true
	if !isRetrySafe(tool) {
		t.Fatal("idempotent policy should be safe")
	}
}

func TestIsRetrySafe_NonIdempotentDangerous(t *testing.T) {
	tool := newTestTool("test/tool")
	tool.HasSideEffects = true
	tool.Idempotent = false
	tool.ExecutionPolicy.Idempotent = false
	if isRetrySafe(tool) {
		t.Fatal("non-idempotent side-effect tool should NOT be safe")
	}
}

func TestRetryController_Deterministic(t *testing.T) {
	ctrl := NewRetryController()
	tool := newTestTool("test/tool")
	tool.HasSideEffects = false
	input := RetryDecisionInput{
		Tool: tool,
		Result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:      capability.ErrorCodeConnectionLost,
				Retryable: true,
			},
		},
		RetryIndex:      0,
		AttemptNumber:   1,
		RemainingBudget: 10 * time.Second,
	}
	d1 := ctrl.Decide(context.Background(), input)
	d2 := ctrl.Decide(context.Background(), input)
	if d1.Retry != d2.Retry || d1.Reason != d2.Reason || d1.Delay != d2.Delay {
		t.Fatalf("decisions should be deterministic: %+v vs %+v", d1, d2)
	}
}

func TestRetryController_NilErrorNotRetried(t *testing.T) {
	ctrl := NewRetryController()
	tool := newTestTool("test/tool")
	decision := ctrl.Decide(context.Background(), RetryDecisionInput{
		Tool: tool,
		Result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusFailed,
		},
		RetryIndex:    0,
		AttemptNumber: 1,
	})
	if decision.Retry {
		t.Fatal("nil error should not retry")
	}
}

func TestRespectBackoff_ZeroDuration(t *testing.T) {
	if RespectBackoff(context.Background(), 0) {
		t.Fatal("zero duration should return false (no wait)")
	}
}

func TestRespectBackoff_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !RespectBackoff(ctx, 10*time.Second) {
		t.Fatal("cancelled context should interrupt backoff (return interrupted=true)")
	}
}

func TestControllerStateless_ConcurrentSafety(t *testing.T) {
	ctrl := NewRetryController()
	tool := newTestTool("test/tool")
	tool.HasSideEffects = false
	tool.ExecutionPolicy.RetryPolicy.MaxRetries = 2

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			decision := ctrl.Decide(context.Background(), RetryDecisionInput{
				Tool: tool,
				Result: capability.UnifiedToolResult{
					Status: capability.ToolResultStatusFailed,
					Error: &capability.ToolError{
						Code:      capability.ErrorCodeConnectionLost,
						Retryable: true,
					},
				},
				RetryIndex:      idx % 3,
				AttemptNumber:   idx%3 + 1,
				RemainingBudget: 10 * time.Second,
			})
			if idx%3 < 2 && !decision.Retry {
				t.Errorf("goroutine %d: expected retry at idx=%d", idx, idx%3)
			}
			if idx%3 >= 2 && decision.Retry {
				t.Errorf("goroutine %d: expected no retry (budget exhausted) at idx=%d", idx, idx%3)
			}
		}(i)
	}
	wg.Wait()
}

type fakeRetryController struct {
	decisions []RetryDecisionResult
	calls     int
}

func (f *fakeRetryController) Decide(ctx context.Context, input RetryDecisionInput) RetryDecisionResult {
	idx := f.calls
	f.calls++
	if idx < len(f.decisions) {
		return f.decisions[idx]
	}
	return RetryDecisionResult{Retry: false, Reason: RetryReasonNoBudget}
}
