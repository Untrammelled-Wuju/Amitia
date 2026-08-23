package manifest_v2

import (
	"encoding/json"
	"strings"
	"testing"
)

const validManifest = `{
  "manifestVersion": 2,
  "extension": {
    "id": "com.example/weather",
    "name": {"default": "Weather"},
    "description": {"default": "Weather extension"},
    "version": "1.2.0"
  },
  "publisher": {
    "id": "com.example",
    "displayName": "Example"
  },
  "compatibility": {
    "minHostVersion": "1.0.0",
    "platforms": ["windows", "linux"]
  },
  "modules": [
    {
      "id": "main",
      "name": {"default": "Main"},
      "type": "javascript",
      "runtime": {"type": "javascript", "entryPoint": "index.js"},
      "contributions": [
        {
          "id": "weather-lookup",
          "kind": "tool",
          "name": {"default": "Weather Lookup"}
        }
      ]
    }
  ],
  "dependencies": [
    {"type": "extension", "id": "com.example/geolocation", "version": "^1.0.0"}
  ],
  "integrity": {
    "algorithm": "sha256",
    "contentTreeHash": "abc123"
  }
}`

func TestParseValidManifest(t *testing.T) {
	m, err := Parse([]byte(validManifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.ManifestVersion != 2 {
		t.Errorf("expected v2, got %d", m.ManifestVersion)
	}
	if m.Extension.ID != "com.example/weather" {
		t.Errorf("unexpected id: %s", m.Extension.ID)
	}
}

func TestValidateValidManifest(t *testing.T) {
	m, _ := Parse([]byte(validManifest))
	report := m.Validate()
	if report.HasErrors() {
		t.Errorf("expected no errors, got: %v", report.Errors)
	}
}

func TestValidateUnsupportedVersion(t *testing.T) {
	m, _ := Parse([]byte(`{"manifestVersion": 1}`))
	report := m.Validate()
	if !report.HasErrors() {
		t.Errorf("expected errors for v1")
	}
}

func TestValidateMissingExtensionID(t *testing.T) {
	m, _ := Parse([]byte(`{
		"manifestVersion": 2,
		"extension": {"name": {"default": "x"}, "version": "1.0.0"},
		"publisher": {"id": "p", "displayName": "P"},
		"modules": [{"id": "m", "name": {"default": "M"}, "type": "javascript"}],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "x"}
	}`))
	report := m.Validate()
	if !report.HasErrors() {
		t.Errorf("expected errors for missing id")
	}
}

func TestValidateInvalidExtensionID(t *testing.T) {
	m, _ := Parse([]byte(`{
		"manifestVersion": 2,
		"extension": {"id": "Invalid ID", "name": {"default": "x"}, "version": "1.0.0"},
		"publisher": {"id": "p", "displayName": "P"},
		"modules": [{"id": "m", "name": {"default": "M"}, "type": "javascript"}],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "x"}
	}`))
	report := m.Validate()
	if !report.HasErrors() {
		t.Errorf("expected errors for invalid id")
	}
}

func TestValidateDuplicateModule(t *testing.T) {
	m, _ := Parse([]byte(`{
		"manifestVersion": 2,
		"extension": {"id": "com.example/test", "name": {"default": "x"}, "version": "1.0.0"},
		"publisher": {"id": "p", "displayName": "P"},
		"modules": [
			{"id": "m", "name": {"default": "M"}, "type": "javascript"},
			{"id": "m", "name": {"default": "M2"}, "type": "javascript"}
		],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "x"}
	}`))
	report := m.Validate()
	found := false
	for _, e := range report.Errors {
		if e.Code == "duplicate" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate error, got: %v", report.Errors)
	}
}

func TestValidateUnknownModuleType(t *testing.T) {
	m, _ := Parse([]byte(`{
		"manifestVersion": 2,
		"extension": {"id": "com.example/test", "name": {"default": "x"}, "version": "1.0.0"},
		"publisher": {"id": "p", "displayName": "P"},
		"modules": [{"id": "m", "name": {"default": "M"}, "type": "unknown_type"}],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "x"}
	}`))
	report := m.Validate()
	found := false
	for _, e := range report.Errors {
		if e.Code == "unknown_type" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unknown_type error")
	}
}

func TestValidateDuplicateContribution(t *testing.T) {
	m, _ := Parse([]byte(`{
		"manifestVersion": 2,
		"extension": {"id": "com.example/test", "name": {"default": "x"}, "version": "1.0.0"},
		"publisher": {"id": "p", "displayName": "P"},
		"modules": [{
			"id": "m", "name": {"default": "M"}, "type": "javascript",
			"contributions": [
				{"id": "c1", "kind": "tool", "name": {"default": "C1"}},
				{"id": "c1", "kind": "tool", "name": {"default": "C2"}}
			]
		}],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "x"}
	}`))
	report := m.Validate()
	found := false
	for _, e := range report.Errors {
		if e.Code == "duplicate" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate contribution error")
	}
}

func TestToExtensionDefinition(t *testing.T) {
	m, _ := Parse([]byte(validManifest))
	def, err := m.ToExtensionDefinition()
	if err != nil {
		t.Fatalf("ToExtensionDefinition: %v", err)
	}
	if string(def.ID) != "com.example/weather" {
		t.Errorf("unexpected id: %s", def.ID)
	}
	if len(def.Modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(def.Modules))
	}
	if def.Modules[0].Type != "javascript" {
		t.Errorf("expected javascript module, got %s", def.Modules[0].Type)
	}
	if len(def.Modules[0].Contributions) != 1 {
		t.Fatalf("expected 1 contribution, got %d", len(def.Modules[0].Contributions))
	}
	if def.Modules[0].Contributions[0].Kind != "tool" {
		t.Errorf("expected tool kind, got %s", def.Modules[0].Contributions[0].Kind)
	}
}

func TestToExtensionDefinitionInvalid(t *testing.T) {
	m, _ := Parse([]byte(`{"manifestVersion": 1}`))
	_, err := m.ToExtensionDefinition()
	if err == nil {
		t.Errorf("expected error for invalid manifest")
	}
}

func TestRoundTrip(t *testing.T) {
	m, _ := Parse([]byte(validManifest))
	def, err := m.ToExtensionDefinition()
	if err != nil {
		t.Fatalf("ToExtensionDefinition: %v", err)
	}
	data, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), "com.example/weather") {
		t.Errorf("expected id in output")
	}
}

