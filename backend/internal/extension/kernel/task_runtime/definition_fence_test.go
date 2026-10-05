package task_runtime

import (
	"sync/atomic"
	"testing"
)

func TestTaskDefinitionFingerprintRejectsUpdatedDefinitionAndMissingOwnedPin(t *testing.T) {
	def := &TaskDefinition{TaskID: "task", ExtensionID: "extension", ModuleID: "module", Entry: "task.cjs", EntryHash: "sha256:entry", DefinitionHash: "advertised-hash"}
	fingerprint, err := taskDefinitionFingerprint(def)
	if err != nil {
		t.Fatal(err)
	}
	run := &TaskRun{TaskDefinitionID: def.TaskID, ExtensionID: def.ExtensionID, ModuleID: def.ModuleID, DefinitionFingerprint: fingerprint}
	if err := validateTaskDefinition(true, run, def); err != nil {
		t.Fatal(err)
	}
	changed := *def
	changed.EntryHash = "sha256:new-entry"
	if err := validateTaskDefinition(true, run, &changed); !IsTaskErrorCode(err, ErrTaskDefinitionInvalid) {
		t.Fatalf("entry changed under the same advertised definition hash: %v", err)
	}
	changed = *def
	changed.PermissionRequirementStrings = []string{"new-permission"}
	if err := validateTaskDefinition(false, run, &changed); !IsTaskErrorCode(err, ErrTaskDefinitionInvalid) {
		t.Fatalf("permission changed for queued task: %v", err)
	}
	run.DefinitionFingerprint = ""
	if err := validateTaskDefinition(true, run, def); !IsTaskErrorCode(err, ErrTaskDefinitionInvalid) {
		t.Fatalf("owned legacy task accepted without pin: %v", err)
	}
	if err := validateTaskDefinition(false, run, def); err != nil {
		t.Fatalf("ordinary legacy task could not be inspected: %v", err)
	}
}

func TestTaskLocalLifecycleRefusesChangedDefinitionBeforeLaunchingProcess(t *testing.T) {
	def := &TaskDefinition{TaskID: "task", ExtensionID: "extension", ModuleID: "module", Entry: "old.cjs"}
	fingerprint, err := taskDefinitionFingerprint(def)
	if err != nil {
		t.Fatal(err)
	}
	def.Entry = "new.cjs"
	store := &pauseStore{run: &TaskRun{TaskRunID: "run", TaskDefinitionID: "task", ExtensionID: "extension", ModuleID: "module", DefinitionFingerprint: fingerprint, Status: RunStatusQueued, Generation: 1, Revision: 1}, def: def}
	service := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
	atomic.StoreInt32(&service.dispatching, 1)
	service.executeTaskRun(t.Context(), CloneTaskRun(store.run))
	if store.run.Status != RunStatusFailed || store.run.ErrorCode == nil || *store.run.ErrorCode != string(ErrTaskDefinitionInvalid) || store.result != nil || len(service.activeHosts) != 0 {
		t.Fatalf("updated definition reached process execution: %+v", store.run)
	}
}
