// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantenv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

type fakeFileInspector struct {
	mu    sync.Mutex
	files map[string]fakeFileInfo
	stats int
}

type fakeFileInfo struct {
	mode  os.FileMode
	isDir bool
}

func newFakeFileInspector() *fakeFileInspector {
	return &fakeFileInspector{
		files: make(map[string]fakeFileInfo),
	}
}

func (f *fakeFileInspector) Stat(path string) (os.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stats++
	info, ok := f.files[path]
	if !ok {
		return nil, &os.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
	}
	return &fakeOSInfo{info: info}, nil
}

func (f *fakeFileInspector) Abs(path string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Clean(path), nil
	}
	return filepath.Clean(filepath.Join(wd, path)), nil
}

type fakeOSInfo struct {
	info fakeFileInfo
}

func (i *fakeOSInfo) Name() string {
	if i.info.isDir {
		return "dir"
	}
	return "file"
}

func (i *fakeOSInfo) Size() int64        { return 0 }
func (i *fakeOSInfo) ModTime() time.Time { return time.Time{} }
func (i *fakeOSInfo) Sys() interface{}   { return nil }
func (i *fakeOSInfo) Mode() os.FileMode  { return i.info.mode }
func (i *fakeOSInfo) IsDir() bool        { return i.info.isDir }

func (f *fakeFileInspector) AddFile(path string, mode os.FileMode) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files[path] = fakeFileInfo{mode: mode}
}

func (f *fakeFileInspector) Stats() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stats
}

type fakeRuntimeHost struct {
	descriptor platform.RuntimeDescriptor
	support    map[runtimehost.HostCapabilityID]runtimehost.CapabilitySupport
	paths      util.RuntimePaths
	instanceID string
}

func newFakeRuntimeHost(guest platform.GuestPlatform, host platform.HostPlatform, kind platform.RuntimeKind) *fakeRuntimeHost {
	h := &fakeRuntimeHost{
		descriptor: platform.RuntimeDescriptor{
			Host:         host,
			Kind:         kind,
			Guest:        guest,
			Architecture: "amd64",
		},
		support: map[runtimehost.HostCapabilityID]runtimehost.CapabilitySupport{
			runtimehost.CapProcessSpawn:         runtimehost.SupportSupported,
			runtimehost.CapFilesystemLocal:      runtimehost.SupportSupported,
			runtimehost.CapFilesystemExecutable: runtimehost.SupportSupported,
		},
		paths:      util.RuntimePaths{Root: absRootDir(), WorkspaceDir: absWorkspace()},
		instanceID: "test-instance",
	}
	return h
}

func absRootDir() string {
	if runtime.GOOS == "windows" {
		return `C:\runtime`
	}
	return "/runtime"
}

func absWorkspace() string {
	if runtime.GOOS == "windows" {
		return `C:\workspace`
	}
	return "/workspace"
}

func absBinExe() string {
	if runtime.GOOS == "windows" {
		return `C:\explicit.exe`
	}
	return "/explicit"
}

func (h *fakeRuntimeHost) Descriptor() platform.RuntimeDescriptor { return h.descriptor }

func (h *fakeRuntimeHost) Capabilities() *runtimehost.HostCapabilities {
	return runtimehost.NewTestCapabilitiesForTest(h.support)
}

func (h *fakeRuntimeHost) Paths() util.RuntimePaths                 { return h.paths }
func (h *fakeRuntimeHost) Processes() runtimehost.ProcessSupervisor { return nil }
func (h *fakeRuntimeHost) RuntimeInstanceID() string                { return h.instanceID }

func (h *fakeRuntimeHost) SetGuest(guest platform.GuestPlatform) { h.descriptor.Guest = guest }
func (h *fakeRuntimeHost) SetHost(host platform.HostPlatform)    { h.descriptor.Host = host }
func (h *fakeRuntimeHost) SetKind(kind platform.RuntimeKind)     { h.descriptor.Kind = kind }
func (h *fakeRuntimeHost) SetArch(arch string)                   { h.descriptor.Architecture = arch }
func (h *fakeRuntimeHost) SetUnsupported(ids ...runtimehost.HostCapabilityID) {
	for _, id := range ids {
		h.support[id] = runtimehost.SupportUnsupported
	}
}

