package packageformat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestQACompletePetxAssetManifest(t *testing.T) {
	root := filepath.Clean("../../../../plugins-code/qa-desktop-pet-plugin/.build/assets/pet")
	data, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if os.IsNotExist(err) {
		t.Skip("QA petx fixture has not been generated")
	}
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	report := NewValidator().ValidateDirectory(root, &manifest)
	if report == nil || report.Verdict == "invalid" {
		if canonical, canonicalErr := CanonicalManifestData(&manifest); canonicalErr == nil {
			t.Logf("canonical manifest: %s", canonical)
		}
		t.Fatalf("QA petx asset manifest is invalid: %+v", report)
	}
}
