package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/package_security"
)

func (r *Runtime) RecoverPackageOperations(ctx context.Context) error {
	if r.container == nil || r.container.PackageRepository == nil {
		return nil
	}
	operations, err := r.container.PackageRepository.ListIncompleteOperations(ctx)
	if err != nil {
		return err
	}
	var failures []error
	for _, operation := range operations {
		if err := r.recoverPackageOperation(ctx, operation); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", operation.OperationID, err))
		}
	}
	return errors.Join(failures...)
}

func (r *Runtime) recoverPackageOperation(ctx context.Context, operation PackageOperationRecord) error {
	lease, leaseErr := r.acquirePackageExtensionLease(ctx, operation.ExtensionID, operation.OperationID)
	if leaseErr != nil {
		return nil
	}
	guard := packageWriteGuard(lease)
	defer func() {
		if releaseErr := r.releasePackageExtensionLease(context.Background(), operation.ExtensionID, operation.OperationID); releaseErr != nil {
			_ = r.container.PackageRepository.PutConsistencyFinding(context.Background(), PackageConsistencyFinding{
				FindingID:         "stale-lease-" + operation.OperationID,
				Metric:            "stale_extension_leases",
				Count:             1,
				ResourceIDsJSON:   `["` + operation.OperationID + `"]`,
				ErrorDetail:       releaseErr.Error(),
				RecommendedAction: "manual_lease_cleanup",
			})
		}
	}()
	_, steps, err := r.container.PackageRepository.GetOperation(ctx, operation.UserID, operation.OperationID)
	if err != nil {
		return r.requirePackageRecovery(ctx, operation, "operation journal unavailable", err, guard)
	}
	completed := completedPackageSteps(steps)
	switch operation.OperationType {
	case "install", "update":
		compensated, reconcileErr := r.reconcileInstalledPackageGeneration(ctx, operation)
		if reconcileErr != nil {
			return r.requirePackageRecovery(ctx, operation, "generation reconciliation failed", reconcileErr, guard)
		}
		if compensated {
			return r.container.PackageRepository.SetOperation(ctx, operation.OperationID, "failed", "recovered_compensated", "PACKAGE_INSTALL_FAILED", "generation switch compensated during recovery", true, guard)
		}
		err = r.proveInstalledPackageOperation(ctx, operation, completed)
		if err == nil {
			if operation.PreviewSessionID != "" {
				if consumeErr := r.container.PackageRepository.ConsumePreview(ctx, operation.PreviewSessionID); consumeErr != nil && !strings.Contains(consumeErr.Error(), "already consumed") {
					return r.requirePackageRecovery(ctx, operation, "preview completion could not be persisted", consumeErr, guard)
				}
			}
			return r.container.PackageRepository.SetOperation(ctx, operation.OperationID, "completed", "recovered_completed", "", "", true, guard)
		}
		return r.requirePackageRecovery(ctx, operation, "installed package consistency could not be proven", err, guard)
	case "rollback":
		compensated, reconcileErr := r.reconcileInstalledPackageGeneration(ctx, operation)
		if reconcileErr != nil {
			return r.requirePackageRecovery(ctx, operation, "rollback generation reconciliation failed", reconcileErr, guard)
		}
		if compensated {
			return r.container.PackageRepository.SetOperation(ctx, operation.OperationID, "failed", "recovered_compensated", "PACKAGE_ROLLBACK_FAILED", "rollback generation switch compensated during recovery", true, guard)
		}
		err = r.proveRollbackPackageOperation(ctx, operation, completed)
		if err == nil {
			return r.container.PackageRepository.SetOperation(ctx, operation.OperationID, "completed", "recovered_completed", "", "", true, guard)
		}
		return r.requirePackageRecovery(ctx, operation, "rollback consistency could not be proven", err, guard)
	case "uninstall":
		outcome, reconcileErr := r.reconcileUninstallPackageGeneration(ctx, operation)
		if reconcileErr != nil {
			return r.requirePackageRecovery(ctx, operation, "uninstall generation reconciliation failed", reconcileErr, guard)
		}
		if outcome == "compensated" {
			return r.container.PackageRepository.SetOperation(ctx, operation.OperationID, "failed", "recovered_compensated", "PACKAGE_UNINSTALL_FAILED", "uninstall quarantine restored during recovery", true, guard)
		}
		if outcome == "completed" {
			return r.container.PackageRepository.SetOperation(ctx, operation.OperationID, "completed", "recovered_completed", "", "", true, guard)
		}
		err = r.proveUninstalledPackageOperation(ctx, operation, completed)
		if err == nil {
			return r.container.PackageRepository.SetOperation(ctx, operation.OperationID, "completed", "recovered_completed", "", "", true, guard)
		}
		return r.requirePackageRecovery(ctx, operation, "uninstall consistency could not be proven", err, guard)
	default:
		return r.requirePackageRecovery(ctx, operation, "unsupported package operation type", nil, guard)
	}
}

