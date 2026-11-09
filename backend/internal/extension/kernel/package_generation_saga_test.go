package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func TestPackageReinstallAfterLegacyUninstall(t *testing.T) {
	for _, legacyUninstall := range []bool{true, false} {
		t.Run(map[bool]string{true: "legacy_uninstall", false: "missing_installation_only"}[legacyUninstall], func(t *testing.T) {
			ctx := t.Context()
			runtime, container := newPackagePipelineRuntime(t)
			installed := installPackagePipelineVersion(t, runtime, "1.0.0")
			stable, err := container.PackageGenerationStore.ReadCurrent(installed.ExtensionID)
			if err != nil {
				t.Fatal(err)
			}
			operation, _, err := container.PackageRepository.GetOperation(ctx, "user-1", installed.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			session, err := container.PackageRepository.GetPreview(ctx, operation.PreviewSessionID, "user-1", "global", "")
			if err != nil {
				t.Fatal(err)
			}
			var original InstallPreview
			if err := json.Unmarshal([]byte(session.PreviewResultJSON), &original); err != nil {
				t.Fatal(err)
			}
			if legacyUninstall {
				err = runtime.Uninstall(ctx, installed.ExtensionID)
			} else {
				err = container.InstallationRepository.DeleteInstallation(ctx, domain.ExtensionID(installed.ExtensionID))
			}
			if err != nil {
				t.Fatal(err)
			}
			reinstallArchivePath := installed.ArtifactPath
			if legacyUninstall {
				reinstallArchivePath = createPackagePipelineArchive(t, "1.0.0")
			}
			archive, err := os.Open(reinstallArchivePath)
			if err != nil {
				t.Fatal(err)
			}
			defer archive.Close()
			preview, err := runtime.PreviewPackage(ctx, PackagePreviewRequest{
				SpaceID: "user-1", ScopeType: "global", FileName: "pipeline.amitiax",
				AllowUnsignedDev: true, DeveloperSessionID: original.DeveloperSessionID,
			}, archive)
			if err != nil {
				t.Fatal(err)
			}
			if legacyUninstall && preview.ArtifactID == stable.ArtifactID {
				t.Fatalf("reinstall fixture reused the installed artifact: %s", preview.ArtifactID)
			}
			confirmations := make(map[string]bool)
			for _, name := range preview.RequiredConfirmations {
				confirmations[name] = true
			}
			confirmation, err := runtime.ConfirmPackagePreview(ctx, PackagePreviewConfirmationRequest{
				SessionID: preview.SessionID, SpaceID: "user-1", ScopeType: "global", Confirmations: confirmations,
			})
			if err != nil {
				t.Fatal(err)
			}
			reinstalled, installErr := runtime.ExecutePackageInstall(ctx, PackageInstallRequest{
				SessionID: preview.SessionID, SpaceID: "user-1", ScopeType: "global",
				ConfirmationToken: confirmation.ConfirmationToken, IdempotencyKey: "reinstall-legacy",
			})
			current, err := container.PackageGenerationStore.ReadCurrent(installed.ExtensionID)
			if err != nil {
				t.Fatal(err)
			}
			if !legacyUninstall {
				if installErr == nil || current != stable {
					t.Fatalf("unproven orphan was replaced: current=%+v err=%v", current, installErr)
				}
				return
			}
			if installErr != nil || current.GenerationID == stable.GenerationID || current.OperationID != reinstalled.OperationID {
				t.Fatalf("reinstall did not activate a new generation: current=%+v err=%v", current, installErr)
			}
			if err := container.PackageGenerationStore.VerifyGeneration(ctx, stable); err != nil {
				t.Fatalf("old generation was lost: %v", err)
			}
		})
	}
}

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
	operation := PackageOperationRecord{OperationID: operationID, TraceID: "trace-generation-split", SpaceID: "user-1", ScopeType: "global", ExtensionID: installed.ExtensionID, TargetVersion: installed.Version, OperationType: "update", Status: "in_progress", CurrentStep: "switch_current_pointer", ArtifactID: artifact.ArtifactID, StartedAt: now, UpdatedAt: now, StableGeneration: stable.GenerationID, TargetGeneration: target.Current.GenerationID, CurrentPointerJSON: packageGenerationJSON(target.Current)}
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
	operation := PackageOperationRecord{OperationID: operationID, TraceID: "trace-uninstall-recovery", SpaceID: "user-1", ScopeType: "global", ExtensionID: installed.ExtensionID, TargetVersion: installed.Version, OperationType: "uninstall", Status: "in_progress", CurrentStep: "move_to_quarantine", ArtifactID: artifact.ArtifactID, StartedAt: now, UpdatedAt: now, StableGeneration: stable.GenerationID, CurrentPointerJSON: packageGenerationJSON(stable)}
	if err := container.PackageRepository.CreateOperation(ctx, operation); err != nil {
		t.Fatal(err)
	}
	installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(installed.ExtensionID))
	if err != nil {
		t.Fatal(err)
	}
	preview, err := runtime.PreviewPackageUninstall(ctx, installed.ExtensionID, "user-1", "global", "")
	if err != nil {
		t.Fatal(err)
	}
	snapshotJSON, snapshotHash, expectedGenerationID, err := captureInstallationSnapshot(installation, preview)
	if err != nil {
		t.Fatal(err)
	}
	quarantinedCurrent, err := container.PackageGenerationStore.QuarantineCurrent(installed.ExtensionID, stable.GenerationID, operationID)
	if err != nil {
		t.Fatal(err)
	}
	stableForQuarantine := stable
	stableForQuarantine.OperationID = operationID
	quarantinePath, err := container.PackageGenerationStore.QuarantineGeneration(ctx, stableForQuarantine)
	if err != nil {
		t.Fatal(err)
	}
	safeExt := safeDirectoryName(installed.ExtensionID)
	originalCurrentPath := filepath.Join(container.ExtRoot, "installations", safeExt, "current.json")
	verificationTreeHash, err := computeGenerationTreeHash(ctx, quarantinePath)
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
		OriginalCurrentPath:      originalCurrentPath,
		TreeHash:                 verificationTreeHash,
		ArtifactID:               artifact.ArtifactID,
		State:                    "active",
		FencingToken:             1,
		SnapshotJSON:             snapshotJSON,
		SnapshotHash:             snapshotHash,
		ExpectedGenerationID:     expectedGenerationID,
		ExpectedVersionID:        preview.CurrentVersionID,
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
