// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/desktoppet"
	"gorm.io/gorm"
)

func writeValidatorPNG(t *testing.T, dataDir, relPath string, width, height int) string {
	t.Helper()
	absPath := filepath.Join(dataDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		t.Fatalf("mkdir for %s: %v", absPath, err)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	if err := os.WriteFile(absPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("write file %s: %v", absPath, err)
	}
	return absPath
}

func writeFileBytes(t *testing.T, dataDir, relPath string, content []byte) string {
	t.Helper()
	absPath := filepath.Join(dataDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		t.Fatalf("mkdir for %s: %v", absPath, err)
	}
	if err := os.WriteFile(absPath, content, 0644); err != nil {
		t.Fatalf("write file %s: %v", absPath, err)
	}
	return absPath
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func assertValidationErrorCode(t *testing.T, err error, wantCode string) {
	t.Helper()
	if err == nil {
		t.Fatalf("期望错误但得到 nil")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("期望 *ValidationError，实际类型 %T: %v", err, err)
	}
	if ve.Code != wantCode {
		t.Fatalf("Code = %s, 期望 %s", ve.Code, wantCode)
	}
}

func seedValidatorTask(t *testing.T, db *gorm.DB, taskID, userID, status string) {
	t.Helper()
	if err := db.Create(&desktoppet.GenerationTask{
		ID:     taskID,
		UserID: userID,
		Name:   "测试任务",
		Status: status,
	}).Error; err != nil {
		t.Fatalf("create generation task: %v", err)
	}
}

func seedValidatorAction(t *testing.T, db *gorm.DB, actionID, taskID, actionKey, status string, currentAttempt, frameCount int, specVersion string) desktoppet.GenerationTaskAction {
	t.Helper()
	action := desktoppet.GenerationTaskAction{
		ID:                    actionID,
		TaskID:                taskID,
		ActionKey:             actionKey,
		ActionNameSnapshot:    actionKey,
		Status:                status,
		CurrentAttempt:        currentAttempt,
		FrameCount:            frameCount,
		GenerationSpecVersion: specVersion,
		SortOrder:             1,
	}
	if err := db.Create(&action).Error; err != nil {
		t.Fatalf("create action %s: %v", actionID, err)
	}
	return action
}

func seedValidatorFrame(t *testing.T, db *gorm.DB, frameID, taskID, taskActionID, resultImagePath, resultHash string, frameIndex, attemptNumber int, status string) {
	t.Helper()
	if err := db.Create(&desktoppet.GenerationFrame{
		ID:              frameID,
		TaskID:          taskID,
		TaskActionID:    taskActionID,
		FrameIndex:      frameIndex,
		AttemptNumber:   attemptNumber,
		Status:          status,
		ResultImagePath: resultImagePath,
		ResultHash:      resultHash,
	}).Error; err != nil {
		t.Fatalf("create frame %s: %v", frameID, err)
	}
}

func TestValidator_ValidateSources_TaskNotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	v := NewValidator(repo, t.TempDir())

	result, err := v.ValidateSources("nonexistent", "user-1")
	if err == nil {
		t.Fatal("期望错误但得到 nil")
	}
	if result != nil {
		t.Fatalf("期望 nil result，实际 %+v", result)
	}
	assertValidationErrorCode(t, err, ErrCodeGenerationTaskNotReady)
}

func TestValidator_ValidateSources_TaskNotOwnedByUser(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	seedValidatorTask(t, db, "gt-1", "user-1", "succeeded")

	v := NewValidator(repo, t.TempDir())
	result, err := v.ValidateSources("gt-1", "user-2")
	if err == nil {
		t.Fatal("期望错误但得到 nil")
	}
	if result != nil {
		t.Fatalf("期望 nil result，实际 %+v", result)
	}
	assertValidationErrorCode(t, err, ErrCodeGenerationTaskNotReady)
}

func TestValidator_ValidateSources_TaskGenerating(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	seedValidatorTask(t, db, "gt-1", "user-1", "generating")

	v := NewValidator(repo, t.TempDir())
	result, err := v.ValidateSources("gt-1", "user-1")
	if err == nil {
		t.Fatal("期望错误但得到 nil")
	}
	if result != nil {
		t.Fatalf("期望 nil result，实际 %+v", result)
	}
	assertValidationErrorCode(t, err, ErrCodeGenerationTaskNotReady)
}

func TestValidator_ValidateSources_NoSucceededActions(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	seedValidatorTask(t, db, "gt-1", "user-1", "succeeded")
	seedValidatorAction(t, db, "gta-1", "gt-1", "idle_normal", "failed", 1, 2, "v1")

	v := NewValidator(repo, t.TempDir())
	result, err := v.ValidateSources("gt-1", "user-1")
	if err == nil {
		t.Fatal("期望错误但得到 nil")
	}
	if result != nil {
		t.Fatalf("期望 nil result，实际 %+v", result)
	}
	assertValidationErrorCode(t, err, ErrCodeNoSuccessfulActions)
}

func TestValidator_ValidateSources_Normal(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	dataDir := t.TempDir()

	seedValidatorTask(t, db, "gt-1", "user-1", "succeeded")
	seedValidatorAction(t, db, "gta-1", "gt-1", "idle_normal", "succeeded", 1, 2, "v1")

	rel1 := "desktop-pets/generation-tasks/gt-1/generated/idle_normal/attempt-1/raw/frame-0001.png"
	rel2 := "desktop-pets/generation-tasks/gt-1/generated/idle_normal/attempt-1/raw/frame-0002.png"
	abs1 := writeValidatorPNG(t, dataDir, rel1, 64, 64)
	abs2 := writeValidatorPNG(t, dataDir, rel2, 64, 64)
	hash1 := fileSHA256(t, abs1)
	hash2 := fileSHA256(t, abs2)

	seedValidatorFrame(t, db, "gf-1", "gt-1", "gta-1", rel1, hash1, 0, 1, "succeeded")
	seedValidatorFrame(t, db, "gf-2", "gt-1", "gta-1", rel2, hash2, 1, 1, "succeeded")

	v := NewValidator(repo, dataDir)
	result, err := v.ValidateSources("gt-1", "user-1")
	if err != nil {
		t.Fatalf("ValidateSources: %v", err)
	}
	if result == nil {
		t.Fatal("期望非 nil result")
	}
	if result.Task == nil || result.Task.ID != "gt-1" {
		t.Fatalf("Task 不正确: %+v", result.Task)
	}
	if len(result.SucceededActions) != 1 {
		t.Fatalf("SucceededActions 长度 = %d, 期望 1", len(result.SucceededActions))
	}
	if len(result.InvalidActions) != 0 {
		t.Fatalf("InvalidActions 长度 = %d, 期望 0", len(result.InvalidActions))
	}
	frames, ok := result.FramePaths["idle_normal"]
	if !ok {
		t.Fatal("FramePaths 缺少 idle_normal")
	}
	if len(frames) != 2 {
		t.Fatalf("帧数 = %d, 期望 2", len(frames))
	}
	if frames[0].Frame.FrameIndex != 0 || frames[1].Frame.FrameIndex != 1 {
		t.Fatalf("帧顺序不正确: %d, %d", frames[0].Frame.FrameIndex, frames[1].Frame.FrameIndex)
	}
	if !frames[0].Exists || !frames[0].Decodable || !frames[0].HashMatch {
		t.Fatalf("帧 0 校验状态不正确: %+v", frames[0])
	}
	if frames[0].Width != 64 || frames[0].Height != 64 {
		t.Fatalf("帧 0 尺寸 = %dx%d, 期望 64x64", frames[0].Width, frames[0].Height)
	}
}

func TestValidator_ResolveActiveAttempt_DefaultCurrentAttempt(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	dataDir := t.TempDir()

	seedValidatorTask(t, db, "gt-1", "user-1", "succeeded")
	action := seedValidatorAction(t, db, "gta-1", "gt-1", "idle_normal", "succeeded", 2, 2, "v1")

	rel1 := "desktop-pets/generation-tasks/gt-1/generated/idle_normal/attempt-1/raw/frame-0001.png"
	rel2 := "desktop-pets/generation-tasks/gt-1/generated/idle_normal/attempt-2/raw/frame-0001.png"
	abs1 := writeValidatorPNG(t, dataDir, rel1, 64, 64)
	abs2 := writeValidatorPNG(t, dataDir, rel2, 64, 64)
	hash1 := fileSHA256(t, abs1)
	hash2 := fileSHA256(t, abs2)

	seedValidatorFrame(t, db, "gf-a1", "gt-1", "gta-1", rel1, hash1, 0, 1, "succeeded")
	seedValidatorFrame(t, db, "gf-a2", "gt-1", "gta-1", rel2, hash2, 0, 2, "succeeded")

	v := NewValidator(repo, dataDir)
	got, err := v.ResolveActiveAttempt(action)
	if err != nil {
		t.Fatalf("ResolveActiveAttempt: %v", err)
	}
	if got != 2 {
		t.Fatalf("ResolveActiveAttempt = %d, 期望 2 (current_attempt)", got)
	}

	action.CurrentAttempt = 0
	got, err = v.ResolveActiveAttempt(action)
	if err != nil {
		t.Fatalf("ResolveActiveAttempt with 0: %v", err)
	}
	if got != 1 {
		t.Fatalf("ResolveActiveAttempt with 0 = %d, 期望 1 (默认)", got)
	}
}

func TestValidator_ResolveActiveAttempt_AttemptNotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	dataDir := t.TempDir()

	seedValidatorTask(t, db, "gt-1", "user-1", "succeeded")
	action := seedValidatorAction(t, db, "gta-1", "gt-1", "idle_normal", "succeeded", 3, 2, "v1")

	rel1 := "desktop-pets/generation-tasks/gt-1/generated/idle_normal/attempt-1/raw/frame-0001.png"
	abs1 := writeValidatorPNG(t, dataDir, rel1, 64, 64)
	seedValidatorFrame(t, db, "gf-a1", "gt-1", "gta-1", rel1, fileSHA256(t, abs1), 0, 1, "succeeded")

	v := NewValidator(repo, dataDir)
	got, err := v.ResolveActiveAttempt(action)
	if err == nil {
		t.Fatalf("期望错误但得到 %d", got)
	}
	assertValidationErrorCode(t, err, ErrCodeSourceAttemptNotFound)
}

