package interaction

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
	"github.com/u-ai/backend/internal/temporal"

	"github.com/glebarez/sqlite"
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

func TestSQLiteInteractionTrackerUpdateMetadata(t *testing.T) {
	tracker := newTestSQLiteInteractionTracker(t)
	ctx := context.Background()
	record := NewInteractionRecord(InteractionScope{
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		Channel:        "web",
		RequestID:      "request-metadata",
	})
	if err := tracker.Create(ctx, record); err != nil {
		t.Fatal(err)
	}

	priority := 2
	pathType := string(PathTypeStandard)
	commitID := "msg-1,msg-2"
	executorID := "worker-1"
	deadline := time.Now().Add(time.Minute).UTC().Truncate(time.Second)
	updated, err := tracker.UpdateMetadata(ctx, record.ID, InteractionMetadataUpdate{
		Priority:   &priority,
		PathType:   &pathType,
		CommitID:   &commitID,
		ExecutorID: &executorID,
		DeadlineAt: &deadline,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Priority != priority || updated.PathType != pathType || updated.CommitID != commitID || updated.ExecutorID != executorID {
		t.Fatalf("metadata was not returned: %#v", updated)
	}
	got, ok, err := tracker.Get(ctx, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("record missing")
	}
	if got.Priority != priority || got.PathType != pathType || got.CommitID != commitID || got.ExecutorID != executorID {
		t.Fatalf("metadata was not persisted: %#v", got)
	}
	if !got.DeadlineAt.Equal(deadline) {
		t.Fatalf("deadline was not persisted: got %s want %s", got.DeadlineAt, deadline)
	}
	if got.StatusVersion != 0 {
		t.Fatalf("metadata update should not bump status version: %d", got.StatusVersion)
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

func TestSQLiteInteractionTrackerListActiveIncludesDelivered(t *testing.T) {
	tracker := newTestSQLiteInteractionTracker(t)
	ctx := context.Background()
	scope := InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web"}
	record := NewInteractionRecord(scope)
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
	delivered, err := tracker.TransitionCAS(ctx, record.ID, committed.StatusVersion, InteractionStatusDelivered)
	if err != nil {
		t.Fatal(err)
	}
	if !delivered.IsActive() {
		t.Fatalf("expected delivered record to be active")
	}

	active, err := tracker.ListActive(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].ID != record.ID || active[0].Status != InteractionStatusDelivered {
		t.Fatalf("expected delivered record in active list, got %#v", active)
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
	if err := tracker.Archive(ctx, record.ID, got.StatusVersion); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected active archive to fail, got %v", err)
	}
	got, err = tracker.TransitionCAS(ctx, record.ID, got.StatusVersion, InteractionStatusCancelled)
	if err != nil {
		t.Fatal(err)
	}
	if err := tracker.Archive(ctx, record.ID, got.StatusVersion); err != nil {
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
	completed, err := tracker.Complete(ctx, record.ID, committed.StatusVersion, "result")
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != InteractionStatusCompleted || completed.StatusVersion != committed.StatusVersion+1 {
		t.Fatalf("complete did not follow state machine: %#v", completed)
	}
	if _, err := tracker.Complete(ctx, record.ID, completed.StatusVersion, "again"); !errors.Is(err, ErrAlreadyTerminal) {
		t.Fatalf("expected terminal complete to fail, got %v", err)
	}
	if _, err := tracker.Fail(ctx, record.ID, completed.StatusVersion, "late", "late"); !errors.Is(err, ErrAlreadyTerminal) {
		t.Fatalf("expected terminal fail to fail, got %v", err)
	}
}

func TestSQLiteInteractionTrackerMarkSupersededRequiresValidSuperseder(t *testing.T) {
	tracker := newTestSQLiteInteractionTracker(t)
	ctx := context.Background()
	scope := InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web"}
	target := NewInteractionRecord(scope)
	if err := tracker.Create(ctx, target); err != nil {
		t.Fatal(err)
	}
	if err := tracker.MarkSuperseded(ctx, target.ID, "missing"); !errors.Is(err, ErrInteractionNotFound) {
		t.Fatalf("expected missing superseder to fail, got %v", err)
	}
	got, ok, err := tracker.Get(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.Status != InteractionStatusReceived || got.SupersededByID != "" {
		t.Fatalf("target should remain active after failed supersede: ok=%v record=%#v", ok, got)
	}

	otherScope := NewInteractionRecord(InteractionScope{UserID: "user-1", CharacterID: "char-2", ConversationID: "conv-2", Channel: "web"})
	if err := tracker.Create(ctx, otherScope); err != nil {
		t.Fatal(err)
	}
	if err := tracker.MarkSuperseded(ctx, target.ID, otherScope.ID); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected cross-scope superseder to fail, got %v", err)
	}

	terminalSuperseder := NewInteractionRecord(scope)
	if err := tracker.Create(ctx, terminalSuperseder); err != nil {
		t.Fatal(err)
	}
	if _, err := tracker.Fail(ctx, terminalSuperseder.ID, terminalSuperseder.StatusVersion, "failed", "failed"); err != nil {
		t.Fatal(err)
	}
	if err := tracker.MarkSuperseded(ctx, target.ID, terminalSuperseder.ID); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected terminal superseder to fail, got %v", err)
	}
	got, ok, err = tracker.Get(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.Status != InteractionStatusReceived || got.SupersededByID != "" {
		t.Fatalf("target should remain active after invalid superseders: ok=%v record=%#v", ok, got)
	}
}

func TestSQLiteInteractionTrackerMarkSupersededValidSupersederIsAtomic(t *testing.T) {
	tracker := newTestSQLiteInteractionTracker(t)
	ctx := context.Background()
	scope := InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web"}
	target := NewInteractionRecord(scope)
	if err := tracker.Create(ctx, target); err != nil {
		t.Fatal(err)
	}
	superseder := NewInteractionRecord(scope)
	if err := tracker.Create(ctx, superseder); err != nil {
		t.Fatal(err)
	}
	if err := tracker.MarkSuperseded(ctx, target.ID, superseder.ID); err != nil {
		t.Fatal(err)
	}
	got, ok, err := tracker.Get(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.Status != InteractionStatusSuperseded || got.SupersededByID != superseder.ID {
		t.Fatalf("target was not superseded by valid superseder: ok=%v record=%#v", ok, got)
	}
	if got.StatusVersion != target.StatusVersion+1 || got.CompletedAt.IsZero() {
		t.Fatalf("supersede should bump version and complete target: %#v", got)
	}
	if err := tracker.MarkSuperseded(ctx, target.ID, superseder.ID); !errors.Is(err, ErrAlreadyTerminal) {
		t.Fatalf("expected terminal target supersede to fail, got %v", err)
	}
	after, ok, err := tracker.Get(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || after.Status != InteractionStatusSuperseded || after.SupersededByID != superseder.ID || after.StatusVersion != got.StatusVersion {
		t.Fatalf("terminal supersede retry should not mutate target: ok=%v record=%#v", ok, after)
	}
}

func TestInMemoryInteractionTrackerTerminalGuardsMatchSQLite(t *testing.T) {
	tracker := NewInMemoryTracker()
	ctx := context.Background()
	scope := InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web"}
	target := NewInteractionRecord(scope)
	if err := tracker.Create(ctx, target); err != nil {
		t.Fatal(err)
	}
	superseder := NewInteractionRecord(scope)
	if err := tracker.Create(ctx, superseder); err != nil {
		t.Fatal(err)
	}
	if err := tracker.Archive(ctx, target.ID, target.StatusVersion); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected active archive to fail, got %v", err)
	}
	if err := tracker.MarkSuperseded(ctx, target.ID, superseder.ID); err != nil {
		t.Fatal(err)
	}
	got, ok, err := tracker.Get(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.Status != InteractionStatusSuperseded || got.SupersededByID != superseder.ID {
		t.Fatalf("target was not superseded: ok=%v record=%#v", ok, got)
	}
	if err := tracker.MarkSuperseded(ctx, target.ID, superseder.ID); !errors.Is(err, ErrAlreadyTerminal) {
		t.Fatalf("expected terminal target supersede to fail, got %v", err)
	}
	after, ok, err := tracker.Get(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || after.Status != InteractionStatusSuperseded || after.StatusVersion != got.StatusVersion {
		t.Fatalf("terminal supersede retry should not mutate target: ok=%v record=%#v", ok, after)
	}
}

func TestInteractionTrackerImplementationsShareTerminalSemantics(t *testing.T) {
	ctx := context.Background()
	implementations := []struct {
		name string
		new  func(t *testing.T) InteractionTracker
	}{
		{name: "sqlite", new: func(t *testing.T) InteractionTracker { return newTestSQLiteInteractionTracker(t) }},
		{name: "memory", new: func(t *testing.T) InteractionTracker { return NewInMemoryTracker() }},
	}

	for _, impl := range implementations {
		t.Run(impl.name+"/cancel_archive_keeps_record", func(t *testing.T) {
			tracker := impl.new(t)
			record := NewInteractionRecord(InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web"})
			if err := tracker.Create(ctx, record); err != nil {
				t.Fatal(err)
			}
			if err := tracker.RequestCancel(ctx, record.ID, "user_cancelled"); err != nil {
				t.Fatal(err)
			}
			cancelRequested, ok, err := tracker.Get(ctx, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				t.Fatal("record missing after cancel request")
			}
			if cancelRequested.Status != InteractionStatusReceived || cancelRequested.CancelReason != "user_cancelled" || cancelRequested.CancelRequestedAt.IsZero() || cancelRequested.StatusVersion != 1 {
				t.Fatalf("cancel request semantics mismatch: %#v", cancelRequested)
			}
			cancelled, err := tracker.TransitionCAS(ctx, record.ID, cancelRequested.StatusVersion, InteractionStatusCancelled)
			if err != nil {
				t.Fatal(err)
			}
			if cancelled.Status != InteractionStatusCancelled || cancelled.StatusVersion != 2 {
				t.Fatalf("cancel transition mismatch: %#v", cancelled)
			}
			if err := tracker.Archive(ctx, record.ID, cancelled.StatusVersion); err != nil {
				t.Fatal(err)
			}
			archived, ok, err := tracker.Get(ctx, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !ok || archived.Status != InteractionStatusArchived || archived.CancelReason != "user_cancelled" {
				t.Fatalf("archive should keep cancelled record: ok=%v record=%#v", ok, archived)
			}
		})

		t.Run(impl.name+"/committed_rejects_late_cancel", func(t *testing.T) {
			tracker := impl.new(t)
			record := NewInteractionRecord(InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-2", Channel: "web"})
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
			if err := tracker.RequestCancel(ctx, record.ID, "too_late"); !errors.Is(err, ErrInvalidTransition) {
				t.Fatalf("expected committed cancel to fail, got %v", err)
			}
			after, ok, err := tracker.Get(ctx, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !ok || after.Status != InteractionStatusCommitted || after.StatusVersion != committed.StatusVersion || after.CancelReason != "" {
				t.Fatalf("late cancel should not mutate committed record: ok=%v record=%#v", ok, after)
			}
		})

		t.Run(impl.name+"/completed_late_cancel_is_idempotent", func(t *testing.T) {
			tracker := impl.new(t)
			record := NewInteractionRecord(InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-3", Channel: "web"})
			if err := tracker.Create(ctx, record); err != nil {
				t.Fatal(err)
			}
			processing, err := tracker.TransitionCAS(ctx, record.ID, record.StatusVersion, InteractionStatusProcessing)
			if err != nil {
				t.Fatal(err)
			}
			completed, err := tracker.Complete(ctx, record.ID, processing.StatusVersion, "result-ref")
			if err != nil {
				t.Fatal(err)
			}
			if completed.Status != InteractionStatusCompleted || completed.StatusVersion != processing.StatusVersion+1 {
				t.Fatalf("complete semantics mismatch: %#v", completed)
			}
			if err := tracker.RequestCancel(ctx, record.ID, "too_late"); err != nil {
				t.Fatalf("terminal cancel should be idempotent, got %v", err)
			}
			after, ok, err := tracker.Get(ctx, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !ok || after.Status != InteractionStatusCompleted || after.StatusVersion != completed.StatusVersion || after.CancelReason != "" || after.ResultRef != "result-ref" {
				t.Fatalf("terminal cancel should not mutate completed record: ok=%v record=%#v", ok, after)
			}
		})
	}
}

func TestInteractionTrackerTerminalOperationsRejectStaleVersion(t *testing.T) {
	ctx := context.Background()
	implementations := []struct {
		name string
		new  func(t *testing.T) InteractionTracker
	}{
		{name: "sqlite", new: func(t *testing.T) InteractionTracker { return newTestSQLiteInteractionTracker(t) }},
		{name: "memory", new: func(t *testing.T) InteractionTracker { return NewInMemoryTracker() }},
	}

	for _, impl := range implementations {
		t.Run(impl.name+"/complete", func(t *testing.T) {
			tracker := impl.new(t)
			record := NewInteractionRecord(InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-stale-complete", Channel: "web"})
			if err := tracker.Create(ctx, record); err != nil {
				t.Fatal(err)
			}
			staleVersion := record.StatusVersion
			processing, err := tracker.TransitionCAS(ctx, record.ID, staleVersion, InteractionStatusProcessing)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tracker.Complete(ctx, record.ID, staleVersion, "result"); !errors.Is(err, ErrVersionConflict) {
				t.Fatalf("expected stale complete to fail, got %v", err)
			}
			got, ok, err := tracker.Get(ctx, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !ok || got.Status != InteractionStatusProcessing || got.StatusVersion != processing.StatusVersion || got.ResultRef != "" {
				t.Fatalf("stale complete mutated record: ok=%v record=%#v", ok, got)
			}
		})

		t.Run(impl.name+"/fail", func(t *testing.T) {
			tracker := impl.new(t)
			record := NewInteractionRecord(InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-stale-fail", Channel: "web"})
			if err := tracker.Create(ctx, record); err != nil {
				t.Fatal(err)
			}
			staleVersion := record.StatusVersion
			processing, err := tracker.TransitionCAS(ctx, record.ID, staleVersion, InteractionStatusProcessing)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tracker.Fail(ctx, record.ID, staleVersion, "late", "late"); !errors.Is(err, ErrVersionConflict) {
				t.Fatalf("expected stale fail to fail, got %v", err)
			}
			got, ok, err := tracker.Get(ctx, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !ok || got.Status != InteractionStatusProcessing || got.StatusVersion != processing.StatusVersion || got.ErrorCode != "" || got.ErrorMessage != "" {
				t.Fatalf("stale fail mutated record: ok=%v record=%#v", ok, got)
			}
		})

		t.Run(impl.name+"/archive", func(t *testing.T) {
			tracker := impl.new(t)
			record := NewInteractionRecord(InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-stale-archive", Channel: "web"})
			if err := tracker.Create(ctx, record); err != nil {
				t.Fatal(err)
			}
			staleVersion := record.StatusVersion
			processing, err := tracker.TransitionCAS(ctx, record.ID, staleVersion, InteractionStatusProcessing)
			if err != nil {
				t.Fatal(err)
			}
			completed, err := tracker.Complete(ctx, record.ID, processing.StatusVersion, "result")
			if err != nil {
				t.Fatal(err)
			}
			if err := tracker.Archive(ctx, record.ID, processing.StatusVersion); !errors.Is(err, ErrVersionConflict) {
				t.Fatalf("expected stale archive to fail, got %v", err)
			}
			got, ok, err := tracker.Get(ctx, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !ok || got.Status != InteractionStatusCompleted || got.StatusVersion != completed.StatusVersion {
				t.Fatalf("stale archive mutated record: ok=%v record=%#v", ok, got)
			}
		})
	}
}

func TestInteractionTrackerRequestCancelBumpsVersion(t *testing.T) {
	ctx := context.Background()
	implementations := []struct {
		name string
		new  func(t *testing.T) InteractionTracker
	}{
		{name: "sqlite", new: func(t *testing.T) InteractionTracker { return newTestSQLiteInteractionTracker(t) }},
		{name: "memory", new: func(t *testing.T) InteractionTracker { return NewInMemoryTracker() }},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			tracker := impl.new(t)
			record := NewInteractionRecord(InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-cancel-version", Channel: "web"})
			if err := tracker.Create(ctx, record); err != nil {
				t.Fatal(err)
			}
			initialVersion := record.StatusVersion
			if err := tracker.RequestCancel(ctx, record.ID, "first"); err != nil {
				t.Fatal(err)
			}
			first, ok, err := tracker.Get(ctx, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !ok || first.StatusVersion != initialVersion+1 || first.CancelReason != "first" || first.CancelRequestedAt.IsZero() {
				t.Fatalf("first cancel request did not bump version: ok=%v record=%#v", ok, first)
			}
			if err := tracker.RequestCancel(ctx, record.ID, "second"); err != nil {
				t.Fatal(err)
			}
			second, ok, err := tracker.Get(ctx, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !ok || second.StatusVersion != first.StatusVersion+1 || second.CancelReason != "second" || second.CancelRequestedAt.Before(first.CancelRequestedAt) {
				t.Fatalf("second cancel request did not bump version: ok=%v record=%#v first=%#v", ok, second, first)
			}
		})
	}
}

func TestInteractionTrackerCompleteRejectsCancelledAndSuperseded(t *testing.T) {
	ctx := context.Background()
	implementations := []struct {
		name string
		new  func(t *testing.T) InteractionTracker
	}{
		{name: "sqlite", new: func(t *testing.T) InteractionTracker { return newTestSQLiteInteractionTracker(t) }},
		{name: "memory", new: func(t *testing.T) InteractionTracker { return NewInMemoryTracker() }},
	}

	for _, impl := range implementations {
		t.Run(impl.name+"/cancelled", func(t *testing.T) {
			tracker := impl.new(t)
			record := NewInteractionRecord(InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-complete-cancelled", Channel: "web"})
			if err := tracker.Create(ctx, record); err != nil {
				t.Fatal(err)
			}
			if err := tracker.RequestCancel(ctx, record.ID, "cancel"); err != nil {
				t.Fatal(err)
			}
			cancelRequested, ok, err := tracker.Get(ctx, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				t.Fatal("record missing")
			}
			cancelled, err := tracker.TransitionCAS(ctx, record.ID, cancelRequested.StatusVersion, InteractionStatusCancelled)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tracker.Complete(ctx, record.ID, cancelled.StatusVersion, "late"); !errors.Is(err, ErrAlreadyTerminal) {
				t.Fatalf("expected cancelled complete to fail, got %v", err)
			}
			after, ok, err := tracker.Get(ctx, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !ok || after.Status != InteractionStatusCancelled || after.StatusVersion != cancelled.StatusVersion || after.ResultRef != "" {
				t.Fatalf("cancelled complete mutated record: ok=%v record=%#v", ok, after)
			}
		})

		t.Run(impl.name+"/superseded", func(t *testing.T) {
			tracker := impl.new(t)
			scope := InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-complete-superseded", Channel: "web"}
			target := NewInteractionRecord(scope)
			if err := tracker.Create(ctx, target); err != nil {
				t.Fatal(err)
			}
			superseder := NewInteractionRecord(scope)
			if err := tracker.Create(ctx, superseder); err != nil {
				t.Fatal(err)
			}
			if err := tracker.MarkSuperseded(ctx, target.ID, superseder.ID); err != nil {
				t.Fatal(err)
			}
			superseded, ok, err := tracker.Get(ctx, target.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				t.Fatal("record missing")
			}
			if _, err := tracker.Complete(ctx, target.ID, superseded.StatusVersion, "late"); !errors.Is(err, ErrAlreadyTerminal) {
				t.Fatalf("expected superseded complete to fail, got %v", err)
			}
			after, ok, err := tracker.Get(ctx, target.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !ok || after.Status != InteractionStatusSuperseded || after.StatusVersion != superseded.StatusVersion || after.ResultRef != "" {
				t.Fatalf("superseded complete mutated record: ok=%v record=%#v", ok, after)
			}
		})
	}
}

func TestInteractionTrackerMarkSupersededRejectsCommittedAndTerminalTargets(t *testing.T) {
	ctx := context.Background()
	implementations := []struct {
		name string
		new  func(t *testing.T) InteractionTracker
	}{
		{name: "sqlite", new: func(t *testing.T) InteractionTracker { return newTestSQLiteInteractionTracker(t) }},
		{name: "memory", new: func(t *testing.T) InteractionTracker { return NewInMemoryTracker() }},
	}

	for _, impl := range implementations {
		t.Run(impl.name+"/committed_status", func(t *testing.T) {
			tracker := impl.new(t)
			scope := InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-supersede-committed", Channel: "web"}
			target := NewInteractionRecord(scope)
			if err := tracker.Create(ctx, target); err != nil {
				t.Fatal(err)
			}
			superseder := NewInteractionRecord(scope)
			if err := tracker.Create(ctx, superseder); err != nil {
				t.Fatal(err)
			}
			processing, err := tracker.TransitionCAS(ctx, target.ID, target.StatusVersion, InteractionStatusProcessing)
			if err != nil {
				t.Fatal(err)
			}
			generated, err := tracker.TransitionCAS(ctx, target.ID, processing.StatusVersion, InteractionStatusGenerated)
			if err != nil {
				t.Fatal(err)
			}
			committed, err := tracker.TransitionCAS(ctx, target.ID, generated.StatusVersion, InteractionStatusCommitted)
			if err != nil {
				t.Fatal(err)
			}
			if err := tracker.MarkSuperseded(ctx, target.ID, superseder.ID); !errors.Is(err, ErrInvalidTransition) {
				t.Fatalf("expected committed supersede to fail, got %v", err)
			}
			after, ok, err := tracker.Get(ctx, target.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !ok || after.Status != InteractionStatusCommitted || after.StatusVersion != committed.StatusVersion || after.SupersededByID != "" {
				t.Fatalf("committed supersede mutated record: ok=%v record=%#v", ok, after)
			}
		})

		t.Run(impl.name+"/commit_marker", func(t *testing.T) {
			tracker := impl.new(t)
			scope := InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-supersede-commit-marker", Channel: "web"}
			target := NewInteractionRecord(scope)
			if err := tracker.Create(ctx, target); err != nil {
				t.Fatal(err)
			}
			superseder := NewInteractionRecord(scope)
			if err := tracker.Create(ctx, superseder); err != nil {
				t.Fatal(err)
			}
			commitID := "commit-1"
			updated, err := tracker.UpdateMetadata(ctx, target.ID, InteractionMetadataUpdate{CommitID: &commitID})
			if err != nil {
				t.Fatal(err)
			}
			if err := tracker.MarkSuperseded(ctx, target.ID, superseder.ID); !errors.Is(err, ErrInvalidTransition) {
				t.Fatalf("expected commit marker supersede to fail, got %v", err)
			}
			after, ok, err := tracker.Get(ctx, target.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !ok || after.Status != InteractionStatusReceived || after.StatusVersion != updated.StatusVersion || after.SupersededByID != "" || after.CommitID != commitID {
				t.Fatalf("commit marker supersede mutated record: ok=%v record=%#v", ok, after)
			}
		})

		t.Run(impl.name+"/terminal", func(t *testing.T) {
			tracker := impl.new(t)
			scope := InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-supersede-terminal", Channel: "web"}
			target := NewInteractionRecord(scope)
			if err := tracker.Create(ctx, target); err != nil {
				t.Fatal(err)
			}
			superseder := NewInteractionRecord(scope)
			if err := tracker.Create(ctx, superseder); err != nil {
				t.Fatal(err)
			}
			failed, err := tracker.Fail(ctx, target.ID, target.StatusVersion, "failed", "failed")
			if err != nil {
				t.Fatal(err)
			}
			if err := tracker.MarkSuperseded(ctx, target.ID, superseder.ID); !errors.Is(err, ErrAlreadyTerminal) {
				t.Fatalf("expected terminal supersede to fail, got %v", err)
			}
			after, ok, err := tracker.Get(ctx, target.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !ok || after.Status != InteractionStatusFailed || after.StatusVersion != failed.StatusVersion || after.SupersededByID != "" {
				t.Fatalf("terminal supersede mutated record: ok=%v record=%#v", ok, after)
			}
		})
	}
}

func TestInteractionTrackerArchiveOnlyAllowsTerminalStatuses(t *testing.T) {
	ctx := context.Background()
	implementations := []struct {
		name string
		new  func(t *testing.T) InteractionTracker
	}{
		{name: "sqlite", new: func(t *testing.T) InteractionTracker { return newTestSQLiteInteractionTracker(t) }},
		{name: "memory", new: func(t *testing.T) InteractionTracker { return NewInMemoryTracker() }},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			for _, status := range allInteractionStatusesForTest() {
				if status == InteractionStatusArchived {
					continue
				}
				t.Run(string(status), func(t *testing.T) {
					tracker := impl.new(t)
					record := NewInteractionRecord(InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-archive-" + string(status), Channel: "web"})
					record.Status = status
					if status != InteractionStatusReceived {
						record.StatusVersion = 1
					}
					if err := tracker.Create(ctx, record); err != nil {
						t.Fatal(err)
					}
					err := tracker.Archive(ctx, record.ID, record.StatusVersion)
					if isTerminalStatus(status) {
						if err != nil {
							t.Fatalf("expected terminal archive to succeed, got %v", err)
						}
						after, ok, err := tracker.Get(ctx, record.ID)
						if err != nil {
							t.Fatal(err)
						}
						if !ok || after.Status != InteractionStatusArchived {
							t.Fatalf("terminal archive did not archive record: ok=%v record=%#v", ok, after)
						}
						return
					}
					if !errors.Is(err, ErrInvalidTransition) {
						t.Fatalf("expected active archive to fail, got %v", err)
					}
					after, ok, err := tracker.Get(ctx, record.ID)
					if err != nil {
						t.Fatal(err)
					}
					if !ok || after.Status != status || after.StatusVersion != record.StatusVersion {
						t.Fatalf("active archive mutated record: ok=%v record=%#v", ok, after)
					}
				})
			}
		})
	}
}

func TestSQLiteInteractionTrackerListActiveMatchesIsActive(t *testing.T) {
	tracker := newTestSQLiteInteractionTracker(t)
	ctx := context.Background()
	scope := InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-active-consistency", Channel: "web"}
	wantActive := map[string]bool{}
	for _, status := range allInteractionStatusesForTest() {
		record := NewInteractionRecord(scope)
		record.Status = status
		record.StatusVersion = int64(len(wantActive))
		if err := tracker.Create(ctx, record); err != nil {
			t.Fatal(err)
		}
		wantActive[record.ID] = record.IsActive()
	}

	active, err := tracker.ListActive(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	gotActive := map[string]bool{}
	for _, record := range active {
		gotActive[record.ID] = true
		if !record.IsActive() {
			t.Fatalf("ListActive returned inactive record: %#v", record)
		}
	}
	for id, want := range wantActive {
		if gotActive[id] != want {
			t.Fatalf("ListActive mismatch for %s: got %v want %v", id, gotActive[id], want)
		}
	}
}

func allInteractionStatusesForTest() []InteractionStatus {
	return []InteractionStatus{
		InteractionStatusReceived,
		InteractionStatusNormalized,
		InteractionStatusQueued,
		InteractionStatusProcessing,
		InteractionStatusContextReady,
		InteractionStatusDecided,
		InteractionStatusGenerated,
		InteractionStatusCommitted,
		InteractionStatusDeliveryPending,
		InteractionStatusDelivered,
		InteractionStatusCompleted,
		InteractionStatusSuperseded,
		InteractionStatusCancelled,
		InteractionStatusFailed,
		InteractionStatusInterrupted,
		InteractionStatusArchived,
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
	entry := NewUnifiedEntry(orch, NewScopeResolver(nil), temporal.SystemClock{})
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
