package main

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

func TestRound75WorkerExecutionUsesDurableParentDeadline(t *testing.T) {
	tracker := interaction.NewInMemoryTracker()
	registerOwnedWorkerParent(t, tracker)
	parentDeadline := time.Now().Add(15 * time.Second).UTC()
	round51SetParentDeadline(t, tracker, parentDeadline)
	observed := make(chan time.Time, 1)
	release := make(chan struct{})
	runner := newOwnedMultiAgentWorkerRunner(ownedWorkerEngineFunc(func(ctx context.Context, _ coordination.ExecutionScope, _ business.Request, _ []business.ProjectWorkspace, _ string) (business.Response, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Error("worker execution context has no deadline")
		} else {
			observed <- deadline
		}
		select {
		case <-release:
			return business.Response{Saved: true, Generation: business.Generation{Text: "done"}}, nil
		case <-ctx.Done():
			return business.Response{}, ctx.Err()
		}
	}), tracker)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	childID, err := runner.StartWorker(context.Background(), ownedWorkerFixture())
	if err != nil {
		t.Fatal(err)
	}
	select {
	case actual := <-observed:
		if !actual.Equal(parentDeadline) {
			t.Fatalf("worker deadline %s differs from durable parent %s", actual, parentDeadline)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not begin before its parent deadline")
	}
	close(release)
	waitOwnedWorkerRecord(t, tracker, childID, interaction.InteractionStatusCompleted)
}

func TestRound75WorkerExecutionCannotOutliveAttemptTimeout(t *testing.T) {
	tracker := interaction.NewInMemoryTracker()
	registerOwnedWorkerParent(t, tracker)
	parentDeadline := time.Now().Add(4 * time.Hour).UTC()
	round51SetParentDeadline(t, tracker, parentDeadline)
	observed := make(chan time.Time, 1)
	runner := newOwnedMultiAgentWorkerRunner(ownedWorkerEngineFunc(func(ctx context.Context, _ coordination.ExecutionScope, _ business.Request, _ []business.ProjectWorkspace, _ string) (business.Response, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Error("worker execution context has no deadline")
		} else {
			observed <- deadline
		}
		return business.Response{Saved: true}, nil
	}), tracker)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	start := time.Now()
	childID, err := runner.StartWorker(context.Background(), ownedWorkerFixture())
	if err != nil {
		t.Fatal(err)
	}
	select {
	case actual := <-observed:
		if actual.Before(start.Add(ownedWorkerAttemptTimeout-time.Second)) || actual.After(time.Now().Add(ownedWorkerAttemptTimeout+time.Second)) {
			t.Fatalf("worker deadline %s does not respect attempt timeout", actual)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not begin")
	}
	waitOwnedWorkerRecord(t, tracker, childID, interaction.InteractionStatusCompleted)
}
