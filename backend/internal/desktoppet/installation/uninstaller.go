// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/log"
)

const (
	uninstallerRootDir      = "desktop-pets"
	uninstallerInstalledDir = "installed"
	uninstallerGenTasksDir  = "generation-tasks"
	uninstallerProcessedDir = "processed"
	uninstallerPackagesDir  = "packages"
)

var uninstallerProtectedSubDirs = []string{
	filepath.Join(uninstallerRootDir, uninstallerGenTasksDir),
	filepath.Join(uninstallerRootDir, uninstallerProcessedDir),
	filepath.Join(uninstallerRootDir, uninstallerPackagesDir),
}

type Uninstaller interface {
	Uninstall(userId, installationId string) error
	PurgeGenerationData(userId, generationTaskId string, confirmed bool) error
}

type uninstaller struct {
	repo    Repository
	dataDir string
}

func NewUninstaller(repo Repository, dataDir string) Uninstaller {
	return &uninstaller{
		repo:    repo,
		dataDir: dataDir,
	}
}

func (u *uninstaller) Uninstall(userId, installationId string) error {
	if userId == "" {
		return NewInstallationError(ErrCodeInstallationInvalid, "用户 ID 为空", ErrInstallationInvalid)
	}
	if installationId == "" {
		return NewInstallationError(ErrCodeInstallationInvalid, "安装 ID 为空", ErrInstallationInvalid)
	}
	if err := validateUninstallerPathSegment(installationId); err != nil {
		return NewInstallationError(ErrCodeInstallationInvalid,
			fmt.Sprintf("安装 ID 路径不安全: %v", err), ErrInstallationInvalid)
	}

	inst, err := u.repo.GetInstallation(installationId)
	if err != nil {
		if errors.Is(err, ErrInstallationNotFound) {
			return NewInstallationError(ErrCodeInstallationNotFound, "安装记录不存在", ErrInstallationNotFound)
		}
		return NewInstallationError(ErrCodeInstallationFailed, "查询安装记录失败", err)
	}

	if inst.UserID != userId {
		return NewInstallationError(ErrCodeInstallationInvalid, "安装记录不属于当前用户", ErrInstallationInvalid)
	}

	if inst.InstallPath == "" {
		return NewInstallationError(ErrCodeInstallationInvalid, "安装路径为空", ErrInstallationInvalid)
	}
	if err := validateInstallPathSafe(inst.InstallPath); err != nil {
		return NewInstallationError(ErrCodeInstallationInvalid,
			fmt.Sprintf("安装路径不安全: %v", err), ErrInstallationInvalid)
	}

	if inst.IsActivated() || inst.Status == StatusEnabled {
		if err := u.markInactive(inst.ID); err != nil {
			return NewInstallationError(ErrCodeInstallationFailed, "停用安装失败", err)
		}
	}

	if err := u.repo.UpdateInstallationStatus(installationId, StatusUninstalling); err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "标记卸载状态失败", err)
	}

	installDir := u.absInstallPath(inst.InstallPath)
	if err := u.ensureNotProtected(installDir); err != nil {
		return NewInstallationError(ErrCodeInstallationInvalid, err.Error(), ErrInstallationInvalid)
	}
	if err := removeTree(installDir); err != nil {
		log.Logger.Errorf("uninstaller: 删除安装目录失败 installationId=%s dir=%s err=%v", installationId, installDir, err)
		return NewInstallationError(ErrCodeInstallationFailed,
			fmt.Sprintf("删除安装目录失败: %s", installDir), err)
	}

	if err := u.repo.UpdateInstallationStatus(installationId, StatusUninstalled); err != nil {
		log.Logger.Errorf("uninstaller: 标记 uninstalled 状态失败（目录已删除） installationId=%s err=%v", installationId, err)
		return NewInstallationError(ErrCodeInstallationFailed,
			"标记 uninstalled 状态失败（目录已删除，下次启动可由恢复逻辑清理）", err)
	}

	if err := u.markInactive(installationId); err != nil {
		log.Logger.Errorf("uninstaller: 标记 is_active=false 失败 installationId=%s err=%v", installationId, err)
	}

	return nil
}

