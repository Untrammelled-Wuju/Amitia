package interaction

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type runtimeCaptureProcessor struct {
	got   *RuntimeAssembly
	calls int
}

func (p *runtimeCaptureProcessor) ProcessMessageCtx(ctx context.Context, req *ProcessRequest) (*ProcessResponse, error) {
	p.calls++
	p.got = req.Runtime
	return &ProcessResponse{
		ConversationID: req.ConversationID,
		Reply:          "ok",
		CharacterID:    req.CharacterID,
		CharacterName:  "角色",
		MessageIDs:     []string{"msg-1"},
		RequestID:      req.RequestID,
	}, nil
}

type runtimePsycheLoader struct{}

func (l runtimePsycheLoader) Name() string           { return "psyche" }
func (l runtimePsycheLoader) IsRequired() bool       { return false }
func (l runtimePsycheLoader) Timeout() time.Duration { return time.Second }
func (l runtimePsycheLoader) CacheKey(scope InteractionScope, version string) string {
	return version + scope.CharacterID
}
func (l runtimePsycheLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	return FieldReady[any](PsycheState{Stress: 0.9, Fatigue: 0.2, Arousal: 0.3}, "psyche", version), nil
}

type runtimeBlockedBeliefLoader struct{}

func (l runtimeBlockedBeliefLoader) Name() string           { return "beliefs" }
func (l runtimeBlockedBeliefLoader) IsRequired() bool       { return false }
func (l runtimeBlockedBeliefLoader) Timeout() time.Duration { return time.Second }
func (l runtimeBlockedBeliefLoader) CacheKey(scope InteractionScope, version string) string {
	return version + scope.CharacterID
}
func (l runtimeBlockedBeliefLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	return FieldReady[any](BeliefSet{Conflict: &BeliefConflict{KeyA: "a", ValueA: "1", KeyB: "b", ValueB: "2", RiskLevel: "blocked"}}, "beliefs", version), nil
}

