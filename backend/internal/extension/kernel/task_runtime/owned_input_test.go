package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type taskInputData struct {
	coordination.DataPort
	resource    *coordination.Resource
	resources   map[string]*coordination.Resource
	wrongAck    bool
	afterCommit func()
}

func (p *taskInputData) Commit(_ context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	if p.resources == nil {
		p.resources = make(map[string]*coordination.Resource)
	}
	versions := make(map[string]int64)
	for _, mutation := range commit.Mutations {
		p.resource = &coordination.Resource{OwnerID: commit.Scope.ResourceOwnerID, RoleID: mutation.RoleID, Kind: mutation.Kind, ID: mutation.ID, Revision: mutation.ExpectedRevision + 1, Body: append(json.RawMessage(nil), mutation.Body...)}
		p.resources[mutation.Kind+"/"+mutation.ID] = p.resource
		versions[mutation.Kind+"/"+mutation.ID] = p.resource.Revision
	}
	owner := commit.Scope.ResourceOwnerID
	if p.wrongAck {
		owner = "another-owner"
	}
	if p.afterCommit != nil {
		p.afterCommit()
	}
	return coordination.Acknowledgement{OwnerID: owner, RequestID: commit.Scope.RequestID, Versions: versions}, nil
}

func (p *taskInputData) Resource(_ context.Context, _ coordination.ExecutionScope, kind, id string) (*coordination.Resource, error) {
	resource := p.resources[kind+"/"+id]
	if resource == nil {
		return nil, nil
	}
	copy := *resource
	copy.Body = append(json.RawMessage(nil), resource.Body...)
	return &copy, nil
}

func TestOwnedTaskInputRequiresOwnerAckAndExactPersistentAuthority(t *testing.T) {
	_, _, authority, run, definition := taskAuthorityFixture(t)
	run.TaskDefinitionID, run.Input = definition.TaskID, json.RawMessage("{\n  \"private\": \"device input\"\n}")
	run.InputHash = hashBytes(run.Input)
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	for _, coordinated := range []bool{false, true} {
		scope := authority
		scope.Coordinated = coordinated
		if coordinated {
			scope.ResourceOwnerID, scope.RoleOwnerID = scope.CoreID, scope.CoreID
		}
		ctx := coordination.WithScope(t.Context(), scope)
		data := &taskInputData{}
		port := AcknowledgedTaskInputPort{Data: data}
		if err := port.SaveInput(ctx, run); err != nil || data.resource.OwnerID != scope.ResourceOwnerID {
			t.Fatalf("wrong task input owner: %v", err)
		}
		metadata := CloneTaskRun(run)
		metadata.Input = nil
		input, err := port.Input(ctx, metadata)
		if err != nil || string(input) != string(run.Input) || len(metadata.Input) != 0 {
			t.Fatalf("input hydration changed persistent metadata: %s %v", input, err)
		}
		changed := scope
		changed.RoleRevision++
		if _, err := port.Input(coordination.WithScope(t.Context(), changed), metadata); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
			t.Fatalf("changed authorization reused input: %v", err)
		}
		data.wrongAck = true
		if err := port.SaveInput(ctx, run); err == nil {
			t.Fatal("wrong owner acknowledgement accepted")
		}
		data.resource.Body = json.RawMessage(`{}`)
		if _, err := port.Input(ctx, metadata); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
			t.Fatalf("changed input accepted: %v", err)
		}
	}
}

type taskInputQueueStore struct {
	TaskStore
	run    *TaskRun
	queued int
}

func (s *taskInputQueueStore) WithinTaskTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (s *taskInputQueueStore) PutTaskRun(_ context.Context, run *TaskRun) error {
	s.run = CloneTaskRun(run)
	return nil
}
func (s *taskInputQueueStore) EnqueueTask(context.Context, *TaskQueueEntry) error {
	s.queued++
	return nil
}

func (s *taskInputQueueStore) GetTaskRun(context.Context, string) (*TaskRun, error) {
	return CloneTaskRun(s.run), nil
}

func (s *taskInputQueueStore) GetTaskDefinition(context.Context, string) (*TaskDefinition, error) {
	return &TaskDefinition{TaskID: "task", ExtensionID: "extension", ModuleID: "module", Idempotent: true}, nil
}

