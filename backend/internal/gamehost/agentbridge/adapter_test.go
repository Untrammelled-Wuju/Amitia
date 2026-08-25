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
	scope, ok := adapter.SessionRegistry().Resolve(rt.ID, "main", "")
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

func TestRuntimeAdapterBindAgentContextWithoutToolInvocation(t *testing.T) {
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
	if err := runtimes.UpdateRuntimeState(rt.ID, ghdomain.RuntimeStateRunning, "test", time.Now()); err != nil {
		t.Fatal(err)
	}
	topology := ghruntime.NewTopologyStore()
	if err := topology.PutRuntimeGraph(rt, plugin, map[ghdomain.ServiceID]string{"main": "def-main"}); err != nil {
		t.Fatal(err)
	}
	adapter, err := NewRuntimeAdapter(plugins, runtimes, topology, &genericControlPlane{})
	if err != nil {
		t.Fatal(err)
	}
	binding := capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeGameHost, Metadata: map[string]any{"extensionId": "com.example/game", "serviceId": "main"}}
	invocation := capability.ToolInvocationContext{UserID: "user", CharacterID: "character", ConversationID: "conversation", Channel: "web", SessionID: "host-session"}
	if err := adapter.BindAgentContext(ctx, binding, invocation); err != nil {
		t.Fatal(err)
	}
	scope, ok := adapter.SessionRegistry().Resolve(rt.ID, "main", "")
	if !ok || scope.CharacterID != "character" || scope.HostSessionID != "host-session" {
		t.Fatalf("unexpected bound scope: %+v ok=%v", scope, ok)
	}
}

func TestSessionRegistryDefaultAgentContextEnrichesColdPluginSessions(t *testing.T) {
	sessions := NewSessionRegistry()
	sessions.Bind(SessionScope{
		PluginSessionID: "plugin-session-1",
		PluginID:        "plugin-1",
		RuntimeID:       "runtime-1",
		ServiceID:       "service-1",
		Generation:      1,
	})
	sessions.Bind(SessionScope{
		PluginID:       "plugin-1",
		RuntimeID:      "runtime-1",
		ServiceID:      "service-1",
		Generation:     1,
		UserID:         "user-1",
		CharacterID:    "character-1",
		ConversationID: "conversation-1",
		Channel:        "web",
		HostSessionID:  "host-session-1",
	})

	scope, ok := sessions.Resolve("runtime-1", "service-1", "plugin-session-1")
	if !ok {
		t.Fatal("expected cold plugin session to remain registered")
	}
	if scope.CharacterID != "character-1" || scope.ConversationID != "conversation-1" || scope.HostSessionID != "host-session-1" {
		t.Fatalf("cold plugin session was not enriched: %+v", scope)
	}
	if scope.PluginSessionID != "plugin-session-1" {
		t.Fatalf("opaque plugin session id changed: %+v", scope)
	}
}

func TestSessionRegistryRetainRuntimesPrunesRemovedContext(t *testing.T) {
	sessions := NewSessionRegistry()
	sessions.Bind(SessionScope{RuntimeID: "runtime-keep", ServiceID: "service-a", PluginID: "plugin-a", CharacterID: "char-a"})
	sessions.Bind(SessionScope{RuntimeID: "runtime-remove", ServiceID: "service-a", PluginID: "plugin-b", CharacterID: "char-b"})
	sessions.RetainRuntimes([]ghdomain.RuntimeInstanceID{"runtime-keep"})
	if _, ok := sessions.Resolve("runtime-remove", "service-a", ""); ok {
		t.Fatal("removed runtime context must be pruned")
	}
	if scope, ok := sessions.Resolve("runtime-keep", "service-a", ""); !ok || scope.CharacterID != "char-a" {
		t.Fatalf("active runtime context must be retained: %+v ok=%v", scope, ok)
	}
}

