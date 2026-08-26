package manifest_v2

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGamePluginContributionManifest(t *testing.T) {
	manifest := `{
		"manifestVersion": 2,
		"extension": {
			"id": "com.example/game-plugin",
			"name": {"default": "Example Game Plugin"},
			"version": "1.0.0"
		},
		"publisher": {
			"id": "com.example",
			"displayName": "Example"
		},
		"modules": [
			{
				"id": "game-runtime",
				"name": {"default": "Game Runtime"},
				"type": "service",
				"runtime": {
					"type": "service",
					"entryPoint": "bin/game-service"
				},
				"contributions": [
					{
						"id": "example-game",
						"kind": "game_plugin",
						"name": {"default": "Example Game"},
						"spec": {
							"protocolVersion": "amitia-game-host/1",
							"runtimeModuleId": "game-runtime", "network": {"mode": "none"}
						}
					}
				]
			}
		],
		"integrity": {
			"algorithm": "sha256",
			"contentTreeHash": "test"
		}
	}`
	m, err := Parse([]byte(manifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	report := m.Validate()
	if report.HasErrors() {
		t.Errorf("expected game_plugin contribution to pass Validate(), got errors: %v", report.Errors)
	}
}

func TestGamePluginContributionSchema(t *testing.T) {
	manifest := `{
		"manifestVersion": 2,
		"extension": {
			"id": "com.example/game-schema",
			"name": {"default": "Example Game Plugin"},
			"version": "1.0.0"
		},
		"publisher": {
			"id": "com.example",
			"displayName": "Example"
		},
		"modules": [
			{
				"id": "game-runtime",
				"name": {"default": "Game Runtime"},
				"type": "service",
				"runtime": {"type": "service", "entryPoint": "bin/game"},
				"contributions": [
					{
						"id": "example-game",
						"kind": "game_plugin",
						"name": {"default": "Example Game"},
						"spec": {
							"protocolVersion": "amitia-game-host/1",
							"runtimeModuleId": "game-runtime", "network": {"mode": "none"}
						}
					}
				]
			}
		],
		"integrity": {
			"algorithm": "sha256",
			"contentTreeHash": "test"
		}
	}`
	m, err := Parse([]byte(manifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	report := m.ValidateWithSchema()
	if report.HasErrors() {
		t.Errorf("expected game_plugin contribution to pass ValidateWithSchema(), got errors: %v", report.Errors)
	}
}

func TestGamePluginRequiresID(t *testing.T) {
	manifest := `{
		"manifestVersion": 2,
		"extension": {
			"id": "com.example/game-test",
			"name": {"default": "Game"},
			"version": "1.0.0"
		},
		"publisher": {"id": "com.example", "displayName": "Example"},
		"modules": [
			{
				"id": "main",
				"name": {"default": "Main"},
				"type": "javascript",
				"contributions": [
					{
						"kind": "game_plugin",
						"name": {"default": "Game"},
						"spec": {"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "main", "network": {"mode": "none"}}
					}
				]
			}
		],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "test"}
	}`
	m, err := Parse([]byte(manifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	report := m.Validate()
	if !report.HasErrors() {
		t.Errorf("expected game_plugin without id to be rejected")
	}
}

func TestGamePluginRequiresProtocolVersion(t *testing.T) {
	manifest := `{
		"manifestVersion": 2,
		"extension": {
			"id": "com.example/game-test",
			"name": {"default": "Game"},
			"version": "1.0.0"
		},
		"publisher": {"id": "com.example", "displayName": "Example"},
		"modules": [
			{
				"id": "main",
				"name": {"default": "Main"},
				"type": "javascript",
				"contributions": [
					{
						"id": "game-1",
						"kind": "game_plugin",
						"name": {"default": "Game"}
					}
				]
			}
		],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "test"}
	}`
	m, err := Parse([]byte(manifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	report := m.Validate()
	if !report.HasErrors() {
		t.Errorf("expected game_plugin without protocolVersion to be rejected")
	}
}

func TestGamePluginRuntimeModuleMustExist(t *testing.T) {
	manifest := `{
		"manifestVersion": 2,
		"extension": {
			"id": "com.example/game-test",
			"name": {"default": "Game"},
			"version": "1.0.0"
		},
		"publisher": {"id": "com.example", "displayName": "Example"},
		"modules": [
			{
				"id": "main",
				"name": {"default": "Main"},
				"type": "javascript",
				"contributions": [
					{
						"id": "game-1",
						"kind": "game_plugin",
						"name": {"default": "Game"},
						"spec": {
							"protocolVersion": "amitia-game-host/1",
							"runtimeModuleId": "nonexistent-module", "network": {"mode": "none"}
						}
					}
				]
			}
		],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "test"}
	}`
	m, err := Parse([]byte(manifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	report := m.Validate()
	if !report.HasErrors() {
		t.Errorf("expected game_plugin referencing nonexistent module to be rejected")
	}
}

func TestDuplicateGamePluginIDRejected(t *testing.T) {
	manifest := `{
		"manifestVersion": 2,
		"extension": {
			"id": "com.example/game-test",
			"name": {"default": "Game"},
			"version": "1.0.0"
		},
		"publisher": {"id": "com.example", "displayName": "Example"},
		"modules": [
			{
				"id": "main",
				"name": {"default": "Main"},
				"type": "javascript",
				"contributions": [
					{
						"id": "game-1",
						"kind": "game_plugin",
						"name": {"default": "Game 1"},
						"spec": {"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "main", "network": {"mode": "none"}}
					},
					{
						"id": "game-1",
						"kind": "game_plugin",
						"name": {"default": "Game 2"},
						"spec": {"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "main", "network": {"mode": "none"}}
					}
				]
			}
		],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "test"}
	}`
	m, err := Parse([]byte(manifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	report := m.Validate()
	if !report.HasErrors() {
		t.Errorf("expected duplicate game_plugin ID to be rejected")
	}
}

func TestMultipleGamePluginsAllowed(t *testing.T) {
	manifest := `{
		"manifestVersion": 2,
		"extension": {
			"id": "com.example/game-test",
			"name": {"default": "Game"},
			"version": "1.0.0"
		},
		"publisher": {"id": "com.example", "displayName": "Example"},
		"modules": [
			{
				"id": "main",
				"name": {"default": "Main"},
				"type": "javascript",
				"runtime": {"type": "javascript", "entryPoint": "dist/index.js"},
				"contributions": [
					{
						"id": "plugin-a",
						"kind": "game_plugin",
						"name": {"default": "Plugin A"},
						"spec": {"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "main", "network": {"mode": "none"}}
					},
					{
						"id": "plugin-b",
						"kind": "game_plugin",
						"name": {"default": "Plugin B"},
						"spec": {"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "main", "network": {"mode": "none"}}
					}
				]
			}
		],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "test"}
	}`
	m, err := Parse([]byte(manifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	report := m.Validate()
	if report.HasErrors() {
		t.Errorf("expected multiple game_plugin contributions to pass, got errors: %v", report.Errors)
	}
}

func TestGamePluginPreservedInExtensionDefinition(t *testing.T) {
	manifest := `{
		"manifestVersion": 2,
		"extension": {
			"id": "com.example/game-test",
			"name": {"default": "Game"},
			"version": "1.0.0"
		},
		"publisher": {"id": "com.example", "displayName": "Example"},
		"modules": [
			{
				"id": "main",
				"name": {"default": "Main"},
				"type": "javascript",
				"runtime": {"type": "javascript", "entryPoint": "dist/index.js"},
				"contributions": [
					{
						"id": "game-1",
						"kind": "game_plugin",
						"name": {"default": "Game"},
						"spec": {"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "main", "network": {"mode": "none"}}
					}
				]
			}
		],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "test"}
	}`
	m, err := Parse([]byte(manifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	def, err := m.ToExtensionDefinition()
	if err != nil {
		t.Fatalf("ToExtensionDefinition: %v", err)
	}
	if len(def.Modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(def.Modules))
	}
	if len(def.Modules[0].Contributions) != 1 {
		t.Fatalf("expected 1 contribution, got %d", len(def.Modules[0].Contributions))
	}
	if string(def.Modules[0].Contributions[0].Kind) != "game_plugin" {
		t.Errorf("expected contribution kind game_plugin, got %s", def.Modules[0].Contributions[0].Kind)
	}
}

func TestGamePluginJSONRoundTrip(t *testing.T) {
	manifest := `{
		"manifestVersion": 2,
		"extension": {
			"id": "com.example/game-test",
			"name": {"default": "Game"},
			"version": "1.0.0"
		},
		"publisher": {"id": "com.example", "displayName": "Example"},
		"modules": [
			{
				"id": "main",
				"name": {"default": "Main"},
				"type": "javascript",
				"contributions": [
					{
						"id": "game-1",
						"kind": "game_plugin",
						"name": {"default": "Game"},
						"spec": {
							"protocolVersion": "amitia-game-host/1",
							"runtimeModuleId": "main", "network": {"mode": "none"}
						}
					}
				]
			}
		],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "test"}
	}`
	m, err := Parse([]byte(manifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"game_plugin"`) {
		t.Errorf("expected game_plugin in JSON output, got: %s", string(data))
	}
}

func TestDomainMappingGamePlugin(t *testing.T) {
	manifest := `{
		"manifestVersion": 2,
		"extension": {
			"id": "com.example/game-test",
			"name": {"default": "Game"},
			"version": "1.0.0"
		},
		"publisher": {"id": "com.example", "displayName": "Example"},
		"modules": [
			{
				"id": "main",
				"name": {"default": "Main"},
				"type": "javascript",
				"runtime": {"type": "javascript", "entryPoint": "dist/index.js"},
				"contributions": [
					{
						"id": "game-1",
						"kind": "game_plugin",
						"name": {"default": "Game"},
						"spec": {"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "main", "network": {"mode": "none"}}
					}
				]
			}
		],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "test"}
	}`
	m, err := Parse([]byte(manifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	report := m.Validate()
	if report.HasErrors() {
		t.Fatalf("Validate errors: %v", report.Errors)
	}
	if m.Modules[0].Contributions[0].Kind != "game_plugin" {
		t.Errorf("expected game_plugin kind, got %s", m.Modules[0].Contributions[0].Kind)
	}
}

func TestGamePluginUnrestrictedNetworkRequiresPermission(t *testing.T) {
	manifest := `{
		"manifestVersion": 2,
		"extension": {"id": "com.example/network-game", "name": {"default": "Network Game"}, "version": "1.0.0"},
		"publisher": {"id": "com.example", "displayName": "Example"},
		"modules": [{
			"id": "runtime",
			"name": {"default": "Runtime"},
			"type": "service",
			"runtime": {"type": "service", "entryPoint": "bin/runtime"},
			"contributions": [{
				"id": "game",
				"kind": "game_plugin",
				"name": {"default": "Game"},
				"spec": {"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "runtime", "network": {"mode": "unrestricted"}}
			}]
		}],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "test"}
	}`
	m, err := Parse([]byte(manifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	report := m.Validate()
	if !report.HasErrors() {
		t.Fatal("expected unrestricted game plugin without service.network.request to be rejected")
	}

	m.Modules[0].Contributions[0].RequiredPermissions = []string{"service.network.request"}
	report = m.Validate()
	if report.HasErrors() {
		t.Fatalf("expected unrestricted game plugin with network permission to pass, got %v", report.Errors)
	}
}

func TestGamePluginRestrictedNetworkRequiresPermission(t *testing.T) {
	manifest := `{
		"manifestVersion": 2,
		"extension": {"id": "com.example/restricted-network-game", "name": {"default": "Restricted Network Game"}, "version": "1.0.0"},
		"publisher": {"id": "com.example", "displayName": "Example"},
		"modules": [{
			"id": "runtime",
			"name": {"default": "Runtime"},
			"type": "service",
			"runtime": {"type": "service", "entryPoint": "bin/runtime"},
			"contributions": [{
				"id": "game",
				"kind": "game_plugin",
				"name": {"default": "Game"},
				"spec": {"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "runtime", "network": {"mode": "restricted", "allowedDomains": ["api.example.com"], "allowedPorts": [443]}}
			}]
		}],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "test"}
	}`
	m, err := Parse([]byte(manifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if report := m.Validate(); !report.HasErrors() {
		t.Fatal("expected restricted game plugin without service.network.request to be rejected")
	}
	m.Modules[0].Contributions[0].RequiredPermissions = []string{"service.network.request"}
	if report := m.Validate(); report.HasErrors() {
		t.Fatalf("expected restricted game plugin with network permission to pass, got %v", report.Errors)
	}
}

func TestNormalizeGamePluginRuntimeDefaultsToDevice(t *testing.T) {
	manifest := `{
		"manifestVersion": 2,
		"extension": {"id": "com.example/device-default", "name": {"default": "Game"}, "version": "1.0.0"},
		"publisher": {"id": "com.example", "displayName": "Example"},
		"modules": [{
			"id": "runtime",
			"name": {"default": "Runtime"},
			"type": "service",
			"runtime": {"type": "service", "entryPoint": "bin/runtime"},
			"contributions": [{
				"id": "game",
				"kind": "game_plugin",
				"name": {"default": "Game"},
				"spec": {"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "runtime", "network": {"mode": "none"}}
			}]
		}],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "test"}
	}`
	m, err := Parse([]byte(manifest))
	if err != nil {
		t.Fatal(err)
	}
	normalized, report := m.NormalizeCompatibility()
	if report.HasErrors() {
		t.Fatalf("normalize errors: %v", report.Errors)
	}
	if normalized.Placement != "device" {
		t.Fatalf("expected extension placement device, got %q", normalized.Placement)
	}
	if got := normalized.Modules[0].Placement; got != "device" {
		t.Fatalf("expected game runtime placement device, got %q", got)
	}
}

func TestGamePluginRuntimeDeclaredLaterIsAllowed(t *testing.T) {
	manifest := `{
		"manifestVersion": 2,
		"extension": {"id": "com.example/forward-ref", "name": {"default": "Game"}, "version": "1.0.0"},
		"publisher": {"id": "com.example", "displayName": "Example"},
		"modules": [
			{
				"id": "metadata",
				"name": {"default": "Metadata"},
				"type": "data_only",
				"contributions": [{
					"id": "game",
					"kind": "game_plugin",
					"name": {"default": "Game"},
					"spec": {"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "runtime", "network": {"mode": "none"}}
				}]
			},
			{
				"id": "runtime",
				"name": {"default": "Runtime"},
				"type": "service",
				"runtime": {"type": "service", "entryPoint": "bin/runtime"}
			}
		],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "test"}
	}`
	m, err := Parse([]byte(manifest))
	if err != nil {
		t.Fatal(err)
	}
	if report := m.Validate(); report.HasErrors() {
		t.Fatalf("forward runtime reference should be order-independent, got %v", report.Errors)
	}
	normalized, report := m.NormalizeCompatibility()
	if report.HasErrors() {
		t.Fatalf("normalize errors: %v", report.Errors)
	}
	if normalized.Modules[1].Placement != "device" {
		t.Fatalf("forward-referenced game runtime must normalize to device, got %q", normalized.Modules[1].Placement)
	}
}

func TestGamePluginRuntimeExplicitCloudPlacementRejected(t *testing.T) {
	manifest := `{
		"manifestVersion": 2,
		"placement": "cloud",
		"extension": {"id": "com.example/cloud-game", "name": {"default": "Game"}, "version": "1.0.0"},
		"publisher": {"id": "com.example", "displayName": "Example"},
		"modules": [{
			"id": "runtime",
			"name": {"default": "Runtime"},
			"type": "service",
			"placement": "cloud",
			"runtime": {"type": "service", "entryPoint": "bin/runtime"},
			"contributions": [{
				"id": "game",
				"kind": "game_plugin",
				"name": {"default": "Game"},
				"spec": {"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "runtime", "network": {"mode": "none"}}
			}]
		}],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "test"}
	}`
	m, err := Parse([]byte(manifest))
	if err != nil {
		t.Fatal(err)
	}
	report := m.Validate()
	if !report.HasErrors() {
		t.Fatal("expected explicit cloud placement for game runtime to be rejected")
	}
	found := false
	for _, item := range report.Errors {
		if strings.Contains(item.Message, "must use placement device") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected device-placement error, got %v", report.Errors)
	}
}

func TestGamePluginCloudExtensionRejectedEvenBeforeNormalization(t *testing.T) {
	manifest := `{
		"manifestVersion": 2,
		"placement": "cloud",
		"extension": {"id": "com.example/cloud-game-default-module", "name": {"default": "Game"}, "version": "1.0.0"},
		"publisher": {"id": "com.example", "displayName": "Example"},
		"modules": [{
			"id": "runtime",
			"name": {"default": "Runtime"},
			"type": "service",
			"runtime": {"type": "service", "entryPoint": "bin/runtime"},
			"contributions": [{
				"id": "game",
				"kind": "game_plugin",
				"name": {"default": "Game"},
				"spec": {"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "runtime", "network": {"mode": "none"}}
			}]
		}],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "test"}
	}`
	m, err := Parse([]byte(manifest))
	if err != nil {
		t.Fatal(err)
	}
	report := m.Validate()
	if !report.HasErrors() {
		t.Fatal("expected cloud extension containing game_plugin to be rejected before compatibility normalization")
	}
	found := false
	for _, item := range report.Errors {
		if strings.Contains(item.Message, "game_plugin extensions cannot use placement cloud") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected cloud-extension rejection, got %v", report.Errors)
	}
}
