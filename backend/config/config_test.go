// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

const testSecret = "Test-Secret-Key-For-Testing-Only-1234567890"

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return dir
}

func cleanupRuntimeEnv(t *testing.T) {
	t.Helper()
	envKeys := []string{
		"AMITIA_RUNTIME_MODE",
		"AMITIA_RUN_MODE",
		"AMITIA_SERVER_HOST",
		"AMITIA_SERVER_PORT",
		"AMITIA_SERVER_MODE",
		"AMITIA_DATA_DIR",
		"AMITIA_DEPLOY_MODE",
		"AMITIA_SECURITY_MODE",
		"AMITIA_ALLOW_REMOTE_ACCESS",
		"AMITIA_LOCAL_TOKEN",
		"AMITIA_LOCAL_TOKEN_FILE",
		"AMITIA_LOCAL_USER_ID",
		"AMITIA_NODE_BIN",
		"AMITIA_NPM_BIN",
		"AMITIA_NPX_BIN",
		"AMITIA_NODE_WORK_DIR",
		"AMITIA_PLUGIN_HOST_ENABLED",
		"AMITIA_PLUGIN_HOST_PATH",
		"AMITIA_PLUGIN_HOST_WORK_DIR",
		"AMITIA_TASK_HOST_ENABLED",
		"AMITIA_TASK_HOST_PATH",
		"AMITIA_TASK_HOST_WORK_DIR",
		"AMITIA_WECHAT_SIDECAR_ENABLED",
		"AMITIA_WECHAT_SIDECAR_PATH",
		"AMITIA_WECHAT_SIDECAR_WORK_DIR",
		"AMITIA_WECHAT_SIDECAR_PORT",
		"AMITIA_WECHAT_SIDECAR_HEALTH_URL",
		"AMITIA_QQ_SIDECAR_ENABLED",
		"AMITIA_QQ_SIDECAR_PATH",
		"AMITIA_QQ_SIDECAR_WORK_DIR",
		"AMITIA_QQ_SIDECAR_PORT",
		"AMITIA_QQ_SIDECAR_HEALTH_URL",
		"AMITIA_QDRANT_ENABLED",
		"AMITIA_QDRANT_HOST",
		"AMITIA_QDRANT_PORT",
		"AMITIA_QDRANT_BIN",
		"AMITIA_QDRANT_DATA_DIR",
		"QDRANT_BIN",
		"QDRANT_DATA_DIR",
		"AMITIA_SURREAL_ENABLED",
		"AMITIA_SURREAL_HOST",
		"AMITIA_SURREAL_PORT",
		"AMITIA_SURREAL_BIN",
		"AMITIA_SURREAL_DATA_PATH",
		"SURREAL_BIN",
		"SURREAL_DATA_PATH",
		"AMITIA_DESKTOP_PET_RUNTIME_ENABLED",
	}
	for _, key := range envKeys {
		t.Setenv(key, "")
	}
}

func TestRuntimeConfigDefaults(t *testing.T) {
	cleanupRuntimeEnv(t)
	dir := writeTestConfig(t, "jwt:\n  secret: "+testSecret+"\n")

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Runtime.Mode != "desktop" {
		t.Fatalf("expected mode desktop, got %q", cfg.Runtime.Mode)
	}
	if !cfg.Qdrant.Enabled {
		t.Fatal("expected qdrant.enabled=true by default")
	}
	if !cfg.Surreal.Enabled {
		t.Fatal("expected surrealdb.enabled=true by default")
	}
	if !cfg.Runtime.PluginHost.Enabled {
		t.Fatal("expected pluginHost.enabled=true by default")
	}
	if !cfg.Runtime.TaskHost.Enabled {
		t.Fatal("expected taskHost.enabled=true by default")
	}
	if !cfg.Runtime.Sidecars.Wechat.Enabled {
		t.Fatal("expected wechat.enabled=true by default")
	}
	if !cfg.Runtime.Sidecars.QQ.Enabled {
		t.Fatal("expected qq.enabled=true by default")
	}
	if !cfg.DesktopPetRuntime.Enabled {
		t.Fatal("expected desktopPetRuntime.enabled=true by default")
	}
	if cfg.Runtime.Sidecars.Wechat.Port != 19876 {
		t.Fatalf("expected wechat port 19876, got %d", cfg.Runtime.Sidecars.Wechat.Port)
	}
	if cfg.Runtime.Sidecars.QQ.Port != 19877 {
		t.Fatalf("expected qq port 19877, got %d", cfg.Runtime.Sidecars.QQ.Port)
	}
	if cfg.Runtime.Node.BinaryPath != "" {
		t.Fatalf("expected empty node binary path, got %q", cfg.Runtime.Node.BinaryPath)
	}
}

