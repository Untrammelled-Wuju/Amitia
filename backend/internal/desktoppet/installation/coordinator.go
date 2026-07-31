// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/packageformat"
	"gorm.io/gorm"
)

const (
	installationOpIDPrefix   = "instop_"
	historyIDPrefix          = "rlh_"
	coordinatorStagingSuffix = ".staging"
)

type Coordinator struct {
	repo      Repository
	storage   *ReleaseStorage
	validator *packageformat.Validator
	notifier  RuntimeNotifier
	dataDir   string
}

func NewCoordinator(repo Repository, storage *ReleaseStorage, notifier RuntimeNotifier) *Coordinator {
	return &Coordinator{
		repo:      repo,
		storage:   storage,
		validator: packageformat.NewValidator(),
		notifier:  notifier,
	}
}

func (c *Coordinator) WithDataDir(dataDir string) *Coordinator {
	c.dataDir = dataDir
	return c
}

func (c *Coordinator) Install(userID, petID, releaseID, characterID, idempotencyKey string) (*Installation, error) {
	if userID == "" || petID == "" || releaseID == "" {
		return nil, NewInstallationError(ErrCodeInstallationInvalid, "安装参数为空", ErrInstallationInvalid)
	}
	if idempotencyKey == "" {
		idempotencyKey = uuid.New().String()
	}

	existingOp, err := c.repo.GetInstallationOperationByIdempotencyKey(userID, "", idempotencyKey, InstOpTypeInstall)
	if err == nil && existingOp != nil {
		if existingOp.Status == OpStatusCompleted {
			inst, gerr := c.repo.GetInstallation(existingOp.InstallationID)
			if gerr != nil {
				return nil, NewInstallationError(ErrCodeInstallationNotFound, "幂等安装记录查询失败", gerr)
			}
			return inst, nil
		}
		return nil, NewInstallationError(ErrCodeInstallationFailed, "存在进行中的安装操作", nil)
	}
	if err != nil && !errors.Is(err, ErrInstallationNotFound) {
		return nil, NewInstallationError(ErrCodeInstallationFailed, "幂等检查失败", err)
	}

	release, err := c.repo.GetRelease(releaseID)
	if err != nil {
		return nil, NewInstallationError(ErrCodeInstallationNotFound, "Release 不存在", err)
	}
	if !release.IsPublished() {
		return nil, NewInstallationError(ErrCodePackageNotReady, "Release 未发布", ErrPackageNotReady)
	}
	if release.PetID != petID {
		return nil, NewInstallationError(ErrCodeInstallationInvalid, "Release 与桌宠不匹配", ErrInstallationInvalid)
	}

	if err := c.checkExistingInstallation(petID); err != nil {
		return nil, err
	}

	var manifest packageformat.Manifest
	if release.ManifestJSON != "" && release.ManifestJSON != "{}" {
		if err := json.Unmarshal([]byte(release.ManifestJSON), &manifest); err != nil {
			return nil, NewInstallationError(ErrCodeInstallationFailed, "解析 Release manifest 失败", err)
		}
	}

	installationID := installationIDPrefix + uuid.New().String()
	opID := installationOpIDPrefix + uuid.New().String()
	now := time.Now().Format(installationTimeFormat)

	op := &InstallationOperation{
		ID:             opID,
		OperationType:  InstOpTypeInstall,
		UserID:         userID,
		InstallationID: installationID,
		PetID:          petID,
		ReleaseID:      releaseID,
		IdempotencyKey: idempotencyKey,
		Stage:          OpStagePrepare,
		Status:         OpStatusRunning,
		StartedAt:      now,
		UpdatedAt:      now,
	}
	if err := c.repo.CreateInstallationOperation(op); err != nil {
		return nil, NewInstallationError(ErrCodeInstallationFailed, "创建安装操作记录失败", err)
	}

	installRelPath := c.installRelPath(installationID)
	inst := &Installation{
		ID:                installationID,
		UserID:            userID,
		CharacterID:       characterID,
		PackageID:         release.ID,
		PackageVersion:    release.Version,
		Name:              manifest.Name,
		Status:            StatusInstalling,
		IsActive:          0,
		InstallPath:       installRelPath,
		ManifestPath:      filepath.ToSlash(filepath.Join(installRelPath, installationManifestFile)),
		PreviewPath:       filepath.ToSlash(filepath.Join(installRelPath, "preview.png")),
		DefaultActionKey:  release.DefaultActionKey,
		CanvasWidth:       manifest.Canvas.Width,
		CanvasHeight:      manifest.Canvas.Height,
		PackageHash:       release.ContentRootHash,
		InstalledAt:       now,
		CreatedAt:         now,
		UpdatedAt:         now,
		PetID:             petID,
		CurrentReleaseID:  releaseID,
		LifecycleState:    LifecyclePreparing,
		DesiredState:      DesiredDisabled,
		RuntimeSyncState:  SyncPending,
		StateRevision:     0,
		InstallStorageKey: c.storage.InstallStorageKey(installationID),
		IntegrityRoot:     release.ContentRootHash,
	}
	if err := c.repo.CreateInstallation(inst); err != nil {
		_ = c.failOp(op, ErrCodeInstallationFailed, "创建安装记录失败", err)
		return nil, NewInstallationError(ErrCodeInstallationFailed, "创建安装记录失败", err)
	}

	if err := c.updateOpStage(op, OpStageStageFiles); err != nil {
		_ = c.failOp(op, ErrCodeInstallationFailed, "更新操作阶段失败", err)
		return nil, NewInstallationError(ErrCodeInstallationFailed, "更新操作阶段失败", err)
	}

	stagingDir, err := c.stageToStagingDir(petID, releaseID, installationID)
	if err != nil {
		_ = c.failOp(op, ErrCodeInstallationFailed, "复制 Release 到 staging 失败", err)
		c.markInstallationInvalid(installationID)
		return nil, NewInstallationError(ErrCodeInstallationFailed, "复制 Release 到 staging 失败", err)
	}
	op.StagingPathKey = c.storage.InstallStorageKey(installationID) + coordinatorStagingSuffix
	_ = c.repo.UpdateInstallationOperation(op)

	if err := c.updateOpStage(op, OpStageVerify); err != nil {
		c.cleanupStagingDir(stagingDir)
		_ = c.failOp(op, ErrCodeInstallationFailed, "更新操作阶段失败", err)
		return nil, NewInstallationError(ErrCodeInstallationFailed, "更新操作阶段失败", err)
	}

	if err := c.verifyIntegrity(stagingDir, release.ContentRootHash); err != nil {
		c.cleanupStagingDir(stagingDir)
		_ = c.failOp(op, ErrCodePackageHashMismatch, "完整性验证失败", err)
		c.markInstallationInvalid(installationID)
		return nil, err
	}

	if err := c.storage.AtomicSwapInstall(stagingDir, installationID); err != nil {
		c.cleanupStagingDir(stagingDir)
		_ = c.failOp(op, ErrCodeInstallationFailed, "原子交换安装目录失败", err)
		c.markInstallationInvalid(installationID)
		return nil, NewInstallationError(ErrCodeInstallationFailed, "原子交换安装目录失败", err)
	}

	if err := c.updateOpStage(op, OpStageCommitDB); err != nil {
		_ = c.failOp(op, ErrCodeInstallationFailed, "更新操作阶段失败", err)
		return nil, NewInstallationError(ErrCodeInstallationFailed, "更新操作阶段失败", err)
	}

	if err := c.commitInstallDB(op, inst, release); err != nil {
		_ = c.failOp(op, ErrCodeInstallationFailed, "提交安装事务失败", err)
		return nil, NewInstallationError(ErrCodeInstallationFailed, "提交安装事务失败", err)
	}

	inst.Status = StatusInstalled
	inst.LifecycleState = LifecycleInstalled
	inst.UpdatedAt = time.Now().Format(installationTimeFormat)
	return inst, nil
}