func TestOrchestratorAssemblesRuntimeBeforeProcessor(t *testing.T) {
	processor := &runtimeCaptureProcessor{}
	tracker := NewInMemoryTracker()
	outbox := NewInMemoryOutboxStore()
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), processor, tracker, outbox)
	registry := NewContextLoaderRegistry()
	registry.Register(runtimePsycheLoader{})
	registry.Register(NewChannelContextLoader())
	orch.SetRuntimePipeline(NewRuntimePipeline(registry, NewPathClassifier(), NewTokenBudgetManager(1200)))
	orch.SetReady(true)

	result, err := orch.Process(context.Background(), &ProcessRequest{
		CharacterID:    "char-runtime",
		ConversationID: "conv-runtime",
		Channel:        "web",
		Source:         "web",
		Message:        "我今天很焦虑，需要你认真听我说",
		RequestID:      "req-runtime",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomeCompleted {
		t.Fatalf("unexpected outcome: %s", result.Outcome)
	}
	if processor.got == nil {
		t.Fatal("runtime assembly was not passed to processor")
	}
	if processor.got.Path != PathTypeStandard {
		t.Fatalf("expected standard path from emotional high-stress context, got %s", processor.got.Path)
	}
	if processor.got.Safety.Level != "conservative" {
		t.Fatalf("expected conservative safety, got %#v", processor.got.Safety)
	}
	if processor.got.Context.Psyche.Status != LoadStatusReady {
		t.Fatalf("psyche context not ready: %#v", processor.got.Context.Psyche)
	}
	if processor.got.Transaction.Name != TransactionBoundaryAll {
		t.Fatalf("expected all transaction boundary, got %s", processor.got.Transaction.Name)
	}
	wantExecutorID := executorIDForProcessor(processor)
	if processor.got.ExecutorID != wantExecutorID {
		t.Fatalf("runtime executor id mismatch: got %q want %q", processor.got.ExecutorID, wantExecutorID)
	}
	records, err := outbox.ListPending()
	if err != nil {
		t.Fatal(err)
	}
	foundRuntimeEvent := false
	for _, record := range records {
		if record.EventType == "interaction.runtime_assembled" {
			foundRuntimeEvent = true
			break
		}
	}
	if !foundRuntimeEvent {
		t.Fatalf("runtime outbox event missing: %#v", records)
	}
	record, ok, err := tracker.Get(context.Background(), result.InteractionID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("interaction record missing")
	}
	if record.PathType != string(PathTypeStandard) {
		t.Fatalf("path type was not persisted: %#v", record)
	}
	if record.Priority != 2 {
		t.Fatalf("priority was not persisted: %#v", record)
	}
	if record.CommitID != "msg-1" {
		t.Fatalf("commit id was not persisted: %#v", record)
	}
	if record.ExecutorID != wantExecutorID {
		t.Fatalf("executor id was not persisted: got %q want %q", record.ExecutorID, wantExecutorID)
	}
	if record.DeadlineAt.IsZero() {
		t.Fatalf("deadline was not persisted: %#v", record)
	}
}

func TestOrchestratorPersistsRuntimeExecutorIDToSQLite(t *testing.T) {
	processor := &runtimeCaptureProcessor{}
	tracker := newTestSQLiteInteractionTracker(t)
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), processor, tracker, NewInMemoryOutboxStore())
	orch.SetReady(true)

	result, err := orch.Process(context.Background(), &ProcessRequest{
		UserID:         "user-sqlite",
		CharacterID:    "char-sqlite",
		ConversationID: "conv-sqlite",
		Channel:        "web",
		Source:         "web",
		Message:        "hello",
		RequestID:      "req-sqlite-executor",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomeCompleted {
		t.Fatalf("unexpected outcome: %s", result.Outcome)
	}
	record, ok, err := tracker.Get(context.Background(), result.InteractionID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("interaction record missing")
	}
	wantExecutorID := executorIDForProcessor(processor)
	if record.ExecutorID != wantExecutorID {
		t.Fatalf("executor id was not persisted to sqlite: got %q want %q", record.ExecutorID, wantExecutorID)
	}
}

func TestOrchestratorSafetyBlockedSkipsProcessor(t *testing.T) {
	processor := &runtimeCaptureProcessor{}
	tracker := NewInMemoryTracker()
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), processor, tracker, NewInMemoryOutboxStore())
	registry := NewContextLoaderRegistry()
	registry.Register(runtimeBlockedBeliefLoader{})
	orch.SetRuntimePipeline(NewRuntimePipeline(registry, NewPathClassifier(), NewTokenBudgetManager(1200)))
	orch.SetReady(true)

	result, err := orch.Process(context.Background(), &ProcessRequest{
		CharacterID:    "char-safety",
		ConversationID: "conv-safety",
		Channel:        "web",
		Source:         "web",
		Message:        "test",
		RequestID:      "req-safety",
	})
	if err == nil {
		t.Fatal("expected safety blocked error")
	}
	if !errors.Is(err, ErrOrchestratorSafetyBlocked) {
		t.Fatalf("expected ErrOrchestratorSafetyBlocked, got %v", err)
	}
	if processor.calls != 0 {
		t.Fatalf("processor should not be called when safety blocks, got %d", processor.calls)
	}
	if result == nil || result.Outcome != OutcomeFailed {
		t.Fatalf("expected failed result, got %#v", result)
	}
	record, ok, err := tracker.Get(context.Background(), result.InteractionID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("interaction record missing")
	}
	if record.Status != InteractionStatusFailed || record.ErrorCode != "safety_blocked" {
		t.Fatalf("expected failed safety record, got %#v", record)
	}
}

func TestOrchestratorDuplicateRequestIDDoesNotReprocess(t *testing.T) {
	processor := &runtimeCaptureProcessor{}
	tracker := NewInMemoryTracker()
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), processor, tracker, NewInMemoryOutboxStore())
	orch.SetReady(true)
	req := ProcessRequest{
		UserID:         "user-1",
		CharacterID:    "char-duplicate",
		ConversationID: "conv-duplicate",
		Channel:        "web",
		Source:         "web",
		Message:        "hello",
		RequestID:      "req-duplicate",
	}

	first, err := orch.Process(context.Background(), &req)
	if err != nil {
		t.Fatal(err)
	}
	if first.Outcome != OutcomeCompleted {
		t.Fatalf("unexpected first outcome: %s", first.Outcome)
	}
	second, err := orch.Process(context.Background(), &ProcessRequest{
		UserID:         "user-1",
		CharacterID:    "char-duplicate",
		ConversationID: "conv-duplicate",
		Channel:        "web",
		Source:         "web",
		Message:        "hello again",
		RequestID:      "req-duplicate",
	})
	if !errors.Is(err, ErrOrchestratorDuplicate) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
	if second == nil || second.InteractionID != first.InteractionID {
		t.Fatalf("duplicate did not return existing interaction: first=%#v second=%#v", first, second)
	}
	if processor.calls != 1 {
		t.Fatalf("processor should be called once, got %d", processor.calls)
	}
	count := 0
	if err := tracker.Range(context.Background(), func(record *InteractionRecord) bool {
		count++
		return true
	}); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one interaction record, got %d", count)
	}
}