func TestOwnedTaskEnqueueStoresOnlyMetadataAfterOwnerInputConfirmation(t *testing.T) {
	service, _, authority, saved, definition := taskAuthorityFixture(t)
	service.config.OwnedExecutionGuard = func(ctx context.Context, scope coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
		return coordination.WithScope(ctx, scope), func() {}, nil
	}
	data := &taskInputData{}
	service.config.OwnedInputs = AcknowledgedTaskInputPort{Data: data}
	store := &taskInputQueueStore{}
	service.store = store
	service.queue = NewTaskQueue(store, "test-owner", service.config.LeaseDuration)
	atomic.StoreInt32(&service.dispatching, 1)
	ctx := coordination.WithScope(t.Context(), authority)
	request := EnqueueTaskRequest{ScopeSnapshotID: saved.ScopeSnapshotID, InvocationID: saved.InvocationID, Input: json.RawMessage(`{"private":"input"}`)}
	result, err := service.Enqueue(ctx, request, definition)
	if err != nil || !result.Queued || store.run == nil || len(store.run.Input) != 0 || store.run.InputHash == "" || store.queued != 1 {
		t.Fatalf("owned input copied into queue: %+v %v", store.run, err)
	}
	if data.resource == nil || data.resource.OwnerID != authority.ResourceOwnerID {
		t.Fatal("input owner did not receive data before enqueue")
	}
	data.wrongAck = true
	if _, err := service.Enqueue(ctx, request, definition); err == nil || store.queued != 1 {
		t.Fatalf("unconfirmed input queued: %v", err)
	}
	denied := coordination.WithAdditionalGuard(ctx, func(context.Context) error { return coordination.ErrScopeExpired })
	if _, err := service.Enqueue(denied, request, definition); !errors.Is(err, coordination.ErrScopeExpired) || store.queued != 1 {
		t.Fatalf("expired input queued: %v", err)
	}
}

func TestOwnedTaskRetryPreservesAuthorizationAndOwnerInputInsteadOfDowngrading(t *testing.T) {
	service, _, authority, saved, definition := taskAuthorityFixture(t)
	definition.Idempotent = true
	saved.TaskDefinitionID = definition.TaskID
	saved.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	saved.Status, saved.Attempt, saved.MaxAttempts = RunStatusFailed, 1, 3
	saved.Input = json.RawMessage(`{"private":"retry input"}`)
	saved.InputHash = hashBytes(saved.Input)
	data := &taskInputData{}
	port := AcknowledgedTaskInputPort{Data: data}
	ctx := coordination.WithScope(t.Context(), authority)
	if err := port.SaveInput(ctx, saved); err != nil {
		t.Fatal(err)
	}
	saved.Input = nil
	store := &taskInputQueueStore{run: CloneTaskRun(saved)}
	service.store, service.queue = store, NewTaskQueue(store, "test-owner", service.config.LeaseDuration)
	service.config.OwnedInputs = port
	service.config.OwnedExecutionGuard = func(ctx context.Context, scope coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
		return coordination.WithScope(ctx, scope), func() {}, nil
	}
	atomic.StoreInt32(&service.dispatching, 1)
	retried, err := service.Retry(t.Context(), saved.TaskRunID)
	if err != nil || retried.ScopeSnapshotID != saved.ScopeSnapshotID || retried.InvocationID != saved.InvocationID || retried.DefinitionFingerprint != saved.DefinitionFingerprint || len(retried.Input) != 0 || store.queued != 1 || retried.TaskRunID == saved.TaskRunID {
		t.Fatalf("retry lost owner authorization: %+v %v", retried, err)
	}
	input, err := port.Input(ctx, retried)
	if err != nil || string(input) != `{"private":"retry input"}` {
		t.Fatalf("retry lost acknowledged owner input: %s %v", input, err)
	}
	store.run.Status = RunStatusSucceeded
	if _, err := service.Retry(t.Context(), retried.TaskRunID); !IsTaskErrorCode(err, ErrTaskNotRetryable) || store.queued != 1 {
		t.Fatalf("completed owned task repeated execution: %v", err)
	}
}
