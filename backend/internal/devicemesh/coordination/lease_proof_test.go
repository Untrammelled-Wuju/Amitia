package coordination_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestCommitLeaseRejectsReplacementUnknownExpiryAndForeignAuthority(t *testing.T) {
	for _, scenario := range []string{"replacement", "unknown", "expired", "paused", "deleted", "space", "role-owner", "resource-owner", "valid"} {
		t.Run(scenario, func(t *testing.T) {
			db, _ := setup(t)
			store := coordination.NewOwnershipStore(db, "a")
			commit := ownedCommit("run|memory")
			commit.LeaseProof = &coordination.CommitLeaseProof{ContinuityID: "thread", LeaseID: "lease", RequestID: "run", AllowCompleted: true}
			scope := commit.Scope
			leaseID, state, threadState := "lease", "running", "active"
			expires := time.Now().Add(time.Minute)
			deleted := false
			switch scenario {
			case "replacement":
				leaseID = "new-lease"
			case "unknown":
				state = "unknown"
			case "expired":
				expires = time.Now().Add(-time.Second)
			case "paused":
				threadState = "paused"
			case "deleted":
				deleted = true
			case "space":
				scope.SpaceID = "other"
			case "role-owner":
				scope.RoleOwnerID = "other"
			case "resource-owner":
				scope.ResourceOwnerID = "other"
			}
			raw, err := json.Marshal(map[string]any{"executionScope": scope, "coreId": commit.Scope.CoreID, "ownerId": "a", "thread": map[string]any{"status": threadState}, "lease": map[string]any{"id": leaseID, "requestId": "run", "state": state, "expiresAt": expires}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(t.Context(), `INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,body,deleted,updated_at) VALUES('a','continuity','thread','role',1,?,?,?)`, raw, deleted, time.Now().Format(time.RFC3339Nano)); err != nil {
				t.Fatal(err)
			}
			_, err = store.Apply(t.Context(), commit)
			if scenario == "valid" {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if !errors.Is(err, coordination.ErrResourceVersion) {
					t.Fatalf("invalid lease accepted: %v", err)
				}
				if resource, err := store.Get(t.Context(), "memory", "memory-1"); err != nil || resource != nil {
					t.Fatalf("late memory saved: %+v %v", resource, err)
				}
			}
		})
	}
}

func TestCompletedLeaseMemoryRequiresSavedReplyFromSameAuthority(t *testing.T) {
	for _, scenario := range []string{"unsaved", "permission", "target", "role-owner", "space", "valid"} {
		t.Run(scenario, func(t *testing.T) {
			db, _ := setup(t)
			store := coordination.NewOwnershipStore(db, "a")
			commit := ownedCommit("run|memory")
			commit.LeaseProof = &coordination.CommitLeaseProof{ContinuityID: "thread", LeaseID: "lease", RequestID: "run", AllowCompleted: true}
			document, err := json.Marshal(map[string]any{"executionScope": commit.Scope, "coreId": commit.Scope.CoreID, "ownerId": "a", "thread": map[string]any{"status": "active"}, "lease": map[string]any{"id": "lease", "requestId": "run", "state": "completed"}})
			if err != nil {
				t.Fatal(err)
			}
			scope := commit.Scope
			scope.RequestID = "run"
			saved := true
			switch scenario {
			case "unsaved":
				saved = false
			case "permission":
				scope.PermissionRevision++
			case "target":
				scope.TargetDeviceID = "other"
			case "role-owner":
				scope.RoleOwnerID = "other"
			case "space":
				scope.SpaceID = "other"
			}
			checkpoint, err := json.Marshal(map[string]any{"status": "completed", "response": map[string]any{"saved": saved, "executionScope": scope}})
			if err != nil {
				t.Fatal(err)
			}
			for kind, raw := range map[string][]byte{"continuity": document, "checkpoint": checkpoint} {
				id := "thread"
				if kind == "checkpoint" {
					id = "turn/run"
				}
				if _, err := db.ExecContext(t.Context(), `INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,body,updated_at) VALUES('a',?,?,'role',1,?,?)`, kind, id, raw, time.Now().Format(time.RFC3339Nano)); err != nil {
					t.Fatal(err)
				}
			}
			_, err = store.Apply(t.Context(), commit)
			if scenario == "valid" {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, coordination.ErrResourceVersion) {
				t.Fatalf("completed lease accepted foreign reply: %v", err)
			}
		})
	}
}
