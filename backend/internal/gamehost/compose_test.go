package gamehost

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/integration"
	"github.com/u-ai/backend/internal/gamehost/rpc"
	"github.com/u-ai/backend/internal/gamehost/storage"
)

type fakeKernelSource struct{}

func (fakeKernelSource) ListEnabledGamePlugins(ctx context.Context) ([]integration.KernelGamePlugin, error) {
	return nil, nil
}

func composeTestContainer(t *testing.T) *GameHostContainer {
	t.Helper()
	root := filepath.Join(t.TempDir(), "data")
	c, err := ComposeGameHost(GameHostComposeOptions{
		DataRoot:     root,
		KernelSource: fakeKernelSource{},
	})
	if err != nil {
		t.Fatalf("ComposeGameHost error: %v", err)
	}
	return c
}

func TestComposeGameHost_AllComponentsNonNull(t *testing.T) {
	c := composeTestContainer(t)
	if c == nil {
		t.Fatal("container is nil")
	}

	if c.DirectoryManager == nil {
		t.Error("DirectoryManager is nil")
	}
	if c.CheckpointStore == nil {
		t.Error("CheckpointStore is nil")
	}
	if c.ConfigStore == nil {
		t.Error("ConfigStore is nil")
	}
	if c.ConfigResolver == nil {
		t.Error("ConfigResolver is nil")
	}
	if c.PluginRegistry == nil {
		t.Error("PluginRegistry is nil")
	}
	if c.ContributionSync == nil {
		t.Error("ContributionSync is nil")
	}
	if c.NamespaceRegistry == nil {
		t.Error("NamespaceRegistry is nil")
	}
	if c.HandshakeManager == nil {
		t.Error("HandshakeManager is nil")
	}
	if c.ReadyGate == nil {
		t.Error("ReadyGate is nil")
	}
	if c.ConnectionRegistry == nil {
		t.Error("ConnectionRegistry is nil")
	}
	if c.ChannelRegistry == nil {
		t.Error("ChannelRegistry is nil")
	}
	if c.NotificationBridge == nil {
		t.Error("NotificationBridge is nil")
	}
	if c.StateStore == nil {
		t.Error("StateStore is nil")
	}
	if c.BinaryObjectRegistry == nil {
		t.Error("BinaryObjectRegistry is nil")
	}
	if c.StreamManager == nil {
		t.Error("StreamManager is nil")
	}
}

func TestComposeGameHost_SingletonIdentity(t *testing.T) {
	c1 := composeTestContainer(t)
	c2 := composeTestContainer(t)

	if c1 == c2 {
		t.Error("two ComposeGameHost calls must produce distinct container instances")
	}
	if c1.PluginRegistry == c2.PluginRegistry {
		t.Error("PluginRegistry must not be shared across containers")
	}
	if c1.StreamManager == c2.StreamManager {
		t.Error("StreamManager must not be shared across containers")
	}
}

func TestComposeGameHost_DirectoryManagerResolvesPaths(t *testing.T) {
	c := composeTestContainer(t)
	if c.DirectoryManager == nil {
		t.Fatal("DirectoryManager is nil")
	}
	if got := c.DirectoryManager.Root(); got == "" {
		t.Error("DirectoryManager.Root() returned empty")
	}
}

func TestComposeGameHost_NamespaceRegistryOperational(t *testing.T) {
	c := composeTestContainer(t)
	if c.NamespaceRegistry == nil {
		t.Fatal("NamespaceRegistry is nil")
	}
	if _, ok := interface{}(c.NamespaceRegistry).(rpc.NamespaceRegistry); !ok {
		t.Error("NamespaceRegistry does not implement rpc.NamespaceRegistry")
	}
}

func TestComposeGameHost_StorageTypeSatisfaction(t *testing.T) {
	c := composeTestContainer(t)
	var _ storage.DirectoryManager = c.DirectoryManager
}

func TestComposeGameHost_ShutdownSafe(t *testing.T) {
	c := composeTestContainer(t)
	if err := c.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown returned error: %v", err)
	}
	if err := c.Shutdown(context.Background()); err != nil {
		t.Errorf("second Shutdown returned error: %v", err)
	}
}

func TestComposeGameHost_NilContainerShutdownSafe(t *testing.T) {
	var c *GameHostContainer
	if err := c.Shutdown(context.Background()); err != nil {
		t.Errorf("nil container Shutdown returned error: %v", err)
	}
}
