package coordination_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestSourceCommitTransactionRejectsCancelledFencedAndForeignProof(t *testing.T) {
	for _, scenario := range []string{"valid", "cancelled", "caller_fenced", "target_fenced", "foreign_proof", "cached_ack"} {
		t.Run(scenario, func(t *testing.T) {
			db, _ := setup(t)
			owner := coordination.NewOwnershipStore(db, "b")
			scope := coordination.ExecutionScope{SpaceID: "space", CoreID: "space", AuthorizationRealm: "space", InitiatorDeviceID: "a", TargetDeviceID: "b", ResourceOwnerID: "b", RoleOwnerID: "b", RoleID: "role", RoleRevision: 1, PermissionRevision: 1, TargetPermissionRevision: 1, RequestID: "source-commit"}
			commit := coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: "private-task", RoleID: "role", Body: json.RawMessage(`{"private":"source-only"}`)}}}
			ctx := coordination.WithSourceCommitAuthority(t.Context(), scope, "source-call")
			if scenario == "cached_ack" {
				if _, err := owner.Apply(ctx, commit); err != nil {
					t.Fatal(err)
				}
			}
			switch scenario {
			case "cancelled", "cached_ack":
				if err := coordination.CancelSourceAuthority(t.Context(), db, "space", []string{"source-call"}); err != nil {
					t.Fatal(err)
				}
			case "caller_fenced":
				if err := coordination.FenceSourceAuthority(t.Context(), db, "space", "a", 1); err != nil {
					t.Fatal(err)
				}
			case "target_fenced":
				if err := coordination.FenceSourceAuthority(t.Context(), db, "space", "b", 1); err != nil {
					t.Fatal(err)
				}
			case "foreign_proof":
				proof := scope
				proof.RoleID = "other"
				ctx = coordination.WithSourceCommitAuthority(t.Context(), proof, "source-call")
			}
			ack, err := owner.Apply(ctx, commit)
			if scenario == "valid" {
				if err != nil || ack.OwnerID != "b" || ack.Versions["checkpoint/private-task"] != 1 {
					t.Fatalf("valid source transaction failed: %+v %v", ack, err)
				}
			} else if err == nil || scenario != "foreign_proof" && !errors.Is(err, coordination.ErrScopeExpired) {
				t.Fatalf("expired source transaction accepted: %+v %v", ack, err)
			}
			var rows int
			if err := db.QueryRow(`SELECT count(*) FROM kernel_device_owned_resources WHERE resource_id='private-task'`).Scan(&rows); err != nil {
				t.Fatal(err)
			}
			expected := 0
			if scenario == "valid" || scenario == "cached_ack" {
				expected = 1
			}
			if rows != expected {
				t.Fatalf("rejected source transaction changed body: %d", rows)
			}
		})
	}
}
