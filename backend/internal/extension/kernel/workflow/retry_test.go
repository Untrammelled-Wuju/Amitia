package workflow

import (
	"testing"
	"time"
)

func TestDefaultRetryPolicy(t *testing.T) {
	p := DefaultRetryPolicy()
	if p.MaxAttempts != 1 {
		t.Errorf("MaxAttempts = %d, want 1", p.MaxAttempts)
	}
	if p.InitialBackoff != 200*time.Millisecond {
		t.Errorf("InitialBackoff = %v, want 200ms", p.InitialBackoff)
	}
	if p.MaxBackoff != 30*time.Second {
		t.Errorf("MaxBackoff = %v, want 30s", p.MaxBackoff)
	}
	if p.Multiplier != 2.0 {
		t.Errorf("Multiplier = %f, want 2.0", p.Multiplier)
	}
}

func TestRetryPolicyNormalize(t *testing.T) {
	t.Run("nil policy returns default", func(t *testing.T) {
		p := (*WorkflowRetryPolicy)(nil).Normalize()
		if p == nil {
			t.Fatal("expected non-nil normalized policy")
		}
		if p.MaxAttempts != 1 {
			t.Errorf("MaxAttempts = %d, want 1", p.MaxAttempts)
		}
	})

	t.Run("invalid values corrected", func(t *testing.T) {
		p := (&WorkflowRetryPolicy{
			MaxAttempts:    -1,
			InitialBackoff: 0,
			MaxBackoff:     0,
			Multiplier:     0.5,
			Jitter:         2.0,
		}).Normalize()
		if p.MaxAttempts != 1 {
			t.Errorf("MaxAttempts = %d, want 1", p.MaxAttempts)
		}
		if p.InitialBackoff != 200*time.Millisecond {
			t.Errorf("InitialBackoff = %v, want 200ms", p.InitialBackoff)
		}
		if p.MaxBackoff != 30*time.Second {
			t.Errorf("MaxBackoff = %v, want 30s", p.MaxBackoff)
		}
		if p.Multiplier != 2.0 {
			t.Errorf("Multiplier = %f, want 2.0", p.Multiplier)
		}
		if p.Jitter != 1.0 {
			t.Errorf("Jitter = %f, want 1.0", p.Jitter)
		}
	})
}

func TestRetryPolicyComputeBackoff(t *testing.T) {
	p := &WorkflowRetryPolicy{
		MaxAttempts:    5,
		InitialBackoff: 200 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		Multiplier:     2.0,
		Jitter:         0,
	}

	backoff0 := p.ComputeBackoff(0)
	if backoff0 != 0 {
		t.Errorf("backoff(0) = %v, want 0", backoff0)
	}

	backoff1 := p.ComputeBackoff(1)
	if backoff1 != 200*time.Millisecond {
		t.Errorf("backoff(1) = %v, want 200ms", backoff1)
	}

	backoff2 := p.ComputeBackoff(2)
	if backoff2 != 400*time.Millisecond {
		t.Errorf("backoff(2) = %v, want 400ms", backoff2)
	}

	backoff3 := p.ComputeBackoff(3)
	if backoff3 != 800*time.Millisecond {
		t.Errorf("backoff(3) = %v, want 800ms", backoff3)
	}

	pJitter := &WorkflowRetryPolicy{
		MaxAttempts:    5,
		InitialBackoff: 200 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		Multiplier:     2.0,
		Jitter:         0.2,
	}

	backoffJitter := pJitter.ComputeBackoff(2)
	if backoffJitter < 300*time.Millisecond || backoffJitter > 500*time.Millisecond {
		t.Errorf("backoff with jitter(2) = %v, want ~320-480ms range", backoffJitter)
	}
}

func TestRetryPolicyMaxBackoffCap(t *testing.T) {
	p := &WorkflowRetryPolicy{
		MaxAttempts:    10,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     5 * time.Second,
		Multiplier:     2.0,
		Jitter:         0,
	}

	backoff10 := p.ComputeBackoff(10)
	if backoff10 > 5*time.Second {
		t.Errorf("backoff(10) = %v, should be capped at 5s", backoff10)
	}
}

func TestRetryableErrorCodes(t *testing.T) {
	t.Run("empty list allows all", func(t *testing.T) {
		p := DefaultRetryPolicy()
		if !p.IsRetryable("any_error") {
			t.Error("expected all errors to be retryable with empty list")
		}
	})

	t.Run("specific codes", func(t *testing.T) {
		p := &WorkflowRetryPolicy{
			RetryableErrorCodes: []string{"timeout", "rate_limit"},
		}
		if !p.IsRetryable("timeout") {
			t.Error("timeout should be retryable")
		}
		if !p.IsRetryable("rate_limit") {
			t.Error("rate_limit should be retryable")
		}
		if p.IsRetryable("not_found") {
			t.Error("not_found should not be retryable")
		}
	})
}

func TestWorkflowErrorMode(t *testing.T) {
	if WorkflowErrorModeFail.ShouldAbort() != true {
		t.Error("fail mode should abort")
	}
	if WorkflowErrorModeContinue.ShouldAbort() != false {
		t.Error("continue mode should not abort")
	}
	if WorkflowErrorModeUseDefault.ShouldAbort() != false {
		t.Error("use_default mode should not abort")
	}

	if WorkflowErrorModeFail.OutcomeState() != NodeStateFailed {
		t.Error("fail outcome should be Failed")
	}
	if WorkflowErrorModeUseDefault.OutcomeState() != NodeStateDefaulted {
		t.Error("use_default outcome should be Defaulted")
	}
	if WorkflowErrorModeContinue.OutcomeState() != NodeStateFailed {
		t.Error("continue outcome should be Failed")
	}
}
