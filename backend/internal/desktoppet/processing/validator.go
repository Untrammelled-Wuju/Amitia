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

	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/internal/desktoppet/contracts"
	_ "golang.org/x/image/webp"
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
	SourceAttempts   map[string]int
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

// ValidateProcessingSources is the authoritative source admission gate used by
// processing task creation and the processing worker. It supports both the
// legacy per-frame generation path and Generation Plan v1 artifact modes while
// keeping ownership, task readiness and storage errors fail-closed.
//
// A V2 artifact is accepted as a compatibility source only when the action is
// explicitly in a non-legacy generation mode and has a succeeded generation
// attempt with a persisted primary artifact whose on-disk bytes pass path,
// hash and image-decode validation. This prevents a generic validation error
// from degrading into the old unsafe "list succeeded actions and continue"
// behavior.
func (v *Validator) ValidateProcessingSources(generationTaskID, userID string) (*SourceValidationResult, error) {
	task, succeededActions, err := v.validateTaskAndSucceededActions(generationTaskID, userID)
	if err != nil {
		return nil, err
	}

	result := &SourceValidationResult{
		Task:             task,
		SucceededActions: succeededActions,
		InvalidActions:   []InvalidActionInfo{},
		FramePaths:       map[string][]FrameSourceInfo{},
		SourceAttempts:   map[string]int{},
	}

	for _, action := range succeededActions {
		mode := strings.TrimSpace(action.GenerationMode)
		if mode != "" && mode != "legacy_frame" {
			attempt, sourceErr := v.validatePersistedArtifactSource(action)
			if sourceErr == nil {
				// FramePaths is intentionally present with an empty slice: callers use
				// key presence as the admission signal, while the mode-aware source
				// resolver expands the artifact into logical frames later.
				result.FramePaths[action.ActionKey] = []FrameSourceInfo{}
				result.SourceAttempts[action.ActionKey] = attempt
				continue
			}
			var validationErr *ValidationError
			if errors.As(sourceErr, &validationErr) {
				result.InvalidActions = append(result.InvalidActions, InvalidActionInfo{
					Action:    action,
					Reason:    validationErr.Error(),
					ErrorCode: validationErr.Code,
				})
				continue
			}
			if errors.Is(sourceErr, gorm.ErrRecordNotFound) {
				result.InvalidActions = append(result.InvalidActions, InvalidActionInfo{
					Action:    action,
					Reason:    fmt.Sprintf("动作 %s 缺少已持久化的 V2 主制品", action.ActionKey),
					ErrorCode: ErrCodeSourceAttemptNotFound,
				})
				continue
			}
			return nil, sourceErr
		}

		attempt, sourceErr := v.ResolveActiveAttempt(action)
		if sourceErr == nil {
			var frames []FrameSourceInfo
			frames, sourceErr = v.ValidateFrames(action, attempt)
			if sourceErr == nil {
				result.FramePaths[action.ActionKey] = frames
				result.SourceAttempts[action.ActionKey] = attempt
				continue
			}
		}
		var validationErr *ValidationError
		if errors.As(sourceErr, &validationErr) {
			result.InvalidActions = append(result.InvalidActions, InvalidActionInfo{
				Action:    action,
				Reason:    validationErr.Error(),
				ErrorCode: validationErr.Code,
			})
			continue
		}
		return nil, sourceErr
	}

	if len(result.FramePaths) == 0 {
		return nil, &ValidationError{
			Code:    ErrCodeSourceFrameMissing,
			Message: "所有成功动作均没有可验证的 legacy 帧或 V2 持久化源制品",
		}
	}

	return result, nil
}

func (v *Validator) ValidateSources(generationTaskID, userID string) (*SourceValidationResult, error) {
	task, succeededActions, err := v.validateTaskAndSucceededActions(generationTaskID, userID)
	if err != nil {
		return nil, err
	}

	result := &SourceValidationResult{
		Task:             task,
		SucceededActions: succeededActions,
		InvalidActions:   []InvalidActionInfo{},
		FramePaths:       map[string][]FrameSourceInfo{},
		SourceAttempts:   map[string]int{},
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
		result.SourceAttempts[action.ActionKey] = attempt
	}

	if len(result.FramePaths) == 0 {
		return nil, &ValidationError{
			Code:    ErrCodeSourceFrameMissing,
			Message: "所有成功动作的原始帧均无效",
		}
	}

	return result, nil
}

