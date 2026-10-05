package business

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

type testDataPort struct {
	db   *sql.DB
	role coordination.Role
}

func (p testDataPort) Roles(context.Context, coordination.ExecutionScope) ([]coordination.Role, error) {
	return []coordination.Role{p.role}, nil
}
func (p testDataPort) Snapshot(ctx context.Context, scope coordination.ExecutionScope, _ coordination.DataQuery) (coordination.DataSnapshot, error) {
	snapshot := coordination.DataSnapshot{OwnerID: scope.ResourceOwnerID, Role: p.role}
	store := coordination.NewOwnershipStore(p.db, scope.ResourceOwnerID)
	for _, kind := range []string{"conversation", "message", "checkpoint", "memory", "fact", "vector", "graph", "working", "summary", "profile", "episodic"} {
		resources, err := store.List(ctx, kind, scope.RoleID, false)
		if err != nil {
			return snapshot, err
		}
		snapshot.Resources = append(snapshot.Resources, resources...)
	}
	return snapshot, nil
}
func (p testDataPort) Commit(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	return coordination.NewOwnershipStore(p.db, commit.Scope.ResourceOwnerID).Apply(ctx, commit)
}

func (p testDataPort) SaveInterrupted(ctx context.Context, reply coordination.InterruptedReply) error {
	return coordination.NewOwnershipStore(p.db, reply.Scope.ResourceOwnerID).SaveInterrupted(ctx, reply)
}

func (p testDataPort) Resource(ctx context.Context, scope coordination.ExecutionScope, kind, id string) (*coordination.Resource, error) {
	resource, err := coordination.NewOwnershipStore(p.db, scope.ResourceOwnerID).Get(ctx, kind, id)
	if resource != nil && resource.RoleID != scope.RoleID {
		return nil, coordination.ErrWrongOwner
	}
	return resource, err
}

type testModel struct {
	generate    func(context.Context, Inference) (Generation, error)
	extract     func(context.Context, Inference, Generation) ([]DerivedMemory, error)
	calls       atomic.Int32
	extractions atomic.Int32
}

func (m *testModel) GenerateOwnedReply(ctx context.Context, inference Inference) (Generation, error) {
	m.calls.Add(1)
	if m.generate != nil {
		return m.generate(ctx, inference)
	}
	return Generation{Text: "reply", Tokens: 10}, nil
}
func (m *testModel) ExtractOwnedMemory(ctx context.Context, inference Inference, generation Generation) ([]DerivedMemory, error) {
	m.extractions.Add(1)
	if m.extract != nil {
		return m.extract(ctx, inference, generation)
	}
	return []DerivedMemory{{Kind: "fact", Key: "favorite-drink", Body: json.RawMessage(`{"value":"tea"}`)}, {Kind: "vector", Key: "favorite-drink", Body: json.RawMessage(`{"values":[0.1,0.2]}`)}, {Kind: "graph", Key: "favorite-drink", Body: json.RawMessage(`{"relation":"likes","target":"tea"}`)}, {Kind: "working", Key: "current", Body: json.RawMessage(`{"summary":"talked about tea"}`)}}, nil
}

