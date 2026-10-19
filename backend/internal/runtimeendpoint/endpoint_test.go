package runtimeendpoint

import (
	"path/filepath"
	"testing"
)

func TestRemoveIfOwnerRemovesMatchingEndpoint(t *testing.T) {
	runDir := t.TempDir()
	endpoint := New("test", "space", "127.0.0.1", 59922)
	if err := Write(runDir, endpoint); err != nil {
		t.Fatalf("write endpoint: %v", err)
	}

	if err := RemoveIfOwner(runDir, endpoint.PID); err != nil {
		t.Fatalf("remove endpoint: %v", err)
	}
	if _, err := Read(runDir); err == nil {
		t.Fatal("endpoint file still exists")
	}
}

func TestRemoveIfOwnerKeepsOtherEndpoint(t *testing.T) {
	runDir := t.TempDir()
	endpoint := New("test", "space", "127.0.0.1", 59922)
	if err := Write(runDir, endpoint); err != nil {
		t.Fatalf("write endpoint: %v", err)
	}

	if err := RemoveIfOwner(runDir, endpoint.PID+1); err != nil {
		t.Fatalf("remove endpoint: %v", err)
	}
	if _, err := Read(runDir); err != nil {
		t.Fatalf("endpoint should remain: %v", err)
	}
}

func TestRemoveIfOwnerMissingFile(t *testing.T) {
	runDir := t.TempDir()
	if err := RemoveIfOwner(runDir, 1234); err != nil {
		t.Fatalf("missing endpoint should be ignored: %v", err)
	}
	if _, err := Read(runDir); err == nil {
		t.Fatalf("unexpected endpoint at %s", filepath.Join(runDir, "endpoint.json"))
	}
}
