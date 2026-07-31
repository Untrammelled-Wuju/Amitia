// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"gorm.io/gorm"
)

const (
	installationIDPrefix      = "inst_"
	installationMetadataFile  = "metadata.json"
	installationIntegrityFile = "integrity.json"
	installationManifestFile  = "manifest.json"
	installationActionJSON    = "action.json"
	installationFramesDir     = "frames"
	installationPreviewFile   = "preview.png"
	installationTmpSegment    = ".tmp"
	installationRootDir       = "desktop-pets"
	installationSubDir        = "installed"
	installationGenerator     = "u-ai-installer"
)

var installationExecutableExtensions = map[string]bool{
	".exe":   true,
	".dll":   true,
	".so":    true,
	".dylib": true,
	".sh":    true,
	".bat":   true,
	".cmd":   true,
	".com":   true,
	".msi":   true,
	".scr":   true,
	".jar":   true,
	".app":   true,
	".bin":   true,
	".ps1":   true,
}

type Installer interface {
	InstallPackage(packageId, userId, characterId string) (*Installation, error)
}

type installer struct {
	repo        Repository
	packageRepo processing.Repository
	charRepo    character.Repository
	dataDir     string
}

func NewInstaller(repo Repository, packageRepo processing.Repository, charRepo character.Repository, dataDir string) Installer {
	return &installer{
		repo:        repo,
		packageRepo: packageRepo,
		charRepo:    charRepo,
		dataDir:     dataDir,
	}
}

func (s *installer) InstallPackage(packageId, userId, characterId string) (*Installation, error) {
	if packageId == "" || userId == "" || characterId == "" {
		return nil, NewInstallationError(ErrCodeInstallationFailed, "安装参数为空", nil)
	}

	pkg, err := s.packageRepo.GetPackage(packageId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewInstallationError(ErrCodeInstallationFailed, "资源包不存在", err)
		}
		return nil, NewInstallationError(ErrCodeInstallationFailed, "获取资源包失败", err)
	}

	if err := s.validateBeforeInstall(pkg, userId, characterId); err != nil {
		return nil, err
	}

	packageVersionStr := fmt.Sprintf("%d", pkg.Version)
	if existing, _ := s.repo.GetInstallationByPackageVersion(pkg.ID, packageVersionStr); existing != nil {
		return nil, NewInstallationError(ErrCodeInstallationDuplicate, "该资源包版本已安装", ErrInstallationDuplicate)
	}

	installId := installationIDPrefix + uuid.New().String()
	srcDir := s.packageSourceDir(pkg)
	tmpDir := s.installTmpDir(installId)
	finalDir := s.installFinalDir(installId)

	state := &installState{}

	defer func() {
		if r := recover(); r != nil {
			s.rollback(installId, state)
			panic(r)
		}
	}()

	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, s.failWithRollback(installId, state, "创建临时安装目录失败", err)
	}
	state.tmpCreated = true

	if err := copyPackageTree(srcDir, tmpDir); err != nil {
		return nil, s.failWithRollback(installId, state, "复制资源包内容失败", err)
	}

	if err := s.verifyCopiedPackage(tmpDir, pkg); err != nil {
		return nil, s.failWithRollback(installId, state, err.Error(), err)
	}

	actualHash, err := s.computePackageHash(tmpDir)
	if err != nil {
		return nil, s.failWithRollback(installId, state, "重新计算完整性哈希失败", err)
	}
	if actualHash != pkg.PackageHash {
		return nil, s.failWithRollback(installId, state,
			fmt.Sprintf("完整性哈希不匹配: 期望 %s, 实际 %s", pkg.PackageHash, actualHash),
			ErrPackageHashMismatch)
	}

	now := time.Now().Format(installationTimeFormat)
	finalRelPath := s.installFinalRelPath(installId)
	inst := &Installation{
		ID:               installId,
		UserID:           userId,
		CharacterID:      characterId,
		PackageID:        pkg.ID,
		PackageVersion:   packageVersionStr,
		Name:             pkg.Name,
		Status:           StatusInstalling,
		IsActive:         0,
		InstallPath:      finalRelPath,
		ManifestPath:     filepath.ToSlash(filepath.Join(finalRelPath, installationManifestFile)),
		PreviewPath:      filepath.ToSlash(filepath.Join(finalRelPath, installationPreviewFile)),
		DefaultActionKey: pkg.DefaultActionKey,
		CanvasWidth:      pkg.CanvasWidth,
		CanvasHeight:     pkg.CanvasHeight,
		PackageHash:      pkg.PackageHash,
		InstalledAt:      now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.repo.CreateInstallation(inst); err != nil {
		return nil, s.failWithRollback(installId, state, "写入安装记录失败", err)
	}
	state.dbRecordCreated = true

	if err := s.atomicMoveDir(tmpDir, finalDir); err != nil {
		return nil, s.failWithRollback(installId, state, "移动安装目录失败", err)
	}
	state.tmpCreated = false
	state.finalMoved = true

	if err := s.writeMetadataFiles(finalDir, installId, pkg, actualHash); err != nil {
		return nil, s.failWithRollback(installId, state, err.Error(), err)
	}

	if err := s.repo.UpdateInstallationStatus(installId, StatusInstalled); err != nil {
		return nil, s.failWithRollback(installId, state, "更新安装状态失败", err)
	}

	inst.Status = StatusInstalled
	inst.UpdatedAt = time.Now().Format(installationTimeFormat)
	return inst, nil
}