type supersedeSQLiteProcessor struct {
	calls        atomic.Int32
	firstStarted chan struct{}
}

func (p *supersedeSQLiteProcessor) ProcessMessageCtx(ctx context.Context, req *ProcessRequest) (*ProcessResponse, error) {
	call := p.calls.Add(1)
	if call == 1 {
		close(p.firstStarted)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return &ProcessResponse{
		ConversationID: req.ConversationID,
		Reply:          "new",
		CharacterID:    req.CharacterID,
		RequestID:      req.RequestID,
	}, nil
}

type queueSerialProcessor struct {
	current       atomic.Int32
	max           atomic.Int32
	firstStarted  chan struct{}
	secondStarted chan struct{}
	releaseFirst  chan struct{}
}

func (p *queueSerialProcessor) ProcessMessageCtx(ctx context.Context, req *ProcessRequest) (*ProcessResponse, error) {
	n := p.current.Add(1)
	defer p.current.Add(-1)
	for {
		max := p.max.Load()
		if n <= max || p.max.CompareAndSwap(max, n) {
			break
		}
	}
	switch req.RequestID {
	case "queue-1":
		close(p.firstStarted)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-p.releaseFirst:
		}
	case "queue-2":
		close(p.secondStarted)
	}
	return &ProcessResponse{
		ConversationID: req.ConversationID,
		Reply:          req.RequestID,
		CharacterID:    req.CharacterID,
		RequestID:      req.RequestID,
	}, nil
}

type postSuccessDriftProcessor struct {
	mutate func(ctx context.Context, req *ProcessRequest) error
}

func (p postSuccessDriftProcessor) ProcessMessageCtx(ctx context.Context, req *ProcessRequest) (*ProcessResponse, error) {
	if p.mutate != nil {
		if err := p.mutate(ctx, req); err != nil {
			return nil, err
		}
	}
	return &ProcessResponse{
		ConversationID: req.ConversationID,
		Reply:          "ok",
		CharacterID:    req.CharacterID,
		MessageIDs:     []string{"msg-success"},
		RequestID:      req.RequestID,
	}, nil
}

type completeConflictTracker struct {
	InteractionTracker
	base   *InMemoryTracker
	status InteractionStatus
}

func (t *completeConflictTracker) Complete(ctx context.Context, id string, expectedVersion int64, resultRef string) (*InteractionRecord, error) {
	t.base.mu.Lock()
	if rec, ok := t.base.records[id]; ok {
		rec.Status = t.status
		rec.StatusVersion++
		rec.CompletedAt = time.Now()
		rec.UpdatedAt = rec.CompletedAt
	}
	t.base.mu.Unlock()
	return nil, ErrVersionConflict
}

type cancelTransitionConflictTracker struct {
	InteractionTracker
	base *InMemoryTracker
}

func (t *cancelTransitionConflictTracker) TransitionCAS(ctx context.Context, id string, expectedVersion int64, target InteractionStatus) (*InteractionRecord, error) {
	if target != InteractionStatusCancelled {
		return t.InteractionTracker.TransitionCAS(ctx, id, expectedVersion, target)
	}
	t.base.mu.Lock()
	if rec, ok := t.base.records[id]; ok {
		rec.Status = InteractionStatusCompleted
		rec.StatusVersion++
		rec.ResultRef = "completed_elsewhere"
		rec.CompletedAt = time.Now()
		rec.UpdatedAt = rec.CompletedAt
	}
	t.base.mu.Unlock()
	return nil, ErrVersionConflict
}

