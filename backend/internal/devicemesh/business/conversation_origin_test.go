package business

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type originDataPort struct{ testDataPort }

func (p originDataPort) Snapshot(ctx context.Context, scope coordination.ExecutionScope, query coordination.DataQuery) (coordination.DataSnapshot, error) {
	snapshot, err := p.testDataPort.Snapshot(ctx, scope, query)
	if err != nil || query.ConversationID == "" {
		return snapshot, err
	}
	resources := []coordination.Resource{}
	for _, resource := range snapshot.Resources {
		if resource.Kind == "conversation" && resource.ID != query.ConversationID {
			continue
		}
		if resource.Kind == "message" || resource.Kind == "summary" {
			var document struct {
				ConversationID string `json:"conversationId"`
			}
			if json.Unmarshal(resource.Body, &document) != nil || document.ConversationID != query.ConversationID {
				continue
			}
		}
		resources = append(resources, resource)
	}
	snapshot.Resources = resources
	return snapshot, nil
}

func (p originDataPort) HistoricalSnapshot(ctx context.Context, scope coordination.ExecutionScope, query coordination.DataQuery) (*coordination.DataSnapshot, error) {
	scope.ResourceOwnerID, scope.RoleOwnerID = scope.TargetDeviceID, scope.TargetDeviceID
	snapshot, err := p.Snapshot(ctx, scope, query)
	return &snapshot, err
}

func TestOwnedConversationOriginContinuesHistoryWithoutOverwritingSameIDAtCore(t *testing.T) {
	engine, db, service, model := engineHarness(t)
	engine.data = originDataPort{testDataPort{db: db, role: coordination.Role{ID: "role", Revision: 1, Profile: json.RawMessage(`{}`)}}}
	for _, owner := range []string{"a", "core"} {
		store := coordination.NewOwnershipStore(db, owner)
		scope := coordination.ExecutionScope{ResourceOwnerID: owner, RoleOwnerID: owner, RoleID: "role", RequestID: "seed-" + owner}
		if _, err := store.Apply(t.Context(), coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{
			{Kind: "conversation", ID: "same-id", RoleID: "role", Body: body(map[string]any{"id": "same-id", "title": owner})},
			{Kind: "message", ID: "seed-message", RoleID: "role", Body: body(map[string]any{"id": "seed-message", "conversationId": "same-id", "role": "user", "content": owner})},
		}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
		t.Fatal(err)
	}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "same-id", RequestID: "ambiguous", Message: "continue"}
	if _, err := engine.Query(t.Context(), request, coordination.DataQuery{ConversationID: "same-id"}); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("ambiguous query merged owners: %v", err)
	}
	if _, err := engine.Run(t.Context(), request); !errors.Is(err, coordination.ErrRequestConflict) || model.calls.Load() != 0 {
		t.Fatalf("ambiguous inference generated a reply: %v", err)
	}
	request.ConversationOrigin = &ConversationOrigin{OwnerID: "core", ID: "same-id"}
	current, err := engine.Query(t.Context(), request, coordination.DataQuery{ConversationID: "same-id"})
	if err != nil || current.HistoricalSnapshot != nil {
		t.Fatalf("Core conversation included same-id device history: %+v %v", current, err)
	}
	request.ConversationOrigin = &ConversationOrigin{OwnerID: "a", ID: "same-id"}
	request.RequestID = "continue-device-history"
	model.generate = func(_ context.Context, inference Inference) (Generation, error) {
		if inference.ConversationID == "same-id" || inference.HistoricalSnapshot == nil || inference.HistoricalSnapshot.OwnerID != "a" {
			t.Fatal("history did not receive a separate continuation")
		}
		for _, resource := range inference.Snapshot.Resources {
			if resource.Kind == "message" && resource.ID == "seed-message" {
				t.Fatal("unrelated Core conversation entered history continuation")
			}
		}
		return Generation{Text: "continued"}, nil
	}
	response, err := engine.Run(t.Context(), request)
	if err != nil || !response.Saved || response.ConversationID == "same-id" || response.ConversationOrigin == nil || *response.ConversationOrigin != *request.ConversationOrigin {
		t.Fatalf("continuation failed: %+v %v", response, err)
	}
	for _, owner := range []string{"a", "core"} {
		original, err := coordination.NewOwnershipStore(db, owner).Get(t.Context(), "conversation", "same-id")
		if err != nil || original == nil || original.Revision != 1 {
			t.Fatalf("original %s conversation was rewritten: %+v %v", owner, original, err)
		}
	}
	continuation, err := coordination.NewOwnershipStore(db, "core").Get(t.Context(), "conversation", response.ConversationID)
	if err != nil || continuation == nil {
		t.Fatal("continuation did not persist at Core")
	}
	var document struct {
		Origin ConversationOrigin `json:"conversationOrigin"`
	}
	if json.Unmarshal(continuation.Body, &document) != nil || document.Origin != *request.ConversationOrigin {
		t.Fatal("persisted continuation lost explicit lineage")
	}
}

