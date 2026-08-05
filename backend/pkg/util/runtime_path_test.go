// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

package util

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExplicitAbsoluteRuntimeRoot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only meaningful on linux")
		return
	}

	tmp := t.TempDir()
	nonExistent := filepath.Join(tmp, "does-not-exist")
	t.Setenv(RuntimeRootEnv, nonExistent)

	got := RuntimeRoot()
	absExpected, _ := filepath.Abs(nonExistent)
	absExpected = filepath.Clean(absExpected)
	if got != absExpected {
		t.Fatalf("expected RuntimeRoot()=%s, got %s", absExpected, got)
	}
}

func TestRelativeRuntimeRoot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only meaningful on linux")
		return
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("cannot get wd: %v", err)
	}

	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	t.Setenv(RuntimeRootEnv, "runtime")

	got := RuntimeRoot()
	expected := filepath.Join(tmp, "runtime")
	expected = filepath.Clean(expected)
	if got != expected {
		t.Fatalf("expected RuntimeRoot()=%s, got %s", expected, got)
	}
}

func TestProjectSourceRootPath(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only meaningful on linux")
		return
	}

	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "backend"), 0755); err != nil {
		t.Fatalf("mkdir backend: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "config"), 0755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}

	if !isRuntimeRoot(tmp) {
		t.Fatalf("expected isRuntimeRoot(%s)=true for layout 1", tmp)
	}
}

func TestGoBackendSourceRootPath(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only meaningful on linux")
		return
	}

	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "cmd"), 0755); err != nil {
		t.Fatalf("mkdir cmd: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "config"), 0755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}

	if !isRuntimeRoot(tmp) {
		t.Fatalf("expected isRuntimeRoot(%s)=true for layout 2", tmp)
	}
}

func TestLegacyDesktopDirectoryRecognition(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only meaningful on linux")
		return
	}

	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "config"), 0755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "qdrant"), 0755); err != nil {
		t.Fatalf("mkdir qdrant: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "surrealdb"), 0755); err != nil {
		t.Fatalf("mkdir surrealdb: %v", err)
	}

	if !isRuntimeRoot(tmp) {
		t.Fatalf("expected isRuntimeRoot(%s)=true for layout 3", tmp)
	}
}

func TestIncompleteDirectoryNotRecognized(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only meaningful on linux")
		return
	}

	cases := map[string]func(string) error{
		"only config": func(root string) error {
			return os.MkdirAll(filepath.Join(root, "config"), 0755)
		},
		"only data": func(root string) error {
			return os.MkdirAll(filepath.Join(root, "data"), 0755)
		},
		"only qdrant": func(root string) error {
			return os.MkdirAll(filepath.Join(root, "qdrant"), 0755)
		},
		"only surrealdb": func(root string) error {
			return os.MkdirAll(filepath.Join(root, "surrealdb"), 0755)
		},
		"only backend": func(root string) error {
			return os.MkdirAll(filepath.Join(root, "backend"), 0755)
		},
	}

	for name, setup := range cases {
		tmp := t.TempDir()
		if err := setup(tmp); err != nil {
			t.Fatalf("setup %s: %v", name, err)
		}
		if isRuntimeRoot(tmp) {
			t.Fatalf("expected isRuntimeRoot(%s)=false for %s", tmp, name)
		}
	}
}

func TestConfigDirPriority(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only meaningful on linux")
		return
	}

	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	configFromEnvA := filepath.Join(tmp, "cfg_a")
	configFromEnvB := filepath.Join(tmp, "cfg_b")
	configDefault := filepath.Join(root, "config")

	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	if err := os.MkdirAll(configFromEnvA, 0755); err != nil {
		t.Fatalf("mkdir cfg_a: %v", err)
	}
	if err := os.MkdirAll(configFromEnvB, 0755); err != nil {
		t.Fatalf("mkdir cfg_b: %v", err)
	}
	if err := os.MkdirAll(configDefault, 0755); err != nil {
		t.Fatalf("mkdir default config: %v", err)
	}

	t.Setenv(ConfigDirEnv, configFromEnvA)
	t.Setenv(CONFIG_PATH, configFromEnvB)
	got := RuntimeConfigDir(root)
	if got != configFromEnvA {
		t.Fatalf("expected ConfigDir to use %s, got %s", configFromEnvA, got)
	}

	t.Setenv(ConfigDirEnv, "")
	got = RuntimeConfigDir(root)
	if got != configFromEnvB {
		t.Fatalf("expected ConfigDir to use CONFIG_PATH %s, got %s", configFromEnvB, got)
	}

	t.Setenv(CONFIG_PATH, "")
	got = RuntimeConfigDir(root)
	if got != configDefault {
		t.Fatalf("expected ConfigDir to use default %s, got %s", configDefault, got)
	}

	relEnv := "custom_config"
	t.Setenv(ConfigDirEnv, relEnv)
	got = RuntimeConfigDir(root)
	expected := filepath.Join(root, relEnv)
	if got != expected {
		t.Fatalf("expected relative ConfigDir %s, got %s", expected, got)
	}

	t.Setenv(ConfigDirEnv, "")
	t.Setenv(CONFIG_PATH, "")
}

