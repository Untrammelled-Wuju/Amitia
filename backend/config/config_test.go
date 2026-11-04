// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProviderConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	yaml := ""
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if !cfg.Providers.ScriptRuntime.Enabled {
		t.Error("ScriptRuntime should be enabled by default")
	}
	if cfg.Providers.ScriptRuntime.Required {
		t.Error("ScriptRuntime should not be required by default")
	}
	if cfg.Providers.ScriptRuntime.Provider != "builtin.node-process" {
		t.Errorf("ScriptRuntime provider should be builtin.node-process, got %q", cfg.Providers.ScriptRuntime.Provider)
	}

	if !cfg.Providers.VectorStore.Enabled {
		t.Error("VectorStore should be enabled by default")
	}
	if cfg.Providers.VectorStore.Provider != "builtin.qdrant-process" {
		t.Errorf("VectorStore provider should be builtin.qdrant-process, got %q", cfg.Providers.VectorStore.Provider)
	}

	if !cfg.Providers.GraphStore.Enabled {
		t.Error("GraphStore should be enabled by default")
	}
	if cfg.Providers.GraphStore.Provider != "builtin.surrealdb-process" {
		t.Errorf("GraphStore provider should be builtin.surrealdb-process, got %q", cfg.Providers.GraphStore.Provider)
	}

	if cfg.Providers.ScriptRuntime.Node.BinaryPath != "" {
		t.Error("Node binaryPath should be empty by default")
	}

	if !cfg.Components.TaskHost.Enabled {
		t.Error("TaskHost should be enabled by default")
	}
}

func TestLoadCanonicalProviderConfig(t *testing.T) {
	dir := t.TempDir()
	yaml := "providers:\n  scriptRuntime:\n    enabled: true\n    required: true\n    provider: \"custom.node-runtime\"\n    node:\n      binaryPath: \"/usr/bin/node\"\n      npmPath: \"/usr/bin/npm\"\n      npxPath: \"/usr/bin/npx\"\n      workDir: \"/tmp/node\"\n  vectorStore:\n    enabled: true\n    required: false\n    qdrant:\n      host: \"10.0.0.1\"\n      port: 6333\n      collectionName: \"embeddings\"\n      vectorDim: 768\n      limit: 20\n      collections:\n        test_col:\n          name: \"test_col\"\n          vectorDim: 768\n  graphStore:\n    enabled: false\n    required: false\n    surrealdb:\n      host: \"10.0.0.2\"\n      port: 9000\n      namespace: \"test\"\n      database: \"test_db\"\n      username: \"admin\"\n      password: \"secret\"\n      dataPath: \"/var/data/graph.db\"\n"
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if !cfg.Providers.ScriptRuntime.Enabled {
		t.Error("ScriptRuntime should be enabled")
	}
	if !cfg.Providers.ScriptRuntime.Required {
		t.Error("ScriptRuntime should be required")
	}
	if cfg.Providers.ScriptRuntime.Provider != "custom.node-runtime" {
		t.Errorf("ScriptRuntime provider = %q, want custom.node-runtime", cfg.Providers.ScriptRuntime.Provider)
	}
	if cfg.Providers.ScriptRuntime.Node.BinaryPath != "/usr/bin/node" {
		t.Errorf("Node binaryPath = %q", cfg.Providers.ScriptRuntime.Node.BinaryPath)
	}

	if cfg.Providers.VectorStore.Qdrant.Host != "10.0.0.1" {
		t.Errorf("Qdrant host = %q", cfg.Providers.VectorStore.Qdrant.Host)
	}
	if cfg.Providers.VectorStore.Qdrant.Port != 6333 {
		t.Errorf("Qdrant port = %d", cfg.Providers.VectorStore.Qdrant.Port)
	}
	if cfg.Providers.VectorStore.Qdrant.VectorDim != 768 {
		t.Errorf("Qdrant vectorDim = %d", cfg.Providers.VectorStore.Qdrant.VectorDim)
	}

	if cfg.Providers.GraphStore.Enabled {
		t.Error("GraphStore should be disabled")
	}
	if cfg.Providers.GraphStore.SurrealDB.Host != "10.0.0.2" {
		t.Errorf("SurrealDB host = %q", cfg.Providers.GraphStore.SurrealDB.Host)
	}
}

