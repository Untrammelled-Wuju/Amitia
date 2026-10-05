package task_runtime

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedTaskStorageConfirmsOwnerAndSeparatesTaskNamespaces(t *testing.T) {
	_, _, authority, run, definition := taskAuthorityFixture(t)
	run.TaskDefinitionID, run.Input = definition.TaskID, json.RawMessage(`{"private":"input"}`)
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	run.InputHash, run.Generation, run.ExecutionAttemptID = hashBytes(run.Input), 1, "attempt"
	ctx := coordination.WithScope(t.Context(), authority)
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	port := AcknowledgedTaskStoragePort{Data: data}
	set := json.RawMessage(`{"task_run_id":"run","key":"cursor","value":{"private":"owner value"}}`)
	data.wrongAck = true
	if _, err := port.Call(ctx, run, "first", "task.storage.set", set); err == nil {
		t.Fatal("unconfirmed owner storage reported saved")
	}
	data.wrongAck = false
	if _, err := port.Call(ctx, run, "second", "task.storage.set", set); err != nil {
		t.Fatal(err)
	}
	get := json.RawMessage(`{"task_run_id":"run","key":"cursor"}`)
	if value, err := port.Call(ctx, run, "third", "task.storage.get", get); err != nil || !strings.Contains(string(value), "owner value") {
		t.Fatalf("owner storage lost value: %s %v", value, err)
	}
	changed := authority
	changed.RoleRevision++
	if _, err := port.Call(coordination.WithScope(t.Context(), changed), run, "changed", "task.storage.get", get); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("changed authority reused owner storage: %v", err)
	}
	other := CloneTaskRun(run)
	other.TaskRunID, other.Input = "other-run", json.RawMessage(`{"private":"input"}`)
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, other); err != nil {
		t.Fatal(err)
	}
	other.Input = nil
	if value, err := port.Call(ctx, other, "other", "task.storage.get", json.RawMessage(`{"task_run_id":"other-run","key":"cursor"}`)); err != nil || string(value) != `{"value":null}` {
		t.Fatalf("task namespace borrowed another task value: %s %v", value, err)
	}
	if _, err := port.Call(ctx, run, "delete", "task.storage.delete", get); err != nil {
		t.Fatal(err)
	}
	if value, err := port.Call(ctx, run, "deleted", "task.storage.get", get); err != nil || string(value) != `{"value":null}` {
		t.Fatalf("deleted task value returned: %s %v", value, err)
	}
	for index := 0; index < 65; index++ {
		params := json.RawMessage(fmt.Sprintf(`{"task_run_id":"run","key":"key-%d","value":1}`, index))
		_, err := port.Call(ctx, run, fmt.Sprintf("bounded-%d", index), "task.storage.set", params)
		if index < 64 && err != nil || index == 64 && err == nil {
			t.Fatalf("storage key budget failed: %d %v", index, err)
		}
	}
}