func TestDataDirPriority(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only meaningful on linux")
		return
	}

	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	envDataDir := filepath.Join(tmp, "custom_data")
	absDataDir := filepath.Join(tmp, "abs_data")
	defaultDataDir := filepath.Join(root, "data")

	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	if err := os.MkdirAll(defaultDataDir, 0755); err != nil {
		t.Fatalf("mkdir default data: %v", err)
	}

	t.Setenv(DataDirEnv, envDataDir)
	got := RuntimeDataDir(root, "relative_data")
	if got != envDataDir {
		t.Fatalf("expected env DataDir to win, got %s", got)
	}

	t.Setenv(DataDirEnv, "")
	got = RuntimeDataDir(root, absDataDir)
	if got != absDataDir {
		t.Fatalf("expected absolute configured DataDir to win, got %s", got)
	}

	relData := "subdir_data"
	got = RuntimeDataDir(root, relData)
	expected := filepath.Join(root, relData)
	if got != expected {
		t.Fatalf("expected relative DataDir resolved, got %s, want %s", got, expected)
	}

	got = RuntimeDataDir(root, "")
	if got != defaultDataDir {
		t.Fatalf("expected default DataDir, got %s", got)
	}

	got = RuntimeDataDir(root, "data")
	if got == filepath.Join(root, "data", "data") {
		t.Fatalf("DataDir should not double-patch %q", got)
	}

	t.Setenv(DataDirEnv, "")
}

func TestLogDir(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only meaningful on linux")
		return
	}

	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	defaultLogDir := filepath.Join(root, "logs")
	envLogDir := filepath.Join(tmp, "custom_logs")

	t.Setenv(LogDirEnv, envLogDir)
	got := RuntimeLogDir(root)
	if got != envLogDir {
		t.Fatalf("expected env LogDir to win, got %s (want %s)", got, envLogDir)
	}

	t.Setenv(LogDirEnv, "")
	got = RuntimeLogDir(root)
	if got != defaultLogDir {
		t.Fatalf("expected default LogDir, got %s", got)
	}

	relEnvLog := "applogs"
	t.Setenv(LogDirEnv, relEnvLog)
	got = RuntimeLogDir(root)
	expected := filepath.Join(root, relEnvLog)
	if got != expected {
		t.Fatalf("expected relative LogDir resolved, got %s, want %s", got, expected)
	}

	t.Setenv(LogDirEnv, "")
}

func TestWorkspaceDir(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only meaningful on linux")
		return
	}

	tmp := t.TempDir()
	envWorkspace := filepath.Join(tmp, "explicit_workspace")
	fallbackRoot := filepath.Join(tmp, "fallback_root")

	if err := os.MkdirAll(envWorkspace, 0755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.MkdirAll(fallbackRoot, 0755); err != nil {
		t.Fatalf("mkdir fallback: %v", err)
	}

	t.Setenv(WorkspaceDirEnv, envWorkspace)
	got := RuntimeWorkspaceDir(fallbackRoot)
	if got != envWorkspace {
		t.Fatalf("expected env WorkspaceDir to win, got %s", got)
	}

	relEnvWorkspace := "relworkspace"
	t.Setenv(WorkspaceDirEnv, relEnvWorkspace)
	got = RuntimeWorkspaceDir(fallbackRoot)
	expected := filepath.Join(fallbackRoot, relEnvWorkspace)
	if got != expected {
		t.Fatalf("expected relative WorkspaceDir resolved, got %s, want %s", got, expected)
	}

	t.Setenv(WorkspaceDirEnv, "")
	got = RuntimeWorkspaceDir(fallbackRoot)
	if got != fallbackRoot {
		t.Fatalf("expected fallback RuntimeRoot, got %s", got)
	}
}

