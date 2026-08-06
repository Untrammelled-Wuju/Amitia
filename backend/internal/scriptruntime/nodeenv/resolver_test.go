package nodeenv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

type fakeFileInfo struct {
	mode os.FileMode
	dir  bool
}

func (fi fakeFileInfo) Name() string       { return "fake" }
func (fi fakeFileInfo) Size() int64        { return 0 }
func (fi fakeFileInfo) Mode() os.FileMode  { return fi.mode }
func (fi fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fi fakeFileInfo) IsDir() bool        { return fi.dir }
func (fi fakeFileInfo) Sys() interface{}   { return nil }

type fakeFileInspector struct {
	mu        sync.Mutex
	statCalls int
	files     map[string]os.FileInfo
}

func (f *fakeFileInspector) Stat(path string) (os.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statCalls++
	if info, ok := f.files[path]; ok {
		return info, nil
	}
	return nil, os.ErrNotExist
}

func (f *fakeFileInspector) Abs(path string) (string, error) {
	return filepath.Abs(path)
}

func (f *fakeFileInspector) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.statCalls
}

func newFakeInspector(files map[string]os.FileInfo) *fakeFileInspector {
	if files == nil {
		files = make(map[string]os.FileInfo)
	}
	return &fakeFileInspector{files: files}
}

type fakeRuntimeHost struct {
	descr platform.RuntimeDescriptor
	caps  *runtimehost.HostCapabilities
	paths util.RuntimePaths
}

func (h *fakeRuntimeHost) Descriptor() platform.RuntimeDescriptor      { return h.descr }
func (h *fakeRuntimeHost) Capabilities() *runtimehost.HostCapabilities { return h.caps }
func (h *fakeRuntimeHost) Paths() util.RuntimePaths                    { return h.paths }
func (h *fakeRuntimeHost) Processes() runtimehost.ProcessSupervisor    { return nil }
func (h *fakeRuntimeHost) RuntimeInstanceID() string                   { return "fake" }

func newNativeHostCaps() *runtimehost.HostCapabilities {
	host, err := runtimehost.NewRuntimeHost(runtimehost.HostBuildContext{
		Descriptor: platform.RuntimeDescriptor{
			Host:  platform.HostPlatformLinux,
			Kind:  platform.RuntimeKindNativeProcess,
			Guest: platform.GuestPlatformLinux,
		},
		Paths: util.RuntimePaths{Root: "/tmp"},
	})
	if err != nil {
		panic(err)
	}
	return host.Capabilities()
}

func makeFakeHost(guest platform.GuestPlatform, arch, root, ws string, caps *runtimehost.HostCapabilities) *fakeRuntimeHost {
	if caps == nil {
		caps = newNativeHostCaps()
	}
	return &fakeRuntimeHost{
		descr: platform.RuntimeDescriptor{
			Host:         guestToHost(guest),
			Kind:         platform.RuntimeKindNativeProcess,
			Guest:        guest,
			Architecture: arch,
		},
		caps: caps,
		paths: util.RuntimePaths{
			Root:         root,
			WorkspaceDir: ws,
		},
	}
}

func guestToHost(guest platform.GuestPlatform) platform.HostPlatform {
	switch guest {
	case platform.GuestPlatformWindows:
		return platform.HostPlatformWindows
	case platform.GuestPlatformLinux:
		return platform.HostPlatformLinux
	case platform.GuestPlatformMacOS:
		return platform.HostPlatformMacOS
	default:
		return platform.HostPlatformUnknown
	}
}

func makeConfig(enabled bool, provider string) *config.Config {
	cfg := &config.Config{}
	cfg.Providers.ScriptRuntime.Enabled = enabled
	cfg.Providers.ScriptRuntime.Provider = provider
	cfg.Providers.ScriptRuntime.Node.BinaryPath = ""
	cfg.Providers.ScriptRuntime.Node.NPMPath = ""
	cfg.Providers.ScriptRuntime.Node.NPXPath = ""
	cfg.Providers.ScriptRuntime.Node.WorkDir = ""
	return cfg
}

func TestResolverReturnsDisabledWithoutScanning(t *testing.T) {
	cfg := makeConfig(false, "builtin.node-process")
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", "/run", "/ws", nil)
	inspector := newFakeInspector(nil)
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = resolver.Resolve(context.Background())
	if !errors.Is(err, ErrScriptRuntimeDisabled) {
		t.Fatalf("expected ErrScriptRuntimeDisabled, got %v", err)
	}
	if inspector.calls() != 0 {
		t.Errorf("expected 0 stat calls, got %d", inspector.calls())
	}
}

