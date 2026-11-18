package kernel

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func p14SupersedeCommit(t *testing.T, store *PackageGenerationStore, extensionID, generationID, operationID, version string) PackageGenerationCurrent {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(dir, "index.js")
	if err := os.WriteFile(source, []byte(version), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = source
	prepared, err := store.PrepareGeneration(context.Background(), PackageGenerationPrepareRequest{
		ExtensionID:  extensionID,
		GenerationID: generationID,
		Version:      version,
		ArtifactID:   "artifact-" + version,
		OperationID:  operationID,
		SourcePath:   dir,
	})
	if err != nil {
		t.Fatalf("prepare %s: %v", generationID, err)
	}
	committed, err := store.CommitGeneration(context.Background(), prepared)
	if err != nil {
		t.Fatalf("commit %s: %v", generationID, err)
	}
	if err := store.SwitchCurrent(extensionID, "", committed.Current); err != nil {
		t.Fatalf("switch current %s: %v", generationID, err)
	}
	return committed.Current
}

func TestP14SupersededOperationResolvesToCompletedNewerOperation(t *testing.T) {
	ctx := context.Background()
	runtime, repo := newR45FinalGateTestRuntime(t)
	store := NewPackageGenerationStore(t.TempDir())
	runtime.container.PackageGenerationStore = store

	extensionID := "com.amitia/p14-supersede"
	current := p14SupersedeCommit(t, store, extensionID, "generation-new", "op-new", "2.0.0")
	if current.OperationID != "op-new" {
		t.Fatalf("expected current pointer to reference op-new, got %q", current.OperationID)
	}

	stale := PackageOperationRecord{
		OperationID:    "op-old",
		SpaceID:        "space-1",
		ExtensionID:    extensionID,
		OperationType:  "update",
		TargetVersion:  "2.0.0",
		Status:         string(PackageOperationRequiresRecovery),
		CurrentStep:    "verify",
		ArtifactID:     "",
		PreviewSessionID: "",
	}
	if err := repo.CreateOperation(ctx, stale); err != nil {
		t.Fatalf("create stale operation: %v", err)
	}
	newer := stale
	newer.OperationID = "op-new"
	newer.Status = string(PackageOperationCompleted)
	newer.CurrentStep = "finalize"
	newer.PreviewSessionID = ""
	if err := repo.CreateOperation(ctx, newer); err != nil {
		t.Fatalf("create newer operation: %v", err)
	}

	superseded, supersededBy := runtime.resolveSupersededPackageOperation(ctx, stale)
	if !superseded || supersededBy != "op-new" {
		t.Fatalf("expected stale operation superseded by op-new, got superseded=%v by=%q", superseded, supersededBy)
	}
}

func TestP14SupersedeRequiresMatchingTargetVersion(t *testing.T) {
	ctx := context.Background()
	runtime, repo := newR45FinalGateTestRuntime(t)
	store := NewPackageGenerationStore(t.TempDir())
	runtime.container.PackageGenerationStore = store

	extensionID := "com.amitia/p14-supersede-mismatch"
	p14SupersedeCommit(t, store, extensionID, "generation-210", "op-210", "2.1.0")

	stale := PackageOperationRecord{
		OperationID:     "op-stale",
		SpaceID:         "space-1",
		ExtensionID:     extensionID,
		OperationType:   "update",
		TargetVersion:   "2.0.0",
		Status:          string(PackageOperationRequiresRecovery),
		CurrentStep:     "verify",
		ArtifactID:      "",
		PreviewSessionID: "",
	}
	if err := repo.CreateOperation(ctx, stale); err != nil {
		t.Fatalf("create stale operation: %v", err)
	}
	newer := stale
	newer.OperationID = "op-210"
	newer.TargetVersion = "2.1.0"
	newer.Status = string(PackageOperationCompleted)
	newer.CurrentStep = "finalize"
	newer.PreviewSessionID = ""
	if err := repo.CreateOperation(ctx, newer); err != nil {
		t.Fatalf("create newer operation: %v", err)
	}

	if superseded, supersededBy := runtime.resolveSupersededPackageOperation(ctx, stale); superseded {
		t.Fatalf("operation with a different target version must not be superseded (by=%q)", supersededBy)
	}
}

func TestP14RecoveryFinalizesSupersededOperation(t *testing.T) {
	ctx := context.Background()
	runtime, repo := newR45FinalGateTestRuntime(t)
	store := NewPackageGenerationStore(t.TempDir())
	runtime.container.PackageGenerationStore = store

	extensionID := "com.amitia/p14-supersede-recovery"
	p14SupersedeCommit(t, store, extensionID, "generation-next", "op-next", "3.0.0")

	stale := PackageOperationRecord{
		OperationID:     "op-stale-recovery",
		SpaceID:         "space-1",
		ExtensionID:     extensionID,
		OperationType:   "update",
		TargetVersion:   "3.0.0",
		Status:          string(PackageOperationRequiresRecovery),
		CurrentStep:     "verify",
		ArtifactID:      "",
		PreviewSessionID: "",
	}
	if err := repo.CreateOperation(ctx, stale); err != nil {
		t.Fatalf("create stale operation: %v", err)
	}
	newer := stale
	newer.OperationID = "op-next"
	newer.Status = string(PackageOperationCompleted)
	newer.CurrentStep = "finalize"
	if err := repo.CreateOperation(ctx, newer); err != nil {
		t.Fatalf("create newer operation: %v", err)
	}

	if err := runtime.recoverPackageOperation(ctx, stale); err != nil {
		t.Fatalf("recover superseded operation: %v", err)
	}
	finalized, _, err := repo.GetOperation(ctx, stale.SpaceID, stale.OperationID)
	if err != nil {
		t.Fatalf("read finalized operation: %v", err)
	}
	if finalized.Status != string(PackageOperationCompleted) || finalized.CurrentStep != "superseded" {
		t.Fatalf("expected completed/superseded, got status=%q step=%q", finalized.Status, finalized.CurrentStep)
	}
}
