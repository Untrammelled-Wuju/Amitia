package mindruntime

import (
	"context"
	"github.com/u-ai/backend/internal/timeoutpolicy"
	"testing"
	"time"
)

func TestOperationTimeoutDeadlinePropagation(t *testing.T) {
	restore := timeoutpolicy.Configure(timeoutpolicy.Settings{Disabled: true, Seconds: 30})
	defer restore()
	propagator := NewDeadlinePropagator(DeadlineConfig{TotalTimeout: time.Millisecond})
	d := propagator.NewDeadline("request")
	if !d.Deadline.IsZero() || propagator.IsExpired("request") {
		t.Fatal("disabled request retained deadline")
	}
	ctx, cancel := propagator.ContextWithDeadline(context.Background(), "request", DeadlineStageGeneration)
	defer cancel()
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("propagated deadline still exists")
	}
	propagator.Cancel("request", "user canceled")
	if propagator.ValidateBeforePersist("request") || propagator.Remaining("request") != 0 {
		t.Fatal("canceled request was allowed to persist")
	}
	ctx, finish := propagator.ContextWithDeadline(context.Background(), "request", DeadlineStagePersist)
	defer finish()
	if ctx.Err() != context.Canceled {
		t.Fatal("manual cancellation was lost")
	}
	timeoutpolicy.Configure(timeoutpolicy.Settings{Seconds: 60})
	d = propagator.NewDeadline("new")
	if d.Total != time.Minute {
		t.Fatal("global duration was not applied")
	}
}