func TestSessionRegistrySeparatesServicesWithSamePluginSession(t *testing.T) {
	sessions := NewSessionRegistry()
	sessions.Bind(SessionScope{RuntimeID: "runtime-1", ServiceID: "service-a", PluginID: "plugin-1", PluginSessionID: "player", CharacterID: "char-a"})
	sessions.Bind(SessionScope{RuntimeID: "runtime-1", ServiceID: "service-b", PluginID: "plugin-1", PluginSessionID: "player", CharacterID: "char-b"})

	a, ok := sessions.Resolve("runtime-1", "service-a", "player")
	if !ok || a.CharacterID != "char-a" {
		t.Fatalf("service-a context was overwritten: %+v ok=%v", a, ok)
	}
	b, ok := sessions.Resolve("runtime-1", "service-b", "player")
	if !ok || b.CharacterID != "char-b" {
		t.Fatalf("service-b context was overwritten: %+v ok=%v", b, ok)
	}
}

func TestSessionRegistryCapacityAndTTLAreBounded(t *testing.T) {
	sessions := NewSessionRegistry()
	sessions.maxEntries = 2
	sessions.ttl = time.Hour
	now := time.Now().UTC()
	sessions.Bind(SessionScope{RuntimeID: "runtime-1", ServiceID: "a", PluginSessionID: "old", UpdatedAt: now.Add(-2 * time.Hour)})
	sessions.Bind(SessionScope{RuntimeID: "runtime-1", ServiceID: "a", PluginSessionID: "one", UpdatedAt: now.Add(-time.Minute)})
	sessions.Bind(SessionScope{RuntimeID: "runtime-1", ServiceID: "a", PluginSessionID: "two", UpdatedAt: now})
	sessions.Bind(SessionScope{RuntimeID: "runtime-1", ServiceID: "a", PluginSessionID: "three", UpdatedAt: now.Add(time.Second)})
	if got := sessions.Size(); got != 2 {
		t.Fatalf("expected bounded registry size=2, got %d", got)
	}
	if _, ok := sessions.Resolve("runtime-1", "a", "old"); ok {
		t.Fatal("expired session must be removed")
	}
}

func TestSessionRegistryCapacityProtectsServiceDefaultContext(t *testing.T) {
	sessions := NewSessionRegistry()
	sessions.maxEntries = 3
	sessions.ttl = time.Hour
	sessions.Bind(SessionScope{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-1", Generation: 1, CharacterID: "char-default"})
	for i := 0; i < 4; i++ {
		sessions.Bind(SessionScope{
			PluginID:        "plugin-1",
			RuntimeID:       "runtime-1",
			ServiceID:       "service-1",
			PluginSessionID: fmt.Sprintf("untrusted-%d", i),
			Generation:      1,
		})
	}
	got, ok := sessions.Resolve("runtime-1", "service-1", "")
	if !ok || got.CharacterID != "char-default" {
		t.Fatalf("service-default Agent context was evicted by plugin sessions: %+v ok=%v", got, ok)
	}
	if gotSize := sessions.Size(); gotSize != 3 {
		t.Fatalf("expected bounded size=3, got %d", gotSize)
	}
}

func TestSessionRegistryPerServiceQuotaCannotEvictOtherServiceDefault(t *testing.T) {
	sessions := NewSessionRegistry()
	sessions.maxEntries = 100
	sessions.maxEntriesPerService = 2
	now := time.Now().UTC()
	sessions.Bind(SessionScope{RuntimeID: "runtime-1", ServiceID: "service-b", CharacterID: "char-b", UpdatedAt: now})
	for i := 0; i < 4; i++ {
		sessions.Bind(SessionScope{RuntimeID: "runtime-1", ServiceID: "service-a", PluginSessionID: fmt.Sprintf("session-%d", i), UpdatedAt: now.Add(time.Duration(i+1) * time.Second)})
	}
	if scope, ok := sessions.Resolve("runtime-1", "service-b", ""); !ok || scope.CharacterID != "char-b" {
		t.Fatalf("service-a flood evicted service-b default context: %+v ok=%v", scope, ok)
	}
	countA := 0
	for _, id := range []string{"session-0", "session-1", "session-2", "session-3"} {
		if _, ok := sessions.Resolve("runtime-1", "service-a", id); ok {
			countA++
		}
	}
	if countA != 2 {
		t.Fatalf("expected service-a quota to retain 2 sessions, got %d", countA)
	}
}