func TestOrchestratorQueuePolicySerializesSameScope(t *testing.T) {
	tracker := NewInMemoryTracker()
	processor := &queueSerialProcessor{
		firstStarted:  make(chan struct{}),
		secondStarted: make(chan struct{}),
		releaseFirst:  make(chan struct{}),
	}
	cfg := DefaultOrchestratorConfig()
	cfg.SupersedePolicy = SupersedePolicyQueue
	cfg.MaxConcurrent = 10
	orch := NewOrchestratorWithStores(cfg, processor, tracker, NewInMemoryOutboxStore())
	orch.SetReady(true)

	firstDone := make(chan error, 1)
	go func() {
		_, err := orch.Process(context.Background(), &ProcessRequest{
			UserID:         "user-queue",
			CharacterID:    "char-queue",
			ConversationID: "conv-queue",
			Channel:        "web",
			Source:         "web",
			Message:        "first",
			RequestID:      "queue-1",
		})
		firstDone <- err
	}()

	select {
	case <-processor.firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first processor call did not start")
	}

	secondDone := make(chan error, 1)
	go func() {
		_, err := orch.Process(context.Background(), &ProcessRequest{
			UserID:         "user-queue",
			CharacterID:    "char-queue",
			ConversationID: "conv-queue",
			Channel:        "web",
			Source:         "web",
			Message:        "second",
			RequestID:      "queue-2",
		})
		secondDone <- err
	}()

	waitForRecordCreated(t, tracker, "queue-2")
	select {
	case <-processor.secondStarted:
		t.Fatal("second processor call started before first completed")
	case <-time.After(100 * time.Millisecond):
	}

	close(processor.releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	select {
	case <-processor.secondStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("second processor call did not start after first completed")
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
	if got := processor.max.Load(); got != 1 {
		t.Fatalf("queue policy allowed concurrent processors: max=%d", got)
	}
}

func TestOrchestratorQueueTurnWaitsForOlderQueuedRecord(t *testing.T) {
	tracker := NewInMemoryTracker()
	cfg := DefaultOrchestratorConfig()
	cfg.SupersedePolicy = SupersedePolicyQueue
	orch := NewOrchestratorWithStores(cfg, &runtimeCaptureProcessor{}, tracker, NewInMemoryOutboxStore())
	scope := InteractionScope{
		UserID:         "user-queued-order",
		CharacterID:    "char-queued-order",
		ConversationID: "conv-queued-order",
		Channel:        "web",
		Source:         "web",
	}.Normalize()
	older := NewInteractionRecord(scope)
	older.CreatedAt = time.Now().Add(-time.Second)
	if err := tracker.Create(context.Background(), older); err != nil {
		t.Fatal(err)
	}
	older, err := tracker.TransitionCAS(context.Background(), older.ID, older.StatusVersion, InteractionStatusQueued)
	if err != nil {
		t.Fatal(err)
	}
	newer := NewInteractionRecord(scope)
	newer.CreatedAt = time.Now()
	if err := tracker.Create(context.Background(), newer); err != nil {
		t.Fatal(err)
	}
	newer, err = tracker.TransitionCAS(context.Background(), newer.ID, newer.StatusVersion, InteractionStatusQueued)
	if err != nil {
		t.Fatal(err)
	}

	released := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := orch.waitForQueueTurn(context.Background(), scope, newer)
		done <- err
		close(released)
	}()

	select {
	case <-released:
		t.Fatal("newer queued record advanced before older queued record finished")
	case <-time.After(100 * time.Millisecond):
	}
	if _, err := tracker.Fail(context.Background(), older.ID, older.StatusVersion, "test_done", "done"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("newer queued record did not advance after older queued record finished")
	}
}

func waitForRecordCreated(t *testing.T, tracker InteractionTracker, requestID string) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		found := false
		err := tracker.Range(context.Background(), func(record *InteractionRecord) bool {
			if record.Scope.RequestID == requestID {
				found = true
				return false
			}
			return true
		})
		if err != nil {
			t.Fatal(err)
		}
		if found {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("record %s was not created", requestID)
		case <-tick.C:
		}
	}
}

