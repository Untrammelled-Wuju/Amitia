// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrant

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

type specRuntimeHost struct {
	descriptor platform.RuntimeDescriptor
	paths      util.RuntimePaths
}

func (h specRuntimeHost) Descriptor() platform.RuntimeDescriptor { return h.descriptor }

func (h specRuntimeHost) Capabilities() *runtimehost.HostCapabilities {
	return runtimehost.NewTestCapabilitiesForTest(map[runtimehost.HostCapabilityID]runtimehost.CapabilitySupport{
		runtimehost.CapProcessSpawn:         runtimehost.SupportSupported,
		runtimehost.CapFilesystemLocal:      runtimehost.SupportSupported,
		runtimehost.CapFilesystemExecutable: runtimehost.SupportSupported,
	})
}

func (h specRuntimeHost) Paths() util.RuntimePaths                 { return h.paths }
func (h specRuntimeHost) Processes() runtimehost.ProcessSupervisor { return nil }
func (h specRuntimeHost) RuntimeInstanceID() string                { return "qa-qdrant-spec" }

func TestBuildQdrantProcessSpecUsesAndroidProfileAndCapturesOutput(t *testing.T) {
	root := t.TempDir()
	binaryPath := filepath.Join(root, "runtime", "qdrant", "bin", "qdrant")
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	previous := config.AppCfg
	t.Cleanup(func() {
		config.AppCfg = previous
	})
	config.AppCfg = &config.Config{
		Providers: config.ProvidersConfig{
			VectorStore: config.VectorStoreProviderConfig{
				Enabled:  true,
				Provider: "builtin.qdrant-process",
				Qdrant: config.QdrantConfig{
					Host:            "127.0.0.1",
					Port:            19178,
					BinaryPath:      binaryPath,
					ConfigDir:       "providers/qdrant",
					DataDir:         filepath.Join(root, "data", "providers", "qdrant"),
					SnapshotsDir:    filepath.Join(root, "data", "providers", "qdrant", "snapshots"),
					ResourceProfile: "auto",
				},
			},
		},
	}
	host := specRuntimeHost{
		descriptor: platform.RuntimeDescriptor{
			Host:         platform.HostPlatformAndroid,
			Kind:         platform.RuntimeKindProot,
			Guest:        platform.GuestPlatformLinux,
			Architecture: "arm64",
		},
		paths: util.RuntimePaths{
			Root:      root,
			ConfigDir: filepath.Join(root, "config"),
			DataDir:   filepath.Join(root, "data"),
		},
	}

	spec, err := BuildQdrantProcessSpec(host)
	if err != nil {
		t.Fatalf("BuildQdrantProcessSpec: %v", err)
	}
	configBytes, err := os.ReadFile(filepath.Join(root, "config", "providers", "qdrant", "config.yaml"))
	if err != nil {
		t.Fatalf("read rendered qdrant config: %v", err)
	}
	rendered := string(configBytes)
	for _, expected := range []string{"max_workers: 2", "io_uring: disabled", "hnsw_index:"} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("rendered config missing %q:\n%s", expected, rendered)
		}
	}
	if spec.OnStdout == nil {
		t.Error("stdout capture is not configured")
	}
	if spec.OnStderr == nil {
		t.Error("stderr capture is not configured")
	}
}
