// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/desktoppet"
	"gorm.io/gorm"
)

func setupPackagerProcessedAction(t *testing.T, dataDir, taskID string, version int, actionKey string, frameCount int) {
	t.Helper()
	relDir := filepath.Join("desktop-pets", "generation-tasks", taskID, "processed",
		filepath.FromSlash("version-"+itoa(version)), "actions", actionKey)

	actionJSON := BuildActionJSON(actionKey, actionKey, frameCount, 10, DefaultFeetCenterAnchor, "loop")
	actionData, err := json.MarshalIndent(actionJSON, "", "  ")
	if err != nil {
		t.Fatalf("marshal action.json for %s: %v", actionKey, err)
	}
	writeFileBytes(t, dataDir, filepath.ToSlash(filepath.Join(relDir, "action.json")), actionData)

	writeValidatorPNG(t, dataDir, filepath.ToSlash(filepath.Join(relDir, "preview.png")), 32, 32)

	for i := 0; i < frameCount; i++ {
		frameFile := frameFileName(i)
		relFrame := filepath.ToSlash(filepath.Join(relDir, "frames", frameFile))
		writeValidatorPNG(t, dataDir, relFrame, 512, 512)
	}
}

func setupPackagePreviewFile(t *testing.T, dataDir, taskID string, version int) {
	t.Helper()
	relDir := filepath.Join("desktop-pets", "generation-tasks", taskID, "processed",
		filepath.FromSlash("version-"+itoa(version)))
	writeValidatorPNG(t, dataDir, filepath.ToSlash(filepath.Join(relDir, "package-preview.png")), 32, 32)
}

func seedPackagerTask(t *testing.T, db *gorm.DB, taskID, userID, status string) {
	t.Helper()
	if err := db.Create(&desktoppet.GenerationTask{
		ID:     taskID,
		UserID: userID,
		Name:   "打包测试任务",
		Status: status,
	}).Error; err != nil {
		t.Fatalf("create generation task %s: %v", taskID, err)
	}
}

func seedPackagerAction(t *testing.T, db *gorm.DB, actionID, taskID, actionKey, status string, supportsIdle int, frameCount int) desktoppet.GenerationTaskAction {
	t.Helper()
	action := desktoppet.GenerationTaskAction{
		ID:                    actionID,
		TaskID:                taskID,
		ActionKey:             actionKey,
		ActionNameSnapshot:    actionKey,
		Status:                status,
		SupportsDefaultIdle:   supportsIdle,
		SortOrder:             1,
		FrameCount:            frameCount,
		GenerationSpecVersion: "v1",
	}
	if err := db.Create(&action).Error; err != nil {
		t.Fatalf("create action %s: %v", actionID, err)
	}
	return action
}

