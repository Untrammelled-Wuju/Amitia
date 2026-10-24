package main

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
	"gorm.io/gorm"
)

func round43Trackers(t *testing.T) (interaction.InteractionTracker, interaction.InteractionTracker) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "round43-shared.db")
	open := func() *gorm.DB {
		db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
		if err != nil {
			t.Fatal(err)
		}
		conn, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}
		conn.SetMaxOpenConns(1)
		t.Cleanup(func() { _ = conn.Close() })
		return db
	}
	first := interaction.NewSQLiteInteractionTracker(open())
	if err := first.InitSchema(); err != nil {
		t.Fatal(err)
	}
	return first, interaction.NewSQLiteInteractionTracker(open())
}

func TestRound43TwoCoreWorkerCASPreventsDuplicateEngineExecution(t *testing.T) {
	first, second := round43Trackers(t)
	registerOwnedWorkerParent(t, first)
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var runs atomic.Int32
	engine := ownedWorkerEngineFunc(func(ctx context.Context, _ coordination.ExecutionScope, _ business.Request, _ []business.ProjectWorkspace, _ string) (business.Response, error) {
		runs.Add(1)
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-release:
			return business.Response{Saved: true, Generation: business.Generation{Text: "one committed result"}}, nil
		case <-ctx.Done():
			return business.Response{}, ctx.Err()
		}
	})
	r1 := newOwnedMultiAgentWorkerRunner(engine, first)
	r2 := newOwnedMultiAgentWorkerRunner(engine, second)
	t.Cleanup(func() { close(release); _ = r1.Shutdown(context.Background()); _ = r2.Shutdown(context.Background()) })
	req := ownedWorkerFixture()
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, runner := range []*ownedMultiAgentWorkerRunner{r1, r2} {
		wg.Add(1)
		go func(r *ownedMultiAgentWorkerRunner) {
			defer wg.Done()
			<-start
			_, _ = r.StartWorker(context.Background(), req)
		}(runner)
	}
	close(start)
	wg.Wait()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("neither core executed the worker")
	}
	time.Sleep(100 * time.Millisecond)
	if runs.Load() != 1 {
		t.Fatalf("same task executed by %d cores", runs.Load())
	}
}

func TestRound43OtherCoreCancellationStopsExecutingWorker(t *testing.T) {
	first, second := round43Trackers(t)
	registerOwnedWorkerParent(t, first)
	started := make(chan struct{})
	stopped := make(chan struct{})
	engine := ownedWorkerEngineFunc(func(ctx context.Context, _ coordination.ExecutionScope, _ business.Request, _ []business.ProjectWorkspace, _ string) (business.Response, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return business.Response{}, ctx.Err()
	})
	owner := newOwnedMultiAgentWorkerRunner(engine, first)
	otherCore := newOwnedMultiAgentWorkerRunner(engine, second)
	t.Cleanup(func() { _ = owner.Shutdown(context.Background()); _ = otherCore.Shutdown(context.Background()) })
	childID, err := owner.StartWorker(context.Background(), ownedWorkerFixture())
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not start")
	}
	if err := otherCore.CancelWorker(context.Background(), childID); err != nil {
		t.Fatal(err)
	}
	record, _, err := second.Get(context.Background(), childID)
	if err != nil || record.IsTerminal() {
		t.Fatalf("remote cancellation fabricated a stopped worker: %+v, %v", record, err)
	}
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("remote cancellation was not observed by active core")
	}
	waitOwnedWorkerRecord(t, second, childID, interaction.InteractionStatusInterrupted)
}

func TestRound43ParentCancelIsObservedByRunningChild(t *testing.T) {
	first, second := round43Trackers(t)
	registerOwnedWorkerParent(t, first)
	parent, _, err := first.Get(context.Background(), "parent-a")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	stopped := make(chan struct{})
	engine := ownedWorkerEngineFunc(func(ctx context.Context, _ coordination.ExecutionScope, _ business.Request, _ []business.ProjectWorkspace, _ string) (business.Response, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return business.Response{}, ctx.Err()
	})
	worker := newOwnedMultiAgentWorkerRunner(engine, first)
	t.Cleanup(func() { _ = worker.Shutdown(context.Background()) })
	childID, err := worker.StartWorker(context.Background(), ownedWorkerFixture())
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("child did not start")
	}
	if err := second.RequestCancel(context.Background(), parent.ID, "parent goal stopped"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("parent cancellation not observed by child")
	}
	waitOwnedWorkerRecord(t, second, childID, interaction.InteractionStatusInterrupted)
}
