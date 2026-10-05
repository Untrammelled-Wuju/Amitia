package business

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type testSummaryModel struct {
	*testModel
	summarize func(context.Context, SummaryInference) (string, error)
	calls     int
}

func (m *testSummaryModel) GenerateOwnedSummary(ctx context.Context, inference SummaryInference) (string, error) {
	m.calls++
	if m.summarize != nil {
		return m.summarize(ctx, inference)
	}
	return "summary", nil
}

func TestOwnedSummaryUsesCoreComputeAndAcknowledgedDeviceOwnerWithoutAddingMessages(t *testing.T) {
	engine, db, service, base := engineHarness(t)
	model := &testSummaryModel{testModel: base}
	engine.model = model
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", ConversationID: "chat", RequestID: "turn", Message: "hello"}
	if _, err := engine.Run(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	query, err := engine.Query(t.Context(), request, coordination.DataQuery{ConversationID: "chat"})
	if err != nil {
		t.Fatal(err)
	}
	request.RequestID, request.ExpectedScope = "generate-summary", &query.Scope
	model.summarize = func(_ context.Context, inference SummaryInference) (string, error) {
		if inference.Scope.ResourceOwnerID != "a" || inference.Role.ID != "role" || len(inference.Messages) != 2 {
			t.Fatalf("wrong summary source: %+v", inference)
		}
		return "device-owned summary", nil
	}
	response, err := engine.GenerateSummary(t.Context(), request, 0)
	if err != nil || !response.Saved || response.OwnerID != "a" || response.Revision != 1 {
		t.Fatalf("summary failed: %+v %v", response, err)
	}
	if replay, err := engine.GenerateSummary(t.Context(), request, 0); err != nil || replay.Text != response.Text || model.calls != 1 {
		t.Fatalf("summary repeated compute: %+v %v", replay, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_device_owned_resources WHERE kind='message'`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("summary appended chat messages: %d %v", count, err)
	}
	core, err := coordination.NewOwnershipStore(db, "core").List(t.Context(), "summary", "role", false)
	if err != nil || len(core) != 0 {
		t.Fatal("device summary mirrored at Core")
	}
	if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.GenerateSummary(t.Context(), request, 0); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("stale summary page accepted: %v", err)
	}
}

func TestOwnedSummaryRejectsMessagesChangedDuringComputeAndNeverRetriesStartedRequest(t *testing.T) {
	engine, db, _, base := engineHarness(t)
	model := &testSummaryModel{testModel: base}
	engine.model = model
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", ConversationID: "chat", RequestID: "turn", Message: "hello"}
	if _, err := engine.Run(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	query, err := engine.Query(t.Context(), request, coordination.DataQuery{ConversationID: "chat"})
	if err != nil {
		t.Fatal(err)
	}
	request.RequestID, request.ExpectedScope = "summary-race", &query.Scope
	model.summarize = func(context.Context, SummaryInference) (string, error) {
		_, err := db.Exec(`UPDATE kernel_device_owned_resources SET revision=revision+1 WHERE kind='message'`)
		return "late summary", err
	}
	if response, err := engine.GenerateSummary(t.Context(), request, 0); !errors.Is(err, coordination.ErrResourceVersion) || response.Saved {
		t.Fatalf("stale summary saved: %+v %v", response, err)
	}
	if _, err := engine.GenerateSummary(t.Context(), request, 0); !errors.Is(err, ErrUncertainExecution) || model.calls != 1 {
		t.Fatalf("started request repeated inference: %v", err)
	}
	resource, err := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "summary", "chat/summary")
	if err != nil || resource != nil {
		t.Fatal("unconfirmed summary persisted")
	}
}

type summaryPagePort struct {
	testDataPort
	repeated bool
}

func (p summaryPagePort) Snapshot(_ context.Context, scope coordination.ExecutionScope, query coordination.DataQuery) (coordination.DataSnapshot, error) {
	id := "first"
	cursors := map[string]string{"message": "next"}
	if query.Cursor != "" {
		id = "second"
		if !p.repeated {
			cursors = nil
		}
	}
	return coordination.DataSnapshot{OwnerID: scope.ResourceOwnerID, Role: p.role, NextCursors: cursors, Resources: []coordination.Resource{{Kind: "message", ID: id, OwnerID: scope.ResourceOwnerID, RoleID: scope.RoleID, Revision: 1, Body: body(map[string]string{"conversationId": query.ConversationID, "role": "user", "content": "hello"})}}}, nil
}
func TestOwnedSummaryReadsAllPagesAndRejectsRepeatedCursors(t *testing.T) {
	engine, db, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", ConversationID: "chat", RoleID: "role"}
	query, err := engine.Query(t.Context(), request, coordination.DataQuery{})
	if err != nil {
		t.Fatal(err)
	}
	port := summaryPagePort{testDataPort: testDataPort{db: db, role: query.Snapshot.Role}}
	engine.data = port
	rows, dependencies, err := engine.summaryHistory(t.Context(), request, query.Scope)
	if err != nil || len(rows) != 2 || len(dependencies) != 2 {
		t.Fatalf("summary lost a page: %d %v", len(rows), err)
	}
	port.repeated = true
	engine.data = port
	if _, _, err := engine.summaryHistory(t.Context(), request, query.Scope); err == nil || !strings.Contains(err.Error(), "游标重复") {
		t.Fatalf("repeated summary cursor accepted: %v", err)
	}
}

func TestCoordinatedSummaryUsesDeviceHistoryAndSavesOnlyCoreContinuation(t *testing.T) {
	engine, db, service, base := engineHarness(t)
	engine.data = originDataPort{testDataPort{db: db, role: coordination.Role{ID: "role", Revision: 1, Profile: body(map[string]any{})}}}
	model := &testSummaryModel{testModel: base}
	engine.model = model
	for _, owner := range []string{"a", "core"} {
		store := coordination.NewOwnershipStore(db, owner)
		scope := coordination.ExecutionScope{ResourceOwnerID: owner, RoleOwnerID: owner, RoleID: "role", RequestID: "seed-" + owner}
		if _, err := store.Apply(t.Context(), coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{
			{Kind: "conversation", ID: "same-id", RoleID: "role", Body: body(map[string]any{"id": "same-id", "title": owner})},
			{Kind: "message", ID: "same-message", RoleID: "role", Body: body(map[string]any{"id": "same-message", "conversationId": "same-id", "role": "user", "content": owner})},
		}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
		t.Fatal(err)
	}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "same-id", RequestID: "summary-continuation", ConversationOrigin: &ConversationOrigin{OwnerID: "a", ID: "same-id"}}
	query, err := engine.Query(t.Context(), request, coordination.DataQuery{ConversationID: "same-id"})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &query.Scope
	model.summarize = func(_ context.Context, inference SummaryInference) (string, error) {
		if inference.Scope.ResourceOwnerID != "core" || len(inference.Messages) != 1 || inference.Messages[0].OwnerID != "a" || inference.Messages[0].Content != "a" {
			t.Fatalf("summary mixed same-name owners: %+v", inference)
		}
		return "historical device summary computed by Core", nil
	}
	response, err := engine.GenerateSummary(t.Context(), request, 0)
	if err != nil || !response.Saved || response.OwnerID != "core" || response.ConversationID == "same-id" || response.ResourceID != response.ConversationID+"/summary" {
		t.Fatalf("Core summary continuation failed: %+v %v", response, err)
	}
	device, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), "summary", "role", false)
	if err != nil || len(device) != 0 {
		t.Fatal("cloud summary mirrored to device")
	}
	for _, owner := range []string{"a", "core"} {
		original, err := coordination.NewOwnershipStore(db, owner).Get(t.Context(), "conversation", "same-id")
		if err != nil || original == nil || original.Revision != 1 {
			t.Fatalf("original conversation changed: %s %v", owner, err)
		}
	}
}