func (r *Runtime) reconcileUninstallPackageGeneration(ctx context.Context, operation PackageOperationRecord) (string, error) {
	if r.container.PackageGenerationStore == nil {
		return "", errors.New("generation store unavailable")
	}
	var stable PackageGenerationCurrent
	if err := json.Unmarshal([]byte(operation.CurrentPointerJSON), &stable); err != nil || stable.GenerationID == "" {
		return "", errors.New("stable generation evidence unavailable")
	}
	_, dbErr := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(operation.ExtensionID))
	current, currentErr := r.container.PackageGenerationStore.ReadCurrent(operation.ExtensionID)
	if dbErr == nil {
		if currentErr == nil {
			if current.GenerationID != stable.GenerationID {
				return "", fmt.Errorf("unexpected current generation %s", current.GenerationID)
			}
			return "compensated", nil
		}
		if !errors.Is(currentErr, ErrPackageGenerationNotFound) {
			return "", currentErr
		}
		state := PackageQuarantinedCurrent{Current: stable, Path: filepath.Join(r.container.ExtRoot, "quarantine", "current", safeDirectoryName(operation.ExtensionID), operation.OperationID)}
		if err := r.container.PackageGenerationStore.RestoreQuarantinedGeneration(ctx, stable); err != nil {
			return "", err
		}
		if err := r.container.PackageGenerationStore.RestoreQuarantinedCurrent(state); err != nil {
			return "", err
		}
		return "compensated", r.container.PackageGenerationStore.VerifyGeneration(ctx, stable)
	}
	if !errors.Is(dbErr, domain.ErrInvalidExtensionID) {
		return "", dbErr
	}
	if currentErr == nil {
		return "", errors.New("database removed while current pointer remains")
	}
	if !errors.Is(currentErr, ErrPackageGenerationNotFound) {
		return "", currentErr
	}
	quarantine, err := r.container.PackageGenerationStore.quarantinePath(stable)
	if err != nil {
		return "", err
	}
	currentQuarantine := filepath.Join(r.container.ExtRoot, "quarantine", "current", safeDirectoryName(operation.ExtensionID), operation.OperationID)
	if err := os.RemoveAll(quarantine); err != nil {
		return "", err
	}
	if err := os.RemoveAll(currentQuarantine); err != nil {
		return "", err
	}
	return "completed", nil
}

func completedPackageSteps(steps []PackageOperationStep) map[string]PackageOperationStep {
	result := make(map[string]PackageOperationStep, len(steps))
	for _, step := range steps {
		if step.Status == "completed" {
			result[step.StepName] = step
		}
	}
	return result
}

func (r *Runtime) reconcileInstalledPackageGeneration(ctx context.Context, operation PackageOperationRecord) (bool, error) {
	if r.container.PackageGenerationStore == nil {
		return false, errors.New("generation store unavailable")
	}
	var target PackageGenerationCurrent
	if err := json.Unmarshal([]byte(operation.CurrentPointerJSON), &target); err != nil || target.GenerationID == "" {
		return false, errors.New("target generation evidence unavailable")
	}
	installation, dbErr := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(operation.ExtensionID))
	dbGeneration := ""
	if dbErr == nil {
		dbGeneration, _ = installation.Metadata["generationId"].(string)
	}
	current, currentErr := r.container.PackageGenerationStore.ReadCurrent(operation.ExtensionID)
	currentGeneration := ""
	if currentErr == nil {
		currentGeneration = current.GenerationID
	} else if !errors.Is(currentErr, ErrPackageGenerationNotFound) {
		return false, currentErr
	}
	if dbGeneration == target.GenerationID {
		if currentGeneration == target.GenerationID {
			verifyErr := r.container.PackageGenerationStore.VerifyGeneration(ctx, target)
			if verifyErr == nil {
				return false, nil
			}
			if !errors.Is(verifyErr, ErrPackageGenerationNotFound) {
				return false, verifyErr
			}
		}
		artifact, err := r.container.PackageRepository.GetArtifact(ctx, operation.ArtifactID)
		if err != nil {
			return false, err
		}
		if _, err := r.VerifyStoredPackage(ctx, artifact); err != nil {
			return false, err
		}
		staging, err := r.container.PackageSecurity.ExtractFileToStaging(ctx, artifact.ArchivePath, operation.OperationID+"-recovery")
		if err != nil {
			return false, err
		}
		defer r.container.PackageSecurity.GetStagingManager().Cleanup(context.Background(), staging.ID)
		prepared, err := r.container.PackageGenerationStore.PrepareGeneration(ctx, PackageGenerationPrepareRequest{ExtensionID: target.ExtensionID, GenerationID: target.GenerationID, Version: target.Version, ArtifactID: target.ArtifactID, OperationID: target.OperationID, SourcePath: staging.Path, ExpectedTreeHash: target.TreeHash})
		if err != nil {
			return false, err
		}
		committed, err := r.container.PackageGenerationStore.CommitGeneration(ctx, prepared)
		if err != nil {
			return false, err
		}
		expected := operation.StableGeneration
		if currentGeneration != "" {
			expected = currentGeneration
		}
		if err := r.container.PackageGenerationStore.SwitchCurrent(operation.ExtensionID, expected, committed.Current); err != nil {
			return false, err
		}
		return false, nil
	}
	if currentGeneration != "" && currentGeneration != operation.StableGeneration && currentGeneration != target.GenerationID {
		return false, fmt.Errorf("unexpected current generation %s", currentGeneration)
	}
	prepared := PackagePreparedGeneration{Current: target}
	if currentGeneration == target.GenerationID {
		if err := r.compensatePackageGeneration(ctx, PackageGenerationCurrent{GenerationID: operation.StableGeneration}, prepared, true); err != nil {
			return false, err
		}
	} else if err := r.container.PackageGenerationStore.VerifyGeneration(ctx, target); err == nil {
		if _, err := r.container.PackageGenerationStore.QuarantineGeneration(ctx, target); err != nil {
			return false, err
		}
	} else if !errors.Is(err, ErrPackageGenerationNotFound) {
		return false, err
	}
	return true, nil
}

