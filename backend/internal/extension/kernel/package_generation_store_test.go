package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func TestPackageGenerationPrepareCommitAndVerify(t *testing.T) {
	store := NewPackageGenerationStore(t.TempDir())
	request := generationRequest(t, "com.example.alpha", "generation-1", "operation-1", map[string]string{
		"manifest.json":       `{"name":"alpha"}`,
		"assets/nested/a.txt": "alpha",
		"assets/b.txt":        "beta",
	})
	prepared, err := store.PrepareGeneration(context.Background(), request)
	if err != nil {
		t.Fatalf("prepare generation: %v", err)
	}
	if prepared.Current.TreeHash == "" {
		t.Fatal("tree hash is empty")
	}
	if filepath.Base(filepath.Dir(prepared.StagingPath)) != "staging" {
		t.Fatalf("unexpected staging path: %s", prepared.StagingPath)
	}
	committed, err := store.CommitGeneration(context.Background(), prepared)
	if err != nil {
		t.Fatalf("commit generation: %v", err)
	}
	if _, err := os.Stat(committed.GenerationPath); err != nil {
		t.Fatalf("generation missing: %v", err)
	}
	if _, err := os.Stat(prepared.StagingPath); !os.IsNotExist(err) {
		t.Fatalf("staging still exists: %v", err)
	}
	if err := store.VerifyGeneration(context.Background(), committed.Current); err != nil {
		t.Fatalf("verify generation: %v", err)
	}
	repeated, err := store.CommitGeneration(context.Background(), prepared)
	if err != nil {
		t.Fatalf("repeat commit: %v", err)
	}
	if repeated.GenerationPath != committed.GenerationPath {
		t.Fatalf("repeat commit returned different path")
	}
	preparedAgain, err := store.PrepareGeneration(context.Background(), request)
	if err != nil {
		t.Fatalf("repeat prepare: %v", err)
	}
	if preparedAgain.StagingPath != "" || preparedAgain.GenerationPath != committed.GenerationPath {
		t.Fatalf("repeat prepare did not reuse committed generation")
	}
}

func TestPackageGenerationCurrentAtomicCASAndRestore(t *testing.T) {
	root := t.TempDir()
	store := NewPackageGenerationStore(root)
	first := prepareAndCommitGeneration(t, store, generationRequest(t, "com.example.alpha", "generation-1", "operation-1", map[string]string{"a.txt": "one"}))
	second := prepareAndCommitGeneration(t, store, generationRequest(t, "com.example.alpha", "generation-2", "operation-2", map[string]string{"a.txt": "two"}))
	if err := store.SwitchCurrent(first.Current.ExtensionID, "", first.Current); err != nil {
		t.Fatalf("switch first current: %v", err)
	}
	current, err := store.ReadCurrent(first.Current.ExtensionID)
	if err != nil {
		t.Fatalf("read first current: %v", err)
	}
	if current.GenerationID != first.Current.GenerationID || current.UpdatedAt.IsZero() {
		t.Fatalf("unexpected first current: %+v", current)
	}
	if err := store.SwitchCurrent(first.Current.ExtensionID, "wrong", second.Current); !errors.Is(err, ErrPackageGenerationCAS) {
		t.Fatalf("expected CAS error, got %v", err)
	}
	if err := store.SwitchCurrent(first.Current.ExtensionID, first.Current.GenerationID, second.Current); err != nil {
		t.Fatalf("switch second current: %v", err)
	}
	if err := store.SwitchCurrent(first.Current.ExtensionID, "stale", second.Current); err != nil {
		t.Fatalf("idempotent current switch: %v", err)
	}
	restored, err := store.RestoreCurrent(first.Current.ExtensionID, second.Current.GenerationID)
	if err != nil {
		t.Fatalf("restore current: %v", err)
	}
	if restored.GenerationID != first.Current.GenerationID {
		t.Fatalf("restored wrong generation: %+v", restored)
	}
	data, err := os.ReadFile(filepath.Join(root, "installations", "com.example.alpha", "current.json"))
	if err != nil {
		t.Fatalf("read current json: %v", err)
	}
	var disk PackageGenerationCurrent
	if err := json.Unmarshal(data, &disk); err != nil {
		t.Fatalf("decode current json: %v", err)
	}
	if disk.ExtensionID == "" || disk.GenerationID == "" || disk.Version == "" || disk.ArtifactID == "" || disk.TreeHash == "" || disk.OperationID == "" || disk.UpdatedAt.IsZero() {
		t.Fatalf("current json incomplete: %+v", disk)
	}
}