type installState struct {
	tmpCreated      bool
	dbRecordCreated bool
	finalMoved      bool
}

func (s *installer) validateBeforeInstall(pkg *processing.Package, userId, characterId string) error {
	if pkg == nil {
		return NewInstallationError(ErrCodeInstallationFailed, "资源包为空", nil)
	}

	if pkg.UserID != userId {
		return NewInstallationError(ErrCodeInstallationInvalid, "资源包不属于当前用户", ErrInstallationInvalid)
	}

	if pkg.Status != "ready" && pkg.Status != "succeeded" {
		return NewInstallationError(ErrCodePackageNotReady,
			fmt.Sprintf("资源包状态为 %s，非 ready", pkg.Status), ErrPackageNotReady)
	}

	srcDir := s.packageSourceDir(pkg)
	if info, err := os.Stat(srcDir); err != nil || !info.IsDir() {
		return NewInstallationError(ErrCodePackageNotReady, "资源包目录不存在", err)
	}

	manifestPath := filepath.Join(srcDir, installationManifestFile)
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "manifest.json 不存在或不可读", err)
	}

	var manifest processing.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "manifest.json 解析失败", err)
	}

	if manifest.SchemaVersion != processing.ManifestSchemaVersion {
		return NewInstallationError(ErrCodeInstallationFailed,
			fmt.Sprintf("schemaVersion %d 不受支持", manifest.SchemaVersion), nil)
	}

	defaultFound := false
	for _, action := range manifest.Actions {
		if action.Key == manifest.DefaultAction {
			defaultFound = true
			break
		}
	}
	if !defaultFound {
		return NewInstallationError(ErrCodePackageDefaultActionInvalid,
			fmt.Sprintf("默认动作 %s 不在 actions 列表中", manifest.DefaultAction),
			ErrPackageDefaultActionInvalid)
	}

	if err := processing.ValidateManifest(&manifest); err != nil {
		return NewInstallationError(ErrCodePackageDefaultActionInvalid,
			fmt.Sprintf("manifest 路径校验失败: %v", err), ErrPackageDefaultActionInvalid)
	}

	defaultActionJSONPath := filepath.Join(srcDir, filepath.FromSlash(s.defaultActionConfigPath(&manifest)))
	if _, err := os.Stat(defaultActionJSONPath); err != nil {
		return NewInstallationError(ErrCodePackageDefaultActionInvalid,
			fmt.Sprintf("默认动作 %s 的 action.json 不存在", manifest.DefaultAction), err)
	}
	if data, err := os.ReadFile(defaultActionJSONPath); err != nil {
		return NewInstallationError(ErrCodePackageDefaultActionInvalid,
			fmt.Sprintf("默认动作 %s 的 action.json 不可读", manifest.DefaultAction), err)
	} else if err := json.Unmarshal(data, &processing.ActionJSON{}); err != nil {
		return NewInstallationError(ErrCodePackageDefaultActionInvalid,
			fmt.Sprintf("默认动作 %s 的 action.json 解析失败", manifest.DefaultAction), err)
	}

	for _, action := range manifest.Actions {
		actionJSONPath := filepath.Join(srcDir, filepath.FromSlash(action.Config))
		if _, err := os.Stat(actionJSONPath); err != nil {
			return NewInstallationError(ErrCodeInstallationFailed,
				fmt.Sprintf("动作 %s 的 action.json 不存在", action.Key), err)
		}

		framesDir := filepath.Join(srcDir, "actions", action.Key, installationFramesDir)
		if info, err := os.Stat(framesDir); err != nil || !info.IsDir() {
			return NewInstallationError(ErrCodeInstallationFailed,
				fmt.Sprintf("动作 %s 的 frames 目录不存在", action.Key), err)
		}

		if err := s.verifyActionFrames(srcDir, action); err != nil {
			return err
		}
	}

	if err := s.verifyPackagePathsSafe(srcDir); err != nil {
		return err
	}

	actualHash, err := s.computePackageHash(srcDir)
	if err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "计算资源包哈希失败", err)
	}
	if actualHash != pkg.PackageHash {
		return NewInstallationError(ErrCodePackageHashMismatch,
			fmt.Sprintf("资源包哈希不匹配: 期望 %s, 实际 %s", pkg.PackageHash, actualHash),
			ErrPackageHashMismatch)
	}

	if _, err := s.charRepo.FindByID(characterId); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewInstallationError(ErrCodeCharacterNotFound, "角色不存在", ErrCharacterNotFound)
		}
		return NewInstallationError(ErrCodeCharacterNotFound, "校验角色失败", err)
	}

	return nil
}

