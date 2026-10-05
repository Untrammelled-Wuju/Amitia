package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension/kernel/script_host"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
)

type sourceNodeResolver struct{ binary string }

func (r sourceNodeResolver) Resolve(context.Context) (nodeenv.Environment, error) {
	return nodeenv.Environment{NodeBinary: r.binary}, nil
}

type sourceHostResolver struct{ entry string }

func (r sourceHostResolver) Resolve(context.Context, script_host.Kind) (script_host.Artifact, error) {
	return script_host.Artifact{Kind: script_host.KindTaskHost, EntryPath: r.entry}, nil
}

func TestOwnedSourceTaskRunsActualHostWithoutCreatingAnotherTaskQueue(t *testing.T) {
	for _, scenario := range []string{"valid", "device_owner", "checkpoint", "resume", "wrong_cursor", "progress", "progress_unconfirmed", "large_result", "foreign_definition", "foreign_connection", "input_changed", "off", "unconfirmed", "installed_changed"} {
		t.Run(scenario, func(t *testing.T) {
			handler := `module.exports=async(input,ctx)=>{ await ctx.storage.set('value',input.value); const value=await ctx.storage.get('value'); return {success:true,output:{value}}; };`
			if scenario == "checkpoint" {
				handler = `module.exports=async(input,ctx)=>{ await ctx.checkpoint.save({value:input.value}); return {success:true,output:{cursor:ctx.checkpoint.getCurrent().cursor}}; };`
			}
			if scenario == "resume" || scenario == "wrong_cursor" {
				handler = `module.exports=async(input,ctx)=>{ const initial=ctx.checkpoint.getCurrent(); await ctx.checkpoint.save({value:input.value}); return {success:true,output:{initial:initial.cursor,cursor:ctx.checkpoint.getCurrent().cursor}}; };`
			}
			if scenario == "progress" || scenario == "progress_unconfirmed" {
				handler = `module.exports=async(input,ctx)=>{ await ctx.progress.report({current:1,total:2,stage:'working'}); return {success:true,output:{done:true}}; };`
			}
			if scenario == "large_result" {
				handler = `module.exports=async()=>({success:true,output:{private:'中'.repeat(24000)}});`
			}
			if scenario == "installed_changed" {
				handler = `module.exports=async(input,ctx)=>{ await ctx.storage.set('value',input.value); await new Promise(resolve=>setTimeout(resolve,5000)); return {success:true,output:{late:true}}; };`
			}
			host := actualSourceTaskHost(t, handler)
			_, _, authority, root, _ := taskAuthorityFixture(t)
			authority.Coordinated, authority.ResourceOwnerID, authority.RoleOwnerID = true, "core", "core"
			if scenario == "device_owner" {
				authority.Coordinated, authority.ResourceOwnerID, authority.RoleOwnerID = false, authority.TargetDeviceID, authority.TargetDeviceID
			}
			definition := &TaskDefinition{TaskID: "task", ExtensionID: "extension", ModuleID: "module", Entry: filepath.Base(host.config.EntryPath), EntryHash: host.config.EntryHash, InstalledGeneration: 2, DefinitionHash: "source-local-hash"}
			var installed atomic.Int64
			installed.Store(2)
			config := DefaultTaskRuntimeConfig()
			config.NodeEnvironmentResolver = sourceNodeResolver{host.config.NodePath}
			config.HostArtifactResolver = sourceHostResolver{host.config.HostPath}
			config.WorkspaceRoot = t.TempDir()
			config.EntryResolver = func(ctx context.Context, def *TaskDefinition) (string, error) {
				return ResolveTaskEntry(ctx, host.config.WorkDir, def)
			}
			config.InstalledDefinitionValidator = func(_ context.Context, def *TaskDefinition) error {
				if def.InstalledGeneration != installed.Load() {
					return coordination.ErrScopeExpired
				}
				return nil
			}
			store := &ownedCallbackStore{definition: definition}
			service := NewTaskRuntimeService(store, config)
			root.TaskDefinitionID, root.TaskRunID, root.Generation, root.ExecutionAttemptID = "task", "run", 2, "attempt"
			root.ExtensionID, root.ModuleID = "extension", "module"
			root.DefinitionFingerprint = hashBytes([]byte("core-independent-definition"))
			input := json.RawMessage(`{"value":"only-at-core"}`)
			root.Input, root.InputHash, root.Attempt, root.MaxAttempts = nil, hashBytes(input), 1, 1
			root.ExecutionPlacement = TaskExecutionPlacementDevice
			root.ExecutionTarget = TaskExecutionTarget{SpaceID: "core", DeviceID: runtimeidentity.DeviceID(authority.TargetDeviceID), RuntimeID: "runtime", RuntimeSessionID: "session", ConnectionGeneration: 4}
			var resume json.RawMessage
			if scenario == "resume" || scenario == "wrong_cursor" {
				root.CheckpointID = strPtr("confirmed-cp")
				payload := json.RawMessage(`{"cursor":7,"data":{"value":"old-core-value"}}`)
				cp := TaskCheckpoint{CheckpointID: *root.CheckpointID, TaskRunID: root.TaskRunID, InputHash: root.InputHash, Version: 7, Payload: payload, PayloadHash: hashBytes(payload)}
				if scenario == "wrong_cursor" {
					cp.Version++
				}
				resume, _ = json.Marshal(cp)
			}
			encodedRoot, _ := json.Marshal(root)
			encodedScope, _ := json.Marshal(authority)
			ctx, cancel := context.WithTimeout(coordination.WithScope(t.Context(), authority), 15*time.Second)
			defer cancel()
			pin, err := service.DescribeInstalledTask(ctx, "task", authority.TargetDeviceID)
			if err != nil {
				t.Fatal(err)
			}
			encodedPin, _ := json.Marshal(pin)
			dispatch := protocol.TaskDispatchPayload{TaskRunID: root.TaskRunID, TaskDefinitionID: "task", TaskGeneration: 2, AttemptID: "attempt", LeaseID: "lease", AuthorityCallID: "authority", DeviceID: root.ExecutionTarget.DeviceID, RuntimeID: "runtime", RuntimeSessionID: "session", ConnectionGeneration: 4, RootTaskMetadata: encodedRoot, OwnedExecutionScope: encodedScope, TargetDefinitionPin: encodedPin, Input: input, ResumeCheckpoint: resume, ProgressBase: 9}
			switch scenario {
			case "foreign_definition":
				pin.InstalledGeneration++
				dispatch.TargetDefinitionPin, _ = json.Marshal(pin)
			case "foreign_connection":
				dispatch.ConnectionGeneration++
			case "input_changed":
				dispatch.Input = json.RawMessage(`{"value":"other"}`)
			case "off":
				authority.Coordinated = false
				ctx = coordination.WithScope(ctx, authority)
			}
			var calls atomic.Int32
			call := func(_ context.Context, _, method string, params json.RawMessage) (json.RawMessage, error) {
				calls.Add(1)
				if scenario == "unconfirmed" {
					return nil, errors.New("owner offline")
				}
				if scenario == "installed_changed" {
					installed.Store(3)
				}
				switch method {
				case "task.storage.set":
					return json.RawMessage(`{"confirmed":true}`), nil
				case "task.storage.get":
					return json.RawMessage(`{"value":"only-at-core"}`), nil
				case "task.checkpoint.save":
					var value struct {
						Version int64 `json:"version"`
					}
					if json.Unmarshal(params, &value) != nil || scenario != "resume" && value.Version != 1 || scenario == "resume" && value.Version != 8 {
						t.Error("checkpoint cursor lost")
					}
					return json.Marshal(map[string]int64{"version": value.Version})
				case "task.progress.save":
					var value struct {
						Sequence int64 `json:"sequence"`
					}
					if json.Unmarshal(params, &value) != nil || value.Sequence != 10 {
						t.Error("progress did not continue the Core sequence")
					}
					if scenario == "progress_unconfirmed" {
						return json.RawMessage(`{"confirmed":false}`), nil
					}
					return json.RawMessage(`{"confirmed":true}`), nil
				case "task.artifact.saveData":
					return json.Marshal(map[string]string{"artifactId": "artifact-" + hashBytes([]byte("owned-large-result"))})
				default:
					return nil, errors.New("unexpected owner method")
				}
			}
			outcome, err := service.ExecuteOwnedSourceDispatch(ctx, dispatch, call)
			valid := scenario == "valid" || scenario == "checkpoint" || scenario == "large_result" || scenario == "resume" || scenario == "progress" || scenario == "device_owner"
			if valid && err != nil || !valid && err == nil {
				t.Fatalf("source scenario=%s outcome=%+v err=%v", scenario, outcome, err)
			}
			if store.run != nil || store.results != 0 {
				t.Fatal("source created another task record or mirrored result")
			}
			if scenario == "valid" && string(outcome.Result) != `{"value":"only-at-core"}` {
				t.Fatalf("wrong source result: %s", outcome.Result)
			}
			if scenario == "large_result" && (outcome.ResultArtifactID == "" || len(outcome.Result) != 0) {
				t.Fatal("large result did not use Core owner artifact")
			}
			if scenario == "foreign_definition" || scenario == "foreign_connection" || scenario == "input_changed" || scenario == "off" {
				if calls.Load() != 0 {
					t.Fatal("invalid source task reached owner data")
				}
			}
		})
	}
}
