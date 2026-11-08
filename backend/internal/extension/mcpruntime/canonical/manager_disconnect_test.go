package canonical

import (
	"context"
	"testing"
)

func TestManagerDisconnectNotifiesRegisteredHandlers(t *testing.T) {
	manager := &Manager{reconnecting: map[string]bool{}, suppressed: map[string]bool{}}
	var received []string
	manager.RegisterDisconnectedHandler(func(_ context.Context, serverID string) error {
		received = append(received, serverID)
		return nil
	})
	if err := manager.Disconnect(context.Background(), "qa-server"); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
	if len(received) != 1 || received[0] != "qa-server" {
		t.Fatalf("disconnected handlers received %v, want [qa-server]", received)
	}
}
