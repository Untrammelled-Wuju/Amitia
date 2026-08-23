package agentbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	ghdomain "github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/registry"
	ghruntime "github.com/u-ai/backend/internal/gamehost/runtime"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type captureCompanionPreparer struct {
	calls       int
	extensionID string
	gameRoot    string
	gameVersion string
}

func (p *captureCompanionPreparer) PrepareRequired(_ context.Context, extensionID, gameRoot, gameVersion string) error {
	p.calls++
	p.extensionID = extensionID
	p.gameRoot = gameRoot
	p.gameVersion = gameVersion
	return nil
}

type gameProtocolControlPlane struct {
	methods []string
}

func (f *gameProtocolControlPlane) Attach(context.Context, ipc.Peer, ipc.Transport) (*ipc.Connection, error) {
	return nil, fmt.Errorf("not used")
}
func (f *gameProtocolControlPlane) Detach(context.Context, ipc.ConnectionID) error { return nil }
func (f *gameProtocolControlPlane) Send(context.Context, ipc.Peer, protocol.Envelope) error {
	return nil
}
func (f *gameProtocolControlPlane) Shutdown(context.Context) error { return nil }
func (f *gameProtocolControlPlane) SendRequest(_ context.Context, peer ipc.Peer, envelope protocol.Envelope, _ time.Duration) (*protocol.Envelope, error) {
	if envelope.RuntimeID != string(peer.RuntimeID) || envelope.PluginID != string(peer.PluginID) || envelope.ServiceID != string(peer.ServiceID) {
		return nil, fmt.Errorf("request routing did not match resolved peer")
	}
	f.methods = append(f.methods, envelope.Method)
	var payload any
	switch envelope.Method {
	case protocol.MethodGameSessionOpen:
		payload = protocol.GameSession{ID: "session-1", GameID: "minecraft-java", Status: protocol.GameSessionReady}
	case protocol.MethodGameObservationGet:
		payload = protocol.GameObservation{SessionID: "session-1", Sequence: 1, ObservedAt: time.Now().UTC(), State: map[string]any{"health": float64(20)}}
	case protocol.MethodGameActionExecute:
		payload = protocol.GameActionResult{ActionID: "action-1", SessionID: "session-1", Status: protocol.GameActionSucceeded}
	default:
		return nil, fmt.Errorf("unexpected method %s", envelope.Method)
	}
	response, err := protocol.NewResponse("response-"+envelope.ID, envelope.ID, payload)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

func TestRuntimeAdapterGameProtocolV2Lifecycle(t *testing.T) {
	ctx := context.Background()
	plugins := registry.NewRegistry()
	plugin := ghdomain.PluginDescriptor{
		ID:              "minecraft",
		ExtensionID:     "com.amitia.minecraft",
		Name:            "Minecraft",
		Version:         "1.0.0",
		ProtocolVersion: protocol.ProtocolVersion,
		Services:        []ghdomain.ServiceDescriptor{{ID: "main", Name: "main", Kind: ghdomain.ServiceKindProcess, Required: true}},
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
	if err := runtimes.UpdateRuntimeState(rt.ID, ghdomain.RuntimeStateStarting, "test", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := runtimes.UpdateRuntimeState(rt.ID, ghdomain.RuntimeStateRunning, "test", time.Now()); err != nil {
		t.Fatal(err)
	}

	topology := ghruntime.NewTopologyStore()
	if err := topology.PutRuntimeGraph(rt, plugin, map[ghdomain.ServiceID]string{"main": "def-main"}); err != nil {
		t.Fatal(err)
	}

	control := &gameProtocolControlPlane{}
	adapter, err := NewRuntimeAdapter(plugins, runtimes, topology, control)
	if err != nil {
		t.Fatal(err)
	}
	preparer := &captureCompanionPreparer{}
	adapter.SetCompanionPreparer(preparer)
	binding := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeTypeGameHost,
		Metadata:    map[string]any{"extensionId": "com.amitia.minecraft", "serviceId": "main"},
	}

	calls := []struct {
		method string
		input  any
	}{
		{protocol.MethodGameSessionOpen, protocol.GameSessionOpenRequest{GameRoot: "/games/minecraft", GameVersion: "1.21.8"}},
		{protocol.MethodGameObservationGet, map[string]any{"sessionId": "session-1"}},
		{protocol.MethodGameActionExecute, protocol.GameAction{ID: "action-1", SessionID: "session-1", Type: "minecraft.move", Parameters: json.RawMessage(`{"x":1}`)}},
	}
	for idx, call := range calls {
		input, err := json.Marshal(call.input)
		if err != nil {
			t.Fatal(err)
		}
		binding.HandlerName = call.method
		result := adapter.Execute(ctx, binding, capability.ToolInvocationContext{
			InvocationID:   fmt.Sprintf("invoke-%d", idx+1),
			UserID:         "user-1",
			CharacterID:    "char-1",
			ConversationID: "conv-1",
			Channel:        "web",
			SessionID:      "host-session-1",
		}, input)
		if result.Status != capability.ToolResultStatusSuccess {
			t.Fatalf("%s failed: %+v", call.method, result.Error)
		}
		if result.RuntimeID != string(rt.ID) || result.Generation != 1 {
			t.Fatalf("unexpected runtime routing for %s: runtime=%s generation=%d", call.method, result.RuntimeID, result.Generation)
		}
		if len(result.Structured) == 0 {
			t.Fatalf("%s returned empty structured result", call.method)
		}
	}

	if preparer.calls != 1 || preparer.extensionID != "com.amitia.minecraft" || preparer.gameRoot != "/games/minecraft" || preparer.gameVersion != "1.21.8" {
		t.Fatalf("unexpected companion preparation: %+v", preparer)
	}
	scope, ok := adapter.SessionRegistry().Resolve(rt.ID, "session-1")
	if !ok {
		t.Fatal("game session scope was not bound")
	}
	if scope.CharacterID != "char-1" || scope.UserID != "user-1" || scope.ConversationID != "conv-1" || scope.HostSessionID != "host-session-1" {
		t.Fatalf("unexpected game session scope: %+v", scope)
	}

	want := []string{protocol.MethodGameSessionOpen, protocol.MethodGameObservationGet, protocol.MethodGameActionExecute}
	if len(control.methods) != len(want) {
		t.Fatalf("unexpected method count: %v", control.methods)
	}
	for i := range want {
		if control.methods[i] != want[i] {
			t.Fatalf("method %d = %q, want %q", i, control.methods[i], want[i])
		}
	}
}