func TestResolverRejectsNonNodeProviderWithoutScanning(t *testing.T) {
	cfg := makeConfig(true, "builtin.wasm")
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", "/run", "/ws", nil)
	inspector := newFakeInspector(nil)
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = resolver.Resolve(context.Background())
	if !errors.Is(err, ErrNodeProviderNotSelected) {
		t.Fatalf("expected ErrNodeProviderNotSelected, got %v", err)
	}
	if inspector.calls() != 0 {
		t.Errorf("expected 0 stat calls, got %d", inspector.calls())
	}
}

func TestResolverRejectsUnsupportedHostCapabilitiesWithoutScanning(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", "/run", "/ws", &runtimehost.HostCapabilities{})
	inspector := newFakeInspector(nil)
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = resolver.Resolve(context.Background())
	if !errors.Is(err, ErrHostCapabilityUnsupported) {
		t.Fatalf("expected ErrHostCapabilityUnsupported, got %v", err)
	}
	if inspector.calls() != 0 {
		t.Errorf("expected 0 stat calls, got %d", inspector.calls())
	}
}

func TestAndroidProotUsesLinuxGuestCandidates(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	host := makeFakeHost(platform.GuestPlatformLinux, "arm64", "/data/data", "/data/data", nil)
	inspector := newFakeInspector(map[string]os.FileInfo{
		filepath.Join("/data/data", "node", "bin", "node"): fakeFileInfo{mode: 0755},
		filepath.Join("/data/data", "node"):                fakeFileInfo{mode: 0755, dir: true},
	})
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Guest != platform.GuestPlatformLinux {
		t.Errorf("expected linux guest, got %s", env.Guest)
	}
	if filepath.Base(env.NodeBinary) != "node" {
		t.Errorf("expected linux node binary 'node', got %q", filepath.Base(env.NodeBinary))
	}
}

func TestEmbeddedAndroidGuestIsUnsupported(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	host := makeFakeHost(platform.GuestPlatformAndroid, "arm64", "/data", "/data", nil)
	inspector := newFakeInspector(nil)
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = resolver.Resolve(context.Background())
	if !errors.Is(err, ErrUnsupportedGuestPlatform) {
		t.Fatalf("expected ErrUnsupportedGuestPlatform, got %v", err)
	}
	if inspector.calls() != 0 {
		t.Errorf("expected 0 stat calls, got %d", inspector.calls())
	}
}

func TestIOSGuestIsUnsupported(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	host := makeFakeHost(platform.GuestPlatformIOS, "arm64", "/data", "/data", nil)
	inspector := newFakeInspector(nil)
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = resolver.Resolve(context.Background())
	if !errors.Is(err, ErrUnsupportedGuestPlatform) {
		t.Fatalf("expected ErrUnsupportedGuestPlatform, got %v", err)
	}
}

func TestExplicitNodePathHasHighestPriority(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	explicitPath := filepath.Join(t.TempDir(), "my-node")
	cfg.Providers.ScriptRuntime.Node.BinaryPath = explicitPath
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", "/tmp/run", "/tmp/run", nil)
	inspector := newFakeInspector(map[string]os.FileInfo{
		explicitPath:               fakeFileInfo{mode: 0755},
		filepath.Dir(explicitPath): fakeFileInfo{mode: 0755, dir: true},
	})
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.NodeBinary != explicitPath {
		t.Errorf("expected node binary %s, got %s", explicitPath, env.NodeBinary)
	}
	if env.Source != SourceExplicit {
		t.Errorf("expected source explicit, got %s", env.Source)
	}
}

