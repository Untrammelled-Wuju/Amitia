package handshake_test

import (
	"context"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/handshake"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/rpc"
)

type fakeRuntimeValidator struct {
	runtimes map[string]bool
	services map[string]bool
}

func newFakeRuntimeValidator() *fakeRuntimeValidator {
	return &fakeRuntimeValidator{
		runtimes: map[string]bool{
			"runtime-1": true,
			"runtime-2": true,
		},
		services: map[string]bool{
			"runtime-1/service-a": true,
			"runtime-1/service-b": true,
			"runtime-2/service-a": true,
		},
	}
}

func (f *fakeRuntimeValidator) RuntimeExists(runtimeID string) (bool, error) {
	return f.runtimes[runtimeID], nil
}

func (f *fakeRuntimeValidator) ServiceBelongsToRuntime(runtimeID, serviceID, pluginID string) error {
	if !f.services[runtimeID+"/"+serviceID] {
		return domain.NewHostError(domain.ErrNotFound, "service not found")
	}
	return nil
}

type fakeDescriptorProvider struct {
	caps  map[string][]string
	chs   map[string][]string
}

func newFakeDescriptorProvider() *fakeDescriptorProvider {
	return &fakeDescriptorProvider{
		caps: map[string][]string{
			"plugin-1": {"custom_rpc", "event_streaming"},
			"plugin-2": {"realtime_control"},
		},
		chs: map[string][]string{
			"plugin-1": {"events", "state"},
		},
	}
}

func (f *fakeDescriptorProvider) DescriptorCapabilities(pluginID string) ([]string, error) {
	return f.caps[pluginID], nil
}

func (f *fakeDescriptorProvider) DescriptorChannels(pluginID string) ([]string, error) {
	return f.chs[pluginID], nil
}

func (f *fakeDescriptorProvider) HasCapability(pluginID, capability string) bool {
	for _, c := range f.caps[pluginID] {
		if c == capability {
			return true
		}
	}
	return false
}

func newTestHandshakeManager() (*handshake.HandshakeManager, *rpc.NamespaceRegistry) {
	valid := newFakeRuntimeValidator()
	validator := rpc.NewNamespaceRegistry(rpc.NamespaceRegistryConfig{})

	adapter := handshake.NewNamespaceAdapter(validator)
	advertiser := handshake.NoopChannelAdvertiser{}

	mgr := handshake.NewHandshakeManager(handshake.HandshakeManagerConfig{
		HostSupportedProtocols: []string{"amitia-game-host/1"},
		HostCapabilities: []domain.Capability{
			domain.CapabilityCustomRPC,
			domain.CapabilityEventStreaming,
			domain.CapabilityStateStreaming,
			domain.CapabilityBinaryStreaming,
			domain.CapabilityHostAPI,
		},
		NamespaceAdapter:    adapter,
		ChannelAdvertiser:   advertiser,
		RuntimeValidator:    valid,
		PreReadyAllowlist:   nil,
	})

	return mgr, &validator
}

func TestHandshakeManager_BasicHello(t *testing.T) {
	mgr, _ := newTestHandshakeManager()
	peer := ipc.Peer{
		PluginID:  "plugin-1",
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
	}
	connID := "conn-1"
	mgr.RegisterConnection(connID)

	hello := &handshake.HelloRequest{
		SupportedProtocols: []string{"amitia-game-host/1"},
		Capabilities:       []string{"custom_rpc"},
		RPCNamespaces:      []string{"minecraft"},
	}

	resp, err := mgr.HandleHello(context.Background(), connID, peer, hello)
	if err != nil {
		t.Fatalf("hello failed: %v", err)
	}

	if resp.Protocol != "amitia-game-host/1" {
		t.Errorf("protocol mismatch: got %s", resp.Protocol)
	}

	if !mgr.IsReady(connID) {
		t.Error("connection should be ready after hello")
	}
}

