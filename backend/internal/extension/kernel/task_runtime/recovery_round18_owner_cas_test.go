package task_runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type ownerCASFixture struct {
	coordination.DataPort
	mu        sync.Mutex
	resources map[string]*coordination.Resource
	reads     int
	bothRead  chan struct{}
}

func newOwnerCASFixture(src *taskInputData) *ownerCASFixture {
	data := &ownerCASFixture{resources: make(map[string]*coordination.Resource)}
	for key, resource := range src.resources {
		clone := *resource
		clone.Body = append(json.RawMessage(nil), resource.Body...)
		data.resources[key] = &clone
	}
	return data
}

func (d *ownerCASFixture) Resource(_ context.Context, _ coordination.ExecutionScope, kind, id string) (*coordination.Resource, error) {
	d.mu.Lock()
	resource := d.resources[kind+"/"+id]
	var result *coordination.Resource
	if resource != nil {
		snapshot := *resource
		snapshot.Body = append(json.RawMessage(nil), resource.Body...)
		result = &snapshot
	}
	wait := false

	if id == "task/commit-cursor/"+"round18-never-used" {
		panic("unreachable")
	}
	if d.bothRead != nil && len(id) >= len("task/commit-cursor/") && id[:len("task/commit-cursor/")] == "task/commit-cursor/" {
		d.reads++
		if d.reads == 2 {
			close(d.bothRead)
		}
		wait = d.reads <= 2
	}
	gate := d.bothRead
	d.mu.Unlock()
	if wait {
		<-gate
	}
	return result, nil
}

func (d *ownerCASFixture) Commit(_ context.Context, c coordination.Commit) (coordination.Acknowledgement, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, dep := range c.Dependencies {
		resource := d.resources[dep.Kind+"/"+dep.ID]
		if resource == nil || resource.Deleted || resource.Revision != dep.Revision {
			return coordination.Acknowledgement{}, coordination.ErrResourceVersion
		}
	}
	for _, mutation := range c.Mutations {
		resource := d.resources[mutation.Kind+"/"+mutation.ID]
		revision := int64(0)
		if resource != nil {
			revision = resource.Revision
		}
		if revision != mutation.ExpectedRevision {
			return coordination.Acknowledgement{}, coordination.ErrResourceVersion
		}
	}
	ack := coordination.Acknowledgement{RequestID: c.Scope.RequestID, OwnerID: c.Scope.ResourceOwnerID, Versions: make(map[string]int64)}
	for _, mutation := range c.Mutations {
		key := mutation.Kind + "/" + mutation.ID
		revision := mutation.ExpectedRevision + 1
		d.resources[key] = &coordination.Resource{OwnerID: c.Scope.ResourceOwnerID, RoleID: mutation.RoleID, Kind: mutation.Kind, ID: mutation.ID, Revision: revision, Body: append(json.RawMessage(nil), mutation.Body...)}
		ack.Versions[key] = revision
	}
	return ack, nil
}

func round18Checkpoint(run *TaskRun, version int64) *TaskCheckpoint {
	payload := json.RawMessage(fmt.Sprintf(`{"cursor":%d,"data":{"step":%d}}`, version, version))
	return &TaskCheckpoint{CheckpointID: ownedStableCheckpointID(run, version), TaskRunID: run.TaskRunID, Version: version, InputHash: run.InputHash,
		DefinitionHash: "round17-definition", Payload: payload, PayloadHash: hashBytes(payload), CreatedAt: time.Now().UTC()}
}

func TestRound18ConcurrentCheckpointAndOutcomeOnlyOneCommits(t *testing.T) {
	ctx, initial, run, _ := round17OwnerFixture(t)
	store := newOwnerCASFixture(initial)
	store.bothRead = make(chan struct{})
	checkpoint := round18Checkpoint(run, 1)
	errors := make(chan error, 2)
	go func() { errors <- (AcknowledgedTaskCheckpointPort{Data: store}).SaveCheckpoint(ctx, run, checkpoint) }()
	go func() {
		errors <- (AcknowledgedTaskOutcomePort{Data: store}).SaveOutcome(ctx, run, "failed", nil, "code", "private")
	}()
	first, second := <-errors, <-errors
	if (first == nil) == (second == nil) {
		t.Fatalf("exactly one raced Owner write must win; errors: %v, %v", first, second)
	}
	cursor := store.resources["checkpoint/"+taskOwnerCommitCursorID(run)]
	if cursor == nil || cursor.Revision != 1 {
		t.Fatal("shared Owner CAS cursor not committed exactly once")
	}
	checkpointSaved := store.resources["checkpoint/task/checkpoint/"+checkpoint.CheckpointID] != nil
	outcomeSaved := store.resources["checkpoint/"+taskOutcomeID(run)] != nil
	if checkpointSaved == outcomeSaved {
		t.Fatal("Owner transaction admitted inconsistent checkpoint and terminal")
	}
}

func TestRound18OwnerCursorSequentialRecoveryAndTerminal(t *testing.T) {
	ctx, data, run, _ := round17OwnerFixture(t)
	checkpointPort := AcknowledgedTaskCheckpointPort{Data: data}
	outcomePort := AcknowledgedTaskOutcomePort{Data: data}
	first := round18Checkpoint(run, 1)
	if err := checkpointPort.SaveCheckpoint(ctx, run, first); err != nil {
		t.Fatal(err)
	}
	run.CheckpointID = &first.CheckpointID
	run.Generation, run.ExecutionAttemptID = 2, "second-attempt"
	second := round18Checkpoint(run, 2)
	if err := checkpointPort.SaveCheckpoint(ctx, run, second); err != nil {
		t.Fatal(err)
	}
	run.CheckpointID = &second.CheckpointID
	if err := outcomePort.SaveOutcome(ctx, run, "failed", nil, "failed", "private"); err != nil {
		t.Fatal(err)
	}
	if err := outcomePort.SaveOutcome(ctx, run, "failed", nil, "failed", "private"); err != nil {
		t.Fatalf("idempotent terminal replay failed: %v", err)
	}
	var cursor ownedTaskCommitCursor
	resource := data.resources["checkpoint/"+taskOwnerCommitCursorID(run)]
	if resource == nil || resource.Revision != 3 || json.Unmarshal(resource.Body, &cursor) != nil || !cursor.Terminal || cursor.CheckpointVersion != 2 || cursor.Generation != 2 {
		t.Fatal("Owner revision cursor lost sequential history or terminal fence")
	}
	third := round18Checkpoint(run, 3)
	if err := checkpointPort.SaveCheckpoint(ctx, run, third); err == nil {
		t.Fatal("checkpoint accepted after Owner terminal")
	}
	if data.resources["checkpoint/task/checkpoint/"+third.CheckpointID] != nil {
		t.Fatal("late checkpoint was written")
	}
	resource.Body = json.RawMessage(`{"taskRunId":"another-task"}`)
	if err := checkpointPort.SaveCheckpoint(ctx, run, third); err == nil {
		t.Fatal("corrupt Owner CAS cursor was trusted")
	}
}