func (r *Runtime) proveInstalledPackageOperation(ctx context.Context, operation PackageOperationRecord, completed map[string]PackageOperationStep) error {
	commitTree, ok := completed["commit_installed_tree"]
	if !ok {
		return errors.New("installed tree commit step missing")
	}
	if _, ok := completed["commit_kernel_repositories"]; !ok {
		return errors.New("kernel repository commit step missing")
	}
	installation, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(operation.ExtensionID))
	if err != nil {
		return fmt.Errorf("installation unavailable: %w", err)
	}
	if installation.PackageID != operation.ArtifactID || installation.InstalledVersion.String() != operation.TargetVersion || installation.InstallationState != domain.InstallationStateInstalled {
		return errors.New("installation identity mismatch")
	}
	artifact, err := r.container.PackageRepository.GetArtifact(ctx, operation.ArtifactID)
	if err != nil {
		return fmt.Errorf("artifact unavailable: %w", err)
	}
	if artifact.ExtensionID != operation.ExtensionID || artifact.Version != operation.TargetVersion {
		return errors.New("artifact identity mismatch")
	}
	if err := r.container.PackageArtifactStore.VerifyArchive(artifact); err != nil {
		return fmt.Errorf("artifact verification failed: %w", err)
	}
	installedPath, _ := installation.Metadata["installedPath"].(string)
	if installedPath == "" || artifact.InstalledPath == "" || filepath.Clean(installedPath) != filepath.Clean(artifact.InstalledPath) {
		return errors.New("installed path identity mismatch")
	}
	var commitResult map[string]string
	if err := json.Unmarshal([]byte(commitTree.ResultJSON), &commitResult); err != nil || filepath.Clean(commitResult["path"]) != filepath.Clean(installedPath) {
		return errors.New("installed path journal mismatch")
	}
	if err := r.proveInstalledTree(installedPath, installation, commitResult["artifactHash"]); err != nil {
		return err
	}
	definition, err := r.container.DefinitionRepository.GetExtension(ctx, installation.ExtensionID, installation.InstalledVersion)
	if err != nil {
		return fmt.Errorf("definition unavailable: %w", err)
	}
	modules, err := r.container.ModuleRepository.ListModules(ctx, installation.ExtensionID)
	if err != nil {
		return fmt.Errorf("modules unavailable: %w", err)
	}
	contributions, err := r.container.ContributionRepository.ListContributions(ctx, installation.ExtensionID)
	if err != nil {
		return fmt.Errorf("contributions unavailable: %w", err)
	}
	expectedContributions := 0
	for _, module := range definition.Modules {
		expectedContributions += len(module.Contributions)
	}
	if len(modules) != len(definition.Modules) || len(contributions) != expectedContributions {
		return errors.New("installed definitions incomplete")
	}
	return nil
}

