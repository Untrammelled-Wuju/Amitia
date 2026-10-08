package kernel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestPackageGenerationExecutionLeaseRetainsFilesAcrossStoreInstancesAndCutover(t *testing.T) {
	root := t.TempDir()
	store := NewPackageGenerationStore(root)
	other := NewPackageGenerationStore(filepath.Join(root, "."))
	first := prepareAndCommitGeneration(t, store, generationRequest(t, "com.example.alpha", "generation-1", "operation-1", map[string]string{"manifest.json": "{}", "task.cjs": "old"}))
	second := prepareAndCommitGeneration(t, store, generationRequest(t, "com.example.alpha", "generation-2", "operation-2", map[string]string{"manifest.json": "{}", "task.cjs": "new"}))
	if err := store.SwitchCurrent(first.Current.ExtensionID, "", first.Current); err != nil {
		t.Fatal(err)
	}
	var releases []func()
	for index := 0; index < 32; index++ {
		path, release, err := other.AcquireCurrentExecution(t.Context(), first.Current.ExtensionID, first.Current.GenerationID, "sha256:"+first.Current.TreeHash)
		if err != nil || !samePath(path, first.GenerationPath) {
			t.Fatalf("acquire old generation: %q %v", path, err)
		}
		releases = append(releases, release)
	}
	t.Cleanup(func() {
		for _, release := range releases {
			release()
		}
	})
	if err := store.SwitchCurrent(second.Current.ExtensionID, first.Current.GenerationID, second.Current); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.AcquireCurrentExecution(t.Context(), first.Current.ExtensionID, first.Current.GenerationID, first.Current.TreeHash); !errors.Is(err, ErrPackageGenerationCAS) {
		t.Fatalf("old generation acquired after cutover: %v", err)
	}
	if _, err := store.QuarantineGeneration(t.Context(), first.Current); !errors.Is(err, ErrPackageGenerationInUse) {
		t.Fatalf("leased generation removed: %v", err)
	}
	if bytes, err := os.ReadFile(filepath.Join(first.GenerationPath, "task.cjs")); err != nil || string(bytes) != "old" {
		t.Fatalf("old execution lost its file: %s %v", bytes, err)
	}
	var workers sync.WaitGroup
	for _, release := range releases {
		for index := 0; index < 2; index++ {
			workers.Add(1)
			go func(release func()) { defer workers.Done(); release() }(release)
		}
	}
	workers.Wait()
	if _, err := store.QuarantineGeneration(t.Context(), first.Current); err != nil {
		t.Fatalf("released generation cannot be removed: %v", err)
	}
	if _, err := os.Stat(first.GenerationPath); !os.IsNotExist(err) {
		t.Fatalf("generation was not quarantined: %v", err)
	}
}

func TestPackageGenerationExecutionLeaseRejectsChangedIdentityCancellationAndOverflow(t *testing.T) {
	store := NewPackageGenerationStore(t.TempDir())
	prepared := prepareAndCommitGeneration(t, store, generationRequest(t, "com.example.alpha", "generation-1", "operation-1", map[string]string{"manifest.json": "{}"}))
	if err := store.SwitchCurrent(prepared.Current.ExtensionID, "", prepared.Current); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := store.AcquireCurrentExecution(cancelled, prepared.Current.ExtensionID, prepared.Current.GenerationID, prepared.Current.TreeHash); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled lease accepted: %v", err)
	}
	if _, _, err := store.AcquireCurrentExecution(t.Context(), prepared.Current.ExtensionID, prepared.Current.GenerationID, "changed"); !errors.Is(err, ErrPackageGenerationCAS) {
		t.Fatalf("changed hash accepted: %v", err)
	}
	var releases []func()
	t.Cleanup(func() {
		for _, release := range releases {
			release()
		}
	})
	for index := 0; index < 128; index++ {
		_, release, err := store.AcquireCurrentExecution(t.Context(), prepared.Current.ExtensionID, prepared.Current.GenerationID, prepared.Current.TreeHash)
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	if _, _, err := store.AcquireCurrentExecution(t.Context(), prepared.Current.ExtensionID, prepared.Current.GenerationID, prepared.Current.TreeHash); err == nil {
		t.Fatal("unbounded execution leases accepted")
	}
}