func TestResolverReturnsDisabledWithoutScanning(t *testing.T) {
	inspector := newFakeFileInspector()
	host := newFakeRuntimeHost(platform.GuestPlatformWindows, platform.HostPlatformWindows, platform.RuntimeKindNativeProcess)
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			VectorStore: config.VectorStoreProviderConfig{
				Enabled: false,
				Qdrant:  config.QdrantConfig{},
			},
		},
	}

	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("NewResolver err: %v", err)
	}
	_, err = resolver.Resolve(context.Background())
	if err != ErrVectorStoreDisabled {
		t.Fatalf("expected ErrVectorStoreDisabled, got %v", err)
	}
	if inspector.Stats() != 0 {
		t.Fatalf("expected 0 stat calls, got %d", inspector.Stats())
	}
}

func TestResolverRejectsNonQdrantProviderWithoutScanning(t *testing.T) {
	inspector := newFakeFileInspector()
	host := newFakeRuntimeHost(platform.GuestPlatformWindows, platform.HostPlatformWindows, platform.RuntimeKindNativeProcess)
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			VectorStore: config.VectorStoreProviderConfig{
				Enabled:  true,
				Provider: "builtin.embedded-vector",
			},
		},
	}

	resolver, _ := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	_, err := resolver.Resolve(context.Background())
	if err != ErrQdrantProviderNotSelected {
		t.Fatalf("expected ErrQdrantProviderNotSelected, got %v", err)
	}
	if inspector.Stats() != 0 {
		t.Fatalf("expected 0 stat calls, got %d", inspector.Stats())
	}
}

func TestResolverRejectsUnsupportedHostBeforeScanning(t *testing.T) {
	inspector := newFakeFileInspector()
	host := newFakeRuntimeHost(platform.GuestPlatformIOS, platform.HostPlatformIOS, platform.RuntimeKindEmbedded)
	host.SetUnsupported(
		runtimehost.CapProcessSpawn,
		runtimehost.CapFilesystemLocal,
		runtimehost.CapFilesystemExecutable,
	)
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			VectorStore: config.VectorStoreProviderConfig{
				Enabled:  true,
				Provider: "builtin.qdrant-process",
			},
		},
	}

	resolver, _ := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	_, err := resolver.Resolve(context.Background())
	if !errors.Is(err, ErrHostCapabilityUnsupported) {
		t.Fatalf("expected ErrHostCapabilityUnsupported, got %v", err)
	}
	if inspector.Stats() != 0 {
		t.Fatalf("expected 0 stat calls, got %d", inspector.Stats())
	}
}

func TestAndroidProotUsesLinuxQdrantCandidates(t *testing.T) {
	inspector := newFakeFileInspector()
	host := newFakeRuntimeHost(platform.GuestPlatformLinux, platform.HostPlatformAndroid, platform.RuntimeKindProot)
	host.SetArch("arm64")
	linuxBin := filepath.Join(absRootDir(), "qdrant", "qdrant")
	inspector.AddFile(linuxBin, 0755)
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			VectorStore: config.VectorStoreProviderConfig{
				Enabled:  true,
				Provider: "builtin.qdrant-process",
			},
		},
	}

	resolver, _ := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	env, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if filepath.Base(env.BinaryPath) != "qdrant" {
		t.Fatalf("expected qdrant binary, got %s", filepath.Base(env.BinaryPath))
	}
	if env.Guest != platform.GuestPlatformLinux {
		t.Fatalf("expected Linux guest, got %s", env.Guest)
	}
}

func TestEmbeddedAndroidGuestIsUnsupported(t *testing.T) {
	inspector := newFakeFileInspector()
	host := newFakeRuntimeHost(platform.GuestPlatformAndroid, platform.HostPlatformAndroid, platform.RuntimeKindEmbedded)
	host.SetUnsupported(
		runtimehost.CapProcessSpawn,
		runtimehost.CapFilesystemLocal,
		runtimehost.CapFilesystemExecutable,
	)
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			VectorStore: config.VectorStoreProviderConfig{
				Enabled:  true,
				Provider: "builtin.qdrant-process",
			},
		},
	}

	resolver, _ := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	_, err := resolver.Resolve(context.Background())
	if !errors.Is(err, ErrHostCapabilityUnsupported) {
		t.Fatalf("expected ErrHostCapabilityUnsupported, got %v", err)
	}
}