func TestRuntimeConfigFromYAML(t *testing.T) {
	cleanupRuntimeEnv(t)
	content := `jwt:
  secret: ` + testSecret + `
runtime:
  mode: android-proot
  node:
    binaryPath: /opt/node/bin/node
    npmPath: /opt/node/bin/npm
    npxPath: /opt/node/bin/npx
    workDir: /opt/amitia/node
  pluginHost:
    enabled: true
    entryPath: /opt/plugin-host/index.js
    workDir: /opt/plugin-host
  taskHost:
    enabled: true
    entryPath: /opt/task-host/index.js
    workDir: /opt/task-host
  sidecars:
    wechat:
      enabled: true
      entryPath: /opt/sidecar/wechat.mjs
      workDir: /opt/sidecar
      port: 20001
      healthUrl: http://127.0.0.1:20001/api/health
    qq:
      enabled: true
      entryPath: /opt/sidecar/qq.mjs
      workDir: /opt/sidecar
      port: 20002
      healthUrl: http://127.0.0.1:20002/api/health
qdrant:
  enabled: true
surrealdb:
  enabled: false
desktopPetRuntime:
  enabled: false
`
	dir := writeTestConfig(t, content)
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Runtime.Mode != "android-proot" {
		t.Fatalf("expected mode android-proot, got %q", cfg.Runtime.Mode)
	}
	if cfg.Runtime.Node.BinaryPath != "/opt/node/bin/node" {
		t.Fatalf("expected node binary path, got %q", cfg.Runtime.Node.BinaryPath)
	}
	if cfg.Runtime.PluginHost.EntryPath != "/opt/plugin-host/index.js" {
		t.Fatalf("expected plugin host entry path, got %q", cfg.Runtime.PluginHost.EntryPath)
	}
	if !cfg.Qdrant.Enabled {
		t.Fatal("expected qdrant enabled")
	}
	if cfg.Surreal.Enabled {
		t.Fatal("expected surrealdb disabled")
	}
	if cfg.Runtime.Sidecars.Wechat.Port != 20001 {
		t.Fatalf("expected wechat port 20001, got %d", cfg.Runtime.Sidecars.Wechat.Port)
	}
	if cfg.DesktopPetRuntime.Enabled {
		t.Fatalf("expected desktop pet false")
	}
}

func TestAndroidProotEnvironmentOverrides(t *testing.T) {
	cleanupRuntimeEnv(t)
	t.Setenv("AMITIA_RUNTIME_MODE", "android-proot")
	t.Setenv("AMITIA_DEPLOY_MODE", "android-local")
	t.Setenv("AMITIA_SERVER_HOST", "127.0.0.1")
	t.Setenv("AMITIA_SERVER_PORT", "19870")
	t.Setenv("AMITIA_SECURITY_MODE", "local_single_user")
	t.Setenv("AMITIA_ALLOW_REMOTE_ACCESS", "false")
	t.Setenv("AMITIA_LOCAL_TOKEN_FILE", "/var/lib/amitia/token")
	t.Setenv("AMITIA_NODE_BIN", "/opt/node/bin/node")
	t.Setenv("AMITIA_NPM_BIN", "/opt/node/bin/npm")
	t.Setenv("AMITIA_NPX_BIN", "/opt/node/bin/npx")
	t.Setenv("AMITIA_QDRANT_ENABLED", "true")
	t.Setenv("AMITIA_QDRANT_HOST", "127.0.0.1")
	t.Setenv("AMITIA_SURREAL_ENABLED", "false")
	t.Setenv("AMITIA_WECHAT_SIDECAR_ENABLED", "false")
	t.Setenv("AMITIA_QQ_SIDECAR_ENABLED", "false")
	t.Setenv("AMITIA_DESKTOP_PET_RUNTIME_ENABLED", "false")

	dir := writeTestConfig(t, "jwt:\n  secret: "+testSecret+"\n")
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Runtime.Mode != "android-proot" {
		t.Fatalf("mode: got %q, want android-proot", cfg.Runtime.Mode)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("server.host: got %q", cfg.Server.Host)
	}
	if cfg.Server.Port != 19870 {
		t.Errorf("server.port: got %d", cfg.Server.Port)
	}
	if cfg.Security.Mode != "local_single_user" {
		t.Errorf("security.mode: got %q", cfg.Security.Mode)
	}
	if cfg.Security.LocalTokenFile != "/var/lib/amitia/token" {
		t.Errorf("localTokenFile: got %q", cfg.Security.LocalTokenFile)
	}
	if cfg.Runtime.Node.BinaryPath != "/opt/node/bin/node" {
		t.Errorf("node bin: got %q", cfg.Runtime.Node.BinaryPath)
	}
	if !cfg.Qdrant.Enabled {
		t.Error("qdrant.enabled should be true")
	}
	if cfg.Surreal.Enabled {
		t.Error("surrealdb.enabled should be false")
	}
	if cfg.Runtime.Sidecars.Wechat.Enabled {
		t.Error("wechat sidecar should be false")
	}
	if cfg.Runtime.Sidecars.QQ.Enabled {
		t.Error("qq sidecar should be false")
	}
	if cfg.DesktopPetRuntime.Enabled {
		t.Error("desktop pet should be false")
	}
}

