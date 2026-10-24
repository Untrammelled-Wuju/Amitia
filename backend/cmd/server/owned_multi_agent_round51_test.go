package main

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

func round51SetParentDeadline(t *testing.T, tracker interaction.InteractionTracker, deadline time.Time) {
	t.Helper()
	parent, found, err := tracker.Get(context.Background(), "parent-a")
	if err != nil || !found || parent == nil || parent.RecoveryDescriptor == nil || parent.RecoveryDescriptor.MultiAgent == nil {
		t.Fatalf("parent missing: %+v %v", parent, err)
	}
	updated := *parent.RecoveryDescriptor
	ref := *updated.MultiAgent
	ref.DeadlineAt = &deadline
	updated.MultiAgent = &ref
	if _, err := tracker.UpdateMetadata(context.Background(), parent.ID, interaction.InteractionMetadataUpdate{RecoveryDescriptor: &updated}); err != nil {
		t.Fatal(err)
	}
}

func TestRound51ExpiredDurableDeadlineRejectsWorkerReservation(t *testing.T) {
	tracker := interaction.NewInMemoryTracker()
	registerOwnedWorkerParent(t, tracker)
	round51SetParentDeadline(t, tracker, time.Now().Add(-time.Second))
	var calls atomic.Int32
	runner := newOwnedMultiAgentWorkerRunner(ownedWorkerEngineFunc(func(context.Context, coordination.ExecutionScope, business.Request, []business.ProjectWorkspace, string) (business.Response, error) {
		calls.Add(1)
		return business.Response{Saved: true}, nil
	}), tracker)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	if _, err := runner.StartWorker(context.Background(), ownedWorkerFixture()); err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("expired parent deadline accepted: %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("expired delegation performed an external operation")
	}
}

func TestRound51ActiveWorkerStopsAfterDurableDeadline(t *testing.T) {
	tracker := interaction.NewInMemoryTracker()
	registerOwnedWorkerParent(t, tracker)
	round51SetParentDeadline(t, tracker, time.Now().Add(350*time.Millisecond))
	started := make(chan struct{})
	stopped := make(chan struct{})
	runner := newOwnedMultiAgentWorkerRunner(ownedWorkerEngineFunc(func(ctx context.Context, _ coordination.ExecutionScope, _ business.Request, _ []business.ProjectWorkspace, _ string) (business.Response, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return business.Response{}, ctx.Err()
	}), tracker)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	childID, err := runner.StartWorker(context.Background(), ownedWorkerFixture())
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not start before deadline")
	}
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("worker continued beyond parent deadline")
	}
	child := waitOwnedWorkerRecord(t, tracker, childID, interaction.InteractionStatusInterrupted)
	if child.ResultRef != "" {
		t.Fatalf("expired worker published a successful result: %+v", child)
	}
}

func TestRound51LateCommittedResponseCannotBypassParentDeadline(t *testing.T) {
	tracker := interaction.NewInMemoryTracker()
	registerOwnedWorkerParent(t, tracker)
	round51SetParentDeadline(t, tracker, time.Now().Add(250*time.Millisecond))
	started := make(chan struct{})
	release := make(chan struct{})
	runner := newOwnedMultiAgentWorkerRunner(ownedWorkerEngineFunc(func(context.Context, coordination.ExecutionScope, business.Request, []business.ProjectWorkspace, string) (business.Response, error) {
		close(started)
		<-release
		return business.Response{Saved: true, Generation: business.Generation{Text: "late external result"}}, nil
	}), tracker)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	childID, err := runner.StartWorker(context.Background(), ownedWorkerFixture())
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not start")
	}
	time.Sleep(350 * time.Millisecond)
	close(release)
	child := waitOwnedWorkerRecord(t, tracker, childID, interaction.InteractionStatusInterrupted)
	if child.ResultRef != "" {
		t.Fatalf("late response bypassed parent deadline: %+v", child)
	}
}
