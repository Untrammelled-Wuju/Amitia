package kernel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

const providerConformanceGoldenPath = "testdata/provider-conformance.json"

type providerConformanceGolden struct {
	SchemaVersion     int                      `json:"schemaVersion"`
	ExtensionID       string                   `json:"extensionId"`
	DeclaredProviders int                      `json:"declaredProviders"`
	RuntimeProviders  int                      `json:"runtimeProviders"`
	Gaps              []ProviderConformanceGap `json:"gaps"`
}

func loadEmoteExtensionDefinition(t *testing.T) domain.ExtensionDefinition {
	t.Helper()
	manifestPath := filepath.Join("..", "..", "..", "..", "plugins-code", "emote", "amitia-extension.json")
	body, err := os.ReadFile(filepath.Clean(manifestPath))
	if err != nil {
		t.Fatalf("read emote manifest: %v", err)
	}
	manifest, err := plugincontract.Parse(body)
	if err != nil {
		t.Fatalf("parse emote manifest: %v", err)
	}
	definition, err := manifest.ToExtensionDefinition()
	if err != nil {
		t.Fatalf("project emote manifest: %v", err)
	}
	return definition
}

func registerEmoteProviders(t *testing.T, registry *capability.ProviderRegistry) []capability.CapabilityProviderDefinition {
	t.Helper()
	definitions, err := capability.ProviderDefinitionsFromExtension(loadEmoteExtensionDefinition(t))
	if err != nil {
		t.Fatalf("derive provider definitions: %v", err)
	}
	for _, definition := range definitions {
		if err := registry.RegisterDefinition(definition); err != nil {
			t.Fatalf("register %s: %v", definition.ID, err)
		}
	}
	return definitions
}

func TestProviderConformanceRealManifestHasNoGaps(t *testing.T) {
	registry := capability.NewProviderRegistry()
	definitions := registerEmoteProviders(t, registry)
	if len(definitions) != 1 {
		t.Fatalf("derived definitions = %d, want 1", len(definitions))
	}
	if definitions[0].ExtensionID != "com.amitia/emote" || definitions[0].ModuleID != "emote-runtime" {
		t.Fatalf("definition module = %s/%s", definitions[0].ExtensionID, definitions[0].ModuleID)
	}
	if definitions[0].CapabilityID != "com.amitia/emote/ai" {
		t.Fatalf("definition capability = %s", definitions[0].CapabilityID)
	}
	report, err := CheckProviderConformance([]domain.ExtensionDefinition{loadEmoteExtensionDefinition(t)}, registry)
	if err != nil {
		t.Fatalf("check conformance: %v", err)
	}
	if report.DeclaredProviders != 1 || report.RuntimeProviders != 1 {
		t.Fatalf("report = %+v", report)
	}
	if len(report.Gaps) != 0 || !report.Conformant() {
		t.Fatalf("gaps = %+v", report.Gaps)
	}
}

func TestProviderConformanceDetectsMissingRuntimeProvider(t *testing.T) {
	registry := capability.NewProviderRegistry()
	definitions := registerEmoteProviders(t, registry)
	if _, err := registry.DeregisterDefinition(definitions[0].ID); err != nil {
		t.Fatalf("deregister: %v", err)
	}
	report, err := CheckProviderConformance([]domain.ExtensionDefinition{loadEmoteExtensionDefinition(t)}, registry)
	if err != nil {
		t.Fatalf("check conformance: %v", err)
	}
	if report.Conformant() {
		t.Fatalf("missing runtime provider reported as conformant")
	}
	advisory := report.AdvisoryGaps()
	if len(advisory) != 1 || advisory[0].CapabilityID != "com.amitia/emote/ai" {
		t.Fatalf("advisory gaps = %+v", advisory)
	}
	if advisory[0].Kind != ProviderGapDeclaredMissing {
		t.Fatalf("gap kind = %s", advisory[0].Kind)
	}
}

