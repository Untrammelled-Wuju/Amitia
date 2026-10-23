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

func TestPrivacySourceManagementOnlyAllowsExactRedaction(t *testing.T) {
	for _, scenario := range []string{"valid", "content", "conversation", "role", "stale"} {
		t.Run(scenario, func(t *testing.T) {
			db, _ := setup(t)
			store := coordination.NewOwnershipStore(db, "b")
			scope := coordination.ExecutionScope{SpaceID: "space", CoreID: "space", AuthorizationRealm: "space", InitiatorDeviceID: "a", TargetDeviceID: "b", ResourceOwnerID: "b", RoleOwnerID: "b", RoleID: "role", RoleRevision: 1, PermissionRevision: 1, TargetPermissionRevision: 1, RequestID: "seed"}
			if _, err := store.Apply(t.Context(), coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "message", ID: "private", RoleID: "role", Body: json.RawMessage(`{"content":"password fixture","conversationId":"original","role":"user"}`)}}}); err != nil {
				t.Fatal(err)
			}
			document := map[string]any{"content": "[已脱敏]", "reasoningContent": "", "safetyLevel": "masked", "conversationId": "original", "role": "user"}
			revision := int64(1)
			switch scenario {
			case "content":
				document["content"] = "replacement"
			case "conversation":
				document["conversationId"] = "other"
			case "role":
				document["role"] = "assistant"
			case "stale":
				revision = 2
			}
			raw, _ := json.Marshal(document)
			proof := scope
			scope.RequestID = "redact"
			_, err := store.Apply(coordination.WithSourceManagementAuthority(t.Context(), proof, "source-call"), coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "message", ID: "private", RoleID: "role", ExpectedRevision: revision, Body: raw}}})
			if scenario == "valid" && err != nil {
				t.Fatal(err)
			}
			if scenario != "valid" && err == nil {
				t.Fatal("unsafe rewrite accepted")
			}
		})
	}
}

func TestPrivacyDeletionPreventsProjectionResurrection(t *testing.T) {
	for _, kind := range []string{"vector", "graph"} {
		t.Run(kind, func(t *testing.T) {
			db, _ := setup(t)
			store := coordination.NewOwnershipStore(db, "b")
			raw, _ := json.Marshal(map[string]any{"references": []map[string]any{{"kind": kind, "id": "purged", "legacy": false}}})
			if _, err := db.Exec(`INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,body,updated_at) VALUES('b','checkpoint','privacy-deletion/test','role',1,?,'2026-10-23T00:00:00Z')`, raw); err != nil {
				t.Fatal(err)
			}
			scope := coordination.ExecutionScope{ResourceOwnerID: "b", RoleOwnerID: "b", TargetDeviceID: "b", RoleID: "role", RequestID: "resurrect"}
			_, err := store.Apply(t.Context(), coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: kind, ID: "purged", RoleID: "role", SourceID: "memory", Body: json.RawMessage(`{"content":{}}`)}}})
			if !errors.Is(err, coordination.ErrResourceVersion) {
				t.Fatalf("deleted projection recreated: %v", err)
			}
		})
	}
}