func waitForRecordStatus(t *testing.T, tracker InteractionTracker, requestID string, status InteractionStatus) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	lastStatus := InteractionStatus("")
	for {
		matched := false
		ready := false
		err := tracker.Range(context.Background(), func(record *InteractionRecord) bool {
			if record.Scope.RequestID != requestID {
				return true
			}
			matched = true
			lastStatus = record.Status
			ready = record.Status == status
			return !ready
		})
		if err != nil {
			t.Fatal(err)
		}
		if matched && ready {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("record %s did not reach status %s, last status %s matched %v", requestID, status, lastStatus, matched)
		case <-tick.C:
		}
	}
}

func TestOrchestratorSuccessReturnDoesNotCompleteWhenInteractionDrifts(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(ctx context.Context, tracker *InMemoryTracker, req *ProcessRequest) error
		wantStatus  InteractionStatus
		wantOutcome Outcome
		wantErr     error
	}{
		{
			name: "cancel_requested",
			mutate: func(ctx context.Context, tracker *InMemoryTracker, req *ProcessRequest) error {
				return tracker.RequestCancel(ctx, req.InteractionID, "test_cancel")
			},
			wantStatus:  InteractionStatusCancelled,
			wantOutcome: OutcomeCancelled,
			wantErr:     ErrOrchestratorCancelled,
		},
		{
			name: "superseded",
			mutate: func(ctx context.Context, tracker *InMemoryTracker, req *ProcessRequest) error {
				scope := InteractionScope{
					UserID:         req.UserID,
					CharacterID:    req.CharacterID,
					ConversationID: req.ConversationID,
					Channel:        req.Channel,
					PeerID:         req.PeerID,
					SessionID:      req.SessionID,
					Source:         req.Source,
					RequestID:      "superseder-" + req.RequestID,
				}.Normalize()
				superseder := NewInteractionRecord(scope)
				if err := tracker.Create(ctx, superseder); err != nil {
					return err
				}
				return tracker.MarkSuperseded(ctx, req.InteractionID, superseder.ID)
			},
			wantStatus:  InteractionStatusSuperseded,
			wantOutcome: OutcomeSuperseded,
			wantErr:     ErrOrchestratorSuperseded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewInMemoryTracker()
			outbox := NewInMemoryOutboxStore()
			processor := postSuccessDriftProcessor{
				mutate: func(ctx context.Context, req *ProcessRequest) error {
					return tt.mutate(ctx, tracker, req)
				},
			}
			orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), processor, tracker, outbox)
			orch.SetReady(true)

			result, err := orch.Process(context.Background(), &ProcessRequest{
				UserID:         "user-drift",
				CharacterID:    "char-drift",
				ConversationID: "conv-drift",
				Channel:        "web",
				Source:         "web",
				Message:        "hello",
				RequestID:      "req-" + tt.name,
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
			if result == nil || result.Outcome != tt.wantOutcome {
				t.Fatalf("unexpected result: %#v", result)
			}
			stored, ok, getErr := tracker.Get(context.Background(), result.InteractionID)
			if getErr != nil {
				t.Fatal(getErr)
			}
			if !ok {
				t.Fatal("interaction record missing")
			}
			if stored.Status != tt.wantStatus {
				t.Fatalf("expected status %s, got %#v", tt.wantStatus, stored)
			}
			records, listErr := outbox.ListPending()
			if listErr != nil {
				t.Fatal(listErr)
			}
			if len(records) != 0 {
				t.Fatalf("outbox records should not be appended after %s drift: %#v", tt.name, records)
			}
		})
	}
}

func TestOrchestratorSuccessReturnDoesNotCompleteWhenStatusVersionChanges(t *testing.T) {
	db := newTestInteractionDB(t)
	tracker := NewSQLiteInteractionTracker(db)
	if err := tracker.InitSchema(); err != nil {
		t.Fatal(err)
	}
	outbox := NewSQLiteOutboxStore(db)
	if err := outbox.InitSchema(); err != nil {
		t.Fatal(err)
	}
	processor := postSuccessDriftProcessor{
		mutate: func(ctx context.Context, req *ProcessRequest) error {
			return db.WithContext(ctx).Model(&InteractionRecordModel{}).
				Where("id = ?", req.InteractionID).
				Update("status_version", gorm.Expr("status_version + 1")).Error
		},
	}
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), processor, tracker, outbox)
	orch.SetReady(true)

	result, err := orch.Process(context.Background(), &ProcessRequest{
		UserID:         "user-version-drift",
		CharacterID:    "char-version-drift",
		ConversationID: "conv-version-drift",
		Channel:        "web",
		Source:         "web",
		Message:        "hello",
		RequestID:      "req-version-drift",
	})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got result=%#v err=%v", result, err)
	}
	if result == nil || result.Outcome == OutcomeCompleted {
		t.Fatalf("version drift should not complete interaction: %#v", result)
	}
	stored, ok, getErr := tracker.Get(context.Background(), result.InteractionID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if !ok {
		t.Fatal("interaction record missing")
	}
	if stored.Status == InteractionStatusCompleted {
		t.Fatalf("version drift should not complete stored interaction: %#v", stored)
	}
	records, listErr := outbox.ListPending()
	if listErr != nil {
		t.Fatal(listErr)
	}
	if len(records) != 0 {
		t.Fatalf("outbox records should not be appended after version drift: %#v", records)
	}
}

