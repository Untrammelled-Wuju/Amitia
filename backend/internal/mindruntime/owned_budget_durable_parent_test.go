package mindruntime

import (
	"context"
	"testing"
)

func TestDurableBudgetContextDoesNotInheritAnotherParent(t *testing.T) {
	limits := OwnedBudgetParentLimits{ToolCalls: 3, Tokens: 512, CostMicroUSD: 900, ChildInteractionID: "child"}
	if _, ok := durableOwnedBudgetParent(context.Background()); ok {
		t.Fatal("unscoped context unexpectedly requires a parent")
	}
	ctx := WithDurableOwnedBudgetParent(context.Background(), limits)
	got, ok := durableOwnedBudgetParent(ctx)
	if !ok || got != limits {
		t.Fatalf("durable parent budget limits changed: %+v %v", got, ok)
	}
	args := ownedBudgetParentArguments("space", "role", "owner", "parent", "coord", "assignment", "child", limits.ToolCalls, limits.Tokens, limits.CostMicroUSD)
	if len(args) != 16 || args[0] != "parent" || args[8] != 3 || args[9] != int64(512) || args[10] != int64(900) || args[11] != "assignment" || args[12] != "child" || args[15] != "coord" {
		t.Fatalf("parent fence parameter order changed: %+v", args)
	}
}
