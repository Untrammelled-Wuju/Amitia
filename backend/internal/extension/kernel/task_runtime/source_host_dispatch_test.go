package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestOwnedSourceTaskNativeDispatchChecksSourcePermissionAndExactOwnerAck(t *testing.T) {
	for _, scenario := range []string{"valid", "permission-denied", "owner-unconfirmed", "cancelled", "bad-result-hash"} {
		t.Run(scenario, func(t *testing.T) {
			host := actualSourceTaskHost(t, `module.exports=async(input,ctx)=>{const value=await ctx.host.executeTool('native.echo',{text:'<>&中文',n:1.2e3});await ctx.host.emitEvent('extension.extension.updated',{text:value.text});return {success:true,output:{value,capabilities:ctx.host.capabilities}};};`)
			_, _, authority, root, _ := taskAuthorityFixture(t)
			definition := &TaskDefinition{TaskID: "task", ExtensionID: "extension", ModuleID: "module", Entry: filepath.Base(host.config.EntryPath), EntryHash: host.config.EntryHash, InstalledGeneration: 2, DefinitionHash: "source-local-hash", PermissionRequirementStrings: []string{"service.tool.execute", "event.emit"}}
			if err := PinTaskEntry(t.Context(), host.config.WorkDir, definition); err != nil {
				t.Fatal(err)
			}
			config := DefaultTaskRuntimeConfig()
			config.NodeEnvironmentResolver = sourceNodeResolver{host.config.NodePath}
			config.HostArtifactResolver = sourceHostResolver{host.config.HostPath}
			config.WorkspaceRoot = t.TempDir()
			config.EntryResolver = func(ctx context.Context, def *TaskDefinition) (string, error) {
				return ResolveTaskEntry(ctx, host.config.WorkDir, def)
			}
			config.InstalledExecutionLease = func(context.Context, *TaskDefinition) (string, func(), error) {
				return host.config.WorkDir, func() {}, nil
			}
			config.InstalledDefinitionValidator = func(context.Context, *TaskDefinition) error { return nil }
			config.SourcePermissionGuard = NewSourceTaskPermissionGuard(taskPermissionEvaluatorFunc(func(context.Context, permission.PermissionEvaluationRequest) permission.PermissionEvaluationResult {
				return permission.PermissionEvaluationResult{Decision: permission.DecisionAllow}
			}))
			permissionCalls := 0
			config.SourceHostPermissionGuard = NewSourceTaskHostPermissionGuard(taskPermissionEvaluatorFunc(func(_ context.Context, request permission.PermissionEvaluationRequest) permission.PermissionEvaluationResult {
				permissionCalls++
				if request.ExecutionContext.DeviceID.String() != authority.TargetDeviceID || request.Generation != 2 || request.Subject.ModuleID != "module" {
					t.Error("native Source permission lost original identity")
				}
				if scenario == "permission-denied" {
					return permission.PermissionEvaluationResult{Decision: permission.DecisionDeny}
				}
				return permission.PermissionEvaluationResult{Decision: permission.DecisionAllow}
			}), func(context.Context, string) ([]permission.PermissionRequirement, error) { return nil, nil })
			config.SourceHostCapabilities = SourceTaskCapabilities{ExecuteTool: true, EmitEvent: true}
			store := &ownedCallbackStore{definition: definition}
			service := NewTaskRuntimeService(store, config)
			root.TaskDefinitionID, root.TaskRunID, root.Generation, root.ExecutionAttemptID = "task", "run", 2, "attempt"
			root.ExtensionID, root.ModuleID = "extension", "module"
			root.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
			input := json.RawMessage(`{}`)
			root.Input, root.InputHash, root.Attempt, root.MaxAttempts = nil, hashBytes(input), 1, 1
			root.ExecutionPlacement = TaskExecutionPlacementDevice
			root.ExecutionTarget = TaskExecutionTarget{SpaceID: "core", DeviceID: runtimeidentity.DeviceID(authority.TargetDeviceID), RuntimeID: "runtime", RuntimeSessionID: "session", ConnectionGeneration: 4}
			ctx, cancel := context.WithTimeout(coordination.WithScope(t.Context(), authority), 15*time.Second)
			defer cancel()
			pin, err := service.DescribeInstalledTask(ctx, "task", authority.TargetDeviceID)
			if err != nil {
				t.Fatal(err)
			}
			encodedRoot, _ := json.Marshal(root)
			encodedScope, _ := json.Marshal(authority)
			encodedPin, _ := json.Marshal(pin)
			dispatch := protocol.TaskDispatchPayload{TaskRunID: "run", TaskDefinitionID: "task", TaskGeneration: 2, AttemptID: "attempt", LeaseID: "lease", AuthorityCallID: "authority", DeviceID: root.ExecutionTarget.DeviceID, RuntimeID: "runtime", RuntimeSessionID: "session", ConnectionGeneration: 4, RootTaskMetadata: encodedRoot, OwnedExecutionScope: encodedScope, TargetDefinitionPin: encodedPin, Input: input}
			data := &taskInputData{}
			copy := CloneTaskRun(root)
			copy.Input = input
			if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, copy); err != nil {
				t.Fatal(err)
			}
			ownerCalls := 0
			port := AcknowledgedTaskHostPort{Data: data, Executor: taskHostExecutorFunc(func(_ context.Context, _ *TaskRun, _ *TaskDefinition, _ string, method string, call TaskHostNativeCall) (json.RawMessage, error) {
				ownerCalls++
				if scenario == "cancelled" {
					cancel()
					return nil, context.Canceled
				}
				if method == "task.host.executeTool" {
					return json.Marshal(map[string]any{"result": call.Input})
				}
				return json.RawMessage(`{"confirmed":true,"eventId":"event","outboxId":"outbox"}`), nil
			})}
			call := func(current context.Context, id, method string, params json.RawMessage) (json.RawMessage, error) {
				if scenario == "owner-unconfirmed" {
					return nil, errors.New("owner ACK unavailable")
				}
				result, err := port.Call(current, root, definition, id, method, params)
				if err != nil {
					return nil, err
				}
				if scenario == "bad-result-hash" {
					var ack TaskHostNativeConfirmation
					_ = json.Unmarshal(result, &ack)
					ack.ResultBytes = json.RawMessage(`{"result":"foreign"}`)
					return json.Marshal(ack)
				}
				return result, nil
			}
			outcome, err := service.ExecuteOwnedSourceDispatch(ctx, dispatch, call)
			if scenario == "valid" {
				if err != nil || ownerCalls != 2 || permissionCalls < 2 {
					t.Fatalf("valid native source dispatch failed: %+v %v %d %d", outcome, err, ownerCalls, permissionCalls)
				}
				var decoded struct {
					Value        struct{ Text string }
					Capabilities SourceTaskCapabilities
				}
				if json.Unmarshal(outcome.Result, &decoded) != nil || decoded.Value.Text != "<>&中文" || !decoded.Capabilities.ExecuteTool || !decoded.Capabilities.EmitEvent {
					t.Fatalf("native source SDK result changed: %s", outcome.Result)
				}
			} else if err == nil {
				t.Fatalf("invalid native source dispatch succeeded: %+v", outcome)
			}
			if scenario == "permission-denied" && ownerCalls != 0 {
				t.Fatal("Source permission denial reached owner Native execution")
			}
			if store.run != nil || store.results != 0 {
				t.Fatal("Source task mirrored owner queue or result")
			}
		})
	}
}
