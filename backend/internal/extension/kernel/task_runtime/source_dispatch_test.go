package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/script_host"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
)

type sourceNodeResolver struct{ binary string }

type taskTestDiagnostics struct{ t *testing.T }

func (w taskTestDiagnostics) Write(data []byte) (int, error) {
	w.t.Logf("task runtime diagnostic: %s", data)
	return len(data), nil
}

func (r sourceNodeResolver) Resolve(context.Context) (nodeenv.Environment, error) {
	return nodeenv.Environment{NodeBinary: r.binary}, nil
}

type sourceHostResolver struct{ entry string }

func (r sourceHostResolver) Resolve(context.Context, script_host.Kind) (script_host.Artifact, error) {
	return script_host.Artifact{Kind: script_host.KindTaskHost, EntryPath: r.entry}, nil
}

func TestOwnedSourceTaskRunsActualHostWithoutCreatingAnotherTaskQueue(t *testing.T) {
	for _, scenario := range []string{"valid", "device_owner", "checkpoint", "resume", "wrong_cursor", "progress", "progress_unconfirmed", "large_result", "foreign_definition", "foreign_connection", "input_changed", "off", "unconfirmed", "installed_changed", "pause", "pause_unconfirmed", "permissions_missing", "permissions_denied", "permissions_allowed", "permissions_revoked", "catalog_alias", "catalog_alias_foreign", "approval_allowed", "approval_revoked", "approval_replay", "lease_denied", "approval_capacity"} {
		t.Run(scenario, func(t *testing.T) {
			handler := `module.exports=async(input,ctx)=>{ await ctx.storage.set('value',input.value); const value=await ctx.storage.get('value'); return {success:true,output:{value}}; };`
			if scenario == "checkpoint" {
				handler = `module.exports=async(input,ctx)=>{ await ctx.checkpoint.save({value:input.value}); return {success:true,output:{cursor:ctx.checkpoint.getCurrent().cursor}}; };`
			}
			if scenario == "pause" || scenario == "pause_unconfirmed" {
				handler = `module.exports=async(input,ctx)=>{ await ctx.checkpoint.save({value:input.value}); await new Promise(resolve=>setTimeout(resolve,5000)); return {success:true,output:{late:true}}; };`
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
			if scenario == "installed_changed" || scenario == "permissions_revoked" || scenario == "approval_revoked" {
				handler = `module.exports=async(input,ctx)=>{ await ctx.storage.set('value',input.value); await new Promise(resolve=>setTimeout(resolve,5000)); return {success:true,output:{late:true}}; };`
			}
			host := actualSourceTaskHost(t, handler)
			_, _, authority, root, _ := taskAuthorityFixture(t)
			authority.Coordinated, authority.ResourceOwnerID, authority.RoleOwnerID = true, "core", "core"
			if scenario == "device_owner" {
				authority.Coordinated, authority.ResourceOwnerID, authority.RoleOwnerID = false, authority.TargetDeviceID, authority.TargetDeviceID
			}
			definition := &TaskDefinition{TaskID: "task", ExtensionID: "extension", ModuleID: "module", Entry: filepath.Base(host.config.EntryPath), EntryHash: host.config.EntryHash, InstalledGeneration: 2, DefinitionHash: "source-local-hash"}
			definition.Checkpoint = scenario == "pause" || scenario == "pause_unconfirmed"
			if err := PinTaskEntry(t.Context(), host.config.WorkDir, definition); err != nil {
				t.Fatal(err)
			}
			var permissionRevoked atomic.Bool
			if scenario == "permissions_missing" || scenario == "permissions_denied" || scenario == "permissions_allowed" || scenario == "permissions_revoked" || scenario == "approval_allowed" || scenario == "approval_revoked" || scenario == "approval_replay" || scenario == "approval_capacity" {
				definition.PermissionRequirementStrings = []string{"test.source.read"}
			}
			var installed atomic.Int64
			installed.Store(2)
			config := DefaultTaskRuntimeConfig()
			config.ProcessDiagnostics = taskTestDiagnostics{t: t}
			config.NodeEnvironmentResolver = sourceNodeResolver{host.config.NodePath}
			config.HostArtifactResolver = sourceHostResolver{host.config.HostPath}
			config.WorkspaceRoot = t.TempDir()
			if scenario != "permissions_missing" {
				config.SourcePermissionGuard = NewSourceTaskPermissionGuard(taskPermissionEvaluatorFunc(func(context.Context, permission.PermissionEvaluationRequest) permission.PermissionEvaluationResult {
					decision := permission.DecisionAllow
					if scenario == "permissions_denied" || permissionRevoked.Load() {
						decision = permission.DecisionDeny
					}
					return permission.PermissionEvaluationResult{Decision: decision}
				}))
			}
			config.EntryResolver = func(ctx context.Context, def *TaskDefinition) (string, error) {
				return ResolveTaskEntry(ctx, host.config.WorkDir, def)
			}
			var activeLeases atomic.Int32
			var leaseDenied atomic.Bool
			config.InstalledExecutionLease = func(ctx context.Context, def *TaskDefinition) (string, func(), error) {
				if leaseDenied.Load() {
					return "", nil, errors.New("installation is being removed")
				}
				if def.BundleHash != definition.BundleHash {
					return "", nil, errors.New("bundle changed")
				}
				activeLeases.Add(1)
				var once sync.Once
				return host.config.WorkDir, func() { once.Do(func() { activeLeases.Add(-1) }) }, nil
			}
			if scenario == "approval_allowed" || scenario == "approval_revoked" || scenario == "approval_replay" || scenario == "approval_capacity" {
				registry := permission.NewPermissionDefinitionRegistry()
				registry.Register(permission.PermissionDefinition{ID: "test.source.read", Category: permission.CategoryFilesystem, RiskLevel: "high", AllowedScopes: []permission.ScopeType{permission.ScopeExtension}, BackgroundAllowed: true, RequiresPerUse: true, RemoteExecution: permission.RemoteExecutionRequireApproval})
				broker := permission.NewDefaultPermissionBroker(registry, permission.NewMemoryPermissionStorage())
				t.Cleanup(func() { _ = broker.Close() })
				config.SourcePermissionGuard, config.SourceApprovalRecorder = NewSourceTaskPermissionGuard(broker), broker
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
			if scenario == "approval_allowed" || scenario == "approval_revoked" || scenario == "approval_replay" || scenario == "approval_capacity" {
				root.InvocationID, root.ScopeSnapshotID = root.TaskRunID, root.TaskRunID
			}
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
			if scenario == "lease_denied" {
				leaseDenied.Store(true)
			}
			if scenario == "catalog_alias" || scenario == "catalog_alias_foreign" {
				core := authority.CoreID
				if scenario == "catalog_alias_foreign" {
					core = "foreign-core"
				}
				aliased, err := NewCoreDeviceTaskDefinition(core, authority.TargetDeviceID, DeviceTaskCatalogEntry{Definition: *definition, Target: pin})
				if err != nil {
					t.Fatal(err)
				}
				root.TaskDefinitionID = aliased.TaskID
				root.DefinitionFingerprint, _ = taskDefinitionFingerprint(aliased)
				root.ExecutionTarget.SourceTaskDefinitionID = definition.TaskID
				encodedRoot, _ = json.Marshal(root)
			}
			encodedPin, _ := json.Marshal(pin)
			dispatch := protocol.TaskDispatchPayload{TaskRunID: root.TaskRunID, TaskDefinitionID: "task", TaskGeneration: 2, AttemptID: "attempt", LeaseID: "lease", AuthorityCallID: "authority", DeviceID: root.ExecutionTarget.DeviceID, RuntimeID: "runtime", RuntimeSessionID: "session", ConnectionGeneration: 4, RootTaskMetadata: encodedRoot, OwnedExecutionScope: encodedScope, TargetDefinitionPin: encodedPin, Input: input, ResumeCheckpoint: resume, ProgressBase: 9}
			var approvalID string
			if scenario == "approval_allowed" || scenario == "approval_revoked" || scenario == "approval_replay" || scenario == "approval_capacity" {
				request := SourceTaskPermissionRequest{Scope: authority, Run: *root, Target: pin, Input: input}
				ack, err := service.CheckInstalledTaskPermissions(ctx, request)
				if err != nil || ack.Allowed || ack.ApprovalID == "" || ack.ApprovalStatus != "pending" {
					t.Fatalf("未批准的任务没有进入目标审批: %+v %v", ack, err)
				}
				approvalID = ack.ApprovalID
				if _, err := service.DecideSourceTaskApproval(ctx, approvalID, 1, true); err != nil {
					t.Fatal(err)
				}
				ack, err = service.CheckInstalledTaskPermissions(ctx, request)
				if err != nil || !ack.Allowed || ack.ApprovalStatus != "approved" {
					t.Fatalf("已批准的任务仍被拒绝: %+v %v", ack, err)
				}
				if scenario == "approval_replay" {
					value, _ := service.SourceTaskApproval(approvalID)
					if err := service.sourceApprovals.Claim(ctx, approvalID, value.Binding, "other-attempt", "other-lease", time.Now().Add(time.Minute)); err != nil {
						t.Fatal(err)
					}
				}
			}
			if scenario == "approval_capacity" {
				release, err := service.reserveTaskProcess(ctx, definition)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(release)
			}
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
			var pauseStarted atomic.Bool
			pauseDone := make(chan error, 1)
			call := func(_ context.Context, _, method string, params json.RawMessage) (json.RawMessage, error) {
				if activeLeases.Load() < 1 {
					t.Error("actual Node accessed owner data without retaining its installation")
				}
				calls.Add(1)
				if scenario == "unconfirmed" {
					return nil, errors.New("owner offline")
				}
				if scenario == "installed_changed" {
					installed.Store(3)
				}
				if scenario == "permissions_revoked" {
					permissionRevoked.Store(true)
				}
				if scenario == "approval_revoked" {
					service.sourceApprovals.Revoke(approvalID)
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
					pausing := scenario == "pause" || scenario == "pause_unconfirmed"
					if json.Unmarshal(params, &value) != nil || !pausing && scenario != "resume" && value.Version != 1 || scenario == "resume" && value.Version != 8 || pausing && value.Version != 1 && value.Version != 2 {
						t.Error("checkpoint cursor lost")
					}
					if pausing && pauseStarted.CompareAndSwap(false, true) {
						go func() {
							request := protocol.TaskPausePayload{TaskRunID: dispatch.TaskRunID, AttemptID: dispatch.AttemptID, LeaseID: "stale", RuntimeSessionID: dispatch.RuntimeSessionID, ConnectionGeneration: dispatch.ConnectionGeneration}
							if err := service.PauseOwnedSourceDispatch(ctx, request); err == nil {
								t.Error("stale pause stopped the current process")
							}
							request.LeaseID = dispatch.LeaseID
							pauseDone <- service.PauseOwnedSourceDispatch(ctx, request)
						}()
					}
					if scenario == "pause_unconfirmed" && value.Version == 2 {
						return nil, errors.New("owner did not confirm the pause checkpoint")
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
			if scenario == "approval_capacity" {
				approval, approvalErr := service.SourceTaskApproval(approvalID)
				if !errors.Is(err, ErrTaskProcessCapacity) || calls.Load() != 0 || approvalErr != nil || approval.Status != "approved" {
					t.Fatalf("capacity consumed approval or executed task: %v %+v", err, approval)
				}
			}
			if activeLeases.Load() != 0 {
				t.Fatal("source execution leaked its installation lease")
			}
			valid := scenario == "valid" || scenario == "checkpoint" || scenario == "large_result" || scenario == "resume" || scenario == "progress" || scenario == "device_owner" || scenario == "pause" || scenario == "permissions_allowed" || scenario == "catalog_alias" || scenario == "approval_allowed"
			if valid && err != nil || !valid && err == nil {
				t.Fatalf("source scenario=%s outcome=%+v err=%v", scenario, outcome, err)
			}
			if store.run != nil || store.results != 0 {
				t.Fatal("source created another task record or mirrored result")
			}
			if scenario == "pause" || scenario == "pause_unconfirmed" {
				select {
				case pauseErr := <-pauseDone:
					if scenario == "pause" && pauseErr != nil || scenario == "pause_unconfirmed" && pauseErr == nil {
						t.Fatalf("pause confirmation mismatch: %v", pauseErr)
					}
				case <-ctx.Done():
					t.Fatal("source pause did not finish")
				}
				if scenario == "pause" && (outcome.PausedCheckpointVersion != 2 || len(outcome.Result) != 0 || outcome.ResultArtifactID != "") {
					t.Fatalf("paused source reported a finished result: %+v", outcome)
				}
				if _, exists := service.sourceHosts.Load(dispatch.TaskRunID); exists {
					t.Fatal("paused source retained a process binding")
				}
			}
			if scenario == "valid" && string(outcome.Result) != `{"value":"only-at-core"}` {
				t.Fatalf("wrong source result: %s", outcome.Result)
			}
			if scenario == "large_result" && (outcome.ResultArtifactID == "" || len(outcome.Result) != 0) {
				t.Fatal("large result did not use Core owner artifact")
			}
			if scenario == "foreign_definition" || scenario == "foreign_connection" || scenario == "input_changed" || scenario == "off" || scenario == "permissions_missing" || scenario == "permissions_denied" || scenario == "catalog_alias_foreign" {
				if calls.Load() != 0 {
					t.Fatal("invalid source task reached owner data")
				}
			}
		})
	}
}