func (c *Coordinator) Upgrade(userID, installationID, targetReleaseID, idempotencyKey string) (*Installation, error) {
	if userID == "" || installationID == "" || targetReleaseID == "" {
		return nil, NewInstallationError(ErrCodeInstallationInvalid, "升级参数为空", ErrInstallationInvalid)
	}
	if idempotencyKey == "" {
		idempotencyKey = uuid.New().String()
	}

	existingOp, err := c.repo.GetInstallationOperationByIdempotencyKey(userID, "", idempotencyKey, InstOpTypeUpgrade)
	if err == nil && existingOp != nil {
		if existingOp.Status == OpStatusCompleted {
			inst, gerr := c.repo.GetInstallation(existingOp.InstallationID)
			if gerr != nil {
				return nil, NewInstallationError(ErrCodeInstallationNotFound, "幂等升级记录查询失败", gerr)
			}
			return inst, nil
		}
		return nil, NewInstallationError(ErrCodeInstallationFailed, "存在进行中的升级操作", nil)
	}
	if err != nil && !errors.Is(err, ErrInstallationNotFound) {
		return nil, NewInstallationError(ErrCodeInstallationFailed, "幂等检查失败", err)
	}

	inst, err := c.repo.GetInstallation(installationID)
	if err != nil {
		return nil, NewInstallationError(ErrCodeInstallationNotFound, "安装记录不存在", err)
	}
	if inst.UserID != userID {
		return nil, NewInstallationError(ErrCodeInstallationInvalid, "安装记录不属于当前用户", ErrInstallationInvalid)
	}
	if inst.CurrentReleaseID == targetReleaseID {
		return nil, NewInstallationError(ErrCodeInstallationInvalid, "目标版本与当前版本相同", ErrInstallationInvalid)
	}

	targetRelease, err := c.repo.GetRelease(targetReleaseID)
	if err != nil {
		return nil, NewInstallationError(ErrCodeInstallationNotFound, "目标 Release 不存在", err)
	}
	if !targetRelease.IsPublished() {
		return nil, NewInstallationError(ErrCodePackageNotReady, "目标 Release 未发布", ErrPackageNotReady)
	}
	if targetRelease.PetID != inst.PetID {
		return nil, NewInstallationError(ErrCodeInstallationInvalid, "目标 Release 与桌宠不匹配", ErrInstallationInvalid)
	}

	opID := installationOpIDPrefix + uuid.New().String()
	now := time.Now().Format(installationTimeFormat)
	op := &InstallationOperation{
		ID:              opID,
		OperationType:   InstOpTypeUpgrade,
		UserID:          userID,
		InstallationID:  installationID,
		PetID:           inst.PetID,
		ReleaseID:       inst.CurrentReleaseID,
		TargetReleaseID: targetReleaseID,
		IdempotencyKey:  idempotencyKey,
		Stage:           OpStagePrepare,
		Status:          OpStatusRunning,
		StartedAt:       now,
		UpdatedAt:       now,
	}
	if err := c.repo.CreateInstallationOperation(op); err != nil {
		return nil, NewInstallationError(ErrCodeInstallationFailed, "创建升级操作记录失败", err)
	}

	if err := c.updateOpLifecycle(installationID, LifecycleUpgrading); err != nil {
		_ = c.failOp(op, ErrCodeInstallationFailed, "更新安装生命周期状态失败", err)
		return nil, NewInstallationError(ErrCodeInstallationFailed, "更新安装生命周期状态失败", err)
	}

	if err := c.updateOpStage(op, OpStageStageFiles); err != nil {
		_ = c.failOp(op, ErrCodeInstallationFailed, "更新操作阶段失败", err)
		return nil, NewInstallationError(ErrCodeInstallationFailed, "更新操作阶段失败", err)
	}

	stagingDir, err := c.stageToStagingDir(inst.PetID, targetReleaseID, installationID)
	if err != nil {
		_ = c.failOp(op, ErrCodeInstallationFailed, "复制 Release 到 staging 失败", err)
		return nil, NewInstallationError(ErrCodeInstallationFailed, "复制 Release 到 staging 失败", err)
	}
	op.StagingPathKey = c.storage.InstallStorageKey(installationID) + coordinatorStagingSuffix
	_ = c.repo.UpdateInstallationOperation(op)

	if err := c.updateOpStage(op, OpStageVerify); err != nil {
		c.cleanupStagingDir(stagingDir)
		_ = c.failOp(op, ErrCodeInstallationFailed, "更新操作阶段失败", err)
		return nil, NewInstallationError(ErrCodeInstallationFailed, "更新操作阶段失败", err)
	}

	if err := c.verifyIntegrity(stagingDir, targetRelease.ContentRootHash); err != nil {
		c.cleanupStagingDir(stagingDir)
		_ = c.failOp(op, ErrCodePackageHashMismatch, "完整性验证失败", err)
		return nil, err
	}

	if err := c.storage.AtomicSwapInstall(stagingDir, installationID); err != nil {
		c.cleanupStagingDir(stagingDir)
		_ = c.failOp(op, ErrCodeInstallationFailed, "原子交换安装目录失败", err)
		return nil, NewInstallationError(ErrCodeInstallationFailed, "原子交换安装目录失败", err)
	}

	if err := c.updateOpStage(op, OpStageCommitDB); err != nil {
		_ = c.failOp(op, ErrCodeInstallationFailed, "更新操作阶段失败", err)
		return nil, NewInstallationError(ErrCodeInstallationFailed, "更新操作阶段失败", err)
	}

	if err := c.commitUpgradeDB(op, inst, inst.CurrentReleaseID, targetRelease); err != nil {
		_ = c.failOp(op, ErrCodeInstallationFailed, "提交升级事务失败", err)
		return nil, NewInstallationError(ErrCodeInstallationFailed, "提交升级事务失败", err)
	}

	inst.CurrentReleaseID = targetReleaseID
	inst.IntegrityRoot = targetRelease.ContentRootHash
	inst.StateRevision = inst.StateRevision + 1
	inst.PackageVersion = targetRelease.Version
	inst.LifecycleState = LifecycleInstalled
	inst.UpdatedAt = time.Now().Format(installationTimeFormat)
	return inst, nil
}