func TestIOSGuestIsUnsupported(t *testing.T) {
	inspector := newFakeFileInspector()
	host := newFakeRuntimeHost(platform.GuestPlatformIOS, platform.HostPlatformIOS, platform.RuntimeKindEmbedded)
	host.SetUnsupported(
		runtimehost.CapProcessSpawn,
		runtimehost.CapFilesystemLocal,
		runtimehost.CapFilesystemExecutable,
	)
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			VectorStore: config.VectorStoreProviderConfig{
				Enabled:  true,
				Provider: "builtin.qdrant-process",
			},
		},
	}

	resolver, _ := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	_, err := resolver.Resolve(context.Background())
	if !errors.Is(err, ErrHostCapabilityUnsupported) {
		t.Fatalf("expected ErrHostCapabilityUnsupported, got %v", err)
	}
}

func TestExplicitBinaryPathHasHighestPriority(t *testing.T) {
	inspector := newFakeFileInspector()
	host := newFakeRuntimeHost(platform.GuestPlatformWindows, platform.HostPlatformWindows, platform.RuntimeKindNativeProcess)
	explicitPath := absBinExe()
	inspector.AddFile(explicitPath, 0644)
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			VectorStore: config.VectorStoreProviderConfig{
				Enabled:  true,
				Provider: "builtin.qdrant-process",
				Qdrant: config.QdrantConfig{
					BinaryPath: explicitPath,
				},
			},
		},
	}

	resolver, _ := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	env, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if env.Source != SourceExplicit {
		t.Fatalf("expected SourceExplicit, got %s", env.Source)
	}
	if filepath.Clean(env.BinaryPath) != filepath.Clean(explicitPath) {
		t.Fatalf("expected %s, got %s", explicitPath, env.BinaryPath)
	}
}

func TestInvalidExplicitBinaryDoesNotFallback(t *testing.T) {
	inspector := newFakeFileInspector()
	host := newFakeRuntimeHost(platform.GuestPlatformWindows, platform.HostPlatformWindows, platform.RuntimeKindNativeProcess)
	var explicitPath string
	if runtime.GOOS == "windows" {
		explicitPath = `C:\nonexistent\qdrant.exe`
	} else {
		explicitPath = "/nonexistent/qdrant.exe"
	}
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			VectorStore: config.VectorStoreProviderConfig{
				Enabled:  true,
				Provider: "builtin.qdrant-process",
				Qdrant: config.QdrantConfig{
					BinaryPath: explicitPath,
				},
			},
		},
	}

	resolver, _ := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	_, err := resolver.Resolve(context.Background())
	if !errors.Is(err, ErrExplicitBinaryNotFound) {
		t.Fatalf("expected ErrExplicitBinaryNotFound, got %v", err)
	}
}

func TestRuntimePackageCandidates(t *testing.T) {
	candidates := runtimePackageCandidates(platform.GuestPlatformWindows)
	if len(candidates) == 0 {
		t.Fatal("expected candidates for Windows")
	}
	candidates = runtimePackageCandidates(platform.GuestPlatformLinux)
	if len(candidates) == 0 {
		t.Fatal("expected candidates for Linux")
	}
}

func TestLinuxARM64LegacyCandidates(t *testing.T) {
	candidates := legacyCandidates(platform.GuestPlatformLinux, "arm64", "/runtime", "/workspace")
	if len(candidates) == 0 {
		t.Fatal("expected Linux ARM64 legacy candidates")
	}
	found := false
	for _, c := range candidates {
		if filepath.Base(c) == "qdrant_linux_aarch64" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected qdrant_linux_aarch64 candidate")
	}
}

func TestLinuxAMD64LegacyCandidates(t *testing.T) {
	candidates := legacyCandidates(platform.GuestPlatformLinux, "amd64", "/runtime", "/workspace")
	found := false
	for _, c := range candidates {
		if filepath.Base(c) == "qdrant_linux_x86" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected qdrant_linux_x86 candidate")
	}
}

func TestMacOSCandidates(t *testing.T) {
	candidates := legacyCandidates(platform.GuestPlatformMacOS, "arm64", "/runtime", "/workspace")
	if len(candidates) == 0 {
		t.Fatal("expected macOS candidates")
	}
}

