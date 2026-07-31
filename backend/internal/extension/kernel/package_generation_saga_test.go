package kernel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func TestPackageGenerationInstallPersistsEvidenceAndReadModel(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0")
	operation, steps, err := container.PackageRepository.GetOperation(ctx, "user-1", installed.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if operation.StableGeneration != "" || operation.TargetGeneration == "" || operation.CurrentPointerJSON == "" {
		t.Fatalf("operation generation evidence incomplete: %+v", operation)
	}
	foundCommit := false
	foundSwitch := false
	for _, step := range steps {
		if step.StepName == StepCommitInstalledTree {
			foundCommit = step.TargetGeneration == operation.TargetGeneration && step.CurrentPointerJSON != ""
		}
		if step.StepName == StepInstallSwitchCurrentPointer {
			foundSwitch = step.TargetGeneration == operation.TargetGeneration && step.CurrentPointerJSON != ""
		}
	}
	if !foundCommit || !foundSwitch {
		t.Fatalf("generation steps missing evidence: %+v", steps)
	}
	installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(installed.ExtensionID))
	if err != nil {
		t.Fatal(err)
	}
	generationID, _ := installation.Metadata["generationId"].(string)
	currentPath, _ := installation.Metadata["currentPath"].(string)
	lastOperation, _ := installation.Metadata["lastOperationId"].(string)
	current, err := container.PackageGenerationStore.ReadCurrent(installed.ExtensionID)
	if err != nil {
		t.Fatal(err)
	}
	if generationID != current.GenerationID || currentPath != installed.InstallPath || lastOperation != installed.OperationID || current.TreeHash == "" {
		t.Fatalf("installation generation read model mismatch: metadata=%+v current=%+v", installation.Metadata, current)
	}
}

