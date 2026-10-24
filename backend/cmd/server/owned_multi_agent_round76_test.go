package main

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/mindruntime"
)

func TestRound76WorkerSharesPersistedToolCallBudget(t *testing.T) {
	tracker, db := ownedWorkerBudgetDatabase(t)
	registerOwnedWorkerParent(t, tracker)
	parent, found, err := tracker.Get(context.Background(), "parent-a")
	if err != nil || !found {
		t.Fatal(err)
	}
	limit := 2
	parent.RecoveryDescriptor.MultiAgent.MaxToolCalls = &limit
	if _, err := tracker.UpdateMetadata(context.Background(), parent.ID, interaction.InteractionMetadataUpdate{RecoveryDescriptor: parent.RecoveryDescriptor}); err != nil {
		t.Fatal(err)
	}
	usage, err := mindruntime.NewModelUsageStore(db, "space-a")
	if err != nil {
		t.Fatal(err)
	}
	seen := make(chan error, 1)
	engine := ownedWorkerEngineFunc(func(ctx context.Context, _ coordination.ExecutionScope, _ business.Request, _ []business.ProjectWorkspace, _ string) (business.Response, error) {
		budget, present := mindruntime.OwnedToolBudgetFromContext(ctx)
		if !present || budget.ParentInteractionID != "parent-a" || budget.Limit != 2 {
			seen <- errors.New("parent budget was not propagated to worker")
			return business.Response{}, errors.New("missing budget")
		}
		if err := usage.ReserveOwnedToolCall(ctx, budget, "first", "read_file"); err != nil {
			seen <- err
			return business.Response{}, err
		}
		if err := usage.ReserveOwnedToolCall(ctx, budget, "second", "read_file"); err != nil {
			seen <- err
			return business.Response{}, err
		}
		if err := usage.ReserveOwnedToolCall(ctx, budget, "third", "shell_run"); !errors.Is(err, mindruntime.ErrOwnedToolBudgetExceeded) {
			seen <- errors.New("sibling could exceed shared limit")
			return business.Response{}, err
		}
		seen <- nil
		return business.Response{Saved: true, Generation: business.Generation{Text: "ok"}}, nil
	})
	runner := newOwnedMultiAgentWorkerRunner(engine, tracker)
	runner.usageStore = usage
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	id, err := runner.StartWorker(context.Background(), ownedWorkerFixture())
	if err != nil {
		t.Fatal(err)
	}
	if err := <-seen; err != nil {
		t.Fatal(err)
	}
	waitOwnedWorkerRecord(t, tracker, id, interaction.InteractionStatusCompleted)
}

func TestRound76WorkerRejectsBudgetWithoutDurableStore(t *testing.T) {
	tracker := interaction.NewInMemoryTracker()
	registerOwnedWorkerParent(t, tracker)
	parent, _, err := tracker.Get(context.Background(), "parent-a")
	if err != nil {
		t.Fatal(err)
	}
	limit := 1
	parent.RecoveryDescriptor.MultiAgent.MaxToolCalls = &limit
	if _, err := tracker.UpdateMetadata(context.Background(), parent.ID, interaction.InteractionMetadataUpdate{RecoveryDescriptor: parent.RecoveryDescriptor}); err != nil {
		t.Fatal(err)
	}
	runner := newOwnedMultiAgentWorkerRunner(nil, tracker)
	if _, err := runner.StartWorker(context.Background(), ownedWorkerFixture()); err == nil {
		t.Fatal("budgeted worker started without usage ledger")
	}
}
