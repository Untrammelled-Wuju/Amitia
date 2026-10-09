package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

type meshSourceNativeExecutor struct{ source *threeCoreFixture }

func (e meshSourceNativeExecutor) Execute(ctx context.Context, run *task_runtime.TaskRun, definition *task_runtime.TaskDefinition, id, method string, call task_runtime.TaskHostNativeCall) (json.RawMessage, error) {
	return e.source.services.DeviceMesh.PublishOwnedSourceTaskEvent(ctx, run, definition, id, call)
}

type meshSourcePermissionEvaluator struct{}

func (meshSourcePermissionEvaluator) Evaluate(context.Context, permission.PermissionEvaluationRequest) permission.PermissionEvaluationResult {
	return permission.PermissionEvaluationResult{Decision: permission.DecisionAllow}
}

func TestSourceTaskEventUsesActualTLSNativeRPCAndSourceDurableOutbox(t *testing.T) {
	binary := os.Getenv("AMITIA_TEST_NODE")
	if binary == "" {
		t.Fatal("真实Source Native联测必须使用项目Node")
	}
	schemas := newThreeCoreSchemas(t)
	a, b := newThreeCoreFixture(t, "native-source", schemas), newThreeCoreFixture(t, "native-core", schemas)
	a.local.SetExecutionJournal(executionjournal.NewStore(a.services.KernelContainer.DeviceRegistry.Database()))
	pairThreeCoreFixtures(t, a, b)
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
	entry := `module.exports=async(input,ctx)=>{await ctx.host.emitEvent('extension.extension.updated',{private:'<>&中文',number:1.2e3});return {success:true,output:{confirmed:true,capabilities:ctx.host.capabilities}};};`
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
	if err := events.RegisterExtensionEvents(t.Context(), "extension", 2, []event.EventTypeDefinition{{EventTypeID: "extension.extension.updated", Version: 1, RiskLevel: event.RiskLevelLow, OrderingPolicy: event.OrderingNone, MaxPayloadBytes: 64 << 10, MaxMetadataBytes: 32 << 10, ProducerPolicy: event.EventProducerPolicy{AllowedProducers: []string{"extension"}, MaxPayloadBytes: 64 << 10, MaxMetadataBytes: 32 << 10, RateLimitPerSecond: 100}}}, nil); err != nil {
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
	if err := service.BindNativeHost(a.services.DeviceMesh.LocalDeviceDataPort, meshSourceNativeExecutor{b}, task_runtime.NewSourceTaskHostPermissionGuard(meshSourcePermissionEvaluator{}, nil), publish); err != nil {
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
	port := task_runtime.AcknowledgedTaskHostPort{Data: b.services.DeviceMesh, Executor: meshSourceNativeExecutor{b}}
	result, err := service.ExecuteOwnedSourceDispatch(ctx, dispatch, func(ctx context.Context, id, method string, params json.RawMessage) (json.RawMessage, error) {
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
	if sourceCount != 1 || coreCount != 0 {
		t.Fatalf("OFF事件没有留在Source同一份数据: source=%d core=%d", sourceCount, coreCount)
	}
}