func TestCanonicalEnvironmentOverridesLegacyEnvironment(t *testing.T) {
	cleanupRuntimeEnv(t)
	t.Setenv("AMITIA_RUNTIME_MODE", "android-proot")
	t.Setenv("AMITIA_RUN_MODE", "desktop")
	t.Setenv("AMITIA_QDRANT_BIN", "/new/path/qdrant")
	t.Setenv("QDRANT_BIN", "/old/path/qdrant")
	t.Setenv("AMITIA_QDRANT_DATA_DIR", "/new/qdrant-data")
	t.Setenv("QDRANT_DATA_DIR", "/old/qdrant-data")
	t.Setenv("AMITIA_SURREAL_BIN", "/new/path/surreal")
	t.Setenv("SURREAL_BIN", "/old/path/surreal")
	t.Setenv("AMITIA_SURREAL_DATA_PATH", "/new/surreal-data")
	t.Setenv("SURREAL_DATA_PATH", "/old/surreal-data")

	dir := writeTestConfig(t, "jwt:\n  secret: "+testSecret+"\n")
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Runtime.Mode != "android-proot" {
		t.Errorf("expected canonical mode, got %q", cfg.Runtime.Mode)
	}
	if cfg.Qdrant.BinaryPath != "/new/path/qdrant" {
		t.Errorf("expected canonical Qdrant bin, got %q", cfg.Qdrant.BinaryPath)
	}
	if cfg.Qdrant.DataDir != "/new/qdrant-data" {
		t.Errorf("expected canonical Qdrant data dir, got %q", cfg.Qdrant.DataDir)
	}
	if cfg.Surreal.BinaryPath != "/new/path/surreal" {
		t.Errorf("expected canonical surreal bin, got %q", cfg.Surreal.BinaryPath)
	}
	if cfg.Surreal.DataPath != "/new/surreal-data" {
		t.Errorf("expected canonical surreal data path, got %q", cfg.Surreal.DataPath)
	}
}

func TestLegacyEnvironmentFallback(t *testing.T) {
	cleanupRuntimeEnv(t)
	t.Setenv("AMITIA_RUN_MODE", "desktop")
	t.Setenv("QDRANT_BIN", "/legacy/qdrant")
	t.Setenv("QDRANT_DATA_DIR", "/legacy/qdrant-data")
	t.Setenv("SURREAL_BIN", "/legacy/surreal")
	t.Setenv("SURREAL_DATA_PATH", "/legacy/surreal-data")

	dir := writeTestConfig(t, "jwt:\n  secret: "+testSecret+"\n")
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Runtime.Mode != "desktop" {
		t.Errorf("expected legacy mode desktop, got %q", cfg.Runtime.Mode)
	}
	if cfg.Qdrant.BinaryPath != "/legacy/qdrant" {
		t.Errorf("expected legacy Qdrant bin, got %q", cfg.Qdrant.BinaryPath)
	}
	if cfg.Qdrant.DataDir != "/legacy/qdrant-data" {
		t.Errorf("expected legacy Qdrant data dir, got %q", cfg.Qdrant.DataDir)
	}
}