func engineHarness(t *testing.T) (*Engine, *sql.DB, *coordination.Service, *testModel) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := kernelsqlite.Migrate(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES('a','core','trusted','now','now')`); err != nil {
		t.Fatal(err)
	}
	service := coordination.NewService(db)
	model := &testModel{}
	data := testDataPort{db: db, role: coordination.Role{ID: "role", Revision: 1, Profile: json.RawMessage(`{"characterId":"role","name":"device-role"}`)}}
	return NewEngine(service, data, model), db, service, model
}

func TestOwnedConversationAndAllDerivedDataAreSavedOnlyByChosenOwner(t *testing.T) {
	engine, db, service, model := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", ConversationID: "conversation", RequestID: "request", Message: "I like tea"}
	response, err := engine.Run(t.Context(), request)
	if err != nil || !response.Saved || response.MemoryStatus != "saved" || response.Scope.ResourceOwnerID != "a" {
		t.Fatalf("response=%+v err=%v", response, err)
	}
	for _, kind := range []string{"message", "memory", "fact", "vector", "graph", "working"} {
		items, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), kind, "role", false)
		if err != nil || len(items) == 0 {
			t.Fatalf("device %s: %v %v", kind, items, err)
		}
		mirrored, err := coordination.NewOwnershipStore(db, "core").List(t.Context(), kind, "role", false)
		if err != nil || len(mirrored) != 0 {
			t.Fatalf("Core mirror exists: %s %v %v", kind, mirrored, err)
		}
	}
	replayed, err := engine.Run(t.Context(), request)
	if err != nil || replayed.Text != "reply" || replayed.MemoryStatus != "saved" || model.calls.Load() != 1 {
		t.Fatalf("request replay called model again: %+v %v %d", replayed, err, model.calls.Load())
	}
	if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
		t.Fatal(err)
	}
	request.RequestID = "core-request"
	response, err = engine.Run(t.Context(), request)
	if err != nil || response.Scope.ResourceOwnerID != "core" {
		t.Fatalf("coordinated response=%+v err=%v", response, err)
	}
	coreMessages, err := coordination.NewOwnershipStore(db, "core").List(t.Context(), "message", "role", false)
	if err != nil || len(coreMessages) != 2 {
		t.Fatalf("Core data missing: %v %v", coreMessages, err)
	}
	deviceMessages, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), "message", "role", false)
	if err != nil || len(deviceMessages) != 2 {
		t.Fatalf("old device data migrated: %v %v", deviceMessages, err)
	}
}

func TestModeChangeDuringGenerationRejectsLateReplyAndNeverReplaysUncertainWork(t *testing.T) {
	engine, db, service, model := engineHarness(t)
	model.generate = func(ctx context.Context, _ Inference) (Generation, error) {
		_, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role")
		if err != nil {
			t.Fatal(err)
		}
		return Generation{Text: "late reply"}, nil
	}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", ConversationID: "conversation", RequestID: "request", Message: "hello"}
	if _, err := engine.Run(t.Context(), request); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("late response accepted: %v", err)
	}
	for _, owner := range []string{"a", "core"} {
		messages, err := coordination.NewOwnershipStore(db, owner).List(t.Context(), "message", "role", false)
		if err != nil {
			t.Fatal(err)
		}
		for _, message := range messages {
			var payload map[string]any
			_ = json.Unmarshal(message.Body, &payload)
			if payload["role"] == "assistant" {
				t.Fatalf("late reply persisted on %s", owner)
			}
		}
	}
	if _, err := service.ChangeMode(t.Context(), "core", "a", 2, false, "role"); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Run(t.Context(), request); !errors.Is(err, ErrUncertainExecution) {
		t.Fatalf("uncertain work was replayed: %v", err)
	}
	if model.calls.Load() != 1 {
		t.Fatal("uncertain tools/model ran again")
	}
}

func TestCompletedRequestCannotRunAgainAfterOwnerSwitch(t *testing.T) {
	engine, _, service, model := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "stable-request", Message: "hello"}
	first, err := engine.Run(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := engine.Run(t.Context(), request)
	if err != nil || replay.ConversationID != first.ConversationID || model.calls.Load() != 1 {
		t.Fatalf("unstable new conversation retry: %+v %v", replay, err)
	}
	if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Run(t.Context(), request); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("old request crossed owner boundary: %v", err)
	}
	if model.calls.Load() != 1 {
		t.Fatal("completed request ran again on new owner")
	}
}

func TestCrossDeviceAIRequiresTargetGrantAndUsesTargetMode(t *testing.T) {
	engine, db, service, model := engineHarness(t)
	if _, err := db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES('b','core','trusted','now','now')`); err != nil {
		t.Fatal(err)
	}
	request := Request{SpaceID: "core", DeviceID: "a", TargetDeviceID: "b", CoreID: "core", RequestID: "cross-device", Message: "hello"}
	if _, err := engine.Run(t.Context(), request); !errors.Is(err, coordination.ErrCapabilityGrant) {
		t.Fatalf("ungranted cross-device call accepted: %v", err)
	}
	if _, err := service.SetCapabilityGrant(t.Context(), "core", "a", "b", "ai.chat", 0, true); err != nil {
		t.Fatal(err)
	}
	response, err := engine.Run(t.Context(), request)
	if err != nil || response.Scope.ResourceOwnerID != "b" || model.calls.Load() != 1 {
		t.Fatalf("target policy not applied: %+v %v", response, err)
	}
	if _, err := service.SetCapabilityGrant(t.Context(), "core", "a", "b", "ai.chat", 1, false); err != nil {
		t.Fatal(err)
	}
	request.RequestID = "cross-revoked"
	if _, err := engine.Run(t.Context(), request); !errors.Is(err, coordination.ErrCapabilityGrant) {
		t.Fatalf("revoked grant accepted: %v", err)
	}
}

func TestMemoryRetryDoesNotGenerateReplyAgainAndPurgesCompletedPlan(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	model.extract = func(context.Context, Inference, Generation) ([]DerivedMemory, error) {
		return nil, errors.New("embedding unavailable")
	}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "retry-memory", Message: "hello"}
	first, err := engine.Run(t.Context(), request)
	if err != nil || !first.Saved || first.MemoryStatus != "failed" {
		t.Fatalf("failed memory lost completed reply: %+v %v", first, err)
	}
	model.extract = nil
	retried, err := engine.Run(t.Context(), request)
	if err != nil || retried.MemoryStatus != "saved" || retried.TurnID != first.TurnID || model.calls.Load() != 1 || model.extractions.Load() != 2 {
		t.Fatalf("memory retry replayed reply: %+v %v calls=%d extracts=%d", retried, err, model.calls.Load(), model.extractions.Load())
	}
	checkpoints, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), "checkpoint", "role", true)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, checkpoint := range checkpoints {
		if checkpoint.ID == "memory/retry-memory" {
			found = true
			if !checkpoint.Deleted || string(checkpoint.Body) != "null" {
				t.Fatal("completed memory payload retained")
			}
		}
	}
	if !found {
		t.Fatal("memory plan did not use durable owner checkpoint")
	}
}

