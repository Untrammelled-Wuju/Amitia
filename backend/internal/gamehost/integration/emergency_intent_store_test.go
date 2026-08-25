package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
	ghruntime "github.com/u-ai/backend/internal/gamehost/runtime"
)

func TestFileEmergencyIntentStorePersistsByStablePluginIdentity(t *testing.T) {
	ctx := context.Background()
	statePath := filepath.Join(t.TempDir(), "gamehost", "control", "emergency-intents.json")
	store, err := NewFileEmergencyIntentStore(statePath)
	if err != nil {
		t.Fatalf("NewFileEmergencyIntentStore: %v", err)
	}
	oldRuntime := domain.RuntimeInstanceID("rt-old")
	newRuntime := domain.RuntimeInstanceID("rt-new")
	pluginID := domain.PluginID("publisher/game")
	if err := store.CommitEmergencyIntentForPlugin(ctx, oldRuntime, pluginID, "op-emergency-1"); err != nil {
		t.Fatalf("CommitEmergencyIntentForPlugin: %v", err)
	}

	reopened, err := NewFileEmergencyIntentStore(statePath)
	if err != nil {
		t.Fatalf("reopen emergency store: %v", err)
	}
	if !reopened.IsEmergencyLatchedForPlugin(ctx, newRuntime, pluginID) {
		t.Fatal("emergency latch must survive restart/runtime-id replacement by plugin identity")
	}
	if got, ok := reopened.GetEmergencyOperationIDForPlugin(ctx, newRuntime, pluginID); !ok || got != "op-emergency-1" {
		t.Fatalf("persisted operation id = %q, %v", got, ok)
	}

	if err := reopened.ClearEmergencyLatchForPlugin(ctx, newRuntime, pluginID, "operator"); err != nil {
		t.Fatalf("ClearEmergencyLatchForPlugin: %v", err)
	}
	reopenedAgain, err := NewFileEmergencyIntentStore(statePath)
	if err != nil {
		t.Fatalf("reopen after clear: %v", err)
	}
	if reopenedAgain.IsEmergencyLatchedForPlugin(ctx, oldRuntime, pluginID) {
		t.Fatal("cleared emergency latch unexpectedly persisted")
	}
}

func TestFileEmergencyIntentStoreFailsClosedOnCorruptState(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "emergency-intents.json")
	if err := os.WriteFile(statePath, []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFileEmergencyIntentStore(statePath); err == nil {
		t.Fatal("corrupt emergency state must fail closed")
	}
}

func TestManagerEmergencyLatchBridgeRestoresLatchForReplacementRuntime(t *testing.T) {
	ctx := context.Background()
	statePath := filepath.Join(t.TempDir(), "emergency-intents.json")
	store, err := NewFileEmergencyIntentStore(statePath)
	if err != nil {
		t.Fatal(err)
	}

	firstManager := ghruntime.NewManager(ghruntime.ManagerOptions{})
	firstRuntime, err := firstManager.Create(ctx, domain.PluginID("publisher/game"))
	if err != nil {
		t.Fatal(err)
	}
	firstBridge := NewManagerEmergencyLatchBridge(firstManager, store)
	if err := firstBridge.CommitEmergencyIntent(ctx, firstRuntime.ID, "op-1"); err != nil {
		t.Fatal(err)
	}

	reopened, err := NewFileEmergencyIntentStore(statePath)
	if err != nil {
		t.Fatal(err)
	}
	secondManager := ghruntime.NewManager(ghruntime.ManagerOptions{})
	secondRuntime, err := secondManager.Create(ctx, domain.PluginID("publisher/game"))
	if err != nil {
		t.Fatal(err)
	}
	if secondRuntime.ID == firstRuntime.ID {
		t.Fatal("test requires a replacement runtime id")
	}
	secondBridge := NewManagerEmergencyLatchBridge(secondManager, reopened)
	if !secondBridge.IsEmergencyLatched(ctx, secondRuntime.ID) {
		t.Fatal("replacement runtime must inherit plugin emergency latch")
	}
	if !secondManager.IsEmergencyLatched(secondRuntime.ID) {
		t.Fatal("bridge must synchronize durable latch into runtime manager")
	}
}
