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
							"runtimeModuleId": "game-runtime"
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
							"runtimeModuleId": "game-runtime"
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
						"spec": {"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "main"}
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
							"runtimeModuleId": "nonexistent-module"
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
						"spec": {"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "main"}
					},
					{
						"id": "game-1",
						"kind": "game_plugin",
						"name": {"default": "Game 2"},
						"spec": {"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "main"}
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
						"spec": {"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "main"}
					},
					{
						"id": "plugin-b",
						"kind": "game_plugin",
						"name": {"default": "Plugin B"},
						"spec": {"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "main"}
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
						"spec": {"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "main"}
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
							"runtimeModuleId": "main"
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
						"spec": {"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "main"}
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
