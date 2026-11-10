package spaceidentity

import (
	"os"
	"path/filepath"
	"sync"
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

func TestOpenConcurrentProcessesConvergeOnOneIdentity(t *testing.T) {
	dataDir := t.TempDir()
	const workers = 64
	start := make(chan struct{})
	spaceIDs := make(chan string, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			store, err := Open(dataDir)
			if err != nil {
				errs <- err
				return
			}
			spaceIDs <- store.SpaceID()
		}()
	}
	close(start)
	wg.Wait()
	close(spaceIDs)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	var canonical string
	for spaceID := range spaceIDs {
		if canonical == "" {
			canonical = spaceID
			continue
		}
		if spaceID != canonical {
			t.Fatalf("concurrent identity mismatch: %s != %s", spaceID, canonical)
		}
	}
	if canonical == "" {
		t.Fatal("no concurrent identity was created")
	}
	if _, err := os.Stat(filepath.Join(dataDir, "identity", "space.json")); err != nil {
		t.Fatalf("persisted identity missing: %v", err)
	}
}
