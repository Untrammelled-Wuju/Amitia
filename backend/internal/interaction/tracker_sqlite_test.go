package interaction

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSQLiteInteractionTrackerPersistsFullScopeAndCAS(t *testing.T) {
	tracker := newTestSQLiteInteractionTracker(t)
	ctx := context.Background()
	record := NewInteractionRecord(InteractionScope{
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		Channel:        "web",
		PeerID:         "peer-1",
		SessionID:      "session-1",
		Source:         "web",
		RequestID:      "request-1",
	})
	record.Priority = 1
	record.PathType = "standard"

	if err := tracker.Create(ctx, record); err != nil {
		t.Fatal(err)
	}

	got, ok, err := tracker.Get(ctx, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("record missing")
	}
	if got.Scope.UserID != "user-1" || got.Scope.PeerID != "peer-1" || got.Scope.SessionID != "session-1" || got.Scope.RequestID != "request-1" {
		t.Fatalf("scope was not fully restored: %#v", got.Scope)
	}

	processing, err := tracker.TransitionCAS(ctx, record.ID, got.StatusVersion, InteractionStatusProcessing)
	if err != nil {
		t.Fatal(err)
	}
	if processing.Status != InteractionStatusProcessing || processing.StatusVersion != got.StatusVersion+1 {
		t.Fatalf("bad transition result: %#v", processing)
	}
	if _, err := tracker.TransitionCAS(ctx, record.ID, got.StatusVersion, InteractionStatusCompleted); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected version conflict, got %v", err)
	}
}

func TestSQLiteInteractionTrackerCancelAndArchiveKeepsRecord(t *testing.T) {
	tracker := newTestSQLiteInteractionTracker(t)
	ctx := context.Background()
	record := NewInteractionRecord(InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web"})
	if err := tracker.Create(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err := tracker.RequestCancel(ctx, record.ID, "user_cancelled"); err != nil {
		t.Fatal(err)
	}
	got, ok, err := tracker.Get(ctx, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("record missing after cancel request")
	}
	if got.CancelReason != "user_cancelled" || got.CancelRequestedAt.IsZero() {
		t.Fatalf("cancel request was not persisted: %#v", got)
	}
	if err := tracker.Archive(ctx, record.ID); err != nil {
		t.Fatal(err)
	}
	got, ok, err = tracker.Get(ctx, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.Status != InteractionStatusArchived {
		t.Fatalf("archive should keep the record with archived status: ok=%v record=%#v", ok, got)
	}
}

func TestUnifiedEntryBackpressureRejectsInvalidConfig(t *testing.T) {
	processor := &runtimeCaptureProcessor{}
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), processor, NewInMemoryTracker(), NewInMemoryOutboxStore())
	orch.SetReady(true)
	entry := NewUnifiedEntry(orch, NewScopeResolver(nil))
	entry.SetBackpressureConfig(BackpressureConfig{MaxQueueDepth: 0, WarningRatio: -1, SheddingRatio: 2})
	if status := entry.GetBackpressureStatus(); status != BackpressureNormal {
		t.Fatalf("invalid config should normalize without shedding, got %s", status)
	}
}

func newTestSQLiteInteractionTracker(t *testing.T) *SQLiteInteractionTracker {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "interaction.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	tracker := NewSQLiteInteractionTracker(db)
	if err := tracker.InitSchema(); err != nil {
		t.Fatal(err)
	}
	return tracker
}