func TestValidator_ValidateFrames_FileMissing(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	dataDir := t.TempDir()

	seedValidatorTask(t, db, "gt-1", "user-1", "succeeded")
	action := seedValidatorAction(t, db, "gta-1", "gt-1", "idle_normal", "succeeded", 1, 2, "v1")

	rel1 := "desktop-pets/generation-tasks/gt-1/generated/idle_normal/attempt-1/raw/frame-0001.png"
	seedValidatorFrame(t, db, "gf-1", "gt-1", "gta-1", rel1, "fakehash", 0, 1, "succeeded")
	seedValidatorFrame(t, db, "gf-2", "gt-1", "gta-1", rel1, "fakehash", 1, 1, "succeeded")

	v := NewValidator(repo, dataDir)
	frames, err := v.ValidateFrames(action, 1)
	if err == nil {
		t.Fatalf("期望错误但得到 frames: %+v", frames)
	}
	assertValidationErrorCode(t, err, ErrCodeSourceFrameMissing)
}

func TestValidator_ValidateFrames_Undecodable(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	dataDir := t.TempDir()

	seedValidatorTask(t, db, "gt-1", "user-1", "succeeded")
	action := seedValidatorAction(t, db, "gta-1", "gt-1", "idle_normal", "succeeded", 1, 1, "v1")

	rel1 := "desktop-pets/generation-tasks/gt-1/generated/idle_normal/attempt-1/raw/frame-0001.png"
	abs1 := writeFileBytes(t, dataDir, rel1, []byte("this is not a valid image"))
	seedValidatorFrame(t, db, "gf-1", "gt-1", "gta-1", rel1, fileSHA256(t, abs1), 0, 1, "succeeded")

	v := NewValidator(repo, dataDir)
	frames, err := v.ValidateFrames(action, 1)
	if err == nil {
		t.Fatalf("期望错误但得到 frames: %+v", frames)
	}
	assertValidationErrorCode(t, err, ErrCodeSourceFrameInvalid)
}

