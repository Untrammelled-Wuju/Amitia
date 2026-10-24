package coordination_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func executionBindingCommit(request, thread, role string) coordination.Commit {
	commit := ownedCommit(request)
	body, _ := json.Marshal(map[string]any{"thread": map[string]any{"id": thread}, "bindings": []map[string]any{{"threadId": thread, "bindingType": "task_run", "bindingId": "shared-task", "role": role}}})
	commit.Mutations = []coordination.Mutation{{Kind: "continuity", ID: thread, RoleID: "role", Body: body}}
	return commit
}

func TestContinuityExecutionBindingUniqueButDependenciesRemainShared(t *testing.T) {
	db, _ := setup(t)
	store := coordination.NewOwnershipStore(db, "a")
	first := executionBindingCommit("first", "goal-1", "execution")
	if _, err := store.Apply(t.Context(), first); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Apply(t.Context(), first); err != nil {
		t.Fatalf("exact retry failed: %v", err)
	}
	for index, role := range []string{"context", "dependency", "", "input"} {
		if _, err := store.Apply(t.Context(), executionBindingCommit(fmt.Sprint("ref-", index), fmt.Sprint("reference-", index), role)); err != nil {
			t.Fatalf("shared dependency rejected: %v", err)
		}
	}
	conflict := executionBindingCommit("conflict", "goal-2", "execution")
	conflict.Mutations = append(conflict.Mutations, coordination.Mutation{Kind: "checkpoint", ID: "must-rollback", RoleID: "role", Body: json.RawMessage(`{"proof":true}`)})
	if _, err := store.Apply(t.Context(), conflict); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("second execution goal admitted: %v", err)
	}
	for _, resource := range []struct{ kind, id string }{{"continuity", "goal-2"}, {"checkpoint", "must-rollback"}} {
		if result, err := store.Get(t.Context(), resource.kind, resource.id); err != nil || result != nil {
			t.Fatalf("conflicting commit partially persisted: %+v %v", result, err)
		}
	}
	var receipts int
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_device_owned_inbox WHERE owner_id='a' AND request_id='conflict'`).Scan(&receipts); err != nil || receipts != 0 {
		t.Fatalf("conflicting commit left success receipt: %d %v", receipts, err)
	}
}

func TestContinuityExecutionBindingConcurrentOwnersAndBulkConflict(t *testing.T) {
	db, _ := setup(t)
	var sequence int
	var name, filename string
	if err := db.QueryRow(`PRAGMA database_list`).Scan(&sequence, &name, &filename); err != nil {
		t.Fatal(err)
	}
	other, err := sql.Open("sqlite", filename)
	if err != nil {
		t.Fatal(err)
	}
	other.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = other.Close() })
	stores := []*coordination.OwnershipStore{coordination.NewOwnershipStore(db, "a"), coordination.NewOwnershipStore(other, "a")}
	var successes atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for index := 0; index < 24; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			if _, err := stores[index%2].Apply(t.Context(), executionBindingCommit(fmt.Sprint("race-", index), fmt.Sprint("goal-", index), "execution")); err == nil {
				successes.Add(1)
			}
		}(index)
	}
	close(start)
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("expected one durable execution owner, got %d", successes.Load())
	}
	foreign := executionBindingCommit("other-owner", "other-goal", "execution")
	foreign.Scope.ResourceOwnerID, foreign.Scope.RoleOwnerID = "b", "b"
	foreign.Scope.InitiatorDeviceID, foreign.Scope.TargetDeviceID = "b", "b"
	if _, err := coordination.NewOwnershipStore(db, "b").Apply(t.Context(), foreign); err != nil {
		t.Fatalf("distinct owner identity collided: %v", err)
	}
	bulk := executionBindingCommit("bulk-conflict", "bulk-1", "execution")
	second := executionBindingCommit("unused", "bulk-2", "execution")
	for _, item := range []*coordination.Commit{&bulk, &second} {
		var body map[string]any
		if err := json.Unmarshal(item.Mutations[0].Body, &body); err != nil {
			t.Fatal(err)
		}
		body["bindings"].([]any)[0].(map[string]any)["bindingId"] = "bulk-task"
		item.Mutations[0].Body, _ = json.Marshal(body)
	}
	bulk.Mutations = append(bulk.Mutations, second.Mutations...)
	if _, err := stores[0].Apply(t.Context(), bulk); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("same-transaction duplicate admitted: %v", err)
	}
}

func TestContinuityLegacyExecutionConflictsRemainReadableAndControllable(t *testing.T) {
	db, _ := setup(t)
	store := coordination.NewOwnershipStore(db, "a")
	var existing coordination.Commit
	for index := 0; index < 2; index++ {
		commit := executionBindingCommit(fmt.Sprint("legacy-", index), fmt.Sprint("legacy-goal-", index), "execution")
		mutation := commit.Mutations[0]
		if _, err := db.Exec(`INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,body,updated_at) VALUES('a','continuity',?,'role',1,?,'now')`, mutation.ID, mutation.Body); err != nil {
			t.Fatal(err)
		}
		existing = commit
	}
	if _, err := store.Apply(t.Context(), executionBindingCommit("new-claim", "third-goal", "execution")); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("legacy conflicts authorized another claim: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(existing.Mutations[0].Body, &document); err != nil {
		t.Fatal(err)
	}
	document["thread"].(map[string]any)["status"] = "cancelled"
	existing.Scope.RequestID = "cancel-legacy"
	existing.Mutations[0].ExpectedRevision = 1
	existing.Mutations[0].Body, _ = json.Marshal(document)
	if _, err := store.Apply(t.Context(), existing); err != nil {
		t.Fatalf("legacy conflict prevented safe control: %v", err)
	}
	resources, err := store.List(t.Context(), "continuity", "role", false)
	if err != nil || len(resources) != 2 {
		t.Fatalf("legacy evidence was deleted or reassigned: %d %v", len(resources), err)
	}
}
