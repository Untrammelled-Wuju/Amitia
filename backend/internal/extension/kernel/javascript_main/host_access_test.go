package javascript_main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/runtime"
)

func TestNodeLaunchArgsUseCanonicalEntrypoints(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"host", "plugin"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o700); err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, filepath.Join(root, dir, "index.js"), "")
	}
	host, err := NewPluginHost(PluginHostConfig{
		InstanceID: "instance", ExtensionID: "extension", ModuleID: "module",
		NodePath: filepath.Join(root, "node"), WorkingDirectory: root,
		PluginHostPath: filepath.Join(alias, "host", "index.js"),
		BootstrapSpec:  runtime.BootstrapSpec{Entry: filepath.Join(alias, "plugin", "index.js")},
	})
	if err != nil {
		t.Fatal(err)
	}
	args := host.nodeLaunchArgs()
	want := []string{"--permission", "--allow-fs-read=" + filepath.Join(root, "host"), "--allow-fs-read=" + filepath.Join(root, "plugin"), filepath.Join(root, "host", "index.js")}
	if strings.Join(args, "\n") != strings.Join(want, "\n") || host.spec.Entry != filepath.Join(root, "plugin", "index.js") {
		t.Fatalf("unexpected launch arguments %q and entry %q", args, host.spec.Entry)
	}
}

func TestRelativePluginEntryIsResolvedInsideWorkingDirectory(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	hostPath := filepath.Join(root, "host", "index.js")
	entryPath := filepath.Join(root, "dist", "index.js")
	if err := os.MkdirAll(filepath.Dir(hostPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "package.json"), `{"type":"module"}`)
	writeTestFile(t, hostPath, "")
	writeTestFile(t, entryPath, "")
	host, err := NewPluginHost(PluginHostConfig{
		InstanceID: "instance", ExtensionID: "extension", ModuleID: "module",
		NodePath: filepath.Join(root, "node"), WorkingDirectory: root,
		PluginHostPath: hostPath,
		BootstrapSpec:  runtime.BootstrapSpec{Entry: filepath.Join("dist", "index.js")},
	})
	if err != nil {
		t.Fatal(err)
	}
	canonicalEntryPath, err := filepath.EvalSymlinks(entryPath)
	if err != nil {
		t.Fatal(err)
	}
	if host.spec.Entry != canonicalEntryPath {
		t.Fatalf("relative entry resolved to %q, want %q", host.spec.Entry, canonicalEntryPath)
	}
	if !strings.Contains(strings.Join(host.nodeLaunchArgs(), "\n"), "--allow-fs-read="+filepath.Dir(canonicalEntryPath)) {
		t.Fatalf("resolved entry directory was not granted read access: %#v", host.nodeLaunchArgs())
	}
	if !strings.Contains(strings.Join(host.nodeLaunchArgs(), "\n"), "--allow-fs-read="+root) {
		t.Fatalf("resolved module package root was not granted read access: %#v", host.nodeLaunchArgs())
	}
}

func TestNodeLaunchArgsApplyHostAccess(t *testing.T) {
	host := &PluginHost{
		nodePath:       "/runtime/node",
		pluginHostPath: "/runtime/plugin-host/dist/index.js",
		spec: runtime.BootstrapSpec{
			Entry: "/plugins/runtime/index.mjs",
		},
		hostAccess: PluginHostAccess{
			FilesystemMode:  "read_write",
			FilesystemRoots: []string{"/plugins/package", "/plugins/data"},
			NetworkMode:     "host",
			ProcessMode:     "direct",
		},
	}
	args := host.nodeLaunchArgs()
	joined := strings.Join(args, "\n")
	for _, expected := range []string{
		"--permission",
		"--allow-fs-read=/runtime/plugin-host/dist",
		"--allow-fs-read=/plugins/runtime",
		"--allow-fs-read=/plugins/package",
		"--allow-fs-write=/plugins/package",
		"--allow-child-process",
		"--allow-net",
		"/runtime/plugin-host/dist/index.js",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing node launch argument %q in %#v", expected, args)
		}
	}
}

func TestNodeLaunchArgsSkipPermissionModelForDurableSyncRuntimes(t *testing.T) {
	host := &PluginHost{
		nodePath:       "/runtime/node",
		pluginHostPath: "/runtime/plugin-host/dist/index.js",
		spec: runtime.BootstrapSpec{
			Entry: "/plugins/runtime/index.mjs",
		},
		hostAccess: PluginHostAccess{
			FilesystemMode:        "read_write",
			FilesystemRoots:       []string{"/plugins/package", "/plugins/data"},
			FilesystemDurableSync: true,
			NetworkMode:           "host",
			ProcessMode:           "direct",
		},
	}
	args := host.nodeLaunchArgs()
	if len(args) != 1 || args[0] != "/runtime/plugin-host/dist/index.js" {
		t.Fatalf("durable sync runtime must run without the permission model, got %#v", args)
	}
}

func TestNodeLaunchArgsDenyHostAccess(t *testing.T) {
	host := &PluginHost{
		pluginHostPath: "/runtime/plugin-host/dist/index.js",
		spec: runtime.BootstrapSpec{
			Entry: "/plugins/runtime/index.mjs",
		},
		hostAccess: PluginHostAccess{
			FilesystemMode: "none",
			NetworkMode:    "none",
			ProcessMode:    "none",
		},
	}
	joined := strings.Join(host.nodeLaunchArgs(), "\n")
	if strings.Contains(joined, "--allow-child-process") || strings.Contains(joined, "--allow-fs-write=") {
		t.Fatalf("unexpected host access in %#v", joined)
	}
}

func TestNodeLaunchArgsAllowNodeDistributionForDirectProcesses(t *testing.T) {
	root := t.TempDir()
	nodePath := filepath.Join(root, "node", "bin", "node")
	npmCli := filepath.Join(root, "node", "lib", "node_modules", "npm", "bin", "npm-cli.js")
	if err := os.MkdirAll(filepath.Dir(nodePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(npmCli), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, nodePath, "")
	writeTestFile(t, npmCli, "")
	host := &PluginHost{
		nodePath:       nodePath,
		pluginHostPath: filepath.Join(root, "plugin-host", "index.js"),
		hostAccess: PluginHostAccess{
			ProcessMode: "direct",
		},
	}
	joined := strings.Join(host.nodeLaunchArgs(), "\n")
	if !strings.Contains(joined, "--allow-fs-read="+filepath.Join(root, "node")) {
		t.Fatalf("missing node distribution read access in %#v", joined)
	}
	if !strings.Contains(joined, "--allow-child-process") {
		t.Fatalf("missing child process access in %#v", joined)
	}
}
