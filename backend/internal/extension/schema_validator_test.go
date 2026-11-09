package extension

import (
	"encoding/json"
	"testing"
)

func TestManifestSchemaAcceptsUnifiedContract(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatalf("compile manifest schema: %v", err)
	}
	raw := json.RawMessage(`{
		"schemaVersion": 1,
		"plugin": {
			"id": "com.example/plugin",
			"name": {"default": "Plugin"},
			"version": "1.0.0",
			"publisher": {"id": "com.example", "displayName": "Example"}
		},
		"compatibility": {"minHostVersion": "0.1.0"},
		"modules": [{
			"id": "main",
			"name": {"default": "Main"},
			"runtime": {"adapter": "javascript", "entry": "index.js"},
			"contributions": [{
				"id": "command",
				"kind": "tool",
				"adapter": "tool",
				"contractVersion": 1,
				"name": {"default": "Command"}
			}]
		}],
		"integrity": {"algorithm": "sha256", "contentTreeHash": ""}
	}`)
	if err := validator.ValidateManifest(raw); err != nil {
		t.Fatalf("validate manifest: %v", err)
	}
}
