// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

import (
	"context"
	"errors"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/internal/imageprovider/seedream"
	"github.com/u-ai/backend/pkg/comment/response"
	"gorm.io/gorm"
)

type Service interface {
	GetActionDefinitions() (*ActionDefinitionsResponse, error)
	CreateTask(ctx context.Context, userID string, characterID string, modelConfigID int, name string, prompt string, negativePrompt string, outputWidth int, outputHeight int, selectedActionKeys []string, fileHeader *multipart.FileHeader) (*TaskSummaryResponse, error)
	GetTask(taskID string) (*TaskDetailResponse, error)
	ListTasks(characterID, status string, page, pageSize int) (*TaskListResponse, error)
	DeleteTask(taskID string) error
	GetTaskSourceImage(taskID string) (fullPath string, mimeType string, err error)
	StartTask(taskID string) (*TaskSummaryResponse, error)
	CancelTask(taskID string) error
	RetryAction(taskID, actionKey string) (*TaskActionResponse, error)
	GetFrameImage(taskID, actionKey string, frameIndex int) (fullPath string, mimeType string, err error)
}

type service struct {
	repo             Repository
	db               *gorm.DB
	providerRegistry *imageprovider.Registry
}

func NewService(repo Repository, db *gorm.DB) Service {
	registry := NewProviderRegistry()
	return &service{repo: repo, db: db, providerRegistry: registry}
}

func NewProviderRegistry() *imageprovider.Registry {
	registry := imageprovider.NewRegistry()
	seedream.Register(registry)
	return registry
}

func (s *service) GetActionDefinitions() (*ActionDefinitionsResponse, error) {
	actions, err := s.repo.ListEnabledActions()
	if err != nil {
		return nil, err
	}

	categoryOrder := []string{}
	categoryMap := map[string]*ActionCategoryResponse{}
	categorySort := map[string]int{}
	allActionKeys := make([]string, 0, len(actions))

	for _, def := range actions {
		allActionKeys = append(allActionKeys, def.ActionKey)
		cat, exists := categoryMap[def.CategoryKey]
		if !exists {
			cat = &ActionCategoryResponse{
				Key:       def.CategoryKey,
				Name:      def.CategoryName,
				SortOrder: def.SortOrder,
				Actions:   []ActionItemResponse{},
			}
			categoryMap[def.CategoryKey] = cat
			categoryOrder = append(categoryOrder, def.CategoryKey)
			categorySort[def.CategoryKey] = def.SortOrder
		}
		if def.SortOrder < categorySort[def.CategoryKey] {
			categorySort[def.CategoryKey] = def.SortOrder
			cat.SortOrder = def.SortOrder
		}
		cat.Actions = append(cat.Actions, ActionItemResponse{
			ID:                       def.ID,
			Key:                      def.ActionKey,
			Name:                     def.Name,
			Description:              def.Description,
			SupportsDefaultIdle:      def.SupportsDefaultIdle == 1,
			Recommended:              def.Recommended == 1,
			DefaultFrameCount:        def.DefaultFrameCount,
			EstimatedGenerationCount: def.EstimatedGenerationCount,
			DefinitionVersion:        def.DefinitionVersion,
		})
	}

	categories := make([]ActionCategoryResponse, 0, len(categoryOrder))
	for _, key := range categoryOrder {
		categories = append(categories, *categoryMap[key])
	}
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].SortOrder < categories[j].SortOrder
	})

	presets := buildPresets(allActionKeys)

	return &ActionDefinitionsResponse{
		Categories: categories,
		Presets:    presets,
	}, nil
}

