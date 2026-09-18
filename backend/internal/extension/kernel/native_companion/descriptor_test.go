package native_companion

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func TestResolveFiltersPlatformAndVerifiesHash(t *testing.T) {
	root := t.TempDir()
	payload := []byte("native helper")
	if err := os.WriteFile(filepath.Join(root, "agent"), payload, 0o700); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	hash := hex.EncodeToString(sum[:])

	got, err := Resolve(root, []domain.NativeCompanionDefinition{
		{ID: "linux-agent", Platform: "linux", Architecture: "amd64", Path: "agent", SHA256: hash, Executable: true},
		{ID: "windows-agent", Platform: "windows", Architecture: "amd64", Path: "agent", SHA256: hash, Executable: true},
	}, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "linux-agent" {
		t.Fatalf("unexpected descriptors: %#v", got)
	}
	if !filepath.IsAbs(got[0].Path) {
		t.Fatalf("expected canonical absolute path, got %q", got[0].Path)
	}
}

func TestResolveRejectsHashMismatch(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "agent"), []byte("native helper"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := Resolve(root, []domain.NativeCompanionDefinition{{
		ID: "agent", Platform: "linux", Architecture: "amd64", Path: "agent",
		SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Executable: true,
	}}, "linux", "amd64")
	if err == nil {
		t.Fatal("expected hash mismatch")
	}
}

func TestResolveRejectsPathEscape(t *testing.T) {
	root := t.TempDir()
	_, err := Resolve(root, []domain.NativeCompanionDefinition{{
		ID: "agent", Platform: "linux", Architecture: "amd64", Path: "../agent",
		SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Executable: true,
	}}, "linux", "amd64")
	if err == nil {
		t.Fatal("expected path escape rejection")
	}
}
