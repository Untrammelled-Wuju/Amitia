package main

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

func TestRound42OwnedSettlementPreservesDurableOutcomes(t *testing.T) {
	tests := []struct {
		name      string
		cancelled bool
		response  business.Response
		err       error
		want      interaction.InteractionStatus
	}{
		{"committed_before_late_cancel", true, business.Response{Saved: true}, nil, interaction.InteractionStatusCompleted},
		{"committed_normal", false, business.Response{Saved: true}, nil, interaction.InteractionStatusCompleted},
		{"interrupted_and_saved_partial", true, business.Response{Saved: true, Interrupted: true}, business.ErrInterrupted, interaction.InteractionStatusInterrupted},
		{"cancelled_external_outcome_unknown", true, business.Response{}, context.Canceled, interaction.InteractionStatusInterrupted},
		{"unsaved_error", false, business.Response{}, errors.New("failed"), interaction.InteractionStatusFailed},
		{"partial_without_cancel", false, business.Response{Saved: true, Generation: business.Generation{Partial: true}}, nil, interaction.InteractionStatusFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyOwnedWorkerSettlement(tt.cancelled, tt.response, tt.err); got != tt.want {
				t.Fatalf("got %s want %s", got, tt.want)
			}
		})
	}
}

func TestRound42LateCancelAfterCommittedOwnedWorkerReturnsCompleted(t *testing.T) {
	tracker := interaction.NewInMemoryTracker()
	registerOwnedWorkerParent(t, tracker)
	started := make(chan struct{})
	release := make(chan struct{})
	runner := newOwnedMultiAgentWorkerRunner(ownedWorkerEngineFunc(func(_ context.Context, _ coordination.ExecutionScope, _ business.Request, _ []business.ProjectWorkspace, _ string) (business.Response, error) {
		close(started)
		<-release
		return business.Response{Saved: true, Generation: business.Generation{Text: "durable before stop"}}, nil
	}), tracker)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	id, err := runner.StartWorker(context.Background(), ownedWorkerFixture())
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if err := runner.CancelWorker(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	close(release)
	record := waitOwnedWorkerRecord(t, tracker, id, interaction.InteractionStatusCompleted)
	if record.ResultRef != "durable before stop" {
		t.Fatalf("committed result was replaced by cancellation: %+v", record)
	}
}