func TestValidator_ValidateFrames_HashMismatch(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	dataDir := t.TempDir()

	seedValidatorTask(t, db, "gt-1", "user-1", "succeeded")
	action := seedValidatorAction(t, db, "gta-1", "gt-1", "idle_normal", "succeeded", 1, 1, "v1")

	rel1 := "desktop-pets/generation-tasks/gt-1/generated/idle_normal/attempt-1/raw/frame-0001.png"
	writeValidatorPNG(t, dataDir, rel1, 64, 64)
	seedValidatorFrame(t, db, "gf-1", "gt-1", "gta-1", rel1, "0000000000000000000000000000000000000000000000000000000000000000", 0, 1, "succeeded")

	v := NewValidator(repo, dataDir)
	frames, err := v.ValidateFrames(action, 1)
	if err == nil {
		t.Fatalf("期望错误但得到 frames: %+v", frames)
	}
	assertValidationErrorCode(t, err, ErrCodeSourceFrameInvalid)
}

func TestValidator_ValidateFrames_FrameIndexNotContinuous(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	dataDir := t.TempDir()

	seedValidatorTask(t, db, "gt-1", "user-1", "succeeded")
	action := seedValidatorAction(t, db, "gta-1", "gt-1", "idle_normal", "succeeded", 1, 3, "v1")

	rel1 := "desktop-pets/generation-tasks/gt-1/generated/idle_normal/attempt-1/raw/frame-0001.png"
	rel3 := "desktop-pets/generation-tasks/gt-1/generated/idle_normal/attempt-1/raw/frame-0003.png"
	abs1 := writeValidatorPNG(t, dataDir, rel1, 64, 64)
	abs3 := writeValidatorPNG(t, dataDir, rel3, 64, 64)
	seedValidatorFrame(t, db, "gf-1", "gt-1", "gta-1", rel1, fileSHA256(t, abs1), 0, 1, "succeeded")
	seedValidatorFrame(t, db, "gf-3", "gt-1", "gta-1", rel3, fileSHA256(t, abs3), 2, 1, "succeeded")

	v := NewValidator(repo, dataDir)
	frames, err := v.ValidateFrames(action, 1)
	if err == nil {
		t.Fatalf("期望错误但得到 frames: %+v", frames)
	}
	assertValidationErrorCode(t, err, ErrCodeSourceFrameInvalid)
}