func (s *service) CreateTask(ctx context.Context, userID string, characterID string, modelConfigID int, name string, prompt string, negativePrompt string, outputWidth int, outputHeight int, selectedActionKeys []string, fileHeader *multipart.FileHeader) (*TaskSummaryResponse, error) {
	if characterID == "" {
		return nil, NewBusinessError(response.NotFound, ErrCodeCharacterNotFound, "角色不存在")
	}
	character, err := s.repo.FindCharacterByID(characterID)
	if err != nil || character == nil {
		return nil, NewBusinessError(response.NotFound, ErrCodeCharacterNotFound, "角色不存在")
	}

	if name == "" {
		return nil, NewBusinessError(response.BusinessError, ErrCodeDesktopPetNameRequired, "桌宠名称不能为空")
	}

	cfg, err := s.repo.GetImageGenConfigByID(modelConfigID)
	if err != nil || cfg == nil {
		return nil, NewBusinessError(response.NotFound, ErrCodeImageModelNotFound, "生图模型配置不存在")
	}
	if cfg.Enabled != 1 {
		return nil, NewBusinessError(response.BusinessError, ErrCodeImageModelDisabled, "生图模型已禁用")
	}

	dedupedKeys := dedupStrings(selectedActionKeys)
	if len(dedupedKeys) == 0 {
		return nil, NewBusinessError(response.BusinessError, ErrCodeActionSelectionRequired, "请至少选择一个动作")
	}

	actions, err := s.repo.GetEnabledActionsByKeys(dedupedKeys)
	if err != nil {
		return nil, err
	}
	if len(actions) < len(dedupedKeys) {
		return nil, NewBusinessError(response.BusinessError, ErrCodeActionNotFound, "存在未知或已禁用的动作")
	}

	hasDefaultIdle := false
	for _, a := range actions {
		if a.SupportsDefaultIdle == 1 {
			hasDefaultIdle = true
			break
		}
	}
	if !hasDefaultIdle {
		return nil, NewBusinessError(response.BusinessError, ErrCodeDefaultIdleActionRequired, "请至少选择一个可作为默认待机的动作")
	}

	taskID := uuid.New().String()
	taskDir := filepath.Join(config.AppCfg.Storage.DataDir, "desktop-pets", "generation-tasks", taskID)
	relativeBase := filepath.ToSlash(filepath.Join("desktop-pets", "generation-tasks", taskID))

	imageInfo, err := ValidateAndSaveReferenceImage(fileHeader, taskDir, relativeBase)
	if err != nil {
		return nil, err
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	estimatedTotal := 0
	taskActions := make([]GenerationTaskAction, 0, len(actions))
	for _, a := range actions {
		estimatedTotal += a.EstimatedGenerationCount
		taskActions = append(taskActions, GenerationTaskAction{
			ID:                        uuid.New().String(),
			TaskID:                    taskID,
			ActionDefinitionID:        a.ID,
			ActionKey:                 a.ActionKey,
			ActionNameSnapshot:        a.Name,
			ActionDescriptionSnapshot: a.Description,
			CategoryKeySnapshot:       a.CategoryKey,
			CategoryNameSnapshot:      a.CategoryName,
			DefinitionVersion:         a.DefinitionVersion,
			SupportsDefaultIdle:       a.SupportsDefaultIdle,
			SortOrder:                 a.SortOrder,
			FrameCount:                a.DefaultFrameCount,
			EstimatedGenerationCount:  a.EstimatedGenerationCount,
			Status:                    "pending",
			Progress:                  0,
			CreatedAt:                 now,
			UpdatedAt:                 now,
		})
	}

	task := &GenerationTask{
		ID:                       taskID,
		UserID:                   userID,
		CharacterID:              characterID,
		ModelConfigID:            modelConfigID,
		Name:                     name,
		SourceImagePath:          imageInfo.Path,
		SourceImageOriginalName:  imageInfo.OriginalName,
		SourceImageMimeType:      imageInfo.MimeType,
		SourceImageSize:          imageInfo.Size,
		SourceImageWidth:         imageInfo.Width,
		SourceImageHeight:        imageInfo.Height,
		SourceImageHash:          imageInfo.Hash,
		Prompt:                   prompt,
		NegativePrompt:           negativePrompt,
		OutputWidth:              outputWidth,
		OutputHeight:             outputHeight,
		Status:                   "pending",
		CurrentStage:             "queued",
		Progress:                 0,
		SelectedActionCount:      len(actions),
		EstimatedGenerationCount: estimatedTotal,
		CreatedAt:                now,
		UpdatedAt:                now,
	}

	tx := s.db.Begin()
	if tx.Error != nil {
		_ = removeAllTaskDir(taskDir)
		return nil, NewBusinessError(response.OperationFailed, ErrCodeGenerationTaskCreateFailed, "任务创建失败")
	}
	if err := s.repo.CreateTask(tx, task); err != nil {
		tx.Rollback()
		_ = removeAllTaskDir(taskDir)
		return nil, NewBusinessError(response.OperationFailed, ErrCodeGenerationTaskCreateFailed, "任务创建失败")
	}
	if err := s.repo.CreateTaskActions(tx, taskActions); err != nil {
		tx.Rollback()
		_ = removeAllTaskDir(taskDir)
		return nil, NewBusinessError(response.OperationFailed, ErrCodeGenerationTaskCreateFailed, "任务创建失败")
	}
	if err := tx.Commit().Error; err != nil {
		_ = removeAllTaskDir(taskDir)
		return nil, NewBusinessError(response.OperationFailed, ErrCodeGenerationTaskCreateFailed, "任务创建失败")
	}

	return &TaskSummaryResponse{
		ID:                       task.ID,
		Name:                     task.Name,
		CharacterID:              task.CharacterID,
		ModelConfigID:            task.ModelConfigID,
		Status:                   task.Status,
		CurrentStage:             task.CurrentStage,
		Progress:                 task.Progress,
		SelectedActionCount:      task.SelectedActionCount,
		EstimatedGenerationCount: task.EstimatedGenerationCount,
		CreatedAt:                task.CreatedAt,
	}, nil
}

func (s *service) GetTask(taskID string) (*TaskDetailResponse, error) {
	task, err := s.repo.GetTaskByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewBusinessError(response.NotFound, ErrCodeGenerationTaskNotFound, "任务不存在")
		}
		return nil, err
	}

	characterName := ""
	if ch, err := s.repo.FindCharacterByID(task.CharacterID); err == nil && ch != nil {
		characterName = ch.Name
	}

	modelName := ""
	if cfg, err := s.repo.GetImageGenConfigByID(task.ModelConfigID); err == nil && cfg != nil {
		modelName = cfg.Name
	}

	actions, err := s.repo.ListActionsByTaskID(taskID)
	if err != nil {
		return nil, err
	}

	frames, err := s.repo.ListFramesByTask(taskID)
	if err != nil {
		return nil, err
	}
	frameStats := map[string]frameStat{}
	for i := range frames {
		f := &frames[i]
		stat := frameStats[f.TaskActionID]
		stat.total++
		switch f.Status {
		case "succeeded":
			stat.succeeded++
		case "failed", "skipped":
			stat.failed++
		}
		frameStats[f.TaskActionID] = stat
	}

	actionResponses := make([]TaskActionResponse, 0, len(actions))
	var currentAction string
	succeededActionCount := 0
	failedActionCount := 0
	for _, a := range actions {
		stat := frameStats[a.ID]
		if a.Status == "running" && currentAction == "" {
			currentAction = a.ActionKey
		}
		switch a.Status {
		case "succeeded", "partially_succeeded":
			succeededActionCount++
		case "failed":
			failedActionCount++
		}
		actionResponses = append(actionResponses, TaskActionResponse{
			ID:                       a.ID,
			ActionKey:                a.ActionKey,
			ActionName:               a.ActionNameSnapshot,
			ActionDescription:        a.ActionDescriptionSnapshot,
			CategoryKey:              a.CategoryKeySnapshot,
			CategoryName:             a.CategoryNameSnapshot,
			DefinitionVersion:        a.DefinitionVersion,
			SupportsDefaultIdle:      a.SupportsDefaultIdle == 1,
			SortOrder:                a.SortOrder,
			FrameCount:               a.FrameCount,
			EstimatedGenerationCount: a.EstimatedGenerationCount,
			Status:                   a.Status,
			Progress:                 a.Progress,
			ErrorCode:                a.ErrorCode,
			ErrorMessage:             a.ErrorMessage,
			AttemptNumber:            a.AttemptNumber,
			StartedAt:                a.StartedAt,
			CompletedAt:              a.CompletedAt,
			FrameSucceeded:           stat.succeeded,
			FrameFailed:              stat.failed,
			FrameTotal:               stat.total,
		})
	}

	return &TaskDetailResponse{
		ID:                       task.ID,
		Name:                     task.Name,
		CharacterID:              task.CharacterID,
		CharacterName:            characterName,
		ModelConfigID:            task.ModelConfigID,
		ModelName:                modelName,
		Status:                   task.Status,
		CurrentStage:             task.CurrentStage,
		Progress:                 task.Progress,
		SelectedActionCount:      task.SelectedActionCount,
		EstimatedGenerationCount: task.EstimatedGenerationCount,
		ReferenceImageUrl:        "/api/desktop-pets/generation-tasks/" + task.ID + "/reference-image",
		ErrorMessage:             task.ErrorMessage,
		CreatedAt:                task.CreatedAt,
		UpdatedAt:                task.UpdatedAt,
		StartedAt:                task.StartedAt,
		CompletedAt:              task.CompletedAt,
		Actions:                  actionResponses,
		SucceededActionCount:     succeededActionCount,
		FailedActionCount:        failedActionCount,
		CurrentAction:            currentAction,
		DurationSeconds:          computeTaskDurationSeconds(task.StartedAt, task.CompletedAt),
	}, nil
}

