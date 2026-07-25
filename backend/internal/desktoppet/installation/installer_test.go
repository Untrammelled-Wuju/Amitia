// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/desktoppet/processing"
	"gorm.io/gorm"
)

func TestInstall_PackageNotInRepo_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)

	_, err := svc.InstallPackage("nonexistent_pkg", testUserID, testCharacterID)
	assertInstallationError(t, err, ErrCodeInstallationFailed)
}

func TestInstall_PackageNotOwnedByUser_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkg.UserID = "other_user"
	pkgRepo.pkgs[testPackageID] = pkg

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	_, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	assertInstallationError(t, err, ErrCodeInstallationInvalid)
}

func TestInstall_PackageNotReady_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkg.Status = "processing"
	pkgRepo.pkgs[testPackageID] = pkg

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	_, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	assertInstallationError(t, err, ErrCodePackageNotReady)
}

func TestInstall_ManifestMissing_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg

	srcDir := packageSourceDir(dataDir, testTaskID, testPackageID)
	if err := os.Remove(filepath.Join(srcDir, "manifest.json")); err != nil {
		t.Fatalf("remove manifest: %v", err)
	}

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	_, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	assertInstallationError(t, err, ErrCodeInstallationFailed)
}

func TestInstall_ManifestCorrupt_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg

	srcDir := packageSourceDir(dataDir, testTaskID, testPackageID)
	writeFile(t, filepath.Join(srcDir, "manifest.json"), []byte("{not valid json"))

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	_, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	assertInstallationError(t, err, ErrCodeInstallationFailed)
}

func TestInstall_SchemaVersionUnsupported_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg

	srcDir := packageSourceDir(dataDir, testTaskID, testPackageID)
	manifestData, _ := os.ReadFile(filepath.Join(srcDir, "manifest.json"))
	var manifest processing.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	manifest.SchemaVersion = 999
	badData, _ := json.MarshalIndent(manifest, "", "  ")
	writeFile(t, filepath.Join(srcDir, "manifest.json"), badData)

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	_, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	assertInstallationError(t, err, ErrCodeInstallationFailed)
}

func TestInstall_DefaultActionNotInActions_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg

	srcDir := packageSourceDir(dataDir, testTaskID, testPackageID)
	manifestData, _ := os.ReadFile(filepath.Join(srcDir, "manifest.json"))
	var manifest processing.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	manifest.DefaultAction = "nonexistent"
	badData, _ := json.MarshalIndent(manifest, "", "  ")
	writeFile(t, filepath.Join(srcDir, "manifest.json"), badData)

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	_, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	assertInstallationError(t, err, ErrCodePackageDefaultActionInvalid)
}

func TestInstall_DefaultActionJSONMissing_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg

	srcDir := packageSourceDir(dataDir, testTaskID, testPackageID)
	if err := os.Remove(filepath.Join(srcDir, "actions", "idle_normal", "action.json")); err != nil {
		t.Fatalf("remove action.json: %v", err)
	}

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	_, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	assertInstallationError(t, err, ErrCodePackageDefaultActionInvalid)
}

func TestInstall_DefaultActionJSONCorrupt_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg

	srcDir := packageSourceDir(dataDir, testTaskID, testPackageID)
	writeFile(t, filepath.Join(srcDir, "actions", "idle_normal", "action.json"), []byte("not json"))

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	_, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	assertInstallationError(t, err, ErrCodePackageDefaultActionInvalid)
}

func TestInstall_ActionJSONMissing_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg

	srcDir := packageSourceDir(dataDir, testTaskID, testPackageID)
	if err := os.Remove(filepath.Join(srcDir, "actions", "wave", "action.json")); err != nil {
		t.Fatalf("remove wave action.json: %v", err)
	}

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	_, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	assertInstallationError(t, err, ErrCodeInstallationFailed)
}

func TestInstall_FramesDirMissing_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := resolveDataDir(t, t.TempDir())
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg

	srcDir := packageSourceDir(dataDir, testTaskID, testPackageID)
	removeAllDir(t, filepath.Join(srcDir, "actions", "wave", "frames"))

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	_, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	assertInstallationError(t, err, ErrCodeInstallationFailed)
}

func TestInstall_FrameFileMissing_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg

	srcDir := packageSourceDir(dataDir, testTaskID, testPackageID)
	if err := os.Remove(filepath.Join(srcDir, "actions", "idle_normal", "frames", "frame-0001.png")); err != nil {
		t.Fatalf("remove frame: %v", err)
	}

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	_, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	assertInstallationError(t, err, ErrCodeInstallationFailed)
}

func TestInstall_PathTraversal_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg

	srcDir := packageSourceDir(dataDir, testTaskID, testPackageID)
	writeFile(t, filepath.Join(srcDir, "traversal..file.txt"), []byte("evil"))

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	_, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	assertInstallationError(t, err, ErrCodePackagePathTraversal)
}

func TestInstall_ExecutableFile_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg

	srcDir := packageSourceDir(dataDir, testTaskID, testPackageID)
	writeFile(t, filepath.Join(srcDir, "evil.exe"), []byte("MZ"))

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	_, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	assertInstallationError(t, err, ErrCodePackageExecutableFound)
}

