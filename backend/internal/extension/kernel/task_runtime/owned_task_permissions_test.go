package task_runtime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type sourcePermissionProviderFunc func(context.Context, SourceTaskPermissionRequest) error

type ownedPermissionPortFunc func(context.Context, *TaskRun, *TaskDefinition, TargetTaskDefinitionPin, json.RawMessage) error

func (f ownedPermissionPortFunc) Prepare(ctx context.Context, run *TaskRun, definition *TaskDefinition, pin TargetTaskDefinitionPin, input json.RawMessage) error {
	return f(ctx, run, definition, pin, input)
}

type ownedDefinitionPortFunc func(context.Context, *TaskRun, *TaskDefinition) (TargetTaskDefinitionPin, error)

func (f ownedDefinitionPortFunc) Prepare(ctx context.Context, run *TaskRun, definition *TaskDefinition) (TargetTaskDefinitionPin, error) {
	return f(ctx, run, definition)
}

func (f sourcePermissionProviderFunc) TargetTaskPermissions(ctx context.Context, request SourceTaskPermissionRequest) error {
	return f(ctx, request)
}

func TestOwnedTaskPermissionPreparationPreservesNewGenerationAndOwnerInput(t *testing.T) {
	ctx, run, definition, input := sourcePermissionFixture(t)
	scope, _ := coordination.FromContext(ctx)
	run.InvocationID, run.ScopeSnapshotID, run.Generation = run.TaskRunID, run.TaskRunID, 3
	run.Input = append(json.RawMessage(nil), input...)
	pin := TargetTaskDefinitionPin{DeviceID: scope.TargetDeviceID, TaskID: definition.TaskID, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, InstalledGeneration: definition.InstalledGeneration, EntryHash: "sha256:" + hashBytes([]byte("entry"))}
	definition.EntryHash = pin.EntryHash
	pin.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	pin.PortableFingerprint, _ = portableTaskDefinitionFingerprint(definition)
	var calls int
	port := AcknowledgedTaskPermissionPort{Provider: sourcePermissionProviderFunc(func(_ context.Context, request SourceTaskPermissionRequest) error {
		calls++
		if request.Run.Generation != 3 || request.Scope != scope || request.Target != pin || len(request.Run.Input) != 0 || request.Run.InvocationID != run.TaskRunID || string(request.Input) != string(input) {
			t.Fatal("新执行代次或所有者输入在权限准备中丢失")
		}
		return coordination.ErrCapabilityGrant
	})}
	if err := port.Prepare(ctx, run, definition, pin, input); err != coordination.ErrCapabilityGrant || calls != 1 {
		t.Fatal("目标设备拒绝后继续执行")
	}
	if string(run.Input) != string(input) {
		t.Fatal("权限准备修改了所有者输入")
	}
	if err := (AcknowledgedTaskPermissionPort{}).Prepare(ctx, run, definition, pin, input); !IsTaskErrorCode(err, ErrTaskPermissionDenied) {
		t.Fatal("缺少来源权限端口仍准备执行")
	}
	run.InputHash = hashBytes([]byte("foreign"))
	if err := port.Prepare(ctx, run, definition, pin, input); !IsTaskErrorCode(err, ErrTaskPermissionDenied) || calls != 1 {
		t.Fatal("被替换的输入参与资源审批")
	}
}

func TestSourceTaskSingleApprovalNewGenerationRequiresIndependentDecision(t *testing.T) {
	ctx, binding := sourceTaskApprovalFixture(t)
	ledger := NewSourceTaskApprovalLedger()
	old, err := ledger.Request(ctx, binding)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Decide(ctx, old.ID, 1, binding, true); err != nil {
		t.Fatal(err)
	}
	resumed := binding
	resumed.TaskGeneration++
	value, err := ledger.Request(ctx, resumed)
	if err != nil || value.ID == old.ID || value.Status != "pending" {
		t.Fatalf("恢复任务借用了旧单次审批: %+v %v", value, err)
	}
}
