// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	_ "golang.org/x/image/webp"

	"github.com/u-ai/backend/internal/desktoppet"
	"gorm.io/gorm"
)

const (
	ErrCodeGenerationTaskNotReady = "GENERATION_TASK_NOT_READY"
	ErrCodeNoSuccessfulActions    = "NO_SUCCESSFUL_ACTIONS"
	ErrCodeSourceAttemptNotFound  = "SOURCE_ATTEMPT_NOT_FOUND"
	ErrCodeSourceFrameMissing     = "SOURCE_FRAME_MISSING"
	ErrCodeSourceFrameInvalid     = "SOURCE_FRAME_INVALID"
)

type ValidationError struct {
	Code    string
	Message string
	Err     error
}

func (e *ValidationError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *ValidationError) Unwrap() error { return e.Err }

type SourceValidationResult struct {
	Task             *desktoppet.GenerationTask
	SucceededActions []desktoppet.GenerationTaskAction
	InvalidActions   []InvalidActionInfo
	FramePaths       map[string][]FrameSourceInfo
}

type InvalidActionInfo struct {
	Action    desktoppet.GenerationTaskAction
	Reason    string
	ErrorCode string
}

type FrameSourceInfo struct {
	Frame     desktoppet.GenerationFrame
	AbsPath   string
	Exists    bool
	Decodable bool
	Width     int
	Height    int
	HashMatch bool
}

type Validator struct {
	repo    Repository
	dataDir string
}

func NewValidator(repo Repository, dataDir string) *Validator {
	return &Validator{repo: repo, dataDir: dataDir}
}

func (v *Validator) ValidateSources(generationTaskID, userID string) (*SourceValidationResult, error) {
	task, err := v.repo.GetGenerationTask(generationTaskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &ValidationError{
				Code:    ErrCodeGenerationTaskNotReady,
				Message: "生成任务不存在",
				Err:     err,
			}
		}
		return nil, err
	}

	if task.UserID != userID {
		return nil, &ValidationError{
			Code:    ErrCodeGenerationTaskNotReady,
			Message: "生成任务不属于当前用户",
		}
	}

	if task.Status == "generating" {
		return nil, &ValidationError{
			Code:    ErrCodeGenerationTaskNotReady,
			Message: "生成任务仍在生成中",
		}
	}

	succeededActions, err := v.repo.ListSucceededActions(generationTaskID)
	if err != nil {
		return nil, err
	}
	if len(succeededActions) == 0 {
		return nil, &ValidationError{
			Code:    ErrCodeNoSuccessfulActions,
			Message: "生成任务没有成功的动作",
		}
	}

	result := &SourceValidationResult{
		Task:             task,
		SucceededActions: succeededActions,
		InvalidActions:   []InvalidActionInfo{},
		FramePaths:       map[string][]FrameSourceInfo{},
	}

	for _, action := range succeededActions {
		attempt, err := v.ResolveActiveAttempt(action)
		if err != nil {
			result.InvalidActions = append(result.InvalidActions, InvalidActionInfo{
				Action:    action,
				Reason:    err.Error(),
				ErrorCode: extractErrorCode(err),
			})
			continue
		}

		frames, err := v.ValidateFrames(action, attempt)
		if err != nil {
			result.InvalidActions = append(result.InvalidActions, InvalidActionInfo{
				Action:    action,
				Reason:    err.Error(),
				ErrorCode: extractErrorCode(err),
			})
			continue
		}

		result.FramePaths[action.ActionKey] = frames
	}

	if len(result.FramePaths) == 0 {
		return nil, &ValidationError{
			Code:    ErrCodeSourceFrameMissing,
			Message: "所有成功动作的原始帧均无效",
		}
	}

	return result, nil
}

func (v *Validator) ResolveActiveAttempt(action desktoppet.GenerationTaskAction) (int, error) {
	attempt := action.CurrentAttempt
	if attempt <= 0 {
		attempt = 1
	}

	frames, err := v.repo.ListFramesByAction(action.ID)
	if err != nil {
		return 0, err
	}

	hasSucceeded := false
	for _, frame := range frames {
		if frame.GenerationAttempt == attempt && frame.Status == "succeeded" {
			hasSucceeded = true
			break
		}
	}

	if !hasSucceeded {
		return 0, &ValidationError{
			Code:    ErrCodeSourceAttemptNotFound,
			Message: fmt.Sprintf("动作 %s 的 attempt %d 没有成功帧", action.ActionKey, attempt),
		}
	}

	return attempt, nil
}

