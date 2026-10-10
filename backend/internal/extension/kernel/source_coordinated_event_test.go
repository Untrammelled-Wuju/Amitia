package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

type sourceContractDefinitionStore struct {
	task_runtime.TaskStore
	definition *task_runtime.TaskDefinition
}

func (s sourceContractDefinitionStore) GetTaskDefinition(context.Context, string) (*task_runtime.TaskDefinition, error) {
	encoded, err := json.Marshal(s.definition)
	var copy task_runtime.TaskDefinition
	if err == nil {
		err = json.Unmarshal(encoded, &copy)
	}
	return &copy, err
}

type sourceContractOwnerPort struct {
	coordination.DataPort
	coordination.ResourcePort
	pin           task_runtime.TargetTaskDefinitionPin
	contract      task_runtime.TaskHostEventContract
	afterContract func()
}

func (p *sourceContractOwnerPort) TargetTaskDefinition(context.Context, coordination.ExecutionScope, string) (task_runtime.TargetTaskDefinitionPin, error) {
	return p.pin, nil
}

func (p *sourceContractOwnerPort) SourceTaskEventContract(context.Context, *task_runtime.TaskRun, *task_runtime.TaskDefinition, string, task_runtime.TaskHostNativeCall) (task_runtime.TaskHostEventContract, error) {
	if p.afterContract != nil {
		p.afterContract()
	}
	return p.contract, nil
}

func TestCoordinatedSourceEventUsesFixedContractAndRejectsForeignExecution(t *testing.T) {
	db := setupSameIDTestDB(t)
	service, err := event.NewService(event.DefaultServiceConfig().WithDB(db))
	if err != nil {
		t.Fatal(err)
	}
	bridge := event.NewRuntimeBridge(service)
	schema := event.EventTypeDefinition{EventTypeID: "extension.source.only", Version: 1, MaxPayloadBytes: 64 << 10, MaxMetadataBytes: 32 << 10, RiskLevel: event.RiskLevelLow, OrderingPolicy: event.OrderingNone, PayloadSchema: json.RawMessage(`{"type":"object","required":["private"],"properties":{"private":{"type":"string"}}}`), ProducerPolicy: event.EventProducerPolicy{AllowedProducers: []string{"extension"}, MaxPayloadBytes: 64 << 10, MaxMetadataBytes: 32 << 10}}
	schema.DefinitionHash = schema.Hash()
	scope := coordination.ExecutionScope{SpaceID: "core", AuthorizationRealm: "core", CoreID: "core", InitiatorDeviceID: "caller", TargetDeviceID: "source", ResourceOwnerID: "core", RoleOwnerID: "core", RoleID: "role", RoleRevision: 1, ProviderEpoch: 1, TargetProviderEpoch: 1, ModeRevision: 1, PermissionRevision: 1, TargetPermissionRevision: 1, RequestID: "request", ExecutionID: "execution", TurnID: "turn", Coordinated: true}
	ctx := coordination.WithScope(t.Context(), scope)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "task.cjs"), []byte("module.exports=async()=>({success:true})"), 0600); err != nil {
		t.Fatal(err)
	}
	source := &task_runtime.TaskDefinition{TaskID: "source-task", ExtensionID: "source", ModuleID: "module", Entry: "task.cjs", InstalledGeneration: 7, ExecutionPlacement: task_runtime.TaskExecutionPlacementDevice}
	if err := task_runtime.PinTaskEntry(ctx, root, source); err != nil {
		t.Fatal(err)
	}
	config := task_runtime.DefaultTaskRuntimeConfig()
	config.EntryResolver = func(ctx context.Context, definition *task_runtime.TaskDefinition) (string, error) {
		return task_runtime.ResolveTaskEntry(ctx, root, definition)
	}
	config.InstalledDefinitionValidator = func(context.Context, *task_runtime.TaskDefinition) error { return nil }
	sourceRuntime := task_runtime.NewTaskRuntimeService(sourceContractDefinitionStore{definition: source}, config)
	pin, err := sourceRuntime.DescribeInstalledTask(ctx, source.TaskID, scope.TargetDeviceID)
	if err != nil {
		t.Fatal(err)
	}
	definition, err := task_runtime.NewCoreDeviceTaskDefinition(scope.CoreID, scope.TargetDeviceID, task_runtime.DeviceTaskCatalogEntry{Definition: *source, Target: pin})
	if err != nil {
		t.Fatal(err)
	}
	run := &task_runtime.TaskRun{TaskRunID: "run", ScopeSnapshotID: "scope", Generation: 3, ExecutionAttemptID: "attempt"}
	port := &sourceContractOwnerPort{pin: pin, contract: task_runtime.TaskHostEventContract{Scope: scope, TaskRunID: run.TaskRunID, Generation: run.Generation, AttemptID: "attempt", RequestID: "native-request", Target: pin, Definition: schema}}
	binding := &task_runtime.OwnedRuntimeBinding{}
	if err := binding.Bind(func(ctx context.Context, _ coordination.ExecutionScope, _ *task_runtime.TaskRun) (context.Context, func(), error) {
		return ctx, func() {}, nil
	}, port); err != nil {
		t.Fatal(err)
	}
	host := &sourceTaskHostBridge{events: bridge, owner: binding}
	call := task_runtime.TaskHostNativeCall{TaskRunID: run.TaskRunID, Type: string(schema.EventTypeID), Payload: json.RawMessage(`{"private":"<>&中文"}`)}
	payloadHash := sha256.Sum256(call.Payload)
	port.contract.PayloadHash = hex.EncodeToString(payloadHash[:])
	if _, err := host.publish(ctx, run, definition, "native-request", call); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetEventType(ctx, schema.EventTypeID, 1); err == nil {
		t.Fatal("Source contract registered in Core namespace")
	}
	original := port.contract
	for _, mutate := range []func(*task_runtime.TaskHostEventContract){
		func(c *task_runtime.TaskHostEventContract) { c.Scope.ProviderEpoch++ },
		func(c *task_runtime.TaskHostEventContract) { c.Generation++ },
		func(c *task_runtime.TaskHostEventContract) { c.AttemptID = "foreign" },
		func(c *task_runtime.TaskHostEventContract) { c.TaskRunID = "foreign" },
		func(c *task_runtime.TaskHostEventContract) { c.RequestID = "foreign" },
		func(c *task_runtime.TaskHostEventContract) { c.Target.InstalledGeneration++ },
		func(c *task_runtime.TaskHostEventContract) { c.Definition.DefinitionHash = "forged" },
		func(c *task_runtime.TaskHostEventContract) { c.PayloadHash = "forged" },
	} {
		port.contract = original
		mutate(&port.contract)
		if _, err := host.publish(ctx, run, definition, "native-request", call); err == nil {
			t.Fatal("foreign Source execution contract published")
		}
	}
	port.contract = original
	port.afterContract = func() { port.pin.InstalledGeneration++ }
	if _, err := host.publish(ctx, run, definition, "native-request", call); err == nil {
		t.Fatal("Source installation changed after contract but Core still published")
	}
	port.afterContract = nil
	port.pin = original.Target
	call.Payload = json.RawMessage(`{"private":4}`)
	if _, err := host.publish(ctx, run, definition, "native-request", call); err == nil {
		t.Fatal("Source schema type mismatch published")
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM extension_event_outbox WHERE aggregate_id=?", run.TaskRunID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("rejected events changed canonical Core data: %d %v", count, err)
	}
}