func TestEnvironmentFalseOverridesEnabledDefaults(t *testing.T) {
	cleanupRuntimeEnv(t)
	t.Setenv("AMITIA_QDRANT_ENABLED", "false")
	t.Setenv("AMITIA_SURREAL_ENABLED", "false")
	t.Setenv("AMITIA_WECHAT_SIDECAR_ENABLED", "false")
	t.Setenv("AMITIA_QQ_SIDECAR_ENABLED", "false")
	t.Setenv("AMITIA_PLUGIN_HOST_ENABLED", "false")
	t.Setenv("AMITIA_TASK_HOST_ENABLED", "false")
	t.Setenv("AMITIA_DESKTOP_PET_RUNTIME_ENABLED", "false")

	dir := writeTestConfig(t, "jwt:\n  secret: "+testSecret+"\n")
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Qdrant.Enabled {
		t.Error("qdrant should be disabled")
	}
	if cfg.Surreal.Enabled {
		t.Error("surrealdb should be disabled")
	}
	if cfg.Runtime.Sidecars.Wechat.Enabled {
		t.Error("wechat should be disabled")
	}
	if cfg.Runtime.Sidecars.QQ.Enabled {
		t.Error("qq should be disabled")
	}
	if cfg.Runtime.PluginHost.Enabled {
		t.Error("plugin host should be disabled")
	}
	if cfg.Runtime.TaskHost.Enabled {
		t.Error("task host should be disabled")
	}
	if cfg.DesktopPetRuntime.Enabled {
		t.Error("desktop pet should be disabled")
	}
}

func TestRuntimeConfigNormalization(t *testing.T) {
	cleanupRuntimeEnv(t)
	content := `jwt:
  secret: ` + testSecret + ` KeepThisExact
runtime:
  mode: " Android-Proot "
  node:
    binaryPath: "  /space/bin  "
  sidecars:
    wechat:
      entryPath: "  /sidecar/entry  "
      healthUrl: "  http://127.0.0.1:19876/api/health  "
qdrant:
  binaryPath: "  /qdrant/bin  "
  dataDir: "  /qdrant/data  "
surrealdb:
  binaryPath: "  /surreal/bin  "
  dataPath: "  /surreal/data  "
server:
  host: "  127.0.0.1  "
security:
  localTokenFile: "  /token  "
  localUserId: "  42  "
app:
  deployMode: "  android-local  "
`
	dir := writeTestConfig(t, content)
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Runtime.Mode != "android-proot" {
		t.Errorf("expected normalized mode, got %q", cfg.Runtime.Mode)
	}
	if cfg.Runtime.Node.BinaryPath != "/space/bin" {
		t.Errorf("expected trimmed node bin, got %q", cfg.Runtime.Node.BinaryPath)
	}
	if cfg.Runtime.Sidecars.Wechat.EntryPath != "/sidecar/entry" {
		t.Errorf("expected trimmed sidecar entry, got %q", cfg.Runtime.Sidecars.Wechat.EntryPath)
	}
	if cfg.Runtime.Sidecars.Wechat.HealthURL != "http://127.0.0.1:19876/api/health" {
		t.Errorf("expected trimmed healthUrl, got %q", cfg.Runtime.Sidecars.Wechat.HealthURL)
	}
	if cfg.Qdrant.BinaryPath != "/qdrant/bin" {
		t.Errorf("expected trimmed qdrant bin, got %q", cfg.Qdrant.BinaryPath)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected trimmed server host, got %q", cfg.Server.Host)
	}

	if cfg.JWT.Secret != testSecret+" KeepThisExact" {
		t.Errorf("JWT secret should be preserved exactly, got %q", cfg.JWT.Secret)
	}
}

func TestRejectUnknownRuntimeMode(t *testing.T) {
	cleanupRuntimeEnv(t)
	invalidModes := []string{"android", "linux-arm64", "desktop-linux", "unknown"}
	for _, mode := range invalidModes {
		t.Setenv("AMITIA_RUNTIME_MODE", mode)
		dir := writeTestConfig(t, "jwt:\n  secret: "+testSecret+"\n")
		_, err := loadConfig(dir)
		if err == nil {
			t.Fatalf("expected error for mode %q, got nil", mode)
		}
	}
}

