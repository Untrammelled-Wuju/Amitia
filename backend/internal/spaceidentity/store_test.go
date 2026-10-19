package spaceidentity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenReusesPersistedIdentity(t *testing.T) {
	dataDir := t.TempDir()
	first, err := Open(dataDir)
	if err != nil {
		t.Fatalf("open first identity: %v", err)
	}
	second, err := Open(dataDir)
	if err != nil {
		t.Fatalf("open second identity: %v", err)
	}
	if first.SpaceID() != second.SpaceID() || first.Identity().InstanceID != second.Identity().InstanceID {
		t.Fatalf("identity changed: first=%+v second=%+v", first.Identity(), second.Identity())
	}
}

func TestOpenRejectsInvalidIdentityWithoutRegenerating(t *testing.T) {
	dataDir := t.TempDir()
	identityPath := filepath.Join(dataDir, "identity", "space.json")
	if err := os.MkdirAll(filepath.Dir(identityPath), 0o700); err != nil {
		t.Fatalf("create identity directory: %v", err)
	}
	original := []byte(`{"schemaVersion":1,"spaceId":"broken","instanceId":"broken"}`)
	if err := os.WriteFile(identityPath, original, 0o600); err != nil {
		t.Fatalf("write invalid identity: %v", err)
	}
	if _, err := Open(dataDir); err == nil {
		t.Fatal("expected invalid identity to be rejected")
	}
	current, err := os.ReadFile(identityPath)
	if err != nil {
		t.Fatalf("read identity after rejection: %v", err)
	}
	if string(current) != string(original) {
		t.Fatal("invalid identity file was overwritten")
	}
}
