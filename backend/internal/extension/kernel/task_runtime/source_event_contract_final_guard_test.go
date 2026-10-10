package task_runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/event"
)

type nativeContractStore struct {
	TaskStore
	definition *TaskDefinition
}

func (s nativeContractStore) GetTaskDefinition(context.Context, string) (*TaskDefinition, error) {
	copy := *s.definition
	return &copy, nil
}

func TestSourceEventContractRechecksActualPermissionAfterResolvingSchema(t *testing.T) {
	ctx, run, definition, _ := sourcePermissionFixture(t)
	scope, _ := coordination.FromContext(ctx)
	scope.Coordinated, scope.ResourceOwnerID, scope.RoleOwnerID = true, scope.CoreID, scope.CoreID
	ctx = context.WithValue(coordination.WithScope(ctx, scope), sourceTaskPreflightKey{}, true)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "task.cjs"), []byte("module.exports=async()=>({success:true})"), 0600); err != nil {
		t.Fatal(err)
	}
	definition.Entry = "task.cjs"
	definition.PermissionRequirementStrings = []string{"event.emit"}
	if err := PinTaskEntry(ctx, root, definition); err != nil {
		t.Fatal(err)
	}
	contract := event.EventTypeDefinition{EventTypeID: "extension.extension.updated", Version: 1}
	contract.DefinitionHash = contract.Hash()
	config := DefaultTaskRuntimeConfig()
	config.SourceHostCapabilities = SourceTaskCapabilities{EmitEvent: true}
	config.InstalledDefinitionValidator = func(context.Context, *TaskDefinition) error { return nil }
	config.EntryResolver = func(ctx context.Context, def *TaskDefinition) (string, error) {
		return ResolveTaskEntry(ctx, root, def)
	}
	allowed := true
	calls := 0
	payload := json.RawMessage("{ \"text\":\"<>&中文\", \"number\":1.20e+3 }")
	config.SourceHostPermissionGuard = func(_ context.Context, _ *TaskRun, _ *TaskDefinition, _ string, call TaskHostNativeCall) error {
		calls++
		if string(call.Payload) != string(payload) {
			t.Fatal("final authorization used rewritten/original task input")
		}
		if !allowed {
			return coordination.ErrScopeExpired
		}
		return nil
	}
	config.SourceEventContracts = func(context.Context, *TaskDefinition) ([]event.EventTypeDefinition, error) {
		allowed = false
		return []event.EventTypeDefinition{contract}, nil
	}
	service := NewTaskRuntimeService(nativeContractStore{definition: definition}, config)
	request := TaskHostSourceEventRequest{Scope: scope, RequestID: "native", Call: TaskHostNativeCall{TaskRunID: run.TaskRunID, Type: string(contract.EventTypeID), Payload: payload}}
	if _, err := service.sourceTaskEventContract(ctx, run, definition, request); err == nil || calls != 1 {
		t.Fatalf("late native revocation escaped contract check: calls=%d err=%v", calls, err)
	}
	config.SourceEventContracts = func(context.Context, *TaskDefinition) ([]event.EventTypeDefinition, error) {
		return []event.EventTypeDefinition{contract}, nil
	}
	service.config.SourceEventContracts = config.SourceEventContracts
	allowed = true
	encoded, err := service.sourceTaskEventContract(ctx, run, definition, request)
	if err != nil {
		t.Fatal(err)
	}
	var confirmed TaskHostEventContract
	if json.Unmarshal(encoded, &confirmed) != nil || confirmed.PayloadHash != hashBytes(payload) {
		t.Fatal("contract did not bind exact original payload bytes")
	}
}
