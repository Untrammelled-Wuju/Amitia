package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestAcquisitionDiscoveryLocalPackageArchive(t *testing.T) {
	store := NewPackageArtifactStore(t.TempDir())
	file := filepath.Join(t.TempDir(), "插件.amitiax")
	content := []byte("isolated package archive")
	if err := os.WriteFile(file, content, 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(content)
	expected := hex.EncodeToString(hash[:])
	fileURL := (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(file)}).String()
	for _, source := range []string{file, fileURL} {
		artifact, err := store.PutArchiveFromURI(context.Background(), source, ArtifactMetadata{ExpectedHash: expected})
		if err != nil || artifact.ArchiveHash != "sha256:"+expected {
			t.Fatalf("artifact=%+v err=%v", artifact, err)
		}
	}
	if _, err := store.PutArchiveFromURI(context.Background(), file, ArtifactMetadata{ExpectedHash: "incorrect"}); err == nil {
		t.Fatal("archive hash mismatch was ignored")
	}
}