func (c *Coordinator) Uninstall(userID, installationID, idempotencyKey string) error {
	if userID == "" || installationID == "" {
		return NewInstallationError(ErrCodeInstallationInvalid, "卸载参数为空", ErrInstallationInvalid)
	}
	if idempotencyKey == "" {
		idempotencyKey = uuid.New().String()
	}

	existingOp, err := c.repo.GetInstallationOperationByIdempotencyKey(userID, "", idempotencyKey, InstOpTypeUninstall)
	if err == nil && existingOp != nil {
		if existingOp.Status == OpStatusCompleted {
			return nil
		}
		return NewInstallationError(ErrCodeInstallationFailed, "存在进行中的卸载操作", nil)
	}
	if err != nil && !errors.Is(err, ErrInstallationNotFound) {
		return NewInstallationError(ErrCodeInstallationFailed, "幂等检查失败", err)
	}

	inst, err := c.repo.GetInstallation(installationID)
	if err != nil {
		return NewInstallationError(ErrCodeInstallationNotFound, "安装记录不存在", err)
	}
	if inst.UserID != userID {
		return NewInstallationError(ErrCodeInstallationInvalid, "安装记录不属于当前用户", ErrInstallationInvalid)
	}
	if inst.Status == StatusUninstalled || inst.Status == StatusUninstalling {
		return NewInstallationError(ErrCodeInstallationInvalid, "安装已卸载或正在卸载", ErrInstallationInvalid)
	}

	opID := installationOpIDPrefix + uuid.New().String()
	now := time.Now().Format(installationTimeFormat)
	op := &InstallationOperation{
		ID:             opID,
		OperationType:  InstOpTypeUninstall,
		UserID:         userID,
		InstallationID: installationID,
		PetID:          inst.PetID,
		ReleaseID:      inst.CurrentReleaseID,
		IdempotencyKey: idempotencyKey,
		Stage:          OpStagePrepare,
		Status:         OpStatusRunning,
		StartedAt:      now,
		UpdatedAt:      now,
	}
	if err := c.repo.CreateInstallationOperation(op); err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "创建卸载操作记录失败", err)
	}

	if inst.IsActivated() || inst.Status == StatusEnabled {
		if c.notifier != nil {
			_ = c.notifier.NotifyInstallationDisabled(userID, installationID)
		}
		_ = c.repo.SetActiveInstallation(userID, "")
	}

	if err := c.updateOpLifecycle(installationID, LifecycleUninstalling); err != nil {
		_ = c.failOp(op, ErrCodeInstallationFailed, "更新安装生命周期状态失败", err)
		return NewInstallationError(ErrCodeInstallationFailed, "更新安装生命周期状态失败", err)
	}

	if err := c.storage.MoveInstallToTrash(installationID); err != nil {
		_ = c.failOp(op, ErrCodeInstallationFailed, "移动安装目录到回收站失败", err)
		return NewInstallationError(ErrCodeInstallationFailed, "移动安装目录到回收站失败", err)
	}
	op.TrashPathKey = c.storage.TrashStorageKey(installationID)
	_ = c.repo.UpdateInstallationOperation(op)

	if err := c.updateOpStage(op, OpStageCommitDB); err != nil {
		_ = c.failOp(op, ErrCodeInstallationFailed, "更新操作阶段失败", err)
		return NewInstallationError(ErrCodeInstallationFailed, "更新操作阶段失败", err)
	}

	if err := c.commitUninstallDB(op, inst); err != nil {
		_ = c.failOp(op, ErrCodeInstallationFailed, "提交卸载事务失败", err)
		return NewInstallationError(ErrCodeInstallationFailed, "提交卸载事务失败", err)
	}

	return nil
}

