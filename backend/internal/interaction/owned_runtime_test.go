package interaction

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func ownedRuntimeInput(t *testing.T, tracker InteractionTracker, id string) (context.Context, *InteractionRecord, *ProcessRequest) {
	t.Helper()
	scope := coordination.ExecutionScope{SpaceID: "space", RoleID: "role", RoleRevision: 1, ResourceOwnerID: "device", RoleOwnerID: "device", RequestID: id}
	ctx := coordination.WithScope(t.Context(), scope)
	ctx, err := WithOwnedContext(ctx, scope, coordination.DataSnapshot{OwnerID: "device", Role: coordination.Role{ID: "role", Revision: 1, Profile: json.RawMessage(`{"characterId":"role","name":"role"}`)}}, "conversation", "web")
	if err != nil {
		t.Fatal(err)
	}
	record := NewInteractionRecord(InteractionScope{SpaceID: "space", CharacterID: "role", ConversationID: "conversation", RequestID: id, Channel: "web"})
	if err := tracker.Create(ctx, record); err != nil {
		t.Fatal(err)
	}
	record, err = tracker.TransitionCAS(ctx, record.ID, record.StatusVersion, InteractionStatusProcessing)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, record, &ProcessRequest{SpaceID: "space", CharacterID: "role", ConversationID: "conversation", RequestID: id, Channel: "web", Message: "hello"}
}

func TestOwnedRuntimeRequiresOwnerSnapshotAndMatchingRole(t *testing.T) {
	tracker := NewInMemoryTracker()
	service := NewRuntimeContextService(tracker, 1)
	ctx, record, input := ownedRuntimeInput(t, tracker, "scope")
	if _, _, _, err := service.PrepareOwnedRuntime(t.Context(), record, input); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("unbound context accepted: %v", err)
	}
	input.CharacterID = "other"
	if _, _, _, err := service.PrepareOwnedRuntime(ctx, record, input); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("cross-role context accepted: %v", err)
	}
}

func TestOwnedRuntimeRetainsConcurrencyLimitAndReleasesExactlyOnce(t *testing.T) {
	tracker := NewInMemoryTracker()
	service := NewRuntimeContextService(tracker, 1)
	ctx, record, input := ownedRuntimeInput(t, tracker, "first")
	_, runtime, finish, err := service.PrepareOwnedRuntime(ctx, record, input)
	if err != nil || runtime == nil || runtime.ExecutorID != "owned" {
		t.Fatalf("runtime not prepared: %+v %v", runtime, err)
	}
	second, record, input := ownedRuntimeInput(t, tracker, "second")
	wait, cancel := context.WithTimeout(second, 50*time.Millisecond)
	defer cancel()
	if _, _, _, err := service.PrepareOwnedRuntime(wait, record, input); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("parallel limit or cancellation lost: %v", err)
	}
	if err := finish(ctx, true); err != nil {
		t.Fatal(err)
	}
	if err := finish(ctx, false); err != nil {
		t.Fatal(err)
	}
	_, _, nextFinish, err := service.PrepareOwnedRuntime(second, record, input)
	if err != nil {
		t.Fatalf("completed runtime leaked its slot: %v", err)
	}
	if err := nextFinish(second, true); err != nil {
		t.Fatal(err)
	}
}