func TestPackageGenerationCrashResidueRecoversPrevious(t *testing.T) {
	root := t.TempDir()
	store := NewPackageGenerationStore(root)
	first := prepareAndCommitGeneration(t, store, generationRequest(t, "com.example.crash", "generation-1", "operation-1", map[string]string{"value": "one"}))
	second := prepareAndCommitGeneration(t, store, generationRequest(t, "com.example.crash", "generation-2", "operation-2", map[string]string{"value": "two"}))
	if err := store.SwitchCurrent(first.Current.ExtensionID, "", first.Current); err != nil {
		t.Fatalf("switch first: %v", err)
	}
	if err := store.SwitchCurrent(first.Current.ExtensionID, first.Current.GenerationID, second.Current); err != nil {
		t.Fatalf("switch second: %v", err)
	}
	base := filepath.Join(root, "installations", "com.example.crash")
	if err := os.Remove(filepath.Join(base, "current.json")); err != nil {
		t.Fatalf("remove current residue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(base, ".current-crash.tmp"), []byte("partial"), 0o600); err != nil {
		t.Fatalf("write temp residue: %v", err)
	}
	recovered, err := store.ReadCurrent(first.Current.ExtensionID)
	if err != nil {
		t.Fatalf("recover current: %v", err)
	}
	if recovered.GenerationID != first.Current.GenerationID {
		t.Fatalf("recovered wrong generation: %+v", recovered)
	}
	if _, err := os.Stat(filepath.Join(base, "current.json")); err != nil {
		t.Fatalf("recovered current missing: %v", err)
	}
}

func TestPackageGenerationTreeHashDetectsMutation(t *testing.T) {
	store := NewPackageGenerationStore(t.TempDir())
	committed := prepareAndCommitGeneration(t, store, generationRequest(t, "com.example.hash", "generation-1", "operation-1", map[string]string{"nested/a.txt": "before"}))
	if err := os.WriteFile(filepath.Join(committed.GenerationPath, "nested", "a.txt"), []byte("after"), 0o600); err != nil {
		t.Fatalf("mutate generation: %v", err)
	}
	if err := store.VerifyGeneration(context.Background(), committed.Current); err == nil {
		t.Fatal("mutated generation verified")
	}
}

func TestPackageGenerationCrashStagingAndConflictPreconditions(t *testing.T) {
	root := t.TempDir()
	store := NewPackageGenerationStore(root)
	request := generationRequest(t, "com.example.staging", "generation-1", "operation-1", map[string]string{"value": "one"})
	preparing := filepath.Join(root, "installations", "com.example.staging", "staging", "operation-1.preparing")
	if err := os.MkdirAll(preparing, 0o700); err != nil {
		t.Fatalf("create preparing residue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(preparing, "partial"), []byte("partial"), 0o600); err != nil {
		t.Fatalf("write preparing residue: %v", err)
	}
	prepared, err := store.PrepareGeneration(context.Background(), request)
	if err != nil {
		t.Fatalf("prepare over crash residue: %v", err)
	}
	if _, err := os.Stat(preparing); !os.IsNotExist(err) {
		t.Fatalf("preparing residue remains: %v", err)
	}
	conflicting := generationRequest(t, request.ExtensionID, request.GenerationID, request.OperationID, map[string]string{"value": "different"})
	if _, err := store.PrepareGeneration(context.Background(), conflicting); !errors.Is(err, ErrPackageGenerationConflict) {
		t.Fatalf("expected staged conflict, got %v", err)
	}
	if err := store.VerifyGeneration(context.Background(), PackageGenerationCurrent{}); !errors.Is(err, ErrPackageGenerationUnsafe) {
		t.Fatalf("expected invalid current error, got %v", err)
	}
	if _, err := store.CommitGeneration(context.Background(), prepared); err != nil {
		t.Fatalf("commit original staging: %v", err)
	}
	if _, err := store.PrepareGeneration(context.Background(), conflicting); !errors.Is(err, ErrPackageGenerationConflict) {
		t.Fatalf("expected committed conflict, got %v", err)
	}
}

func TestPackageGenerationCrashDuringWindowsRotationRestoresBothStates(t *testing.T) {
	root := t.TempDir()
	store := NewPackageGenerationStore(root)
	first := prepareAndCommitGeneration(t, store, generationRequest(t, "com.example.rotation", "generation-1", "operation-1", map[string]string{"value": "one"}))
	second := prepareAndCommitGeneration(t, store, generationRequest(t, "com.example.rotation", "generation-2", "operation-2", map[string]string{"value": "two"}))
	if err := store.SwitchCurrent(first.Current.ExtensionID, "", first.Current); err != nil {
		t.Fatalf("switch first: %v", err)
	}
	if err := store.SwitchCurrent(first.Current.ExtensionID, first.Current.GenerationID, second.Current); err != nil {
		t.Fatalf("switch second: %v", err)
	}
	base := filepath.Join(root, "installations", "com.example.rotation")
	currentPath := filepath.Join(base, "current.json")
	previousPath := filepath.Join(base, "previous.json")
	backupPath := filepath.Join(base, "previous.backup.json")
	if err := os.Rename(previousPath, backupPath); err != nil {
		t.Fatalf("rotate previous to backup: %v", err)
	}
	if err := os.Rename(currentPath, previousPath); err != nil {
		t.Fatalf("rotate current to previous: %v", err)
	}
	recovered, err := store.ReadCurrent(first.Current.ExtensionID)
	if err != nil {
		t.Fatalf("recover interrupted rotation: %v", err)
	}
	if recovered.GenerationID != second.Current.GenerationID {
		t.Fatalf("wrong recovered current: %+v", recovered)
	}
	previous, err := readCurrentFile(previousPath)
	if err != nil {
		t.Fatalf("read recovered previous: %v", err)
	}
	if previous.GenerationID != first.Current.GenerationID {
		t.Fatalf("wrong recovered previous: %+v", previous)
	}
}

func TestPackageGenerationRejectsPathEscape(t *testing.T) {
	store := NewPackageGenerationStore(t.TempDir())
	tests := []PackageGenerationPrepareRequest{
		generationRequest(t, "../escape", "generation-1", "operation-1", map[string]string{"a": "a"}),
		generationRequest(t, "com.example.safe", "../generation", "operation-1", map[string]string{"a": "a"}),
		generationRequest(t, "com.example.safe", "generation-1", `..\operation`, map[string]string{"a": "a"}),
	}
	for _, request := range tests {
		if _, err := store.PrepareGeneration(context.Background(), request); !errors.Is(err, ErrPackageGenerationUnsafe) {
			t.Fatalf("expected unsafe path error for %+v, got %v", request, err)
		}
	}
}

func TestPackageGenerationConcurrentCurrentCAS(t *testing.T) {
	store := NewPackageGenerationStore(t.TempDir())
	first := prepareAndCommitGeneration(t, store, generationRequest(t, "com.example.concurrent", "generation-1", "operation-1", map[string]string{"value": "one"}))
	if err := store.SwitchCurrent(first.Current.ExtensionID, "", first.Current); err != nil {
		t.Fatalf("switch first: %v", err)
	}
	candidates := make([]PackagePreparedGeneration, 8)
	for index := range candidates {
		candidates[index] = prepareAndCommitGeneration(t, store, generationRequest(t, "com.example.concurrent", "generation-"+string(rune('a'+index)), "operation-"+string(rune('a'+index)), map[string]string{"value": string(rune('a' + index))}))
	}
	var wait sync.WaitGroup
	results := make(chan error, len(candidates))
	for _, candidate := range candidates {
		wait.Add(1)
		go func(candidate PackagePreparedGeneration) {
			defer wait.Done()
			results <- store.SwitchCurrent(first.Current.ExtensionID, first.Current.GenerationID, candidate.Current)
		}(candidate)
	}
	wait.Wait()
	close(results)
	succeeded := 0
	casFailed := 0
	for err := range results {
		if err == nil {
			succeeded++
		} else if errors.Is(err, ErrPackageGenerationCAS) {
			casFailed++
		} else {
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if succeeded != 1 || casFailed != len(candidates)-1 {
		t.Fatalf("unexpected concurrent results: success=%d cas=%d", succeeded, casFailed)
	}
}

func TestPackageGenerationQuarantineIsIdempotent(t *testing.T) {
	store := NewPackageGenerationStore(t.TempDir())
	committed := prepareAndCommitGeneration(t, store, generationRequest(t, "com.example.quarantine", "generation-1", "operation-1", map[string]string{"value": "one"}))
	path, err := store.QuarantineGeneration(context.Background(), committed.Current)
	if err != nil {
		t.Fatalf("quarantine generation: %v", err)
	}
	repeated, err := store.QuarantineGeneration(context.Background(), committed.Current)
	if err != nil {
		t.Fatalf("repeat quarantine: %v", err)
	}
	if repeated != path {
		t.Fatalf("quarantine paths differ")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("quarantine missing: %v", err)
	}
}

func TestPackageGenerationWindowsReplacementLeavesRecoverablePrevious(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows replacement behavior")
	}
	root := t.TempDir()
	store := NewPackageGenerationStore(root)
	first := prepareAndCommitGeneration(t, store, generationRequest(t, "com.example.windows", "generation-1", "operation-1", map[string]string{"value": "one"}))
	second := prepareAndCommitGeneration(t, store, generationRequest(t, "com.example.windows", "generation-2", "operation-2", map[string]string{"value": "two"}))
	if err := store.SwitchCurrent(first.Current.ExtensionID, "", first.Current); err != nil {
		t.Fatalf("switch first: %v", err)
	}
	if err := store.SwitchCurrent(first.Current.ExtensionID, first.Current.GenerationID, second.Current); err != nil {
		t.Fatalf("switch second: %v", err)
	}
	previous, err := readCurrentFile(filepath.Join(root, "installations", "com.example.windows", "previous.json"))
	if err != nil {
		t.Fatalf("read previous: %v", err)
	}
	if previous.GenerationID != first.Current.GenerationID {
		t.Fatalf("wrong previous generation: %+v", previous)
	}
}

func generationRequest(t *testing.T, extensionID, generationID, operationID string, files map[string]string) PackageGenerationPrepareRequest {
	t.Helper()
	source := t.TempDir()
	for name, content := range files {
		path := filepath.Join(source, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("create source directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write source file: %v", err)
		}
	}
	return PackageGenerationPrepareRequest{ExtensionID: extensionID, GenerationID: generationID, Version: "1.0.0", ArtifactID: "artifact-1", OperationID: operationID, SourcePath: source}
}

func prepareAndCommitGeneration(t *testing.T, store *PackageGenerationStore, request PackageGenerationPrepareRequest) PackagePreparedGeneration {
	t.Helper()
	prepared, err := store.PrepareGeneration(context.Background(), request)
	if err != nil {
		t.Fatalf("prepare generation: %v", err)
	}
	committed, err := store.CommitGeneration(context.Background(), prepared)
	if err != nil {
		t.Fatalf("commit generation: %v", err)
	}
	return committed
}
