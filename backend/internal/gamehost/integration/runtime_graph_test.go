package integration

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	ghdomain "github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/integration/service_definition"
	"github.com/u-ai/backend/internal/gamehost/registry"
	ghruntime "github.com/u-ai/backend/internal/gamehost/runtime"
)

type testKernelSource struct {
	plugins []KernelGamePlugin
}

func (s testKernelSource) ListEnabledGamePlugins(ctx context.Context) ([]KernelGamePlugin, error) {
	return s.plugins, nil
}

func newTestProvisioner(t *testing.T, source KernelContributionSource) (*RuntimeGraphProvisioner, *ghruntime.Manager, *trusted_service.ProcessSupervisor) {
	t.Helper()
	rtManager := ghruntime.NewManager(ghruntime.ManagerOptions{})
	topoStore := ghruntime.NewTopologyStore()
	pluginReg := registry.NewRegistry()
	supervisor := trusted_service.NewProcessSupervisor(t.TempDir())
	defMapper := service_definition.NewDefinitionMapper()

	provisioner, err := NewRuntimeGraphProvisioner(RuntimeGraphProvisionerOptions{
		Source:           source,
		Mapper:           NewDefaultGamePluginContributionMapper(),
		PluginRegistry:   pluginReg,
		RuntimeManager:   rtManager,
		TopologyStore:    topoStore,
		Supervisor:       supervisor,
		DefinitionMapper: defMapper,
	})
	if err != nil {
		t.Fatalf("NewRuntimeGraphProvisioner error: %v", err)
	}
	return provisioner, rtManager, supervisor
}

func makeTestPlugin(extensionID, pluginID string, entryPoint string) KernelGamePlugin {
	return KernelGamePlugin{
		Extension: domain.ExtensionDefinition{
			ID:   domain.ExtensionID(extensionID),
			Name: domain.LocalizedText{Default: "Test Ext"},
			Modules: []domain.ModuleDefinition{{
				ID:          domain.ModuleID("runtime-module"),
				ExtensionID: domain.ExtensionID(extensionID),
				Type:        domain.ModuleTypeService,
				Runtime:     &domain.RuntimeDefinition{Type: domain.RuntimeTypeService, EntryPoint: entryPoint},
			}},
		},
		Contribution: domain.ContributionDefinition{
			ID:          domain.ContributionID(pluginID),
			ModuleID:    domain.ModuleID(pluginID),
			ExtensionID: domain.ExtensionID(extensionID),
			Kind:        domain.ContributionKindGamePlugin,
			Name:        domain.LocalizedText{Default: "Test Plugin"},
			Definition: map[string]any{
				"protocolVersion": "amitia-game-host/1",
				"runtimeModuleId": "runtime-module",
				"network":         map[string]any{"mode": "none"},
			},
		},
	}
}

func TestRuntimeGraphProvisioner_Reconcile_CreatesRuntime(t *testing.T) {
	source := testKernelSource{plugins: []KernelGamePlugin{
		makeTestPlugin("ext-a", "game", "C:/test/game.exe"),
	}}

	provisioner, rtManager, _ := newTestProvisioner(t, source)

	err := provisioner.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("Reconcile error: %v", err)
	}

	runtimes := rtManager.ListRuntimes()
	if len(runtimes) != 1 {
		t.Errorf("Reconcile created %d runtimes, want 1", len(runtimes))
	}
	if runtimes[0].State != ghdomain.RuntimeStateCreated {
		t.Errorf("runtime state = %q, want created", runtimes[0].State)
	}
}

func TestRuntimeGraphProvisioner_Reconcile_NoAutoStart(t *testing.T) {
	source := testKernelSource{plugins: []KernelGamePlugin{
		makeTestPlugin("ext-a", "game", "C:/test/game.exe"),
	}}

	provisioner, rtManager, _ := newTestProvisioner(t, source)

	err := provisioner.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("Reconcile error: %v", err)
	}

	runtimes := rtManager.ListRuntimes()
	for _, rt := range runtimes {
		if rt.State != ghdomain.RuntimeStateCreated {
			t.Errorf("runtime %s state = %q, want created (no auto-start)", rt.ID, rt.State)
		}
	}
}

func TestRuntimeGraphProvisioner_Reconcile_TwoPlugins_TwoRuntimes(t *testing.T) {
	source := testKernelSource{plugins: []KernelGamePlugin{
		makeTestPlugin("ext-a", "game-a", "C:/test/a.exe"),
		makeTestPlugin("ext-b", "game-b", "C:/test/b.exe"),
	}}

	provisioner, rtManager, _ := newTestProvisioner(t, source)

	err := provisioner.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("Reconcile error: %v", err)
	}

	runtimes := rtManager.ListRuntimes()
	if len(runtimes) != 2 {
		t.Errorf("Reconcile created %d runtimes, want 2", len(runtimes))
	}

	if len(runtimes) == 2 && runtimes[0].ID == runtimes[1].ID {
		t.Error("two plugins should produce two distinct RuntimeIDs")
	}
}

