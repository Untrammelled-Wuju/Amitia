// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUninstall_Success_DeletesDirAndUpdatesStatus(t *testing.T) {
	svc, db, dataDir, inst, _ := setupInstalledService(t)
	installDir := filepath.Join(dataDir, filepath.FromSlash(inst.InstallPath))
	if _, err := os.Stat(installDir); err != nil {
		t.Fatalf("安装目录应存在: %v", err)
	}

	if err := svc.Uninstall(inst.ID); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	waitForDirDeleted(t, installDir)

	dbInst := getInstallationFromDB(t, db, inst.ID)
	if dbInst.Status != StatusUninstalled {
		t.Fatalf("状态 = %s, 期望 %s", dbInst.Status, StatusUninstalled)
	}
	if dbInst.IsActive != 0 {
		t.Fatalf("IsActive = %d, 期望 0", dbInst.IsActive)
	}
}

func TestUninstall_EnabledInstance_MarksInactiveFirst(t *testing.T) {
	svc, db, dataDir, inst, notifier := setupInstalledService(t)
	if err := svc.EnableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("EnableInstallation: %v", err)
	}
	if len(notifier.enabledCalls) != 1 {
		t.Fatalf("期望 1 次 enabled 通知, 实际 %d", len(notifier.enabledCalls))
	}

	installDir := filepath.Join(dataDir, filepath.FromSlash(inst.InstallPath))
	if err := svc.Uninstall(inst.ID); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	waitForDirDeleted(t, installDir)

	dbInst := getInstallationFromDB(t, db, inst.ID)
	if dbInst.Status != StatusUninstalled {
		t.Fatalf("状态 = %s, 期望 %s", dbInst.Status, StatusUninstalled)
	}
	if dbInst.IsActive != 0 {
		t.Fatalf("IsActive = %d, 期望 0", dbInst.IsActive)
	}
}

func TestUninstall_StateTransition_UninstallingToUninstalled(t *testing.T) {
	svc, db, _, inst, _ := setupInstalledService(t)

	if err := svc.Uninstall(inst.ID); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	dbInst := getInstallationFromDB(t, db, inst.ID)
	if dbInst.Status != StatusUninstalled {
		t.Fatalf("最终状态 = %s, 期望 %s", dbInst.Status, StatusUninstalled)
	}
}

func TestUninstall_PreservesGenerationHistory(t *testing.T) {
	svc, _, dataDir, inst, _ := setupInstalledService(t)

	genTaskDir := filepath.Join(dataDir, "desktop-pets", "generation-tasks", testTaskID)
	writeFile(t, filepath.Join(genTaskDir, "source", "reference.png"), []byte("ref"))

	packagesDir := filepath.Join(genTaskDir, "packages", testPackageID)
	writeFile(t, filepath.Join(packagesDir, "manifest.json"), []byte("{}"))

	processedDir := filepath.Join(genTaskDir, "processed", "version-1")
	writeFile(t, filepath.Join(processedDir, "actions", "idle_normal", "action.json"), []byte("{}"))

	if err := svc.Uninstall(inst.ID); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	if _, err := os.Stat(genTaskDir); err != nil {
		t.Fatalf("生成任务目录不应被删除: %v", err)
	}
	if _, err := os.Stat(filepath.Join(genTaskDir, "source", "reference.png")); err != nil {
		t.Fatalf("原始素材不应被删除: %v", err)
	}
	if _, err := os.Stat(packagesDir); err != nil {
		t.Fatalf("资源包目录不应被删除: %v", err)
	}
	if _, err := os.Stat(processedDir); err != nil {
		t.Fatalf("处理任务目录不应被删除: %v", err)
	}

	pkgManifestPath := filepath.Join(packagesDir, "manifest.json")
	if _, err := os.Stat(pkgManifestPath); err != nil {
		t.Fatalf("资源包 manifest 不应被删除: %v", err)
	}
}

func TestUninstall_NotFound_Rejected(t *testing.T) {
	svc, _, _, _, _ := setupInstalledService(t)

	err := svc.Uninstall("nonexistent_id")
	assertInstallationError(t, err, ErrCodeInstallationNotFound)
}

func TestUninstall_EmptyID_Rejected(t *testing.T) {
	svc, _, _, _, _ := setupInstalledService(t)

	err := svc.Uninstall("")
	assertInstallationError(t, err, ErrCodeInstallationInvalid)
}