func TestRejectRemovedTopLevelProviderConfig(t *testing.T) {
	for _, key := range []string{"qdrant", "surrealdb"} {
		t.Run(key, func(t *testing.T) {
			dir := t.TempDir()
			configFile := filepath.Join(dir, "config.yml")
			if err := os.WriteFile(configFile, []byte(key+":\n  host: \"127.0.0.1\"\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := loadConfig(dir); err == nil {
				t.Fatalf("top-level %s config must be rejected", key)
			}
		})
	}
}

func TestCanonicalProviderConfigLoads(t *testing.T) {
	dir := t.TempDir()
	yaml := "providers:\n  vectorStore:\n    qdrant:\n      host: \"127.0.0.1\"\n      port: 6333\n      vectorDim: 768\n      limit: 10\n  graphStore:\n    surrealdb:\n      host: \"127.0.0.1\"\n      port: 8000\n      namespace: \"current_ns\"\n"
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}
	if cfg.Providers.VectorStore.Qdrant.Host != "127.0.0.1" || cfg.Providers.VectorStore.Qdrant.Port != 6333 {
		t.Fatalf("Qdrant config mismatch: %+v", cfg.Providers.VectorStore.Qdrant)
	}
	if cfg.Providers.GraphStore.SurrealDB.Host != "127.0.0.1" || cfg.Providers.GraphStore.SurrealDB.Namespace != "current_ns" {
		t.Fatalf("SurrealDB config mismatch: %+v", cfg.Providers.GraphStore.SurrealDB)
	}
}

func TestConfigKeepsCanonicalProviderStructure(t *testing.T) {
	v := reflect.TypeOf(Config{})
	for i := 0; i < v.NumField(); i++ {
		fieldName := v.Field(i).Name
		if fieldName == "Qdrant" {
			t.Error("Config should not expose top-level Qdrant field")
		}
		if fieldName == "Surreal" {
			t.Error("Config should not expose top-level Surreal field")
		}
	}
}

func TestProviderIDNormalization(t *testing.T) {
	dir := t.TempDir()
	yaml := "providers:\n  scriptRuntime:\n    enabled: true\n    required: false\n    provider: \"builtin.node-process\"\n"
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if cfg.Providers.ScriptRuntime.Provider != "builtin.node-process" {
		t.Errorf("Provider ID should be normalized, got %q", cfg.Providers.ScriptRuntime.Provider)
	}
}

func TestRejectInvalidProviderID(t *testing.T) {
	invalidIDs := []string{
		"has space",
		"slash/test",
		"colon:test",
		"ab",
		".starts-with-dot",
		"-starts-with-dash",
	}

	for _, id := range invalidIDs {
		dir := t.TempDir()
		yaml := "providers:\n  scriptRuntime:\n    enabled: true\n    required: false\n    provider: \"" + id + "\"\n"
		configFile := filepath.Join(dir, "config.yml")
		if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := loadConfig(dir)
		if err == nil {
			t.Errorf("Should reject invalid provider ID %q", id)
		}
	}
}