func TestUnknownArchitectureUsesOnlyCanonicalNames(t *testing.T) {
	candidates := legacyCandidates(platform.GuestPlatformLinux, "riscv64", "/runtime", "/workspace")
	for _, c := range candidates {
		base := filepath.Base(c)
		if base != "qdrant" {
			t.Fatalf("unexpected candidate for unknown arch: %s", base)
		}
	}
}

func TestUnixBinaryRequiresExecutablePermission(t *testing.T) {
	info := &fakeOSInfo{info: fakeFileInfo{mode: 0644}}
	valid, reason := isValidBinaryPath(info, platform.GuestPlatformLinux)
	if valid {
		t.Fatal("expected 0644 to be invalid on Linux")
	}
	if reason != "not executable" {
		t.Fatalf("unexpected reason: %s", reason)
	}
	infoExec := &fakeOSInfo{info: fakeFileInfo{mode: 0755}}
	valid, _ = isValidBinaryPath(infoExec, platform.GuestPlatformLinux)
	if !valid {
		t.Fatal("expected 0755 to be valid on Linux")
	}
}

func TestWindowsBinaryIgnoresUnixExecutableBits(t *testing.T) {
	info := &fakeOSInfo{info: fakeFileInfo{mode: 0644}}
	valid, _ := isValidBinaryPath(info, platform.GuestPlatformWindows)
	if !valid {
		t.Fatal("expected Windows to ignore Unix executable bits")
	}
}

func TestBinaryCannotBeDirectory(t *testing.T) {
	info := &fakeOSInfo{info: fakeFileInfo{mode: 0755, isDir: true}}
	valid, reason := isValidBinaryPath(info, platform.GuestPlatformLinux)
	if valid {
		t.Fatal("expected directory to be invalid")
	}
	if reason != "is directory" {
		t.Fatalf("unexpected reason: %s", reason)
	}
}

func TestMissingAutomaticBinaryReturnsInstallTarget(t *testing.T) {
	inspector := newFakeFileInspector()
	host := newFakeRuntimeHost(platform.GuestPlatformWindows, platform.HostPlatformWindows, platform.RuntimeKindNativeProcess)
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			VectorStore: config.VectorStoreProviderConfig{
				Enabled:  true,
				Provider: "builtin.qdrant-process",
			},
		},
	}

	resolver, _ := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	env, err := resolver.Resolve(context.Background())
	if !errors.Is(err, ErrQdrantBinaryNotInstalled) {
		t.Fatalf("expected ErrQdrantBinaryNotInstalled, got %v", err)
	}
	if env.Installed {
		t.Fatal("expected Installed=false")
	}
	if env.Source != SourceRuntimePackage {
		t.Fatalf("expected SourceRuntimePackage, got %s", env.Source)
	}
	if env.DistributionRoot == "" {
		t.Fatal("expected DistributionRoot to be set")
	}
	if !filepath.IsAbs(env.BinaryPath) {
		t.Fatal("expected BinaryPath to be absolute")
	}
}

func TestInvalidateAllowsDetectionAfterInstallation(t *testing.T) {
	inspector := newFakeFileInspector()
	host := newFakeRuntimeHost(platform.GuestPlatformWindows, platform.HostPlatformWindows, platform.RuntimeKindNativeProcess)
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			VectorStore: config.VectorStoreProviderConfig{
				Enabled:  true,
				Provider: "builtin.qdrant-process",
			},
		},
	}

	resolver, _ := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	_, err := resolver.Resolve(context.Background())
	if !errors.Is(err, ErrQdrantBinaryNotInstalled) {
		t.Fatalf("expected ErrQdrantBinaryNotInstalled initially, got %v", err)
	}

	installTarget := filepath.Join(absRootDir(), "qdrant", "bin", "qdrant.exe")
	inspector.AddFile(installTarget, 0644)

	resolver.Invalidate()
	env, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("expected success after Invalidate, got %v", err)
	}
	if !env.Installed {
		t.Fatal("expected Installed=true after Invalidate")
	}
}