func TestUninstall_PathInjection_Rejected(t *testing.T) {
	svc, _, _, _, _ := setupInstalledService(t)

	err := svc.Uninstall("../escape")
	assertInstallationError(t, err, ErrCodeInstallationInvalid)

	err = svc.Uninstall("foo/bar")
	assertInstallationError(t, err, ErrCodeInstallationInvalid)

	err = svc.Uninstall("C:\\evil")
	assertInstallationError(t, err, ErrCodeInstallationInvalid)
}

func TestUninstall_AlreadyUninstalled_DirAlreadyGone(t *testing.T) {
	svc, db, dataDir, inst, _ := setupInstalledService(t)

	installDir := filepath.Join(dataDir, filepath.FromSlash(inst.InstallPath))
	if err := svc.Uninstall(inst.ID); err != nil {
		t.Fatalf("首次 Uninstall: %v", err)
	}

	waitForDirDeleted(t, installDir)

	dbInst := getInstallationFromDB(t, db, inst.ID)
	if dbInst.Status != StatusUninstalled {
		t.Fatalf("状态 = %s, 期望 %s", dbInst.Status, StatusUninstalled)
	}
}

func TestUninstall_ProtectedPath_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := resolveDataDir(t, t.TempDir())
	un := newTestUninstaller(t, db, dataDir)

	protectedDirs := []string{
		filepath.Join(dataDir, "desktop-pets", "generation-tasks"),
		filepath.Join(dataDir, "desktop-pets", "processed"),
		filepath.Join(dataDir, "desktop-pets", "packages"),
	}
	for _, dir := range protectedDirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	for _, segment := range []string{"..", "../", "..\\", "foo/bar"} {
		err := un.Uninstall(segment)
		if err == nil {
			t.Fatalf("segment=%s 期望被拒绝但得到 nil", segment)
		}
	}

	for _, dir := range protectedDirs {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("受保护目录不应被删除: %s: %v", dir, err)
		}
	}
}

func TestUninstall_RemovesRuntimeSettingsInDB(t *testing.T) {
	svc, db, _, inst, _ := setupInstalledService(t)
	if err := svc.EnableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("EnableInstallation: %v", err)
	}

	if err := svc.Uninstall(inst.ID); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	dbInst := getInstallationFromDB(t, db, inst.ID)
	if dbInst.Status != StatusUninstalled {
		t.Fatalf("状态 = %s, 期望 %s", dbInst.Status, StatusUninstalled)
	}

	var rs RuntimeSettings
	if err := db.Where("installation_id = ?", inst.ID).First(&rs).Error; err != nil {
		t.Fatalf("运行时设置记录应保留: %v", err)
	}
	if rs.InstallationID != inst.ID {
		t.Fatalf("InstallationID = %s, 期望 %s", rs.InstallationID, inst.ID)
	}
}

func TestPurgeGenerationData_NotConfirmed_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := resolveDataDir(t, t.TempDir())
	un := newTestUninstaller(t, db, dataDir)

	err := un.PurgeGenerationData(testUserID, testTaskID, false)
	assertInstallationError(t, err, ErrCodePurgeNotConfirmed)
}

func TestPurgeGenerationData_DeletesGenTaskDir(t *testing.T) {
	db := setupTestDB(t)
	dataDir := resolveDataDir(t, t.TempDir())
	un := newTestUninstaller(t, db, dataDir)

	genTaskDir := filepath.Join(dataDir, "desktop-pets", "generation-tasks", testTaskID)
	writeFile(t, filepath.Join(genTaskDir, "source", "reference.png"), []byte("ref"))

	if err := un.PurgeGenerationData(testUserID, testTaskID, true); err != nil {
		t.Fatalf("PurgeGenerationData: %v", err)
	}

	waitForDirDeleted(t, genTaskDir)
}

func TestPurgeGenerationData_NonExistentDir_NoError(t *testing.T) {
	db := setupTestDB(t)
	dataDir := resolveDataDir(t, t.TempDir())
	un := newTestUninstaller(t, db, dataDir)

	if err := un.PurgeGenerationData(testUserID, "nonexistent_task", true); err != nil {
		t.Fatalf("不存在的目录应不报错: %v", err)
	}
}
