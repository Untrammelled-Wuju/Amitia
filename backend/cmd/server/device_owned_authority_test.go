package main

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/spaceidentity"
)

func TestDeviceOwnedHandlerRejectsDelayedCancelledAndOldPermissionWrites(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	dir := t.TempDir()
	identity, err := agent.NewIdentityStore(dir).Load()
	if err != nil {
		t.Fatal(err)
	}
	space, err := spaceidentity.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.services.DB.Model(&character.Character{}).Where("space_id=?", p.legacySpaceID).Update("space_id", space.SpaceID()).Error; err != nil {
		t.Fatal(err)
	}
	credential := &agent.StoredCredential{SpaceID: runtimeidentity.SpaceID("core"), DeviceID: identity.DeviceID, RuntimeID: identity.RuntimeID, CredentialID: "test-credential", Credential: "test-value", ExpiresAt: time.Now().Add(time.Hour)}
	if err := agent.NewCredentialStore(dir).SaveCredential(credential); err != nil {
		t.Fatal(err)
	}
	handler, err := newDeviceOwnedDataHandler(p.services, dir)
	if err != nil {
		t.Fatal(err)
	}
	scope := coordination.ExecutionScope{SpaceID: "core", CoreID: "core", AuthorizationRealm: "core", InitiatorDeviceID: "caller", TargetDeviceID: identity.DeviceID.String(), RoleOwnerID: identity.DeviceID.String(), ResourceOwnerID: identity.DeviceID.String(), PermissionRevision: 1, TargetPermissionRevision: 1, ModeRevision: 1, ProviderEpoch: 1, TargetProviderEpoch: 1, RoleID: "one", RoleRevision: 3, RequestID: "first"}
	invoke := func(id string, request any) error {
		input, err := json.Marshal(request)
		if err != nil {
			return err
		}
		_, err = handler(t.Context(), protocol.RuntimeInvokePayload{InvocationID: id, SpaceID: credential.SpaceID, DeviceID: identity.DeviceID, RuntimeID: identity.RuntimeID, Input: input})
		return err
	}
	commit := func(request string, current coordination.ExecutionScope) map[string]any {
		current.RequestID = request
		payload, _ := json.Marshal(map[string]string{"id": request, "conversationId": "conversation", "role": "user", "content": "allowed"})
		return map[string]any{"operation": "apply", "commit": coordination.Commit{Scope: current, Mutations: []coordination.Mutation{{Kind: "message", ID: request, RoleID: "one", Body: payload}}}}
	}
	if err := invoke("first-call", commit("first", scope)); err != nil {
		t.Fatal(err)
	}
	if err := invoke("reconcile", map[string]any{"operation": "authority-reconcile", "scope": scope, "cancelledCallIds": []string{"delayed-call"}}); err != nil {
		t.Fatal(err)
	}
	if err := invoke("delayed-call", commit("cancelled", scope)); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("cancelled call wrote: %v", err)
	}
	if err := invoke("fence", map[string]any{"operation": "authority-fence", "scope": scope, "fencedDeviceId": "caller", "closedPermissionRevision": 1}); err != nil {
		t.Fatal(err)
	}
	if err := invoke("late-call", commit("late", scope)); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("old caller wrote: %v", err)
	}
	scope.PermissionRevision = 2
	if err := invoke("new-call", commit("new", scope)); err != nil {
		t.Fatalf("new authorized call blocked: %v", err)
	}
	store := coordination.NewOwnershipStore(p.services.KernelContainer.DeviceRegistry.Database(), identity.DeviceID.String())
	for _, id := range []string{"cancelled", "late"} {
		row, err := store.Get(t.Context(), "message", id)
		if err != nil || row != nil {
			t.Fatalf("rejected data saved: %s %v", id, err)
		}
	}
}