func (c *Coordinator) Switch(userID, installationID, idempotencyKey string) error {
	if userID == "" || installationID == "" {
		return NewInstallationError(ErrCodeInstallationInvalid, "切换参数为空", ErrInstallationInvalid)
	}
	if idempotencyKey == "" {
		idempotencyKey = uuid.New().String()
	}

	existingOp, err := c.repo.GetInstallationOperationByIdempotencyKey(userID, "", idempotencyKey, InstOpTypeSwitch)
	if err == nil && existingOp != nil {
		if existingOp.Status == OpStatusCompleted {
			return nil
		}
		return NewInstallationError(ErrCodeInstallationFailed, "存在进行中的切换操作", nil)
	}
	if err != nil && !errors.Is(err, ErrInstallationNotFound) {
		return NewInstallationError(ErrCodeInstallationFailed, "幂等检查失败", err)
	}

	target, err := c.repo.GetInstallation(installationID)
	if err != nil {
		return NewInstallationError(ErrCodeInstallationNotFound, "安装记录不存在", err)
	}
	if target.UserID != userID {
		return NewInstallationError(ErrCodeInstallationInvalid, "安装记录不属于当前用户", ErrInstallationInvalid)
	}
	if target.Status != StatusInstalled && target.Status != StatusEnabled && target.Status != StatusDisabled {
		return NewInstallationError(ErrCodeInstallationInvalid, "目标安装状态不可切换", ErrInstallationInvalid)
	}

	opID := installationOpIDPrefix + uuid.New().String()
	now := time.Now().Format(installationTimeFormat)
	op := &InstallationOperation{
		ID:             opID,
		OperationType:  InstOpTypeSwitch,
		UserID:         userID,
		InstallationID: installationID,
		PetID:          target.PetID,
		ReleaseID:      target.CurrentReleaseID,
		IdempotencyKey: idempotencyKey,
		Stage:          OpStageCommitDB,
		Status:         OpStatusRunning,
		StartedAt:      now,
		UpdatedAt:      now,
	}
	if err := c.repo.CreateInstallationOperation(op); err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "创建切换操作记录失败", err)
	}

	currentBinding, _ := c.repo.GetActiveBinding(userID)
	newRevision := 1
	if currentBinding != nil {
		newRevision = currentBinding.BindingRevision + 1
	}

	err = c.repo.Transaction(func(tx *gorm.DB) error {
		nowTx := time.Now().Format(installationTimeFormat)
		if err := tx.Model(&Installation{}).Where("user_id = ? AND is_active = ?", userID, 1).
			Updates(map[string]interface{}{"is_active": 0, "updated_at": nowTx}).Error; err != nil {
			return err
		}
		if err := tx.Model(&Installation{}).Where("id = ?", installationID).
			Updates(map[string]interface{}{"is_active": 1, "updated_at": nowTx}).Error; err != nil {
			return err
		}
		binding := &ActiveBinding{
			UserID:           userID,
			InstallationID:   installationID,
			PetID:            target.PetID,
			ReleaseID:        target.CurrentReleaseID,
			BindingRevision:  newRevision,
			DesiredState:     DesiredDisabled,
			RuntimeSyncState: SyncPending,
			DesiredUpdatedAt: nowTx,
			CreatedAt:        nowTx,
			UpdatedAt:        nowTx,
		}
		if err := tx.Save(binding).Error; err != nil {
			return err
		}
		op.Status = OpStatusCompleted
		op.Stage = OpStageCompleted
		op.CompletedAt = nowTx
		op.UpdatedAt = nowTx
		return tx.Save(op).Error
	})
	if err != nil {
		_ = c.failOp(op, ErrCodeInstallationFailed, "切换安装事务失败", err)
		return NewInstallationError(ErrCodeInstallationFailed, "切换安装事务失败", err)
	}
	return nil
}