func TestCacheAndTempDir(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only meaningful on linux")
		return
	}

	tmp := t.TempDir()
	dataDir := filepath.Join(tmp, "data")
	root := filepath.Join(tmp, "root")
	envCache := filepath.Join(tmp, "custom_cache")
	envTemp := filepath.Join(tmp, "custom_temp")

	t.Setenv(CacheDirEnv, envCache)
	t.Setenv(TempDirEnv, envTemp)

	gotCache := RuntimeCacheDir(root, dataDir)
	gotTemp := RuntimeTempDir(root, dataDir)
	if gotCache != envCache {
		t.Fatalf("expected env CacheDir to win, got %s", gotCache)
	}
	if gotTemp != envTemp {
		t.Fatalf("expected env TempDir to win, got %s", gotTemp)
	}

	t.Setenv(CacheDirEnv, "")
	t.Setenv(TempDirEnv, "")

	gotCache = RuntimeCacheDir(root, dataDir)
	gotTemp = RuntimeTempDir(root, dataDir)
	expectedCache := filepath.Join(dataDir, "cache")
	expectedTemp := filepath.Join(dataDir, "tmp")
	if gotCache != expectedCache {
		t.Fatalf("expected CacheDir from dataDir %s, got %s", expectedCache, gotCache)
	}
	if gotTemp != expectedTemp {
		t.Fatalf("expected TempDir from dataDir %s, got %s", expectedTemp, gotTemp)
	}

	gotCache = RuntimeCacheDir(root, "")
	gotTemp = RuntimeTempDir(root, "")
	expectedCache = filepath.Join(root, "data", "cache")
	expectedTemp = filepath.Join(root, "data", "tmp")
	if gotCache != expectedCache {
		t.Fatalf("expected fallback CacheDir %s, got %s", expectedCache, gotCache)
	}
	if gotTemp != expectedTemp {
		t.Fatalf("expected fallback TempDir %s, got %s", expectedTemp, gotTemp)
	}
}

func TestNoDirectoryCreation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only meaningful on linux")
		return
	}

	tmp := t.TempDir()
	t.Setenv(RuntimeRootEnv, filepath.Join(tmp, "nonexistent_root"))
	t.Setenv(ConfigDirEnv, filepath.Join(tmp, "nonexistent_cfg"))
	t.Setenv(DataDirEnv, filepath.Join(tmp, "nonexistent_data"))
	t.Setenv(LogDirEnv, filepath.Join(tmp, "nonexistent_log"))
	t.Setenv(WorkspaceDirEnv, filepath.Join(tmp, "nonexistent_ws"))
	t.Setenv(CacheDirEnv, filepath.Join(tmp, "nonexistent_cache"))
	t.Setenv(TempDirEnv, filepath.Join(tmp, "nonexistent_temp"))

	root := RuntimeRoot()
	paths := DetectRuntimePaths("data")
	_ = RuntimeConfigDir(root)
	_ = RuntimeDataDir(root, "")
	_ = RuntimeLogDir(root)
	_ = RuntimeWorkspaceDir(root)
	_ = RuntimeCacheDir(root, "")
	_ = RuntimeTempDir(root, "")

	dirs := []string{root, paths.ConfigDir, paths.DataDir, paths.LogDir, paths.WorkspaceDir, paths.CacheDir, paths.TempDir}
	for _, d := range dirs {
		if _, err := os.Stat(d); err == nil {
			t.Fatalf("directory should not be created for %s", d)
		}
	}

	if paths.Root != root {
		t.Fatalf("DetectRuntimePaths root mismatch: %s vs %s", paths.Root, root)
	}
}

