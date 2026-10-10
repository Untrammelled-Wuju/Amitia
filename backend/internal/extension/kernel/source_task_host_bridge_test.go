package kernel

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestSourceTaskNativeEventPersistsRealOwnerOutboxAndRejectsCancelledScope(t *testing.T) {
	db := setupSameIDTestDB(t)
	service, err := event.NewService(event.DefaultServiceConfig().WithDB(db))
	if err != nil {
		t.Fatal(err)
	}
	bridge := event.NewRuntimeBridge(service)
	definition := event.EventTypeDefinition{EventTypeID: "extension.fixture.updated", Version: 1, PayloadSchema: json.RawMessage(`{"type":"object","required":["private"],"additionalProperties":false,"properties":{"private":{"type":"string"}}}`), MaxPayloadBytes: 64 << 10, MaxMetadataBytes: 32 << 10, RiskLevel: event.RiskLevelLow, OrderingPolicy: event.OrderingNone, ProducerPolicy: event.EventProducerPolicy{AllowedProducers: []string{"extension"}, MaxPayloadBytes: 64 << 10, MaxMetadataBytes: 32 << 10, RateLimitPerSecond: 100}}
	if err := bridge.RegisterExtensionEvents(t.Context(), "fixture", 2, []event.EventTypeDefinition{definition}, nil); err != nil {
		t.Fatal(err)
	}
	scope := coordination.ExecutionScope{SpaceID: "core", AuthorizationRealm: "core", CoreID: "core", InitiatorDeviceID: "caller", TargetDeviceID: "source", ResourceOwnerID: "source", RoleOwnerID: "source", RoleID: "role", RoleRevision: 1, ProviderEpoch: 1, TargetProviderEpoch: 1, ModeRevision: 1, PermissionRevision: 1, TargetPermissionRevision: 1, RequestID: "request", ExecutionID: "execution", TurnID: "turn"}
	ctx := coordination.WithScope(t.Context(), scope)
	run := &task_runtime.TaskRun{TaskRunID: "run", ScopeSnapshotID: "scope"}
	task := &task_runtime.TaskDefinition{ExtensionID: "fixture", ModuleID: "module", InstalledGeneration: 2}
	call := task_runtime.TaskHostNativeCall{TaskRunID: "run", Type: "extension.fixture.updated", Payload: json.RawMessage(`{"private":"<>&中文"}`)}
	host := &sourceTaskHostBridge{events: bridge}
	result, err := host.publish(ctx, run, task, "native-request", call)
	if err != nil {
		t.Fatal(err)
	}
	var ack struct {
		Confirmed bool
		EventID   string
		OutboxID  string
	}
	if json.Unmarshal(result, &ack) != nil || !ack.Confirmed || ack.EventID == "" || ack.OutboxID == "" {
		t.Fatalf("event did not return real durable ACK: %s", result)
	}
	var payload, producer, module, metadata string
	var generation int64
	if err := db.QueryRowContext(ctx, "SELECT payload_json,producer_id,producer_generation,COALESCE(metadata_json,'') FROM extension_event_outbox WHERE outbox_id=? AND event_id=?", ack.OutboxID, ack.EventID).Scan(&payload, &producer, &generation, &metadata); err != nil {
		t.Fatal(err)
	}
	var decoded struct{ Private string }
	if json.Unmarshal([]byte(payload), &decoded) != nil || decoded.Private != "<>&中文" || producer != "fixture" || generation != 2 {
		t.Fatalf("durable Source event identity/private payload changed: %s %s %d %s", payload, producer, generation, module)
	}
	if strings.Contains(metadata, "amitiaSourceTaskContract") {
		t.Fatal("Source local authorized subscriptions were disabled by an ON-only cross-device marker")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := host.publish(cancelled, run, task, "cancelled", call); err == nil {
		t.Fatal("cancelled event scope committed durable event")
	}
	for _, invalid := range []json.RawMessage{json.RawMessage(`{"private":4}`), json.RawMessage(`{"private":"ok","undeclared":true}`)} {
		changed := call
		changed.Payload = invalid
		if _, err := host.publish(ctx, run, task, "invalid-schema", changed); err == nil {
			t.Fatal("OFF Native event bypassed complete schema validation")
		}
	}
	oldGeneration := *task
	oldGeneration.InstalledGeneration--
	if _, err := host.publish(ctx, run, &oldGeneration, "old-install", call); err == nil {
		t.Fatal("OFF Native event borrowed another installed generation")
	}
	foreign := call
	foreign.Type = "extension.fixture.undeclared"
	if _, err := host.publish(ctx, run, task, "undeclared-type", foreign); err == nil {
		t.Fatal("OFF Native event used undeclared installed event type")
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM extension_event_outbox").Scan(&count); err != nil || count != 1 {
		t.Fatalf("late scope wrote event: %d %v", count, err)
	}
}

type sourceNativeAdapterPort struct {
	t     *testing.T
	calls int
}

func (p *sourceNativeAdapterPort) Execute(_ context.Context, request capability.DeviceRuntimeInvocationRequest) capability.UnifiedToolResult {
	p.calls++
	if !request.Route.RemoteDevice || request.Route.DeviceID != "source" || request.Route.RuntimeID != "source-runtime" || request.Route.RuntimeSessionID != "source-session" || request.Route.Placement != capability.ProviderPlacementDevice || request.Invocation.ExecutionTarget.ProviderID != "native-provider" || request.Binding.HandlerName != "native.echo" {
		p.t.Errorf("Native adapter used foreign/Core route: %+v %+v", request.Route, request.Invocation.ExecutionTarget)
	}
	result := capability.NewToolSuccessResult(request.Invocation.InvocationID, "native.echo")
	result.DeviceID, result.RuntimeID, result.RuntimeSessionID, result.Structured = "source", "source-runtime", "source-session", append(json.RawMessage(nil), request.Input...)
	return result
}
func (*sourceNativeAdapterPort) Health(context.Context, capability.RuntimeExecutionRoute) capability.HealthStatus {
	return capability.HealthReady
}
func (*sourceNativeAdapterPort) Cancel(context.Context, capability.DeviceRuntimeInvocationRequest, capability.ToolCancellationReason) error {
	return nil
}

type sourceNativeSessionResolver struct{}

func (sourceNativeSessionResolver) ResolveActiveSession(context.Context, runtimeidentity.SpaceID, runtimeidentity.DeviceID, runtimeidentity.RuntimeID) (runtimeidentity.RuntimeSessionID, bool) {
	return "source-session", true
}

func TestSourceTaskNativeUsesActualDeviceAdapterWithUniqueSourceProvider(t *testing.T) {
	registry := capability.NewProviderRegistry()
	tool := capability.ToolDefinition{ID: "native.echo", CapabilityID: "native.echo", Runtime: capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeDesktop_Extension, HandlerName: "native.echo"}, Enabled: true, Compatible: true, TimeoutMS: 5000}
	provider := capability.CapabilityProviderDefinition{ID: "native-provider", CapabilityID: tool.CapabilityID, Kind: capability.ProviderKindBuiltin, Placement: capability.ProviderPlacementDevice, Runtime: tool.Runtime}
	if err := registry.RegisterDefinition(provider); err != nil {
		t.Fatal(err)
	}
	instance := capability.CapabilityProviderInstance{ID: "native-instance", ProviderID: provider.ID, CapabilityID: tool.CapabilityID, Placement: capability.ProviderPlacementDevice, SpaceID: "core", DeviceID: "source", RuntimeID: "source-runtime", Health: capability.HealthReady, Availability: capability.ProviderAvailabilityAvailable}
	if err := registry.RegisterInstance(instance); err != nil {
		t.Fatal(err)
	}
	host := &sourceTaskHostBridge{providers: registry}
	target := task_runtime.TaskExecutionTarget{SpaceID: "core", DeviceID: "source", RuntimeID: "source-runtime", RuntimeSessionID: "source-session", ConnectionGeneration: 2}
	actual, err := host.nativeTarget(tool, target)
	if err != nil {
		t.Fatal(err)
	}
	port := &sourceNativeAdapterPort{t: t}
	adapters := capability.NewRuntimeAdapterRegistry()
	if err := adapters.RegisterDeviceAdapter(capability.NewDeviceRuntimeAdapter(port)); err != nil {
		t.Fatal(err)
	}
	resolver := capability.NewProviderRuntimeExecutionResolverWithSessions(&capability.ProviderRegistryExecutionLookup{Registry: registry}, sourceNativeSessionResolver{})
	pipeline := &execution.ExecutionPipeline{ToolResolver: func(context.Context, string) (capability.ToolDefinition, error) { return tool, nil }, Dispatcher: execution.NewRuntimeDispatcher(adapters, resolver), TimeoutCtrl: execution.NewTimeoutController(5 * time.Second)}
	result := pipeline.Execute(t.Context(), execution.ToolExecutionRequest{ToolID: "native.echo", Input: json.RawMessage(`{"text":"<>&中文"}`), Invocation: capability.ToolInvocationContext{InvocationID: "native-invocation", SpaceID: "core", ExecutionTarget: actual, DeadlineDuration: 5 * time.Second}})
	if result.Status != capability.ToolResultStatusSuccess || result.DeviceID != "source" || port.calls != 1 {
		t.Fatalf("actual device adapter failed: %+v calls=%d", result, port.calls)
	}
	foreign := target
	foreign.DeviceID = "foreign"
	if _, err := host.nativeTarget(tool, foreign); err == nil {
		t.Fatal("missing Source provider fell back to Core")
	}
	duplicate := instance
	duplicate.ID = "duplicate"
	if err := registry.RegisterInstance(duplicate); err != nil {
		t.Fatal(err)
	}
	if _, err := host.nativeTarget(tool, target); err == nil {
		t.Fatal("ambiguous Source provider arbitrarily selected")
	}
	if port.calls != 1 {
		t.Fatal("rejected native provider invoked adapter")
	}
}
