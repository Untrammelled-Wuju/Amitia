package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func newTestDirManager(t *testing.T) (DirectoryManager, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dataRoot := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dataRoot, 0o700); err != nil {
		t.Fatalf("failed to create temp data root: %v", err)
	}
	dm, err := NewDirectoryManager(dataRoot)
	if err != nil {
		t.Fatalf("failed to create directory manager: %v", err)
	}
	return dm, dataRoot
}

func TestNewDirectoryManager_EmptyRoot(t *testing.T) {
	_, err := NewDirectoryManager("")
	if err == nil {
		t.Fatal("expected error for empty data root")
	}
}

func TestNewDirectoryManager_RelativeRoot(t *testing.T) {
	dm, err := NewDirectoryManager("relative/path")
	if err != nil {
		t.Fatalf("relative path should be accepted and resolved: %v", err)
	}
	paths, err := dm.ResolvePluginPaths("test.plugin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !filepath.IsAbs(paths.Root) {
		t.Fatalf("expected absolute path, got: %s", paths.Root)
	}
}

func TestResolvePluginPaths(t *testing.T) {
	dm, dataRoot := newTestDirManager(t)
	paths, err := dm.ResolvePluginPaths("com.example.plugin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if paths.Root == "" {
		t.Fatal("root path must not be empty")
	}
	if paths.Data == "" {
		t.Fatal("data path must not be empty")
	}
	if paths.Cache == "" {
		t.Fatal("cache path must not be empty")
	}
	if paths.Shared == "" {
		t.Fatal("shared path must not be empty")
	}

	if !strings.HasPrefix(paths.Root, dataRoot) {
		t.Fatalf("root %s not under data root %s", paths.Root, dataRoot)
	}
	if !strings.HasPrefix(paths.Data, paths.Root) {
		t.Fatalf("data %s not under root %s", paths.Data, paths.Root)
	}
	if !strings.HasPrefix(paths.Cache, paths.Root) {
		t.Fatalf("cache %s not under root %s", paths.Cache, paths.Root)
	}
	if !strings.HasPrefix(paths.Shared, paths.Root) {
		t.Fatalf("shared %s not under root %s", paths.Shared, paths.Root)
	}

	if !strings.Contains(paths.Root, "gamehost") || !strings.Contains(paths.Root, "plugins") {
		t.Fatalf("root %s should be under gamehost/plugins", paths.Root)
	}
}

func TestResolveRuntimePaths(t *testing.T) {
	dm, dataRoot := newTestDirManager(t)
	paths, err := dm.ResolveRuntimePaths("runtime-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if paths.Root == "" || paths.Data == "" || paths.Temp == "" || paths.Cache == "" || paths.Services == "" {
		t.Fatal("all runtime paths must be set")
	}

	if !strings.HasPrefix(paths.Root, dataRoot) {
		t.Fatalf("root %s not under data root %s", paths.Root, dataRoot)
	}
	if !strings.HasPrefix(paths.Data, paths.Root) {
		t.Fatalf("data not under root")
	}
	if !strings.HasPrefix(paths.Temp, paths.Root) {
		t.Fatalf("temp not under root")
	}
	if !strings.HasPrefix(paths.Cache, paths.Root) {
		t.Fatalf("cache not under root")
	}
	if !strings.HasPrefix(paths.Services, paths.Root) {
		t.Fatalf("services not under root")
	}

	if !strings.Contains(paths.Root, "gamehost") || !strings.Contains(paths.Root, "runtimes") {
		t.Fatalf("root %s should be under gamehost/runtimes", paths.Root)
	}
}

