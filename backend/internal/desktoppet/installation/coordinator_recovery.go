// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"os"
	"time"

	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

func (c *Coordinator) RecoverPendingOperations() error {
	ops, err := c.repo.ListPendingInstallationOperations()
	if err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "查询待恢复操作失败", err)
	}
	for _, op := range ops {
		if err := c.recoverOperation(op); err != nil {
			log.Logger.Errorf("coordinator: 恢复操作失败 opID=%s type=%s err=%v", op.ID, op.OperationType, err)
		}
	}
	return nil
}

func (c *Coordinator) RecoverOperation(opID string) error {
	op, err := c.repo.GetInstallationOperation(opID)
	if err != nil {
		return NewInstallationError(ErrCodeInstallationNotFound, "操作记录不存在", err)
	}
	if op.Status == OpStatusCompleted || op.Status == OpStatusFailed {
		return nil
	}
	return c.recoverOperation(op)
}

func (c *Coordinator) recoverOperation(op *InstallationOperation) error {
	switch op.Stage {
	case OpStagePrepare, OpStageStageFiles:
		return c.recoverStagingStage(op)
	case OpStageVerify:
		return c.recoverVerifyStage(op)
	case OpStageCommitDB:
		return c.recoverCommitDBStage(op)
	case OpStageCompleted:
		return c.recoverCompletedStage(op)
	default:
		return c.recoverStagingStage(op)
	}
}

func (c *Coordinator) recoverStagingStage(op *InstallationOperation) error {
	stagingDir := c.stagingDirForOp(op)
	if stagingDir != "" {
		if _, err := os.Stat(stagingDir); err == nil {
			_ = removeTree(stagingDir)
		}
	}
	if op.InstallationID != "" {
		var inst Installation
		dbErr := c.repo.DB().Where("id = ?", op.InstallationID).First(&inst).Error
		if dbErr == nil && inst.Status == StatusInstalling {
			_ = c.repo.UpdateInstallationStatus(op.InstallationID, StatusInvalid)
		}
	}
	return c.failOp(op, ErrCodeInstallationFailed, "staging 阶段中断，已回滚", nil)
}

func (c *Coordinator) recoverVerifyStage(op *InstallationOperation) error {
	stagingDir := c.stagingDirForOp(op)
	if stagingDir == "" {
		return c.failOp(op, ErrCodeInstallationFailed, "无法确定 staging 目录", nil)
	}
	if _, err := os.Stat(stagingDir); err != nil {
		return c.failOp(op, ErrCodeInstallationFailed, "staging 目录不存在", nil)
	}
	releaseID := op.ReleaseID
	if op.TargetReleaseID != "" {
		releaseID = op.TargetReleaseID
	}
	release, err := c.repo.GetRelease(releaseID)
	if err != nil {
		_ = removeTree(stagingDir)
		return c.failOp(op, ErrCodeInstallationNotFound, "关联 Release 不存在", nil)
	}
	if err := c.verifyIntegrity(stagingDir, release.ContentRootHash); err != nil {
		_ = removeTree(stagingDir)
		return c.failOp(op, ErrCodePackageHashMismatch, "恢复验证失败", nil)
	}
	if err := c.storage.AtomicSwapInstall(stagingDir, op.InstallationID); err != nil {
		_ = removeTree(stagingDir)
		return c.failOp(op, ErrCodeInstallationFailed, "恢复时原子交换失败", nil)
	}
	return c.recoverCommitDBStage(op)
}

func (c *Coordinator) recoverCommitDBStage(op *InstallationOperation) error {
	if op.InstallationID == "" {
		return c.failOp(op, ErrCodeInstallationInvalid, "操作缺少 installationID", nil)
	}
	inst, err := c.repo.GetInstallation(op.InstallationID)
	if err != nil {
		return c.failOp(op, ErrCodeInstallationNotFound, "安装记录不存在", nil)
	}
	switch op.OperationType {
	case InstOpTypeInstall:
		return c.recoverInstallCommit(op, inst)
	case InstOpTypeUpgrade:
		return c.recoverUpgradeCommit(op, inst)
	case InstOpTypeUninstall:
		return c.recoverUninstallCommit(op, inst)
	case InstOpTypeRepair:
		return c.recoverRepairCommit(op, inst)
	case InstOpTypeSwitch:
		return c.completeOp(op)
	default:
		return c.completeOp(op)
	}
}

func (c *Coordinator) recoverInstallCommit(op *InstallationOperation, inst *Installation) error {
	if inst.Status == StatusInstalled && inst.LifecycleState == LifecycleInstalled {
		return c.completeOp(op)
	}
	release, err := c.repo.GetRelease(op.ReleaseID)
	if err != nil {
		return c.failOp(op, ErrCodeInstallationNotFound, "关联 Release 不存在", nil)
	}
	if err := c.commitInstallDB(op, inst, release); err != nil {
		return c.failOp(op, ErrCodeInstallationFailed, "恢复安装提交失败", err)
	}
	return nil
}

func (c *Coordinator) recoverUpgradeCommit(op *InstallationOperation, inst *Installation) error {
	targetReleaseID := op.TargetReleaseID
	if targetReleaseID == "" {
		targetReleaseID = op.ReleaseID
	}
	if inst.CurrentReleaseID == targetReleaseID {
		return c.completeOp(op)
	}
	targetRelease, err := c.repo.GetRelease(targetReleaseID)
	if err != nil {
		return c.failOp(op, ErrCodeInstallationNotFound, "目标 Release 不存在", nil)
	}
	if err := c.commitUpgradeDB(op, inst, inst.CurrentReleaseID, targetRelease); err != nil {
		return c.failOp(op, ErrCodeInstallationFailed, "恢复升级提交失败", err)
	}
	return nil
}

func (c *Coordinator) recoverUninstallCommit(op *InstallationOperation, inst *Installation) error {
	if inst.Status == StatusUninstalled {
		return c.completeOp(op)
	}
	if err := c.commitUninstallDB(op, inst); err != nil {
		return c.failOp(op, ErrCodeInstallationFailed, "恢复卸载提交失败", err)
	}
	return nil
}

func (c *Coordinator) recoverRepairCommit(op *InstallationOperation, inst *Installation) error {
	if inst.LifecycleState == LifecycleInstalled && inst.LastErrorCode == "" {
		return c.completeOp(op)
	}
	now := time.Now().Format(installationTimeFormat)
	return c.repo.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Installation{}).Where("id = ?", inst.ID).Updates(map[string]interface{}{
			"lifecycle_state":    LifecycleInstalled,
			"last_error_code":    "",
			"last_error_message": "",
			"updated_at":         now,
		}).Error; err != nil {
			return err
		}
		op.Status = OpStatusCompleted
		op.Stage = OpStageCompleted
		op.CompletedAt = now
		op.UpdatedAt = now
		return tx.Save(op).Error
	})
}

func (c *Coordinator) recoverCompletedStage(op *InstallationOperation) error {
	if op.Status != OpStatusCompleted {
		return c.completeOp(op)
	}
	return nil
}

func (c *Coordinator) stagingDirForOp(op *InstallationOperation) string {
	if op.InstallationID == "" {
		return ""
	}
	return c.storage.InstallDir(op.InstallationID) + coordinatorStagingSuffix
}
