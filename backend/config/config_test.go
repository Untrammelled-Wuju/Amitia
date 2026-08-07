// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const testJWTSecret = "Test-JWT-Secret-Key-1234567890-Xyz"

func TestProviderConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\n"
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

	if !cfg.Components.PluginHost.Enabled {
		t.Error("PluginHost should be enabled by default")
	}
	if !cfg.Components.TaskHost.Enabled {
		t.Error("TaskHost should be enabled by default")
	}
	if !cfg.Components.Sidecars.Wechat.Enabled {
		t.Error("Wechat sidecar should be enabled by default")
	}
	if !cfg.Components.Sidecars.QQ.Enabled {
		t.Error("QQ sidecar should be enabled by default")
	}
}

func TestLoadCanonicalProviderConfig(t *testing.T) {
	dir := t.TempDir()
	yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\nproviders:\n  scriptRuntime:\n    enabled: true\n    required: true\n    provider: \"custom.node-runtime\"\n    node:\n      binaryPath: \"/usr/bin/node\"\n      npmPath: \"/usr/bin/npm\"\n      npxPath: \"/usr/bin/npx\"\n      workDir: \"/tmp/node\"\n  vectorStore:\n    enabled: true\n    required: false\n    qdrant:\n      host: \"10.0.0.1\"\n      port: 6333\n      collectionName: \"embeddings\"\n      vectorDim: 768\n      limit: 20\n      collections:\n        test_col:\n          name: \"test_col\"\n          vectorDim: 768\n  graphStore:\n    enabled: false\n    required: false\n    surrealdb:\n      host: \"10.0.0.2\"\n      port: 9000\n      namespace: \"test\"\n      database: \"test_db\"\n      username: \"admin\"\n      password: \"secret\"\n      dataPath: \"/var/data/graph.db\"\n"
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

func TestLoadLegacyQdrantConfig(t *testing.T) {
	dir := t.TempDir()
	yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\nqdrant:\n  host: \"127.0.0.1\"\n  port: 9999\n  collectionName: \"legacy\"\n  vectorDim: 512\n  limit: 5\n"
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if cfg.Providers.VectorStore.Qdrant.Host != "127.0.0.1" {
		t.Errorf("Qdrant host = %q", cfg.Providers.VectorStore.Qdrant.Host)
	}
	if cfg.Providers.VectorStore.Qdrant.Port != 9999 {
		t.Errorf("Qdrant port = %d, want 9999", cfg.Providers.VectorStore.Qdrant.Port)
	}
	if cfg.Providers.VectorStore.Qdrant.VectorDim != 512 {
		t.Errorf("Qdrant vectorDim = %d, want 512", cfg.Providers.VectorStore.Qdrant.VectorDim)
	}
}

func TestLoadLegacySurrealConfig(t *testing.T) {
	dir := t.TempDir()
	yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\nsurrealdb:\n  host: \"127.0.0.1\"\n  port: 8888\n  namespace: \"legacy_ns\"\n  database: \"legacy_db\"\n  username: \"user\"\n  password: \"pass\"\n  dataPath: \"/tmp/legacy.db\"\n"
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if cfg.Providers.GraphStore.SurrealDB.Host != "127.0.0.1" {
		t.Errorf("SurrealDB host = %q", cfg.Providers.GraphStore.SurrealDB.Host)
	}
	if cfg.Providers.GraphStore.SurrealDB.Port != 8888 {
		t.Errorf("SurrealDB port = %d, want 8888", cfg.Providers.GraphStore.SurrealDB.Port)
	}
	if cfg.Providers.GraphStore.SurrealDB.Namespace != "legacy_ns" {
		t.Errorf("SurrealDB namespace = %q", cfg.Providers.GraphStore.SurrealDB.Namespace)
	}
	if cfg.Providers.GraphStore.SurrealDB.Password != "pass" {
		t.Errorf("SurrealDB password = %q", cfg.Providers.GraphStore.SurrealDB.Password)
	}
}

func TestCanonicalProviderConfigOverridesLegacyConfig(t *testing.T) {
	dir := t.TempDir()
	yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\nqdrant:\n  host: \"10.0.0.9\"\n  port: 1111\n  vectorDim: 999\n  limit: 1\nsurrealdb:\n  host: \"10.0.0.9\"\n  port: 1111\n  namespace: \"legacy_ns\"\nproviders:\n  vectorStore:\n    qdrant:\n      host: \"127.0.0.1\"\n      port: 6333\n      vectorDim: 768\n      limit: 10\n  graphStore:\n    surrealdb:\n      host: \"127.0.0.1\"\n      port: 8000\n      namespace: \"new_ns\"\n"
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if cfg.Providers.VectorStore.Qdrant.Host != "127.0.0.1" {
		t.Errorf("Qdrant host should use canonical config, got %q", cfg.Providers.VectorStore.Qdrant.Host)
	}
	if cfg.Providers.VectorStore.Qdrant.Port != 6333 {
		t.Errorf("Qdrant port should use canonical config, got %d", cfg.Providers.VectorStore.Qdrant.Port)
	}

	if cfg.Providers.GraphStore.SurrealDB.Host != "127.0.0.1" {
		t.Errorf("SurrealDB host should use canonical config, got %q", cfg.Providers.GraphStore.SurrealDB.Host)
	}
	if cfg.Providers.GraphStore.SurrealDB.Namespace != "new_ns" {
		t.Errorf("SurrealDB namespace should use canonical config, got %q", cfg.Providers.GraphStore.SurrealDB.Namespace)
	}
}

func TestConfigDoesNotExposeLegacyProviderFields(t *testing.T) {
	v := reflect.TypeOf(Config{})
	for i := 0; i < v.NumField(); i++ {
		fieldName := v.Field(i).Name
		if fieldName == "Qdrant" {
			t.Error("Config should not expose legacy Qdrant field")
		}
		if fieldName == "Surreal" {
			t.Error("Config should not expose legacy Surreal field")
		}
	}
}

func TestProviderIDNormalization(t *testing.T) {
	dir := t.TempDir()
	yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\nproviders:\n  scriptRuntime:\n    enabled: true\n    required: false\n    provider: \"builtin.node-process\"\n"
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
		yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\nproviders:\n  scriptRuntime:\n    enabled: true\n    required: false\n    provider: \"" + id + "\"\n"
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
	yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\nproviders:\n  scriptRuntime:\n    enabled: false\n    required: true\n"
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := loadConfig(dir)
	if err == nil {
		t.Error("Should reject required=true and enabled=false for scriptRuntime")
	}

	dir2 := t.TempDir()
	yaml2 := "jwt:\n  secret: \"" + testJWTSecret + "\"\nproviders:\n  vectorStore:\n    enabled: false\n    required: true\n"
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
	yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\n"
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AMITIA_QDRANT_HOST", "10.0.0.5")
	t.Setenv("AMITIA_QDRANT_PORT", "5555")
	t.Setenv("AMITIA_SURREAL_HOST", "10.0.0.6")
	t.Setenv("AMITIA_SURREAL_PORT", "6666")
	t.Setenv("AMITIA_PLUGIN_HOST_ENABLED", "false")
	t.Setenv("AMITIA_NODE_BIN", "/env/path/node")
	defer func() {
		os.Unsetenv("AMITIA_QDRANT_HOST")
		os.Unsetenv("AMITIA_QDRANT_PORT")
		os.Unsetenv("AMITIA_SURREAL_HOST")
		os.Unsetenv("AMITIA_SURREAL_PORT")
		os.Unsetenv("AMITIA_PLUGIN_HOST_ENABLED")
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
	if cfg.Components.PluginHost.Enabled {
		t.Error("PluginHost should be disabled via env")
	}
	if cfg.Providers.ScriptRuntime.Node.BinaryPath != "/env/path/node" {
		t.Errorf("Node binary via env = %q", cfg.Providers.ScriptRuntime.Node.BinaryPath)
	}
}

func TestProviderEnvironmentFalseOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\n"
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AMITIA_QDRANT_ENABLED", "false")
	t.Setenv("AMITIA_SURREAL_ENABLED", "false")
	t.Setenv("AMITIA_PLUGIN_HOST_ENABLED", "false")
	t.Setenv("AMITIA_TASK_HOST_ENABLED", "false")
	t.Setenv("AMITIA_WECHAT_SIDECAR_ENABLED", "false")
	t.Setenv("AMITIA_QQ_SIDECAR_ENABLED", "false")
	defer func() {
		os.Unsetenv("AMITIA_QDRANT_ENABLED")
		os.Unsetenv("AMITIA_SURREAL_ENABLED")
		os.Unsetenv("AMITIA_PLUGIN_HOST_ENABLED")
		os.Unsetenv("AMITIA_TASK_HOST_ENABLED")
		os.Unsetenv("AMITIA_WECHAT_SIDECAR_ENABLED")
		os.Unsetenv("AMITIA_QQ_SIDECAR_ENABLED")
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
	if cfg.Components.PluginHost.Enabled {
		t.Error("PluginHost should be disabled via env false")
	}
	if cfg.Components.TaskHost.Enabled {
		t.Error("TaskHost should be disabled via env false")
	}
	if cfg.Components.Sidecars.Wechat.Enabled {
		t.Error("Wechat should be disabled via env false")
	}
	if cfg.Components.Sidecars.QQ.Enabled {
		t.Error("QQ should be disabled via env false")
	}
}

func TestCanonicalBinaryEnvironmentOverridesLegacy(t *testing.T) {
	dir := t.TempDir()
	yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\n"
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

func TestLegacyBinaryEnvironmentFallback(t *testing.T) {
	dir := t.TempDir()
	yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\n"
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
	yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\ncomponents:\n  pluginHost:\n    enabled: true\n    entryUri: \"amitia://runtime/polyglot/launcher.mjs\"\n    workUri: \"amitia://runtime/polyglot\"\n"
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig should accept valid URI: %v", err)
	}
	if cfg.Components.PluginHost.EntryURI != "amitia://runtime/polyglot/launcher.mjs" {
		t.Errorf("PluginHost entryUri = %q", cfg.Components.PluginHost.EntryURI)
	}

	dir2 := t.TempDir()
	yaml2 := "jwt:\n  secret: \"" + testJWTSecret + "\"\ncomponents:\n  pluginHost:\n    entryUri: \"file:///etc/passwd\"\n"
	configFile2 := filepath.Join(dir2, "config.yml")
	if err := os.WriteFile(configFile2, []byte(yaml2), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = loadConfig(dir2)
	if err == nil {
		t.Error("Should reject file:// URI")
	}

	dir3 := t.TempDir()
	yaml3 := "jwt:\n  secret: \"" + testJWTSecret + "\"\ncomponents:\n  pluginHost:\n    entryUri: \"\"\n"
	configFile3 := filepath.Join(dir3, "config.yml")
	if err := os.WriteFile(configFile3, []byte(yaml3), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = loadConfig(dir3)
	if err != nil {
		t.Errorf("Empty entryUri should be allowed: %v", err)
	}
}

func TestSidecarHealthURLValidation(t *testing.T) {
	dir := t.TempDir()
	yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\ncomponents:\n  sidecars:\n    wechat:\n      enabled: true\n      healthUrl: \"http://127.0.0.1:19876/api/health\"\n    qq:\n      enabled: true\n      healthUrl: \"https://example.com/health\"\n"
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig should accept valid URLs: %v", err)
	}
	if cfg.Components.Sidecars.Wechat.HealthURL != "http://127.0.0.1:19876/api/health" {
		t.Errorf("Wechat healthUrl = %q", cfg.Components.Sidecars.Wechat.HealthURL)
	}
	if cfg.Components.Sidecars.QQ.HealthURL != "https://example.com/health" {
		t.Errorf("QQ healthUrl = %q", cfg.Components.Sidecars.QQ.HealthURL)
	}

	dir2 := t.TempDir()
	yaml2 := "jwt:\n  secret: \"" + testJWTSecret + "\"\ncomponents:\n  sidecars:\n    wechat:\n      healthUrl: \"\"\n"
	configFile2 := filepath.Join(dir2, "config.yml")
	if err := os.WriteFile(configFile2, []byte(yaml2), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = loadConfig(dir2)
	if err != nil {
		t.Errorf("Empty healthUrl should be allowed: %v", err)
	}

	dir3 := t.TempDir()
	yaml3 := "jwt:\n  secret: \"" + testJWTSecret + "\"\ncomponents:\n  sidecars:\n    wechat:\n      healthUrl: \"ftp://example.com/health\"\n"
	configFile3 := filepath.Join(dir3, "config.yml")
	if err := os.WriteFile(configFile3, []byte(yaml3), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = loadConfig(dir3)
	if err == nil {
		t.Error("Should reject ftp:// scheme")
	}

	dir4 := t.TempDir()
	yaml4 := "jwt:\n  secret: \"" + testJWTSecret + "\"\ncomponents:\n  sidecars:\n    wechat:\n      healthUrl: \"http:///health\"\n"
	configFile4 := filepath.Join(dir4, "config.yml")
	if err := os.WriteFile(configFile4, []byte(yaml4), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = loadConfig(dir4)
	if err == nil {
		t.Error("Should reject healthUrl without host")
	}

	dir5 := t.TempDir()
	yaml5 := "jwt:\n  secret: \"" + testJWTSecret + "\"\ncomponents:\n  sidecars:\n    wechat:\n      healthUrl: \"http://user:pass@host/health\"\n"
	configFile5 := filepath.Join(dir5, "config.yml")
	if err := os.WriteFile(configFile5, []byte(yaml5), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = loadConfig(dir5)
	if err == nil {
		t.Error("Should reject healthUrl with userinfo")
	}
}

func TestProviderConfigDoesNotRequireBinaries(t *testing.T) {
	dir := t.TempDir()
	yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\nproviders:\n  scriptRuntime:\n    node:\n      binaryPath: \"/nonexistent/node\"\n  vectorStore:\n    qdrant:\n      binaryPath: \"/nonexistent/qdrant\"\n  graphStore:\n    surrealdb:\n      binaryPath: \"/nonexistent/surreal\"\n"
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
	yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\nproviders:\n  scriptRuntime:\n    enabled: true\n  vectorStore:\n    enabled: true\n  graphStore:\n    enabled: true\n"
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
	yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\n"
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
	if cfg.Server.Port != 18899 {
		t.Errorf("default server.port = %d, want 18899", cfg.Server.Port)
	}
}

func TestServerHostEnvOverride(t *testing.T) {
	dir := t.TempDir()
	yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\n"
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
	yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\n"
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
	yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\n"
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

	if cfg.Providers.VectorStore.Qdrant.Port != 19178 {
		t.Errorf("qdrant port should remain 19178, got %d", cfg.Providers.VectorStore.Qdrant.Port)
	}
	if cfg.Providers.GraphStore.SurrealDB.Port != 18000 {
		t.Errorf("surrealdb port should remain 18000, got %d", cfg.Providers.GraphStore.SurrealDB.Port)
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
	yaml := "jwt:\n  secret: \"" + testJWTSecret + "\"\n"
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AMITIA_SCRIPT_RUNTIME_ENABLED", "true")
	t.Setenv("AMITIA_SCRIPT_RUNTIME_REQUIRED", "true")
	t.Setenv("AMITIA_VECTOR_STORE_ENABLED", "true")
	t.Setenv("AMITIA_GRAPH_STORE_ENABLED", "false")
	t.Setenv("AMITIA_WECHAT_SIDECAR_ENABLED", "false")
	t.Setenv("AMITIA_QQ_SIDECAR_ENABLED", "false")
	t.Setenv("AMITIA_DESKTOP_PET_RUNTIME_ENABLED", "false")
	defer func() {
		os.Unsetenv("AMITIA_SCRIPT_RUNTIME_ENABLED")
		os.Unsetenv("AMITIA_SCRIPT_RUNTIME_REQUIRED")
		os.Unsetenv("AMITIA_VECTOR_STORE_ENABLED")
		os.Unsetenv("AMITIA_GRAPH_STORE_ENABLED")
		os.Unsetenv("AMITIA_WECHAT_SIDECAR_ENABLED")
		os.Unsetenv("AMITIA_QQ_SIDECAR_ENABLED")
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
	if cfg.Components.Sidecars.Wechat.Enabled {
		t.Error("Wechat should be disabled")
	}
	if cfg.Components.Sidecars.QQ.Enabled {
		t.Error("QQ should be disabled")
	}
	if cfg.DesktopPetRuntime.Enabled {
		t.Error("DesktopPetRuntime should be disabled")
	}
}