func TestRuntimePortValidation(t *testing.T) {
	cleanupRuntimeEnv(t)
	invalidContent := `jwt:
  secret: ` + testSecret + `
server:
  port: 0
`
	dir := writeTestConfig(t, invalidContent)
	if _, err := loadConfig(dir); err == nil {
		t.Error("expected error for server port 0")
	}

	t.Setenv("AMITIA_SERVER_PORT", "70000")
	dir = writeTestConfig(t, "jwt:\n  secret: "+testSecret+"\n")
	if _, err := loadConfig(dir); err == nil {
		t.Error("expected error for server port 70000")
	}
	t.Setenv("AMITIA_SERVER_PORT", "")

	qdrantContent := `jwt:
  secret: ` + testSecret + `
qdrant:
  enabled: true
  port: 70000
`
	dir = writeTestConfig(t, qdrantContent)
	if _, err := loadConfig(dir); err == nil {
		t.Error("expected error for enabled qdrant with invalid port")
	}

	sidecarContent := `jwt:
  secret: ` + testSecret + `
runtime:
  sidecars:
    wechat:
      enabled: true
      port: 99999
`
	dir = writeTestConfig(t, sidecarContent)
	if _, err := loadConfig(dir); err == nil {
		t.Error("expected error for enabled wechat sidecar with invalid port")
	}

	t.Setenv("AMITIA_PLUGIN_HOST_ENABLED", "false")
	pluginContent := `jwt:
  secret: ` + testSecret + `
runtime:
  pluginHost:
    enabled: true
    port: 0
`
	dir = writeTestConfig(t, pluginContent)
	if _, err := loadConfig(dir); err != nil {
		t.Errorf("disabled component with port 0 should pass, got error: %v", err)
	}
}

func TestAndroidProotRequiresLoopback(t *testing.T) {
	cleanupRuntimeEnv(t)
	t.Setenv("AMITIA_RUNTIME_MODE", "android-proot")

	invalidHosts := []string{"0.0.0.0", "192.168.1.100", "10.0.0.1"}
	for _, host := range invalidHosts {
		t.Setenv("AMITIA_SERVER_HOST", host)
		t.Setenv("AMITIA_SERVER_PORT", "18899")
		dir := writeTestConfig(t, "jwt:\n  secret: "+testSecret+"\n")
		if _, err := loadConfig(dir); err == nil {
			t.Errorf("expected error for non-loopback host %q", host)
		}
	}
	t.Setenv("AMITIA_SERVER_HOST", "")
	t.Setenv("AMITIA_SERVER_PORT", "")

	t.Setenv("AMITIA_SERVER_HOST", "127.0.0.1")
	t.Setenv("AMITIA_SERVER_PORT", "18899")
	t.Setenv("AMITIA_ALLOW_REMOTE_ACCESS", "true")
	dir := writeTestConfig(t, "jwt:\n  secret: "+testSecret+"\n")
	if _, err := loadConfig(dir); err == nil {
		t.Error("expected error for allowRemoteAccess=true in android-proot")
	}
	t.Setenv("AMITIA_ALLOW_REMOTE_ACCESS", "")

	qdrantContent := `jwt:
  secret: ` + testSecret + `
qdrant:
  enabled: true
  host: 192.168.0.10
`
	dir = writeTestConfig(t, qdrantContent)
	if _, err := loadConfig(dir); err == nil {
		t.Error("expected error for non-loopback qdrant host")
	}

	surrealContent := `jwt:
  secret: ` + testSecret + `
surrealdb:
  enabled: true
  host: 10.0.0.1
`
	dir = writeTestConfig(t, surrealContent)
	if _, err := loadConfig(dir); err == nil {
		t.Error("expected error for non-loopback surrealdb host")
	}

	loopbackContent := `jwt:
  secret: ` + testSecret + `
server:
  host: localhost
runtime:
  sidecars:
    wechat:
      enabled: true
      healthUrl: http://127.0.0.1:19876/api/health
`
	dir = writeTestConfig(t, loopbackContent)
	if _, err := loadConfig(dir); err != nil {
		t.Errorf("loopback addresses should pass, got: %v", err)
	}

	t.Setenv("AMITIA_RUNTIME_MODE", "")
}

