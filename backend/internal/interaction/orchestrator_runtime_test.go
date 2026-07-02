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
