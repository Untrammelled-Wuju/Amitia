package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh"
	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	kernel "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
	"github.com/u-ai/backend/internal/extension/kernel/script_host"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
)

type meshTaskNodeResolver struct{ binary string }

func taskCallbackState(run *task_runtime.TaskRun) string {
	if run == nil {
		return "missing"
	}
	return fmt.Sprintf("status=%s generation=%d revision=%d attempt=%s", run.Status, run.Generation, run.Revision, run.ExecutionAttemptID)
}

func (r meshTaskNodeResolver) Resolve(context.Context) (nodeenv.Environment, error) {
	return nodeenv.Environment{NodeBinary: r.binary}, nil
}

type meshTaskHostResolver struct{ entry string }

func (r meshTaskHostResolver) Resolve(context.Context, script_host.Kind) (script_host.Artifact, error) {
	return script_host.Artifact{Kind: script_host.KindTaskHost, EntryPath: r.entry}, nil
}

func sourceTaskTLSFixture(t *testing.T, db *sql.DB) (*task_runtime.TaskRuntimeService, *task_runtime.TaskDefinition) {
	t.Helper()
	binary := os.Getenv("AMITIA_TEST_NODE")
	if binary == "" {
		t.Fatal("真实设备任务联测必须使用项目 Node 运行时")
	}
	root, err := filepath.Abs("../../../runtime/task-host")
	if err != nil {
		t.Fatal(err)
	}
	compiler, _ := json.Marshal(filepath.Join(root, "node_modules/typescript/lib/typescript.js"))
	sourceRoot, _ := json.Marshal(filepath.Join(root, "src"))
	dir := t.TempDir()
	compile := fmt.Sprintf(`const fs=require('node:fs'),path=require('node:path'),ts=require(%s),root=%s,out=process.argv[1];for(const file of fs.readdirSync(root)){if(!file.endsWith('.ts'))continue;const source=fs.readFileSync(path.join(root,file),'utf8');fs.writeFileSync(path.join(out,file.slice(0,-3)+'.js'),ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ES2022,target:ts.ScriptTarget.ES2022}}).outputText);}fs.writeFileSync(path.join(out,'package.json'),JSON.stringify({type:'module'}));`, compiler, sourceRoot)
	if output, err := exec.Command(binary, "-e", compile, dir).CombinedOutput(); err != nil {
		t.Fatalf("test TaskHost preparation failed: %v %s", err, output)
	}
	launcher := `import('./bootstrap.js').then(m=>m.bootstrap()).catch(error=>{console.error(error);process.exit(2)});`
	host := filepath.Join(dir, "host.cjs")
	if err := os.WriteFile(host, []byte(launcher), 0600); err != nil {
		t.Fatal(err)
	}
	entry := `module.exports=async(input,ctx)=>{
const initial=ctx.checkpoint.getCurrent()?.cursor||0;
await ctx.storage.set('private',{value:input.value});
const saved=await ctx.storage.get('private');
await ctx.checkpoint.save({value:input.value});
await ctx.progress.report({current:1,total:1,stage:'confirmed'});
const binary=await ctx.artifacts.saveFile('私有结果.bin',new Uint8Array([0,255,7,13,10,128]),{mimeType:'application/octet-stream'});
if(input.pause&&initial===0)await new Promise(resolve=>setTimeout(resolve,10000));
return {success:true,output:{private:saved.value,initial,binary:binary.artifactId,large:input.large?'中'.repeat(24000):''}};
};`
	bundleSource := t.TempDir()
	if err := os.WriteFile(filepath.Join(bundleSource, "task.cjs"), []byte(entry), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundleSource, "manifest.json"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	generationStore := kernel.NewPackageGenerationStore(t.TempDir())
	prepared, err := generationStore.PrepareGeneration(t.Context(), kernel.PackageGenerationPrepareRequest{ExtensionID: "extension", GenerationID: "generation-2", Version: "1.0.0", ArtifactID: "tls-task-artifact", OperationID: "tls-task-installation", SourcePath: bundleSource})
	if err != nil {
		t.Fatal(err)
	}
	committed, err := generationStore.CommitGeneration(t.Context(), prepared)
	if err != nil {
		t.Fatal(err)
	}
	if err := generationStore.SwitchCurrent("extension", "", committed.Current); err != nil {
		t.Fatal(err)
	}
	definition := &task_runtime.TaskDefinition{TaskID: "tls-executing-task", ExtensionID: "extension", ModuleID: "module", ContributionID: "tls-executing-task", Entry: "task.cjs", InstalledGeneration: 2, Checkpoint: true, ExecutionPlacement: task_runtime.TaskExecutionPlacementDevice}
	if err := task_runtime.PinTaskEntry(t.Context(), committed.GenerationPath, definition); err != nil {
		t.Fatal(err)
	}
	config := task_runtime.DefaultTaskRuntimeConfig()
	config.NodeEnvironmentResolver, config.HostArtifactResolver = meshTaskNodeResolver{binary}, meshTaskHostResolver{host}
	config.WorkspaceRoot = t.TempDir()
	permissions := permission.NewPermissionDefinitionRegistry()
	permissions.Register(permission.PermissionDefinition{ID: "test.source.per-use", Category: permission.CategoryFilesystem, AllowedScopes: []permission.ScopeType{permission.ScopeExtension}, BackgroundAllowed: true, RequiresPerUse: true, RemoteExecution: permission.RemoteExecutionRequireApproval})
	broker := permission.NewDefaultPermissionBroker(permissions, permission.NewMemoryPermissionStorage())
	t.Cleanup(func() { _ = broker.Close() })
	config.SourcePermissionGuard = task_runtime.NewSourceTaskPermissionGuard(broker)
	config.SourceApprovalRecorder = broker
	config.ProcessDiagnostics = os.Stderr
	config.InstalledDefinitionValidator = func(_ context.Context, current *task_runtime.TaskDefinition) error {
		if current == nil || current.TaskID != definition.TaskID || current.InstalledGeneration != definition.InstalledGeneration || current.EntryHash != definition.EntryHash {
			return coordination.ErrScopeExpired
		}
		return nil
	}
	config.EntryResolver = func(ctx context.Context, def *task_runtime.TaskDefinition) (string, error) {
		return task_runtime.ResolveTaskEntry(ctx, committed.GenerationPath, def)
	}
	config.InstalledExecutionLease = func(ctx context.Context, def *task_runtime.TaskDefinition) (string, func(), error) {
		return generationStore.AcquireCurrentExecution(ctx, def.ExtensionID, committed.Current.GenerationID, def.BundleHash)
	}
	service := task_runtime.NewTaskRuntimeService(kernelsqlite.NewTaskRepository(db), config)
	if err := service.PutTaskDefinition(t.Context(), definition); err != nil {
		t.Fatal(err)
	}
	return service, definition
}

func testOwnedTaskExecutionOverActualTLS(t *testing.T, rt *devicemesh.Runtime, db, sourceDB *sql.DB, services *AppServices, device runtimeidentity.DeviceID, sourceDefinition *task_runtime.TaskDefinition, sourceRuntime *task_runtime.TaskRuntimeService, identity *agent.IdentityStore, credential string, server *httptest.Server) {
	t.Helper()
	binding := &task_runtime.OwnedRuntimeBinding{}
	if err := bindCoreOwnedTaskRuntime(binding, rt, "core"); err != nil {
		t.Fatal(err)
	}
	config := task_runtime.DefaultTaskRuntimeConfig()
	binding.Apply(&config)
	snapshots := scope.NewSQLiteScopeStore(db)
	config.AuthoritySnapshots = snapshots
	repository := kernelsqlite.NewTaskRepository(db)
	callAPI := func(method, path string, input []byte, expected int) []byte {
		t.Helper()
		request, err := http.NewRequestWithContext(t.Context(), method, server.URL+"/api"+path, bytes.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "AmitiaDevice "+credential)
		request.Header.Set("Content-Type", "application/json")
		if err := identity.SignRequest(request, "core"); err != nil {
			t.Fatal(err)
		}
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
		if err != nil || response.StatusCode != expected {
			t.Fatalf("设备任务页面接口状态错误: method=%s path=%s status=%d expected=%d err=%v body=%s", method, path, response.StatusCode, expected, err, body)
		}
		return body
	}
	callExtension := func(method, path string, input []byte, expected int) []byte {
		return callAPI(method, "/extensions"+path, input, expected)
	}
	callTask := func(method, path string, input []byte, expected int) []byte {
		return callExtension(method, "/tasks"+path, input, expected)
	}
	readTask := func(path string, expected int) []byte {
		return callTask(http.MethodGet, path, nil, expected)
	}
	service := task_runtime.NewTaskRuntimeService(repository, config)
	services.KernelContainer.TaskRuntimeService = service
	definition := *sourceDefinition
	definition.InstalledGeneration, definition.DefinitionHash = 7, "core-executing-definition"
	if err := service.PutTaskDefinition(t.Context(), &definition); err != nil {
		t.Fatal(err)
	}
	pending := task_runtime.NewPendingTaskManager()
	rt.SetPendingTasks(pending)
	remote := task_runtime.NewMeshRemoteTaskExecutor(rt.Hub, nil, pending)
	service.SetRemoteExecutor(remote)
	errors := make(chan error, 16)
	report := func(err error) {
		if err != nil {
			select {
			case errors <- err:
			default:
			}
		}
	}
	rt.Handler.SetOnTaskClaimPayload(func(claim protocol.TaskClaimPayload) bool {
		if !pending.ValidateBound(claim.TaskRunID, claim.AttemptID, claim.LeaseID, claim.RuntimeSessionID.String(), claim.ConnectionGeneration) {
			return false
		}
		duration := time.Duration(claim.LeaseDurationMs) * time.Millisecond
		if err := service.HandleRemoteClaim(context.Background(), claim.TaskRunID, claim.AttemptID, claim.LeaseID, time.Now().UTC().Add(duration)); err != nil {
			current, _ := repository.GetTaskRun(context.Background(), claim.TaskRunID)
			report(fmt.Errorf("claim task=%s attempt=%s current=%+v: %w", claim.TaskRunID, claim.AttemptID, taskCallbackState(current), err))
			return false
		}
		return pending.ClaimBound(claim.TaskRunID, claim.AttemptID, claim.LeaseID, claim.RuntimeSessionID.String(), claim.ConnectionGeneration, claim.WorkerID, duration)
	})
	rt.Handler.SetOnTaskCompletePayload(func(result protocol.TaskCompletePayload) {
		if !pending.ValidateBound(result.TaskRunID, result.AttemptID, result.LeaseID, result.RuntimeSessionID.String(), result.ConnectionGeneration) {
			report(fmt.Errorf("完成回执的派发连接已失效"))
			return
		}
		if result.OutcomeUnknown {
			report(fmt.Errorf("实际设备任务返回结果未知"))
			return
		}
		if result.PausedCheckpointVersion > 0 {
			if err := service.HandleRemotePaused(context.Background(), result.TaskRunID, result.AttemptID, result.LeaseID, result.PausedCheckpointVersion); err != nil {
				report(fmt.Errorf("paused task=%s attempt=%s: %w", result.TaskRunID, result.AttemptID, err))
				return
			}
		} else {
			artifacts := []string{}
			if result.ResultArtifactID != "" {
				artifacts = append(artifacts, result.ResultArtifactID)
			}
			if err := service.HandleCompletionArtifact(context.Background(), result.TaskRunID, result.AttemptID, result.LeaseID, result.Success, result.Result, result.Error, artifacts...); err != nil {
				report(fmt.Errorf("completed task=%s attempt=%s: %w", result.TaskRunID, result.AttemptID, err))
				return
			}
		}
		if !pending.CompleteBound(result.TaskRunID, result.AttemptID, result.LeaseID, result.RuntimeSessionID.String(), result.ConnectionGeneration, result.Success, result.Error) {
			report(fmt.Errorf("完成回执未取得授权确认"))
		}
	})
	service.Start(t.Context())
	defer service.Shutdown(context.Background())
	legacy := &task_runtime.TaskRun{TaskRunID: "private-core-global-task", TaskDefinitionID: definition.TaskID, DefinitionFingerprint: "global-pin", Status: task_runtime.RunStatusSucceeded, Generation: 1, Revision: 1, CreatedAt: time.Now().UTC()}
	if err := repository.PutTaskRun(t.Context(), legacy); err != nil {
		t.Fatal(err)
	}
	readTask("/"+legacy.TaskRunID, http.StatusForbidden)
	readTask("/unknown-device-task", http.StatusNotFound)
	historicalRuns := make(map[string]string)
	previousBinary := ""
	for _, coordinated := range []bool{false, true} {
		policy, err := rt.Coordination.Get(t.Context(), "core", device.String())
		if err != nil {
			t.Fatal(err)
		}
		if policy.Coordinated != coordinated {
			if _, err := rt.Coordination.ChangeMode(t.Context(), "core", device.String(), policy.ModeRevision, coordinated, "one"); err != nil {
				t.Fatal(err)
			}
		}
		for _, scenario := range []string{"inline", "artifact", "pause_resume", "per_use_pause_resume"} {
			t.Logf("真实设备任务场景: coordinated=%t scenario=%s", coordinated, scenario)
			perUse := scenario == "per_use_pause_resume"
			pausing := scenario == "pause_resume" || perUse
			definition.PermissionRequirementStrings = nil
			sourceVersion := *sourceDefinition
			if perUse {
				definition.PermissionRequirementStrings = []string{"test.source.per-use"}
				sourceVersion.PermissionRequirementStrings = []string{"test.source.per-use"}
			}
			if err := sourceRuntime.PutTaskDefinition(t.Context(), &sourceVersion); err != nil {
				t.Fatal(err)
			}
			if err := service.PutTaskDefinition(t.Context(), &definition); err != nil {
				t.Fatal(err)
			}
			id := fmt.Sprintf("task-tls-%t-%s", coordinated, scenario)
			ctx, authority, finish, err := rt.Coordination.Begin(t.Context(), "core", "caller", device.String(), "core", "one", id)
			if err != nil {
				t.Fatal(err)
			}
			authority.RoleRevision, authority.ExecutionID, authority.TurnID = 3, id+"-execution", id+"-turn"
			ctx = coordination.WithScope(ctx, authority)
			if perUse {
				id, err = task_runtime.OwnedRequestTaskRunID(authority)
				if err != nil {
					t.Fatal(err)
				}
			}
			encoded, _ := json.Marshal(authority)
			if err := snapshots.SaveSnapshot(ctx, scope.ScopeSnapshot{SnapshotID: id, SpaceID: "core", InvocationID: id, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, CharacterID: "one", OwnedExecutionScope: encoded}); err != nil {
				t.Fatal(err)
			}
			connection, ok := rt.Hub.GetByDevice("core", device)
			if !ok {
				t.Fatal("设备执行连接未就绪")
			}
			input, _ := json.Marshal(map[string]any{"value": "owner-private-value", "large": scenario == "artifact", "pause": pausing})
			request := task_runtime.EnqueueTaskRequest{DeduplicateOwnedRequest: true, TaskDefinitionID: definition.TaskID, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, InvocationID: id, ScopeSnapshotID: id, Input: input, ExecutionPlacement: task_runtime.TaskExecutionPlacementDevice, TrustedExecutionTarget: &task_runtime.TrustedExecutionTargetRequest{Placement: task_runtime.TaskExecutionPlacementDevice, Target: task_runtime.TaskExecutionTarget{ProviderID: capability.ProviderID("tls-device-provider"), ProviderInstanceID: capability.ProviderInstanceID("tls-device-instance"), SpaceID: "core", DeviceID: device, RuntimeID: connection.RuntimeID, RuntimeSessionID: connection.SessionID, ConnectionGeneration: connection.Generation}}}
			approveGeneration := func(generation int64) {
				t.Helper()
				for _, approval := range sourceRuntime.ListSourceTaskApprovals() {
					if approval.Binding.Scope == authority && approval.Binding.TaskRunID == id && approval.Binding.TaskGeneration == generation && approval.Status == "pending" {
						if _, err := sourceRuntime.DecideSourceTaskApproval(ctx, approval.ID, approval.Revision, true); err != nil {
							t.Fatal(err)
						}
						return
					}
				}
				t.Fatalf("目标设备没有第 %d 代执行的独立本机审批", generation)
			}
			if perUse {
				pin, err := rt.TargetTaskDefinition(ctx, authority, definition.TaskID)
				if err != nil {
					t.Fatal(err)
				}
				digest := sha256.Sum256(input)
				preflight := task_runtime.SourceTaskPermissionRequest{Scope: authority, Target: pin, Input: input, Run: task_runtime.TaskRun{TaskRunID: id, TaskDefinitionID: definition.TaskID, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, InvocationID: id, ScopeSnapshotID: id, Generation: 1, InputHash: hex.EncodeToString(digest[:]), ExecutionTarget: request.TrustedExecutionTarget.Target}}
				if err := rt.TargetTaskPermissions(ctx, preflight); !task_runtime.IsTaskErrorCode(err, task_runtime.ErrTaskPermissionDenied) {
					t.Fatalf("未审批的实际 TLS 任务被允许执行: %v", err)
				}
				approveGeneration(1)
				if err := rt.TargetTaskPermissions(ctx, preflight); err != nil {
					t.Fatal(err)
				}
			}
			result, err := service.Enqueue(ctx, request, &definition)
			if err != nil {
				t.Fatal(err)
			}
			duplicate, err := service.Enqueue(ctx, request, &definition)
			if err != nil || duplicate.TaskRunID != result.TaskRunID {
				t.Fatalf("实际设备重复请求创建了其他任务: %+v %v", duplicate, err)
			}
			changed := request
			changed.Input = json.RawMessage(`{"value":"changed"}`)
			if _, err := service.Enqueue(ctx, changed, &definition); !stderrors.Is(err, coordination.ErrRequestConflict) {
				t.Fatalf("实际设备同一请求接受了不同输入: %v", err)
			}
			wait := func(predicate func(*task_runtime.TaskRun) bool) *task_runtime.TaskRun {
				t.Helper()
				deadline := time.NewTimer(45 * time.Second)
				defer deadline.Stop()
				ticker := time.NewTicker(20 * time.Millisecond)
				defer ticker.Stop()
				for {
					run, err := repository.GetTaskRun(t.Context(), result.TaskRunID)
					if err != nil {
						t.Fatal(err)
					}
					if predicate(run) {
						return run
					}
					if run.Status == task_runtime.RunStatusRecoveryRequired || run.Status == task_runtime.RunStatusFailed {
						message := ""
						if run.ErrorMessage != nil {
							message = *run.ErrorMessage
						}
						t.Fatalf("实际任务未确认: %s %+v", message, run)
					}
					select {
					case err := <-errors:
						t.Fatal(err)
					case <-deadline.C:
						t.Fatalf("实际任务联测超时: %+v", run)
					case <-ticker.C:
					}
				}
			}
			if pausing {
				run := wait(func(run *task_runtime.TaskRun) bool {
					return run.CheckpointID != nil && run.Status == task_runtime.RunStatusRunning
				})
				var taskView struct {
					Scope coordination.ExecutionScope `json:"executionScope"`
				}
				if json.Unmarshal(readTask("/"+run.TaskRunID, http.StatusOK), &taskView) != nil || taskView.Scope.CoreID != authority.CoreID || taskView.Scope.ResourceOwnerID != authority.ResourceOwnerID {
					t.Fatal("任务页面没有原始执行范围")
				}
				missingScope, _ := json.Marshal(map[string]any{"generation": run.Generation})
				callTask(http.MethodPost, "/"+run.TaskRunID+"/pause", missingScope, http.StatusConflict)
				foreignScope := taskView.Scope
				foreignScope.SpaceID += "-foreign"
				foreignInput, _ := json.Marshal(map[string]any{"generation": run.Generation, "expectedExecutionScope": foreignScope})
				callTask(http.MethodPost, "/"+run.TaskRunID+"/pause", foreignInput, http.StatusConflict)
				unchanged, err := repository.GetTaskRun(t.Context(), run.TaskRunID)
				if err != nil || unchanged.Status != task_runtime.RunStatusRunning || unchanged.Generation != run.Generation || unchanged.Revision != run.Revision {
					t.Fatal("旧范围或其他空间改变了实际任务")
				}
				pauseInput, _ := json.Marshal(map[string]any{"generation": run.Generation, "expectedExecutionScope": taskView.Scope})
				callTask(http.MethodPost, "/"+run.TaskRunID+"/pause", pauseInput, http.StatusOK)
				paused := wait(func(run *task_runtime.TaskRun) bool { return run.Status == task_runtime.RunStatusPaused })
				resumeInput, _ := json.Marshal(map[string]any{"generation": paused.Generation, "expectedExecutionScope": taskView.Scope})
				if perUse {
					callTask(http.MethodPost, "/"+paused.TaskRunID+"/resume", resumeInput, http.StatusForbidden)
					unchanged, err := repository.GetTaskRun(t.Context(), paused.TaskRunID)
					if err != nil || unchanged.Generation != paused.Generation || unchanged.Revision != paused.Revision || unchanged.Status != task_runtime.RunStatusPaused {
						t.Fatalf("未获得新审批的恢复改变了旧检查点任务: %+v %v", unchanged, err)
					}
					approveGeneration(paused.Generation + 1)
				}
				var resumed struct {
					Status string `json:"status"`
				}
				if json.Unmarshal(callTask(http.MethodPost, "/"+paused.TaskRunID+"/resume", resumeInput, http.StatusOK), &resumed) != nil || resumed.Status != "queued" {
					t.Fatal("任务恢复在设备确认之前被报告为已经运行")
				}
			}
			run := wait(func(run *task_runtime.TaskRun) bool { return run.Status == task_runtime.RunStatusSucceeded })
			if perUse {
				for _, approval := range sourceRuntime.ListSourceTaskApprovals() {
					if approval.Binding.Scope == authority && approval.Binding.TaskRunID == run.TaskRunID && approval.Status == "claimed" {
						if _, err := sourceRuntime.RevokeSourceTaskApproval(ctx, approval.ID, approval.Revision); err != nil {
							t.Fatal(err)
						}
					}
				}
				confirmed, err := service.ExistingOwnedDeviceTaskRequest(ctx, definition.TaskID, input, nil)
				if err != nil || confirmed == nil || confirmed.Status != task_runtime.RunStatusSucceeded || confirmed.Queued || confirmed.TaskRunID != run.TaskRunID {
					t.Fatalf("实际单次审批撤销后原任务无法确认或再次入队: %+v %v", confirmed, err)
				}
			}
			duplicate, err = service.Enqueue(ctx, request, &definition)
			if err != nil || duplicate.TaskRunID != run.TaskRunID || duplicate.Status != task_runtime.RunStatusSucceeded || duplicate.Queued {
				t.Fatalf("实际设备完成后的请求重试重复执行: %+v %v", duplicate, err)
			}
			output, err := service.GetResult(ctx, run.TaskRunID)
			if err != nil || output == nil {
				t.Fatalf("所有者结果读取失败: %v", err)
			}
			var body struct {
				Private string `json:"private"`
				Initial int64  `json:"initial"`
				Large   string `json:"large"`
				Binary  string `json:"binary"`
			}
			if json.Unmarshal(output.ResultJSON, &body) != nil || body.Private != "owner-private-value" || pausing && (body.Initial != 2 || run.Generation != 2) || scenario == "artifact" && len(body.Large) != 72000 {
				t.Fatalf("实际设备任务结果改变: %s", output.ResultJSON)
			}
			if downloaded := readTask("/"+run.TaskRunID+"/artifacts/"+body.Binary, http.StatusOK); !bytes.Equal(downloaded, []byte{0, 255, 7, 13, 10, 128}) {
				t.Fatal("真实 Node 任务的二进制产物下载改变了内容")
			}
			if previousBinary != "" {
				readTask("/"+run.TaskRunID+"/artifacts/"+previousBinary, http.StatusForbidden)
			}
			previousBinary = body.Binary
			var publicResult task_runtime.TaskRunResult
			if json.Unmarshal(readTask("/"+run.TaskRunID+"/result", http.StatusOK), &publicResult) != nil || publicResult.ResultHash != output.ResultHash || len(publicResult.ResultJSON) != len(output.ResultJSON) {
				t.Fatal("设备任务页面未直接读取所有者的同一份结果")
			}
			if scenario == "artifact" {
				if downloaded := readTask("/"+run.TaskRunID+"/result/artifact", http.StatusOK); !bytes.Equal(downloaded, output.ResultJSON) {
					t.Fatal("任务结果下载与所有者的正文不一致")
				}
				if downloaded := readTask("/"+run.TaskRunID+"/artifacts/"+output.ArtifactID, http.StatusOK); !bytes.Equal(downloaded, output.ResultJSON) {
					t.Fatal("任务产物下载未使用原所有者的同一份正文")
				}
				readTask("/"+run.TaskRunID+"/result/artifact?artifactId=other", http.StatusForbidden)
			}
			var page struct {
				Items []task_runtime.TaskRun `json:"items"`
			}
			pageBody := readTask("?limit=1", http.StatusOK)
			if json.Unmarshal(pageBody, &page) != nil || len(page.Items) != 1 || page.Items[0].TaskRunID != run.TaskRunID {
				t.Fatalf("设备任务页面未按持久设备归属分页: expected=%s body=%s", run.TaskRunID, pageBody)
			}
			metadata, err := repository.GetResult(t.Context(), run.TaskRunID)
			if err != nil || metadata == nil || len(metadata.ResultJSON) != 0 {
				t.Fatal("Core 调度表复制了私有结果正文")
			}
			for _, database := range []struct {
				db       *sql.DB
				expected int
			}{{sourceDB, 1}, {db, 0}} {
				expected := database.expected
				if coordinated {
					expected = 1 - expected
				}
				var count int
				if err := database.db.QueryRow(`SELECT count(*) FROM kernel_device_owned_resources WHERE resource_id=?`, "task/storage/"+run.TaskRunID).Scan(&count); err != nil || count != expected {
					t.Fatalf("真实任务产生数据双份: count=%d expected=%d err=%v", count, expected, err)
				}
			}
			hash := sha256.Sum256(output.ResultJSON)
			if metadata.ResultHash != hex.EncodeToString(hash[:]) {
				t.Fatal("结果正文和调度摘要不一致")
			}
			historicalRuns[run.TaskRunID] = metadata.ResultHash
			finish()
		}
	}
	definition.PermissionRequirementStrings = nil
	if err := service.PutTaskDefinition(t.Context(), &definition); err != nil {
		t.Fatal(err)
	}
	if err := sourceRuntime.PutTaskDefinition(t.Context(), sourceDefinition); err != nil {
		t.Fatal(err)
	}
	policy, err := rt.Coordination.Get(t.Context(), "core", device.String())
	if err != nil || !policy.Coordinated {
		t.Fatalf("管理员授权测试要求设备已开启统筹模式: %v", err)
	}
	services.OwnedBusiness = business.NewEngine(rt.Coordination, rt, nil)
	services.KernelContainer.ExecutionKernel = &execution.ExecutionPipeline{ScopeStore: snapshots}
	providers := capability.NewProviderRegistry()
	services.KernelContainer.CapabilityProviders = providers
	connection, connected := rt.Hub.GetByDevice("core", device)
	if !connected {
		t.Fatal("普通设备任务提交测试缺少有效连接")
	}
	provider := capability.CapabilityProviderDefinition{ID: "ordinary-task-provider", CapabilityID: "ordinary-task", Kind: capability.ProviderKindExtension, Placement: capability.ProviderPlacementDevice, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, Runtime: capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeTask, RuntimeID: definition.TaskID}}
	if err := providers.RegisterDefinition(provider); err != nil {
		t.Fatal(err)
	}
	if err := providers.RegisterInstance(capability.CapabilityProviderInstance{ID: "ordinary-task-instance", ProviderID: provider.ID, CapabilityID: provider.CapabilityID, Placement: provider.Placement, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, SpaceID: "core", DeviceID: device, RuntimeID: connection.RuntimeID, Health: capability.HealthReady, Availability: capability.ProviderAvailabilityAvailable}); err != nil {
		t.Fatal(err)
	}
	submission := meshTaskSubmission{TaskDefinitionID: definition.TaskID, TargetDeviceID: device.String(), RoleID: "one", RequestID: "ordinary-task-request", Input: json.RawMessage(`{"value":"ordinary-private"}`), ExpectedCoreID: "core", ExpectedModeRevision: policy.ModeRevision, ExpectedRoleRevision: 3}
	encodedSubmission, _ := json.Marshal(submission)
	callAPI(http.MethodPost, "/device-mesh/v1/business/tasks", encodedSubmission, http.StatusServiceUnavailable)
	rt.BusinessCoordinationReady = true
	defer func() { rt.BusinessCoordinationReady = false }()
	var roleOptions struct {
		Data struct {
			Roles []struct {
				ID       string          `json:"id"`
				Revision int64           `json:"revision"`
				Profile  json.RawMessage `json:"profile"`
			} `json:"roles"`
			Scope coordination.ExecutionScope `json:"executionScope"`
			Owner string                      `json:"roleOwnerId"`
		} `json:"data"`
	}
	if json.Unmarshal(callAPI(http.MethodGet, "/device-mesh/v1/business/tasks/roles?targetDeviceId="+device.String(), nil, http.StatusOK), &roleOptions) != nil || len(roleOptions.Data.Roles) != 2 || roleOptions.Data.Owner != "core" || roleOptions.Data.Scope.ModeRevision != policy.ModeRevision {
		t.Fatal("普通设备任务角色列表未使用当前归属或泄露了角色完整资料")
	}
	roleFound := false
	for _, option := range roleOptions.Data.Roles {
		if len(option.Profile) != 0 {
			t.Fatal("任务角色列表泄露完整资料")
		}
		if option.ID == "one" && option.Revision == 3 {
			roleFound = true
		}
	}
	if !roleFound {
		t.Fatal("任务角色列表缺少当前版本角色")
	}
	expectedTaskScope := roleOptions.Data.Scope
	expectedTaskScope.RoleID, expectedTaskScope.RoleRevision = submission.RoleID, submission.ExpectedRoleRevision
	submission.ExpectedScope = &expectedTaskScope
	encodedSubmission, _ = json.Marshal(submission)
	callAPI(http.MethodGet, "/device-mesh/v1/business/tasks/roles?targetDeviceId=caller", nil, http.StatusForbidden)
	var unknownFields map[string]any
	_ = json.Unmarshal(encodedSubmission, &unknownFields)
	unknownFields["administrator"] = true
	unknownBody, _ := json.Marshal(unknownFields)
	callAPI(http.MethodPost, "/device-mesh/v1/business/tasks", unknownBody, http.StatusBadRequest)
	var submitted struct {
		Data struct {
			Task  task_runtime.EnqueueTaskResult `json:"task"`
			Scope coordination.ExecutionScope    `json:"executionScope"`
		} `json:"data"`
	}
	if json.Unmarshal(callAPI(http.MethodPost, "/device-mesh/v1/business/tasks", encodedSubmission, http.StatusOK), &submitted) != nil || submitted.Data.Task.TaskRunID == "" || submitted.Data.Scope.InitiatorDeviceID != device.String() || submitted.Data.Scope.ResourceOwnerID != "core" || submitted.Data.Scope.RoleID != "one" {
		t.Fatal("普通设备任务提交未使用已认证调用者与当前统筹角色")
	}
	deadline := time.Now().Add(45 * time.Second)
	for {
		run, err := repository.GetTaskRun(t.Context(), submitted.Data.Task.TaskRunID)
		if err != nil {
			t.Fatal(err)
		}
		if run.Status == task_runtime.RunStatusSucceeded {
			break
		}
		if time.Now().After(deadline) || run.Status == task_runtime.RunStatusFailed || run.Status == task_runtime.RunStatusRecoveryRequired {
			t.Fatalf("普通设备提交的真实任务未完成: %+v", run)
		}
		select {
		case err := <-errors:
			t.Fatal(err)
		case <-time.After(20 * time.Millisecond):
		}
	}
	var repeated struct {
		Data struct {
			Task task_runtime.EnqueueTaskResult `json:"task"`
		} `json:"data"`
	}
	if json.Unmarshal(callAPI(http.MethodPost, "/device-mesh/v1/business/tasks", encodedSubmission, http.StatusOK), &repeated) != nil || repeated.Data.Task.TaskRunID != submitted.Data.Task.TaskRunID || repeated.Data.Task.Status != task_runtime.RunStatusSucceeded || repeated.Data.Task.Queued {
		t.Fatal("普通设备请求重试重新执行了已完成任务")
	}
	var catalog struct {
		Data devicemesh.DeviceTaskCatalogPage `json:"data"`
	}
	catalogRequest, _ := json.Marshal(meshTaskCatalogRequest{TargetDeviceID: device.String(), RoleID: "one", RequestID: "source-only-catalog", ExpectedScope: &expectedTaskScope, Limit: 8})
	if json.Unmarshal(callAPI(http.MethodPost, "/device-mesh/v1/business/tasks/catalog", catalogRequest, http.StatusOK), &catalog) != nil || len(catalog.Data.Entries) != 1 {
		t.Fatal("真实任务来源目录读取失败")
	}
	reference := catalog.Data.Entries[0].Reference
	aliasedSubmission := submission
	aliasedSubmission.TaskDefinitionID, aliasedSubmission.RequestID, aliasedSubmission.TaskCatalogReference = reference.CatalogID, "source-only-submit", &reference
	aliasedSubmission.Input = json.RawMessage(`{"value":"source-only-private"}`)
	aliasBody, _ := json.Marshal(aliasedSubmission)
	var aliased struct {
		Data struct {
			Task task_runtime.EnqueueTaskResult `json:"task"`
		} `json:"data"`
	}
	if json.Unmarshal(callAPI(http.MethodPost, "/device-mesh/v1/business/tasks", aliasBody, http.StatusOK), &aliased) != nil || aliased.Data.Task.TaskRunID == "" {
		t.Fatal("设备目录任务未通过普通签名身份提交")
	}
	aliasDeadline := time.Now().Add(45 * time.Second)
	for {
		run, err := repository.GetTaskRun(t.Context(), aliased.Data.Task.TaskRunID)
		if err != nil {
			t.Fatal(err)
		}
		if run.TaskDefinitionID != reference.CatalogID || run.ExecutionTarget.SourceTaskDefinitionID != reference.SourceTaskID || run.ExecutionTarget.DeviceID != device {
			t.Fatalf("目录任务丢失了原来源映射: %+v", run)
		}
		if run.Status == task_runtime.RunStatusSucceeded {
			break
		}
		if time.Now().After(aliasDeadline) || run.Status == task_runtime.RunStatusFailed || run.Status == task_runtime.RunStatusRecoveryRequired {
			t.Fatalf("目录任务的实际来源进程未完成: %+v", run)
		}
		select {
		case err := <-errors:
			t.Fatal(err)
		case <-time.After(20 * time.Millisecond):
		}
	}
	if json.Unmarshal(callAPI(http.MethodPost, "/device-mesh/v1/business/tasks", aliasBody, http.StatusOK), &repeated) != nil || repeated.Data.Task.TaskRunID != aliased.Data.Task.TaskRunID || repeated.Data.Task.Status != task_runtime.RunStatusSucceeded || repeated.Data.Task.Queued {
		t.Fatal("目录请求重试重新执行了已经确认的任务")
	}
	updatedSource := *sourceDefinition
	updatedSource.PermissionRequirementStrings = []string{"test.source.per-use"}
	if err := sourceRuntime.PutTaskDefinition(t.Context(), &updatedSource); err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal(callAPI(http.MethodPost, "/device-mesh/v1/business/tasks", aliasBody, http.StatusOK), &repeated) != nil || repeated.Data.Task.TaskRunID != aliased.Data.Task.TaskRunID || repeated.Data.Task.Status != task_runtime.RunStatusSucceeded || repeated.Data.Task.Queued {
		t.Fatal("来源设备更新插件后无法确认旧任务或重新执行旧任务")
	}
	if err := sourceRuntime.PutTaskDefinition(t.Context(), sourceDefinition); err != nil {
		t.Fatal(err)
	}
	missingReference := aliasedSubmission
	missingReference.TaskCatalogReference = nil
	missingBody, _ := json.Marshal(missingReference)
	callAPI(http.MethodPost, "/device-mesh/v1/business/tasks", missingBody, http.StatusConflict)
	for _, scenario := range []string{"input", "mode", "role-revision", "missing-role-revision", "foreign-target", "role", "permission", "epoch", "realm", "owner", "missing-scope"} {
		changed := submission
		changedScope := *submission.ExpectedScope
		changed.ExpectedScope = &changedScope
		expected := http.StatusConflict
		switch scenario {
		case "input":
			changed.Input = json.RawMessage(`{"value":"changed"}`)
		case "mode":
			changed.ExpectedModeRevision++
		case "role-revision":
			changed.ExpectedRoleRevision++
		case "missing-role-revision":
			changed.ExpectedRoleRevision, expected = 0, http.StatusBadRequest
		case "foreign-target":
			changed.TargetDeviceID, expected = "caller", http.StatusForbidden
		case "role":
			changed.RoleID = "missing-role"
		case "permission":
			changedScope.TargetPermissionRevision++
		case "epoch":
			changedScope.ProviderEpoch++
		case "realm":
			changedScope.AuthorizationRealm = "foreign-core"
		case "owner":
			changedScope.ResourceOwnerID = "foreign-device"
		case "missing-scope":
			changed.ExpectedScope, expected = nil, http.StatusBadRequest
		}
		body, _ := json.Marshal(changed)
		callAPI(http.MethodPost, "/device-mesh/v1/business/tasks", body, expected)
	}
	var ordinaryResult task_runtime.TaskRunResult
	if json.Unmarshal(readTask("/"+submitted.Data.Task.TaskRunID+"/result", http.StatusOK), &ordinaryResult) != nil || len(ordinaryResult.ResultJSON) == 0 {
		t.Fatal("普通设备提交的任务结果不能通过同一所有者接口读取")
	}
	historicalRuns[submitted.Data.Task.TaskRunID] = ordinaryResult.ResultHash
	policy, err = rt.Coordination.GrantAdministrator(t.Context(), "core", device.String(), policy.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	readTask("/"+legacy.TaskRunID, http.StatusOK)
	readHistorical := func() {
		t.Helper()
		for id, expectedHash := range historicalRuns {
			var result task_runtime.TaskRunResult
			if json.Unmarshal(readTask("/"+id+"/result", http.StatusOK), &result) != nil || result.ResultHash != expectedHash || len(result.ResultJSON) == 0 {
				t.Fatal("模式或管理员权限变化后无法从原所有者读取历史任务结果")
			}
			if result.ResultType == task_runtime.ResultArtifact {
				if downloaded := readTask("/"+id+"/result/artifact", http.StatusOK); !bytes.Equal(downloaded, result.ResultJSON) {
					t.Fatal("模式或管理员权限变化后历史结果下载改变了所有者正文")
				}
			}
			var body struct {
				Binary string `json:"binary"`
			}
			if json.Unmarshal(result.ResultJSON, &body) != nil || body.Binary == "" || !bytes.Equal(readTask("/"+id+"/artifacts/"+body.Binary, http.StatusOK), []byte{0, 255, 7, 13, 10, 128}) {
				t.Fatal("模式或权限变化后无法按原归属下载历史二进制产物")
			}
			readTask("/"+id+"/checkpoint", http.StatusOK)
			readTask("/"+id+"/progress", http.StatusOK)
		}
	}
	readHistorical()
	adminDefinition := task_runtime.TaskDefinition{TaskID: "admin-core-definition", ExtensionID: "admin-extension", ModuleID: "admin-module", ContributionID: "admin-core-definition", Entry: "task.cjs"}
	definitionBody, err := json.Marshal(adminDefinition)
	if err != nil {
		t.Fatal(err)
	}
	callExtension(http.MethodPost, "/task-definitions", definitionBody, http.StatusCreated)
	if stored, err := repository.GetTaskDefinition(t.Context(), adminDefinition.TaskID); err != nil || stored == nil || stored.Entry != adminDefinition.Entry {
		t.Fatalf("管理员未直接修改 Core 的任务定义: %v", err)
	}
	if stored, err := kernelsqlite.NewTaskRepository(sourceDB).GetTaskDefinition(t.Context(), adminDefinition.TaskID); err == nil || stored != nil {
		t.Fatalf("管理员设备保存了 Core 定义的本地副本: %v", err)
	}
	if _, err := rt.Coordination.GrantAdministrator(t.Context(), "core", device.String(), policy.PermissionRevision, false); err != nil {
		t.Fatal(err)
	}
	readTask("/"+legacy.TaskRunID, http.StatusForbidden)
	callExtension(http.MethodPost, "/task-definitions", definitionBody, http.StatusForbidden)
	readHistorical()
	currentPolicy, err := rt.Coordination.Get(t.Context(), "core", device.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.Coordination.ChangeMode(t.Context(), "core", device.String(), currentPolicy.ModeRevision, false, "one"); err != nil {
		t.Fatal(err)
	}
	readHistorical()
}
