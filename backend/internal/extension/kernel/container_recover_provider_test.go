package kernel

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestRecoverRestoresDeviceProviderInstances(t *testing.T) {
	for _, state := range []string{"ready", "failed", "disabled"} {
		t.Run(state, func(t *testing.T) {
			ctx := t.Context()
			extID := domain.ExtensionID("com.amitia.test/recover-provider")
			container, fake := setupRecoverTestContainer(t, ctx, string(extID))
			defer container.Close()
			module := domain.ModuleDefinition{
				ID: "main", ExtensionID: extID, Type: domain.ModuleTypeService,
				Name:                 domain.LocalizedText{Default: "Channel"},
				Runtime:              &domain.RuntimeDefinition{Type: domain.RuntimeTypeGo},
				Placement:            domain.ModulePlacementDevice,
				Provider:             &domain.ProviderMetadata{ID: "channel", Metadata: map[string]any{"channelId": "test"}},
				ProvidedCapabilities: []domain.ProvidedCapability{{ID: "channel.provider", Version: "1.0.0"}},
			}
			definition := domain.ExtensionDefinition{
				ID: extID, Version: domain.SemanticVersion{Major: 1}, ManifestVersion: 1,
				Name: domain.LocalizedText{Default: "Channel"}, Modules: []domain.ModuleDefinition{module},
			}
			if err := container.DefinitionRepository.PutExtension(ctx, definition); err != nil {
				t.Fatal(err)
			}
			if err := container.ModuleRepository.PutModule(ctx, module); err != nil {
				t.Fatal(err)
			}
			container.ProviderInstanceReconciler = capability.NewProviderInstanceReconciler(
				container.ProviderLifecycle, container.CapabilityProviders,
				runtimeidentity.Identity{SpaceID: "space", DeviceID: "device", RuntimeID: "runtime"})
			if state == "failed" {
				fake.SetStartErr(runtime_supervisor.ErrFakeStart)
			}
			if state == "disabled" {
				installation, err := container.InstallationRepository.GetInstallation(ctx, extID)
				if err != nil {
					t.Fatal(err)
				}
				installation.EnablementState = domain.EnablementDisabled
				if err := container.InstallationRepository.PutInstallation(ctx, installation); err != nil {
					t.Fatal(err)
				}
			}
			for attempt := 0; attempt < 2; attempt++ {
				err := container.Recover(ctx)
				if (err != nil) != (state == "failed") {
					t.Fatalf("unexpected recovery result: %v", err)
				}
				instances := container.CapabilityProviders.ListInstancesByCapability("channel.provider")
				if state != "ready" {
					if len(instances) != 0 {
						t.Fatalf("inactive runtime registered providers: %+v", instances)
					}
					continue
				}
				if len(instances) != 1 {
					t.Fatalf("expected one provider after recovery %d, got %d", attempt, len(instances))
				}
				instance := instances[0]
				if !instance.IsExecutable() || instance.SpaceID != "space" || instance.DeviceID != "device" || instance.RuntimeID != "runtime" || instance.RuntimeInstanceID == "" {
					t.Fatalf("provider runtime binding missing: %+v", instance)
				}
				if _, err := container.RuntimeSupervisor.GetInstance(ctx, instance.RuntimeInstanceID); err != nil {
					t.Fatalf("provider references unavailable runtime: %v", err)
				}
			}
		})
	}
}
