// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet"
	_ "golang.org/x/image/webp"
)

const (
	ErrCodePackageBuildFailed          = "PACKAGE_BUILD_FAILED"
	ErrCodePackageManifestInvalid      = "PACKAGE_MANIFEST_INVALID"
	ErrCodePackageFileMissing          = "PACKAGE_FILE_MISSING"
	ErrCodePackageHashFailed           = "PACKAGE_HASH_FAILED"
	ErrCodePackageDefaultActionInvalid = "PACKAGE_DEFAULT_ACTION_INVALID"

	packageIDPrefix       = "pet_"
	packageMetadataFile   = "metadata.json"
	packageManifestFile   = "manifest.json"
	packagePreviewFile    = "preview.png"
	packagePreviewSrcFile = "package-preview.png"
	packageActionJSONFile = "action.json"
	packageFramesDir      = "frames"
	packageBuildGenerator = "u-ai-processing"
)

var (
	ErrPackageBuildFailed          = errors.New("package build failed")
	ErrPackageManifestInvalid      = errors.New("package manifest invalid")
	ErrPackageFileMissing          = errors.New("package file missing")
	ErrPackageHashFailed           = errors.New("package hash failed")
	ErrPackageDefaultActionInvalid = errors.New("package default action invalid")
)

type PackageError struct {
	Code    string
	Message string
	Err     error
}

func (e *PackageError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *PackageError) Unwrap() error { return e.Err }

type PackageBuildRequest struct {
	ProcessingTaskID  string
	UserID            string
	CharacterID       string
	GenerationTaskID  string
	PackageName       string
	DefaultAction     string
	IncludedActions   []string
	CanvasWidth       int
	CanvasHeight      int
	ProcessingVersion int
	UserDefaultAction string
	SucceededActions  []desktoppet.GenerationTaskAction
}

type PackageBuildResult struct {
	Package      *Package
	Manifest     *Manifest
	PackageHash  string
	PackageDir   string
	ManifestData []byte
}

type PackageMetadata struct {
	PackageID string           `json:"packageId"`
	Version   int              `json:"version"`
	CreatedAt string           `json:"createdAt"`
	BuildInfo PackageBuildInfo `json:"buildInfo"`
}

type PackageBuildInfo struct {
	ProcessingVersion int    `json:"processingVersion"`
	Generator         string `json:"generator"`
}

type Packager struct {
	repo            Repository
	dataDir         string
	manifestBuilder *ManifestBuilder
}

func NewPackager(repo Repository, dataDir string) *Packager {
	return &Packager{
		repo:            repo,
		dataDir:         dataDir,
		manifestBuilder: NewManifestBuilder(dataDir),
	}
}

func (p *Packager) BuildPackage(req *PackageBuildRequest) (*PackageBuildResult, error) {
	return p.buildPackage(req, true)
}

// BuildReleaseSource builds the same validated package artifact without creating a
// legacy desktop_pet_packages record. The caller owns PackageDir and must remove it
// after the V2 Release has copied/published the artifact.
func (p *Packager) BuildReleaseSource(req *PackageBuildRequest) (*PackageBuildResult, error) {
	return p.buildPackage(req, false)
}

