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

type genericControlPlane struct{ methods []string }

func (f *genericControlPlane) Attach(context.Context, ipc.Peer, ipc.Transport) (*ipc.Connection, error) {
	return nil, fmt.Errorf("not used")
}
func (f *genericControlPlane) Detach(context.Context, ipc.ConnectionID) error          { return nil }
func (f *genericControlPlane) Send(context.Context, ipc.Peer, protocol.Envelope) error { return nil }
func (f *genericControlPlane) Shutdown(context.Context) error                          { return nil }
func (f *genericControlPlane) SendRequest(_ context.Context, peer ipc.Peer, envelope protocol.Envelope, _ time.Duration) (*protocol.Envelope, error) {
	if envelope.RuntimeID != string(peer.RuntimeID) || envelope.PluginID != string(peer.PluginID) || envelope.ServiceID != string(peer.ServiceID) {
		return nil, fmt.Errorf("route mismatch")
	}
	f.methods = append(f.methods, envelope.Method)
	response, err := protocol.NewResponse("response-"+envelope.ID, envelope.ID, map[string]any{"method": envelope.Method, "ok": true})
	if err != nil {
		return nil, err
	}
	return &response, nil
}

func TestRuntimeAdapterForwardsOpaquePluginMethods(t *testing.T) {
	ctx := context.Background()
	plugins := registry.NewRegistry()
	plugin := ghdomain.PluginDescriptor{ID: "example", ExtensionID: "com.example/game", Name: "Example", Version: "1.0.0", ProtocolVersion: protocol.ProtocolVersion, Services: []ghdomain.ServiceDescriptor{{ID: "main", Name: "main", Kind: ghdomain.ServiceKindProcess, Required: true}}}
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
	control := &genericControlPlane{}
	adapter, err := NewRuntimeAdapter(plugins, runtimes, topology, control)
	if err != nil {
		t.Fatal(err)
	}
	binding := capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeGameHost, HandlerName: "vendor.player.move", Metadata: map[string]any{"extensionId": "com.example/game", "serviceId": "main"}}
	input, _ := json.Marshal(map[string]any{"opaque": true})
	result := adapter.Execute(ctx, binding, capability.ToolInvocationContext{InvocationID: "invoke-1", UserID: "u", CharacterID: "c", ConversationID: "conv", Channel: "web", SessionID: "host-session"}, input)
	if result.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("Execute failed: %+v", result.Error)
	}
	if len(control.methods) != 1 || control.methods[0] != "vendor.player.move" {
		t.Fatalf("unexpected methods %v", control.methods)
	}
	scope, ok := adapter.SessionRegistry().Resolve(rt.ID, "")
	if !ok {
		t.Fatal("runtime context not bound")
	}
	if scope.HostSessionID != "host-session" || scope.CharacterID != "c" {
		t.Fatalf("unexpected scope %+v", scope)
	}
	if result.Metadata["pluginId"] != "example" {
		t.Fatalf("unexpected metadata %+v", result.Metadata)
	}
}
