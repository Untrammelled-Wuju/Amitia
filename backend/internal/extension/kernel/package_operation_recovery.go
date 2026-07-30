package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
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
	stagingRoot := filepath.Join(r.root, "staging")
	entries, readErr := os.ReadDir(stagingRoot)
	if readErr == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				if err := os.RemoveAll(filepath.Join(stagingRoot, entry.Name())); err != nil {
					failures = append(failures, err)
				}
			}
		}
	} else if !os.IsNotExist(readErr) {
		failures = append(failures, readErr)
	}
	return errors.Join(failures...)
}

func (r *Runtime) recoverPackageOperation(ctx context.Context, operation PackageOperationRecord) error {
	installation, installationErr := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(operation.ExtensionID))
	switch operation.OperationType {
	case "install", "update":
		if installationErr == nil && installation.PackageID == operation.ArtifactID && installation.InstalledVersion.String() == operation.TargetVersion {
			if operation.PreviewSessionID != "" {
				_ = r.container.PackageRepository.ConsumePreview(ctx, operation.PreviewSessionID)
			}
			return r.container.PackageRepository.SetOperation(ctx, operation.OperationID, "completed", "recovered_completed", "", "", true)
		}
		_, steps, _ := r.container.PackageRepository.GetOperation(ctx, operation.UserID, operation.OperationID)
		for _, step := range steps {
			if step.StepName == "commit_installed_tree" {
				var result map[string]string
				if json.Unmarshal([]byte(step.ResultJSON), &result) == nil && result["path"] != "" {
					_ = r.container.PackageArtifactStore.RemoveInstalled(result["path"])
				}
			}
		}
		return r.container.PackageRepository.SetOperation(ctx, operation.OperationID, "failed", "recovered_compensated", "PACKAGE_PROCESS_INTERRUPTED", "operation was compensated during startup", true)
	case "rollback":
		if installationErr == nil && installation.InstalledVersion.String() == operation.TargetVersion {
			return r.container.PackageRepository.SetOperation(ctx, operation.OperationID, "completed", "recovered_completed", "", "", true)
		}
	case "uninstall":
		if installationErr != nil {
			return r.container.PackageRepository.SetOperation(ctx, operation.OperationID, "completed", "recovered_completed", "", "", true)
		}
	}
	return r.container.PackageRepository.SetOperation(ctx, operation.OperationID, "requires_recovery", "recovery_manual", "PACKAGE_RECOVERY_REQUIRED", "automatic recovery could not prove a safe terminal state", false)
}
