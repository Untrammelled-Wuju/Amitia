package manifest_v1

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestManifestAcceptsDesktopPetPluginContribution(t *testing.T) {
	manifest := `{
		"manifestVersion": 1,
		"extension": {
			"id": "com.example/pet-extension",
			"name": {"default": "Pet Extension"},
			"version": "1.0.0"
		},
		"publisher": {"id": "com.example", "displayName": "Example"},
		"modules": [
			{
				"id": "pet-runtime",
				"name": {"default": "Pet Runtime"},
				"type": "service",
				"runtime": {"type": "service", "entryPoint": "bin/pet-service"},
				"contributions": [
					{
						"id": "pet-overlay",
						"kind": "petx",
						"name": {"default": "Pet Overlay"},
						"spec": {"runtimeModuleId": "pet-runtime"}
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
		t.Errorf("expected petx to pass Validate(), got errors: %v", report.Errors)
	}
}

func TestSchemaAcceptsDesktopPetPluginContribution(t *testing.T) {
	manifest := `{
		"manifestVersion": 1,
		"extension": {
			"id": "com.example/pet-schema",
			"name": {"default": "Pet Extension"},
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
						"id": "pet-interaction",
						"kind": "petx",
						"name": {"default": "Pet Interaction"}
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
	report := m.ValidateWithSchema()
	if report.HasErrors() {
		t.Errorf("expected petx to pass ValidateWithSchema(), got errors: %v", report.Errors)
	}
}

func TestPetPluginRequiresID(t *testing.T) {
	manifest := `{
		"manifestVersion": 1,
		"extension": {
			"id": "com.example/pet-test",
			"name": {"default": "Pet"},
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
						"kind": "petx",
						"name": {"default": "Pet Plugin"}
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
		t.Errorf("expected petx without id to be rejected")
	}
}

func TestDesktopPetPluginRuntimeModuleMustExist(t *testing.T) {
	manifest := `{
		"manifestVersion": 1,
		"extension": {
			"id": "com.example/pet-test",
			"name": {"default": "Pet"},
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
						"id": "pet-1",
						"kind": "petx",
						"name": {"default": "Pet"},
						"spec": {"runtimeModuleId": "nonexistent-module"}
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
		t.Errorf("expected petx referencing nonexistent module to be rejected")
	}
}

func TestDesktopPetPluginWithoutRuntimeModule(t *testing.T) {
	manifest := `{
		"manifestVersion": 1,
		"extension": {
			"id": "com.example/pet-test",
			"name": {"default": "Pet"},
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
						"id": "pet-no-runtime",
						"kind": "petx",
						"name": {"default": "Pet Without Runtime"}
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
		t.Errorf("expected petx without runtimeModuleId to pass, got errors: %v", report.Errors)
	}
}

func TestDuplicateDesktopPetPluginIDRejected(t *testing.T) {
	manifest := `{
		"manifestVersion": 1,
		"extension": {
			"id": "com.example/pet-test",
			"name": {"default": "Pet"},
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
						"id": "pet-dup",
						"kind": "petx",
						"name": {"default": "Pet 1"}
					},
					{
						"id": "pet-dup",
						"kind": "petx",
						"name": {"default": "Pet 2"}
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
		t.Errorf("expected duplicate petx ID to be rejected")
	}
}

func TestMultipleDesktopPetPluginContributionsAllowed(t *testing.T) {
	manifest := `{
		"manifestVersion": 1,
		"extension": {
			"id": "com.example/pet-test",
			"name": {"default": "Pet"},
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
						"id": "pet-overlay",
						"kind": "petx",
						"name": {"default": "Overlay"}
					},
					{
						"id": "pet-interaction",
						"kind": "petx",
						"name": {"default": "Interaction"}
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
		t.Errorf("expected multiple petx contributions to pass, got errors: %v", report.Errors)
	}
}

func TestDesktopPetPluginPreservedInExtensionDefinition(t *testing.T) {
	manifest := `{
		"manifestVersion": 1,
		"extension": {
			"id": "com.example/pet-test",
			"name": {"default": "Pet"},
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
						"id": "pet-1",
						"kind": "petx",
						"name": {"default": "Pet Plugin"}
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
	if string(def.Modules[0].Contributions[0].Kind) != "petx" {
		t.Errorf("expected contribution kind petx, got %s", def.Modules[0].Contributions[0].Kind)
	}
}

func TestDesktopPetPluginJSONRoundTrip(t *testing.T) {
	manifest := `{
		"manifestVersion": 1,
		"extension": {
			"id": "com.example/pet-test",
			"name": {"default": "Pet"},
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
						"id": "pet-1",
						"kind": "petx",
						"name": {"default": "Pet"},
						"spec": {"runtimeModuleId": "main"}
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
	if !strings.Contains(string(data), `"petx"`) {
		t.Errorf("expected petx in JSON output, got: %s", string(data))
	}
}

func TestDomainMappingDesktopPetPlugin(t *testing.T) {
	manifest := `{
		"manifestVersion": 1,
		"extension": {
			"id": "com.example/pet-test",
			"name": {"default": "Pet"},
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
						"id": "pet-1",
						"kind": "petx",
						"name": {"default": "Pet"}
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
	if m.Modules[0].Contributions[0].Kind != "petx" {
		t.Errorf("expected petx kind, got %s", m.Modules[0].Contributions[0].Kind)
	}
}