func TestInvalidExplicitNodePathDoesNotFallback(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	missingPath := filepath.Join(t.TempDir(), "missing-node")
	cfg.Providers.ScriptRuntime.Node.BinaryPath = missingPath
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", "/run", "/ws", nil)
	inspector := newFakeInspector(nil)
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = resolver.Resolve(context.Background())
	if err == nil {
		t.Fatal("expected error for missing explicit node path")
	}
	if !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestRelativeExplicitPathsResolveAgainstRuntimeRoot(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	cfg.Providers.ScriptRuntime.Node.BinaryPath = "node/bin/node"
	dir := t.TempDir()
	expected := filepath.Join(dir, "node", "bin", "node")
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", dir, dir, nil)
	inspector := newFakeInspector(map[string]os.FileInfo{
		expected:                   fakeFileInfo{mode: 0755},
		filepath.Join(dir, "node"): fakeFileInfo{mode: 0755, dir: true},
	})
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.NodeBinary != expected {
		t.Errorf("expected %s, got %s", expected, env.NodeBinary)
	}
}

func TestRelativeExplicitPathRequiresRuntimeRoot(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	cfg.Providers.ScriptRuntime.Node.BinaryPath = "node/bin/node"
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", "", "", nil)
	inspector := newFakeInspector(nil)
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = resolver.Resolve(context.Background())
	if !errors.Is(err, ErrRuntimeRootUnavailable) {
		t.Fatalf("expected ErrRuntimeRootUnavailable, got %v", err)
	}
}

func TestResolverRejectsNPMCommandWrapper(t *testing.T) {
	exts := []string{".cmd", ".bat", ".ps1", ".sh"}
	for _, ext := range exts {
		cfg := makeConfig(true, "builtin.node-process")
		runDir := t.TempDir()
		nodeBin := filepath.Join(runDir, "node")
		wrapper := filepath.Join(runDir, "npm"+ext)
		cfg.Providers.ScriptRuntime.Node.BinaryPath = nodeBin
		cfg.Providers.ScriptRuntime.Node.NPMPath = wrapper
		host := makeFakeHost(platform.GuestPlatformLinux, "amd64", runDir, runDir, nil)
		inspector := newFakeInspector(map[string]os.FileInfo{
			nodeBin: fakeFileInfo{mode: 0755},
			wrapper: fakeFileInfo{mode: 0755},
			runDir:  fakeFileInfo{mode: 0755, dir: true},
		})
		resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, err = resolver.Resolve(context.Background())
		if !errors.Is(err, ErrShellWrapperUnsupported) {
			t.Fatalf("ext=%s: expected ErrShellWrapperUnsupported, got %v", ext, err)
		}
	}
}

func TestUnixNodeRequiresExecutablePermission(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", "/run", "/ws", nil)
	nonExecPath := filepath.Join("/run", "node", "bin", "node")
	inspector := newFakeInspector(map[string]os.FileInfo{
		nonExecPath: fakeFileInfo{mode: 0644},
	})
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = resolver.Resolve(context.Background())
	if err == nil || !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("expected ErrNodeNotFound due to non-executable, got %v", err)
	}
}

func TestWindowsNodeDoesNotRequireUnixExecutableBits(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	host := makeFakeHost(platform.GuestPlatformWindows, "amd64", "/run", "/ws", nil)
	nodeExe := filepath.Join("/run", "node", "node.exe")
	inspector := newFakeInspector(map[string]os.FileInfo{
		nodeExe:                       fakeFileInfo{mode: 0644},
		filepath.Join("/run", "node"): fakeFileInfo{mode: 0755, dir: true},
	})
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.NodeBinary != nodeExe {
		t.Errorf("expected %s, got %s", nodeExe, env.NodeBinary)
	}
}

func TestNodeBinaryCannotBeDirectory(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", "/run", "/ws", nil)
	dirPath := filepath.Join("/run", "node")
	inspector := newFakeInspector(map[string]os.FileInfo{
		dirPath: fakeFileInfo{mode: 0755, dir: true},
	})
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = resolver.Resolve(context.Background())
	if err == nil || !errors.Is(err, errNodeNotFoundOrInvalid(err)) {
		t.Fatalf("expected error for directory node path, got %v", err)
	}
}

func errNodeNotFoundOrInvalid(err error) error {
	if errors.Is(err, ErrNodeNotFound) || errors.Is(err, ErrInvalidNodeBinary) {
		return err
	}
	return nil
}

func TestMissingAutomaticPackageManagerReturnsPartialEnvironment(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", "/run", "/ws", nil)
	nodeBin := filepath.Join("/run", "node", "bin", "node")
	inspector := newFakeInspector(map[string]os.FileInfo{
		nodeBin:                       fakeFileInfo{mode: 0755},
		filepath.Join("/run", "node"): fakeFileInfo{mode: 0755, dir: true},
	})
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.NodeBinary != nodeBin {
		t.Errorf("expected node binary %s, got %s", nodeBin, env.NodeBinary)
	}
	if env.PackageManagementAvailable {
		t.Error("expected package management unavailable")
	}
	snap := resolver.Snapshot()
	if snap.State != DetectionStatePartial {
		t.Errorf("expected partial state, got %s", snap.State)
	}
}