type frameStat struct {
	total     int
	succeeded int
	failed    int
}

func computeTaskDurationSeconds(startedAt, completedAt string) int64 {
	if startedAt == "" {
		return 0
	}
	start, err := time.ParseInLocation(desktopPetTimeFormat, startedAt, time.Local)
	if err != nil {
		return 0
	}
	end := time.Now()
	if completedAt != "" {
		if parsed, pErr := time.ParseInLocation(desktopPetTimeFormat, completedAt, time.Local); pErr == nil {
			end = parsed
		}
	}
	if end.Before(start) {
		return 0
	}
	return int64(end.Sub(start).Seconds())
}

func (s *service) ListTasks(characterID, status string, page, pageSize int) (*TaskListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	tasks, total, err := s.repo.ListTasks(characterID, status, page, pageSize)
	if err != nil {
		return nil, err
	}

	items := make([]TaskListItemResponse, 0, len(tasks))
	for _, t := range tasks {
		characterName := ""
		if ch, err := s.repo.FindCharacterByID(t.CharacterID); err == nil && ch != nil {
			characterName = ch.Name
		}
		modelName := ""
		if cfg, err := s.repo.GetImageGenConfigByID(t.ModelConfigID); err == nil && cfg != nil {
			modelName = cfg.Name
		}
		items = append(items, TaskListItemResponse{
			ID:                       t.ID,
			Name:                     t.Name,
			CharacterID:              t.CharacterID,
			CharacterName:            characterName,
			ModelConfigID:            t.ModelConfigID,
			ModelName:                modelName,
			Status:                   t.Status,
			CurrentStage:             t.CurrentStage,
			Progress:                 t.Progress,
			SelectedActionCount:      t.SelectedActionCount,
			EstimatedGenerationCount: t.EstimatedGenerationCount,
			CreatedAt:                t.CreatedAt,
		})
	}

	return &TaskListResponse{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Items:    items,
	}, nil
}