func TestSidecarHealthURLValidation(t *testing.T) {
	cleanupRuntimeEnv(t)

	validContent := `jwt:
  secret: ` + testSecret + `
runtime:
  sidecars:
    wechat:
      enabled: true
      healthUrl: http://127.0.0.1:19876/api/health
    qq:
      enabled: true
      healthUrl: https://localhost:19877/health
`
	dir := writeTestConfig(t, validContent)
	if _, err := loadConfig(dir); err != nil {
		t.Errorf("valid healthUrl should pass, got: %v", err)
	}

	invalidSchemeContent := `jwt:
  secret: ` + testSecret + `
runtime:
  sidecars:
    wechat:
      enabled: true
      healthUrl: ftp://127.0.0.1:19876/health
`
	dir = writeTestConfig(t, invalidSchemeContent)
	if _, err := loadConfig(dir); err == nil {
		t.Error("expected error for unsupported scheme")
	}

	missingHostContent := `jwt:
  secret: ` + testSecret + `
runtime:
  sidecars:
    qq:
      enabled: true
      healthUrl: http:///api/health
`
	dir = writeTestConfig(t, missingHostContent)
	if _, err := loadConfig(dir); err == nil {
		t.Error("expected error for missing host")
	}

	emptyHealthContent := `jwt:
  secret: ` + testSecret + `
runtime:
  sidecars:
    wechat:
      enabled: true
      healthUrl: ""
    qq:
      enabled: true
`
	dir = writeTestConfig(t, emptyHealthContent)
	if _, err := loadConfig(dir); err != nil {
		t.Errorf("empty healthUrl should be allowed, got: %v", err)
	}

	malformedContent := `jwt:
  secret: ` + testSecret + `
runtime:
  sidecars:
    qq:
      enabled: true
      healthUrl: "://bad url"
`
	dir = writeTestConfig(t, malformedContent)
	if _, err := loadConfig(dir); err == nil {
		t.Error("expected error for malformed URL")
	}
}

func TestRuntimeConfigDoesNotRequireBinaries(t *testing.T) {
	cleanupRuntimeEnv(t)
	content := `jwt:
  secret: ` + testSecret + `
runtime:
  mode: android-proot
  node:
    binaryPath: /nonexistent/node
    npmPath: /nonexistent/npm
    npxPath: /nonexistent/npx
    workDir: /nonexistent/node
  pluginHost:
    enabled: true
    entryPath: /nonexistent/plugin-host/index.js
    workDir: /nonexistent/plugin-host
  taskHost:
    enabled: true
    entryPath: /nonexistent/task-host/index.js
    workDir: /nonexistent/task-host
  sidecars:
    wechat:
      enabled: true
      entryPath: /nonexistent/sidecar/wechat.mjs
      workDir: /nonexistent/sidecar
    qq:
      enabled: true
      entryPath: /nonexistent/sidecar/qq.mjs
      workDir: /nonexistent/sidecar
qdrant:
  enabled: true
  binaryPath: /nonexistent/qdrant
  dataDir: /nonexistent/qdrant-data
surrealdb:
  enabled: true
  binaryPath: /nonexistent/surreal
  dataPath: /nonexistent/surreal-data
desktopPetRuntime:
  enabled: true
  path: /nonexistent/desktop-pet
`
	dir := writeTestConfig(t, content)
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("expected success for non-existent binaries, got: %v", err)
	}
	if cfg.Runtime.Node.BinaryPath != "/nonexistent/node" {
		t.Errorf("expected node path preserved, got %q", cfg.Runtime.Node.BinaryPath)
	}
}

func TestRuntimeConfigTemplates(t *testing.T) {
	templates := []string{
		"D:/桌面/跟进项目/U-Ai/backend/config/config.yml",
		"D:/桌面/跟进项目/U-Ai/config/config.yml",
		"D:/桌面/跟进项目/U-Ai/desktop/resources/config-template/config.yaml",
	}
	for _, path := range templates {
		if _, err := os.Stat(path); err != nil {
			t.Logf("skip template %s: %v", path, err)
			continue
		}
		v := viper.New()
		v.SetConfigName("config")
		v.SetConfigType("yml")
		dir := filepath.Dir(path)
		v.AddConfigPath(dir)
		ext := filepath.Ext(path)
		if ext == ".yaml" {
			v.SetConfigType("yaml")
		}
		if err := v.ReadInConfig(); err != nil {
			t.Errorf("template %s should parse: %v", path, err)
			continue
		}
		if !v.IsSet("runtime") {
			t.Errorf("template %s missing runtime block", path)
		}
		if !v.IsSet("qdrant.enabled") {
			t.Errorf("template %s missing qdrant.enabled", path)
		}
		if !v.IsSet("surrealdb.enabled") {
			t.Errorf("template %s missing surrealdb.enabled", path)
		}
	}
}