func (r *Runtime) proveRollbackPackageOperation(ctx context.Context, operation PackageOperationRecord, completed map[string]PackageOperationStep) error {
	if _, ok := completed["validate_rollback_point"]; !ok {
		return errors.New("rollback point validation step missing")
	}
	if _, ok := completed["restore_repositories"]; !ok {
		return errors.New("repository restore step missing")
	}
	installation, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(operation.ExtensionID))
	if err != nil {
		return fmt.Errorf("installation unavailable: %w", err)
	}
	if installation.InstalledVersion.String() != operation.TargetVersion || installation.PackageID != operation.ArtifactID {
		return errors.New("rollback installation identity mismatch")
	}
	artifact, err := r.container.PackageRepository.GetArtifact(ctx, operation.ArtifactID)
	if err != nil {
		return fmt.Errorf("rollback artifact unavailable: %w", err)
	}
	if _, err := r.VerifyStoredPackage(ctx, artifact); err != nil {
		return fmt.Errorf("rollback artifact verification failed: %w", err)
	}
	installedPath, _ := installation.Metadata["installedPath"].(string)
	return r.proveInstalledTree(installedPath, installation, "")
}

func (r *Runtime) proveUninstalledPackageOperation(ctx context.Context, operation PackageOperationRecord, completed map[string]PackageOperationStep) error {
	if _, ok := completed["move_to_quarantine"]; !ok {
		return errors.New("quarantine move step missing")
	}
	if _, ok := completed["cleanup_kernel_repositories"]; !ok {
		return errors.New("repository cleanup step missing")
	}
	_, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(operation.ExtensionID))
	if err == nil {
		return errors.New("installation still exists")
	}
	if !errors.Is(err, domain.ErrInvalidExtensionID) {
		return fmt.Errorf("installation state unavailable: %w", err)
	}
	artifact, err := r.container.PackageRepository.GetArtifact(ctx, operation.ArtifactID)
	if err != nil {
		return fmt.Errorf("artifact unavailable: %w", err)
	}
	if artifact.InstalledPath != "" {
		if _, statErr := os.Stat(artifact.InstalledPath); statErr == nil || !os.IsNotExist(statErr) {
			return errors.New("installed path absence could not be proven")
		}
	}
	quarantinePath := filepath.Join(r.root, "quarantine", operation.OperationID)
	if _, statErr := os.Stat(quarantinePath); statErr == nil || !os.IsNotExist(statErr) {
		return errors.New("quarantine finalization could not be proven")
	}
	return nil
}

func (r *Runtime) proveInstalledTree(installedPath string, installation domain.ExtensionInstallation, journalHash string) error {
	if installedPath == "" {
		return errors.New("installed path missing")
	}
	info, err := os.Stat(installedPath)
	if err != nil || !info.IsDir() {
		return errors.New("installed tree unavailable")
	}
	expectedHash, _ := installation.Metadata["installedTreeHash"].(string)
	generationID, _ := installation.Metadata["generationId"].(string)
	if generationID != "" && r.container.PackageGenerationStore != nil {
		current, err := r.container.PackageGenerationStore.ReadCurrent(string(installation.ExtensionID))
		if err != nil || current.GenerationID != generationID || current.TreeHash != expectedHash || filepath.Clean(installedPath) != filepath.Clean(filepath.Join(r.container.ExtRoot, "installations", safeDirectoryName(string(installation.ExtensionID)), "generations", generationID)) {
			return errors.New("installed generation current pointer mismatch")
		}
		if err := r.container.PackageGenerationStore.VerifyGeneration(context.Background(), current); err != nil {
			return err
		}
		if journalHash != "" && journalHash != expectedHash {
			return errors.New("installed tree journal hash mismatch")
		}
		return nil
	}
	actualHash := package_security.ComputeDirHash(installedPath, r.container.PackageSecurity.GetHasher())
	if actualHash == "" || expectedHash == "" || actualHash != expectedHash {
		return errors.New("installed tree hash mismatch")
	}
	if journalHash != "" && journalHash != actualHash {
		return errors.New("installed tree journal hash mismatch")
	}
	return nil
}

func (r *Runtime) requirePackageRecovery(ctx context.Context, operation PackageOperationRecord, detail string, cause error, guard PackageWriteGuard) error {
	if cause != nil {
		detail = detail + ": " + cause.Error()
	}
	setErr := r.container.PackageRepository.SetOperation(ctx, operation.OperationID, "requires_recovery", "recovery_manual", "PACKAGE_RECOVERY_REQUIRED", detail, false, guard)
	if setErr != nil {
		setErr = r.container.PackageRepository.SetOperation(ctx, operation.OperationID, "requires_recovery", "recovery_manual", "PACKAGE_RECOVERY_REQUIRED", detail, false, PackageWriteGuard{})
	}
	if setErr != nil {
		return errors.Join(errors.New(detail), fmt.Errorf("persist recovery state: %w", setErr))
	}
	return errors.New(detail)
}
