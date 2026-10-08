package main

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedRealtimeInvitationRealTLSOnlyRecipientAcceptsAtOriginalOwner(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a, b, other := newThreeCoreFixture(t, "invite-a", schemas), newThreeCoreFixture(t, "invite-b", schemas), newThreeCoreFixture(t, "invite-other", schemas)
	pairThreeCoreFixtures(t, a, b)
	pairThreeCoreFixtures(t, other, b)
	const endpoint = "/api/device-mesh/v1/business"
	for _, coordinated := range []bool{false, true} {
		if coordinated {
			policy, err := b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
			if err != nil {
				t.Fatal(err)
			}
			b.request(t, a, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "expectedRevision": policy.ModeRevision, "selectedRole": "one"}, 200)
		}
		view := decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, b.request(t, a, http.MethodGet, endpoint+"/memories?characterId=one", nil, 200))
		payload := map[string]any{"requestId": uuid.NewString(), "characterId": "one", "expectedExecutionScope": view.Scope, "callType": "audio"}
		created := decodeThreeCoreMemoryResponse[struct {
			Invitation business.RealtimeInvitation `json:"invitation"`
			Saved      bool                        `json:"saved"`
		}](t, b.request(t, a, http.MethodPost, endpoint+"/realtime/invitations", payload, 200))
		invitation := created.Invitation
		if !created.Saved || invitation.RecipientDeviceID != a.device.DeviceID.String() || invitation.Scope.ResourceOwnerID != view.Scope.ResourceOwnerID || invitation.Revision != 1 {
			t.Fatalf("invitation did not acknowledge recipient owner: %+v", created)
		}
		path := endpoint + "/realtime/invitations/" + invitation.ID
		b.request(t, other, http.MethodGet, path+"?characterId=one", nil, 409)
		read := decodeThreeCoreMemoryResponse[struct {
			Invitation business.RealtimeInvitation `json:"invitation"`
		}](t, b.request(t, a, http.MethodGet, path+"?characterId=one", nil, 200))
		if read.Invitation.Nonce != invitation.Nonce {
			t.Fatal("recipient invitation nonce changed")
		}
		accept := map[string]any{"requestId": uuid.NewString(), "characterId": "one", "expectedExecutionScope": invitation.Scope, "expectedRevision": 1, "nonce": invitation.Nonce}
		b.request(t, other, http.MethodPost, path+"/accept", accept, 409)
		accepted := decodeThreeCoreMemoryResponse[struct {
			Invitation business.RealtimeInvitation `json:"invitation"`
			Ticket     map[string]any              `json:"ticket"`
		}](t, b.request(t, a, http.MethodPost, path+"/accept", accept, 200))
		if accepted.Invitation.Status != "accepted" || accepted.Invitation.Revision != 2 || accepted.Ticket["ticket"] == "" || accepted.Ticket["ticket"] == nil {
			t.Fatal("accepted invitation did not issue recipient realtime ticket")
		}
		accept["requestId"] = uuid.NewString()
		b.request(t, a, http.MethodPost, path+"/accept", accept, 409)
		holder, otherHolder := a, b
		if coordinated {
			holder, otherHolder = b, a
		}
		row, err := coordination.NewOwnershipStore(holder.services.KernelContainer.DeviceRegistry.Database(), invitation.Scope.ResourceOwnerID).Get(t.Context(), "checkpoint", "realtime-invitation/"+invitation.ID)
		if err != nil || row == nil || row.Revision != 2 {
			t.Fatal("accepted invitation missing at actual owner", err)
		}
		if row, err := coordination.NewOwnershipStore(otherHolder.services.KernelContainer.DeviceRegistry.Database(), invitation.Scope.ResourceOwnerID).Get(t.Context(), "checkpoint", "realtime-invitation/"+invitation.ID); err != nil || row != nil {
			t.Fatal("invitation mirrored at another owner", err)
		}
	}
}