func (v *Validator) validateTaskAndSucceededActions(generationTaskID, userID string) (*desktoppet.GenerationTask, []desktoppet.GenerationTaskAction, error) {
	task, err := v.repo.GetGenerationTask(generationTaskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, &ValidationError{
				Code:    ErrCodeGenerationTaskNotReady,
				Message: "生成任务不存在",
				Err:     err,
			}
		}
		return nil, nil, err
	}

	if task.UserID != userID {
		return nil, nil, &ValidationError{
			Code:    ErrCodeGenerationTaskNotReady,
			Message: "生成任务不属于当前用户",
		}
	}

	status := contracts.LifecycleStatus(task.Status)
	if status != contracts.StatusSucceeded && status != contracts.StatusPartiallySucceeded {
		return nil, nil, &ValidationError{
			Code:    ErrCodeGenerationTaskNotReady,
			Message: fmt.Sprintf("生成任务尚未进入可处理终态: %s", task.Status),
		}
	}

	succeededActions, err := v.repo.ListSucceededActions(generationTaskID)
	if err != nil {
		return nil, nil, err
	}
	if len(succeededActions) == 0 {
		return nil, nil, &ValidationError{
			Code:    ErrCodeNoSuccessfulActions,
			Message: "生成任务没有成功的动作",
		}
	}

	return task, succeededActions, nil
}

