package event

import (
	"math/rand"
	"time"
)

type RetryPolicy struct {
	MaxAttempts         int
	InitialBackoff      time.Duration
	MaxBackoff          time.Duration
	Multiplier          float64
	Jitter              float64
	RetryableErrorCodes []string
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:    5,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     5 * time.Minute,
		Multiplier:     2.0,
		Jitter:         0.2,
		RetryableErrorCodes: []string{
			"runtime_unavailable",
			"runtime_crashed",
			"timeout",
			"temporary_dependency_unavailable",
			"temporary_host_error",
			"rate_limited",
		},
	}
}

func (p RetryPolicy) IsRetryable(code string) bool {
	if p.isNonRetryable(code) {
		return false
	}
	codes := p.RetryableErrorCodes
	if len(codes) == 0 {
		codes = DefaultRetryPolicy().RetryableErrorCodes
	}
	for _, c := range codes {
		if c == code {
			return true
		}
	}
	return false
}

func (p RetryPolicy) isNonRetryable(code string) bool {
	nonRetryable := []string{
		"permission_denied",
		"scope_denied",
		"invalid_payload",
		"invalid_subscription",
		"handler_not_found",
		"unsupported_event_version",
		"extension_disabled",
		"permanent_dependency_missing",
		"event_loop_detected",
		"event_depth_exceeded",
		"invalid_result",
		"protocol_error",
		"host_api_abuse",
	}
	for _, c := range nonRetryable {
		if c == code {
			return true
		}
	}
	return false
}

func (p RetryPolicy) ComputeBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return p.InitialBackoff
	}
	backoff := float64(p.InitialBackoff)
	for i := 1; i < attempt; i++ {
		backoff *= p.Multiplier
		if time.Duration(backoff) > p.MaxBackoff {
			backoff = float64(p.MaxBackoff)
			break
		}
	}
	if p.Jitter > 0 {
		jitterRange := backoff * p.Jitter
		offset := (rand.Float64() - 0.5) * 2 * jitterRange
		backoff += offset
		if backoff < 0 {
			backoff = 0
		}
	}
	d := time.Duration(backoff)
	if d > p.MaxBackoff {
		d = p.MaxBackoff
	}
	if d < p.InitialBackoff && attempt > 1 {
		d = p.InitialBackoff
	}
	return d
}

func (p RetryPolicy) ShouldRetry(attempt int, code string) bool {
	if attempt >= p.MaxAttempts {
		return false
	}
	return p.IsRetryable(code)
}

func (p RetryPolicy) MergeWith(override RetryPolicy) RetryPolicy {
	result := p
	if override.MaxAttempts > 0 {
		result.MaxAttempts = override.MaxAttempts
	}
	if override.InitialBackoff > 0 {
		result.InitialBackoff = override.InitialBackoff
	}
	if override.MaxBackoff > 0 {
		result.MaxBackoff = override.MaxBackoff
	}
	if override.Multiplier > 0 {
		result.Multiplier = override.Multiplier
	}
	if override.Jitter > 0 {
		result.Jitter = override.Jitter
	}
	if len(override.RetryableErrorCodes) > 0 {
		result.RetryableErrorCodes = override.RetryableErrorCodes
	}
	if result.MaxAttempts > p.MaxAttempts {
		result.MaxAttempts = p.MaxAttempts
	}
	return result
}

type BackoffSchedule struct {
	policy RetryPolicy
}

func NewBackoffSchedule(policy RetryPolicy) *BackoffSchedule {
	return &BackoffSchedule{policy: policy}
}

func (s *BackoffSchedule) Next(attempt int) time.Duration {
	return s.policy.ComputeBackoff(attempt)
}

func (s *BackoffSchedule) MaxAttempts() int {
	return s.policy.MaxAttempts
}