func (u *uninstaller) PurgeGenerationData(userId, generationTaskId string, confirmed bool) error {
	if !confirmed {
		return NewInstallationError(ErrCodePurgeNotConfirmed, "必须确认才能删除生成数据", ErrPurgeNotConfirmed)
	}
	if generationTaskId == "" {
		return NewInstallationError(ErrCodeInstallationFailed, "生成任务 ID 为空", nil)
	}
	if userId == "" {
		return NewInstallationError(ErrCodeInstallationInvalid, "用户 ID 为空", ErrInstallationInvalid)
	}
	if err := validateUninstallerPathSegment(generationTaskId); err != nil {
		return NewInstallationError(ErrCodeInstallationInvalid,
			fmt.Sprintf("生成任务 ID 路径不安全: %v", err), ErrInstallationInvalid)
	}

	genTaskDir := u.generationTaskDir(generationTaskId)
	if err := u.ensureGenTaskDeletable(genTaskDir); err != nil {
		return NewInstallationError(ErrCodeInstallationInvalid, err.Error(), ErrInstallationInvalid)
	}
	if _, err := os.Stat(genTaskDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return NewInstallationError(ErrCodeInstallationFailed,
			fmt.Sprintf("查询生成任务目录失败: %s", genTaskDir), err)
	}
	if err := removeTree(genTaskDir); err != nil {
		return NewInstallationError(ErrCodeInstallationFailed,
			fmt.Sprintf("删除生成任务目录失败: %s", genTaskDir), err)
	}
	return nil
}

func (u *uninstaller) ensureGenTaskDeletable(absPath string) error {
	if absPath == "" {
		return fmt.Errorf("path is empty")
	}
	cleaned := filepath.Clean(absPath)
	dataDirCleaned := filepath.Clean(u.dataDir)
	if !strings.HasPrefix(cleaned, dataDirCleaned+string(filepath.Separator)) && cleaned != dataDirCleaned {
		return fmt.Errorf("path escapes data dir: %s", cleaned)
	}
	genTasksRoot := filepath.Clean(filepath.Join(u.dataDir, uninstallerRootDir, uninstallerGenTasksDir))
	if cleaned == genTasksRoot {
		return fmt.Errorf("cannot delete generation-tasks root: %s", cleaned)
	}
	if !strings.HasPrefix(cleaned, genTasksRoot+string(filepath.Separator)) {
		return fmt.Errorf("path is not under generation-tasks: %s", cleaned)
	}
	return nil
}

func (u *uninstaller) markInactive(installationId string) error {
	db := u.repo.DB()
	if db == nil {
		return nil
	}
	now := time.Now().Format(installationTimeFormat)
	return db.Model(&Installation{}).
		Where("id = ?", installationId).
		Updates(map[string]interface{}{
			"is_active":  0,
			"updated_at": now,
		}).Error
}

func (u *uninstaller) absInstallPath(relPath string) string {
	cleaned := filepath.Clean(filepath.FromSlash(relPath))
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return ""
	}
	return filepath.Join(u.dataDir, cleaned)
}

func (u *uninstaller) generationTaskDir(generationTaskId string) string {
	return filepath.Join(u.dataDir, uninstallerRootDir, uninstallerGenTasksDir, generationTaskId)
}

func (u *uninstaller) ensureNotProtected(absPath string) error {
	if absPath == "" {
		return fmt.Errorf("path is empty")
	}
	cleaned := filepath.Clean(absPath)
	dataDirCleaned := filepath.Clean(u.dataDir)
	if !strings.HasPrefix(cleaned, dataDirCleaned+string(filepath.Separator)) && cleaned != dataDirCleaned {
		return fmt.Errorf("path escapes data dir: %s", cleaned)
	}
	for _, sub := range uninstallerProtectedSubDirs {
		protected := filepath.Clean(filepath.Join(u.dataDir, sub))
		if cleaned == protected {
			return fmt.Errorf("path is protected (forbidden to delete): %s", cleaned)
		}
	}
	return nil
}

func validateInstallPathSafe(relPath string) error {
	if relPath == "" {
		return errors.New("install path is empty")
	}
	cleaned := filepath.Clean(filepath.FromSlash(relPath))
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return errors.New("install path is root or current dir")
	}
	slash := filepath.ToSlash(cleaned)
	if strings.Contains(slash, "..") {
		return fmt.Errorf("install path contains parent traversal: %s", slash)
	}
	if strings.Contains(slash, ":") {
		return fmt.Errorf("install path contains drive letter: %s", slash)
	}
	if strings.HasPrefix(slash, "/") {
		return fmt.Errorf("install path is absolute: %s", slash)
	}
	if strings.HasPrefix(slash, "\\") {
		return fmt.Errorf("install path is absolute: %s", slash)
	}
	return nil
}

func validateUninstallerPathSegment(segment string) error {
	if segment == "" {
		return errors.New("segment is empty")
	}
	if strings.Contains(segment, "..") {
		return fmt.Errorf("segment contains parent traversal: %s", segment)
	}
	if strings.Contains(segment, "\\") {
		return fmt.Errorf("segment contains backslash: %s", segment)
	}
	if strings.Contains(segment, "/") {
		return fmt.Errorf("segment contains slash: %s", segment)
	}
	if strings.Contains(segment, ":") {
		return fmt.Errorf("segment contains drive letter: %s", segment)
	}
	if strings.HasPrefix(segment, "/") {
		return fmt.Errorf("segment is absolute: %s", segment)
	}
	if strings.ContainsAny(segment, "\x00") {
		return fmt.Errorf("segment contains null byte: %s", segment)
	}
	return nil
}