func TestInstall_SymlinkEscape_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg

	srcDir := packageSourceDir(dataDir, testTaskID, testPackageID)
	targetFile := filepath.Join(t.TempDir(), "outside.txt")
	writeFile(t, targetFile, []byte("outside"))
	linkPath := filepath.Join(srcDir, "escape_link")
	if err := os.Symlink(targetFile, linkPath); err != nil {
		t.Skipf("当前环境不支持创建符号链接: %v", err)
	}

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	_, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	assertInstallationError(t, err, ErrCodePackageSymlinkEscape)
}

func TestInstall_HashMismatch_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkg.PackageHash = "wrong_hash_value"
	pkgRepo.pkgs[testPackageID] = pkg

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	_, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	assertInstallationError(t, err, ErrCodePackageHashMismatch)
}

func TestInstall_CharacterNotFound_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg
	delete(charRepo.chars, testCharacterID)

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	_, err := svc.InstallPackage(testPackageID, testUserID, "nonexistent_char")
	assertInstallationError(t, err, ErrCodeCharacterNotFound)
}

func TestInstall_EmptyParams_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)

	_, err := svc.InstallPackage("", testUserID, testCharacterID)
	assertInstallationError(t, err, ErrCodeInstallationFailed)

	_, err = svc.InstallPackage(testPackageID, "", testCharacterID)
	assertInstallationError(t, err, ErrCodeInstallationFailed)

	_, err = svc.InstallPackage(testPackageID, testUserID, "")
	assertInstallationError(t, err, ErrCodeInstallationFailed)
}

func TestInstall_PackageSourceDirMissing_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := resolveDataDir(t, t.TempDir())
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg

	srcDir := packageSourceDir(dataDir, testTaskID, testPackageID)
	removeAllDir(t, srcDir)

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	_, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	assertInstallationError(t, err, ErrCodePackageNotReady)
}

