package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type meshSourceNativeExecutor struct {
	source *threeCoreFixture
	events *event.RuntimeBridge
}

func (e meshSourceNativeExecutor) Execute(ctx context.Context, run *task_runtime.TaskRun, definition *task_runtime.TaskDefinition, id, method string, call task_runtime.TaskHostNativeCall) (json.RawMessage, error) {
	if e.events != nil {
		contract, err := e.source.services.DeviceMesh.SourceTaskEventContract(ctx, run, definition, id, call)
		if err != nil {
			return nil, err
		}
		payloadHash := sha256.Sum256(call.Payload)
		if contract.PayloadHash != hex.EncodeToString(payloadHash[:]) {
			return nil, fmt.Errorf("原设备契约改变实际 Native 事件字节")
		}
		provenance := event.SourceEventProvenance{ScopeSnapshotID: run.ScopeSnapshotID, SourceDeviceID: contract.Target.DeviceID, TaskRunID: run.TaskRunID, TaskGeneration: run.Generation, AttemptID: run.ExecutionAttemptID.String(), RequestID: id, InstalledGeneration: contract.Target.InstalledGeneration, DefinitionFingerprint: contract.Target.DefinitionFingerprint, EntryHash: contract.Target.EntryHash, BundleHash: definition.BundleHash, SchemaHash: contract.Definition.DefinitionHash, PayloadHash: contract.PayloadHash}
		result, err := e.events.PublishSourceContract(ctx, contract.Definition, definition.ExtensionID, call.Payload, event.PublishOptions{ProducerGeneration: contract.Target.InstalledGeneration, ProducerModuleID: definition.ModuleID, AggregateType: "source-task", AggregateID: run.TaskRunID, ScopeSnapshotID: run.ScopeSnapshotID, TraceID: id}, provenance)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"confirmed": result.Accepted, "eventId": result.EventID, "outboxId": result.OutboxID})
	}
	return e.source.services.DeviceMesh.PublishOwnedSourceTaskEvent(ctx, run, definition, id, call)
}

type meshSourcePermissionEvaluator struct{}

func (meshSourcePermissionEvaluator) Evaluate(context.Context, permission.PermissionEvaluationRequest) permission.PermissionEvaluationResult {
	return permission.PermissionEvaluationResult{Decision: permission.DecisionAllow}
}

func TestSourceTaskEventUsesActualTLSNativeRPCAndSourceDurableOutbox(t *testing.T) {
	t.Run("device-owned", func(t *testing.T) { runSourceTaskEventTLS(t, false) })
	t.Run("core-owned", func(t *testing.T) { runSourceTaskEventTLS(t, true) })
}