func (v *Validator) ValidateFrames(action desktoppet.GenerationTaskAction, attemptNumber int) ([]FrameSourceInfo, error) {
	if action.GenerationSpecVersion == "" || action.FrameCount <= 0 {
		return nil, &ValidationError{
			Code:    ErrCodeSourceFrameInvalid,
			Message: fmt.Sprintf("动作 %s 的规格或帧数快照缺失", action.ActionKey),
		}
	}

	allFrames, err := v.repo.ListFramesByAction(action.ID)
	if err != nil {
		return nil, err
	}

	var frames []desktoppet.GenerationFrame
	for _, frame := range allFrames {
		if frame.GenerationAttempt == attemptNumber && frame.Status == "succeeded" {
			frames = append(frames, frame)
		}
	}

	if len(frames) == 0 {
		return nil, &ValidationError{
			Code:    ErrCodeSourceFrameMissing,
			Message: fmt.Sprintf("动作 %s 的 attempt %d 没有成功帧", action.ActionKey, attemptNumber),
		}
	}

	sort.Slice(frames, func(i, j int) bool {
		return frames[i].FrameIndex < frames[j].FrameIndex
	})

	for i, frame := range frames {
		if frame.FrameIndex != i {
			return nil, &ValidationError{
				Code:    ErrCodeSourceFrameInvalid,
				Message: fmt.Sprintf("动作 %s 的帧索引不连续，期望 %d 实际 %d", action.ActionKey, i, frame.FrameIndex),
			}
		}
	}

	results := make([]FrameSourceInfo, 0, len(frames))
	for _, frame := range frames {
		info := FrameSourceInfo{Frame: frame}

		if frame.ResultImagePath == "" {
			results = append(results, info)
			return results, &ValidationError{
				Code:    ErrCodeSourceFrameMissing,
				Message: fmt.Sprintf("帧 %s 的结果图片路径为空", frame.ID),
			}
		}

		absPath := filepath.Join(v.dataDir, frame.ResultImagePath)
		info.AbsPath = absPath

		stat, err := os.Stat(absPath)
		if err != nil || stat.IsDir() {
			results = append(results, info)
			return results, &ValidationError{
				Code:    ErrCodeSourceFrameMissing,
				Message: fmt.Sprintf("帧 %s 的结果文件不存在: %s", frame.ID, absPath),
				Err:     err,
			}
		}
		info.Exists = true

		content, err := os.ReadFile(absPath)
		if err != nil {
			results = append(results, info)
			return results, &ValidationError{
				Code:    ErrCodeSourceFrameInvalid,
				Message: fmt.Sprintf("读取帧 %s 的文件失败: %v", frame.ID, err),
			}
		}

		cfg, _, err := image.DecodeConfig(bytes.NewReader(content))
		if err != nil {
			results = append(results, info)
			return results, &ValidationError{
				Code:    ErrCodeSourceFrameInvalid,
				Message: fmt.Sprintf("解码帧 %s 的图片失败: %v", frame.ID, err),
			}
		}
		info.Decodable = true
		info.Width = cfg.Width
		info.Height = cfg.Height

		if cfg.Width <= 0 || cfg.Height <= 0 {
			results = append(results, info)
			return results, &ValidationError{
				Code:    ErrCodeSourceFrameInvalid,
				Message: fmt.Sprintf("帧 %s 的图片尺寸无效: %dx%d", frame.ID, cfg.Width, cfg.Height),
			}
		}

		ext := strings.ToLower(filepath.Ext(absPath))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
			results = append(results, info)
			return results, &ValidationError{
				Code:    ErrCodeSourceFrameInvalid,
				Message: fmt.Sprintf("帧 %s 的图片格式不支持: %s", frame.ID, ext),
			}
		}

		sum := sha256.Sum256(content)
		hash := hex.EncodeToString(sum[:])
		info.HashMatch = hash == frame.ResultHash
		if !info.HashMatch {
			results = append(results, info)
			return results, &ValidationError{
				Code:    ErrCodeSourceFrameInvalid,
				Message: fmt.Sprintf("帧 %s 的 Hash 不匹配", frame.ID),
			}
		}

		results = append(results, info)
	}

	return results, nil
}

func extractErrorCode(err error) string {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve.Code
	}
	return ""
}
