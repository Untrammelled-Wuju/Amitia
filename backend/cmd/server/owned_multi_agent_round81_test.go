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

func TestRound81OwnedWorkerAdmissionRejectsRevocationAndBudgetRevision(t *testing.T) {
	for _, mode := range []string{"revoked", "budget-revised"} {
		t.Run(mode, func(t *testing.T) {
			tracker, db := ownedWorkerBudgetDatabase(t)
			registerOwnedWorkerParent(t, tracker)
			parent, found, err := tracker.Get(context.Background(), "parent-a")
			if err != nil || !found {
				t.Fatal(err)
			}
			toolLimit := 4
			tokenLimit := int64(512)
			parent.RecoveryDescriptor.MultiAgent.MaxToolCalls = &toolLimit
			parent.RecoveryDescriptor.MultiAgent.MaxTotalTokens = &tokenLimit
			if _, err := tracker.UpdateMetadata(context.Background(), parent.ID, interaction.InteractionMetadataUpdate{RecoveryDescriptor: parent.RecoveryDescriptor}); err != nil {
				t.Fatal(err)
			}
			store, err := mindruntime.NewModelUsageStore(db, "space-a")
			if err != nil {
				t.Fatal(err)
			}
			ready := make(chan struct{})
			proceed := make(chan struct{})
			results := make(chan error, 1)
			engine := ownedWorkerEngineFunc(func(ctx context.Context, _ coordination.ExecutionScope, _ business.Request, _ []business.ProjectWorkspace, _ string) (business.Response, error) {
				model, ok := mindruntime.OwnedModelBudgetFromContext(ctx)
				if !ok {
					results <- errors.New("missing model budget")
					return business.Response{}, errors.New("missing model budget")
				}
				if _, err := store.Begin(ctx, mindruntime.ModelUsageRecord{SpaceID: model.SpaceID, ReservedTokens: 128}); err != nil {
					results <- err
					return business.Response{}, err
				}
				close(ready)
				<-proceed
				if _, err := store.Begin(ctx, mindruntime.ModelUsageRecord{SpaceID: model.SpaceID, ReservedTokens: 128}); err == nil {
					results <- errors.New("revoked or revised parent admitted model invocation")
					return business.Response{}, errors.New("unauthorized model invocation")
				}
				tools, ok := mindruntime.OwnedToolBudgetFromContext(ctx)
				if !ok || store.ReserveOwnedToolCall(ctx, tools, "after-revocation", "read_file") == nil {
					results <- errors.New("revoked or revised parent admitted tool invocation")
					return business.Response{}, errors.New("unauthorized tool invocation")
				}
				results <- nil
				return business.Response{}, business.ErrInterrupted
			})
			runner := newOwnedMultiAgentWorkerRunner(engine, tracker)
			runner.usageStore = store
			t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
			if _, err := runner.StartWorker(context.Background(), ownedWorkerFixture()); err != nil {
				t.Fatal(err)
			}
			select {
			case <-ready:
			case err := <-results:
				t.Fatalf("worker failed before admission probe: %v", err)
			}
			if mode == "revoked" {
				if err := tracker.RequestCancel(context.Background(), "parent-a", "owner revoked budget authority"); err != nil {
					t.Fatal(err)
				}
			} else {
				current, found, err := tracker.Get(context.Background(), "parent-a")
				if err != nil || !found {
					t.Fatal(err)
				}
				changed := 5
				current.RecoveryDescriptor.MultiAgent.MaxToolCalls = &changed
				if _, err := tracker.UpdateMetadata(context.Background(), current.ID, interaction.InteractionMetadataUpdate{RecoveryDescriptor: current.RecoveryDescriptor}); err != nil {
					t.Fatal(err)
				}
			}
			close(proceed)
			if err := <-results; err != nil {
				t.Fatal(err)
			}
			var models, tools int64
			if err := db.Model(&mindruntime.ModelUsageRecord{}).Count(&models).Error; err != nil || models != 1 {
				t.Fatalf("expected only first model reservation: %d %v", models, err)
			}
			if err := db.Model(&mindruntime.OwnedToolCallRecord{}).Count(&tools).Error; err != nil || tools != 0 {
				t.Fatalf("unauthorized tool reservation persisted: %d %v", tools, err)
			}
		})
	}
}