func (p *Packager) buildPackage(req *PackageBuildRequest, persistLegacyRecord bool) (*PackageBuildResult, error) {
	if req == nil {
		return nil, &PackageError{Code: ErrCodePackageBuildFailed, Message: "构建请求为空"}
	}
	if err := p.validateIncludedActions(req); err != nil {
		return nil, err
	}

	packageID := packageIDPrefix + uuid.New().String()

	packageDir := p.packageDir(req.GenerationTaskID, packageID)
	if err := os.MkdirAll(packageDir, 0755); err != nil {
		return nil, &PackageError{Code: ErrCodePackageBuildFailed, Message: "创建包目录失败", Err: err}
	}
	completed := false
	if !persistLegacyRecord {
		defer func() {
			if !completed {
				_ = os.RemoveAll(packageDir) // audit:ok: packageDir is UUID-derived under the packager-owned generation-task package root
			}
		}()
	}

	if err := p.copyActionFiles(req.GenerationTaskID, packageID, req.IncludedActions, req.ProcessingVersion); err != nil {
		return nil, err
	}

	manifestActions := buildManifestActions(req.IncludedActions, req.SucceededActions)
	manifest := BuildManifest(
		packageID,
		req.PackageName,
		req.CharacterID,
		req.GenerationTaskID,
		req.ProcessingVersion,
		req.CanvasWidth,
		req.CanvasHeight,
		req.DefaultAction,
		manifestActions,
	)

	manifestRelPath, err := p.manifestBuilder.WriteManifest(req.GenerationTaskID, packageID, manifest)
	if err != nil {
		return nil, &PackageError{Code: ErrCodePackageBuildFailed, Message: "写入 manifest.json 失败", Err: err}
	}

	if err := p.copyPackagePreview(req.GenerationTaskID, req.ProcessingVersion, packageDir); err != nil {
		return nil, err
	}

	version, err := p.resolvePackageVersion(req.GenerationTaskID)
	if err != nil {
		return nil, &PackageError{Code: ErrCodePackageBuildFailed, Message: "解析包版本失败", Err: err}
	}

	pkg := &Package{
		ID:               packageID,
		UserID:           req.UserID,
		CharacterID:      req.CharacterID,
		GenerationTaskID: req.GenerationTaskID,
		ProcessingTaskID: req.ProcessingTaskID,
		Name:             req.PackageName,
		Version:          version,
		Status:           "pending",
		DefaultActionKey: req.DefaultAction,
		CanvasWidth:      req.CanvasWidth,
		CanvasHeight:     req.CanvasHeight,
		ManifestPath:     manifestRelPath,
		PreviewPath:      p.packagePreviewRelPath(req.GenerationTaskID, packageID),
		ActionCount:      len(req.IncludedActions),
		IncludedActions:  encodeIncludedActions(req.IncludedActions),
		CreatedAt:        time.Now().Format(desktopPetTimeFormat),
		UpdatedAt:        time.Now().Format(desktopPetTimeFormat),
	}

	if err := p.generateMetadata(packageDir, pkg, req.ProcessingVersion); err != nil {
		return nil, err
	}

	if err := p.VerifyPackageIntegrity(packageDir, len(req.IncludedActions)); err != nil {
		return nil, err
	}

	hash, err := p.computePackageHash(packageDir)
	if err != nil {
		return nil, &PackageError{Code: ErrCodePackageHashFailed, Message: "计算包哈希失败", Err: err}
	}
	pkg.PackageHash = hash
	pkg.PackagePath = p.packageDirRelPath(req.GenerationTaskID, packageID)
	pkg.Status = "ready"

	if persistLegacyRecord {
		if err := p.repo.CreatePackage(pkg); err != nil {
			return nil, &PackageError{Code: ErrCodePackageBuildFailed, Message: "写入包记录失败", Err: err}
		}
	}

	manifestData, err := os.ReadFile(filepath.Join(packageDir, packageManifestFile))
	if err != nil {
		return nil, &PackageError{Code: ErrCodePackageFileMissing, Message: "读取 manifest.json 失败", Err: err}
	}

	completed = true
	return &PackageBuildResult{
		Package:      pkg,
		Manifest:     manifest,
		PackageHash:  hash,
		PackageDir:   packageDir,
		ManifestData: manifestData,
	}, nil
}

func (p *Packager) validateIncludedActions(req *PackageBuildRequest) error {
	if len(req.IncludedActions) == 0 {
		return &PackageError{Code: ErrCodePackageBuildFailed, Message: "包含动作为空"}
	}
	if req.DefaultAction == "" {
		return &PackageError{Code: ErrCodePackageDefaultActionInvalid, Message: "默认动作为空"}
	}

	succeededMap := make(map[string]desktoppet.GenerationTaskAction, len(req.SucceededActions))
	for _, a := range req.SucceededActions {
		succeededMap[a.ActionKey] = a
	}

	includedSet := make(map[string]bool, len(req.IncludedActions))
	for _, key := range req.IncludedActions {
		includedSet[key] = true
	}

	for _, key := range req.IncludedActions {
		action, ok := succeededMap[key]
		if !ok {
			return &PackageError{Code: ErrCodePackageBuildFailed, Message: fmt.Sprintf("动作 %s 不在成功动作中", key)}
		}
		if action.Status != "succeeded" {
			return &PackageError{Code: ErrCodePackageBuildFailed, Message: fmt.Sprintf("动作 %s 状态为 %s，非 succeeded", key, action.Status)}
		}
	}

	if !includedSet[req.DefaultAction] {
		return &PackageError{Code: ErrCodePackageDefaultActionInvalid, Message: fmt.Sprintf("默认动作 %s 不在包含动作中", req.DefaultAction)}
	}

	defaultAction, ok := succeededMap[req.DefaultAction]
	if !ok {
		return &PackageError{Code: ErrCodePackageDefaultActionInvalid, Message: fmt.Sprintf("默认动作 %s 不在成功动作中", req.DefaultAction)}
	}
	if defaultAction.SupportsDefaultIdle != 1 {
		return &PackageError{Code: ErrCodePackageDefaultActionInvalid, Message: fmt.Sprintf("默认动作 %s 不支持待机", req.DefaultAction)}
	}

	return nil
}

