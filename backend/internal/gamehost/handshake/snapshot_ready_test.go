package handshake_test

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/handshake"
	"github.com/u-ai/backend/internal/gamehost/ipc"
)

func TestHandshakeSnapshotReadyAtTracksCommittedReady(t *testing.T) {
	mgr, _ := newTestHandshakeManager()
	const connID = "conn-ready-at"
	mgr.RegisterConnection(connID)

	_, err := mgr.HandleHello(context.Background(), connID, ipc.Peer{
		PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-a",
	}, &handshake.HelloRequest{SupportedProtocols: []string{"amitia-game-host/1"}})
	if err != nil {
		t.Fatalf("hello failed: %v", err)
	}

	staged := mgr.GetSnapshot(connID)
	if staged == nil {
		t.Fatal("staged snapshot is missing")
	}
	if !staged.ReadyAt.IsZero() {
		t.Fatalf("staged handshake reported ReadyAt before transport commit: %s", staged.ReadyAt)
	}

	if !mgr.ConfirmReady(connID) {
		t.Fatal("expected ConfirmReady to succeed")
	}
	ready := mgr.GetSnapshot(connID)
	if ready == nil || ready.ReadyAt.IsZero() {
		t.Fatalf("committed ready handshake must have ReadyAt, got %#v", ready)
	}
}

func TestHandshakeManagerGetSnapshotReturnsDeepCopy(t *testing.T) {
	mgr, _ := newTestHandshakeManager()
	const connID = "conn-snapshot-copy"
	mgr.RegisterConnection(connID)

	_, err := mgr.HandleHello(context.Background(), connID, ipc.Peer{
		PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-a",
	}, &handshake.HelloRequest{
		SupportedProtocols: []string{"amitia-game-host/1"},
		Capabilities:       []string{"custom_rpc"},
		RPCNamespaces:      []string{"example.core"},
	})
	if err != nil {
		t.Fatalf("hello failed: %v", err)
	}
	if !mgr.ConfirmReady(connID) {
		t.Fatal("expected ConfirmReady to succeed")
	}

	first := mgr.GetSnapshot(connID)
	if first == nil {
		t.Fatal("snapshot missing")
	}
	originalReadyAt := first.ReadyAt
	first.Protocol = "mutated"
	first.Capabilities[0] = "state_streaming"
	first.RPCNamespaces[0] = "mutated.namespace"
	first.ReadyAt = first.ReadyAt.AddDate(1, 0, 0)

	second := mgr.GetSnapshot(connID)
	if second == nil {
		t.Fatal("snapshot missing after mutation")
	}
	if second.Protocol != "amitia-game-host/1" {
		t.Fatalf("external mutation changed protocol: %q", second.Protocol)
	}
	if len(second.Capabilities) != 1 || second.Capabilities[0] != "custom_rpc" {
		t.Fatalf("external mutation changed capabilities: %#v", second.Capabilities)
	}
	if len(second.RPCNamespaces) != 1 || second.RPCNamespaces[0] != "example.core" {
		t.Fatalf("external mutation changed namespaces: %#v", second.RPCNamespaces)
	}
	if !second.ReadyAt.Equal(originalReadyAt) {
		t.Fatalf("clone changed ReadyAt: got %s want %s", second.ReadyAt, originalReadyAt)
	}
}
