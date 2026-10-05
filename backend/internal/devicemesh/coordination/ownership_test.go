package coordination_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type pendingRolePort struct {
	roles []coordination.Role
	err   error
}

func (p pendingRolePort) Roles(context.Context, coordination.ExecutionScope) ([]coordination.Role, error) {
	return p.roles, p.err
}
func (pendingRolePort) Snapshot(context.Context, coordination.ExecutionScope, coordination.DataQuery) (coordination.DataSnapshot, error) {
	panic("unexpected snapshot")
}
func (pendingRolePort) Commit(context.Context, coordination.Commit) (coordination.Acknowledgement, error) {
	panic("unexpected commit")
}

func TestPendingInvalidRoleIsPurgedButOfflineRoleIsRetained(t *testing.T) {
	_, service := setup(t)
	commit := ownedCommit("old-role")
	commit.Scope.RoleRevision = 1
	if err := service.Enqueue(t.Context(), commit); err != nil {
		t.Fatal(err)
	}
	pending, err := service.Pending(t.Context(), "a")
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending=%v err=%v", pending, err)
	}
	valid := pendingRolePort{roles: []coordination.Role{{ID: "role", Revision: 1, Profile: json.RawMessage(`{}`)}}}
	if err := service.DiscardPending(t.Context(), pending[0], valid); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("valid pending discarded: %v", err)
	}
	offline := errors.New("device offline")
	if err := service.DiscardPending(t.Context(), pending[0], pendingRolePort{err: offline}); !errors.Is(err, offline) {
		t.Fatalf("offline pending discarded: %v", err)
	}
	if err := service.DiscardPending(t.Context(), pending[0], pendingRolePort{}); err != nil {
		t.Fatal(err)
	}
	pending, err = service.Pending(t.Context(), "a")
	if err != nil || len(pending) != 0 {
		t.Fatalf("invalid role content retained: %v %v", pending, err)
	}
}