func (s *service) DeleteTask(taskID string) error {
	task, err := s.repo.GetTaskByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewBusinessError(response.NotFound, ErrCodeGenerationTaskNotFound, "任务不存在")
		}
		return err
	}
	if task.Status != "pending" && task.Status != "failed" && task.Status != "cancelled" {
		return NewBusinessError(response.OperationFailed, ErrCodeTaskStatusNotDeletable, "只能删除未执行、失败或已取消的任务")
	}

	tx := s.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	if err := s.repo.DeleteActionsByTaskID(tx, taskID); err != nil {
		tx.Rollback()
		return err
	}
	if err := s.repo.DeleteTask(tx, taskID); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}

	taskDir := filepath.Join(config.AppCfg.Storage.DataDir, "desktop-pets", "generation-tasks", taskID)
	_ = removeAllTaskDir(taskDir)
	return nil
}

func (s *service) GetTaskSourceImage(taskID string) (fullPath string, mimeType string, err error) {
	task, err := s.repo.GetTaskByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", NewBusinessError(response.NotFound, ErrCodeGenerationTaskNotFound, "任务不存在")
		}
		return "", "", err
	}
	if task.SourceImagePath == "" {
		return "", "", NewBusinessError(response.NotFound, ErrCodeGenerationTaskNotFound, "参考图片不存在")
	}
	fullPath = filepath.Join(config.AppCfg.Storage.DataDir, task.SourceImagePath)
	return fullPath, task.SourceImageMimeType, nil
}