func runSourceTaskEventTLS(t *testing.T, coordinated bool) {
	binary := os.Getenv("AMITIA_TEST_NODE")
	if binary == "" {
		t.Fatal("真实Source Native联测必须使用项目Node")
	}
	schemas := newThreeCoreSchemas(t)
	a, b := newThreeCoreFixture(t, "native-source", schemas), newThreeCoreFixture(t, "native-core", schemas)
	a.local.SetExecutionJournal(executionjournal.NewStore(a.services.KernelContainer.DeviceRegistry.Database()))
	pairThreeCoreFixtures(t, a, b)
	if coordinated {
		policy, err := b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
		if err != nil {
			t.Fatal(err)
		}
		b.request(t, a, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "expectedRevision": policy.ModeRevision, "selectedRole": "one"}, http.StatusOK)
	}
	dir := t.TempDir()
	root, err := filepath.Abs("../../../runtime/task-host")
	if err != nil {
		t.Fatal(err)
	}
	compiler, _ := json.Marshal(filepath.Join(root, "node_modules/typescript/lib/typescript.js"))
	sourceRoot, _ := json.Marshal(filepath.Join(root, "src"))
	compile := fmt.Sprintf(`const fs=require('node:fs'),path=require('node:path'),ts=require(%s),root=%s,out=process.argv[1];for(const file of fs.readdirSync(root)){if(!file.endsWith('.ts'))continue;fs.writeFileSync(path.join(out,file.slice(0,-3)+'.js'),ts.transpileModule(fs.readFileSync(path.join(root,file),'utf8'),{compilerOptions:{module:ts.ModuleKind.ES2022,target:ts.ScriptTarget.ES2022}}).outputText);}fs.writeFileSync(path.join(out,'package.json'),JSON.stringify({type:'module'}));`, compiler, sourceRoot)
	if output, err := exec.Command(binary, "-e", compile, dir).CombinedOutput(); err != nil {
		t.Fatalf("TaskHost准备失败: %v %s", err, output)
	}
	host := filepath.Join(dir, "host.cjs")
	if err := os.WriteFile(host, []byte(`import('./bootstrap.js').then(m=>m.bootstrap()).catch(error=>{console.error(error);process.exit(2)});`), 0600); err != nil {
		t.Fatal(err)
	}
	entry := `module.exports=async(input,ctx)=>{await ctx.host.emitEvent('extension.extension.updated',{private:'<>&中文',number:1.2e3,padding:'x'.repeat(60000)});return {success:true,output:{confirmed:true,capabilities:ctx.host.capabilities}};};`
	bundle := t.TempDir()
	if err := os.WriteFile(filepath.Join(bundle, "task.cjs"), []byte(entry), 0600); err != nil {
		t.Fatal(err)
	}
	definition := &task_runtime.TaskDefinition{TaskID: "native-event-task", ExtensionID: "extension", ModuleID: "module", Entry: "task.cjs", InstalledGeneration: 2, PermissionRequirementStrings: []string{"event.emit"}}
	if err := task_runtime.PinTaskEntry(t.Context(), bundle, definition); err != nil {
		t.Fatal(err)
	}
	config := task_runtime.DefaultTaskRuntimeConfig()
	config.NodeEnvironmentResolver = meshTaskNodeResolver{binary}
	config.HostArtifactResolver = meshTaskHostResolver{host}
	config.WorkspaceRoot = t.TempDir()
	config.EntryResolver = func(ctx context.Context, def *task_runtime.TaskDefinition) (string, error) {
		return task_runtime.ResolveTaskEntry(ctx, bundle, def)
	}
	config.InstalledDefinitionValidator = func(context.Context, *task_runtime.TaskDefinition) error { return nil }
	config.InstalledExecutionLease = func(context.Context, *task_runtime.TaskDefinition) (string, func(), error) {
		return bundle, func() {}, nil
	}
	config.SourcePermissionGuard = task_runtime.NewSourceTaskPermissionGuard(meshSourcePermissionEvaluator{})
	repository := kernelsqlite.NewTaskRepository(a.services.KernelContainer.DeviceRegistry.Database())
	service := task_runtime.NewTaskRuntimeService(repository, config)
	a.services.KernelContainer.TaskRuntimeService = service
	if err := service.PutTaskDefinition(t.Context(), definition); err != nil {
		t.Fatal(err)
	}
	eventService, err := event.NewService(event.DefaultServiceConfig().WithDB(a.services.KernelContainer.DeviceRegistry.Database()))
	if err != nil {
		t.Fatal(err)
	}
	events := event.NewRuntimeBridge(eventService)
	if err := events.RegisterExtensionEvents(t.Context(), "extension", 2, []event.EventTypeDefinition{{EventTypeID: "extension.extension.updated", Version: 1, RiskLevel: event.RiskLevelLow, OrderingPolicy: event.OrderingNone, MaxPayloadBytes: 64 << 10, MaxMetadataBytes: 0, ProducerPolicy: event.EventProducerPolicy{AllowedProducers: []string{"extension"}, MaxPayloadBytes: 64 << 10, MaxMetadataBytes: 0, RateLimitPerSecond: 100}}}, nil); err != nil {
		t.Fatal(err)
	}
	publish := func(ctx context.Context, run *task_runtime.TaskRun, def *task_runtime.TaskDefinition, id string, call task_runtime.TaskHostNativeCall) (json.RawMessage, error) {
		var result event.PublishResult
		err := coordination.CommitCurrent(ctx, func() error {
			var err error
			result, err = events.PublishFromRuntime(ctx, def.ExtensionID, event.EventTypeID(call.Type), 1, call.Payload, event.PublishOptions{ProducerGeneration: def.InstalledGeneration, ProducerModuleID: def.ModuleID, AggregateType: "source-task", AggregateID: run.TaskRunID, ScopeSnapshotID: run.ScopeSnapshotID, TraceID: id})
			return err
		})
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"confirmed": result.Accepted, "eventId": result.EventID, "outboxId": result.OutboxID})
	}
	executor := meshSourceNativeExecutor{source: b}
	if coordinated {
		coreEvents, err := event.NewService(event.DefaultServiceConfig().WithDB(b.services.KernelContainer.DeviceRegistry.Database()))
		if err != nil {
			t.Fatal(err)
		}
		executor.events = event.NewRuntimeBridge(coreEvents)
	}
	if err := service.BindNativeHost(a.services.DeviceMesh.LocalDeviceDataPort, executor, task_runtime.NewSourceTaskHostPermissionGuard(meshSourcePermissionEvaluator{}, nil), publish, func(ctx context.Context, def *task_runtime.TaskDefinition) ([]event.EventTypeDefinition, error) {
		return events.InstalledEventContracts(ctx, def.ExtensionID, def.InstalledGeneration)
	}); err != nil {
		t.Fatal(err)
	}
	registerSourceTaskHostEventDispatcher(a.dispatcher, a.services)
	current, scope, finish, err := b.services.DeviceMesh.Coordination.Begin(t.Context(), b.core, a.device.DeviceID.String(), a.device.DeviceID.String(), b.core, "one", "native-event-request")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	scope.RoleRevision, scope.ExecutionID, scope.TurnID = 3, "native-event-execution", "native-event-turn"
	current = coordination.WithScope(current, scope)
	connection, ok := b.services.DeviceMesh.Hub.GetByDevice(runtimeidentity.SpaceID(b.core), a.device.DeviceID)
	if !ok {
		t.Fatal("Source未建立真实WS连接")
	}
	input := json.RawMessage(`{}`)
	hash := sha256.Sum256(input)
	run := &task_runtime.TaskRun{TaskRunID: "native-event-run", TaskDefinitionID: definition.TaskID, ExtensionID: "extension", ModuleID: "module", InvocationID: "native-event-run", ScopeSnapshotID: "native-event-run", Generation: 1, ExecutionAttemptID: "attempt", Attempt: 1, MaxAttempts: 1, InputHash: hex.EncodeToString(hash[:]), DefinitionFingerprint: definition.DefinitionHash, ExecutionPlacement: task_runtime.TaskExecutionPlacementDevice, ExecutionTarget: task_runtime.TaskExecutionTarget{SpaceID: connection.SpaceID, DeviceID: connection.DeviceID, RuntimeID: connection.RuntimeID, RuntimeSessionID: connection.SessionID, ConnectionGeneration: connection.Generation}}
	if len(run.DefinitionFingerprint) != 64 {
		run.DefinitionFingerprint = hex.EncodeToString(hash[:])
	}
	ctx, cancel := context.WithTimeout(current, 20*time.Second)
	defer cancel()
	pin, err := service.DescribeInstalledTask(ctx, definition.TaskID, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ownerInput := *run
	ownerInput.Input = input
	if err := (task_runtime.AcknowledgedTaskInputPort{Data: b.services.DeviceMesh}).SaveInput(ctx, &ownerInput); err != nil {
		t.Fatal(err)
	}
	encodedRun, _ := json.Marshal(run)
	encodedScope, _ := json.Marshal(scope)
	encodedPin, _ := json.Marshal(pin)
	dispatch := protocol.TaskDispatchPayload{TaskRunID: run.TaskRunID, TaskDefinitionID: definition.TaskID, TaskGeneration: 1, AttemptID: "attempt", LeaseID: "lease", AuthorityCallID: "authority", DeviceID: connection.DeviceID, RuntimeID: connection.RuntimeID, RuntimeSessionID: connection.SessionID, ConnectionGeneration: connection.Generation, RootTaskMetadata: encodedRun, OwnedExecutionScope: encodedScope, TargetDefinitionPin: encodedPin, Input: input}
	port := task_runtime.AcknowledgedTaskHostPort{Data: b.services.DeviceMesh, Executor: executor}
	result, err := service.ExecuteOwnedSourceDispatch(ctx, dispatch, func(ctx context.Context, id, method string, params json.RawMessage) (json.RawMessage, error) {
		var native task_runtime.TaskHostNativeCall
		if err := json.Unmarshal(params, &native); err != nil {
			return nil, err
		}
		for _, kind := range []string{"generation", "attempt", "call-run"} {
			foreign := task_runtime.CloneTaskRun(run)
			call := native
			switch kind {
			case "generation":
				foreign.Generation++
			case "attempt":
				foreign.ExecutionAttemptID = "foreign-attempt"
			case "call-run":
				call.TaskRunID = "foreign-run"
			}
			var rejected error
			if coordinated {
				_, rejected = b.services.DeviceMesh.SourceTaskEventContract(ctx, foreign, definition, id+"/"+kind, call)
			} else {
				_, rejected = b.services.DeviceMesh.PublishOwnedSourceTaskEvent(ctx, foreign, definition, id+"/"+kind, call)
			}
			if rejected == nil {
				t.Errorf("实际Source Native接受错误%s", kind)
			}
		}
		return port.Call(ctx, run, definition, id, method, params)
	})
	if err != nil {
		t.Fatalf("真实TLS Source event失败: %+v %v", result, err)
	}
	var sourceCount, coreCount int
	if err := a.services.KernelContainer.DeviceRegistry.Database().QueryRowContext(t.Context(), "SELECT COUNT(*) FROM extension_event_outbox WHERE aggregate_id=?", run.TaskRunID).Scan(&sourceCount); err != nil {
		t.Fatal(err)
	}
	if err := b.services.KernelContainer.DeviceRegistry.Database().QueryRowContext(t.Context(), "SELECT COUNT(*) FROM extension_event_outbox WHERE aggregate_id=?", run.TaskRunID).Scan(&coreCount); err != nil {
		t.Fatal(err)
	}
	expectedSource, expectedCore := 1, 0
	if coordinated {
		expectedSource, expectedCore = 0, 1
		if _, err := executor.events.InstalledEventContracts(t.Context(), "extension", 2); err == nil {
			t.Fatal("来源插件事件注册污染 Core 全局目录")
		}
	}
	if sourceCount != expectedSource || coreCount != expectedCore {
		t.Fatalf("事件数据归属错误: source=%d core=%d coordinated=%v", sourceCount, coreCount, coordinated)
	}
	ownerDB := a.services.KernelContainer.DeviceRegistry.Database()
	if coordinated {
		ownerDB = b.services.KernelContainer.DeviceRegistry.Database()
	}
	var storedPayload []byte
	if err := ownerDB.QueryRowContext(t.Context(), "SELECT payload_json FROM extension_event_outbox WHERE aggregate_id=?", run.TaskRunID).Scan(&storedPayload); err != nil {
		t.Fatal(err)
	}
	var body struct {
		Private string `json:"private"`
		Padding string `json:"padding"`
	}
	if json.Unmarshal(storedPayload, &body) != nil || body.Private != "<>&中文" || len(body.Padding) != 60000 {
		t.Fatal("较大事件正文经过真实 Source Native 路由后发生变化")
	}
	var hostCount int
	if err := ownerDB.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM extension_event_host_provenance p JOIN extension_event_outbox o ON o.outbox_id=p.outbox_id WHERE o.aggregate_id=?", run.TaskRunID).Scan(&hostCount); err != nil {
		t.Fatal(err)
	}
	if coordinated {
		var provenanceBytes []byte
		if hostCount != 1 {
			t.Fatal("Core 事件缺少独立宿主来源记录")
		}
		if err := ownerDB.QueryRowContext(t.Context(), "SELECT p.provenance_json FROM extension_event_host_provenance p JOIN extension_event_outbox o ON o.outbox_id=p.outbox_id WHERE o.aggregate_id=?", run.TaskRunID).Scan(&provenanceBytes); err != nil {
			t.Fatal(err)
		}
		var provenance event.SourceEventProvenance
		payloadHash := sha256.Sum256(storedPayload)
		if json.Unmarshal(provenanceBytes, &provenance) != nil || provenance.SourceDeviceID != a.device.DeviceID.String() || provenance.TaskRunID != run.TaskRunID || provenance.PayloadHash != hex.EncodeToString(payloadHash[:]) {
			t.Fatal("独立宿主来源记录未绑定实际 Source 及持久化正文")
		}
	} else if hostCount != 0 {
		t.Fatal("设备自管事件被标记为 Core 统筹事件")
	}
	if _, err := b.services.DeviceMesh.PublishOwnedSourceTaskEvent(ctx, run, definition, "retired-source", task_runtime.TaskHostNativeCall{TaskRunID: run.TaskRunID, Type: "extension.extension.updated", Payload: json.RawMessage(`{"private":"late"}`)}); err == nil {
		t.Fatal("已结束Source任务仍发布Native事件")
	}
	if coordinated {
		if _, err := b.services.DeviceMesh.SourceTaskEventContract(ctx, run, definition, "retired-contract", task_runtime.TaskHostNativeCall{TaskRunID: run.TaskRunID, Type: "extension.extension.updated", Payload: json.RawMessage(`{"private":"late"}`)}); err == nil {
			t.Fatal("已结束来源任务仍返回可用事件契约")
		}
	}
	if err := a.services.KernelContainer.DeviceRegistry.Database().QueryRowContext(t.Context(), "SELECT COUNT(*) FROM extension_event_outbox WHERE aggregate_id=?", run.TaskRunID).Scan(&sourceCount); err != nil || sourceCount != expectedSource {
		t.Fatalf("拒绝的Native事件产生迟到写入: %d %v", sourceCount, err)
	}
}