func TestProviderConformanceDetectsUndeclaredRuntimeProvider(t *testing.T) {
	registry := capability.NewProviderRegistry()
	registerEmoteProviders(t, registry)
	orphan := capability.CapabilityProviderDefinition{
		ID:           "extprov_orphan",
		CapabilityID: "com.amitia/emote/ghost",
		Kind:         capability.ProviderKindExtension,
		Placement:    capability.ProviderPlacementDevice,
		ExtensionID:  "com.amitia/emote",
		ModuleID:     "emote-runtime",
		Runtime:      capability.RuntimeBinding{RuntimeType: capability.RuntimeTypePluginJS},
	}
	if err := registry.RegisterDefinition(orphan); err != nil {
		t.Fatalf("register orphan: %v", err)
	}
	report, err := CheckProviderConformance([]domain.ExtensionDefinition{loadEmoteExtensionDefinition(t)}, registry)
	if err != nil {
		t.Fatalf("check conformance: %v", err)
	}
	if report.Conformant() {
		t.Fatalf("orphan provider reported as conformant")
	}
	blocking := report.BlockingGaps()
	if len(blocking) != 1 || blocking[0].ProviderID != "extprov_orphan" {
		t.Fatalf("blocking gaps = %+v", blocking)
	}
	if blocking[0].CapabilityID != "com.amitia/emote/ghost" {
		t.Fatalf("blocking capability = %s", blocking[0].CapabilityID)
	}
}

func TestProviderConformanceIgnoresBuiltinProviders(t *testing.T) {
	registry := capability.NewProviderRegistry()
	builtin := capability.CapabilityProviderDefinition{
		ID:           "builtin.core/core/memory.search",
		CapabilityID: "core/memory.search",
		Kind:         capability.ProviderKindBuiltin,
		Placement:    capability.ProviderPlacementCore,
		Runtime:      capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeBuiltin},
	}
	if err := registry.RegisterDefinition(builtin); err != nil {
		t.Fatalf("register builtin: %v", err)
	}
	report, err := CheckProviderConformance(nil, registry)
	if err != nil {
		t.Fatalf("check conformance: %v", err)
	}
	if report.RuntimeProviders != 0 || len(report.Gaps) != 0 {
		t.Fatalf("builtin provider leaked into report: %+v", report)
	}
	if !report.Conformant() {
		t.Fatalf("empty declaration with builtin providers reported non conformant")
	}
}

func TestProviderConformanceRequiresRegistry(t *testing.T) {
	if _, err := CheckProviderConformance(nil, nil); err == nil {
		t.Fatalf("nil registry accepted")
	}
}

func TestProviderConformanceMatchesGolden(t *testing.T) {
	registry := capability.NewProviderRegistry()
	registerEmoteProviders(t, registry)
	report, err := CheckProviderConformance([]domain.ExtensionDefinition{loadEmoteExtensionDefinition(t)}, registry)
	if err != nil {
		t.Fatalf("check conformance: %v", err)
	}
	golden := providerConformanceGolden{
		SchemaVersion:     1,
		ExtensionID:       "com.amitia/emote",
		DeclaredProviders: report.DeclaredProviders,
		RuntimeProviders:  report.RuntimeProviders,
		Gaps:              append([]ProviderConformanceGap{}, report.Gaps...),
	}
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		body, err := json.MarshalIndent(golden, "", "  ")
		if err != nil {
			t.Fatalf("marshal golden: %v", err)
		}
		if err := os.WriteFile(providerConformanceGoldenPath, append(body, '\n'), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	body, err := os.ReadFile(providerConformanceGoldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var recorded providerConformanceGolden
	if err := json.Unmarshal(body, &recorded); err != nil {
		t.Fatalf("decode golden: %v", err)
	}
	if recorded.SchemaVersion != golden.SchemaVersion {
		t.Fatalf("golden schema version = %d", recorded.SchemaVersion)
	}
	if recorded.ExtensionID != golden.ExtensionID {
		t.Fatalf("golden extension = %s", recorded.ExtensionID)
	}
	if recorded.DeclaredProviders != golden.DeclaredProviders || recorded.RuntimeProviders != golden.RuntimeProviders {
		t.Fatalf("golden providers = %d/%d, live = %d/%d", recorded.DeclaredProviders, recorded.RuntimeProviders, golden.DeclaredProviders, golden.RuntimeProviders)
	}
	if len(recorded.Gaps) != len(golden.Gaps) {
		t.Fatalf("golden gaps = %d, live = %d", len(recorded.Gaps), len(golden.Gaps))
	}
	for index := range recorded.Gaps {
		if recorded.Gaps[index] != golden.Gaps[index] {
			t.Fatalf("gap %d: golden %+v, live %+v", index, recorded.Gaps[index], golden.Gaps[index])
		}
	}
}