func TestOwnedMessagePagesPreserveCreationOrderAndRejectForeignCursors(t *testing.T) {
	db, _ := setup(t)
	store := coordination.NewOwnershipStore(db, "a")
	tx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	base := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 301; i++ {
		created := base.Add(time.Duration(i) * time.Millisecond).Format(time.RFC3339Nano)
		body, _ := json.Marshal(map[string]any{"conversationId": "chat", "createdAt": created, "content": i})
		if _, err := tx.Exec(`INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,body,updated_at) VALUES('a','message',?,'role',1,?,?)`, fmt.Sprintf("m-%04d", i), body, base.Add(time.Hour).Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	query := coordination.DataQuery{ResourceKind: "message", ConversationID: "chat", Limit: 128}
	seen := map[string]bool{}
	firstCursor := ""
	newestBefore := 301
	for page := 0; page < 4; page++ {
		rows, next, err := store.ListPage(t.Context(), "message", "role", query)
		if err != nil {
			t.Fatal(err)
		}
		for i, row := range rows {
			if seen[row.ID] {
				t.Fatalf("duplicate %s", row.ID)
			}
			seen[row.ID] = true
			if i > 0 && rows[i-1].ID >= row.ID {
				t.Fatal("page order changed")
			}
		}
		if len(rows) > 0 {
			var number int
			fmt.Sscanf(rows[len(rows)-1].ID, "m-%04d", &number)
			if number >= newestBefore {
				t.Fatal("edited old message moved into recent history")
			}
			newestBefore = number
		}
		if page == 0 {
			firstCursor = next
			if _, err := db.Exec(`UPDATE kernel_device_owned_resources SET updated_at='2027-01-01T00:00:00Z' WHERE resource_id='m-0000'`); err != nil {
				t.Fatal(err)
			}
		}
		query.Cursor = next
		if next == "" {
			break
		}
	}
	if len(seen) != 301 || firstCursor == "" || query.Cursor != "" {
		t.Fatalf("truncated history: %d", len(seen))
	}
	query.Cursor = firstCursor
	for _, other := range []struct{ owner, role, conversation string }{{"b", "role", "chat"}, {"a", "other", "chat"}, {"a", "role", "other"}} {
		query.ConversationID = other.conversation
		if _, _, err := coordination.NewOwnershipStore(db, other.owner).ListPage(t.Context(), "message", other.role, query); !errors.Is(err, coordination.ErrWrongOwner) {
			t.Fatalf("foreign cursor accepted: %v", err)
		}
	}
}

func TestOwnedQuerySelectsCurrentConversationBeforeApplyingHistoryLimit(t *testing.T) {
	db, _ := setup(t)
	store := coordination.NewOwnershipStore(db, "a")
	tx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for i := 0; i < 1005; i++ {
		conversation := "other"
		if i < 5 {
			conversation = "current"
		}
		payload := fmt.Sprintf(`{"conversationId":%q,"content":%q}`, conversation, fmt.Sprintf("message-%d", i))
		if _, err := tx.Exec(`INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,body,updated_at) VALUES('a','message',?,'role',1,?,?)`, fmt.Sprintf("message-%04d", i), []byte(payload), fmt.Sprintf("2026-10-04T00:00:%04dZ", i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	rows, err := store.ListForQuery(t.Context(), "message", "role", coordination.DataQuery{ConversationID: "current", Limit: 3})
	if err != nil || len(rows) != 3 || rows[0].ID != "message-0002" || rows[2].ID != "message-0004" {
		t.Fatalf("current conversation tail: %+v %v", rows, err)
	}
	rows, err = store.ListForQuery(t.Context(), "message", "role", coordination.DataQuery{ListConversations: true})
	if err != nil || len(rows) != 0 {
		t.Fatal("sidebar query exposed unrelated message bodies", err)
	}
}

func ownedCommit(request string) coordination.Commit {
	return coordination.Commit{Scope: coordination.ExecutionScope{SpaceID: "space", InitiatorDeviceID: "a", TargetDeviceID: "a", CoreID: "core", ProviderEpoch: 1, ModeRevision: 1, PermissionRevision: 1, ResourceOwnerID: "a", RoleOwnerID: "a", RoleID: "role", RequestID: request}, Mutations: []coordination.Mutation{{Kind: "memory", ID: "memory-1", RoleID: "role", Body: json.RawMessage(`{"key":"preference","value":"tea"}`)}}}
}

func TestDeviceCommitIsAtomicIdempotentAndRejectsWrongOwner(t *testing.T) {
	db, _ := setup(t)
	store := coordination.NewOwnershipStore(db, "a")
	commit := ownedCommit("request-1")
	ack, err := store.Apply(t.Context(), commit)
	if err != nil || ack.Versions["memory/memory-1"] != 1 {
		t.Fatalf("commit: %+v %v", ack, err)
	}
	replayed, err := store.Apply(t.Context(), commit)
	if err != nil || replayed.Versions["memory/memory-1"] != 1 {
		t.Fatalf("replay: %+v %v", replayed, err)
	}
	commit.Mutations[0].Body = json.RawMessage(`{"value":"coffee"}`)
	if _, err := store.Apply(t.Context(), commit); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("request changed: %v", err)
	}
	commit = ownedCommit("request-2")
	commit.Scope.ResourceOwnerID = "b"
	if _, err := store.Apply(t.Context(), commit); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("wrong owner: %v", err)
	}
	commit = ownedCommit("request-3")
	commit.Mutations = append(commit.Mutations, coordination.Mutation{Kind: "summary", ID: "summary-1", RoleID: "role", Body: json.RawMessage(`"summary"`)})
	if _, err := store.Apply(t.Context(), commit); !errors.Is(err, coordination.ErrResourceVersion) {
		t.Fatalf("stale version: %v", err)
	}
	items, err := store.List(t.Context(), "summary", "role", false)
	if err != nil || len(items) != 0 {
		t.Fatalf("partial transaction survived: %+v %v", items, err)
	}
}

func TestDeletedReadDependencyRejectsNewUnrelatedMemory(t *testing.T) {
	db, _ := setup(t)
	store := coordination.NewOwnershipStore(db, "a")
	if _, err := store.Apply(t.Context(), ownedCommit("create-dependency")); err != nil {
		t.Fatal(err)
	}
	deletion := ownedCommit("delete-dependency")
	deletion.Mutations[0].ExpectedRevision = 1
	deletion.Mutations[0].Deleted = true
	if _, err := store.Apply(t.Context(), deletion); err != nil {
		t.Fatal(err)
	}
	late := ownedCommit("late-computed-memory")
	late.Mutations[0].ID = "new-memory-id"
	late.Dependencies = []coordination.ResourceVersion{{Kind: "memory", ID: "memory-1", Revision: 1}}
	if _, err := store.Apply(t.Context(), late); !errors.Is(err, coordination.ErrResourceVersion) {
		t.Fatalf("deleted evidence revived under a new ID: %v", err)
	}
	items, err := store.List(t.Context(), "memory", "role", false)
	if err != nil || len(items) != 0 {
		t.Fatalf("late memory survived: %v %v", items, err)
	}
}

func TestMemoryDeletionRemovesDerivedDataAndRejectsResurrection(t *testing.T) {
	db, _ := setup(t)
	store := coordination.NewOwnershipStore(db, "a")
	commit := ownedCommit("create")
	commit.Mutations = append(commit.Mutations, coordination.Mutation{Kind: "vector", ID: "vector-1", RoleID: "role", SourceID: "memory-1", Body: json.RawMessage(`[0.1,0.2]`)}, coordination.Mutation{Kind: "graph", ID: "graph-1", RoleID: "role", SourceID: "memory-1", Body: json.RawMessage(`{"relation":"likes"}`)})
	if _, err := store.Apply(t.Context(), commit); err != nil {
		t.Fatal(err)
	}
	deletion := ownedCommit("delete")
	deletion.Mutations[0].ExpectedRevision = 1
	deletion.Mutations[0].Deleted = true
	if _, err := store.Apply(t.Context(), deletion); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"memory", "vector", "graph"} {
		items, err := store.List(t.Context(), kind, "role", false)
		if err != nil || len(items) != 0 {
			t.Fatalf("deleted %s visible: %+v %v", kind, items, err)
		}
	}
	late := ownedCommit("late-index")
	late.Mutations = []coordination.Mutation{{Kind: "vector", ID: "vector-1", RoleID: "role", SourceID: "memory-1", ExpectedRevision: 2, Body: json.RawMessage(`[0.3]`)}}
	if _, err := store.Apply(t.Context(), late); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("late index resurrected memory: %v", err)
	}
}

func TestPendingSaveRequiresMatchingAcknowledgementAndIsPurged(t *testing.T) {
	db, svc := setup(t)
	commit := ownedCommit("pending-save")
	if err := svc.Enqueue(t.Context(), commit); err != nil {
		t.Fatal(err)
	}
	if err := svc.Enqueue(t.Context(), commit); err != nil {
		t.Fatal(err)
	}
	list, err := svc.Pending(t.Context(), "a")
	if err != nil || len(list) != 1 {
		t.Fatalf("pending: %+v %v", list, err)
	}
	ack, err := coordination.NewOwnershipStore(db, "a").Apply(t.Context(), commit)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Acknowledge(t.Context(), ack, "wrong"); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("unmatched ACK accepted: %v", err)
	}
	if err := svc.Acknowledge(t.Context(), ack, list[0].Hash); err != nil {
		t.Fatal(err)
	}
	if err := svc.Acknowledge(t.Context(), ack, list[0].Hash); err != nil {
		t.Fatalf("duplicate delivery ACK was not idempotent: %v", err)
	}
	list, err = svc.Pending(t.Context(), "a")
	if err != nil || len(list) != 0 {
		t.Fatalf("acknowledged content retained in Core: %+v %v", list, err)
	}
}