func TestRejectRequiredDisabledProvider(t *testing.T) {
	dir := t.TempDir()
	yaml := "providers:\n  scriptRuntime:\n    enabled: false\n    required: true\n"
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := loadConfig(dir)
	if err == nil {
		t.Error("Should reject required=true and enabled=false for scriptRuntime")
	}

	dir2 := t.TempDir()
	yaml2 := "providers:\n  vectorStore:\n    enabled: false\n    required: true\n"
	configFile2 := filepath.Join(dir2, "config.yml")
	if err := os.WriteFile(configFile2, []byte(yaml2), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = loadConfig(dir2)
	if err == nil {
		t.Error("Should reject required=true and enabled=false for vectorStore")
	}
}

func TestProviderEnvironmentOverrides(t *testing.T) {
	dir := t.TempDir()
	yaml := ""
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AMITIA_QDRANT_HOST", "10.0.0.5")
	t.Setenv("AMITIA_QDRANT_PORT", "5555")
	t.Setenv("AMITIA_SURREAL_HOST", "10.0.0.6")
	t.Setenv("AMITIA_SURREAL_PORT", "6666")
	t.Setenv("AMITIA_NODE_BIN", "/env/path/node")
	defer func() {
		os.Unsetenv("AMITIA_QDRANT_HOST")
		os.Unsetenv("AMITIA_QDRANT_PORT")
		os.Unsetenv("AMITIA_SURREAL_HOST")
		os.Unsetenv("AMITIA_SURREAL_PORT")
		os.Unsetenv("AMITIA_NODE_BIN")
	}()

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if cfg.Providers.VectorStore.Qdrant.Host != "10.0.0.5" {
		t.Errorf("Qdrant host via env = %q", cfg.Providers.VectorStore.Qdrant.Host)
	}
	if cfg.Providers.VectorStore.Qdrant.Port != 5555 {
		t.Errorf("Qdrant port via env = %d", cfg.Providers.VectorStore.Qdrant.Port)
	}
	if cfg.Providers.GraphStore.SurrealDB.Host != "10.0.0.6" {
		t.Errorf("SurrealDB host via env = %q", cfg.Providers.GraphStore.SurrealDB.Host)
	}
	if cfg.Providers.GraphStore.SurrealDB.Port != 6666 {
		t.Errorf("SurrealDB port via env = %d", cfg.Providers.GraphStore.SurrealDB.Port)
	}
	if cfg.Providers.ScriptRuntime.Node.BinaryPath != "/env/path/node" {
		t.Errorf("Node binary via env = %q", cfg.Providers.ScriptRuntime.Node.BinaryPath)
	}
}

func TestProviderEnvironmentFalseOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	yaml := ""
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AMITIA_QDRANT_ENABLED", "false")
	t.Setenv("AMITIA_SURREAL_ENABLED", "false")
	t.Setenv("AMITIA_TASK_HOST_ENABLED", "false")
	defer func() {
		os.Unsetenv("AMITIA_QDRANT_ENABLED")
		os.Unsetenv("AMITIA_SURREAL_ENABLED")
		os.Unsetenv("AMITIA_TASK_HOST_ENABLED")
	}()

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if cfg.Providers.VectorStore.Qdrant.Enabled {
		t.Error("Qdrant should be disabled via env false")
	}
	if cfg.Providers.GraphStore.SurrealDB.Enabled {
		t.Error("SurrealDB should be disabled via env false")
	}
	if cfg.Components.TaskHost.Enabled {
		t.Error("TaskHost should be disabled via env false")
	}
}

func TestProviderBinaryEnvironment(t *testing.T) {
	dir := t.TempDir()
	yaml := ""
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AMITIA_QDRANT_BINARY", "/new/qdrant")
	t.Setenv("AMITIA_QDRANT_DATA_DIR", "/new/qdrant-data")
	t.Setenv("AMITIA_SURREAL_BINARY", "/new/surreal")
	t.Setenv("AMITIA_SURREAL_DATA_PATH", "/new/surreal-data")
	defer func() {
		os.Unsetenv("AMITIA_QDRANT_BINARY")
		os.Unsetenv("AMITIA_QDRANT_DATA_DIR")
		os.Unsetenv("AMITIA_SURREAL_BINARY")
		os.Unsetenv("AMITIA_SURREAL_DATA_PATH")
	}()

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if cfg.Providers.VectorStore.Qdrant.BinaryPath != "/new/qdrant" {
		t.Errorf("Qdrant binary should use new env, got %q", cfg.Providers.VectorStore.Qdrant.BinaryPath)
	}
	if cfg.Providers.VectorStore.Qdrant.DataDir != "/new/qdrant-data" {
		t.Errorf("Qdrant dataDir should use new env, got %q", cfg.Providers.VectorStore.Qdrant.DataDir)
	}
	if cfg.Providers.GraphStore.SurrealDB.BinaryPath != "/new/surreal" {
		t.Errorf("SurrealDB binary should use new env, got %q", cfg.Providers.GraphStore.SurrealDB.BinaryPath)
	}
	if cfg.Providers.GraphStore.SurrealDB.DataPath != "/new/surreal-data" {
		t.Errorf("SurrealDB dataPath should use new env, got %q", cfg.Providers.GraphStore.SurrealDB.DataPath)
	}
}

