package execution

import (
	"context"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/timeoutpolicy"
	"testing"
	"time"
)

func TestOperationTimeoutOverridesKernelBudgets(t *testing.T) {
	restore := timeoutpolicy.Configure(timeoutpolicy.Settings{Seconds: 60})
	defer restore()
	controller := NewTimeoutController(time.Second)
	now := time.Now()
	inv := capability.ToolInvocationContext{ExpiresAt: now.Add(time.Second)}
	tool := capability.ToolDefinition{TimeoutMS: 1}
	budget, err := controller.ResolveBudget(context.Background(), now, inv, tool)
	if err != nil || budget.ConfiguredTimeout != time.Minute {
		t.Fatalf("unexpected budget: %+v %v", budget, err)
	}
	timeoutpolicy.Configure(timeoutpolicy.Settings{Disabled: true, Seconds: 60})
	budget, err = controller.ResolveBudget(context.Background(), now, inv, tool)
	if err != nil || !budget.Unlimited || budget.Expired(now) || budget.Remaining(now) <= 0 {
		t.Fatalf("disabled budget: %+v %v", budget, err)
	}
	parent, cancel := context.WithCancel(context.Background())
	ctx, end, err := controller.Wrap(parent, budget)
	defer end()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("disabled tool retained deadline")
	}
	cancel()
	if ctx.Err() != context.Canceled {
		t.Fatal("tool cancellation was lost")
	}
}