func (c *Coordinator) Repair(userID, installationID, idempotencyKey string) error {
	if userID == "" || installationID == "" {
		return NewInstallationError(ErrCodeInstallationInvalid, "修复参数为空", ErrInstallationInvalid)
	}
	if idempotencyKey == "" {
		idempotencyKey = uuid.New().String()
	}

	existingOp, err := c.repo.GetInstallationOperationByIdempotencyKey(userID, "", idempotencyKey, InstOpTypeRepair)
	if err == nil && existingOp != nil {
		if existingOp.Status == OpStatusCompleted {
			return nil
		}
		return NewInstallationError(ErrCodeInstallationFailed, "存在进行中的修复操作", nil)
	}
	if err != nil && !errors.Is(err, ErrInstallationNotFound) {
		return NewInstallationError(ErrCodeInstallationFailed, "幂等检查失败", err)
	}

	inst, err := c.repo.GetInstallation(installationID)
	if err != nil {
		return NewInstallationError(ErrCodeInstallationNotFound, "安装记录不存在", err)
	}
	if inst.UserID != userID {
		return NewInstallationError(ErrCodeInstallationInvalid, "安装记录不属于当前用户", ErrInstallationInvalid)
	}
	if inst.CurrentReleaseID == "" {
		return NewInstallationError(ErrCodeInstallationInvalid, "安装无关联 Release", ErrInstallationInvalid)
	}

	release, err := c.repo.GetRelease(inst.CurrentReleaseID)
	if err != nil {
		return NewInstallationError(ErrCodeInstallationNotFound, "关联 Release 不存在", err)
	}

	opID := installationOpIDPrefix + uuid.New().String()
	now := time.Now().Format(installationTimeFormat)
	op := &InstallationOperation{
		ID:             opID,
		OperationType:  InstOpTypeRepair,
		UserID:         userID,
		InstallationID: installationID,
		PetID:          inst.PetID,
		ReleaseID:      inst.CurrentReleaseID,
		IdempotencyKey: idempotencyKey,
		Stage:          OpStagePrepare,
		Status:         OpStatusRunning,
		StartedAt:      now,
		UpdatedAt:      now,
	}
	if err := c.repo.CreateInstallationOperation(op); err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "创建修复操作记录失败", err)
	}

	if err := c.updateOpLifecycle(installationID, LifecycleRecoveryReq); err != nil {
		_ = c.failOp(op, ErrCodeInstallationFailed, "更新安装生命周期状态失败", err)
		return NewInstallationError(ErrCodeInstallationFailed, "更新安装生命周期状态失败", err)
	}

	if err := c.updateOpStage(op, OpStageStageFiles); err != nil {
		_ = c.failOp(op, ErrCodeInstallationFailed, "更新操作阶段失败", err)
		return NewInstallationError(ErrCodeInstallationFailed, "更新操作阶段失败", err)
	}

	stagingDir, err := c.stageToStagingDir(inst.PetID, inst.CurrentReleaseID, installationID)
	if err != nil {
		_ = c.failOp(op, ErrCodeInstallationFailed, "复制 Release 到 staging 失败", err)
		return NewInstallationError(ErrCodeInstallationFailed, "复制 Release 到 staging 失败", err)
	}
	op.StagingPathKey = c.storage.InstallStorageKey(installationID) + coordinatorStagingSuffix
	_ = c.repo.UpdateInstallationOperation(op)

	if err := c.updateOpStage(op, OpStageVerify); err != nil {
		c.cleanupStagingDir(stagingDir)
		_ = c.failOp(op, ErrCodeInstallationFailed, "更新操作阶段失败", err)
		return NewInstallationError(ErrCodeInstallationFailed, "更新操作阶段失败", err)
	}

	if err := c.verifyIntegrity(stagingDir, release.ContentRootHash); err != nil {
		c.cleanupStagingDir(stagingDir)
		_ = c.failOp(op, ErrCodePackageHashMismatch, "完整性验证失败", err)
		return err
	}

	if err := c.storage.AtomicSwapInstall(stagingDir, installationID); err != nil {
		c.cleanupStagingDir(stagingDir)
		_ = c.failOp(op, ErrCodeInstallationFailed, "原子交换安装目录失败", err)
		return NewInstallationError(ErrCodeInstallationFailed, "原子交换安装目录失败", err)
	}

	if err := c.updateOpStage(op, OpStageCommitDB); err != nil {
		_ = c.failOp(op, ErrCodeInstallationFailed, "更新操作阶段失败", err)
		return NewInstallationError(ErrCodeInstallationFailed, "更新操作阶段失败", err)
	}

	commitNow := time.Now().Format(installationTimeFormat)
	if err := c.repo.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Installation{}).Where("id = ?", installationID).Updates(map[string]interface{}{
			"lifecycle_state":    LifecycleInstalled,
			"last_error_code":    "",
			"last_error_message": "",
			"integrity_root":     release.ContentRootHash,
			"updated_at":         commitNow,
		}).Error; err != nil {
			return err
		}
		op.Status = OpStatusCompleted
		op.Stage = OpStageCompleted
		op.CompletedAt = commitNow
		op.UpdatedAt = commitNow
		return tx.Save(op).Error
	}); err != nil {
		_ = c.failOp(op, ErrCodeInstallationFailed, "提交修复事务失败", err)
		return NewInstallationError(ErrCodeInstallationFailed, "提交修复事务失败", err)
	}

	return nil
}