func (v *Validator) validatePersistedArtifactSource(action desktoppet.GenerationTaskAction) (int, error) {
	mode := strings.TrimSpace(action.GenerationMode)
	if mode == "" || mode == "legacy_frame" {
		return 0, gorm.ErrRecordNotFound
	}

	type attemptRow struct {
		ID            string `gorm:"column:id"`
		AttemptNumber int    `gorm:"column:attempt_number"`
		Mode          string `gorm:"column:mode"`
	}
	var attempt attemptRow
	query := v.repo.DB().Table("desktop_pet_action_generation_attempts").
		Select("id, attempt_number, mode").
		Where("task_action_id = ? AND status = ?", action.ID, "succeeded")
	if strings.TrimSpace(action.ActiveAttemptID) != "" {
		query = query.Where("id = ?", action.ActiveAttemptID)
	} else {
		query = query.Order("attempt_number DESC")
	}
	if err := query.First(&attempt).Error; err != nil {
		return 0, err
	}
	if strings.TrimSpace(attempt.Mode) == "" || attempt.Mode == "legacy_frame" {
		return 0, gorm.ErrRecordNotFound
	}
	if attempt.Mode != mode {
		return 0, &ValidationError{
			Code:    ErrCodeSourceFrameInvalid,
			Message: fmt.Sprintf("动作 %s 的生成模式与活跃 attempt 不一致: action=%s attempt=%s", action.ActionKey, mode, attempt.Mode),
		}
	}

	type artifactRow struct {
		ID           string `gorm:"column:id"`
		Status       string `gorm:"column:status"`
		RelativePath string `gorm:"column:relative_path"`
		Hash         string `gorm:"column:hash"`
		Width        int    `gorm:"column:width"`
		Height       int    `gorm:"column:height"`
	}
	var artifact artifactRow
	if err := v.repo.DB().Table("desktop_pet_generation_artifacts").
		Select("id, status, relative_path, hash, width, height").
		Where("attempt_id = ? AND task_action_id = ? AND is_primary = 1", attempt.ID, action.ID).
		Order("candidate_index ASC, segment_index ASC").
		First(&artifact).Error; err != nil {
		return 0, err
	}
	if !isGenerationArtifactDurable(artifact.Status) {
		return 0, &ValidationError{
			Code:    ErrCodeSourceFrameInvalid,
			Message: fmt.Sprintf("动作 %s 的主制品尚未达到可持久读取状态: %s", action.ActionKey, artifact.Status),
		}
	}
	if strings.TrimSpace(artifact.RelativePath) == "" {
		return 0, &ValidationError{
			Code:    ErrCodeSourceFrameMissing,
			Message: fmt.Sprintf("动作 %s 的主制品路径为空", action.ActionKey),
		}
	}

	absPath, err := resolveValidatedRelativePath(v.dataDir, artifact.RelativePath)
	if err != nil {
		return 0, &ValidationError{Code: ErrCodeSourceFrameInvalid, Message: "V2 主制品路径非法", Err: err}
	}
	content, err := os.ReadFile(absPath)
	if err != nil {
		return 0, &ValidationError{Code: ErrCodeSourceFrameMissing, Message: fmt.Sprintf("读取动作 %s 的 V2 主制品失败", action.ActionKey), Err: err}
	}
	if artifact.Hash != "" {
		sum := sha256.Sum256(content)
		actual := hex.EncodeToString(sum[:])
		if !hashEquals(artifact.Hash, actual) {
			return 0, &ValidationError{
				Code:    ErrCodeSourceFrameInvalid,
				Message: fmt.Sprintf("动作 %s 的 V2 主制品哈希不匹配", action.ActionKey),
			}
		}
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, &ValidationError{Code: ErrCodeSourceFrameInvalid, Message: fmt.Sprintf("动作 %s 的 V2 主制品不是可解码图片", action.ActionKey), Err: err}
	}
	if artifact.Width > 0 && cfg.Width != artifact.Width {
		return 0, &ValidationError{Code: ErrCodeSourceFrameInvalid, Message: fmt.Sprintf("动作 %s 的 V2 主制品宽度不匹配", action.ActionKey)}
	}
	if artifact.Height > 0 && cfg.Height != artifact.Height {
		return 0, &ValidationError{Code: ErrCodeSourceFrameInvalid, Message: fmt.Sprintf("动作 %s 的 V2 主制品高度不匹配", action.ActionKey)}
	}

	if attempt.AttemptNumber <= 0 {
		attempt.AttemptNumber = action.ActiveAttemptNumber
	}
	if attempt.AttemptNumber <= 0 {
		attempt.AttemptNumber = 1
	}
	return attempt.AttemptNumber, nil
}

func isGenerationArtifactDurable(status string) bool {
	switch strings.TrimSpace(status) {
	case "persisted", "saved", "verified":
		return true
	default:
		return false
	}
}

func resolveValidatedRelativePath(root, relative string) (string, error) {
	clean := filepath.Clean(relative)
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes root: %s", relative)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(filepath.Join(absRoot, clean))
	if err != nil {
		return "", err
	}
	if err := ensurePathWithinRoot(absRoot, absPath, relative); err != nil {
		return "", err
	}

	// Both legacy frame and V2 artifact admission eventually read an existing
	// file. Resolve symlinks before the read so an on-disk symlink cannot turn a
	// syntactically safe relative path into an escape outside DataDir.
	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", fmt.Errorf("resolve storage root: %w", err)
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", err
	}
	if err := ensurePathWithinRoot(resolvedRoot, resolvedPath, relative); err != nil {
		return "", err
	}
	return resolvedPath, nil
}

func ensurePathWithinRoot(root, target, original string) error {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes root: %s", original)
	}
	return nil
}

func hashEquals(expected, actual string) bool {
	expected = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(expected), "sha256:"))
	actual = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(actual), "sha256:"))
	return expected != "" && expected == actual
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

		absPath, pathErr := resolveValidatedRelativePath(v.dataDir, frame.ResultImagePath)
		if pathErr != nil {
			results = append(results, info)
			return results, &ValidationError{
				Code:    ErrCodeSourceFrameInvalid,
				Message: fmt.Sprintf("帧 %s 的结果图片路径非法", frame.ID),
				Err:     pathErr,
			}
		}
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
