package interaction

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
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

func TestSQLiteInteractionTrackerGetByRequestID(t *testing.T) {
	tracker := newTestSQLiteInteractionTracker(t)
	ctx := context.Background()
	record := NewInteractionRecord(InteractionScope{
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		Channel:        "web",
		RequestID:      "request-1",
	})
	if err := tracker.Create(ctx, record); err != nil {
		t.Fatal(err)
	}

	got, ok, err := tracker.GetByRequestID(ctx, "user-1", "request-1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("record missing by request id")
	}
	if got.ID != record.ID || got.Scope.UserID != "user-1" || got.Scope.RequestID != "request-1" {
		t.Fatalf("unexpected record: %#v", got)
	}
	if _, ok, err := tracker.GetByRequestID(ctx, "user-2", "request-1"); err != nil || ok {
		t.Fatalf("unexpected record for different user: ok=%v err=%v", ok, err)
	}
	if _, ok, err := tracker.GetByRequestID(ctx, "user-1", ""); err != nil || ok {
		t.Fatalf("unexpected record for empty request id: ok=%v err=%v", ok, err)
	}
}

func TestSQLiteInteractionTrackerRejectsDuplicateRequestID(t *testing.T) {
	tracker := newTestSQLiteInteractionTracker(t)
	ctx := context.Background()
	first := NewInteractionRecord(InteractionScope{
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		Channel:        "web",
		RequestID:      "request-1",
	})
	if err := tracker.Create(ctx, first); err != nil {
		t.Fatal(err)
	}
	duplicate := NewInteractionRecord(InteractionScope{
		UserID:         "user-1",
		CharacterID:    "char-2",
		ConversationID: "conv-2",
		Channel:        "web",
		RequestID:      "request-1",
	})
	if err := tracker.Create(ctx, duplicate); !errors.Is(err, ErrDuplicateRequest) {
		t.Fatalf("expected duplicate request error, got %v", err)
	}
	got, ok, err := tracker.GetByRequestID(ctx, "user-1", "request-1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.ID != first.ID {
		t.Fatalf("duplicate should keep first record, ok=%v got=%#v", ok, got)
	}
}

func TestSQLiteInteractionTrackerAllowsSameRequestIDForDifferentUsers(t *testing.T) {
	tracker := newTestSQLiteInteractionTracker(t)
	ctx := context.Background()
	for _, userID := range []string{"user-1", "user-2"} {
		record := NewInteractionRecord(InteractionScope{
			UserID:         userID,
			CharacterID:    "char-1",
			ConversationID: "conv-" + userID,
			Channel:        "web",
			RequestID:      "request-1",
		})
		if err := tracker.Create(ctx, record); err != nil {
			t.Fatalf("create for %s: %v", userID, err)
		}
	}
}

func TestSQLiteInteractionTrackerConcurrentDuplicateRequestID(t *testing.T) {
	tracker := newTestSQLiteInteractionTracker(t)
	ctx := context.Background()
	const workers = 12
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			record := NewInteractionRecord(InteractionScope{
				UserID:         "user-1",
				CharacterID:    "char-1",
				ConversationID: "conv-1",
				Channel:        "web",
				RequestID:      "request-concurrent",
			})
			errs <- tracker.Create(ctx, record)
		}()
	}
	wg.Wait()
	close(errs)
	successes := 0
	duplicates := 0
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrDuplicateRequest):
			duplicates++
		default:
			t.Fatalf("unexpected create error: %v", err)
		}
	}
	if successes != 1 || duplicates != workers-1 {
		t.Fatalf("unexpected results: successes=%d duplicates=%d", successes, duplicates)
	}
	var count int64
	if err := tracker.db.Model(&InteractionRecordModel{}).Where("user_id = ? AND request_id = ?", "user-1", "request-concurrent").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one row, got %d", count)
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
	if got.StatusVersion != record.StatusVersion+1 {
		t.Fatalf("cancel request should bump status version: got %d want %d", got.StatusVersion, record.StatusVersion+1)
	}
	if err := tracker.Archive(ctx, record.ID); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected active archive to fail, got %v", err)
	}
	got, err = tracker.TransitionCAS(ctx, record.ID, got.StatusVersion, InteractionStatusCancelled)
	if err != nil {
		t.Fatal(err)
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

func TestSQLiteInteractionTrackerTerminalOperationsRespectStateMachine(t *testing.T) {
	tracker := newTestSQLiteInteractionTracker(t)
	ctx := context.Background()
	record := NewInteractionRecord(InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web"})
	if err := tracker.Create(ctx, record); err != nil {
		t.Fatal(err)
	}
	processing, err := tracker.TransitionCAS(ctx, record.ID, record.StatusVersion, InteractionStatusProcessing)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := tracker.TransitionCAS(ctx, record.ID, processing.StatusVersion, InteractionStatusGenerated)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := tracker.TransitionCAS(ctx, record.ID, generated.StatusVersion, InteractionStatusCommitted)
	if err != nil {
		t.Fatal(err)
	}
	if err := tracker.MarkSuperseded(ctx, record.ID, "new-id"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected committed supersede to fail, got %v", err)
	}
	if err := tracker.RequestCancel(ctx, record.ID, "late_cancel"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected committed cancel to fail, got %v", err)
	}
	completed, err := tracker.Complete(ctx, record.ID, "result")
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != InteractionStatusCompleted || completed.StatusVersion != committed.StatusVersion+1 {
		t.Fatalf("complete did not follow state machine: %#v", completed)
	}
	if _, err := tracker.Complete(ctx, record.ID, "again"); !errors.Is(err, ErrAlreadyTerminal) {
		t.Fatalf("expected terminal complete to fail, got %v", err)
	}
	if _, err := tracker.Fail(ctx, record.ID, "late", "late"); !errors.Is(err, ErrAlreadyTerminal) {
		t.Fatalf("expected terminal fail to fail, got %v", err)
	}
}

func TestSupersedeResolverExcludesCurrentRecord(t *testing.T) {
	tracker := newTestSQLiteInteractionTracker(t)
	ctx := context.Background()
	scope := InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web"}.Normalize()
	oldRecord := NewInteractionRecord(scope)
	if err := tracker.Create(ctx, oldRecord); err != nil {
		t.Fatal(err)
	}
	newRecord := NewInteractionRecord(scope)
	if err := tracker.Create(ctx, newRecord); err != nil {
		t.Fatal(err)
	}
	resolver := NewSupersedeResolver(SupersedePolicyLatest, tracker)
	resolution, err := resolver.ResolveExcluding(ctx, scope, newRecord.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resolution.SupersedeTargetID != oldRecord.ID {
		t.Fatalf("expected old record to be superseded, got %q want %q", resolution.SupersedeTargetID, oldRecord.ID)
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
