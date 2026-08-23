// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigTemplatesUseProviderLayout(t *testing.T) {
	t.Setenv("AMITIA_JWT_SECRET", testJWTSecret)
	dirs := []string{
		".",
		"../../config",
		"../../desktop/resources/config-template",
	}

	for _, dir := range dirs {
		configFile := filepath.Join(dir, "config.yml")
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			configFile = filepath.Join(dir, "config.yaml")
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				t.Logf("Skipping non-existent dir: %s", dir)
				continue
			}
		}

		t.Run(filepath.Base(dir), func(t *testing.T) {
			cfg, err := loadConfig(dir)
			if err != nil {
				t.Fatalf("Failed to load config from %s: %v", dir, err)
			}

			if !cfg.Providers.ScriptRuntime.Enabled {
				t.Error("providers.scriptRuntime.enabled should be true")
			}
			if cfg.Providers.ScriptRuntime.Provider == "" {
				t.Error("providers.scriptRuntime.provider should be set")
			}
			if !cfg.Providers.VectorStore.Enabled {
				t.Error("providers.vectorStore.enabled should be true")
			}
			if cfg.Providers.VectorStore.Provider == "" {
				t.Error("providers.vectorStore.provider should be set")
			}
			if !cfg.Providers.GraphStore.Enabled {
				t.Error("providers.graphStore.enabled should be true")
			}
			if cfg.Providers.GraphStore.Provider == "" {
				t.Error("providers.graphStore.provider should be set")
			}
			if cfg.Components.Sidecars.Wechat.Port == 0 {
				t.Error("components.sidecars.wechat.port should be set")
			}
			if cfg.Components.Sidecars.QQ.Port == 0 {
				t.Error("components.sidecars.qq.port should be set")
			}
			if !cfg.DesktopPetRuntime.Enabled {
				t.Error("desktopPetRuntime.enabled should be true in templates")
			}

			data, err := os.ReadFile(configFile)
			if err != nil {
				t.Fatal(err)
			}
			content := string(data)

			if strings.Contains(content, "\nqdrant:\n") {
				t.Error("Template should not have top-level qdrant section")
			}
			if strings.Contains(content, "\nsurrealdb:\n") {
				t.Error("Template should not have top-level surrealdb section")
			}
		})
	}
}

func TestConfigTemplatesDoNotHardcodeHostRuntime(t *testing.T) {
	paths := []string{
		"config.yml",
		"../../config/config.yml",
		"../../desktop/resources/config-template/config.yaml",
	}

	forbiddenPatterns := []string{
		"android-proot",
		"hostPlatform",
		"runtimeKind",
		"guestPlatform",
		"/opt/amitia",
		"/var/lib/amitia",
		"node.exe",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Skipf("Cannot read %s: %v", path, err)
				return
			}
			content := string(data)

			for _, pattern := range forbiddenPatterns {
				if strings.Contains(content, pattern) {
					t.Errorf("Template %s should not contain %q", path, pattern)
				}
			}
		})
	}
}

func TestConfigTemplatePreservesExistingProviderValues(t *testing.T) {
	t.Setenv("AMITIA_JWT_SECRET", testJWTSecret)
	tests := []struct {
		name       string
		dir        string
		expectPort int
		expectVDim int
		expectNS   string
		expectDB   string
	}{
		{
			name:       "backend/config",
			dir:        ".",
			expectPort: 9178,
			expectVDim: 1536,
			expectNS:   "uai",
			expectDB:   "memory_graph",
		},
		{
			name:       "config",
			dir:        "../../config",
			expectPort: 19178,
			expectVDim: 1536,
			expectNS:   "uai",
			expectDB:   "memory_graph",
		},
		{
			name:       "desktop-template",
			dir:        "../../desktop/resources/config-template",
			expectPort: 19178,
			expectVDim: 1536,
			expectNS:   "uai",
			expectDB:   "memory_graph",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := loadConfig(tt.dir)
			if err != nil {
				t.Fatalf("Failed to load config from %s: %v", tt.dir, err)
			}

			if cfg.Providers.VectorStore.Qdrant.Port != tt.expectPort {
				t.Errorf("Qdrant port = %d, want %d", cfg.Providers.VectorStore.Qdrant.Port, tt.expectPort)
			}
			if cfg.Providers.VectorStore.Qdrant.VectorDim != tt.expectVDim {
				t.Errorf("Qdrant vectorDim = %d, want %d", cfg.Providers.VectorStore.Qdrant.VectorDim, tt.expectVDim)
			}
			if cfg.Providers.GraphStore.SurrealDB.Namespace != tt.expectNS {
				t.Errorf("SurrealDB namespace = %q, want %q", cfg.Providers.GraphStore.SurrealDB.Namespace, tt.expectNS)
			}
			if cfg.Providers.GraphStore.SurrealDB.Database != tt.expectDB {
				t.Errorf("SurrealDB database = %q, want %q", cfg.Providers.GraphStore.SurrealDB.Database, tt.expectDB)
			}
			if cfg.Providers.GraphStore.SurrealDB.DataPath != "data/graph.db" {
				t.Errorf("SurrealDB dataPath = %q, want data/graph.db", cfg.Providers.GraphStore.SurrealDB.DataPath)
			}
		})
	}
}