func TestMemoryRecoveryAfterRestartLoadsOwnerDataWithoutReplayingReply(t *testing.T) {
	engine, db, service, model := engineHarness(t)
	model.extract = func(context.Context, Inference, Generation) ([]DerivedMemory, error) {
		return nil, errors.New("temporary failure")
	}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "recover-memory", Message: "hello"}
	first, err := engine.Run(t.Context(), request)
	if err != nil || first.MemoryStatus != "failed" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	if _, err := db.Exec(`UPDATE kernel_device_memory_jobs SET next_attempt_at=0`); err != nil {
		t.Fatal(err)
	}
	jobs, err := service.PendingMemoryJobs(t.Context())
	if err != nil || len(jobs) != 1 {
		t.Fatalf("durable job missing: %+v %v", jobs, err)
	}
	restartedModel := &testModel{}
	restarted := NewEngine(service, engine.data, restartedModel)
	response, err := restarted.ResumeMemory(t.Context(), jobs[0])
	if err != nil || response.MemoryStatus != "saved" || restartedModel.calls.Load() != 0 || restartedModel.extractions.Load() != 1 {
		t.Fatalf("recovery repeated reply or missed memory: %+v %v", response, err)
	}
	jobs, err = service.PendingMemoryJobs(t.Context())
	if err != nil || len(jobs) != 0 {
		t.Fatalf("completed job remains: %+v %v", jobs, err)
	}
}

func TestStreamCutoverSavesOnlyAcceptedPartialOnOriginalOwner(t *testing.T) {
	engine, db, service, model := engineHarness(t)
	model.generate = func(ctx context.Context, inference Inference) (Generation, error) {
		if err := inference.Emit(Event{Type: "delta", Text: "accepted partial"}); err != nil {
			return Generation{}, err
		}
		if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
			t.Fatal(err)
		}
		if err := inference.Emit(Event{Type: "delta", Text: "late text"}); !errors.Is(err, coordination.ErrScopeExpired) {
			t.Fatalf("late event emitted: %v", err)
		}
		return Generation{Text: "accepted partial", Partial: true}, context.Cause(ctx)
	}
	events := make([]Event, 0)
	response, err := engine.RunEvents(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "stream-cutover", Message: "hello"}, func(event Event) error { events = append(events, event); return nil })
	if !errors.Is(err, coordination.ErrScopeExpired) || !response.Interrupted || !response.Saved {
		t.Fatalf("interrupt result=%+v %v", response, err)
	}
	if len(events) != 2 || events[1].Text != "accepted partial" {
		t.Fatalf("invalid event stream: %+v", events)
	}
	deviceMessages, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), "message", "role", false)
	if err != nil || len(deviceMessages) != 2 {
		t.Fatalf("device partial missing: %+v %v", deviceMessages, err)
	}
	var partial struct {
		Content string `json:"content"`
		Status  string `json:"status"`
	}
	for _, message := range deviceMessages {
		if message.ID == "stream-cutover/assistant" {
			if err := json.Unmarshal(message.Body, &partial); err != nil {
				t.Fatal(err)
			}
		}
	}
	if partial.Content != "accepted partial" || partial.Status != "interrupted" {
		t.Fatalf("late or completed partial: %+v", partial)
	}
	coreMessages, err := coordination.NewOwnershipStore(db, "core").List(t.Context(), "message", "role", false)
	if err != nil || len(coreMessages) != 0 {
		t.Fatalf("partial followed new owner: %+v %v", coreMessages, err)
	}
}

func TestExplicitStopCancelsOwnedGeneration(t *testing.T) {
	engine, _, _, model := engineHarness(t)
	model.generate = func(ctx context.Context, inference Inference) (Generation, error) {
		if !engine.Interrupt("core", "a", "stop-request") {
			t.Fatal("active request missing")
		}
		return Generation{}, context.Cause(ctx)
	}
	response, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "stop-request", Message: "hello"})
	if !errors.Is(err, ErrInterrupted) || !response.Interrupted || !response.Saved {
		t.Fatalf("stop not persisted: %+v %v", response, err)
	}
	if engine.Interrupt("core", "a", "stop-request") {
		t.Fatal("finished request remains active")
	}
}
