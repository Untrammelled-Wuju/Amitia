package gamehost

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/integration"
	"github.com/u-ai/backend/internal/gamehost/integration/service_definition"
	"github.com/u-ai/backend/internal/gamehost/rpc"
	"github.com/u-ai/backend/internal/gamehost/storage"
)

type fakeDefinitionReconcile struct{}

func (fakeDefinitionReconcile) ReconcileExtension(extensionID string) *service_definition.ReconcileReport {
	return &service_definition.ReconcileReport{ExtensionID: extensionID}
}

type fakeKernelSource struct{}

func (fakeKernelSource) ListEnabledGamePlugins(ctx context.Context) ([]integration.KernelGamePlugin, error) {
	return nil, nil
}

func composeTestContainer(t *testing.T) *GameHostContainer {
	t.Helper()
	root := filepath.Join(t.TempDir(), "data")
	supervisorDir := filepath.Join(t.TempDir(), "supervisor")
	supervisor := trusted_service.NewProcessSupervisor(supervisorDir)
	c, err := ComposeGameHost(GameHostComposeOptions{
		DataRoot:            root,
		KernelSource:        fakeKernelSource{},
		TrustedSupervisor:   supervisor,
		DefinitionReconcile: fakeDefinitionReconcile{},
	})
	if err != nil {
		t.Fatalf("ComposeGameHost error: %v", err)
	}
	return c
}

func composeTestContainerWithSupervisor(t *testing.T) *GameHostContainer {
	t.Helper()
	root := filepath.Join(t.TempDir(), "data")
	supervisorDir := filepath.Join(t.TempDir(), "supervisor")
	supervisor := trusted_service.NewProcessSupervisor(supervisorDir)
	c, err := ComposeGameHost(GameHostComposeOptions{
		DataRoot:            root,
		KernelSource:        fakeKernelSource{},
		TrustedSupervisor:   supervisor,
		DefinitionReconcile: fakeDefinitionReconcile{},
	})
	if err != nil {
		t.Fatalf("ComposeGameHost with supervisor error: %v", err)
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
	if c.RuntimeManager == nil {
		t.Error("RuntimeManager is nil")
	}
	if c.RuntimeTopologyStore == nil {
		t.Error("RuntimeTopologyStore is nil")
	}
	if c.RuntimeExecutor == nil {
		t.Error("RuntimeExecutor is nil")
	}
	if c.RuntimeHealth == nil {
		t.Error("RuntimeHealth is nil")
	}
	if c.RuntimeProvisioner == nil {
		t.Error("RuntimeProvisioner is nil")
	}
}

func TestComposeGameHost_RuntimeComponentsShared(t *testing.T) {
	c := composeTestContainer(t)

	if c.RuntimeManager == nil {
		t.Fatal("RuntimeManager is nil")
	}
	if c.RuntimeTopologyStore == nil {
		t.Fatal("RuntimeTopologyStore is nil")
	}
	if c.RuntimeExecutor == nil {
		t.Fatal("RuntimeExecutor is nil")
	}

	ctx := context.Background()
	rt, err := c.RuntimeManager.Create(ctx, "plugin-test")
	if err != nil {
		t.Fatalf("RuntimeManager.Create error: %v", err)
	}

	ref, err := c.RuntimeManager.GetRuntime(rt.ID)
	if err != nil {
		t.Fatalf("RuntimeManager.GetRuntime error: %v", err)
	}
	if ref.ID != rt.ID {
		t.Errorf("got RuntimeID %q, want %q", ref.ID, rt.ID)
	}

	topo, err := c.RuntimeTopologyStore.GetTopology(rt.ID)
	if err == nil {
		t.Error("expected error for runtime without topology, got nil")
	}
	_ = topo
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
	if err := c.Start(context.Background()); err != nil {
		t.Errorf("Start returned error: %v", err)
	}
	if err := c.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown returned error: %v", err)
	}
	if err := c.Shutdown(context.Background()); err != nil {
		t.Errorf("second Shutdown returned error: %v", err)
	}
}

func TestComposeGameHost_ControlComponentsWired(t *testing.T) {
	c := composeTestContainer(t)
	if c.AuthorityManager == nil {
		t.Error("AuthorityManager is nil")
	}
	if c.OutputGate == nil {
		t.Error("OutputGate is nil")
	}
	if c.TakeoverService == nil {
		t.Error("TakeoverService is nil")
	}
	if c.AuthorityAudit == nil {
		t.Error("AuthorityAudit is nil")
	}
}

func TestComposeGameHost_ControlComponentsShared(t *testing.T) {
	c := composeTestContainer(t)
	if c.TakeoverService == nil || c.AuthorityManager == nil {
		t.Fatal("control components nil")
	}
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-shared-check")
	pluginID := domain.PluginID("plugin-shared-check")
	if _, err := c.AuthorityManager.Create(ctx, runtimeID, pluginID); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	snap, err := c.AuthorityManager.GetSnapshot(ctx, runtimeID)
	if err != nil {
		t.Fatalf("GetSnapshot failed: %v", err)
	}
	if snap.PluginID != pluginID {
		t.Errorf("AuthorityManager state not consistent: got %q want %q", snap.PluginID, pluginID)
	}
}

func TestComposeGameHost_NilContainerShutdownSafe(t *testing.T) {
	var c *GameHostContainer
	if err := c.Shutdown(context.Background()); err != nil {
		t.Errorf("nil container Shutdown returned error: %v", err)
	}
}
