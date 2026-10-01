package workspace

import (
	"context"
	"testing"
)

func TestResolveURIToMountAcceptsMountRoot(t *testing.T) {
	registry := NewRegistry()
	mount, err := registry.RegisterLocalMount(context.Background(), "root", t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(registry, nil)
	resolved, relative, err := svc.resolveURIToMount(MountURI(mount.ID))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ID != mount.ID || relative != "" {
		t.Fatalf("unexpected root resolution: mount=%s relative=%q", resolved.ID, relative)
	}
}