func TestHandshakeManager_DuplicateHelloRejected(t *testing.T) {
	mgr, _ := newTestHandshakeManager()
	peer := ipc.Peer{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-a"}
	connID := "conn-1"
	mgr.RegisterConnection(connID)

	hello := &handshake.HelloRequest{
		SupportedProtocols: []string{"amitia-game-host/1"},
	}

	_, err := mgr.HandleHello(context.Background(), connID, peer, hello)
	if err != nil {
		t.Fatalf("first hello failed: %v", err)
	}

	_, err = mgr.HandleHello(context.Background(), connID, peer, hello)
	if err == nil {
		t.Error("duplicate hello should be rejected")
	}
}

func TestHandshakeManager_ProtocolMismatch(t *testing.T) {
	mgr, _ := newTestHandshakeManager()
	peer := ipc.Peer{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-a"}
	connID := "conn-1"
	mgr.RegisterConnection(connID)

	hello := &handshake.HelloRequest{
		SupportedProtocols: []string{"amitia-game-host/2"},
	}

	_, err := mgr.HandleHello(context.Background(), connID, peer, hello)
	if err == nil {
		t.Error("unknown protocol should fail")
	}
}

func TestHandshakeManager_EmptyProtocolRejected(t *testing.T) {
	mgr, _ := newTestHandshakeManager()
	peer := ipc.Peer{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-a"}
	connID := "conn-1"
	mgr.RegisterConnection(connID)

	hello := &handshake.HelloRequest{
		SupportedProtocols: []string{},
	}

	_, err := mgr.HandleHello(context.Background(), connID, peer, hello)
	if err == nil {
		t.Error("empty protocol list should fail")
	}
}

func TestHandshakeManager_ReservedNamespaceRejected(t *testing.T) {
	mgr, _ := newTestHandshakeManager()
	peer := ipc.Peer{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-a"}
	connID := "conn-1"
	mgr.RegisterConnection(connID)

	hello := &handshake.HelloRequest{
		SupportedProtocols: []string{"amitia-game-host/1"},
		Capabilities:       []string{"custom_rpc"},
		RPCNamespaces:      []string{"host"},
	}

	_, err := mgr.HandleHello(context.Background(), connID, peer, hello)
	if err == nil {
		t.Error("reserved namespace should fail")
	}
}

func TestHandshakeManager_NamespaceConflict(t *testing.T) {
	mgr, _ := newTestHandshakeManager()
	peer1 := ipc.Peer{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-a"}
	peer2 := ipc.Peer{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-b"}

	connID1 := "conn-1"
	connID2 := "conn-2"

	mgr.RegisterConnection(connID1)
	mgr.RegisterConnection(connID2)

	hello1 := &handshake.HelloRequest{
		SupportedProtocols: []string{"amitia-game-host/1"},
		Capabilities:       []string{"custom_rpc"},
		RPCNamespaces:      []string{"minecraft"},
	}

	_, err := mgr.HandleHello(context.Background(), connID1, peer1, hello1)
	if err != nil {
		t.Fatalf("first hello failed: %v", err)
	}

	hello2 := &handshake.HelloRequest{
		SupportedProtocols: []string{"amitia-game-host/1"},
		Capabilities:       []string{"custom_rpc"},
		RPCNamespaces:      []string{"minecraft"},
	}

	_, err = mgr.HandleHello(context.Background(), connID2, peer2, hello2)
	if err == nil {
		t.Error("namespace conflict should fail")
	}
}

func TestHandshakeManager_CrossRuntimeSameNamespace(t *testing.T) {
	mgr, _ := newTestHandshakeManager()
	peer1 := ipc.Peer{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-a"}
	peer2 := ipc.Peer{PluginID: "plugin-2", RuntimeID: "runtime-2", ServiceID: "service-a"}

	connID1, connID2 := "conn-1", "conn-2"
	mgr.RegisterConnection(connID1)
	mgr.RegisterConnection(connID2)

	hello := &handshake.HelloRequest{
		SupportedProtocols: []string{"amitia-game-host/1"},
		Capabilities:       []string{"custom_rpc"},
		RPCNamespaces:      []string{"minecraft"},
	}

	_, err := mgr.HandleHello(context.Background(), connID1, peer1, hello)
	if err != nil {
		t.Fatalf("first hello failed: %v", err)
	}

	_, err = mgr.HandleHello(context.Background(), connID2, peer2, hello)
	if err != nil {
		t.Errorf("cross-runtime same namespace should succeed: %v", err)
	}
}

func TestHandshakeManager_SnapshotAfterReady(t *testing.T) {
	mgr, _ := newTestHandshakeManager()
	peer := ipc.Peer{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-a"}
	connID := "conn-1"

	mgr.RegisterConnection(connID)

	hello := &handshake.HelloRequest{
		SupportedProtocols: []string{"amitia-game-host/1"},
		Capabilities:       []string{"custom_rpc"},
		RPCNamespaces:      []string{"minecraft", "agent"},
	}

	_, err := mgr.HandleHello(context.Background(), connID, peer, hello)
	if err != nil {
		t.Fatalf("hello failed: %v", err)
	}

	snap := mgr.GetSnapshot(connID)
	if snap == nil {
		t.Fatal("snapshot should not be nil")
	}

	if snap.Protocol != "amitia-game-host/1" {
		t.Errorf("protocol mismatch in snapshot")
	}

	if len(snap.RPCNamespaces) != 2 {
		t.Errorf("expected 2 namespaces in snapshot, got %d", len(snap.RPCNamespaces))
	}

	for i := 1; i < len(snap.RPCNamespaces); i++ {
		if strings.Compare(snap.RPCNamespaces[i-1], snap.RPCNamespaces[i]) > 0 {
			t.Error("namespaces should be sorted")
		}
	}
}

func TestHandshakeManager_NamespacesRequireCustomRPC(t *testing.T) {
	mgr, _ := newTestHandshakeManager()
	peer := ipc.Peer{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-a"}
	connID := "conn-1"
	mgr.RegisterConnection(connID)

	hello := &handshake.HelloRequest{
		SupportedProtocols: []string{"amitia-game-host/1"},
		Capabilities:       []string{"event_streaming"},
		RPCNamespaces:      []string{"minecraft"},
	}

	_, err := mgr.HandleHello(context.Background(), connID, peer, hello)
	if err == nil {
		t.Error("namespaces without custom_rpc should fail")
	}
}

func TestHandshakeManager_HandleHelloFromEnvelope(t *testing.T) {
	mgr, _ := newTestHandshakeManager()
	peer := ipc.Peer{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-a"}
	connID := "conn-1"
	mgr.RegisterConnection(connID)

	payload := []byte(`{
		"supportedProtocols": ["amitia-game-host/1"],
		"capabilities": ["custom_rpc"],
		"rpcNamespaces": ["minecraft"]
	}`)

	resp, err := mgr.HandleHelloFromEnvelope(context.Background(), connID, peer, payload)
	if err != nil {
		t.Fatalf("hello failed: %v", err)
	}
	if resp.Protocol != "amitia-game-host/1" {
		t.Error("protocol mismatch")
	}
}

func TestHandshakeManager_InvalidJSONRejected(t *testing.T) {
	mgr, _ := newTestHandshakeManager()
	peer := ipc.Peer{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-a"}
	connID := "conn-1"
	mgr.RegisterConnection(connID)

	_, err := mgr.HandleHelloFromEnvelope(context.Background(), connID, peer, []byte(`invalid`))
	if err == nil {
		t.Error("invalid JSON should be rejected")
	}
}

func TestHandshakeManager_RemoveConnection(t *testing.T) {
	mgr, _ := newTestHandshakeManager()
	connID := "conn-1"
	mgr.RegisterConnection(connID)
	mgr.RemoveConnection(connID)

	state, ok := mgr.GetState(connID)
	if ok {
		t.Errorf("expected not found after removal, got state %s", state)
	}
}