func TestValidator_ValidateFrames_SpecMissing(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	dataDir := t.TempDir()

	seedValidatorTask(t, db, "gt-1", "user-1", "succeeded")
	action := seedValidatorAction(t, db, "gta-1", "gt-1", "idle_normal", "succeeded", 1, 0, "")

	rel1 := "desktop-pets/generation-tasks/gt-1/generated/idle_normal/attempt-1/raw/frame-0001.png"
	abs1 := writeValidatorPNG(t, dataDir, rel1, 64, 64)
	seedValidatorFrame(t, db, "gf-1", "gt-1", "gta-1", rel1, fileSHA256(t, abs1), 0, 1, "succeeded")

	v := NewValidator(repo, dataDir)
	frames, err := v.ValidateFrames(action, 1)
	if err == nil {
		t.Fatalf("期望错误但得到 frames: %+v", frames)
	}
	assertValidationErrorCode(t, err, ErrCodeSourceFrameInvalid)
}

func TestValidator_ValidateFrames_NoSucceededFrames(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	dataDir := t.TempDir()

	seedValidatorTask(t, db, "gt-1", "user-1", "succeeded")
	action := seedValidatorAction(t, db, "gta-1", "gt-1", "idle_normal", "succeeded", 1, 1, "v1")

	rel1 := "desktop-pets/generation-tasks/gt-1/generated/idle_normal/attempt-1/raw/frame-0001.png"
	abs1 := writeValidatorPNG(t, dataDir, rel1, 64, 64)
	seedValidatorFrame(t, db, "gf-1", "gt-1", "gta-1", rel1, fileSHA256(t, abs1), 0, 1, "failed")

	v := NewValidator(repo, dataDir)
	frames, err := v.ValidateFrames(action, 1)
	if err == nil {
		t.Fatalf("期望错误但得到 frames: %+v", frames)
	}
	assertValidationErrorCode(t, err, ErrCodeSourceFrameMissing)
}