func (p *Packager) copyActionFiles(taskID, packageID string, actionKeys []string, processingVersion int) error {
	for _, actionKey := range actionKeys {
		srcDir := p.processedActionDir(taskID, processingVersion, actionKey)
		dstDir := p.packageActionDir(taskID, packageID, actionKey)

		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return &PackageError{Code: ErrCodePackageBuildFailed, Message: fmt.Sprintf("创建动作目录失败: %s", actionKey), Err: err}
		}

		srcActionJSON := filepath.Join(srcDir, packageActionJSONFile)
		dstActionJSON := filepath.Join(dstDir, packageActionJSONFile)
		if err := copyFile(srcActionJSON, dstActionJSON); err != nil {
			return &PackageError{Code: ErrCodePackageFileMissing, Message: fmt.Sprintf("复制 action.json 失败: %s", actionKey), Err: err}
		}

		srcPreview := filepath.Join(srcDir, packagePreviewFile)
		dstPreview := filepath.Join(dstDir, packagePreviewFile)
		if err := copyFile(srcPreview, dstPreview); err != nil {
			return &PackageError{Code: ErrCodePackageFileMissing, Message: fmt.Sprintf("复制 preview.png 失败: %s", actionKey), Err: err}
		}

		srcFrames := filepath.Join(srcDir, packageFramesDir)
		dstFrames := filepath.Join(dstDir, packageFramesDir)
		if err := copyDir(srcFrames, dstFrames); err != nil {
			return &PackageError{Code: ErrCodePackageFileMissing, Message: fmt.Sprintf("复制 frames 目录失败: %s", actionKey), Err: err}
		}
	}
	return nil
}

func (p *Packager) copyPackagePreview(taskID string, processingVersion int, packageDir string) error {
	srcPath := filepath.Join(p.dataDir, "desktop-pets", "generation-tasks", taskID, "processed",
		fmt.Sprintf("version-%d", processingVersion), packagePreviewSrcFile)
	dstPath := filepath.Join(packageDir, packagePreviewFile)

	if _, err := os.Stat(srcPath); err != nil {
		return &PackageError{Code: ErrCodePackageFileMissing, Message: "package-preview.png 不存在", Err: err}
	}
	if err := copyFile(srcPath, dstPath); err != nil {
		return &PackageError{Code: ErrCodePackageBuildFailed, Message: "复制 package-preview.png 失败", Err: err}
	}
	return nil
}

func (p *Packager) generateMetadata(packageDir string, pkg *Package, processingVersion int) error {
	metadata := PackageMetadata{
		PackageID: pkg.ID,
		Version:   pkg.Version,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		BuildInfo: PackageBuildInfo{
			ProcessingVersion: processingVersion,
			Generator:         packageBuildGenerator,
		},
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return &PackageError{Code: ErrCodePackageBuildFailed, Message: "序列化 metadata.json 失败", Err: err}
	}

	metadataPath := filepath.Join(packageDir, packageMetadataFile)
	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return &PackageError{Code: ErrCodePackageBuildFailed, Message: "写入 metadata.json 失败", Err: err}
	}
	return nil
}

