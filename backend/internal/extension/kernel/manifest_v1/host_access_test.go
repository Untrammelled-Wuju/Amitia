package manifest_v1

import (
	"encoding/json"
	"testing"
)

func TestRuntimeHostAccessRequiresTrustedPublisher(t *testing.T) {
	manifest := Manifest{
		ManifestVersion: ManifestVersion,
		Placement:       "device",
		Extension: ExtensionMeta{
			ID:      "com.example/host-access",
			Name:    LocalizedText{Default: "Host Access"},
			Version: "1.0.0",
		},
		Publisher: PublisherMeta{ID: "com.example", TrustLevel: "community"},
		Modules: []ModuleMeta{{
			ID:   "runtime",
			Name: LocalizedText{Default: "Runtime"},
			Type: "javascript",
			Runtime: &RuntimeMeta{
				Type:       "javascript",
				EntryPoint: "index.js",
				HostAccess: &RuntimeHostAccessMeta{
					Network: &RuntimeNetworkAccessMeta{Mode: "host"},
				},
			},
		}},
		Integrity: IntegrityMeta{Algorithm: "sha256", ContentTreeHash: "hash"},
	}
	report := manifest.Validate()
	if !report.HasErrors() {
		t.Fatal("expected untrusted host access to fail")
	}
	found := false
	for _, issue := range report.Errors {
		if issue.Code == "untrusted_publisher" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected untrusted_publisher, got %#v", report.Errors)
	}
}

func TestRuntimeHostAccessAcceptsTrustedPublisher(t *testing.T) {
	manifest := Manifest{
		ManifestVersion: ManifestVersion,
		Placement:       "device",
		Extension: ExtensionMeta{
			ID:      "com.example/host-access",
			Name:    LocalizedText{Default: "Host Access"},
			Version: "1.0.0",
		},
		Publisher: PublisherMeta{ID: "com.example", TrustLevel: "official"},
		Modules: []ModuleMeta{{
			ID:   "runtime",
			Name: LocalizedText{Default: "Runtime"},
			Type: "javascript",
			Runtime: &RuntimeMeta{
				Type:       "javascript",
				EntryPoint: "index.js",
				HostAccess: &RuntimeHostAccessMeta{
					Filesystem: &RuntimeFilesystemAccessMeta{Mode: "read_write", Roots: []string{"package", "plugin_data"}},
					Network:    &RuntimeNetworkAccessMeta{Mode: "host"},
					Process:    &RuntimeProcessAccessMeta{Mode: "direct", MaxProcesses: 1},
				},
			},
		}},
		Integrity: IntegrityMeta{Algorithm: "sha256", ContentTreeHash: "hash"},
	}
	report := manifest.Validate()
	if report.HasErrors() {
		t.Fatalf("unexpected validation errors: %#v", report.Errors)
	}
}

func TestRawHostAccessValidationDoesNotPanic(t *testing.T) {
	raw := map[string]any{
		"manifestVersion": 1,
		"placement":       "device",
		"extension": map[string]any{
			"id":      "com.example/host-access",
			"name":    map[string]any{"default": "Host Access"},
			"version": "1.0.0",
		},
		"publisher": map[string]any{"id": "com.example", "trustLevel": "official"},
		"modules": []any{map[string]any{
			"id":   "runtime",
			"name": map[string]any{"default": "Runtime"},
			"type": "javascript",
			"runtime": map[string]any{
				"type":       "javascript",
				"entryPoint": "index.js",
				"hostAccess": map[string]any{
					"filesystem": map[string]any{"mode": "read_write", "roots": []any{"package", "plugin_data"}},
					"network":    map[string]any{"mode": "host"},
					"process":    map[string]any{"mode": "direct", "maxProcesses": 1},
				},
			},
		}},
		"integrity": map[string]any{"algorithm": "sha256", "contentTreeHash": "hash"},
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	report := ValidateRawWithSchema(data)
	for _, issue := range report.Errors {
		if issue.Code == "schema_type" && issue.Path != "integrity.contentTreeHash" {
			t.Fatalf("unexpected schema type error: %#v", issue)
		}
	}
}
