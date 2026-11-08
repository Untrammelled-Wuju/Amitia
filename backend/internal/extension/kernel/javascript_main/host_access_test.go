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
		"/runtime/plugin-host/dist/index.js",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing node launch argument %q in %#v", expected, args)
		}
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