func TestInvalidExplicitNPMPathFailsDetection(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	missingNpm := filepath.Join(t.TempDir(), "missing-npm.js")
	cfg.Providers.ScriptRuntime.Node.NPMPath = missingNpm
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", "/run", "/ws", nil)
	nodeBin := filepath.Join("/run", "node", "bin", "node")
	inspector := newFakeInspector(map[string]os.FileInfo{
		nodeBin:                       fakeFileInfo{mode: 0755},
		filepath.Join("/run", "node"): fakeFileInfo{mode: 0755, dir: true},
	})
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = resolver.Resolve(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid explicit npm path")
	}
	snap := resolver.Snapshot()
	if snap.State != DetectionStateFailed {
		t.Errorf("expected failed state, got %s", snap.State)
	}
}

func TestResolverFindsPackageManagerCLIScripts(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", "/run", "/ws", nil)
	nodeBin := filepath.Join("/run", "node", "bin", "node")
	npmCli := filepath.Join("/run", "node", "lib", "node_modules", "npm", "bin", "npm-cli.js")
	npxCli := filepath.Join("/run", "node", "lib", "node_modules", "npm", "bin", "npx-cli.js")
	inspector := newFakeInspector(map[string]os.FileInfo{
		nodeBin:                       fakeFileInfo{mode: 0755},
		npmCli:                        fakeFileInfo{mode: 0644},
		npxCli:                        fakeFileInfo{mode: 0644},
		filepath.Join("/run", "node"): fakeFileInfo{mode: 0755, dir: true},
	})
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.NPMCLI != npmCli {
		t.Errorf("expected npm CLI %s, got %s", npmCli, env.NPMCLI)
	}
	if env.NPXCLI != npxCli {
		t.Errorf("expected npx CLI %s, got %s", npxCli, env.NPXCLI)
	}
	if !env.PackageManagementAvailable {
		t.Error("expected package management available")
	}
}

func TestDefaultWorkDirUsesDistributionRoot(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", "/run", "/ws", nil)
	nodeBin := filepath.Join("/run", "node", "bin", "node")
	npmCli := filepath.Join("/run", "node", "lib", "node_modules", "npm", "bin", "npm-cli.js")
	npxCli := filepath.Join("/run", "node", "lib", "node_modules", "npm", "bin", "npx-cli.js")
	inspector := newFakeInspector(map[string]os.FileInfo{
		nodeBin:                       fakeFileInfo{mode: 0755},
		npmCli:                        fakeFileInfo{mode: 0644},
		npxCli:                        fakeFileInfo{mode: 0644},
		filepath.Join("/run", "node"): fakeFileInfo{mode: 0755, dir: true},
	})
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.WorkDir != env.DistributionRoot {
		t.Errorf("expected work dir to equal distribution root %s, got %s", env.DistributionRoot, env.WorkDir)
	}
}

func TestExplicitWorkDirIsAuthoritative(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	workDir := filepath.Join(t.TempDir(), "workdir")
	cfg.Providers.ScriptRuntime.Node.WorkDir = workDir
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", "/run", "/ws", nil)
	nodeBin := filepath.Join("/run", "node", "bin", "node")
	inspector := newFakeInspector(map[string]os.FileInfo{
		nodeBin: fakeFileInfo{mode: 0755},
		workDir: fakeFileInfo{mode: 0755, dir: true},
	})
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.WorkDir != workDir {
		t.Errorf("expected work dir %s, got %s", workDir, env.WorkDir)
	}
}

