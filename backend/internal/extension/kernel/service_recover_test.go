package kernel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
)

type recoveryNodeResolver struct{ path string }

func (r recoveryNodeResolver) Resolve(context.Context) (nodeenv.Environment, error) {
	return nodeenv.Environment{NodeBinary: r.path}, nil
}

func TestRecoverRestoresServiceDefinitionsBeforeStarting(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "valid", true: "missing-entry"}[missing], func(t *testing.T) {
			ctx := t.Context()
			extID := "com.amitia.test/recover-service"
			container, fake := setupRecoverTestContainer(t, ctx, extID)
			defer container.Close()
			supervisor := runtime_supervisor.NewDefaultSupervisor()
			container.RuntimeSupervisor = supervisor
			if err := supervisor.RegisterFactory(runtime_supervisor.NewFakeFactory(domain.RuntimeTypeService, fake)); err != nil {
				t.Fatal(err)
			}
			version := domain.SemanticVersion{Major: 1}
			module := domain.ModuleDefinition{ID: "main", ExtensionID: domain.ExtensionID(extID), Type: domain.ModuleTypeService,
				Name:    domain.LocalizedText{Default: "main"},
				Runtime: &domain.RuntimeDefinition{Type: domain.RuntimeTypeService, EntryPoint: "launcher.mjs"}}
			def := domain.ExtensionDefinition{ID: domain.ExtensionID(extID), Version: version,
				Name: domain.LocalizedText{Default: "Recovery Service"}, ManifestVersion: 1,
				Modules:   []domain.ModuleDefinition{module},
				Publisher: domain.PublisherReference{PublisherID: "amitia", TrustLevel: "official"}}
			if err := container.DefinitionRepository.PutExtension(ctx, def); err != nil {
				t.Fatal(err)
			}
			if err := container.ModuleRepository.PutModule(ctx, module); err != nil {
				t.Fatal(err)
			}
			root := filepath.Join(container.ExtRoot, "installed", "com.amitia.test__recover-service", "1.0.0", "bundle")
			if err := os.MkdirAll(filepath.Join(root, "modules", "main"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte(`{}`), 0600); err != nil {
				t.Fatal(err)
			}
			if !missing {
				if err := os.WriteFile(filepath.Join(root, "modules", "main", "launcher.mjs"), []byte("export {};"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			node := filepath.Join(t.TempDir(), "managed-node")
			if err := os.WriteFile(node, []byte("managed runtime fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			container.NodeEnvironmentResolver = recoveryNodeResolver{node}
			err := container.Recover(ctx)
			serviceID := string(runtime_supervisor.BuildRuntimeDefinitionID(extID, "main", domain.RuntimeTypeService))
			if missing {
				if err == nil || !strings.Contains(err.Error(), "service entrypoint unavailable") {
					t.Fatalf("missing entry accepted: %v", err)
				}
				inst, getErr := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
				if getErr != nil || inst.EnablementState != domain.EnablementRequiresRecovery {
					t.Fatalf("failure state: %#v %v", inst, getErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			service, err := container.ServiceDefinitions.GetServiceDefinition(serviceID)
			if err != nil || service.ExtensionID != extID || service.ModuleID != "main" {
				t.Fatalf("service definition: %#v %v", service, err)
			}
		})
	}
}