func TestEnvironmentChangeImmediateEffect(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only meaningful on linux")
		return
	}

	temp := t.TempDir()
	rootA := filepath.Join(temp, "a")
	rootB := filepath.Join(temp, "b")
	dataA := filepath.Join(temp, "dataA")
	dataB := filepath.Join(temp, "dataB")
	logA := filepath.Join(temp, "logA")
	logB := filepath.Join(temp, "logB")

	t.Setenv(RuntimeRootEnv, rootA)
	t.Setenv(DataDirEnv, dataA)
	t.Setenv(LogDirEnv, logA)

	root1 := RuntimeRoot()
	data1 := RuntimeDataDir(root1, "")
	log1 := RuntimeLogDir(root1)
	if root1 != rootA || data1 != dataA || log1 != logA {
		t.Fatalf("first read not matching env: %s %s %s", root1, data1, log1)
	}

	t.Setenv(RuntimeRootEnv, rootB)
	t.Setenv(DataDirEnv, dataB)
	t.Setenv(LogDirEnv, logB)

	root2 := RuntimeRoot()
	data2 := RuntimeDataDir(root2, "")
	log2 := RuntimeLogDir(root2)
	if root2 != rootB || data2 != dataB || log2 != logB {
		t.Fatalf("second read not matching env: %s %s %s", root2, data2, log2)
	}

	t.Setenv(RuntimeRootEnv, "")
	t.Setenv(DataDirEnv, "")
	t.Setenv(LogDirEnv, "")
}

func TestResolveRuntimePath(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only meaningful on linux")
		return
	}

	got := ResolveRuntimePath("/root", "")
	if got != "" {
		t.Fatalf("expected empty for empty p, got %s", got)
	}

	got = ResolveRuntimePath("/root", "/abs/path/file.txt")
	want := "/abs/path/file.txt"
	if got != want {
		t.Fatalf("expected abs path %s, got %s", want, got)
	}

	got = ResolveRuntimePath("/root", "rel/path/file.txt")
	want = filepath.Join("/root", "rel/path/file.txt")
	if got != want {
		t.Fatalf("expected joined path %s, got %s", want, got)
	}

	got = ResolveRuntimePath("", "rel/path/file.txt")
	want = filepath.Clean("rel/path/file.txt")
	if got != want {
		t.Fatalf("expected clean rel path %s, got %s", want, got)
	}

	got = ResolveRuntimePath("/root", "../escape/file.txt")
	want = filepath.Clean(filepath.Join("/root", "../escape/file.txt"))
	if got != want {
		t.Fatalf("expected escaped path %s, got %s", want, got)
	}

	got = ResolveRuntimePath("/root", "path//double//file.txt")
	want = filepath.Clean(filepath.Join("/root", "path//double//file.txt"))
	if got != want {
		t.Fatalf("expected double-slash cleaned path %s, got %s", want, got)
	}
}

func TestDetectRuntimePaths(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only meaningful on linux")
		return
	}

	temp := t.TempDir()
	root := filepath.Join(temp, "root")
	cfg := filepath.Join(temp, "cfg")
	data := filepath.Join(temp, "data")
	logs := filepath.Join(temp, "logs")
	ws := filepath.Join(temp, "ws")
	cache := filepath.Join(temp, "cache")
	tmp := filepath.Join(temp, "tmp")

	t.Setenv(RuntimeRootEnv, root)
	t.Setenv(ConfigDirEnv, cfg)
	t.Setenv(DataDirEnv, data)
	t.Setenv(LogDirEnv, logs)
	t.Setenv(WorkspaceDirEnv, ws)
	t.Setenv(CacheDirEnv, cache)
	t.Setenv(TempDirEnv, tmp)

	paths := DetectRuntimePaths("")
	if paths.Root != root {
		t.Fatalf("Root mismatch: %s", paths.Root)
	}
	if paths.ConfigDir != cfg {
		t.Fatalf("ConfigDir mismatch: %s", paths.ConfigDir)
	}
	if paths.DataDir != data {
		t.Fatalf("DataDir mismatch: %s", paths.DataDir)
	}
	if paths.LogDir != logs {
		t.Fatalf("LogDir mismatch: %s", paths.LogDir)
	}
	if paths.WorkspaceDir != ws {
		t.Fatalf("WorkspaceDir mismatch: %s", paths.WorkspaceDir)
	}
	if paths.CacheDir != cache {
		t.Fatalf("CacheDir mismatch: %s", paths.CacheDir)
	}
	if paths.TempDir != tmp {
		t.Fatalf("TempDir mismatch: %s", paths.TempDir)
	}

	t.Setenv(RuntimeRootEnv, "")
	t.Setenv(ConfigDirEnv, "")
	t.Setenv(DataDirEnv, "")
	t.Setenv(LogDirEnv, "")
	t.Setenv(WorkspaceDirEnv, "")
	t.Setenv(CacheDirEnv, "")
	t.Setenv(TempDirEnv, "")
}
