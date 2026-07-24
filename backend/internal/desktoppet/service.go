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
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/config"
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
}

type service struct {
	repo Repository
	db   *gorm.DB
}

func NewService(repo Repository, db *gorm.DB) Service {
	return &service{repo: repo, db: db}
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

	actionResponses := make([]TaskActionResponse, 0, len(actions))
	for _, a := range actions {
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
	}, nil
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