func (s *service) GetFrameImage(taskID, actionKey string, frameIndex int) (fullPath string, mimeType string, err error) {
	if frameIndex < 0 {
		return "", "", NewBusinessError(response.InvalidParams, ErrCodeFrameNotFound, "帧索引无效")
	}
	if _, err := s.repo.GetTaskByID(taskID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", NewBusinessError(response.NotFound, ErrCodeGenerationTaskNotFound, "任务不存在")
		}
		return "", "", err
	}
	actions, err := s.repo.ListActionsByTaskID(taskID)
	if err != nil {
		return "", "", err
	}
	var actionID string
	for _, a := range actions {
		if a.ActionKey == actionKey {
			actionID = a.ID
			break
		}
	}
	if actionID == "" {
		return "", "", NewBusinessError(response.NotFound, ErrCodeActionNotFound, "动作不存在")
	}
	frames, err := s.repo.ListFramesByAction(actionID)
	if err != nil {
		return "", "", err
	}
	var target *GenerationFrame
	for i := range frames {
		if frames[i].FrameIndex == frameIndex {
			target = &frames[i]
			break
		}
	}
	if target == nil {
		return "", "", NewBusinessError(response.NotFound, ErrCodeFrameNotFound, "帧不存在")
	}
	if target.Status != "succeeded" || target.ResultImagePath == "" {
		return "", "", NewBusinessError(response.NotFound, ErrCodeFrameNotFound, "帧图片不存在")
	}
	fullPath = filepath.Join(config.AppCfg.Storage.DataDir, target.ResultImagePath)
	mimeType = target.ResultMimeType
	if mimeType == "" {
		mimeType = "image/png"
	}
	return fullPath, mimeType, nil
}