func (s *installer) verifyActionFrames(srcDir string, action processing.ManifestAction) error {
	actionJSONPath := filepath.Join(srcDir, filepath.FromSlash(action.Config))
	data, err := os.ReadFile(actionJSONPath)
	if err != nil {
		return NewInstallationError(ErrCodeInstallationFailed,
			fmt.Sprintf("读取动作 %s 的 action.json 失败", action.Key), err)
	}
	var actionJSON processing.ActionJSON
	if err := json.Unmarshal(data, &actionJSON); err != nil {
		return NewInstallationError(ErrCodeInstallationFailed,
			fmt.Sprintf("解析动作 %s 的 action.json 失败", action.Key), err)
	}

	framesDir := filepath.Join(srcDir, "actions", action.Key, installationFramesDir)
	entries, err := os.ReadDir(framesDir)
	if err != nil {
		return NewInstallationError(ErrCodeInstallationFailed,
			fmt.Sprintf("读取动作 %s 的 frames 目录失败", action.Key), err)
	}

	actualFrameCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		actualFrameCount++
	}

	expectedFrameCount := actionJSON.FrameCount
	if expectedFrameCount == 0 {
		expectedFrameCount = len(actionJSON.Frames)
	}
	if expectedFrameCount == 0 {
		expectedFrameCount = actualFrameCount
	}
	if actualFrameCount != expectedFrameCount {
		return NewInstallationError(ErrCodeInstallationFailed,
			fmt.Sprintf("动作 %s 帧数量不一致: 配置 %d, 实际 %d", action.Key, expectedFrameCount, actualFrameCount),
			nil)
	}

	if len(actionJSON.Frames) > 0 {
		for _, frame := range actionJSON.Frames {
			if frame.File == "" {
				continue
			}
			framePath := filepath.Join(srcDir, "actions", action.Key, filepath.FromSlash(frame.File))
			if _, err := os.Stat(framePath); err != nil {
				return NewInstallationError(ErrCodeInstallationFailed,
					fmt.Sprintf("动作 %s 的帧文件 %s 不存在", action.Key, frame.File), err)
			}
		}
	}

	return nil
}

func (s *installer) verifyPackagePathsSafe(srcDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, relErr := filepath.Rel(srcDir, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}

		relSlash := filepath.ToSlash(rel)
		if strings.Contains(relSlash, "..") {
			return NewInstallationError(ErrCodePackagePathTraversal,
				fmt.Sprintf("路径包含穿越: %s", relSlash), ErrPackagePathTraversal)
		}
		if strings.Contains(relSlash, "\\") {
			return NewInstallationError(ErrCodePackagePathTraversal,
				fmt.Sprintf("路径包含反斜杠: %s", relSlash), ErrPackagePathTraversal)
		}
		if strings.Contains(relSlash, ":") {
			return NewInstallationError(ErrCodePackagePathTraversal,
				fmt.Sprintf("路径包含盘符: %s", relSlash), ErrPackagePathTraversal)
		}
		if strings.HasPrefix(relSlash, "/") {
			return NewInstallationError(ErrCodePackagePathTraversal,
				fmt.Sprintf("路径为绝对路径: %s", relSlash), ErrPackagePathTraversal)
		}

		lstat, lerr := os.Lstat(path)
		if lerr != nil {
			return NewInstallationError(ErrCodeInstallationFailed,
				fmt.Sprintf("Lstat 失败: %s", relSlash), lerr)
		}
		if lstat.Mode()&os.ModeSymlink != 0 {
			target, terr := os.Readlink(path)
			if terr != nil {
				return NewInstallationError(ErrCodePackageSymlinkEscape,
					fmt.Sprintf("符号链接不可读: %s", relSlash), terr)
			}
			if !s.isSymlinkSafe(relSlash, target) {
				return NewInstallationError(ErrCodePackageSymlinkEscape,
					fmt.Sprintf("符号链接逃逸: %s -> %s", relSlash, target), ErrPackageSymlinkEscape)
			}
		}

		if !info.IsDir() {
			if isExecutableFile(relSlash) {
				return NewInstallationError(ErrCodePackageExecutableFound,
					fmt.Sprintf("包含可执行文件: %s", relSlash), ErrPackageExecutableFound)
			}
		}

		return nil
	})
}

