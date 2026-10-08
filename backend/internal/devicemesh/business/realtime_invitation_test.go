package business

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRealtimeInvitationRequiresOriginalOwnerAuthorityAndSingleAccept(t *testing.T) {
	for _, coordinated := range []bool{false, true} {
		t.Run(map[bool]string{false: "device", true: "core"}[coordinated], func(t *testing.T) {
			engine, db, service, _ := engineHarness(t)
			if coordinated {
				if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
					t.Fatal(err)
				}
			}
			request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: uuid.NewString()}
			_, scope, finish, err := engine.continuityAuthority(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			finish()
			request.ExpectedScope = &scope
			invitation, ack, err := engine.CreateRealtimeInvitation(t.Context(), request, "audio")
			if err != nil || invitation.Revision != 1 || invitation.Status != "pending" || ack.OwnerID != scope.ResourceOwnerID {
				t.Fatalf("invitation creation failed: %+v %v", invitation, err)
			}
			other := "core"
			if coordinated {
				other = "a"
			}
			if row, err := coordination.NewOwnershipStore(db, other).Get(t.Context(), "checkpoint", "realtime-invitation/"+invitation.ID); err != nil || row != nil {
				t.Fatal("invitation mirrored at other owner", err)
			}
			read, err := engine.ReadRealtimeInvitation(t.Context(), request, invitation.ID)
			if err != nil || read.Nonce != invitation.Nonce {
				t.Fatal("recipient cannot read original invitation", err)
			}
			request.RequestID, request.ExpectedScope = uuid.NewString(), &invitation.Scope
			if _, _, err := engine.AcceptRealtimeInvitation(t.Context(), request, invitation.ID, "forged", 1); !errors.Is(err, coordination.ErrScopeExpired) {
				t.Fatalf("forged nonce accepted: %v", err)
			}
			accepted, _, err := engine.AcceptRealtimeInvitation(t.Context(), request, invitation.ID, invitation.Nonce, 1)
			if err != nil || accepted.Status != "accepted" || accepted.Revision != 2 {
				t.Fatal("recipient accept failed", err)
			}
			if _, _, err := engine.AcceptRealtimeInvitation(t.Context(), request, invitation.ID, invitation.Nonce, 1); err != nil {
				t.Fatal("same operation replay lost original acknowledgement", err)
			}
			request.RequestID = uuid.NewString()
			if _, _, err := engine.AcceptRealtimeInvitation(t.Context(), request, invitation.ID, invitation.Nonce, 1); !errors.Is(err, coordination.ErrRequestConflict) {
				t.Fatalf("second operation accepted invitation: %v", err)
			}
			policy, err := service.Get(t.Context(), "core", "a")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.ChangeMode(t.Context(), "core", "a", policy.ModeRevision, !coordinated, "role"); err != nil {
				t.Fatal(err)
			}
			if _, err := engine.ReadRealtimeInvitation(t.Context(), request, invitation.ID); !errors.Is(err, coordination.ErrScopeExpired) {
				t.Fatalf("expired owner invitation read succeeded: %v", err)
			}
		})
	}
}