func TestParseRejectsDuplicateJSONKeys(t *testing.T) {
	_, err := Parse([]byte(`{"manifestVersion":2,"manifestVersion":2}`))
	if err == nil || !strings.Contains(err.Error(), "duplicate JSON key") {
		t.Fatalf("expected duplicate key rejection, got %v", err)
	}
}

const serviceManifest = `{
  "manifestVersion": 2,
  "extension": {
    "id": "com.example/service-test",
    "name": {"default": "Service Test"},
    "version": "1.0.0"
  },
  "publisher": {
    "id": "com.example",
    "displayName": "Example"
  },
  "modules": [
    {
      "id": "service-main",
      "name": {"default": "Service Main"},
      "type": "service",
      "runtime": {
        "type": "service",
        "entryPoint": "bin/service"
      }
    }
  ],
  "integrity": {
    "algorithm": "sha256",
    "contentTreeHash": "test"
  }
}`

func TestValidateServiceRuntime(t *testing.T) {
	m, err := Parse([]byte(serviceManifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	report := m.Validate()
	if report.HasErrors() {
		t.Errorf("expected no errors for service runtime, got: %v", report.Errors)
	}
}

func TestValidateWithSchemaServiceRuntime(t *testing.T) {
	m, err := Parse([]byte(serviceManifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	report := m.ValidateWithSchema()
	if report.HasErrors() {
		t.Errorf("expected no errors for service runtime schema validation, got: %v", report.Errors)
	}
}

func TestValidateUnknownRuntimeStillRejected(t *testing.T) {
	m, _ := Parse([]byte(`{
		"manifestVersion": 2,
		"extension": {"id": "com.example/test", "name": {"default": "x"}, "version": "1.0.0"},
		"publisher": {"id": "p", "displayName": "P"},
		"modules": [{"id": "m", "name": {"default": "M"}, "type": "javascript", "runtime": {"type": "unknown_runtime"}}],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "x"}
	}`))
	report := m.Validate()
	found := false
	for _, e := range report.Errors {
		if e.Code == "unsupported_runtime" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected unsupported_runtime error, got: %v", report.Errors)
	}
}

func TestValidateTrustedServiceManifestRejected(t *testing.T) {
	m, _ := Parse([]byte(`{
		"manifestVersion": 2,
		"extension": {"id": "com.example/test", "name": {"default": "x"}, "version": "1.0.0"},
		"publisher": {"id": "p", "displayName": "P"},
		"modules": [{"id": "m", "name": {"default": "M"}, "type": "service", "runtime": {"type": "trusted_service"}}],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "x"}
	}`))
	report := m.Validate()
	if !report.HasErrors() {
		t.Errorf("expected trusted_service to be rejected by manifest, got no errors")
	}
}

func TestValidatePluginServiceManifestRejected(t *testing.T) {
	m, _ := Parse([]byte(`{
		"manifestVersion": 2,
		"extension": {"id": "com.example/test", "name": {"default": "x"}, "version": "1.0.0"},
		"publisher": {"id": "p", "displayName": "P"},
		"modules": [{"id": "m", "name": {"default": "M"}, "type": "service", "runtime": {"type": "plugin_service"}}],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "x"}
	}`))
	report := m.Validate()
	if !report.HasErrors() {
		t.Errorf("expected plugin_service to be rejected by manifest, got no errors")
	}
}

func TestValidateTrustedServiceSchemaRejected(t *testing.T) {
	m, _ := Parse([]byte(`{
		"manifestVersion": 2,
		"extension": {"id": "com.example/test", "name": {"default": "x"}, "version": "1.0.0"},
		"publisher": {"id": "p", "displayName": "P"},
		"modules": [{"id": "m", "name": {"default": "M"}, "type": "service", "runtime": {"type": "trusted_service"}}],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "x"}
	}`))
	report := m.ValidateWithSchema()
	if !report.HasErrors() {
		t.Errorf("expected trusted_service to be rejected by schema, got no errors")
	}
}

func TestValidatePluginServiceSchemaRejected(t *testing.T) {
	m, _ := Parse([]byte(`{
		"manifestVersion": 2,
		"extension": {"id": "com.example/test", "name": {"default": "x"}, "version": "1.0.0"},
		"publisher": {"id": "p", "displayName": "P"},
		"modules": [{"id": "m", "name": {"default": "M"}, "type": "service", "runtime": {"type": "plugin_service"}}],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "x"}
	}`))
	report := m.ValidateWithSchema()
	if !report.HasErrors() {
		t.Errorf("expected plugin_service to be rejected by schema, got no errors")
	}
}

func TestToExtensionDefinitionServiceRuntime(t *testing.T) {
	m, err := Parse([]byte(serviceManifest))
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
	if def.Modules[0].Runtime == nil {
		t.Fatalf("expected runtime to be non-nil")
	}
	if string(def.Modules[0].Runtime.Type) != "service" {
		t.Errorf("expected runtime type service, got %s", def.Modules[0].Runtime.Type)
	}
	if def.Modules[0].Type != "service" {
		t.Errorf("expected module type service, got %s", def.Modules[0].Type)
	}
}

func TestToExtensionDefinitionDoesNotTrustManifestPublisherClaim(t *testing.T) {
	m, err := Parse([]byte(`{
		"manifestVersion": 2,
		"extension": {"id": "com.example/nav", "name": {"default": "Nav"}, "version": "1.0.0"},
		"publisher": {"id": "com.example", "displayName": "Example", "trustLevel": "trusted"},
		"modules": [{
			"id": "ui", "name": {"default": "UI"}, "type": "javascript",
			"runtime": {"type": "javascript", "entryPoint": "index.js"},
			"contributions": [{
				"id": "nav-provider", "kind": "ui_provider", "name": {"default": "Navigation"},
				"definition": {"providerId": "nav-provider", "capability": "app.navigation", "trustLevel": "trusted"}
			}]
		}],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "abc123"}
	}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	def, err := m.ToExtensionDefinition()
	if err != nil {
		t.Fatalf("ToExtensionDefinition: %v", err)
	}
	if def.Publisher.TrustLevel != "unknown" {
		t.Fatalf("manifest publisher trust must not become authoritative, got %q", def.Publisher.TrustLevel)
	}
	got := def.Modules[0].Contributions[0].Definition["trustLevel"]
	if got != "unknown" {
		t.Fatalf("ui provider trust must default to unknown, got %#v", got)
	}
}