func TestResourceCannotBeTakenOverByAnotherRole(t *testing.T) {
	db, _ := setup(t)
	store := coordination.NewOwnershipStore(db, "a")
	if _, err := store.Apply(t.Context(), ownedCommit("one")); err != nil {
		t.Fatal(err)
	}
	commit := ownedCommit("two")
	commit.Scope.RoleID = "other-role"
	commit.Mutations[0].RoleID = "other-role"
	commit.Mutations[0].ExpectedRevision = 1
	if _, err := store.Apply(t.Context(), commit); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("role takeover: %v", err)
	}
}

func TestPendingRequestsAreNamespacedByOwnerAndBackoffIsDurable(t *testing.T) {
	db, svc := setup(t)
	a := ownedCommit("same-request")
	b := ownedCommit("same-request")
	b.Scope.InitiatorDeviceID = "b"
	b.Scope.TargetDeviceID = "b"
	b.Scope.ResourceOwnerID = "b"
	b.Scope.RoleOwnerID = "b"
	for _, commit := range []coordination.Commit{a, b} {
		if err := svc.Enqueue(t.Context(), commit); err != nil {
			t.Fatal(err)
		}
	}
	owners, err := svc.PendingOwners(t.Context())
	if err != nil || len(owners) != 2 {
		t.Fatalf("owners=%v error=%v", owners, err)
	}
	pending, err := svc.Pending(t.Context(), "a")
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending=%v error=%v", pending, err)
	}
	if err := svc.RetryLater(t.Context(), pending[0]); err != nil {
		t.Fatal(err)
	}
	restarted := coordination.NewService(db)
	owners, err = restarted.PendingOwners(t.Context())
	if err != nil || len(owners) != 1 || owners[0] != "b" {
		t.Fatalf("retry ignored: %v %v", owners, err)
	}
	var attempts int
	if err := db.QueryRow(`SELECT attempts FROM kernel_device_owned_outbox WHERE owner_id='a' AND request_id='same-request'`).Scan(&attempts); err != nil || attempts != 1 {
		t.Fatalf("attempts=%d error=%v", attempts, err)
	}
}