func assertPackageErrorCode(t *testing.T, err error, wantCode string) {
	t.Helper()
	if err == nil {
		t.Fatalf("期望错误但得到 nil")
	}
	var pe *PackageError
	if !errors.As(err, &pe) {
		t.Fatalf("期望 *PackageError，实际类型 %T: %v", err, err)
	}
	if pe.Code != wantCode {
		t.Fatalf("Code = %s, 期望 %s", pe.Code, wantCode)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf []byte
	if n < 0 {
		buf = append(buf, '-')
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	for i := len(digits) - 1; i >= 0; i-- {
		buf = append(buf, digits[i])
	}
	return string(buf)
}

func frameFileName(index int) string {
	return "frame-" + padZero4(index+1) + ".png"
}

func padZero4(n int) string {
	s := itoa(n)
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}

func TestPackager_BuildPackage_Success(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	dataDir := t.TempDir()

	taskID := "gt-pkg-success"
	seedPackagerTask(t, db, taskID, "user-1", "succeeded")

	action1 := seedPackagerAction(t, db, "gta-1", taskID, "idle_normal", "succeeded", 1, 2)
	action2 := seedPackagerAction(t, db, "gta-2", taskID, "wave", "succeeded", 0, 1)

	setupPackagerProcessedAction(t, dataDir, taskID, 1, "idle_normal", 2)
	setupPackagerProcessedAction(t, dataDir, taskID, 1, "wave", 1)
	setupPackagePreviewFile(t, dataDir, taskID, 1)

	p := NewPackager(repo, dataDir)
	req := &PackageBuildRequest{
		ProcessingTaskID:  "pt-1",
		UserID:            "user-1",
		CharacterID:       "char-1",
		GenerationTaskID:  taskID,
		PackageName:       "测试包",
		DefaultAction:     "idle_normal",
		IncludedActions:   []string{"idle_normal", "wave"},
		CanvasWidth:       512,
		CanvasHeight:      512,
		ProcessingVersion: 1,
		SucceededActions:  []desktoppet.GenerationTaskAction{action1, action2},
	}

	result, err := p.BuildPackage(req)
	if err != nil {
		t.Fatalf("BuildPackage 失败: %v", err)
	}

	if result.Package == nil {
		t.Fatal("Package 为空")
	}
	if result.Manifest == nil {
		t.Fatal("Manifest 为空")
	}
	if result.PackageHash == "" {
		t.Fatal("PackageHash 为空")
	}

	if result.Package.Status != "ready" {
		t.Fatalf("Status = %s, 期望 ready", result.Package.Status)
	}
	if result.Package.ID == "" {
		t.Fatal("ID 为空")
	}
	if len(result.Package.ID) <= len(packageIDPrefix) {
		t.Fatalf("ID 缺少前缀: %s", result.Package.ID)
	}
	if result.Package.ID[:len(packageIDPrefix)] != packageIDPrefix {
		t.Fatalf("ID 前缀不正确: %s", result.Package.ID)
	}
	if result.Package.DefaultActionKey != "idle_normal" {
		t.Fatalf("DefaultActionKey = %s, 期望 idle_normal", result.Package.DefaultActionKey)
	}
	if result.Package.ActionCount != 2 {
		t.Fatalf("ActionCount = %d, 期望 2", result.Package.ActionCount)
	}
	if result.Package.Version != 1 {
		t.Fatalf("Version = %d, 期望 1", result.Package.Version)
	}

	packageDir := filepath.Join(dataDir, "desktop-pets", "generation-tasks", taskID, "packages", result.Package.ID)
	if _, err := os.Stat(filepath.Join(packageDir, "manifest.json")); err != nil {
		t.Fatalf("manifest.json 不存在: %v", err)
	}
	if _, err := os.Stat(filepath.Join(packageDir, "preview.png")); err != nil {
		t.Fatalf("preview.png 不存在: %v", err)
	}
	if _, err := os.Stat(filepath.Join(packageDir, "metadata.json")); err != nil {
		t.Fatalf("metadata.json 不存在: %v", err)
	}
	if _, err := os.Stat(filepath.Join(packageDir, "actions", "idle_normal", "action.json")); err != nil {
		t.Fatalf("idle_normal action.json 不存在: %v", err)
	}
	if _, err := os.Stat(filepath.Join(packageDir, "actions", "idle_normal", "frames", "frame-0001.png")); err != nil {
		t.Fatalf("idle_normal frame-0001.png 不存在: %v", err)
	}
	if _, err := os.Stat(filepath.Join(packageDir, "actions", "idle_normal", "frames", "frame-0002.png")); err != nil {
		t.Fatalf("idle_normal frame-0002.png 不存在: %v", err)
	}

	gotPkg, err := repo.GetPackage(result.Package.ID)
	if err != nil {
		t.Fatalf("GetPackage 失败: %v", err)
	}
	if gotPkg.Status != "ready" {
		t.Fatalf("数据库中 Status = %s, 期望 ready", gotPkg.Status)
	}
	if gotPkg.PackageHash != result.PackageHash {
		t.Fatalf("数据库中 PackageHash = %s, 期望 %s", gotPkg.PackageHash, result.PackageHash)
	}
}

func TestPackager_ValidateIncludedActions_ActionNotSucceeded(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	p := NewPackager(repo, t.TempDir())

	req := &PackageBuildRequest{
		DefaultAction:   "idle_normal",
		IncludedActions: []string{"idle_normal", "walk_left"},
		SucceededActions: []desktoppet.GenerationTaskAction{
			{ActionKey: "idle_normal", Status: "succeeded", SupportsDefaultIdle: 1},
			{ActionKey: "walk_left", Status: "failed", SupportsDefaultIdle: 0},
		},
	}

	err := p.validateIncludedActions(req)
	assertPackageErrorCode(t, err, ErrCodePackageBuildFailed)
}

func TestPackager_ValidateIncludedActions_DefaultActionNotIncluded(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	p := NewPackager(repo, t.TempDir())

	req := &PackageBuildRequest{
		DefaultAction:   "idle_normal",
		IncludedActions: []string{"wave"},
		SucceededActions: []desktoppet.GenerationTaskAction{
			{ActionKey: "idle_normal", Status: "succeeded", SupportsDefaultIdle: 1},
			{ActionKey: "wave", Status: "succeeded", SupportsDefaultIdle: 0},
		},
	}

	err := p.validateIncludedActions(req)
	assertPackageErrorCode(t, err, ErrCodePackageDefaultActionInvalid)
}

func TestPackager_ValidateIncludedActions_DefaultActionNotSupportsIdle(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	p := NewPackager(repo, t.TempDir())

	req := &PackageBuildRequest{
		DefaultAction:   "wave",
		IncludedActions: []string{"wave"},
		SucceededActions: []desktoppet.GenerationTaskAction{
			{ActionKey: "wave", Status: "succeeded", SupportsDefaultIdle: 0},
		},
	}

	err := p.validateIncludedActions(req)
	assertPackageErrorCode(t, err, ErrCodePackageDefaultActionInvalid)
}

func TestPackager_ComputePackageHash_Consistency(t *testing.T) {
	dir := t.TempDir()

	writeFileBytes(t, dir, "manifest.json", []byte(`{"schemaVersion":1}`))
	writeFileBytes(t, dir, "preview.png", []byte("preview-data"))
	writeFileBytes(t, dir, "actions/idle_normal/action.json", []byte(`{"key":"idle_normal"}`))
	writeValidatorPNG(t, dir, "actions/idle_normal/frames/frame-0001.png", 8, 8)

	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	p := NewPackager(repo, dir)

	hash1, err := p.computePackageHash(dir)
	if err != nil {
		t.Fatalf("第一次 computePackageHash 失败: %v", err)
	}
	if hash1 == "" {
		t.Fatal("hash 为空")
	}

	hash2, err := p.computePackageHash(dir)
	if err != nil {
		t.Fatalf("第二次 computePackageHash 失败: %v", err)
	}
	if hash1 != hash2 {
		t.Fatalf("相同内容 hash 不一致: %s vs %s", hash1, hash2)
	}

	writeFileBytes(t, dir, "actions/idle_normal/action.json", []byte(`{"key":"idle_normal","modified":true}`))
	hash3, err := p.computePackageHash(dir)
	if err != nil {
		t.Fatalf("修改后 computePackageHash 失败: %v", err)
	}
	if hash1 == hash3 {
		t.Fatal("内容变更后 hash 未变化")
	}
}

func TestPackager_VerifyPackageIntegrity_ManifestMissing(t *testing.T) {
	dir := t.TempDir()
	p := NewPackager(nil, dir)

	err := p.VerifyPackageIntegrity(dir, 0)
	assertPackageErrorCode(t, err, ErrCodePackageManifestInvalid)
}

func TestPackager_VerifyPackageIntegrity_ManifestInvalid(t *testing.T) {
	dir := t.TempDir()
	writeFileBytes(t, dir, "manifest.json", []byte(`{invalid json}`))

	p := NewPackager(nil, dir)
	err := p.VerifyPackageIntegrity(dir, 0)
	assertPackageErrorCode(t, err, ErrCodePackageManifestInvalid)
}

func TestPackager_VerifyPackageIntegrity_ActionConfigMissing(t *testing.T) {
	dir := t.TempDir()

	manifest := BuildManifest("pkg-test", "测试", "char-1", "task-1", 1, 512, 512, "idle_normal",
		[]ManifestAction{BuildManifestAction("idle_normal", "待机")})
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	writeFileBytes(t, dir, "manifest.json", manifestData)

	framesDir := filepath.Join(dir, "actions", "idle_normal", "frames")
	if err := os.MkdirAll(framesDir, 0755); err != nil {
		t.Fatalf("mkdir frames: %v", err)
	}
	writeValidatorPNG(t, dir, "actions/idle_normal/frames/frame-0001.png", 8, 8)

	p := NewPackager(nil, dir)
	err := p.VerifyPackageIntegrity(dir, 1)
	assertPackageErrorCode(t, err, ErrCodePackageFileMissing)
}

func TestPackager_VerifyPackageIntegrity_FrameCountMismatch(t *testing.T) {
	dir := t.TempDir()

	actionJSON := BuildActionJSON("idle_normal", "待机", 3, 10, DefaultFeetCenterAnchor, "loop")
	actionData, _ := json.MarshalIndent(actionJSON, "", "  ")
	writeFileBytes(t, dir, "actions/idle_normal/action.json", actionData)

	framesDir := filepath.Join(dir, "actions", "idle_normal", "frames")
	if err := os.MkdirAll(framesDir, 0755); err != nil {
		t.Fatalf("mkdir frames: %v", err)
	}
	writeValidatorPNG(t, dir, "actions/idle_normal/frames/frame-0001.png", 8, 8)

	manifest := BuildManifest("pkg-test", "测试", "char-1", "task-1", 1, 512, 512, "idle_normal",
		[]ManifestAction{BuildManifestAction("idle_normal", "待机")})
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	writeFileBytes(t, dir, "manifest.json", manifestData)

	p := NewPackager(nil, dir)
	err := p.VerifyPackageIntegrity(dir, 1)
	assertPackageErrorCode(t, err, ErrCodePackageManifestInvalid)
}

func TestPackager_VerifyPackageIntegrity_PathTraversal(t *testing.T) {
	dir := t.TempDir()

	actionJSON := BuildActionJSON("idle_normal", "待机", 1, 10, DefaultFeetCenterAnchor, "loop")
	actionData, _ := json.MarshalIndent(actionJSON, "", "  ")
	writeFileBytes(t, dir, "actions/idle_normal/action.json", actionData)

	framesDir := filepath.Join(dir, "actions", "idle_normal", "frames")
	if err := os.MkdirAll(framesDir, 0755); err != nil {
		t.Fatalf("mkdir frames: %v", err)
	}
	writeValidatorPNG(t, dir, "actions/idle_normal/frames/frame-0001.png", 8, 8)

	manifest := &Manifest{
		SchemaVersion: ManifestSchemaVersion,
		PackageID:     "pkg-test",
		Name:          "测试",
		DefaultAction: "idle_normal",
		Preview:       "preview.png",
		Actions: []ManifestAction{
			{Key: "idle_normal", Name: "待机", Config: "../actions/idle_normal/action.json"},
		},
	}
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	writeFileBytes(t, dir, "manifest.json", manifestData)
	writeValidatorPNG(t, dir, "preview.png", 8, 8)

	p := NewPackager(nil, dir)
	err := p.VerifyPackageIntegrity(dir, 1)
	assertPackageErrorCode(t, err, ErrCodePackageManifestInvalid)
}

func TestPackager_VerifyPackageIntegrity_ForbiddenFile(t *testing.T) {
	dir := t.TempDir()

	actionJSON := BuildActionJSON("idle_normal", "待机", 1, 10, DefaultFeetCenterAnchor, "loop")
	actionData, _ := json.MarshalIndent(actionJSON, "", "  ")
	writeFileBytes(t, dir, "actions/idle_normal/action.json", actionData)

	framesDir := filepath.Join(dir, "actions", "idle_normal", "frames")
	if err := os.MkdirAll(framesDir, 0755); err != nil {
		t.Fatalf("mkdir frames: %v", err)
	}
	writeValidatorPNG(t, dir, "actions/idle_normal/frames/frame-0001.png", 8, 8)

	writeFileBytes(t, dir, "secret.key", []byte("fake-key"))

	manifest := BuildManifest("pkg-test", "测试", "char-1", "task-1", 1, 8, 8, "idle_normal",
		[]ManifestAction{BuildManifestAction("idle_normal", "待机")})
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	writeFileBytes(t, dir, "manifest.json", manifestData)
	writeValidatorPNG(t, dir, "preview.png", 8, 8)

	p := NewPackager(nil, dir)
	err := p.VerifyPackageIntegrity(dir, 1)
	assertPackageErrorCode(t, err, ErrCodePackageFileMissing)
}

func TestPackager_VerifyPackageIntegrity_Success(t *testing.T) {
	dir := t.TempDir()

	frameCount := 2
	actionJSON := BuildActionJSON("idle_normal", "待机", frameCount, 10, DefaultFeetCenterAnchor, "loop")
	actionData, _ := json.MarshalIndent(actionJSON, "", "  ")
	writeFileBytes(t, dir, "actions/idle_normal/action.json", actionData)

	framesDir := filepath.Join(dir, "actions", "idle_normal", "frames")
	if err := os.MkdirAll(framesDir, 0755); err != nil {
		t.Fatalf("mkdir frames: %v", err)
	}
	for i := 0; i < frameCount; i++ {
		writeValidatorPNG(t, dir, filepath.ToSlash(filepath.Join("actions", "idle_normal", "frames", frameFileName(i))), 8, 8)
	}

	manifest := BuildManifest("pkg-test", "测试", "char-1", "task-1", 1, 8, 8, "idle_normal",
		[]ManifestAction{BuildManifestAction("idle_normal", "待机")})
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	writeFileBytes(t, dir, "manifest.json", manifestData)
	writeValidatorPNG(t, dir, "preview.png", 8, 8)

	metadata := PackageMetadata{PackageID: "pkg-test", Version: 1, BuildInfo: PackageBuildInfo{Generator: packageBuildGenerator}}
	metadataData, _ := json.MarshalIndent(metadata, "", "  ")
	writeFileBytes(t, dir, "metadata.json", metadataData)

	p := NewPackager(nil, dir)
	if err := p.VerifyPackageIntegrity(dir, 1); err != nil {
		t.Fatalf("VerifyPackageIntegrity 失败: %v", err)
	}
}

func TestPackager_CopyActionFiles(t *testing.T) {
	dataDir := t.TempDir()
	taskID := "gt-copy"
	packageID := "pet_copy_test"
	processingVersion := 1
	actionKey := "idle_normal"
	frameCount := 2

	setupPackagerProcessedAction(t, dataDir, taskID, processingVersion, actionKey, frameCount)

	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	p := NewPackager(repo, dataDir)

	err := p.copyActionFiles(taskID, packageID, []string{actionKey}, processingVersion)
	if err != nil {
		t.Fatalf("copyActionFiles 失败: %v", err)
	}

	packageDir := filepath.Join(dataDir, "desktop-pets", "generation-tasks", taskID, "packages", packageID)
	actionDir := filepath.Join(packageDir, "actions", actionKey)

	if _, err := os.Stat(filepath.Join(actionDir, "action.json")); err != nil {
		t.Fatalf("action.json 未复制: %v", err)
	}
	if _, err := os.Stat(filepath.Join(actionDir, "preview.png")); err != nil {
		t.Fatalf("preview.png 未复制: %v", err)
	}
	for i := 0; i < frameCount; i++ {
		framePath := filepath.Join(actionDir, "frames", frameFileName(i))
		if _, err := os.Stat(framePath); err != nil {
			t.Fatalf("帧文件 %s 未复制: %v", frameFileName(i), err)
		}
	}

	srcActionData, _ := os.ReadFile(filepath.Join(dataDir, "desktop-pets", "generation-tasks", taskID, "processed",
		"version-1", "actions", actionKey, "action.json"))
	dstActionData, _ := os.ReadFile(filepath.Join(actionDir, "action.json"))
	if string(srcActionData) != string(dstActionData) {
		t.Fatal("action.json 内容不一致")
	}
}

func TestPackager_CopyActionFiles_SourceMissing(t *testing.T) {
	dataDir := t.TempDir()
	taskID := "gt-copy-missing"
	packageID := "pet_copy_missing"
	processingVersion := 1

	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	p := NewPackager(repo, dataDir)

	err := p.copyActionFiles(taskID, packageID, []string{"idle_normal"}, processingVersion)
	if err == nil {
		t.Fatal("期望错误但得到 nil")
	}
	assertPackageErrorCode(t, err, ErrCodePackageFileMissing)
}

func TestPackager_IsForbiddenFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"key文件", "secret.key", true},
		{"pem文件", "cert.pem", true},
		{"tmp文件", "data.tmp", true},
		{"tmp目录", "actions/tmp/file.png", true},
		{"tmp目录反斜杠", "actions\\tmp\\file.png", true},
		{"credentials文件", "credentials.json", true},
		{"secret关键词", "my_secret_data.json", true},
		{"api_key文件", "api_key.json", true},
		{"provider_response文件", "provider_response.json", true},
		{"attempt目录", "generated/attempt-1/file.png", true},
		{"正常png文件", "actions/idle_normal/frames/frame-0001.png", false},
		{"正常json文件", "actions/idle_normal/action.json", false},
		{"正常preview", "preview.png", false},
		{"正常manifest", "manifest.json", false},
		{"正常metadata", "metadata.json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isForbiddenFile(tt.filename)
			if got != tt.want {
				t.Fatalf("isForbiddenFile(%q) = %v, 期望 %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestPackager_BuildPackage_PreviewMissing(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	dataDir := t.TempDir()

	taskID := "gt-pkg-no-preview"
	seedPackagerTask(t, db, taskID, "user-1", "succeeded")

	action1 := seedPackagerAction(t, db, "gta-1", taskID, "idle_normal", "succeeded", 1, 2)

	setupPackagerProcessedAction(t, dataDir, taskID, 1, "idle_normal", 2)

	p := NewPackager(repo, dataDir)
	req := &PackageBuildRequest{
		ProcessingTaskID:  "pt-1",
		UserID:            "user-1",
		CharacterID:       "char-1",
		GenerationTaskID:  taskID,
		PackageName:       "测试包",
		DefaultAction:     "idle_normal",
		IncludedActions:   []string{"idle_normal"},
		CanvasWidth:       512,
		CanvasHeight:      512,
		ProcessingVersion: 1,
		SucceededActions:  []desktoppet.GenerationTaskAction{action1},
	}

	_, err := p.BuildPackage(req)
	if err == nil {
		t.Fatal("期望错误但得到 nil")
	}
	assertPackageErrorCode(t, err, ErrCodePackageFileMissing)
}

func TestPackager_BuildPackage_IncludedActionNotInSucceeded(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	dataDir := t.TempDir()

	taskID := "gt-pkg-not-succeeded"
	seedPackagerTask(t, db, taskID, "user-1", "succeeded")

	action1 := seedPackagerAction(t, db, "gta-1", taskID, "idle_normal", "succeeded", 1, 2)

	setupPackagerProcessedAction(t, dataDir, taskID, 1, "idle_normal", 2)
	setupPackagePreviewFile(t, dataDir, taskID, 1)

	p := NewPackager(repo, dataDir)
	req := &PackageBuildRequest{
		ProcessingTaskID:  "pt-1",
		UserID:            "user-1",
		CharacterID:       "char-1",
		GenerationTaskID:  taskID,
		PackageName:       "测试包",
		DefaultAction:     "idle_normal",
		IncludedActions:   []string{"idle_normal", "walk_left"},
		CanvasWidth:       512,
		CanvasHeight:      512,
		ProcessingVersion: 1,
		SucceededActions:  []desktoppet.GenerationTaskAction{action1},
	}

	_, err := p.BuildPackage(req)
	assertPackageErrorCode(t, err, ErrCodePackageBuildFailed)
}

func TestPackager_BuildPackage_VersionIncrement(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	dataDir := t.TempDir()

	taskID := "gt-pkg-version"
	seedPackagerTask(t, db, taskID, "user-1", "succeeded")

	action1 := seedPackagerAction(t, db, "gta-1", taskID, "idle_normal", "succeeded", 1, 2)

	setupPackagerProcessedAction(t, dataDir, taskID, 1, "idle_normal", 2)
	setupPackagePreviewFile(t, dataDir, taskID, 1)

	if err := repo.CreatePackage(&Package{
		ID:               "pkg-existing-1",
		UserID:           "user-1",
		GenerationTaskID: taskID,
		ProcessingTaskID: "pt-old",
		Name:             "旧包",
		Version:          1,
		Status:           "ready",
	}); err != nil {
		t.Fatalf("CreatePackage existing: %v", err)
	}

	p := NewPackager(repo, dataDir)
	req := &PackageBuildRequest{
		ProcessingTaskID:  "pt-1",
		UserID:            "user-1",
		CharacterID:       "char-1",
		GenerationTaskID:  taskID,
		PackageName:       "测试包v2",
		DefaultAction:     "idle_normal",
		IncludedActions:   []string{"idle_normal"},
		CanvasWidth:       512,
		CanvasHeight:      512,
		ProcessingVersion: 1,
		SucceededActions:  []desktoppet.GenerationTaskAction{action1},
	}

	result, err := p.BuildPackage(req)
	if err != nil {
		t.Fatalf("BuildPackage 失败: %v", err)
	}
	if result.Package.Version != 2 {
		t.Fatalf("Version = %d, 期望 2", result.Package.Version)
	}
}

func TestVerifyActionIntegrity_ImageSize(t *testing.T) {
	dir := t.TempDir()

	actionJSON := BuildActionJSON("idle_normal", "待机", 1, 10, DefaultFeetCenterAnchor, "loop")
	actionData, _ := json.MarshalIndent(actionJSON, "", "  ")
	writeFileBytes(t, dir, "actions/idle_normal/action.json", actionData)

	framesDir := filepath.Join(dir, "actions", "idle_normal", "frames")
	if err := os.MkdirAll(framesDir, 0755); err != nil {
		t.Fatalf("mkdir frames: %v", err)
	}

	writeValidatorPNG(t, dir, "actions/idle_normal/frames/frame-0001.png", 256, 256)

	manifest := BuildManifest("pkg-test", "测试", "char-1", "task-1", 1, 512, 512, "idle_normal",
		[]ManifestAction{BuildManifestAction("idle_normal", "待机")})
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	writeFileBytes(t, dir, "manifest.json", manifestData)
	writeValidatorPNG(t, dir, "preview.png", 8, 8)

	p := NewPackager(nil, dir)
	err := p.VerifyPackageIntegrity(dir, 1)
	assertPackageErrorCode(t, err, ErrCodePackageFileMissing)

	writeValidatorPNG(t, dir, "actions/idle_normal/frames/frame-0001.png", 512, 512)
	if err := p.VerifyPackageIntegrity(dir, 1); err != nil {
		t.Fatalf("尺寸修正后 VerifyPackageIntegrity 应通过, 实际: %v", err)
	}
}

func TestVerifyActionIntegrity_ImageUndecodable(t *testing.T) {
	dir := t.TempDir()

	actionJSON := BuildActionJSON("idle_normal", "待机", 1, 10, DefaultFeetCenterAnchor, "loop")
	actionData, _ := json.MarshalIndent(actionJSON, "", "  ")
	writeFileBytes(t, dir, "actions/idle_normal/action.json", actionData)

	framesDir := filepath.Join(dir, "actions", "idle_normal", "frames")
	if err := os.MkdirAll(framesDir, 0755); err != nil {
		t.Fatalf("mkdir frames: %v", err)
	}

	writeFileBytes(t, dir, "actions/idle_normal/frames/frame-0001.png", []byte("not a valid png"))

	manifest := BuildManifest("pkg-test", "测试", "char-1", "task-1", 1, 512, 512, "idle_normal",
		[]ManifestAction{BuildManifestAction("idle_normal", "待机")})
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	writeFileBytes(t, dir, "manifest.json", manifestData)
	writeValidatorPNG(t, dir, "preview.png", 8, 8)

	p := NewPackager(nil, dir)
	err := p.VerifyPackageIntegrity(dir, 1)
	assertPackageErrorCode(t, err, ErrCodePackageFileMissing)
}
