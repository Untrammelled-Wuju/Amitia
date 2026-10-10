package interaction

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/temporal"
	"gorm.io/gorm"
)

func TestAsyncWorkerReopensDurableReservationFromSQLite(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "multi-agent-reservation.db")
	db1, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	original := NewSQLiteInteractionTracker(db1)
	if err := original.InitSchema(); err != nil {
		t.Fatal(err)
	}
	req := asyncReq("disk-reserved-worker")
	pending := NewInteractionRecord(InteractionScope{
		SpaceID: req.SpaceID, CharacterID: req.CharacterID,
		ConversationID: workerConversationID(req), Channel: "web",
		RequestID: "ma-" + req.AssignmentID, Source: req.Source,
	})
	id := NewAsyncUnifiedEntryWorkerRunner(nil, nil).(*asyncUnifiedEntryWorkerRunner).WorkerInteractionID(req)
	pending.ID = id
	if err := original.Create(ctx, pending); err != nil {
		t.Fatal(err)
	}
	sql1, _ := db1.DB()
	if err := sql1.Close(); err != nil {
		t.Fatal(err)
	}
	db2, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sql2, _ := db2.DB()
	defer sql2.Close()
	reopened := NewSQLiteInteractionTracker(db2)
	processor := &blockingAsyncWorkerProcessor{entered: make(chan string, 2), release: make(chan struct{})}
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), processor, reopened, nil)
	orch.SetReady(true)
	entry := NewUnifiedEntry(orch, NewScopeResolver(nil), temporal.SystemClock{})
	runner := NewAsyncUnifiedEntryWorkerRunner(entry, reopened).(*asyncUnifiedEntryWorkerRunner)
	defer func() { close(processor.release); _ = runner.Shutdown(context.Background()) }()
	resumed, err := runner.StartWorker(ctx, req)
	if err != nil || resumed != id {
		t.Fatalf("SQLite worker reservation changed on reopen: %q %v", resumed, err)
	}
	select {
	case <-processor.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("SQLite reserved worker did not enter execution")
	}
	rec, exists, err := reopened.Get(ctx, id)
	if err != nil || !exists || rec.Scope.RequestID != pending.Scope.RequestID {
		t.Fatalf("durable child identity changed: %+v %v", rec, err)
	}
	if processor.calls.Load() != 1 {
		t.Fatalf("reserved child invoked %d times", processor.calls.Load())
	}
}

func TestSQLiteTrackerRejectsOversizedMultiAgentCheckpoint(t *testing.T) {
	ctx := context.Background()
	tracker := newTestSQLiteInteractionTracker(t)
	record := NewInteractionRecord(InteractionScope{
		SpaceID: "space", CharacterID: "character", ConversationID: "conversation",
		RequestID: "parent-with-large-plan", Channel: "web",
	})
	if err := tracker.Create(ctx, record); err != nil {
		t.Fatal(err)
	}
	desc := &RecoveryDescriptor{MultiAgent: &MultiAgentRecoveryRef{
		CoordinationID: "too-large", Status: "running",
		AssignmentRefs: []AssignmentRecoveryRef{{AssignmentID: "worker", WorkerID: "worker",
			Objective: strings.Repeat("A", RecoveryDescriptorMaxSizeBytes*2), Status: "pending"}},
	}}
	if _, err := tracker.UpdateMetadata(ctx, record.ID, InteractionMetadataUpdate{RecoveryDescriptor: desc}); err == nil {
		t.Fatal("SQLite silently ignored a checkpoint that cannot be stored")
	}
	fresh, found, err := tracker.Get(ctx, record.ID)
	if err != nil || !found {
		t.Fatal(err)
	}
	if fresh.RecoveryDescriptor != nil {
		t.Fatal("partially written oversized recovery checkpoint")
	}
}
