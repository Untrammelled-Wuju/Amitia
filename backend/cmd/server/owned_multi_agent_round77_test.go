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

func TestRound77WorkerReceivesAuthorizedPersistentTotalTokens(t *testing.T) {
	tracker, db := ownedWorkerBudgetDatabase(t)
	registerOwnedWorkerParent(t, tracker)
	parent, _, err := tracker.Get(context.Background(), "parent-a")
	if err != nil {
		t.Fatal(err)
	}
	cap := int64(1024)
	parent.RecoveryDescriptor.MultiAgent.MaxTotalTokens = &cap
	if _, err := tracker.UpdateMetadata(context.Background(), parent.ID, interaction.InteractionMetadataUpdate{RecoveryDescriptor: parent.RecoveryDescriptor}); err != nil {
		t.Fatal(err)
	}
	store, err := mindruntime.NewModelUsageStore(db, "space-a")
	if err != nil {
		t.Fatal(err)
	}
	seen := make(chan error, 1)
	engine := ownedWorkerEngineFunc(func(ctx context.Context, _ coordination.ExecutionScope, _ business.Request, _ []business.ProjectWorkspace, _ string) (business.Response, error) {
		budget, ok := mindruntime.OwnedModelBudgetFromContext(ctx)
		if !ok || budget.TokenLimit != 1024 || budget.ParentInteractionID != "parent-a" {
			seen <- errors.New("missing authorized model token cap")
			return business.Response{}, errors.New("budget missing")
		}
		seen <- nil
		return business.Response{Saved: true, Generation: business.Generation{Text: "ok"}}, nil
	})
	runner := newOwnedMultiAgentWorkerRunner(engine, tracker)
	runner.usageStore = store
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