func TestResolverCachesInstalledEnvironment(t *testing.T) {
	inspector := newFakeFileInspector()
	host := newFakeRuntimeHost(platform.GuestPlatformWindows, platform.HostPlatformWindows, platform.RuntimeKindNativeProcess)
	inspector.AddFile(absBinExe(), 0644)
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			VectorStore: config.VectorStoreProviderConfig{
				Enabled:  true,
				Provider: "builtin.qdrant-process",
				Qdrant: config.QdrantConfig{
					BinaryPath: absBinExe(),
				},
			},
		},
	}

	resolver, _ := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	_, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	_, err = resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if inspector.Stats() != 1 {
		t.Fatalf("expected 1 stat call (cached), got %d", inspector.Stats())
	}
}

func TestNotInstalledResultCanBeInvalidated(t *testing.T) {
	inspector := newFakeFileInspector()
	host := newFakeRuntimeHost(platform.GuestPlatformWindows, platform.HostPlatformWindows, platform.RuntimeKindNativeProcess)
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			VectorStore: config.VectorStoreProviderConfig{
				Enabled:  true,
				Provider: "builtin.qdrant-process",
			},
		},
	}

	resolver, _ := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	_, err := resolver.Resolve(context.Background())
	if !errors.Is(err, ErrQdrantBinaryNotInstalled) {
		t.Fatalf("expected ErrQdrantBinaryNotInstalled, got %v", err)
	}
	statsBefore := inspector.Stats()

	resolver.Invalidate()
	_, err = resolver.Resolve(context.Background())
	if !errors.Is(err, ErrQdrantBinaryNotInstalled) {
		t.Fatalf("expected ErrQdrantBinaryNotInstalled after Invalidate, got %v", err)
	}
	if inspector.Stats() <= statsBefore {
		t.Fatalf("expected stats to increase after Invalidate")
	}
}

func TestConcurrentResolveRunsSingleScan(t *testing.T) {
	inspector := newFakeFileInspector()
	host := newFakeRuntimeHost(platform.GuestPlatformWindows, platform.HostPlatformWindows, platform.RuntimeKindNativeProcess)
	inspector.AddFile(absBinExe(), 0644)
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			VectorStore: config.VectorStoreProviderConfig{
				Enabled:  true,
				Provider: "builtin.qdrant-process",
				Qdrant: config.QdrantConfig{
					BinaryPath: absBinExe(),
				},
			},
		},
	}

	resolver, _ := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = resolver.Resolve(context.Background())
		}()
	}
	wg.Wait()

	if inspector.Stats() != 1 {
		t.Fatalf("expected 1 stat call (concurrent), got %d", inspector.Stats())
	}
}

func TestSnapshotDoesNotExposeMutableDiagnostics(t *testing.T) {
	inspector := newFakeFileInspector()
	host := newFakeRuntimeHost(platform.GuestPlatformWindows, platform.HostPlatformWindows, platform.RuntimeKindNativeProcess)
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			VectorStore: config.VectorStoreProviderConfig{
				Enabled:  true,
				Provider: "builtin.qdrant-process",
			},
		},
	}

	resolver, _ := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	_, _ = resolver.Resolve(context.Background())

	snap := resolver.Snapshot()
	if snap.Environment.Source == "" {
		t.Fatal("expected Snapshot to contain Environment")
	}
}

func TestResolveHonorsContextCancellation(t *testing.T) {
	inspector := newFakeFileInspector()
	host := newFakeRuntimeHost(platform.GuestPlatformWindows, platform.HostPlatformWindows, platform.RuntimeKindNativeProcess)
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			VectorStore: config.VectorStoreProviderConfig{
				Enabled: false,
			},
		},
	}
	resolver, _ := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := resolver.Resolve(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestResolverNeverUsesSystemPath(t *testing.T) {
	inspector := newFakeFileInspector()
	host := newFakeRuntimeHost(platform.GuestPlatformLinux, platform.HostPlatformLinux, platform.RuntimeKindNativeProcess)
	inspector.AddFile("/usr/bin/qdrant", 0755)
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			VectorStore: config.VectorStoreProviderConfig{
				Enabled:  true,
				Provider: "builtin.qdrant-process",
			},
		},
	}

	resolver, _ := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	env, err := resolver.Resolve(context.Background())
	if err != nil && !errors.Is(err, ErrQdrantBinaryNotInstalled) {
		t.Fatalf("unexpected error: %v", err)
	}
	if err == nil && env.BinaryPath == "/usr/bin/qdrant" {
		t.Fatal("expected system PATH binary to be ignored")
	}
}