func TestResolverCachesSuccessfulDetection(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", "/run", "/ws", nil)
	nodeBin := filepath.Join("/run", "node", "bin", "node")
	inspector := newFakeInspector(map[string]os.FileInfo{
		nodeBin:                       fakeFileInfo{mode: 0755},
		filepath.Join("/run", "node"): fakeFileInfo{mode: 0755, dir: true},
	})
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := resolver.Resolve(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	firstCalls := inspector.calls()
	for i := 0; i < 3; i++ {
		if _, err := resolver.Resolve(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if inspector.calls() != firstCalls {
		t.Errorf("expected stat calls to be unchanged (%d), got %d", firstCalls, inspector.calls())
	}
}

func TestResolverCachesFailedDetection(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", "", "", nil)
	inspector := newFakeInspector(nil)
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 0; i < 3; i++ {
		resolver.Resolve(context.Background())
	}
	callsAfter := inspector.calls()
	if callsAfter > 3 {
		t.Errorf("expected minimal repeated stat calls, got %d", callsAfter)
	}
}

func TestResolverConcurrentResolveRunsDetectionOnce(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", "/run", "/ws", nil)
	nodeBin := filepath.Join("/run", "node", "bin", "node")
	inspector := newFakeInspector(map[string]os.FileInfo{
		nodeBin:                       fakeFileInfo{mode: 0755},
		filepath.Join("/run", "node"): fakeFileInfo{mode: 0755, dir: true},
	})
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var wg sync.WaitGroup
	const goroutines = 16
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = resolver.Resolve(context.Background())
		}()
	}
	wg.Wait()
	snap := resolver.Snapshot()
	if snap.State != DetectionStatePartial {
		t.Errorf("expected partial state, got %s", snap.State)
	}
}

func TestDetectionSnapshotDoesNotExposeMutableState(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", "/run", "/ws", nil)
	nodeBin := filepath.Join("/run", "node", "bin", "node")
	inspector := newFakeInspector(map[string]os.FileInfo{
		nodeBin:                       fakeFileInfo{mode: 0755},
		filepath.Join("/run", "node"): fakeFileInfo{mode: 0755, dir: true},
	})
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := resolver.Resolve(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	snap1 := resolver.Snapshot()
	if len(snap1.Diagnostics) == 0 {
		t.Fatal("expected diagnostics in snapshot")
	}
	snap1.Diagnostics = append(snap1.Diagnostics, CandidateDiagnostic{
		Kind:   "tampered",
		Source: "tampered",
		Result: "tampered",
	})
	snap2 := resolver.Snapshot()
	if len(snap2.Diagnostics) != len(snap1.Diagnostics)-1 {
		t.Error("snapshot diagnostics appear to share underlying state")
	}
}

func TestResolverHonorsContextCancellation(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", "/run", "/ws", nil)
	inspector := newFakeInspector(nil)
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = resolver.Resolve(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestResolverNeverUsesSystemPath(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", "", "", nil)
	inspector := newFakeInspector(map[string]os.FileInfo{
		"/usr/local/bin/node": fakeFileInfo{mode: 0755},
	})
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = resolver.Resolve(context.Background())
	if err == nil {
		t.Fatal("expected error since root/workspace are empty")
	}
	if inspector.calls() != 0 {
		t.Errorf("expected 0 stat calls with no valid candidates, got %d", inspector.calls())
	}
}

func TestRuntimePackagePrecedesLegacyBundled(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	runDir := t.TempDir()
	runtimePkgNode := filepath.Join(runDir, "node", "bin", "node")
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", runDir, runDir, nil)
	inspector := newFakeInspector(map[string]os.FileInfo{
		runtimePkgNode:                fakeFileInfo{mode: 0755},
		filepath.Join(runDir, "node"): fakeFileInfo{mode: 0755, dir: true},
	})
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Source != SourceRuntimePackage && env.Source != SourceLegacyBundled {
		t.Errorf("expected runtime-package or legacy-bundled source, got %s (binary=%s)", env.Source, env.NodeBinary)
	}
}

func TestLegacyLinuxNodeCandidatesIntegration(t *testing.T) {
	cfg := makeConfig(true, "builtin.node-process")
	host := makeFakeHost(platform.GuestPlatformLinux, "amd64", "/run", "/run", nil)
	legacyBinNode := filepath.Join("/run", "backend", "node", "node")
	inspector := newFakeInspector(map[string]os.FileInfo{
		legacyBinNode:                            fakeFileInfo{mode: 0755},
		filepath.Join("/run", "backend", "node"): fakeFileInfo{mode: 0755, dir: true},
	})
	resolver, err := NewResolver(ResolveContext{Config: cfg, Host: host, Inspector: inspector})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Source != SourceLegacyBundled {
		t.Errorf("expected legacy-bundled, got %s", env.Source)
	}
}