func TestValidator_ValidateSources_PartialMode(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	dataDir := t.TempDir()

	seedValidatorTask(t, db, "gt-1", "user-1", "succeeded")
	seedValidatorAction(t, db, "gta-good", "gt-1", "idle_normal", "succeeded", 1, 2, "v1")
	seedValidatorAction(t, db, "gta-bad", "gt-1", "walk_left", "succeeded", 1, 1, "v1")

	relGood1 := "desktop-pets/generation-tasks/gt-1/generated/idle_normal/attempt-1/raw/frame-0001.png"
	relGood2 := "desktop-pets/generation-tasks/gt-1/generated/idle_normal/attempt-1/raw/frame-0002.png"
	absGood1 := writeValidatorPNG(t, dataDir, relGood1, 64, 64)
	absGood2 := writeValidatorPNG(t, dataDir, relGood2, 64, 64)
	seedValidatorFrame(t, db, "gf-good-1", "gt-1", "gta-good", relGood1, fileSHA256(t, absGood1), 0, 1, "succeeded")
	seedValidatorFrame(t, db, "gf-good-2", "gt-1", "gta-good", relGood2, fileSHA256(t, absGood2), 1, 1, "succeeded")

	relBad := "desktop-pets/generation-tasks/gt-1/generated/walk_left/attempt-1/raw/frame-0001.png"
	seedValidatorFrame(t, db, "gf-bad-1", "gt-1", "gta-bad", relBad, "fakehash", 0, 1, "succeeded")

	v := NewValidator(repo, dataDir)
	result, err := v.ValidateSources("gt-1", "user-1")
	if err != nil {
		t.Fatalf("部分处理模式不应返回错误: %v", err)
	}
	if result == nil {
		t.Fatal("期望非 nil result")
	}
	if len(result.SucceededActions) != 2 {
		t.Fatalf("SucceededActions 长度 = %d, 期望 2", len(result.SucceededActions))
	}
	if len(result.InvalidActions) != 1 {
		t.Fatalf("InvalidActions 长度 = %d, 期望 1", len(result.InvalidActions))
	}
	if result.InvalidActions[0].Action.ActionKey != "walk_left" {
		t.Fatalf("InvalidActions[0].ActionKey = %s, 期望 walk_left", result.InvalidActions[0].Action.ActionKey)
	}
	if result.InvalidActions[0].ErrorCode != ErrCodeSourceFrameMissing {
		t.Fatalf("InvalidActions[0].ErrorCode = %s, 期望 %s", result.InvalidActions[0].ErrorCode, ErrCodeSourceFrameMissing)
	}
	goodFrames, ok := result.FramePaths["idle_normal"]
	if !ok {
		t.Fatal("FramePaths 缺少 idle_normal")
	}
	if len(goodFrames) != 2 {
		t.Fatalf("idle_normal 帧数 = %d, 期望 2", len(goodFrames))
	}
	if _, ok := result.FramePaths["walk_left"]; ok {
		t.Fatal("FramePaths 不应包含 walk_left")
	}
}

func TestValidator_ValidateSources_AllActionsInvalid(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	dataDir := t.TempDir()

	seedValidatorTask(t, db, "gt-1", "user-1", "succeeded")
	seedValidatorAction(t, db, "gta-bad", "gt-1", "idle_normal", "succeeded", 1, 1, "v1")

	relBad := "desktop-pets/generation-tasks/gt-1/generated/idle_normal/attempt-1/raw/frame-0001.png"
	seedValidatorFrame(t, db, "gf-bad-1", "gt-1", "gta-bad", relBad, "fakehash", 0, 1, "succeeded")

	v := NewValidator(repo, dataDir)
	result, err := v.ValidateSources("gt-1", "user-1")
	if err == nil {
		t.Fatalf("所有动作无效时应返回错误，result=%+v", result)
	}
	assertValidationErrorCode(t, err, ErrCodeSourceFrameMissing)
}