func (p *Packager) computePackageHash(packageDir string) (string, error) {
	files, err := listPackageFiles(packageDir)
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

func (p *Packager) VerifyPackageIntegrity(packageDir string, expectedActionCount int) error {
	manifestPath := filepath.Join(packageDir, packageManifestFile)
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return &PackageError{Code: ErrCodePackageManifestInvalid, Message: "manifest.json 不存在或不可读", Err: err}
	}

	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return &PackageError{Code: ErrCodePackageManifestInvalid, Message: "manifest.json 解析失败", Err: err}
	}

	if manifest.SchemaVersion != ManifestSchemaVersion {
		return &PackageError{Code: ErrCodePackageManifestInvalid, Message: fmt.Sprintf("schemaVersion %d 不受支持", manifest.SchemaVersion)}
	}

	if err := ValidateManifest(&manifest); err != nil {
		return &PackageError{Code: ErrCodePackageManifestInvalid, Message: "manifest 校验失败", Err: err}
	}

	if expectedActionCount > 0 && len(manifest.Actions) != expectedActionCount {
		return &PackageError{Code: ErrCodePackageManifestInvalid, Message: fmt.Sprintf("动作数量 %d 与期望 %d 不一致", len(manifest.Actions), expectedActionCount)}
	}

	for _, action := range manifest.Actions {
		if err := p.verifyActionIntegrity(packageDir, action, manifest.Canvas); err != nil {
			return err
		}
	}

	files, err := listPackageFiles(packageDir)
	if err != nil {
		return &PackageError{Code: ErrCodePackageHashFailed, Message: "列出包文件失败", Err: err}
	}

	for _, relPath := range files {
		if strings.Contains(relPath, "..") {
			return &PackageError{Code: ErrCodePackageManifestInvalid, Message: fmt.Sprintf("路径包含穿越: %s", relPath)}
		}

		if isForbiddenFile(relPath) {
			return &PackageError{Code: ErrCodePackageFileMissing, Message: fmt.Sprintf("包含禁止文件: %s", relPath)}
		}

		absPath := filepath.Join(packageDir, filepath.FromSlash(relPath))
		f, err := os.Open(absPath)
		if err != nil {
			return &PackageError{Code: ErrCodePackageHashFailed, Message: fmt.Sprintf("文件不可读: %s", relPath), Err: err}
		}
		f.Close()
	}

	metadataPath := filepath.Join(packageDir, packageMetadataFile)
	if metadataData, err := os.ReadFile(metadataPath); err == nil {
		var raw map[string]interface{}
		if err := json.Unmarshal(metadataData, &raw); err == nil {
			if hasForbiddenMetadata(raw) {
				return &PackageError{Code: ErrCodePackageFileMissing, Message: "metadata.json 包含敏感字段"}
			}
		}
	}

	return nil
}

func (p *Packager) verifyActionIntegrity(packageDir string, action ManifestAction, canvas ManifestCanvas) error {
	actionDir := filepath.Join(packageDir, "actions", action.Key)
	actionJSONPath := filepath.Join(actionDir, packageActionJSONFile)
	framesDir := filepath.Join(actionDir, packageFramesDir)

	actionData, err := os.ReadFile(actionJSONPath)
	if err != nil {
		return &PackageError{Code: ErrCodePackageFileMissing, Message: fmt.Sprintf("动作 %s 的 action.json 不存在", action.Key), Err: err}
	}

	info, err := os.Stat(framesDir)
	if err != nil || !info.IsDir() {
		return &PackageError{Code: ErrCodePackageFileMissing, Message: fmt.Sprintf("动作 %s 的 frames 目录不存在", action.Key), Err: err}
	}

	var actionJSON ActionJSON
	if err := json.Unmarshal(actionData, &actionJSON); err != nil {
		return &PackageError{Code: ErrCodePackageManifestInvalid, Message: fmt.Sprintf("动作 %s 的 action.json 解析失败", action.Key), Err: err}
	}

	frameEntries, err := os.ReadDir(framesDir)
	if err != nil {
		return &PackageError{Code: ErrCodePackageFileMissing, Message: fmt.Sprintf("读取动作 %s 的 frames 目录失败", action.Key), Err: err}
	}

	frameFiles := make([]string, 0, len(frameEntries))
	actualFrameCount := 0
	for _, entry := range frameEntries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		actualFrameCount++
		frameFiles = append(frameFiles, entry.Name())
	}

	expectedFrameCount := actionJSON.FrameCount
	if expectedFrameCount == 0 {
		expectedFrameCount = len(actionJSON.Frames)
	}
	if actualFrameCount != expectedFrameCount {
		return &PackageError{Code: ErrCodePackageManifestInvalid, Message: fmt.Sprintf("动作 %s 帧数量不一致: 配置 %d, 实际 %d", action.Key, expectedFrameCount, actualFrameCount)}
	}

	for _, frameName := range frameFiles {
		framePath := filepath.Join(framesDir, frameName)
		f, err := os.Open(framePath)
		if err != nil {
			return &PackageError{Code: ErrCodePackageFileMissing, Message: fmt.Sprintf("动作 %s 的帧文件 %s 无法打开", action.Key, frameName), Err: err}
		}
		config, _, err := image.DecodeConfig(f)
		f.Close()
		if err != nil {
			return &PackageError{Code: ErrCodePackageFileMissing, Message: fmt.Sprintf("动作 %s 的帧文件 %s 解码失败", action.Key, frameName), Err: err}
		}
		if config.Width != canvas.Width || config.Height != canvas.Height {
			return &PackageError{Code: ErrCodePackageFileMissing, Message: fmt.Sprintf("动作 %s 的帧文件 %s 尺寸 %dx%d 与画布 %dx%d 不符", action.Key, frameName, config.Width, config.Height, canvas.Width, canvas.Height)}
		}
	}

	return nil
}