func TestConversationOriginSeparatesDeviceHistoryFromCoreNamespace(t *testing.T) {
	scope := coordination.ExecutionScope{CoreID: "core", RoleID: "role", TargetDeviceID: "device", ResourceOwnerID: "core", Coordinated: true}
	origin := &ConversationOrigin{OwnerID: "device", ID: "same-id"}
	device, err := resolveConversationRoute(scope, "same-id", origin)
	if err != nil || device.CurrentID == "same-id" || device.HistoricalID != "same-id" {
		t.Fatalf("history overwrote current Core conversation: %+v %v", device, err)
	}
	retry, err := resolveConversationRoute(scope, "same-id", origin)
	if err != nil || retry.CurrentID != device.CurrentID {
		t.Fatal("continuation identity changed between requests")
	}
	core, err := resolveConversationRoute(scope, "same-id", &ConversationOrigin{OwnerID: "core", ID: "same-id"})
	if err != nil || core.CurrentID != "same-id" || core.HistoricalID != "" {
		t.Fatal("Core conversation read unrelated same-id device history")
	}
	for _, change := range []func(*coordination.ExecutionScope){
		func(s *coordination.ExecutionScope) { s.CoreID, s.ResourceOwnerID = "replacement", "replacement" },
		func(s *coordination.ExecutionScope) { s.RoleID = "other-role" },
	} {
		changed := scope
		change(&changed)
		route, err := resolveConversationRoute(changed, "same-id", origin)
		if err != nil || route.CurrentID == device.CurrentID {
			t.Fatal("continuation crossed Core or role namespace")
		}
	}
	for _, origin := range []*ConversationOrigin{{OwnerID: "other", ID: "same-id"}, {OwnerID: "device", ID: "different"}, {OwnerID: "", ID: "same-id"}, {OwnerID: "device\x00copied", ID: "same-id"}} {
		if _, err := resolveConversationRoute(scope, "same-id", origin); !errors.Is(err, coordination.ErrWrongOwner) {
			t.Fatalf("unbound or mismatched origin accepted: %+v %v", origin, err)
		}
	}
	scope.Coordinated, scope.ResourceOwnerID = false, "device"
	local, err := resolveConversationRoute(scope, "same-id", &ConversationOrigin{OwnerID: "device", ID: "same-id"})
	if err != nil || local.CurrentID != "same-id" || local.HistoricalID != "" {
		t.Fatal("device-managed conversation changed identity")
	}
	if _, err := resolveConversationRoute(scope, "same-id", &ConversationOrigin{OwnerID: "core", ID: "same-id"}); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatal("device mode accepted Core-owned conversation as local")
	}
}

func TestConversationWithoutOriginRejectsAmbiguousOwners(t *testing.T) {
	current := coordination.DataSnapshot{OwnerID: "core", Resources: []coordination.Resource{{Kind: "conversation", ID: "same-id"}}}
	historical := coordination.DataSnapshot{OwnerID: "device", LegacyConversations: []json.RawMessage{json.RawMessage(`{"id":"same-id"}`)}}
	route := conversationRoute{CurrentID: "same-id", HistoricalID: "same-id"}
	if err := rejectAmbiguousConversation(route, current, &historical); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatal("same-id unrelated conversations were merged")
	}
	route.Origin = &ConversationOrigin{OwnerID: "device", ID: "same-id"}
	if err := rejectAmbiguousConversation(route, current, &historical); err != nil {
		t.Fatal(err)
	}
	route.Origin = nil
	historical.LegacyConversations = []json.RawMessage{json.RawMessage(`{"id":"other"}`)}
	if err := rejectAmbiguousConversation(route, current, &historical); err != nil {
		t.Fatal("unambiguous old conversation was rejected")
	}
}

func TestCoreCutoverContinuesOnlyForwardedContextWithoutReadingFormerCore(t *testing.T) {
	engine, db, service, model := engineHarness(t)
	engine.data = originDataPort{testDataPort{db: db, role: coordination.Role{ID: "role", Revision: 1, Profile: json.RawMessage(`{}`)}}}
	if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
		t.Fatal(err)
	}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "old-conversation", ConversationOrigin: &ConversationOrigin{OwnerID: "former-core", ID: "old-conversation"}, RequestID: "cutover-context", Message: "continue"}
	query := coordination.DataQuery{ConversationID: request.ConversationID}
	if _, err := engine.Query(t.Context(), request, query); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("new Core pretended to expose old Core data: %v", err)
	}
	if _, err := engine.Run(t.Context(), request); !errors.Is(err, coordination.ErrWrongOwner) || model.calls.Load() != 0 {
		t.Fatalf("foreign conversation started without explicit context: %v", err)
	}
	request.Context = &ForwardedContext{PreviousCoreID: "former-core", ConversationID: request.ConversationID, Messages: []ContextMessage{{ID: "old-message", OwnerID: "former-core", Role: "user", Content: "old client context"}}}
	model.generate = func(_ context.Context, inference Inference) (Generation, error) {
		if inference.HistoricalSnapshot != nil || inference.Context == nil || len(inference.Context.Messages) != 1 || inference.Scope.ResourceOwnerID != "core" {
			t.Fatal("cutover did not isolate client context from old Core storage")
		}
		return Generation{Text: "new Core reply"}, nil
	}
	response, err := engine.Run(t.Context(), request)
	if err != nil || !response.Saved || response.ConversationID == request.ConversationID {
		t.Fatalf("cutover continuation failed: %+v %v", response, err)
	}
	current, err := engine.Query(t.Context(), request, query)
	if err != nil || current.HistoricalSnapshot != nil || current.ConversationOrigin == nil || *current.ConversationOrigin != *request.ConversationOrigin {
		t.Fatalf("new Core continuation was not independently readable: %+v %v", current, err)
	}
	var formerRows int
	if err := db.QueryRow(`SELECT count(*) FROM kernel_device_owned_resources WHERE owner_id='former-core'`).Scan(&formerRows); err != nil || formerRows != 0 {
		t.Fatalf("cutover migrated former Core data: %d %v", formerRows, err)
	}
	otherRole := request
	otherRole.ConversationOrigin = &ConversationOrigin{OwnerID: "unbound-owner", ID: request.ConversationID}
	if _, err := engine.Query(t.Context(), otherRole, query); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatal("stored lineage opened an unrelated owner namespace")
	}
}
