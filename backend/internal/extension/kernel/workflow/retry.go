package workflow

import (
	"encoding/json"
	"math"
	"math/rand"
	"time"
)

type WorkflowRetryPolicy struct {
	MaxAttempts        int           `json:"maxAttempts,omitempty"`
	InitialBackoff     time.Duration `json:"initialBackoff,omitempty"`
	MaxBackoff         time.Duration `json:"maxBackoff,omitempty"`
	Multiplier         float64       `json:"multiplier,omitempty"`
	Jitter             float64       `json:"jitter,omitempty"`
	RetryableErrorCodes []string     `json:"retryableErrorCodes,omitempty"`
}

func DefaultRetryPolicy() *WorkflowRetryPolicy {
	return &WorkflowRetryPolicy{
		MaxAttempts:    1,
		InitialBackoff: 200 * time.Millisecond,
		MaxBackoff:     30 * time.Second,
		Multiplier:     2.0,
		Jitter:         0.2,
	}
}

func (p *WorkflowRetryPolicy) Normalize() *WorkflowRetryPolicy {
	if p == nil {
		return DefaultRetryPolicy()
	}
	np := *p
	if np.MaxAttempts < 1 {
		np.MaxAttempts = 1
	}
	if np.InitialBackoff <= 0 {
		np.InitialBackoff = 200 * time.Millisecond
	}
	if np.MaxBackoff <= 0 {
		np.MaxBackoff = 30 * time.Second
	}
	if np.Multiplier <= 1.0 {
		np.Multiplier = 2.0
	}
	if np.Jitter < 0 {
		np.Jitter = 0
	} else if np.Jitter > 1 {
		np.Jitter = 1
	}
	return &np
}

func (p *WorkflowRetryPolicy) ComputeBackoff(attempt int) time.Duration {
	np := p.Normalize()
	if attempt <= 0 {
		return 0
	}
	backoff := float64(np.InitialBackoff) * math.Pow(np.Multiplier, float64(attempt-1))
	if backoff > float64(np.MaxBackoff) {
		backoff = float64(np.MaxBackoff)
	}
	if np.Jitter > 0 {
		delta := backoff * np.Jitter
		backoff = backoff - delta + rand.Float64()*(2*delta)
	}
	return time.Duration(backoff)
}

func (p *WorkflowRetryPolicy) IsRetryable(errorCode string) bool {
	np := p.Normalize()
	if len(np.RetryableErrorCodes) == 0 {
		return true
	}
	for _, code := range np.RetryableErrorCodes {
		if code == errorCode {
			return true
		}
	}
	return false
}

type WorkflowNodeErrorPolicy struct {
	Mode    WorkflowErrorMode `json:"mode"`
	Default json.RawMessage   `json:"default,omitempty"`
}

type WorkflowErrorMode string

const (
	WorkflowErrorModeFail      WorkflowErrorMode = "fail"
	WorkflowErrorModeContinue  WorkflowErrorMode = "continue"
	WorkflowErrorModeUseDefault WorkflowErrorMode = "use_default"
)

func (m WorkflowErrorMode) OutcomeState() NodeState {
	switch m {
	case WorkflowErrorModeContinue:
		return NodeStateFailed
	case WorkflowErrorModeUseDefault:
		return NodeStateDefaulted
	case WorkflowErrorModeFail:
		return NodeStateFailed
	default:
		return NodeStateFailed
	}
}

func (m WorkflowErrorMode) ShouldAbort() bool {
	return m == WorkflowErrorModeFail
}
