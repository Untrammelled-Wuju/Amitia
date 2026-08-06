// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package builtin

import (
	"context"
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

type testRuntimeHost struct {
	runtimehost.RuntimeHost
}

func newTestRuntimeHost() *testRuntimeHost {
	return &testRuntimeHost{}
}

func (h *testRuntimeHost) Descriptor() platform.RuntimeDescriptor {
	return platform.RuntimeDescriptor{
		Guest:        platform.GuestPlatformLinux,
		Host:         platform.HostPlatformWindows,
		Kind:         platform.RuntimeKindNativeProcess,
		Architecture: "amd64",
	}
}

func (h *testRuntimeHost) Capabilities() *runtimehost.HostCapabilities {
	return runtimehost.NewTestCapabilitiesForTest(map[runtimehost.HostCapabilityID]runtimehost.CapabilitySupport{
		runtimehost.CapProcessSpawn:         runtimehost.SupportSupported,
		runtimehost.CapProcessTreeControl:   runtimehost.SupportSupported,
		runtimehost.CapProcessRestart:       runtimehost.SupportSupported,
		runtimehost.CapFilesystemExecutable: runtimehost.SupportSupported,
		runtimehost.CapNetworkLoopback:      runtimehost.SupportSupported,
	})
}

func (h *testRuntimeHost) Paths() util.RuntimePaths {
	return util.RuntimePaths{
		Root:      "C:\\AmitiaTest",
		ConfigDir: "C:\\AmitiaTest\\config",
		DataDir:   "C:\\AmitiaTest\\data",
	}
}

func (h *testRuntimeHost) Processes() runtimehost.ProcessSupervisor { return nil }
func (h *testRuntimeHost) RuntimeInstanceID() string                { return "test-instance" }

func TestQdrantProviderFactorySetsDescriptor(t *testing.T) {
	factory := NewQdrantProviderFactory()
	ctx := runtimeorchestrator.ProviderBuildContext{
		Config: &config.Config{
			Providers: config.ProvidersConfig{
				VectorStore: config.VectorStoreProviderConfig{
					Enabled:  true,
					Required: false,
					Qdrant: config.QdrantConfig{
						Port: 19178,
					},
				},
			},
		},
		Host: newTestRuntimeHost(),
	}

	inst, err := factory.Build(ctx)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	desc := inst.Descriptor()
	if desc.ID != "provider.vector-store" {
		t.Fatalf("descriptor ID=%s, want provider.vector-store", desc.ID)
	}
	if desc.Phase != "infrastructure" {
		t.Fatalf("descriptor phase=%s, want infrastructure", desc.Phase)
	}
	if inst.Slot() != "vector-store" {
		t.Fatalf("slot=%s, want vector-store", inst.Slot())
	}
	if inst.ProviderID() != "builtin.qdrant-process" {
		t.Fatalf("provider ID=%s, want builtin.qdrant-process", inst.ProviderID())
	}
}

func TestQdrantProviderBuildInitializesResolvers(t *testing.T) {
	factory := NewQdrantProviderFactory()
	ctx := runtimeorchestrator.ProviderBuildContext{
		Config: &config.Config{
			Providers: config.ProvidersConfig{
				VectorStore: config.VectorStoreProviderConfig{
					Enabled: true,
					Qdrant:  config.QdrantConfig{Port: 19178},
				},
			},
		},
		Host: newTestRuntimeHost(),
	}

	inst, err := factory.Build(ctx)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	provider, ok := inst.(*qdrantProvider)
	if !ok {
		t.Fatalf("expected *qdrantProvider")
	}
	if provider.envResolver == nil {
		t.Error("envResolver should be initialized")
	}
	if provider.layoutResolver == nil {
		t.Error("layoutResolver should be initialized")
	}
	if provider.directoryManager == nil {
		t.Error("directoryManager should be initialized")
	}
	if provider.configRenderer == nil {
		t.Error("configRenderer should be initialized")
	}
	if provider.configWriter == nil {
		t.Error("configWriter should be initialized")
	}
}

func TestQdrantProviderStartRequiresInstall(t *testing.T) {
	factory := NewQdrantProviderFactory()
	ctx := runtimeorchestrator.ProviderBuildContext{
		Config: &config.Config{
			Providers: config.ProvidersConfig{
				VectorStore: config.VectorStoreProviderConfig{
					Enabled: true,
					Qdrant:  config.QdrantConfig{Port: 19178},
				},
			},
		},
		Host: newTestRuntimeHost(),
	}

	inst, err := factory.Build(ctx)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	err = inst.Start(context.Background())
	if err == nil {
		t.Error("expected error since qdrant is not installed in test environment")
	}
}

func TestQdrantProviderStopIsIdempotent(t *testing.T) {
	factory := NewQdrantProviderFactory()
	ctx := runtimeorchestrator.ProviderBuildContext{
		Config: &config.Config{
			Providers: config.ProvidersConfig{
				VectorStore: config.VectorStoreProviderConfig{
					Enabled: true,
					Qdrant:  config.QdrantConfig{Port: 19178},
				},
			},
		},
		Host: newTestRuntimeHost(),
	}

	inst, err := factory.Build(ctx)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	_ = inst.Stop(context.Background())
	_ = inst.Stop(context.Background())
}

func TestQdrantProviderCapabilityBeforeReady(t *testing.T) {
	factory := NewQdrantProviderFactory()
	ctx := runtimeorchestrator.ProviderBuildContext{
		Config: &config.Config{
			Providers: config.ProvidersConfig{
				VectorStore: config.VectorStoreProviderConfig{
					Enabled: true,
					Qdrant:  config.QdrantConfig{Port: 19178},
				},
			},
		},
		Host: newTestRuntimeHost(),
	}

	inst, err := factory.Build(ctx)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	cap := inst.Capability()
	if cap != nil {
		if _, ok := cap.(*qdrant.Client); !ok {
			t.Errorf("capability type unexpected: %T", cap)
		}
	}
}