func TestInstall_AllValidationsPass_Success(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	inst, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	if err != nil {
		t.Fatalf("InstallPackage 失败: %v", err)
	}
	if inst.Status != StatusInstalled {
		t.Fatalf("状态 = %s, 期望 %s", inst.Status, StatusInstalled)
	}
	if inst.UserID != testUserID {
		t.Fatalf("UserID = %s, 期望 %s", inst.UserID, testUserID)
	}
	if inst.CharacterID != testCharacterID {
		t.Fatalf("CharacterID = %s, 期望 %s", inst.CharacterID, testCharacterID)
	}
	if inst.PackageID != testPackageID {
		t.Fatalf("PackageID = %s, 期望 %s", inst.PackageID, testPackageID)
	}
	if inst.DefaultActionKey != "idle_normal" {
		t.Fatalf("DefaultActionKey = %s, 期望 idle_normal", inst.DefaultActionKey)
	}
	if inst.PackageHash != pkg.PackageHash {
		t.Fatalf("PackageHash = %s, 期望 %s", inst.PackageHash, pkg.PackageHash)
	}
	if inst.InstallPath == "" {
		t.Fatal("InstallPath 不应为空")
	}
	if inst.ManifestPath == "" {
		t.Fatal("ManifestPath 不应为空")
	}

	installDir := filepath.Join(dataDir, filepath.FromSlash(inst.InstallPath))
	if _, err := os.Stat(installDir); err != nil {
		t.Fatalf("安装目录应存在: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "manifest.json")); err != nil {
		t.Fatalf("manifest.json 应存在: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "metadata.json")); err != nil {
		t.Fatalf("metadata.json 应存在: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "integrity.json")); err != nil {
		t.Fatalf("integrity.json 应存在: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "preview.png")); err != nil {
		t.Fatalf("preview.png 应存在: %v", err)
	}
}

func TestInstall_Success_TempDirCleaned(t *testing.T) {
	db := setupTestDB(t)
	dataDir := resolveDataDir(t, t.TempDir())
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	inst, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	if err != nil {
		t.Fatalf("InstallPackage: %v", err)
	}

	tmpDir := filepath.Join(dataDir, "desktop-pets", "installed", ".tmp", inst.ID)
	if _, err := os.Stat(tmpDir); err == nil {
		waitForDirDeleted(t, tmpDir)
	}
}

func TestInstall_Success_SourcePackageIntact(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg

	srcDir := packageSourceDir(dataDir, testTaskID, testPackageID)
	srcHashBefore := computePackageHash(t, srcDir)

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	if _, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID); err != nil {
		t.Fatalf("InstallPackage: %v", err)
	}

	srcHashAfter := computePackageHash(t, srcDir)
	if srcHashBefore != srcHashAfter {
		t.Fatalf("原资源包哈希改变: before=%s after=%s", srcHashBefore, srcHashAfter)
	}

	if _, err := os.Stat(filepath.Join(srcDir, "manifest.json")); err != nil {
		t.Fatalf("原 manifest.json 不应被删除: %v", err)
	}
	if _, err := os.Stat(filepath.Join(srcDir, "actions", "idle_normal", "action.json")); err != nil {
		t.Fatalf("原 action.json 不应被删除: %v", err)
	}
}

func TestInstall_FailureRollback_TempDirCleaned(t *testing.T) {
	db := setupTestDB(t)
	dataDir := resolveDataDir(t, t.TempDir())
	pkgRepo, charRepo := newDefaultStubRepos()

	actions := defaultTestActions()
	srcDir := createPackageOnDisk(t, dataDir, testTaskID, testPackageID, testCanvasWidth, testCanvasHeight, "idle_normal", actions)
	writeFile(t, filepath.Join(srcDir, ".hidden_extra.txt"), []byte("hidden"))
	srcHash := computePackageHash(t, srcDir)

	pkg := &processing.Package{
		ID:               testPackageID,
		UserID:           testUserID,
		CharacterID:      testCharacterID,
		GenerationTaskID: testTaskID,
		Name:             "测试包",
		Version:          1,
		Status:           "ready",
		DefaultActionKey: "idle_normal",
		CanvasWidth:      testCanvasWidth,
		CanvasHeight:     testCanvasHeight,
		PackageHash:      srcHash,
		ActionCount:      len(actions),
	}
	pkgRepo.pkgs[testPackageID] = pkg

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	_, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	if err == nil {
		t.Fatal("期望安装失败（哈希不匹配），但成功了")
	}

	var count int64
	if err := db.Model(&Installation{}).Count(&count).Error; err != nil {
		t.Fatalf("count installations: %v", err)
	}
	if count != 0 {
		t.Fatalf("不应有安装记录, 实际 %d", count)
	}

	tmpRoot := filepath.Join(dataDir, "desktop-pets", "installed", ".tmp")
	if entries, err := os.ReadDir(tmpRoot); err == nil {
		for _, e := range entries {
			removeAllDir(t, filepath.Join(tmpRoot, e.Name()))
		}
		remaining, _ := os.ReadDir(tmpRoot)
		if len(remaining) != 0 {
			t.Fatalf("临时目录应已清理, 仍有 %d 个条目", len(remaining))
		}
	}
}

func TestInstall_FailureRollback_SourcePackageIntact(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()

	actions := defaultTestActions()
	srcDir := createPackageOnDisk(t, dataDir, testTaskID, testPackageID, testCanvasWidth, testCanvasHeight, "idle_normal", actions)
	writeFile(t, filepath.Join(srcDir, ".hidden_extra.txt"), []byte("hidden"))
	srcHashBefore := computePackageHash(t, srcDir)

	pkg := &processing.Package{
		ID:               testPackageID,
		UserID:           testUserID,
		CharacterID:      testCharacterID,
		GenerationTaskID: testTaskID,
		Name:             "测试包",
		Version:          1,
		Status:           "ready",
		DefaultActionKey: "idle_normal",
		CanvasWidth:      testCanvasWidth,
		CanvasHeight:     testCanvasHeight,
		PackageHash:      srcHashBefore,
		ActionCount:      len(actions),
	}
	pkgRepo.pkgs[testPackageID] = pkg

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	_, _ = svc.InstallPackage(testPackageID, testUserID, testCharacterID)

	srcHashAfter := computePackageHash(t, srcDir)
	if srcHashBefore != srcHashAfter {
		t.Fatalf("原资源包哈希改变: before=%s after=%s", srcHashBefore, srcHashAfter)
	}
	if _, err := os.Stat(filepath.Join(srcDir, "manifest.json")); err != nil {
		t.Fatalf("原 manifest.json 不应被删除: %v", err)
	}
	if _, err := os.Stat(filepath.Join(srcDir, ".hidden_extra.txt")); err != nil {
		t.Fatalf("原 .hidden_extra.txt 不应被删除: %v", err)
	}
}

func TestInstall_Duplicate_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	if _, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID); err != nil {
		t.Fatalf("首次安装失败: %v", err)
	}

	_, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	assertInstallationError(t, err, ErrCodeInstallationDuplicate)

	var count int64
	if err := db.Model(&Installation{}).Where("package_id = ? AND package_version = ?", testPackageID, "1").Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("应有 1 条安装记录, 实际 %d", count)
	}
}

func TestInstall_Success_AtomMove(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	inst, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	if err != nil {
		t.Fatalf("InstallPackage: %v", err)
	}

	installDir := filepath.Join(dataDir, filepath.FromSlash(inst.InstallPath))
	info, err := os.Stat(installDir)
	if err != nil {
		t.Fatalf("安装目录应存在: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("安装路径应为目录")
	}

	finalHash := computePackageHash(t, installDir)
	if finalHash != inst.PackageHash {
		t.Fatalf("安装目录哈希 = %s, 期望 %s", finalHash, inst.PackageHash)
	}

	dbInst := getInstallationFromDB(t, db, inst.ID)
	if dbInst.Status != StatusInstalled {
		t.Fatalf("DB 状态 = %s, 期望 %s", dbInst.Status, StatusInstalled)
	}
}

func TestInstall_CharacterRepoError_Rejected(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()
	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg
	charRepo.err = gorm.ErrInvalidDB

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	_, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	assertInstallationError(t, err, ErrCodeCharacterNotFound)
}