func TestNodeBinaryEnvironment(t *testing.T) {
	dir := t.TempDir()
	yaml := ""
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AMITIA_NODE_BIN", "/new/node")
	t.Setenv("AMITIA_NPM_BIN", "/new/npm")
	t.Setenv("AMITIA_NPX_BIN", "/new/npx")
	defer func() {
		os.Unsetenv("AMITIA_NODE_BIN")
		os.Unsetenv("AMITIA_NPM_BIN")
		os.Unsetenv("AMITIA_NPX_BIN")
	}()

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if cfg.Providers.ScriptRuntime.Node.BinaryPath != "/new/node" {
		t.Errorf("Node binary should use env, got %q", cfg.Providers.ScriptRuntime.Node.BinaryPath)
	}
	if cfg.Providers.ScriptRuntime.Node.NPMPath != "/new/npm" {
		t.Errorf("Node npmPath should use env, got %q", cfg.Providers.ScriptRuntime.Node.NPMPath)
	}
}

func TestComponentEntryPathValidation(t *testing.T) {
	dir := t.TempDir()
	yaml := "components:\n  taskHost:\n    enabled: true\n    entryUri: \"amitia://runtime/polyglot/launcher.mjs\"\n    workUri: \"amitia://runtime/polyglot\"\n"
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig should accept valid URI: %v", err)
	}
	if cfg.Components.TaskHost.EntryURI != "amitia://runtime/polyglot/launcher.mjs" {
		t.Errorf("TaskHost entryUri = %q", cfg.Components.TaskHost.EntryURI)
	}

	dir2 := t.TempDir()
	yaml2 := "components:\n  taskHost:\n    entryUri: \"file:///etc/passwd\"\n"
	configFile2 := filepath.Join(dir2, "config.yml")
	if err := os.WriteFile(configFile2, []byte(yaml2), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = loadConfig(dir2)
	if err == nil {
		t.Error("Should reject file:// URI")
	}

	dir3 := t.TempDir()
	yaml3 := "components:\n  taskHost:\n    entryUri: \"\"\n"
	configFile3 := filepath.Join(dir3, "config.yml")
	if err := os.WriteFile(configFile3, []byte(yaml3), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = loadConfig(dir3)
	if err != nil {
		t.Errorf("Empty entryUri should be allowed: %v", err)
	}
}

func TestProviderConfigDoesNotRequireBinaries(t *testing.T) {
	dir := t.TempDir()
	yaml := "providers:\n  scriptRuntime:\n    node:\n      binaryPath: \"/nonexistent/node\"\n  vectorStore:\n    qdrant:\n      binaryPath: \"/nonexistent/qdrant\"\n  graphStore:\n    surrealdb:\n      binaryPath: \"/nonexistent/surreal\"\n"
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("Config load should succeed even with non-existent paths: %v", err)
	}
	if cfg.Providers.ScriptRuntime.Node.BinaryPath != "/nonexistent/node" {
		t.Error("Binary path should be preserved")
	}
}

func TestConfigDoesNotDependOnRuntimeDescriptor(t *testing.T) {
	dir := t.TempDir()
	yaml := "providers:\n  scriptRuntime:\n    enabled: true\n  vectorStore:\n    enabled: true\n  graphStore:\n    enabled: true\n"
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AMITIA_RUNTIME_MODE", "android-proot")
	cfg1, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig with android-proot failed: %v", err)
	}

	os.Unsetenv("AMITIA_RUNTIME_MODE")
	cfg2, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig without runtime mode failed: %v", err)
	}

	if !reflect.DeepEqual(cfg1.Providers, cfg2.Providers) {
		t.Errorf("Providers config should not depend on AMITIA_RUNTIME_MODE")
	}
}

func TestServerEnvironmentBinding(t *testing.T) {
	dir := t.TempDir()
	yaml := ""
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("default server.host = %q, want 127.0.0.1", cfg.Server.Host)
	}
	if cfg.Server.Port != 0 {
		t.Errorf("default server.port = %d, want 0", cfg.Server.Port)
	}
}

func TestServerHostEnvOverride(t *testing.T) {
	dir := t.TempDir()
	yaml := ""
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AMITIA_SERVER_HOST", "127.0.0.1")
	defer os.Unsetenv("AMITIA_SERVER_HOST")

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("server.host via env = %q, want 127.0.0.1", cfg.Server.Host)
	}
}

