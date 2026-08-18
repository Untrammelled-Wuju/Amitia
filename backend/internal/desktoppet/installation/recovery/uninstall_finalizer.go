// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package recovery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
	"github.com/u-ai/backend/log"
)

type UninstallFinalizer struct {
	repo     RecoveryRepo
	dataDir  string
	trashDir string
}

type UninstallRepo interface {
	GetInstallation(installationID string) (installPath string, err error)
	CreateTrashEntry(ctx context.Context, operationID, installationID, trashPath, sourcePath string) error
	FindTrashBySourcePath(ctx context.Context, sourcePath string) (trashID string, exists bool, err error)
	MarkInstallationUninstalled(ctx context.Context, installationID string) error
	UpdateOperationCompleted(ctx context.Context, operationID string) error
}

func NewUninstallFinalizer(repo RecoveryRepo, dataDir string) *UninstallFinalizer {
	return &UninstallFinalizer{
		repo:     repo,
		dataDir:  dataDir,
		trashDir: filepath.Join(dataDir, "desktop-pets", "storage-trash"),
	}
}

func (f *UninstallFinalizer) FinalizeDesiredStateApplied(ctx context.Context, op *operation.InstallationOperation) error {
	if op.OperationType != operation.TypeUninstall {
		return nil
	}

	sourcePath, err := f.getSourcePath(ctx, op)
	if err != nil {
		return fmt.Errorf("uninstall finalizer: get source path: %w", err)
	}

	orphanTrashPath, err := f.findOrphanTrash(sourcePath)
	if err != nil {
		return fmt.Errorf("uninstall finalizer: find orphan trash: %w", err)
	}

	if orphanTrashPath == "" {
		trashPath, exists, err := f.checkExistingTrashEntry(ctx, sourcePath)
		if err != nil {
			return fmt.Errorf("uninstall finalizer: check existing trash: %w", err)
		}
		if exists {
			return f.completeUninstall(ctx, op, trashPath)
		}
		return nil
	}

	trashEntryExists, err := f.trashEntryExistsForOperation(ctx, op.ID)
	if err != nil {
		return fmt.Errorf("uninstall finalizer: check trash entry: %w", err)
	}
	if trashEntryExists {
		return nil
	}

	if err := f.createTrashEntry(ctx, op, orphanTrashPath, sourcePath); err != nil {
		return fmt.Errorf("uninstall finalizer: create trash entry: %w", err)
	}

	return f.completeUninstall(ctx, op, orphanTrashPath)
}

func (f *UninstallFinalizer) getSourcePath(ctx context.Context, op *operation.InstallationOperation) (string, error) {
	if v, ok := f.repo.(interface {
		GetInstallationPath(ctx context.Context, installationID string) (string, error)
	}); ok {
		return v.GetInstallationPath(ctx, op.InstallationID)
	}
	return filepath.Join(f.dataDir, "desktop-pets", "installations", op.InstallationID), nil
}

func (f *UninstallFinalizer) findOrphanTrash(sourcePath string) (string, error) {
	installationID := filepath.Base(sourcePath)
	operationTrashDir := filepath.Join(f.trashDir, installationID)

	entries, err := os.ReadDir(operationTrashDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read trash dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			trashPath := filepath.Join(operationTrashDir, entry.Name())
			if f.isOrphanTrash(trashPath, sourcePath) {
				return trashPath, nil
			}
		}
	}
	return "", nil
}

func (f *UninstallFinalizer) isOrphanTrash(trashPath, sourcePath string) bool {
	trashContentHash := f.computeDirHash(trashPath)
	sourceContentHash := f.computeDirHash(sourcePath)
	if trashContentHash == "" || sourceContentHash == "" {
		return false
	}
	return trashContentHash == sourceContentHash
}

func (f *UninstallFinalizer) computeDirHash(dirPath string) string {
	_, err := os.Stat(dirPath)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s_%d", filepath.Base(dirPath), time.Now().Unix())
}

func (f *UninstallFinalizer) checkExistingTrashEntry(ctx context.Context, sourcePath string) (string, bool, error) {
	if v, ok := f.repo.(UninstallRepo); ok {
		return v.FindTrashBySourcePath(ctx, sourcePath)
	}
	return "", false, nil
}

func (f *UninstallFinalizer) trashEntryExistsForOperation(ctx context.Context, operationID string) (bool, error) {
	if v, ok := f.repo.(interface {
		TrashEntryExistsForOperation(ctx context.Context, operationID string) (bool, error)
	}); ok {
		return v.TrashEntryExistsForOperation(ctx, operationID)
	}
	return false, nil
}

func (f *UninstallFinalizer) createTrashEntry(ctx context.Context, op *operation.InstallationOperation, trashPath, sourcePath string) error {
	if v, ok := f.repo.(UninstallRepo); ok {
		return v.CreateTrashEntry(ctx, op.ID, op.InstallationID, trashPath, sourcePath)
	}
	return nil
}

func (f *UninstallFinalizer) completeUninstall(ctx context.Context, op *operation.InstallationOperation, trashPath string) error {
	if v, ok := f.repo.(UninstallRepo); ok {
		if err := v.MarkInstallationUninstalled(ctx, op.InstallationID); err != nil {
			return fmt.Errorf("mark installation uninstalled: %w", err)
		}
		if err := v.UpdateOperationCompleted(ctx, op.ID); err != nil {
			return fmt.Errorf("update operation completed: %w", err)
		}
	}
	return nil
}

func generateTrashEntryID() string {
	return "trash_" + uuid.New().String()
}

func validateTrashPath(trashPath string) error {
	if trashPath == "" {
		return errors.New("trash path is empty")
	}
	if strings.Contains(trashPath, "..") {
		return errors.New("trash path contains parent traversal")
	}
	return nil
}

var _ = log.Warn
