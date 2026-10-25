package javascript_main

import (
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/runtime"
)

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