func (s *service) StartTask(taskID string) (*TaskSummaryResponse, error) {
	task, err := s.repo.GetTaskByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewBusinessError(response.NotFound, ErrCodeGenerationTaskNotFound, "任务不存在")
		}
		return nil, err
	}

	if task.Status != "pending" && task.Status != "failed" && task.Status != "partially_succeeded" {
		return nil, NewBusinessError(response.BusinessError, ErrCodeGenerationStateConflict, "任务当前状态不允许开始生成")
	}

	actions, err := s.repo.ListActionsByTaskIDOrdered(taskID)
	if err != nil {
		return nil, err
	}

	pendingActions := make([]GenerationTaskAction, 0, len(actions))
	for _, a := range actions {
		if a.Status == "pending" || a.Status == "failed" || a.Status == "cancelled" {
			pendingActions = append(pendingActions, a)
		}
	}
	if len(pendingActions) == 0 {
		return nil, NewBusinessError(response.BusinessError, ErrCodeGenerationStateConflict, "任务当前状态不允许开始生成")
	}

	if _, err := s.validateModelConfigForExecution(task.ModelConfigID); err != nil {
		return nil, err
	}

	fullPath := filepath.Join(config.AppCfg.Storage.DataDir, task.SourceImagePath)
	if _, err := os.Stat(fullPath); err != nil {
		return nil, NewBusinessError(response.BusinessError, ErrCodeReferenceImageInvalid, "参考图片文件不存在")
	}

	executionID := uuid.New().String()
	now := time.Now().Format(desktopPetTimeFormat)

	taskUpdates := map[string]interface{}{
		"status":        "queued",
		"execution_id":  executionID,
		"current_stage": "queued",
		"started_at":    now,
		"error_code":    "",
		"error_message": "",
		"updated_at":    now,
	}
	if err := s.repo.UpdateTaskStatusNoTx(taskID, taskUpdates); err != nil {
		return nil, err
	}

	actionUpdates := map[string]interface{}{
		"status":        "queued",
		"progress":      0,
		"error_code":    "",
		"error_message": "",
		"started_at":    "",
		"completed_at":  "",
		"updated_at":    now,
	}
	for _, a := range pendingActions {
		if err := s.repo.UpdateActionStatusNoTx(a.ID, actionUpdates); err != nil {
			return nil, err
		}
	}

	updatedTask, err := s.repo.GetTaskByID(taskID)
	if err != nil {
		return nil, err
	}
	return s.buildTaskSummary(updatedTask), nil
}

func (s *service) CancelTask(taskID string) error {
	task, err := s.repo.GetTaskByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewBusinessError(response.NotFound, ErrCodeGenerationTaskNotFound, "任务不存在")
		}
		return err
	}

	if task.Status != "processing" && task.Status != "queued" {
		return NewBusinessError(response.BusinessError, ErrCodeGenerationStateConflict, "任务当前状态不允许取消")
	}

	now := time.Now().Format(desktopPetTimeFormat)
	if err := s.repo.SetCancelRequested(taskID, now); err != nil {
		return err
	}
	return nil
}

func (s *service) RetryAction(taskID, actionKey string) (*TaskActionResponse, error) {
	task, err := s.repo.GetTaskByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewBusinessError(response.NotFound, ErrCodeGenerationTaskNotFound, "任务不存在")
		}
		return nil, err
	}

	actions, err := s.repo.ListActionsByTaskID(taskID)
	if err != nil {
		return nil, err
	}

	var target *GenerationTaskAction
	for i := range actions {
		if actions[i].ActionKey == actionKey {
			target = &actions[i]
			break
		}
	}
	if target == nil {
		return nil, NewBusinessError(response.NotFound, ErrCodeActionNotFound, "动作不存在")
	}

	if target.Status != "failed" && target.Status != "succeeded" && target.Status != "cancelled" {
		return nil, NewBusinessError(response.BusinessError, ErrCodeGenerationStateConflict, "动作当前状态不允许重试")
	}

	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	if err := s.repo.IncrementActionAttempt(tx, target.ID); err != nil {
		tx.Rollback()
		return nil, err
	}
	now := time.Now().Format(desktopPetTimeFormat)
	actionUpdates := map[string]interface{}{
		"status":        "queued",
		"progress":      0,
		"error_code":    "",
		"error_message": "",
		"started_at":    "",
		"completed_at":  "",
		"updated_at":    now,
	}
	if err := s.repo.UpdateActionStatus(tx, target.ID, actionUpdates); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	if task.Status != "processing" {
		taskUpdates := map[string]interface{}{
			"status":        "queued",
			"current_stage": "queued",
			"updated_at":    now,
		}
		if err := s.repo.UpdateTaskStatusNoTx(taskID, taskUpdates); err != nil {
			return nil, err
		}
	}

	refreshedActions, err := s.repo.ListActionsByTaskID(taskID)
	if err != nil {
		return nil, err
	}
	for i := range refreshedActions {
		if refreshedActions[i].ActionKey == actionKey {
			return s.buildTaskActionResponse(&refreshedActions[i]), nil
		}
	}
	return nil, NewBusinessError(response.NotFound, ErrCodeActionNotFound, "动作不存在")
}

