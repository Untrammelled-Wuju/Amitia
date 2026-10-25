package kernel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func TestBuildServiceModuleRuntimeDefinitionAppliesHostAccess(t *testing.T) {
	root := t.TempDir()
	bundleRoot := filepath.Join(root, "bundle")
	moduleRoot := filepath.Join(bundleRoot, "modules", "service")
	if err := os.MkdirAll(moduleRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moduleRoot, "launcher.mjs"), []byte("export {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	nodePath := filepath.Join(root, "node")
	if err := os.WriteFile(nodePath, []byte("node"), 0o755); err != nil {
		t.Fatal(err)
	}
	pluginData := filepath.Join(root, "plugin-data")
	definition, err := buildServiceModuleRuntimeDefinition(
		domain.ExtensionDefinition{Publisher: domain.PublisherReference{TrustLevel: "official"}},
		domain.ModuleDefinition{
			ID:   "service",
			Name: domain.LocalizedText{Default: "Service"},
			Runtime: &domain.RuntimeDefinition{
				Type:       domain.RuntimeTypeService,
				EntryPoint: "launcher.mjs",
				HostAccess: domain.RuntimeHostAccess{
					Filesystem: domain.RuntimeFilesystemAccess{Mode: "read_write", Roots: []string{"package", "plugin_data"}},
					Network:    domain.RuntimeNetworkAccess{Mode: "host"},
					Process:    domain.RuntimeProcessAccess{Mode: "direct", MaxProcesses: 1},
				},
			},
		},
		bundleRoot,
		pluginData,
		nodePath,
	)
	if err != nil {
		t.Fatal(err)
	}
	if definition.Network.Mode != "host" || definition.Network.Enforce {
		t.Fatalf("unexpected network policy: %#v", definition.Network)
	}
	if definition.Limits.MaxSubprocesses != 1 {
		t.Fatalf("unexpected subprocess limit: %d", definition.Limits.MaxSubprocesses)
	}
	env := definition.Executables[0].EnvTemplate
	if env["AMITIA_HOST_FILESYSTEM_MODE"] != "read_write" || env["AMITIA_HOST_NETWORK_MODE"] != "host" || env["AMITIA_HOST_PROCESS_MODE"] != "direct" {
		t.Fatalf("unexpected host access environment: %#v", env)
	}
	if env["AMITIA_HOST_PROCESS_LIMIT"] != "1" {
		t.Fatalf("unexpected process limit environment: %q", env["AMITIA_HOST_PROCESS_LIMIT"])
	}
	roots := strings.Split(env["AMITIA_HOST_FILESYSTEM_ROOTS"], string(os.PathListSeparator))
	if len(roots) != 2 || roots[0] != bundleRoot || roots[1] != pluginData {
		t.Fatalf("unexpected filesystem roots: %#v", roots)
	}
	var rootMap map[string]string
	if err := json.Unmarshal([]byte(env["AMITIA_HOST_FILESYSTEM_ROOTS_JSON"]), &rootMap); err != nil {
		t.Fatal(err)
	}
	if rootMap["package"] != bundleRoot || rootMap["plugin_data"] != pluginData {
		t.Fatalf("unexpected filesystem root map: %#v", rootMap)
	}
}