func TestRuntimeGraphProvisioner_Reconcile_Idempotent(t *testing.T) {
	source := testKernelSource{plugins: []KernelGamePlugin{
		makeTestPlugin("ext-a", "game", "C:/test/game.exe"),
	}}

	provisioner, rtManager, _ := newTestProvisioner(t, source)

	ctx := context.Background()
	err := provisioner.Reconcile(ctx)
	if err != nil {
		t.Fatalf("first Reconcile error: %v", err)
	}

	err = provisioner.Reconcile(ctx)
	if err != nil {
		t.Fatalf("second Reconcile error: %v", err)
	}

	runtimes := rtManager.ListRuntimes()
	if len(runtimes) != 1 {
		t.Errorf("idempotent reconcile created %d runtimes, want 1", len(runtimes))
	}
}

func TestRuntimeGraphProvisioner_Reconcile_RegistersDefinition(t *testing.T) {
	source := testKernelSource{plugins: []KernelGamePlugin{
		makeTestPlugin("ext-a", "game", "C:/test/game.exe"),
	}}

	provisioner, _, supervisor := newTestProvisioner(t, source)

	err := provisioner.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("Reconcile error: %v", err)
	}

	view := service_definition.ServiceRuntimeView{
		ExtensionID: "ext-a",
		ModuleID:    "runtime-module",
		RuntimeType: "service",
		EntryPoint:  "C:/test/game.exe",
	}

	defID := view.ToDefinitionID()
	if !supervisor.HasDefinition(defID) {
		t.Errorf("expected supervisor to have definition %q", defID)
	}
}

func TestRuntimeGraphProvisioner_Reconcile_NoEntryPoint_FailClosed(t *testing.T) {
	plugin := KernelGamePlugin{
		Extension: domain.ExtensionDefinition{
			ID:   domain.ExtensionID("ext-a"),
			Name: domain.LocalizedText{Default: "Test"},
			Modules: []domain.ModuleDefinition{{
				ID:          domain.ModuleID("runtime-module"),
				ExtensionID: domain.ExtensionID("ext-a"),
				Type:        domain.ModuleTypeService,
				Runtime:     &domain.RuntimeDefinition{Type: domain.RuntimeTypeService},
			}},
		},
		Contribution: domain.ContributionDefinition{
			ID:          domain.ContributionID("game"),
			ModuleID:    domain.ModuleID("game"),
			ExtensionID: domain.ExtensionID("ext-a"),
			Kind:        domain.ContributionKindGamePlugin,
			Definition:  map[string]any{"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "runtime-module", "network": map[string]any{"mode": "none"}},
		},
	}

	source := testKernelSource{plugins: []KernelGamePlugin{plugin}}
	provisioner, rtManager, _ := newTestProvisioner(t, source)

	err := provisioner.Reconcile(context.Background())
	if err == nil {
		t.Error("expected error for plugin with no entry point")
	}

	runtimes := rtManager.ListRuntimes()
	if len(runtimes) != 0 {
		t.Errorf("failed reconcile should not create runtime, got %d", len(runtimes))
	}
}

func TestResolveGamePluginEntry_UsesModuleRootAndRejectsEscape(t *testing.T) {
	bundle := t.TempDir()
	got, err := resolveGamePluginEntry(bundle, "runtime-module", "bin/game")
	if err != nil {
		t.Fatalf("resolveGamePluginEntry error: %v", err)
	}
	want := filepath.Join(bundle, "modules", "runtime-module", "bin", "game")
	if got != want {
		t.Fatalf("resolved path = %q, want %q", got, want)
	}
	if _, err := resolveGamePluginEntry(bundle, "runtime-module", "../escape"); err == nil {
		t.Fatal("expected traversal entry point to be rejected")
	}
	if _, err := resolveGamePluginEntry(bundle, "../runtime", "bin/game"); err == nil {
		t.Fatal("expected invalid module id to be rejected")
	}
}

func TestBytesToMiBCeil(t *testing.T) {
	const mib = int64(1024 * 1024)
	cases := map[int64]int64{
		0:       0,
		1:       1,
		mib:     1,
		mib + 1: 2,
	}
	for in, want := range cases {
		if got := bytesToMiBCeil(in); got != want {
			t.Fatalf("bytesToMiBCeil(%d)=%d, want %d", in, got, want)
		}
	}
}
