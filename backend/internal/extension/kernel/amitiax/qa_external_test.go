package amitiax

import (
	"os"
	"testing"
)

func TestQAExternalPackage(t *testing.T) {
	path := os.Getenv("AMITIA_QA_AMITIAX_PATH")
	if path == "" {
		t.Skip("AMITIA_QA_AMITIAX_PATH is not set")
	}
	pkg, err := OpenArchive(path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	if err := VerifyIntegrity(pkg); err != nil {
		t.Fatalf("verify integrity: %v", err)
	}
	if pkg.Manifest.Extension.ID != "com.amitia.qa/desktop-pet-plugin" {
		t.Fatalf("extension ID=%s", pkg.Manifest.Extension.ID)
	}
}