func (c *Coordinator) installRelPath(installationID string) string {
	return filepath.ToSlash(filepath.Join("desktop-pets", "installations", installationID)) + "/"
}

func (c *Coordinator) stageToStagingDir(petID, releaseID, installationID string) (string, error) {
	published := c.storage.PublishedDir(petID, releaseID)
	stagingDir := c.storage.InstallDir(installationID) + coordinatorStagingSuffix
	if _, err := os.Stat(published); err != nil {
		return "", fmt.Errorf("published 目录不存在: %w", err)
	}
	_ = removeTree(stagingDir)
	if err := copyDirContents(published, stagingDir); err != nil {
		return "", fmt.Errorf("复制 published 到 staging 失败: %w", err)
	}
	return stagingDir, nil
}

func (c *Coordinator) verifyIntegrity(dir, expectedRootHash string) error {
	manifestPath := filepath.Join(dir, installationManifestFile)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "manifest.json 不可读", err)
	}
	var manifest packageformat.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "manifest.json 解析失败", err)
	}
	report := c.validator.ValidateDirectory(dir, &manifest)
	if report.ErrorCount > 0 {
		return NewInstallationError(ErrCodePackageHashMismatch,
			fmt.Sprintf("目录验证失败: %d 个错误", report.ErrorCount), ErrPackageHashMismatch)
	}
	fileManifest, err := packageformat.BuildFileManifestFromDir(dir)
	if err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "构建文件清单失败", err)
	}
	var entries []packageformat.FileEntry
	for _, e := range fileManifest.Entries {
		entries = append(entries, packageformat.FileEntry{
			Path:   e.Path,
			SHA256: e.SHA256,
			Bytes:  e.Bytes,
		})
	}
	actualHash := packageformat.ComputeTreeHash(entries)
	if expectedRootHash != "" && actualHash != expectedRootHash {
		return NewInstallationError(ErrCodePackageHashMismatch,
			fmt.Sprintf("ContentRootHash 不匹配: 期望 %s, 实际 %s", expectedRootHash, actualHash),
			ErrPackageHashMismatch)
	}
	return nil
}