func TestPackageGenerationRecoveryCompensatesCurrentDBSplit(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0")
	stable, err := container.PackageGenerationStore.ReadCurrent(installed.ExtensionID)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := container.PackageRepository.GetArtifactByVersion(ctx, installed.ExtensionID, installed.Version)
	if err != nil {
		t.Fatal(err)
	}
	operationID := "package-operation-generation-split"
	prepared, err := container.PackageGenerationStore.PrepareGeneration(ctx, PackageGenerationPrepareRequest{ExtensionID: installed.ExtensionID, GenerationID: "generation-split-target", Version: installed.Version, ArtifactID: artifact.ArtifactID, OperationID: operationID, SourcePath: installed.InstallPath})
	if err != nil {
		t.Fatal(err)
	}
	target, err := container.PackageGenerationStore.CommitGeneration(ctx, prepared)
	if err != nil {
		t.Fatal(err)
	}
	if err := container.PackageGenerationStore.SwitchCurrent(installed.ExtensionID, stable.GenerationID, target.Current); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	operation := PackageOperationRecord{OperationID: operationID, TraceID: "trace-generation-split", UserID: "user-1", ScopeType: "global", ExtensionID: installed.ExtensionID, TargetVersion: installed.Version, OperationType: "update", Status: "in_progress", CurrentStep: "switch_current_pointer", ArtifactID: artifact.ArtifactID, ConfirmationsJSON: "{}", StartedAt: now, UpdatedAt: now, StableGeneration: stable.GenerationID, TargetGeneration: target.Current.GenerationID, CurrentPointerJSON: packageGenerationJSON(target.Current)}
	if err := container.PackageRepository.CreateOperation(ctx, operation); err != nil {
		t.Fatal(err)
	}
	if err := runtime.recoverPackageOperation(ctx, operation); err != nil {
		t.Fatal(err)
	}
	current, err := container.PackageGenerationStore.ReadCurrent(installed.ExtensionID)
	if err != nil {
		t.Fatal(err)
	}
	if current.GenerationID != stable.GenerationID {
		t.Fatalf("current pointer not compensated: %+v", current)
	}
	if err := container.PackageGenerationStore.VerifyGeneration(ctx, target.Current); !errors.Is(err, ErrPackageGenerationNotFound) {
		t.Fatalf("target generation not quarantined: %v", err)
	}
	recovered, _, err := container.PackageRepository.GetOperation(ctx, "user-1", operationID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Status != "failed" || recovered.CurrentStep != "recovered_compensated" {
		t.Fatalf("unexpected recovery outcome: %+v", recovered)
	}
}

func TestPackageGenerationRecoveryRebuildsMissingDBTarget(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0")
	target, err := container.PackageGenerationStore.ReadCurrent(installed.ExtensionID)
	if err != nil {
		t.Fatal(err)
	}
	_, generationPath, err := container.PackageGenerationStore.paths(target.ExtensionID, target.GenerationID, target.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	quarantinePath, err := container.PackageGenerationStore.quarantinePath(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(quarantinePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(generationPath, quarantinePath); err != nil {
		t.Fatal(err)
	}
	artifact, err := container.PackageRepository.GetArtifactByVersion(ctx, installed.ExtensionID, installed.Version)
	if err != nil {
		t.Fatal(err)
	}
	operation := PackageOperationRecord{OperationID: target.OperationID, ExtensionID: installed.ExtensionID, ArtifactID: artifact.ArtifactID, StableGeneration: "", TargetGeneration: target.GenerationID, CurrentPointerJSON: packageGenerationJSON(target)}
	compensated, err := runtime.reconcileInstalledPackageGeneration(ctx, operation)
	if err != nil {
		t.Fatal(err)
	}
	if compensated {
		t.Fatal("database target must recover forward")
	}
	if err := container.PackageGenerationStore.VerifyGeneration(ctx, target); err != nil {
		t.Fatalf("generation was not rebuilt: %v", err)
	}
}

func TestPackageGenerationRecoveryRestoresUninstallQuarantine(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0")
	stable, err := container.PackageGenerationStore.ReadCurrent(installed.ExtensionID)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := container.PackageRepository.GetArtifactByVersion(ctx, installed.ExtensionID, installed.Version)
	if err != nil {
		t.Fatal(err)
	}
	operationID := "package-operation-uninstall-recovery"
	now := time.Now().UTC().Format(time.RFC3339Nano)
	operation := PackageOperationRecord{OperationID: operationID, TraceID: "trace-uninstall-recovery", UserID: "user-1", ScopeType: "global", ExtensionID: installed.ExtensionID, TargetVersion: installed.Version, OperationType: "uninstall", Status: "in_progress", CurrentStep: "move_to_quarantine", ArtifactID: artifact.ArtifactID, ConfirmationsJSON: "{}", StartedAt: now, UpdatedAt: now, StableGeneration: stable.GenerationID, CurrentPointerJSON: packageGenerationJSON(stable)}
	if err := container.PackageRepository.CreateOperation(ctx, operation); err != nil {
		t.Fatal(err)
	}
	quarantinedCurrent, err := container.PackageGenerationStore.QuarantineCurrent(installed.ExtensionID, stable.GenerationID, operationID)
	if err != nil {
		t.Fatal(err)
	}
	quarantinePath, err := container.PackageGenerationStore.QuarantineGeneration(ctx, stable)
	if err != nil {
		t.Fatal(err)
	}
	qm := PackageQuarantineMetadata{
		QuarantineID:             "quarantine-" + operationID,
		OperationID:              operationID,
		ExtensionID:              installed.ExtensionID,
		GenerationQuarantinePath: quarantinePath,
		CurrentQuarantinePath:    quarantinedCurrent.Path,
		OriginalGenerationPath:   installed.InstallPath,
		TreeHash:                 stable.TreeHash,
		ArtifactID:               artifact.ArtifactID,
		State:                    "active",
	}
	if err := container.PackageRepository.PutQuarantineMetadata(ctx, qm, PackageWriteGuard{}); err != nil {
		t.Fatal(err)
	}
	if err := runtime.recoverPackageOperation(ctx, operation); err != nil {
		t.Fatal(err)
	}
	current, err := container.PackageGenerationStore.ReadCurrent(installed.ExtensionID)
	if err != nil {
		t.Fatal(err)
	}
	if current.GenerationID != stable.GenerationID {
		t.Fatalf("uninstall recovery restored wrong pointer: %+v", current)
	}
	if err := container.PackageGenerationStore.VerifyGeneration(ctx, stable); err != nil {
		t.Fatalf("uninstall recovery did not restore generation: %v", err)
	}
}