func (s *installer) isSymlinkSafe(relPath, target string) bool {
	if target == "" {
		return false
	}
	if filepath.IsAbs(target) {
		return false
	}
	cleanTarget := filepath.Clean(target)
	if strings.HasPrefix(cleanTarget, "..") {
		return false
	}
	combined := filepath.Join(filepath.Dir(relPath), cleanTarget)
	cleaned := filepath.Clean(combined)
	if strings.HasPrefix(cleaned, "..") {
		return false
	}
	return true
}

func (s *installer) verifyCopiedPackage(installDir string, pkg *processing.Package) error {
	manifestPath := filepath.Join(installDir, installationManifestFile)
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "复制后 manifest.json 不可读", err)
	}
	var manifest processing.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "复制后 manifest.json 解析失败", err)
	}
	if manifest.SchemaVersion != processing.ManifestSchemaVersion {
		return NewInstallationError(ErrCodeInstallationFailed,
			fmt.Sprintf("复制后 schemaVersion %d 不受支持", manifest.SchemaVersion), nil)
	}
	if err := processing.ValidateManifest(&manifest); err != nil {
		return NewInstallationError(ErrCodeInstallationFailed,
			fmt.Sprintf("复制后 manifest 校验失败: %v", err), err)
	}

	for _, action := range manifest.Actions {
		actionJSONPath := filepath.Join(installDir, filepath.FromSlash(action.Config))
		if _, err := os.Stat(actionJSONPath); err != nil {
			return NewInstallationError(ErrCodeInstallationFailed,
				fmt.Sprintf("复制后动作 %s 的 action.json 不存在", action.Key), err)
		}
		framesDir := filepath.Join(installDir, "actions", action.Key, installationFramesDir)
		if info, err := os.Stat(framesDir); err != nil || !info.IsDir() {
			return NewInstallationError(ErrCodeInstallationFailed,
				fmt.Sprintf("复制后动作 %s 的 frames 目录不存在", action.Key), err)
		}
		if err := s.verifyActionFrames(installDir, action); err != nil {
			return err
		}
	}

	if err := s.verifyPackagePathsSafe(installDir); err != nil {
		return err
	}

	return nil
}

func (s *installer) computePackageHash(packageDir string) (string, error) {
	files, err := listInstallationFiles(packageDir)
	if err != nil {
		return "", err
	}
	sort.Strings(files)

	hasher := sha256.New()
	for _, relPath := range files {
		hasher.Write([]byte(relPath))
		hasher.Write([]byte{0})

		absPath := filepath.Join(packageDir, filepath.FromSlash(relPath))
		f, err := os.Open(absPath)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(hasher, f); err != nil {
			f.Close()
			return "", err
		}
		f.Close()
		hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (s *installer) writeMetadataFiles(finalDir, installId string, pkg *processing.Package, packageHash string) error {
	metadata := map[string]interface{}{
		"installId":        installId,
		"packageId":        pkg.ID,
		"packageVersion":   pkg.Version,
		"characterId":      pkg.CharacterID,
		"installedAt":      time.Now().UTC().Format(time.RFC3339),
		"canvasWidth":      pkg.CanvasWidth,
		"canvasHeight":     pkg.CanvasHeight,
		"defaultActionKey": pkg.DefaultActionKey,
		"generator":        installationGenerator,
	}
	metadataData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "序列化 metadata.json 失败", err)
	}
	metadataPath := filepath.Join(finalDir, installationMetadataFile)
	if err := os.WriteFile(metadataPath, metadataData, 0o644); err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "写入 metadata.json 失败", err)
	}

	filesHashes, err := s.computeFileHashes(finalDir)
	if err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "计算完整性文件哈希失败", err)
	}
	integrity := map[string]interface{}{
		"packageHash": packageHash,
		"files":       filesHashes,
		"verifiedAt":  time.Now().UTC().Format(time.RFC3339),
	}
	integrityData, err := json.MarshalIndent(integrity, "", "  ")
	if err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "序列化 integrity.json 失败", err)
	}
	integrityPath := filepath.Join(finalDir, installationIntegrityFile)
	if err := os.WriteFile(integrityPath, integrityData, 0o644); err != nil {
		return NewInstallationError(ErrCodeInstallationFailed, "写入 integrity.json 失败", err)
	}

	return nil
}

