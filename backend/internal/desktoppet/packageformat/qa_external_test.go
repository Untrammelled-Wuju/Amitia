package packageformat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestQAExternalArchive(t *testing.T) {
	path := os.Getenv("AMITIA_QA_PETX_PATH")
	root := os.Getenv("AMITIA_QA_PETX_ROOT")
	output := os.Getenv("AMITIA_QA_PETX_OUTPUT")
	if root != "" && output != "" {
		manifestPath := filepath.Join(root, "manifest.json")
		manifestData, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatalf("read manifest: %v", err)
		}
		var manifest Manifest
		if err := json.Unmarshal(manifestData, &manifest); err != nil {
			t.Fatalf("decode manifest: %v", err)
		}
		writer := NewArchiveWriter()
		built, err := writer.BuildManifestForArchive(root, &manifest)
		if err != nil {
			t.Fatalf("build manifest: %v", err)
		}
		manifestData, err = json.MarshalIndent(built, "", "  ")
		if err != nil {
			t.Fatalf("encode manifest: %v", err)
		}
		manifestData = append(manifestData, '\n')
		if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
		if err := writer.WriteArchive(root, output); err != nil {
			t.Fatalf("write archive: %v", err)
		}
		path = output
	}
	if path == "" {
		t.Skip("AMITIA_QA_PETX_PATH is not set")
	}
	report := NewValidator().ValidateArchive(path)
	for _, finding := range report.Findings {
		t.Logf("%s %s path=%s action=%s message=%s", finding.Severity, finding.Code, finding.Path, finding.ActionKey, finding.Message)
	}
	if report.Verdict != "valid" {
		t.Fatalf("verdict=%s errors=%d warnings=%d", report.Verdict, report.ErrorCount, report.WarningCount)
	}
}