func (c *Coordinator) checkExistingInstallation(petID string) error {
	var existing Installation
	err := c.repo.DB().Where("pet_id = ? AND status NOT IN ?", petID,
		[]string{StatusUninstalled, StatusUninstalling, StatusInvalid}).
		First(&existing).Error
	if err == nil {
		return NewInstallationError(ErrCodeInstallationDuplicate, "该桌宠已存在活跃安装", ErrInstallationDuplicate)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return NewInstallationError(ErrCodeInstallationFailed, "查询已存在安装失败", err)
	}
	return nil
}

func (c *Coordinator) updateOpStage(op *InstallationOperation, stage string) error {
	op.Stage = stage
	op.UpdatedAt = time.Now().Format(installationTimeFormat)
	return c.repo.UpdateInstallationOperation(op)
}

func (c *Coordinator) updateOpLifecycle(installationID, lifecycle string) error {
	now := time.Now().Format(installationTimeFormat)
	return c.repo.DB().Model(&Installation{}).Where("id = ?", installationID).
		Updates(map[string]interface{}{
			"lifecycle_state": lifecycle,
			"updated_at":      now,
		}).Error
}

func (c *Coordinator) failOp(op *InstallationOperation, code, message string, err error) error {
	now := time.Now().Format(installationTimeFormat)
	op.Status = OpStatusFailed
	op.ErrorCode = code
	op.ErrorMessage = message
	op.Stage = OpStageCompleted
	op.CompletedAt = now
	op.UpdatedAt = now
	_ = c.repo.UpdateInstallationOperation(op)
	return NewInstallationError(code, message, err)
}

func (c *Coordinator) completeOp(op *InstallationOperation) error {
	now := time.Now().Format(installationTimeFormat)
	op.Status = OpStatusCompleted
	op.Stage = OpStageCompleted
	op.CompletedAt = now
	op.UpdatedAt = now
	return c.repo.UpdateInstallationOperation(op)
}

func (c *Coordinator) cleanupStagingDir(stagingDir string) error {
	return removeTree(stagingDir)
}

func (c *Coordinator) markInstallationInvalid(installationID string) error {
	return c.repo.UpdateInstallationStatus(installationID, StatusInvalid)
}

