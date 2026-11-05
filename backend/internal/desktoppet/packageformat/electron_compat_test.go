package packageformat

import (
	"encoding/json"
	"testing"
)

func TestElectronCanonicalManifestCompatibilityFixture(t *testing.T) {
	raw := `{
  "schemaVersion": 1,
  "name": "Hash Test",
  "integrity": {
    "algorithm": "amitia-package-sha256-v1",
    "manifestHash": "1111111111111111111111111111111111111111111111111111111111111111",
    "contentRootHash": "2222222222222222222222222222222222222222222222222222222222222222",
    "fileCount": 0,
    "totalBytes": 0,
    "files": []
  }
}`
	var manifest Manifest
	if err := json.Unmarshal([]byte(raw), &manifest); err != nil {
		t.Fatal(err)
	}
	canonical, err := CanonicalManifestData(&manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(canonical) != 1134 {
		t.Fatalf("canonical bytes=%d", len(canonical))
	}
	if hash := ManifestHash(canonical); hash != "0bb42e38b4b918a5aa8c8bf35743b1a71ec1b2448d36c2b605865554b32c5eec" {
		t.Fatalf("canonical hash=%s", hash)
	}
}