func (s *installer) computeFileHashes(root string) (map[string]string, error) {
	files, err := listInstallationFiles(root)
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	result := make(map[string]string, len(files))
	for _, relPath := range files {
		absPath := filepath.Join(root, filepath.FromSlash(relPath))
		f, err := os.Open(absPath)
		if err != nil {
			return nil, err
		}
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			f.Close()
			return nil, err
		}
		f.Close()
		result[relPath] = hex.EncodeToString(h.Sum(nil))
	}
	return result, nil
}

func (s *installer) atomicMoveDir(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := s.atomicMoveDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := moveFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return removeTree(src)
}

func (s *installer) rollback(installId string, state *installState) {
	if state == nil {
		return
	}

	tmpDir := s.installTmpDir(installId)
	finalDir := s.installFinalDir(installId)

	if state.tmpCreated {
		if _, err := os.Stat(tmpDir); err == nil {
			_ = removeTree(tmpDir)
		}
	}

	if state.finalMoved {
		if _, err := os.Stat(finalDir); err == nil {
			_ = removeTree(finalDir)
		}
	}

	if state.dbRecordCreated {
		_ = s.repo.UpdateInstallationStatus(installId, StatusInvalid)
	}
}

func (s *installer) failWithRollback(installId string, state *installState, message string, err error) error {
	s.rollback(installId, state)
	return NewInstallationError(ErrCodeInstallationFailed, message, err)
}

func (s *installer) defaultActionConfigPath(manifest *processing.Manifest) string {
	if manifest == nil {
		return ""
	}
	for _, action := range manifest.Actions {
		if action.Key == manifest.DefaultAction {
			return action.Config
		}
	}
	return ""
}

func (s *installer) packageSourceDir(pkg *processing.Package) string {
	return filepath.Join(s.dataDir, installationRootDir, "generation-tasks",
		pkg.GenerationTaskID, "packages", pkg.ID)
}

func (s *installer) installTmpDir(installId string) string {
	return filepath.Join(s.dataDir, installationRootDir, installationSubDir,
		installationTmpSegment, installId)
}

func (s *installer) installFinalDir(installId string) string {
	return filepath.Join(s.dataDir, installationRootDir, installationSubDir, installId)
}

func (s *installer) installFinalRelPath(installId string) string {
	rel := filepath.Join(installationRootDir, installationSubDir, installId)
	return filepath.ToSlash(rel) + "/"
}

var installationHashExcludedFiles = map[string]bool{
	installationMetadataFile:  true,
	installationIntegrityFile: true,
}

func listInstallationFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		relSlash := filepath.ToSlash(rel)
		if installationHashExcludedFiles[relSlash] {
			return nil
		}
		files = append(files, relSlash)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func copyPackageTree(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}

	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if isForbiddenInstallationFile(entry.Name()) {
			continue
		}

		if entry.IsDir() {
			if err := copyPackageTree(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyInstallationFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyInstallationFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("source is a directory: %s", src)
	}
	if srcInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("source is a symlink: %s", src)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	if err := os.Chmod(dst, srcInfo.Mode().Perm()); err != nil {
		return err
	}
	return nil
}

func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyInstallationFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

func isExecutableFile(relPath string) bool {
	lower := strings.ToLower(relPath)
	for ext := range installationExecutableExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func isForbiddenInstallationFile(name string) bool {
	lower := strings.ToLower(name)

	if strings.HasSuffix(lower, ".key") {
		return true
	}
	if strings.HasSuffix(lower, ".pem") {
		return true
	}
	if strings.HasSuffix(lower, ".tmp") {
		return true
	}

	sensitiveKeywords := []string{"credentials", "secret", "api_key", "provider_response", "providerresponse"}
	for _, kw := range sensitiveKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}
