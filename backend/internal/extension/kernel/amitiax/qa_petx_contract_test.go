package amitiax

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

func TestQAPetxSourceUsesCanonicalManifestContract(t *testing.T) {
	const manifestPath = "../../../../../plugins-code/qa-desktop-pet-plugin/amitia-extension.json"
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read QA petx source manifest: %v", err)
	}
	manifest, err := plugincontract.Parse(data)
	if err != nil {
		t.Fatalf("parse QA petx source manifest: %v", err)
	}
	assertQAPetxManifest(t, manifest)
}

func TestQAPetxArchiveUsesCanonicalManifestContract(t *testing.T) {
	const manifestPath = "../../../../../plugins-code/qa-desktop-pet-plugin/amitia-extension.json"
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read QA petx source manifest: %v", err)
	}
	manifest, err := plugincontract.Parse(data)
	if err != nil {
		t.Fatalf("parse QA petx source manifest: %v", err)
	}
	archivePath := filepath.Join(
		"../../../../../plugin/qa",
		"qa-desktop-pet-plugin-"+manifest.Plugin.Version+".petx",
	)
	pkg, err := OpenArchive(archivePath)
	if os.IsNotExist(err) {
		t.Skip("QA petx archive has not been generated")
	}
	if err != nil {
		t.Fatalf("open QA petx: %v", err)
	}
	assertQAPetxManifest(t, &pkg.Manifest)
}

func assertQAPetxManifest(t *testing.T, manifest *plugincontract.Manifest) {
	t.Helper()
	report := manifest.Validate()
	if report.HasErrors() {
		t.Fatalf("QA petx manifest is invalid: %+v", report.Issues)
	}
	if manifest.SchemaVersion != plugincontract.SchemaVersion {
		t.Fatalf("unexpected schema version: %d", manifest.SchemaVersion)
	}
	if manifest.Plugin.ID != "com.amitia.qa/desktop-pet-plugin" {
		t.Fatalf("unexpected plugin identity: %s", manifest.Plugin.ID)
	}
	if len(manifest.Modules) != 1 {
		t.Fatalf("unexpected module count: %d", len(manifest.Modules))
	}
	runtime := manifest.Modules[0].Runtime
	if runtime == nil || runtime.Adapter != "javascript" || runtime.Entry != "dist/index.js" {
		t.Fatalf("unexpected runtime contract: %+v", runtime)
	}
	contributions := manifest.Modules[0].Contributions
	if len(contributions) != 1 {
		t.Fatalf("unexpected contribution count: %d", len(contributions))
	}
	contribution := contributions[0]
	if contribution.Kind != "petx" || contribution.Adapter != string(domain.ContributionAdapterDesktopPet) || contribution.ContractVersion != 1 {
		t.Fatalf("unexpected desktop pet contribution: %+v", contribution)
	}
	if contribution.Spec["resourceRef"] != "assets/pet/manifest.json" {
		t.Fatalf("unexpected resource reference: %+v", contribution.Spec)
	}
}
