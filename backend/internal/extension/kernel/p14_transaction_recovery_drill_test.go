package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const p14DrillExtensionID = "com.amitia/p14-drill"

func p14DrillSource(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func p14DrillCommit(t *testing.T, store *PackageGenerationStore, generationID, operationID, version, body string) PackagePreparedGeneration {
	t.Helper()
	source := p14DrillSource(t, "index.js", body)
	prepared, err := store.PrepareGeneration(context.Background(), PackageGenerationPrepareRequest{
		ExtensionID:  p14DrillExtensionID,
		GenerationID: generationID,
		Version:      version,
		ArtifactID:   "artifact-" + version,
		OperationID:  operationID,
		SourcePath:   source,
	})
	if err != nil {
		t.Fatalf("prepare %s: %v", generationID, err)
	}
	committed, err := store.CommitGeneration(context.Background(), prepared)
	if err != nil {
		t.Fatalf("commit %s: %v", generationID, err)
	}
	return committed
}

func p14DrillStatePath(root, name string) string {
	return filepath.Join(root, "installations", safeDirectoryName(p14DrillExtensionID), name)
}

func p14DrillReadCurrentFile(t *testing.T, root string) (PackageGenerationCurrent, bool) {
	t.Helper()
	raw, err := os.ReadFile(p14DrillStatePath(root, "current.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return PackageGenerationCurrent{}, false
		}
		t.Fatal(err)
	}
	var current PackageGenerationCurrent
	if err := json.Unmarshal(raw, &current); err != nil {
		t.Fatal(err)
	}
	return current, true
}

func p14DrillStagingEntries(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(p14DrillStatePath(root, "staging"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestP14GenerationPointerCrashRecoversPreviousGeneration(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store := NewPackageGenerationStore(root)

	first := p14DrillCommit(t, store, "generation-p14-a", "package-operation-p14-a", "1.0.0", "first")
	if err := store.SwitchCurrent(p14DrillExtensionID, "", first.Current); err != nil {
		t.Fatalf("switch to first generation: %v", err)
	}
	second := p14DrillCommit(t, store, "generation-p14-b", "package-operation-p14-b", "1.1.0", "second")
	if err := store.SwitchCurrent(p14DrillExtensionID, first.Current.GenerationID, second.Current); err != nil {
		t.Fatalf("switch to second generation: %v", err)
	}
	if current, ok := p14DrillReadCurrentFile(t, root); !ok || current.GenerationID != second.Current.GenerationID {
		t.Fatalf("expected current pointer at %s, got ok=%v current=%+v", second.Current.GenerationID, ok, current)
	}

	if err := os.Remove(p14DrillStatePath(root, "current.json")); err != nil {
		t.Fatalf("simulate crash between unlink and write: %v", err)
	}

	recovered, err := store.ReadCurrent(p14DrillExtensionID)
	if err != nil {
		t.Fatalf("recover current pointer from previous: %v", err)
	}
	if recovered.GenerationID != first.Current.GenerationID {
		t.Fatalf("recovered generation=%s, want %s", recovered.GenerationID, first.Current.GenerationID)
	}
	if recovered.TreeHash != first.Current.TreeHash {
		t.Fatalf("recovered tree hash=%s, want %s", recovered.TreeHash, first.Current.TreeHash)
	}
	if recovered.Version != "1.0.0" {
		t.Fatalf("recovered version=%s, want 1.0.0", recovered.Version)
	}
	if onDisk, ok := p14DrillReadCurrentFile(t, root); !ok || onDisk.GenerationID != first.Current.GenerationID {
		t.Fatalf("recovered pointer was not persisted: ok=%v current=%+v", ok, onDisk)
	}

	resolved, generationPath, err := store.ResolveCurrentGeneration(ctx, p14DrillExtensionID)
	if err != nil {
		t.Fatalf("resolve recovered generation: %v", err)
	}
	if resolved.GenerationID != first.Current.GenerationID {
		t.Fatalf("resolved generation=%s, want %s", resolved.GenerationID, first.Current.GenerationID)
	}
	if info, err := os.Stat(generationPath); err != nil || !info.IsDir() {
		t.Fatalf("recovered generation tree missing at %s: info=%v err=%v", generationPath, info, err)
	}

	generations, err := os.ReadDir(p14DrillStatePath(root, "generations"))
	if err != nil {
		t.Fatal(err)
	}
	if len(generations) != 2 {
		t.Fatalf("generation directories=%d, want 2 (pointer crash must not drop trees)", len(generations))
	}
	if staged := p14DrillStagingEntries(t, root); len(staged) != 0 {
		t.Fatalf("staging residue after commit: %v", staged)
	}
}

func TestP14StaleGenerationSwitchIsRejectedAfterRecovery(t *testing.T) {
	root := t.TempDir()
	store := NewPackageGenerationStore(root)

	first := p14DrillCommit(t, store, "generation-p14-c", "package-operation-p14-c", "1.0.0", "first")
	if err := store.SwitchCurrent(p14DrillExtensionID, "", first.Current); err != nil {
		t.Fatalf("switch to first generation: %v", err)
	}
	second := p14DrillCommit(t, store, "generation-p14-d", "package-operation-p14-d", "1.1.0", "second")
	if err := store.SwitchCurrent(p14DrillExtensionID, first.Current.GenerationID, second.Current); err != nil {
		t.Fatalf("switch to second generation: %v", err)
	}

	if err := os.Remove(p14DrillStatePath(root, "current.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadCurrent(p14DrillExtensionID); err != nil {
		t.Fatalf("recover current pointer: %v", err)
	}

	err := store.SwitchCurrent(p14DrillExtensionID, second.Current.GenerationID, second.Current)
	if !errors.Is(err, ErrPackageGenerationCAS) {
		t.Fatalf("stale switch error=%v, want %v", err, ErrPackageGenerationCAS)
	}
	current, err := store.ReadCurrent(p14DrillExtensionID)
	if err != nil {
		t.Fatal(err)
	}
	if current.GenerationID != first.Current.GenerationID {
		t.Fatalf("stale switch mutated current to %s, want %s", current.GenerationID, first.Current.GenerationID)
	}
}

func TestP14InterruptedPrepareLeavesOnlyItsOwnStaging(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store := NewPackageGenerationStore(root)

	request := PackageGenerationPrepareRequest{
		ExtensionID:  p14DrillExtensionID,
		GenerationID: "generation-p14-e",
		Version:      "2.0.0",
		ArtifactID:   "artifact-2.0.0",
		OperationID:  "package-operation-p14-e",
		SourcePath:   p14DrillSource(t, "index.js", "interrupted"),
	}
	prepared, err := store.PrepareGeneration(ctx, request)
	if err != nil {
		t.Fatalf("prepare before interruption: %v", err)
	}
	if staged := p14DrillStagingEntries(t, root); len(staged) != 1 || staged[0] != request.OperationID {
		t.Fatalf("staging after interruption=%v, want [%s]", staged, request.OperationID)
	}
	if err := os.MkdirAll(p14DrillStatePath(root, filepath.Join("staging", "package-operation-p14-unowned")), 0o700); err != nil {
		t.Fatal(err)
	}

	resumed, err := store.PrepareGeneration(ctx, request)
	if err != nil {
		t.Fatalf("resume prepare after interruption: %v", err)
	}
	if resumed.StagingPath != prepared.StagingPath {
		t.Fatalf("resume staging=%s, want %s", resumed.StagingPath, prepared.StagingPath)
	}
	if _, err := store.CommitGeneration(ctx, resumed); err != nil {
		t.Fatalf("commit resumed generation: %v", err)
	}

	staged := p14DrillStagingEntries(t, root)
	if len(staged) != 1 || staged[0] != "package-operation-p14-unowned" {
		t.Fatalf("staging residue after commit=%v, want only the unowned directory preserved", staged)
	}
	if _, _, err := store.ResolveCurrentGeneration(ctx, p14DrillExtensionID); !errors.Is(err, ErrPackageGenerationNotFound) {
		t.Fatalf("resolve current before pointer switch error=%v, want %v", err, ErrPackageGenerationNotFound)
	}
}