func TestOrchestratorMapsFinalCompleteCancelConflict(t *testing.T) {
	base := NewInMemoryTracker()
	tracker := &completeConflictTracker{InteractionTracker: base, base: base, status: InteractionStatusCancelled}
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), &runtimeCaptureProcessor{}, tracker, NewInMemoryOutboxStore())
	orch.SetReady(true)

	result, err := orch.Process(context.Background(), &ProcessRequest{
		UserID:         "user-final-cancel",
		CharacterID:    "char-final-cancel",
		ConversationID: "conv-final-cancel",
		Channel:        "web",
		Source:         "web",
		Message:        "hello",
		RequestID:      "req-final-cancel",
	})
	if !errors.Is(err, ErrOrchestratorCancelled) {
		t.Fatalf("expected cancel outcome, got result=%#v err=%v", result, err)
	}
	if result == nil || result.Outcome != OutcomeCancelled || result.Response != nil {
		t.Fatalf("cancel conflict should not return success response: %#v", result)
	}
	stored, ok, getErr := tracker.Get(context.Background(), result.InteractionID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if !ok || stored.Status != InteractionStatusCancelled || stored.ResultRef != "" {
		t.Fatalf("cancel conflict mutated incorrectly: ok=%v record=%#v", ok, stored)
	}
}

