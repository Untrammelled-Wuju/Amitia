package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/gamehost/agentbridge"
	ghdomain "github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/registry"
	ghruntime "github.com/u-ai/backend/internal/gamehost/runtime"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type pipelineGameControlPlane struct {
	lastPeer   ipc.Peer
	lastMethod string
}

func (p *pipelineGameControlPlane) Attach(context.Context, ipc.Peer, ipc.Transport) (*ipc.Connection, error) {
	return nil, fmt.Errorf("not used")
}
func (p *pipelineGameControlPlane) Detach(context.Context, ipc.ConnectionID) error { return nil }
func (p *pipelineGameControlPlane) Send(context.Context, ipc.Peer, protocol.Envelope) error {
	return nil
}
func (p *pipelineGameControlPlane) Shutdown(context.Context) error { return nil }
func (p *pipelineGameControlPlane) SendRequest(_ context.Context, peer ipc.Peer, envelope protocol.Envelope, _ time.Duration) (*protocol.Envelope, error) {
	p.lastPeer = peer
	p.lastMethod = envelope.Method
	response, err := protocol.NewResponse("response-"+envelope.ID, envelope.ID, map[string]any{
		"ok":      true,
		"service": string(peer.ServiceID),
		"method":  envelope.Method,
	})
	if err != nil {
		return nil, err
	}
	return &response, nil
}

func TestGameHostToolRunsThroughAgentExecutionPipeline(t *testing.T) {
	ctx := context.Background()

	plugins := registry.NewRegistry()
	plugin := ghdomain.PluginDescriptor{
		ID:              "pipeline-plugin",
		ExtensionID:     "com.example.pipeline-game",
		Name:            "Pipeline Game",
		Version:         "1.0.0",
		ProtocolVersion: protocol.ProtocolVersion,
		Services: []ghdomain.ServiceDescriptor{
			{ID: "primary", Name: "primary", Kind: ghdomain.ServiceKindProcess, Required: true},
			{ID: "secondary", Name: "secondary", Kind: ghdomain.ServiceKindProcess, Required: true},
		},
	}
	if err := plugins.Register(ctx, plugin); err != nil {
		t.Fatal(err)
	}

	runtimes := ghruntime.NewManager(ghruntime.ManagerOptions{})
	rt, _, err := runtimes.EnsurePrimaryRuntime(ctx, plugin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtimes.AllocateGeneration(rt.ID); err != nil {
		t.Fatal(err)
	}
	if err := runtimes.UpdateRuntimeState(rt.ID, ghdomain.RuntimeStateRunning, "test", time.Now()); err != nil {
		t.Fatal(err)
	}

	topology := ghruntime.NewTopologyStore()
	if err := topology.PutRuntimeGraph(rt, plugin, map[ghdomain.ServiceID]string{
		"primary":   "def-primary",
		"secondary": "def-secondary",
	}); err != nil {
		t.Fatal(err)
	}

	control := &pipelineGameControlPlane{}
	gameAdapter, err := agentbridge.NewRuntimeAdapter(plugins, runtimes, topology, control)
	if err != nil {
		t.Fatal(err)
	}

	adapterRegistry := capability.NewRuntimeAdapterRegistry()
	if err := adapterRegistry.RegisterAdapter(capability.RuntimeTypeGameHost, gameAdapter); err != nil {
		t.Fatal(err)
	}

	toolRegistry := capability.NewToolRegistry()
	toolDef := capability.ToolDefinition{
		ID:          "plugin/pipeline/move",
		ModelName:   "pipeline_move",
		ExtensionID: plugin.ExtensionID,
		ModuleID:    "agent-tools",
		Source:      capability.ToolSourcePlugin,
		Name:        "Move",
		Description: "opaque game operation",
		Enabled:     true,
		InputSchema: json.RawMessage(`{"type":"object"}`),
		Runtime: capability.RuntimeBinding{
			RuntimeType: capability.RuntimeTypeGameHost,
			HandlerName: "vendor.player.move",
			Metadata: map[string]any{
				"extensionId": plugin.ExtensionID,
				"pluginId":    string(plugin.ID),
				"serviceId":   "secondary",
			},
		},
	}
	if err := toolRegistry.Register(ctx, toolDef); err != nil {
		t.Fatal(err)
	}

	pipeline := &execution.ExecutionPipeline{
		TimeoutCtrl: execution.NewTimeoutController(5 * time.Second),
		Dispatcher:  execution.NewRuntimeDispatcher(adapterRegistry),
		ToolResolver: func(ctx context.Context, toolID string) (capability.ToolDefinition, error) {
			def, ok := toolRegistry.Get(ctx, toolID)
			if !ok {
				return capability.ToolDefinition{}, fmt.Errorf("tool not found: %s", toolID)
			}
			return def, nil
		},
	}
	facade := NewToolFacade(toolRegistry, pipeline)

	result, ok := facade.ExecuteTool(
		ctx,
		capability.CapabilityID(toolDef.ID),
		json.RawMessage(`{"direction":"north"}`),
		LegacyScope{
			UserID:         "user-1",
			CharacterID:    "character-1",
			ConversationID: "conversation-1",
			Channel:        "web",
			SessionID:      "host-session-1",
		},
		"tool-call-1",
		"idempotency-1",
	)
	if !ok {
		t.Fatalf("expected tool to resolve, got status=%s err=%+v", result.Status, result.Error)
	}
	if result.Status != string(capability.ToolResultStatusSuccess) {
		t.Fatalf("expected success, got status=%s err=%+v", result.Status, result.Error)
	}
	if control.lastPeer.PluginID != plugin.ID || control.lastPeer.ServiceID != "secondary" {
		t.Fatalf("wrong GameHost route: %+v", control.lastPeer)
	}
	if control.lastMethod != "vendor.player.move" {
		t.Fatalf("wrong plugin method: %s", control.lastMethod)
	}

	bound, found := gameAdapter.SessionRegistry().Resolve(rt.ID, "")
	if !found {
		t.Fatal("expected Agent context to be bound by the real tool pipeline")
	}
	if bound.CharacterID != "character-1" || bound.ConversationID != "conversation-1" || bound.HostSessionID != "host-session-1" {
		t.Fatalf("unexpected Agent context: %+v", bound)
	}
}

func TestEnrichGameHostToolRuntimeBindingPreservesSelectors(t *testing.T) {
	base := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeTypeGameHost,
		Metadata:    map[string]any{"extensionId": "com.example.game"},
	}
	got := enrichGameHostToolRuntimeBinding(base, map[string]any{
		"runtimeType": "game_host",
		"pluginId":    "plugin-a",
		"serviceId":   "service-b",
	})
	if got.Metadata["pluginId"] != "plugin-a" || got.Metadata["serviceId"] != "service-b" {
		t.Fatalf("selectors were not preserved: %+v", got.Metadata)
	}
}