func (p *Packager) resolvePackageVersion(generationTaskID string) (int, error) {
	existing, err := p.repo.ListPackagesByGenerationTask(generationTaskID)
	if err != nil {
		return 0, err
	}
	maxVersion := 0
	for _, pkg := range existing {
		if pkg.Version > maxVersion {
			maxVersion = pkg.Version
		}
	}
	return maxVersion + 1, nil
}

func (p *Packager) packageDir(taskID, packageID string) string {
	return filepath.Join(p.dataDir, "desktop-pets", "generation-tasks", taskID, "packages", packageID)
}

func (p *Packager) packageDirRelPath(taskID, packageID string) string {
	rel := filepath.Join("desktop-pets", "generation-tasks", taskID, "packages", packageID)
	return filepath.ToSlash(rel) + "/"
}

func (p *Packager) packagePreviewRelPath(taskID, packageID string) string {
	rel := filepath.Join("desktop-pets", "generation-tasks", taskID, "packages", packageID, packagePreviewFile)
	return filepath.ToSlash(rel)
}

func (p *Packager) processedActionDir(taskID string, processingVersion int, actionKey string) string {
	return filepath.Join(p.dataDir, "desktop-pets", "generation-tasks", taskID, "processed",
		fmt.Sprintf("version-%d", processingVersion), "actions", actionKey)
}

func (p *Packager) packageActionDir(taskID, packageID, actionKey string) string {
	return filepath.Join(p.dataDir, "desktop-pets", "generation-tasks", taskID, "packages", packageID, "actions", actionKey)
}

func listPackageFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func isForbiddenFile(filename string) bool {
	lower := strings.ToLower(filename)

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

	normalized := strings.ReplaceAll(lower, "\\", "/")
	parts := strings.Split(normalized, "/")
	for _, part := range parts {
		if part == "tmp" {
			return true
		}
		if strings.HasPrefix(part, "attempt-") {
			return true
		}
		if strings.HasPrefix(part, ".tmp") {
			return true
		}
	}
	return false
}

func hasForbiddenMetadata(raw map[string]interface{}) bool {
	forbiddenKeys := []string{"provider_response", "providerresponse", "api_key", "apikey", "secret", "credentials"}
	for key := range raw {
		lowerKey := strings.ToLower(key)
		for _, fk := range forbiddenKeys {
			if strings.Contains(lowerKey, fk) {
				return true
			}
		}
	}
	return false
}

func buildManifestActions(includedActions []string, succeededActions []desktoppet.GenerationTaskAction) []ManifestAction {
	nameMap := make(map[string]string, len(succeededActions))
	for _, a := range succeededActions {
		nameMap[a.ActionKey] = a.ActionNameSnapshot
	}

	result := make([]ManifestAction, 0, len(includedActions))
	for _, key := range includedActions {
		name := nameMap[key]
		if name == "" {
			name = key
		}
		result = append(result, BuildManifestAction(key, name))
	}
	return result
}

func encodeIncludedActions(actions []string) string {
	data, err := json.Marshal(actions)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("source is a directory: %s", src)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
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
	return nil
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
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
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if isForbiddenFile(entry.Name()) {
				continue
			}
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if isForbiddenFile(entry.Name()) {
				continue
			}
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}
