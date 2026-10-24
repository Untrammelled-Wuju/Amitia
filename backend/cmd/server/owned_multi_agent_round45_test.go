package main

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

func TestRound45ParentRevocationCannotProduceChildSuccess(t *testing.T) {
	tracker := interaction.NewInMemoryTracker()
	registerOwnedWorkerParent(t, tracker)
	started := make(chan struct{})
	release := make(chan struct{})
	runner := newOwnedMultiAgentWorkerRunner(ownedWorkerEngineFunc(func(context.Context, coordination.ExecutionScope, business.Request, []business.ProjectWorkspace, string) (business.Response, error) {
		close(started)
		<-release
		return business.Response{Saved: true, Generation: business.Generation{Text: "external result exists"}}, nil
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
	if err := tracker.RequestCancel(context.Background(), "parent-a", "parent authorization revoked"); err != nil {
		t.Fatal(err)
	}
	close(release)
	child := waitOwnedWorkerRecord(t, tracker, childID, interaction.InteractionStatusInterrupted)
	if child.ResultRef != "" {
		t.Fatalf("revoked parent authorized successful child: %+v", child)
	}
}

func TestRound45ChildLineageDriftCannotProduceChildSuccess(t *testing.T) {
	tracker := interaction.NewInMemoryTracker()
	registerOwnedWorkerParent(t, tracker)
	started := make(chan struct{})
	release := make(chan struct{})
	runner := newOwnedMultiAgentWorkerRunner(ownedWorkerEngineFunc(func(context.Context, coordination.ExecutionScope, business.Request, []business.ProjectWorkspace, string) (business.Response, error) {
		close(started)
		<-release
		return business.Response{Saved: true, Generation: business.Generation{Text: "untrusted result"}}, nil
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
	unrelatedParent := "wrong-parent"
	if _, err := tracker.UpdateMetadata(context.Background(), childID, interaction.InteractionMetadataUpdate{CausationID: &unrelatedParent}); err != nil {
		t.Fatal(err)
	}
	close(release)
	child := waitOwnedWorkerRecord(t, tracker, childID, interaction.InteractionStatusInterrupted)
	if child.ResultRef != "" {
		t.Fatalf("changed lineage produced success: %+v", child)
	}
}

func TestRound45ValidParentStillSettlesCommittedResult(t *testing.T) {
	tracker := interaction.NewInMemoryTracker()
	registerOwnedWorkerParent(t, tracker)
	runner := newOwnedMultiAgentWorkerRunner(ownedWorkerEngineFunc(func(context.Context, coordination.ExecutionScope, business.Request, []business.ProjectWorkspace, string) (business.Response, error) {
		return business.Response{Saved: true, Generation: business.Generation{Text: "committed result"}}, nil
	}), tracker)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	childID, err := runner.StartWorker(context.Background(), ownedWorkerFixture())
	if err != nil {
		t.Fatal(err)
	}
	child := waitOwnedWorkerRecord(t, tracker, childID, interaction.InteractionStatusCompleted)
	if child.ResultRef != "committed result" {
		t.Fatalf("committed result missing: %+v", child)
	}
}