func TestResolveServicePaths(t *testing.T) {
	dm, dataRoot := newTestDirManager(t)
	runtimeID := domain.RuntimeInstanceID("runtime-456")
	serviceID := domain.ServiceID("bridge")

	paths, err := dm.ResolveServicePaths(runtimeID, serviceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if paths.Root == "" || paths.Data == "" || paths.Temp == "" || paths.Cache == "" {
		t.Fatal("all service paths must be set")
	}

	if !strings.HasPrefix(paths.Root, dataRoot) {
		t.Fatalf("root %s not under data root %s", paths.Root, dataRoot)
	}
	if !strings.HasPrefix(paths.Data, paths.Root) {
		t.Fatalf("data not under root")
	}
	if !strings.HasPrefix(paths.Temp, paths.Root) {
		t.Fatalf("temp not under root")
	}
	if !strings.HasPrefix(paths.Cache, paths.Root) {
		t.Fatalf("cache not under root")
	}

	if !strings.Contains(paths.Root, "gamehost") || !strings.Contains(paths.Root, "runtimes") || !strings.Contains(paths.Root, "services") {
		t.Fatalf("root %s should be under gamehost/runtimes/.../services", paths.Root)
	}
}

func TestMultipleServices_DifferentPaths(t *testing.T) {
	dm, _ := newTestDirManager(t)
	runtimeID := domain.RuntimeInstanceID("runtime-multi")
	svcA := domain.ServiceID("service-a")
	svcB := domain.ServiceID("service-b")

	pathsA, err := dm.ResolveServicePaths(runtimeID, svcA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pathsB, err := dm.ResolveServicePaths(runtimeID, svcB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pathsA.Root == pathsB.Root {
		t.Fatalf("different services should have different roots")
	}
	if pathsA.Data == pathsB.Data {
		t.Fatalf("different services should have different data paths")
	}
	if pathsA.Temp == pathsB.Temp {
		t.Fatalf("different services should have different temp paths")
	}
	if pathsA.Cache == pathsB.Cache {
		t.Fatalf("different services should have different cache paths")
	}
}

func TestMultipleRuntimes_DifferentPaths(t *testing.T) {
	dm, _ := newTestDirManager(t)
	pluginID := domain.PluginID("com.example.plugin")
	rtA := domain.RuntimeInstanceID("runtime-a")
	rtB := domain.RuntimeInstanceID("runtime-b")

	pathsA, err := dm.ResolveRuntimePaths(rtA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pathsB, err := dm.ResolveRuntimePaths(rtB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pathsA.Root == pathsB.Root {
		t.Fatalf("different runtimes should have different roots")
	}
	if pathsA.Data == pathsB.Data {
		t.Fatalf("different runtimes should have different data paths")
	}
	if pathsA.Temp == pathsB.Temp {
		t.Fatalf("different runtimes should have different temp paths")
	}
	if pathsA.Cache == pathsB.Cache {
		t.Fatalf("different runtimes should have different cache paths")
	}

	pluginPaths, _ := dm.ResolvePluginPaths(pluginID)
	if pathsA.Data == pluginPaths.Data {
		t.Fatalf("runtime data should differ from plugin data")
	}
}

func TestPluginShared_StableAcrossRuntimes(t *testing.T) {
	dm, _ := newTestDirManager(t)
	pluginID := domain.PluginID("com.example.shared-test")
	rtA := domain.RuntimeInstanceID("runtime-1")
	rtB := domain.RuntimeInstanceID("runtime-2")

	ctx := context.Background()
	_, err := dm.EnsurePluginPaths(ctx, pluginID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = dm.EnsureRuntimePaths(ctx, rtA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = dm.EnsureRuntimePaths(ctx, rtB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctxA, err := dm.BuildRuntimeContext(pluginID, rtA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ctxB, err := dm.BuildRuntimeContext(pluginID, rtB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ctxA.Plugin.Shared != ctxB.Plugin.Shared {
		t.Fatalf("plugin shared should be stable across runtimes: %s vs %s", ctxA.Plugin.Shared, ctxB.Plugin.Shared)
	}
	if ctxA.Plugin.Data != ctxB.Plugin.Data {
		t.Fatalf("plugin data should be stable across runtimes")
	}
	if ctxA.Plugin.Cache != ctxB.Plugin.Cache {
		t.Fatalf("plugin cache should be stable across runtimes")
	}
}

func TestEnsure_Idempotent(t *testing.T) {
	dm, _ := newTestDirManager(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		pluginID := domain.PluginID("com.example.idempotent")
		paths, err := dm.EnsurePluginPaths(ctx, pluginID)
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if _, err := os.Stat(paths.Data); err != nil {
			t.Fatalf("iteration %d: data dir missing: %v", i, err)
		}

		rtID := domain.RuntimeInstanceID("runtime-idem")
		rtPaths, err := dm.EnsureRuntimePaths(ctx, rtID)
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if _, err := os.Stat(rtPaths.Data); err != nil {
			t.Fatalf("iteration %d: runtime data dir missing: %v", i, err)
		}

		svcID := domain.ServiceID("svc-idem")
		svcPaths, err := dm.EnsureServicePaths(ctx, rtID, svcID)
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if _, err := os.Stat(svcPaths.Data); err != nil {
			t.Fatalf("iteration %d: service data dir missing: %v", i, err)
		}
	}
}

func TestRemoveRuntime_PreservesPluginData(t *testing.T) {
	dm, _ := newTestDirManager(t)
	ctx := context.Background()

	pluginID := domain.PluginID("com.example.cleanup")
	rtID := domain.RuntimeInstanceID("runtime-cleanup")
	svcID := domain.ServiceID("svc-cleanup")

	pluginPaths, err := dm.EnsurePluginPaths(ctx, pluginID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginPaths.Data, "persist.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatalf("failed to write plugin data: %v", err)
	}

	rtPaths, err := dm.EnsureRuntimePaths(ctx, rtID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rtPaths.Data, "runtime.txt"), []byte("remove"), 0o600); err != nil {
		t.Fatalf("failed to write runtime data: %v", err)
	}

	svcPaths, err := dm.EnsureServicePaths(ctx, rtID, svcID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(svcPaths.Data, "service.txt"), []byte("remove"), 0o600); err != nil {
		t.Fatalf("failed to write service data: %v", err)
	}

	if err := dm.RemoveRuntime(ctx, rtID); err != nil {
		t.Fatalf("unexpected error removing runtime: %v", err)
	}

	if _, err := os.Stat(pluginPaths.Data); err != nil {
		t.Fatalf("plugin data should be preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(pluginPaths.Data, "persist.txt")); err != nil {
		t.Fatalf("plugin data file should be preserved: %v", err)
	}

	if _, err := os.Stat(rtPaths.Root); !os.IsNotExist(err) {
		t.Fatalf("runtime root should be removed, err: %v", err)
	}
}

func TestRemoveRuntimeTemp(t *testing.T) {
	dm, _ := newTestDirManager(t)
	ctx := context.Background()

	rtID := domain.RuntimeInstanceID("runtime-temp")
	rtPaths, err := dm.EnsureRuntimePaths(ctx, rtID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tempFile := filepath.Join(rtPaths.Temp, "tmp.txt")
	if err := os.WriteFile(tempFile, []byte("temp"), 0o600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	dataFile := filepath.Join(rtPaths.Data, "persist.txt")
	if err := os.WriteFile(dataFile, []byte("keep"), 0o600); err != nil {
		t.Fatalf("failed to write data file: %v", err)
	}

	if err := dm.RemoveRuntimeTemp(ctx, rtID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
		t.Fatal("temp file should be removed")
	}
	if _, err := os.Stat(dataFile); err != nil {
		t.Fatalf("data file should be preserved: %v", err)
	}
}

func TestRemovePluginCache(t *testing.T) {
	dm, _ := newTestDirManager(t)
	ctx := context.Background()

	pluginID := domain.PluginID("com.example.cache")
	pluginPaths, err := dm.EnsurePluginPaths(ctx, pluginID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cacheFile := filepath.Join(pluginPaths.Cache, "cached.bin")
	if err := os.WriteFile(cacheFile, []byte("cache"), 0o600); err != nil {
		t.Fatalf("failed to write cache: %v", err)
	}
	dataFile := filepath.Join(pluginPaths.Data, "persist.bin")
	if err := os.WriteFile(dataFile, []byte("keep"), 0o600); err != nil {
		t.Fatalf("failed to write data: %v", err)
	}

	if err := dm.RemovePluginCache(ctx, pluginID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(cacheFile); !os.IsNotExist(err) {
		t.Fatal("cache file should be removed")
	}
	if _, err := os.Stat(dataFile); err != nil {
		t.Fatalf("plugin data should be preserved: %v", err)
	}
}

func TestEnsureWithinRoot_ValidChild(t *testing.T) {
	root := "/data/gamehost"
	tests := []string{
		"/data/gamehost/plugins/p1",
		"/data/gamehost/plugins/p1/data",
		"/data/gamehost",
	}
	for _, candidate := range tests {
		if err := EnsureWithinRoot(root, candidate); err != nil {
			t.Fatalf("expected %s to be within %s: %v", candidate, root, err)
		}
	}
}

func TestEnsureWithinRoot_Escape(t *testing.T) {
	root := "/data/gamehost"
	tests := []string{
		"/data/gamehost-evil",
		"/data",
		"/etc/passwd",
		"/data/gamehost/../etc",
		"/data/gamehost/../../secret",
	}
	for _, candidate := range tests {
		if err := EnsureWithinRoot(root, candidate); err == nil {
			t.Fatalf("expected %s to escape %s", candidate, root)
		}
	}
}

func TestEnsureWithinRoot_DotDot(t *testing.T) {
	root := "/data/gamehost"
	candidates := []string{
		"/data/gamehost/../../../etc/passwd",
		"/data/gamehost/plugins/../../secret",
	}
	for _, c := range candidates {
		if err := EnsureWithinRoot(root, c); err == nil {
			t.Fatalf("expected path traversal to fail: %s", c)
		}
	}
}

func TestEnsureWithinRoot_EmptyInputs(t *testing.T) {
	if err := EnsureWithinRoot("", "/foo"); err == nil {
		t.Fatal("expected error for empty root")
	}
	if err := EnsureWithinRoot("/foo", ""); err == nil {
		t.Fatal("expected error for empty candidate")
	}
}

func TestBuildRuntimeContext(t *testing.T) {
	dm, _ := newTestDirManager(t)
	pluginID := domain.PluginID("com.example.ctx")
	rtID := domain.RuntimeInstanceID("runtime-ctx")

	ctx, err := dm.BuildRuntimeContext(pluginID, rtID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ctx.Plugin.Root == "" {
		t.Fatal("plugin root required")
	}
	if ctx.Runtime.Root == "" {
		t.Fatal("runtime root required")
	}
	if ctx.Plugin.Root == ctx.Runtime.Root {
		t.Fatal("plugin and runtime roots must differ")
	}
}

func TestBuildServiceContext(t *testing.T) {
	dm, _ := newTestDirManager(t)
	pluginID := domain.PluginID("com.example.svc-ctx")
	rtID := domain.RuntimeInstanceID("runtime-svc-ctx")
	svcID := domain.ServiceID("svc-ctx")

	ctx, err := dm.BuildServiceContext(pluginID, rtID, svcID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ctx.Plugin.Root == "" {
		t.Fatal("plugin root required")
	}
	if ctx.Runtime.Root == "" {
		t.Fatal("runtime root required")
	}
	if ctx.Service.Root == "" {
		t.Fatal("service root required")
	}

	if ctx.Plugin.Root == ctx.Runtime.Root || ctx.Runtime.Root == ctx.Service.Root || ctx.Plugin.Root == ctx.Service.Root {
		t.Fatal("plugin, runtime, service roots must all differ")
	}
}

func TestContextImmutability(t *testing.T) {
	dm, _ := newTestDirManager(t)
	pluginID := domain.PluginID("com.example.immutable")
	rtID := domain.RuntimeInstanceID("runtime-immutable")

	ctx, err := dm.BuildRuntimeContext(pluginID, rtID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	originalPluginRoot := ctx.Plugin.Root
	originalRuntimeRoot := ctx.Runtime.Root

	ctx.Plugin.Root = "/hijacked"
	ctx.Runtime.Root = "/hijacked"

	newCtx, err := dm.BuildRuntimeContext(pluginID, rtID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if newCtx.Plugin.Root != originalPluginRoot {
		t.Fatalf("plugin root was mutated: %s vs %s", newCtx.Plugin.Root, originalPluginRoot)
	}
	if newCtx.Runtime.Root != originalRuntimeRoot {
		t.Fatalf("runtime root was mutated: %s vs %s", newCtx.Runtime.Root, originalRuntimeRoot)
	}
}

func TestResolve_NoSideEffects(t *testing.T) {
	dm, _ := newTestDirManager(t)
	pluginID := domain.PluginID("com.example.nosideeffect")

	_, err := dm.ResolvePluginPaths(pluginID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	paths, err := dm.ResolvePluginPaths(pluginID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(paths.Root); !os.IsNotExist(err) {
		t.Fatal("Resolve should not create directories")
	}
}

func TestEnsure_CreatesDirectories(t *testing.T) {
	dm, _ := newTestDirManager(t)
	ctx := context.Background()

	pluginID := domain.PluginID("com.example.ensure")
	paths, err := dm.EnsurePluginPaths(ctx, pluginID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dirs := []string{paths.Root, paths.Data, paths.Cache, paths.Shared}
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("directory %s should exist: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("path %s should be a directory", dir)
		}
	}
}