func TestOrchestratorCancelIgnoresCompletedTransitionRace(t *testing.T) {
	base := NewInMemoryTracker()
	tracker := &cancelTransitionConflictTracker{InteractionTracker: base, base: base}
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), &runtimeCaptureProcessor{}, tracker, NewInMemoryOutboxStore())
	record := NewInteractionRecord(InteractionScope{
		UserID:         "user-cancel-race",
		CharacterID:    "char-cancel-race",
		ConversationID: "conv-cancel-race",
		Channel:        "web",
		Source:         "web",
		RequestID:      "req-cancel-race",
	}.Normalize())
	if err := tracker.Create(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	processing, err := tracker.TransitionCAS(context.Background(), record.ID, record.StatusVersion, InteractionStatusProcessing)
	if err != nil {
		t.Fatal(err)
	}
	contextReady, err := tracker.TransitionCAS(context.Background(), processing.ID, processing.StatusVersion, InteractionStatusContextReady)
	if err != nil {
		t.Fatal(err)
	}

	if err := orch.Cancel(contextReady.ID); err != nil {
		t.Fatalf("completed transition race should be idempotent, got %v", err)
	}
	stored, ok, getErr := tracker.Get(context.Background(), contextReady.ID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if !ok || stored.Status != InteractionStatusCompleted || stored.ResultRef != "completed_elsewhere" {
		t.Fatalf("unexpected final record after cancel race: ok=%v record=%#v", ok, stored)
	}
}

func TestOrchestratorLatestSupersedeExcludesCurrentSQLiteRecord(t *testing.T) {
	tracker := newTestSQLiteInteractionTracker(t)
	processor := &supersedeSQLiteProcessor{firstStarted: make(chan struct{})}
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), processor, tracker, NewInMemoryOutboxStore())
	orch.SetReady(true)

	firstDone := make(chan *OrchestrationResult, 1)
	firstErr := make(chan error, 1)
	go func() {
		result, err := orch.Process(context.Background(), &ProcessRequest{
			UserID:         "user-1",
			CharacterID:    "char-1",
			ConversationID: "conv-1",
			Channel:        "web",
			Source:         "web",
			Message:        "old",
			RequestID:      "request-old",
		})
		firstDone <- result
		firstErr <- err
	}()

	select {
	case <-processor.firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first processor call did not start")
	}

	second, err := orch.Process(context.Background(), &ProcessRequest{
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		Channel:        "web",
		Source:         "web",
		Message:        "new",
		RequestID:      "request-new",
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Outcome != OutcomeCompleted {
		t.Fatalf("expected second interaction completed, got %#v", second)
	}

	var first *OrchestrationResult
	select {
	case first = <-firstDone:
	case <-time.After(2 * time.Second):
		t.Fatal("first interaction did not finish after supersede")
	}
	if err := <-firstErr; err == nil {
		t.Fatal("expected first interaction to be cancelled by supersede")
	}
	if first == nil || first.InteractionID == "" {
		t.Fatalf("first result missing interaction id: %#v", first)
	}
	oldRecord, ok, err := tracker.Get(context.Background(), first.InteractionID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("old interaction missing")
	}
	if oldRecord.Status != InteractionStatusSuperseded || oldRecord.SupersededByID != second.InteractionID {
		t.Fatalf("old interaction was not superseded by second: %#v", oldRecord)
	}
}

func TestOrchestratorCompletedEventFallbackUsesSQLiteTransaction(t *testing.T) {
	db := newTestInteractionDB(t)
	tracker := NewSQLiteInteractionTracker(db)
	if err := tracker.InitSchema(); err != nil {
		t.Fatal(err)
	}
	outbox := NewSQLiteOutboxStore(db)
	if err := outbox.InitSchema(); err != nil {
		t.Fatal(err)
	}
	processor := &runtimeCaptureProcessor{}
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), processor, tracker, outbox)
	orch.SetReady(true)

	result, err := orch.Process(context.Background(), &ProcessRequest{
		UserID:         "user-events",
		CharacterID:    "char-events",
		ConversationID: "conv-events",
		Channel:        "web",
		Source:         "web",
		Message:        "hello",
		RequestID:      "req-empty-events",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomeCompleted {
		t.Fatalf("expected completed outcome, got %#v", result)
	}
	if len(result.Events) != 3 {
		t.Fatalf("expected fallback completed events, got %#v", result.Events)
	}
	records, err := outbox.ListPending()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 pending outbox records, got %#v", records)
	}
	gotCompleted := false
	for _, record := range records {
		if record.EventType == "interaction.completed" {
			gotCompleted = true
		}
	}
	if !gotCompleted {
		t.Fatalf("completed outbox event missing: %#v", records)
	}
	stored, ok, err := tracker.Get(context.Background(), result.InteractionID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || stored.Status != InteractionStatusCompleted {
		t.Fatalf("interaction should be completed with fallback events: ok=%v record=%#v", ok, stored)
	}
}

func TestOrchestratorCompletedEventAppendFailureRollsBackSQLiteComplete(t *testing.T) {
	db := newTestInteractionDB(t)
	tracker := NewSQLiteInteractionTracker(db)
	if err := tracker.InitSchema(); err != nil {
		t.Fatal(err)
	}
	outbox := NewSQLiteOutboxStore(db)
	processor := &runtimeCaptureProcessor{}
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), processor, tracker, outbox)
	orch.SetReady(true)

	result, err := orch.Process(context.Background(), &ProcessRequest{
		UserID:         "user-append-fail",
		CharacterID:    "char-append-fail",
		ConversationID: "conv-append-fail",
		Channel:        "web",
		Source:         "web",
		Message:        "hello",
		RequestID:      "req-append-fail",
	})
	if err == nil {
		t.Fatal("expected outbox append failure")
	}
	if result == nil || result.InteractionID == "" {
		t.Fatalf("failed result should keep interaction id: %#v", result)
	}
	stored, ok, getErr := tracker.Get(context.Background(), result.InteractionID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if !ok {
		t.Fatal("interaction record missing")
	}
	if stored.Status == InteractionStatusCompleted {
		t.Fatalf("complete should roll back when fallback outbox append fails: %#v", stored)
	}
	if stored.Status != InteractionStatusCommitted {
		t.Fatalf("expected record to remain committed after rollback, got %#v", stored)
	}
}

func newTestInteractionDB(t *testing.T) *gorm.DB {
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
	return db
}
