package rpc

import (
	"context"
	"time"
)

const (
	MetadataRPCTimeoutMS = "rpc.timeout_ms"

	DefaultRPCTimeout = 30 * time.Second
	MaximumRPCTimeout = 5 * time.Minute
	MinimumRPCTimeout = 100 * time.Millisecond
)

type TimeoutConfig struct {
	Default time.Duration
	Maximum time.Duration
	Minimum time.Duration
}

func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		Default: DefaultRPCTimeout,
		Maximum: MaximumRPCTimeout,
		Minimum: MinimumRPCTimeout,
	}
}

func EffectiveDeadline(
	ctx context.Context,
	requestedTimeoutMS int64,
	config TimeoutConfig,
	now time.Time,
) (time.Time, time.Duration) {
	var deadline time.Time

	if d, ok := ctx.Deadline(); ok {
		deadline = d
	}

	if requestedTimeoutMS > 0 {
		requestedDuration := time.Duration(requestedTimeoutMS) * time.Millisecond
		if requestedDuration > config.Maximum {
			requestedDuration = config.Maximum
		}
		if requestedDuration < config.Minimum {
			requestedDuration = config.Minimum
		}
		requestedDeadline := now.Add(requestedDuration)
		if !deadline.IsZero() && requestedDeadline.Before(deadline) {
			deadline = requestedDeadline
		} else if deadline.IsZero() {
			deadline = requestedDeadline
		}
	}

	if deadline.IsZero() {
		deadline = now.Add(config.Default)
	}

	duration := deadline.Sub(now)
	if duration > config.Maximum {
		deadline = now.Add(config.Maximum)
		duration = config.Maximum
	}
	if duration < config.Minimum {
		deadline = now.Add(config.Minimum)
		duration = config.Minimum
	}

	return deadline, duration
}
