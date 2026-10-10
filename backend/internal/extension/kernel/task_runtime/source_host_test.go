package task_runtime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

func TestSourceTaskNativeGuardChecksActualPermissionInputAndTargetEachTime(t *testing.T) {
	ctx, run, definition, _ := sourcePermissionFixture(t)
	definition.PermissionRequirementStrings = []string{"service.tool.execute", "native.file.read", "event.emit"}
	call := TaskHostNativeCall{TaskRunID: run.TaskRunID, ToolID: "native.read", Input: json.RawMessage(`{"path":"allowed"}`)}
	allowed := true
	calls := 0
	guard := NewSourceTaskHostPermissionGuard(taskPermissionEvaluatorFunc(func(_ context.Context, request permission.PermissionEvaluationRequest) permission.PermissionEvaluationResult {
		calls++
		if request.Subject.ID != definition.ModuleID || request.Generation != definition.InstalledGeneration || request.ExecutionContext.DeviceID != run.ExecutionTarget.DeviceID || request.ExecutionContext.RuntimeSessionID != run.ExecutionTarget.RuntimeSessionID || request.InvocationID != run.InvocationID || request.ScopeSnapshotID != run.ScopeSnapshotID || !request.IsBackground {
			t.Error("native operation lost original Source authority")
		}
		if request.Target.ID == call.ToolID && string(request.Input) != string(call.Input) {
			t.Error("native permission evaluated original task input instead of actual tool input")
		}
		for _, requirement := range request.Requirements {
			if requirement.Scope != permission.ScopeForExtension(definition.ExtensionID) {
				t.Error("native permission borrowed tool provider scope")
			}
		}
		if !allowed {
			return permission.PermissionEvaluationResult{Decision: permission.DecisionDeny}
		}
		return permission.PermissionEvaluationResult{Decision: permission.DecisionAllow}
	}), func(context.Context, string) ([]permission.PermissionRequirement, error) {
		return []permission.PermissionRequirement{{PermissionID: "native.file.read", Scope: permission.ScopeForExtension("native-provider")}}, nil
	})
	if err := guard(ctx, run, definition, "task.host.executeTool", call); err != nil {
		t.Fatal(err)
	}
	allowed = false
	if err := guard(ctx, run, definition, "task.host.executeTool", call); err == nil || calls != 2 {
		t.Fatal("revoked actual native permission reused cached allow")
	}
	allowed = true
	definition.PermissionRequirementStrings = []string{"service.tool.execute", "event.emit"}
	if err := guard(ctx, run, definition, "task.host.executeTool", call); err == nil || calls != 2 {
		t.Fatal("undeclared native capability reached evaluator")
	}
	foreign := CloneTaskRun(run)
	foreign.ExecutionTarget.DeviceID = "foreign"
	if err := guard(ctx, foreign, definition, "task.host.executeTool", call); err == nil {
		t.Fatal("native permission used another target")
	}
	event := TaskHostNativeCall{TaskRunID: run.TaskRunID, Type: "extension.extension.updated", Payload: json.RawMessage(`{"text":"<>&中文"}`)}
	if err := guard(ctx, run, definition, "task.host.emitEvent", event); err != nil {
		t.Fatal(err)
	}
	event.Type = "extension.foreign.updated"
	if err := guard(ctx, run, definition, "task.host.emitEvent", event); err == nil {
		t.Fatal("foreign event namespace authorized")
	}
	changed, cancel := context.WithCancel(ctx)
	cancel()
	if err := guard(changed, run, definition, "task.host.emitEvent", TaskHostNativeCall{TaskRunID: run.TaskRunID, Type: "extension.extension.updated", Payload: json.RawMessage(`{}`)}); err == nil {
		t.Fatal("cancelled native permission succeeded")
	}
	if _, ok := coordination.FromContext(ctx); !ok {
		t.Fatal("fixture lost owner scope")
	}
}

func TestSourceTaskNativeGuardRetainsDeclaredConditionsAndScope(t *testing.T) {
	ctx, run, definition, _ := sourcePermissionFixture(t)
	condition := json.RawMessage(`[{"field":"path","operator":"eq","value":"allowed"}]`)
	definition.PermissionRequirementStrings = []string{"service.tool.execute"}
	definition.PermissionRequirements = []permission.PermissionRequirement{{PermissionID: "native.file.read", Scope: permission.ScopeForExtension(definition.ExtensionID), Conditions: condition}}
	calls := 0
	guard := NewSourceTaskHostPermissionGuard(taskPermissionEvaluatorFunc(func(_ context.Context, request permission.PermissionEvaluationRequest) permission.PermissionEvaluationResult {
		calls++
		found := false
		for _, requirement := range request.Requirements {
			if string(requirement.Conditions) == string(condition) && requirement.Scope == definition.PermissionRequirements[0].Scope && !requirement.Optional {
				found = true
			}
		}
		if !found {
			t.Fatal("original task condition or scope discarded")
		}
		return permission.PermissionEvaluationResult{Decision: permission.DecisionAllowPersistent}
	}), func(context.Context, string) ([]permission.PermissionRequirement, error) {
		return []permission.PermissionRequirement{{PermissionID: "native.file.read"}}, nil
	})
	call := TaskHostNativeCall{TaskRunID: run.TaskRunID, ToolID: "native.read", Input: json.RawMessage(`{"path":"allowed"}`)}
	if err := guard(ctx, run, definition, "task.host.executeTool", call); err != nil {
		t.Fatal(err)
	}
	call.Input = json.RawMessage(`{"path":"private"}`)
	if err := guard(ctx, run, definition, "task.host.executeTool", call); err == nil || calls != 1 {
		t.Fatal("persistent approval bypassed declared input condition")
	}
	definition.PermissionRequirements[0].Conditions = json.RawMessage(`{"paths":["allowed"]}`)
	call.Input = json.RawMessage(`{"path":"allowed"}`)
	if err := guard(ctx, run, definition, "task.host.executeTool", call); err == nil || calls != 1 {
		t.Fatal("unsupported permission condition was silently ignored")
	}
}