func TestServerPortEnvOverride(t *testing.T) {
	dir := t.TempDir()
	yaml := ""
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AMITIA_SERVER_PORT", "18899")
	defer os.Unsetenv("AMITIA_SERVER_PORT")

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if cfg.Server.Port != 18899 {
		t.Errorf("server.port via env = %d, want 18899", cfg.Server.Port)
	}
}

func TestServerEnvDoesNotAffectProviderPorts(t *testing.T) {
	dir := t.TempDir()
	yaml := ""
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AMITIA_SERVER_PORT", "18899")
	defer os.Unsetenv("AMITIA_SERVER_PORT")

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if cfg.Providers.VectorStore.Qdrant.Port != 0 {
		t.Errorf("qdrant port should remain dynamic, got %d", cfg.Providers.VectorStore.Qdrant.Port)
	}
	if cfg.Providers.GraphStore.SurrealDB.Port != 0 {
		t.Errorf("surrealdb port should remain dynamic, got %d", cfg.Providers.GraphStore.SurrealDB.Port)
	}
}

func TestNoAndroidSpecificServerEnv(t *testing.T) {
	os.Unsetenv("ANDROID_SERVER_PORT")
	if val, ok := os.LookupEnv("ANDROID_SERVER_PORT"); ok {
		t.Errorf("ANDROID_SERVER_PORT should not be defined, got %q", val)
	}
}

func TestAndroidStyleRequiredProviderOverrides(t *testing.T) {
	dir := t.TempDir()
	yaml := ""
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AMITIA_SCRIPT_RUNTIME_ENABLED", "true")
	t.Setenv("AMITIA_SCRIPT_RUNTIME_REQUIRED", "true")
	t.Setenv("AMITIA_VECTOR_STORE_ENABLED", "true")
	t.Setenv("AMITIA_GRAPH_STORE_ENABLED", "false")
	t.Setenv("AMITIA_DESKTOP_PET_RUNTIME_ENABLED", "false")
	defer func() {
		os.Unsetenv("AMITIA_SCRIPT_RUNTIME_ENABLED")
		os.Unsetenv("AMITIA_SCRIPT_RUNTIME_REQUIRED")
		os.Unsetenv("AMITIA_VECTOR_STORE_ENABLED")
		os.Unsetenv("AMITIA_GRAPH_STORE_ENABLED")
		os.Unsetenv("AMITIA_DESKTOP_PET_RUNTIME_ENABLED")
	}()

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if !cfg.Providers.ScriptRuntime.Enabled {
		t.Error("ScriptRuntime should be enabled")
	}
	if !cfg.Providers.ScriptRuntime.Required {
		t.Error("ScriptRuntime should be required")
	}
	if !cfg.Providers.VectorStore.Enabled {
		t.Error("VectorStore should be enabled")
	}
	if cfg.DesktopPetRuntime.Enabled {
		t.Error("DesktopPetRuntime should be disabled")
	}
}

func TestLoadConfigUsesRuntimeAppVersion(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte("app:\n  version: \"26.2.0-beta.2\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AMITIA_VERSION", "")

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}
	if cfg.App.Version != DefaultAppVersion {
		t.Fatalf("app version = %q, want %q", cfg.App.Version, DefaultAppVersion)
	}

	t.Setenv("AMITIA_VERSION", "26.2.0-beta.5")
	cfg, err = loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig with AMITIA_VERSION failed: %v", err)
	}
	if cfg.App.Version != "26.2.0-beta.5" {
		t.Fatalf("app version = %q, want runtime override", cfg.App.Version)
	}
}

func TestExplicitPortOneIsPreserved(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte("server:\n  port: 1\nproviders:\n  vectorStore:\n    qdrant:\n      port: 1\n  graphStore:\n    surrealdb:\n      port: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}
	if cfg.Server.Port != 1 {
		t.Fatalf("server port = %d, want explicit 1", cfg.Server.Port)
	}
	if cfg.Providers.VectorStore.Qdrant.Port != 1 {
		t.Fatalf("qdrant port = %d, want explicit 1", cfg.Providers.VectorStore.Qdrant.Port)
	}
	if cfg.Providers.GraphStore.SurrealDB.Port != 1 {
		t.Fatalf("surrealdb port = %d, want explicit 1", cfg.Providers.GraphStore.SurrealDB.Port)
	}
}