func (s *service) validateModelConfigForExecution(modelConfigID int) (*imageGenConfigView, error) {
	cfg, err := s.repo.GetImageGenConfigByID(modelConfigID)
	if err != nil || cfg == nil {
		return nil, NewBusinessError(response.BusinessError, ErrCodeImageModelUnavailable, "生图模型不可用")
	}
	if cfg.Enabled != 1 {
		return nil, NewBusinessError(response.BusinessError, ErrCodeImageModelUnavailable, "生图模型已禁用")
	}
	if s.providerRegistry == nil {
		return nil, NewBusinessError(response.BusinessError, ErrCodeImageModelUnavailable, "生图模型提供者未注册")
	}
	providerName := s.resolveProviderName(cfg)
	if _, ok := s.providerRegistry.Get(providerName); !ok {
		return nil, NewBusinessError(response.BusinessError, ErrCodeImageModelUnavailable, "生图模型提供者未注册")
	}
	return cfg, nil
}

func (s *service) resolveProviderName(cfg *imageGenConfigView) string {
	if cfg != nil && strings.Contains(strings.ToLower(cfg.ModelName), seedream.ProviderName) {
		return seedream.ProviderName
	}
	return seedream.ProviderName
}

func (s *service) buildTaskSummary(task *GenerationTask) *TaskSummaryResponse {
	return &TaskSummaryResponse{
		ID:                       task.ID,
		Name:                     task.Name,
		CharacterID:              task.CharacterID,
		ModelConfigID:            task.ModelConfigID,
		Status:                   task.Status,
		CurrentStage:             task.CurrentStage,
		Progress:                 task.Progress,
		SelectedActionCount:      task.SelectedActionCount,
		EstimatedGenerationCount: task.EstimatedGenerationCount,
		CreatedAt:                task.CreatedAt,
	}
}

func (s *service) buildTaskActionResponse(a *GenerationTaskAction) *TaskActionResponse {
	return &TaskActionResponse{
		ID:                       a.ID,
		ActionKey:                a.ActionKey,
		ActionName:               a.ActionNameSnapshot,
		ActionDescription:        a.ActionDescriptionSnapshot,
		CategoryKey:              a.CategoryKeySnapshot,
		CategoryName:             a.CategoryNameSnapshot,
		DefinitionVersion:        a.DefinitionVersion,
		SupportsDefaultIdle:      a.SupportsDefaultIdle == 1,
		SortOrder:                a.SortOrder,
		FrameCount:               a.FrameCount,
		EstimatedGenerationCount: a.EstimatedGenerationCount,
		Status:                   a.Status,
		Progress:                 a.Progress,
		ErrorCode:                a.ErrorCode,
		ErrorMessage:             a.ErrorMessage,
		AttemptNumber:            a.AttemptNumber,
		StartedAt:                a.StartedAt,
		CompletedAt:              a.CompletedAt,
	}
}

func dedupStrings(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, v := range items {
		if v == "" {
			continue
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func removeAllTaskDir(dir string) error {
	const maxAttempts = 10
	const retryDelay = 100 * time.Millisecond
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return nil
		}
		if err := os.RemoveAll(dir); err != nil {
			lastErr = err
		}
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return nil
		}
		if i < maxAttempts-1 {
			time.Sleep(retryDelay)
		}
	}
	if runtime.GOOS == "windows" {
		if err := exec.Command("cmd", "/c", "rmdir", "/s", "/q", dir).Run(); err != nil {
			if lastErr == nil {
				lastErr = err
			}
		} else {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				return nil
			}
		}
	}
	return lastErr
}
