package repair_baseline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBaseline_E2E_TestExtensionDirExists(t *testing.T) {
	dir := testExtensionsDir(t)
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("testdata/extensions/extension-kernel-repair/ directory must exist; Phase 10 section 16 requires test extensions for E2E verification: %v", err)
	}
}

func TestBaseline_E2E_ToolBasicManifestIsValid(t *testing.T) {
	manifestPath := filepath.Join(testExtensionsDir(t), "tool-basic", "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("tool-basic/manifest.json must exist and be readable: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("tool-basic/manifest.json must be valid JSON: %v", err)
	}
	ext, ok := manifest["extension"].(map[string]any)
	if !ok {
		t.Fatalf("tool-basic/manifest.json must have an 'extension' object")
	}
	if ext["id"] != "com.amitia.repair/tool-basic" {
		t.Fatalf("tool-basic extension id must be 'com.amitia.repair/tool-basic', got: %v", ext["id"])
	}
	modules, ok := manifest["modules"].([]any)
	if !ok || len(modules) == 0 {
		t.Fatalf("tool-basic/manifest.json must have at least one module")
	}
}

func TestBaseline_E2E_ToolBasicHasEntryPoint(t *testing.T) {
	entryPath := filepath.Join(testExtensionsDir(t), "tool-basic", "modules", "main", "index.js")
	if _, err := os.Stat(entryPath); err != nil {
		t.Fatalf("tool-basic/modules/main/index.js entry point must exist: %v", err)
	}
}

func TestBaseline_E2E_RequiredTestExtensions(t *testing.T) {
	requiredExtensions := []string{
		"tool-basic",
		"tool-permission-denied",
		"tool-scope-denied",
		"event-basic",
		"event-permission-denied",
		"event-scope-denied",
		"event-generation-v1",
		"event-generation-v2",
		"schedule-tool",
		"schedule-workflow",
		"schedule-task",
		"signature-valid",
		"signature-unknown-key",
		"signature-publisher-mismatch",
		"signature-tampered",
		"dev-hot-reload-v1",
		"dev-hot-reload-v2",
		"runtime-crash",
		"uninstall-cleanup",
	}
	dir := testExtensionsDir(t)
	missing := []string{}
	for _, name := range requiredExtensions {
		extDir := filepath.Join(dir, name)
		if _, err := os.Stat(extDir); err != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("Phase 10 section 16 requires %d test extensions; %d exist, missing=%v",
			len(requiredExtensions), len(requiredExtensions)-len(missing), missing)
	}
}

func TestBaseline_E2E_AllExtensionsHaveValidManifest(t *testing.T) {
	requiredExtensions := []string{
		"tool-basic",
		"tool-permission-denied",
		"tool-scope-denied",
		"event-basic",
		"event-permission-denied",
		"event-scope-denied",
		"event-generation-v1",
		"event-generation-v2",
		"schedule-tool",
		"schedule-workflow",
		"schedule-task",
		"signature-valid",
		"signature-unknown-key",
		"signature-publisher-mismatch",
		"signature-tampered",
		"dev-hot-reload-v1",
		"dev-hot-reload-v2",
		"runtime-crash",
		"uninstall-cleanup",
	}
	dir := testExtensionsDir(t)
	for _, name := range requiredExtensions {
		t.Run(name, func(t *testing.T) {
			manifestPath := filepath.Join(dir, name, "manifest.json")
			data, err := os.ReadFile(manifestPath)
			if err != nil {
				t.Fatalf("manifest.json must exist and be readable: %v", err)
			}
			var manifest map[string]any
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatalf("manifest.json must be valid JSON: %v", err)
			}
			if manifest["manifestVersion"] != float64(2) {
				t.Fatalf("manifestVersion must be 2, got %v", manifest["manifestVersion"])
			}
			ext, ok := manifest["extension"].(map[string]any)
			if !ok {
				t.Fatalf("manifest must have an 'extension' object")
			}
			if ext["id"] == "" {
				t.Fatalf("extension.id must not be empty")
			}
			if ext["version"] == "" {
				t.Fatalf("extension.version must not be empty")
			}
			pub, ok := manifest["publisher"].(map[string]any)
			if !ok {
				t.Fatalf("manifest must have a 'publisher' object")
			}
			if pub["id"] == "" {
				t.Fatalf("publisher.id must not be empty")
			}
			modules, ok := manifest["modules"].([]any)
			if !ok || len(modules) == 0 {
				t.Fatalf("manifest must have at least one module")
			}
			integrity, ok := manifest["integrity"].(map[string]any)
			if !ok {
				t.Fatalf("manifest must have an 'integrity' object")
			}
			if integrity["algorithm"] == "" {
				t.Fatalf("integrity.algorithm must not be empty")
			}
		})
	}
}

func TestBaseline_E2E_AllExtensionsHaveEntryPoint(t *testing.T) {
	requiredExtensions := []string{
		"tool-basic",
		"tool-permission-denied",
		"tool-scope-denied",
		"event-basic",
		"event-permission-denied",
		"event-scope-denied",
		"event-generation-v1",
		"event-generation-v2",
		"schedule-tool",
		"schedule-workflow",
		"schedule-task",
		"signature-valid",
		"signature-unknown-key",
		"signature-publisher-mismatch",
		"signature-tampered",
		"dev-hot-reload-v1",
		"dev-hot-reload-v2",
		"runtime-crash",
		"uninstall-cleanup",
	}
	dir := testExtensionsDir(t)
	for _, name := range requiredExtensions {
		t.Run(name, func(t *testing.T) {
			entryPath := filepath.Join(dir, name, "modules", "main", "index.js")
			if _, err := os.Stat(entryPath); err != nil {
				t.Fatalf("modules/main/index.js entry point must exist: %v", err)
			}
		})
	}
}

func TestBaseline_E2E_EventE2ETestExists(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	eventE2E := filepath.Join(filepath.Dir(file), "..", "event", "event_e2e_test.go")
	if _, err := os.Stat(eventE2E); err != nil {
		t.Fatalf("event/event_e2e_test.go must exist as E2E test infrastructure; Phase 10 requires real E2E tests: %v", err)
	}
}

func TestBaseline_E2E_LegacyCallCounterExposedInDevConsole(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	devConsolePath := filepath.Join(filepath.Dir(file), "..", "developer_console", "service.go")
	data, err := os.ReadFile(devConsolePath)
	if err != nil {
		t.Fatalf("read developer_console/service.go: %v", err)
	}
	src := string(data)
	if !strings.Contains(src, "LegacyCallProvider") {
		t.Fatalf("developer_console/service.go must expose LegacyCallProvider for Phase 10 legacy zero-call verification")
	}
	if !strings.Contains(src, "LegacyCallCounters") {
		t.Fatalf("ConsoleOverview must include LegacyCallCounters for Phase 10 readiness reporting")
	}
}

func testExtensionsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "testdata", "extensions", "extension-kernel-repair")
}