func (c *Coordinator) commitInstallDB(op *InstallationOperation, inst *Installation, release *PackageRelease) error {
	now := time.Now().Format(installationTimeFormat)
	return c.repo.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Installation{}).Where("id = ?", inst.ID).Updates(map[string]interface{}{
			"status":              StatusInstalled,
			"lifecycle_state":     LifecycleInstalled,
			"install_storage_key": c.storage.InstallStorageKey(inst.ID),
			"integrity_root":      release.ContentRootHash,
			"current_release_id":  release.ID,
			"updated_at":          now,
		}).Error; err != nil {
			return err
		}
		history := &InstallationReleaseHistory{
			ID:                historyIDPrefix + uuid.New().String(),
			InstallationID:    inst.ID,
			ReleaseID:         release.ID,
			PetID:             inst.PetID,
			Version:           release.Version,
			ActivatedAt:       now,
			IsCurrent:         1,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		if err := tx.Create(history).Error; err != nil {
			return err
		}
		binding := &ActiveBinding{
			UserID:           inst.UserID,
			InstallationID:   inst.ID,
			PetID:            inst.PetID,
			ReleaseID:        release.ID,
			BindingRevision:  1,
			DesiredState:     DesiredDisabled,
			RuntimeSyncState: SyncPending,
			DesiredUpdatedAt: now,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := tx.Save(binding).Error; err != nil {
			return err
		}
		op.Status = OpStatusCompleted
		op.Stage = OpStageCompleted
		op.CompletedAt = now
		op.UpdatedAt = now
		return tx.Save(op).Error
	})
}

func (c *Coordinator) commitUpgradeDB(op *InstallationOperation, inst *Installation, oldReleaseID string, newRelease *PackageRelease) error {
	now := time.Now().Format(installationTimeFormat)
	return c.repo.Transaction(func(tx *gorm.DB) error {
		newRevision := inst.StateRevision + 1
		if err := tx.Model(&Installation{}).Where("id = ?", inst.ID).Updates(map[string]interface{}{
			"current_release_id": newRelease.ID,
			"integrity_root":     newRelease.ContentRootHash,
			"state_revision":     newRevision,
			"package_version":    newRelease.Version,
			"lifecycle_state":    LifecycleInstalled,
			"updated_at":         now,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&InstallationReleaseHistory{}).
			Where("installation_id = ? AND is_current = ?", inst.ID, 1).
			Updates(map[string]interface{}{
				"is_current":          0,
				"deactivated_at":      now,
				"deactivation_reason": "upgraded",
				"updated_at":          now,
			}).Error; err != nil {
			return err
		}
		history := &InstallationReleaseHistory{
			ID:                historyIDPrefix + uuid.New().String(),
			InstallationID:    inst.ID,
			ReleaseID:         newRelease.ID,
			PetID:             inst.PetID,
			Version:           newRelease.Version,
			ActivatedAt:       now,
			IsCurrent:         1,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		if err := tx.Create(history).Error; err != nil {
			return err
		}
		var binding ActiveBinding
		bindErr := tx.Where("user_id = ?", inst.UserID).First(&binding).Error
		if bindErr == nil {
			binding.InstallationID = inst.ID
			binding.ReleaseID = newRelease.ID
			binding.PetID = inst.PetID
			binding.BindingRevision = binding.BindingRevision + 1
			binding.UpdatedAt = now
			if err := tx.Save(&binding).Error; err != nil {
				return err
			}
		} else if errors.Is(bindErr, gorm.ErrRecordNotFound) {
			newBinding := &ActiveBinding{
				UserID:           inst.UserID,
				InstallationID:   inst.ID,
				PetID:            inst.PetID,
				ReleaseID:        newRelease.ID,
				BindingRevision:  1,
				DesiredState:     DesiredDisabled,
				RuntimeSyncState: SyncPending,
				DesiredUpdatedAt: now,
				CreatedAt:        now,
				UpdatedAt:        now,
			}
			if err := tx.Save(newBinding).Error; err != nil {
				return err
			}
		} else {
			return bindErr
		}
		op.Status = OpStatusCompleted
		op.Stage = OpStageCompleted
		op.CompletedAt = now
		op.UpdatedAt = now
		return tx.Save(op).Error
	})
}

func (c *Coordinator) commitUninstallDB(op *InstallationOperation, inst *Installation) error {
	now := time.Now().Format(installationTimeFormat)
	return c.repo.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Installation{}).Where("id = ?", inst.ID).Updates(map[string]interface{}{
			"status":          StatusUninstalled,
			"lifecycle_state": LifecycleUninstalled,
			"is_active":       0,
			"updated_at":      now,
		}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", inst.UserID).Delete(&ActiveBinding{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&InstallationReleaseHistory{}).
			Where("installation_id = ? AND is_current = ?", inst.ID, 1).
			Updates(map[string]interface{}{
				"is_current":          0,
				"deactivated_at":      now,
				"deactivation_reason": "uninstalled",
				"updated_at":          now,
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
