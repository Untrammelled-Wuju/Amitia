// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/internal/desktoppet/referenceasset"
	"github.com/u-ai/backend/internal/desktoppet/security"
	"github.com/u-ai/backend/internal/desktoppet/specs"
	"github.com/u-ai/backend/internal/desktoppet/taskstate"
	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/internal/imageprovider/seedream"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/comment/response"
	"gorm.io/gorm"
)

func computeSHA256HexLocal(data string) string {
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

type Service interface {
	GetActionDefinitions() (*ActionDefinitionsResponse, error)
	CreateTask(ctx context.Context, userID string, characterID string, modelConfigID int, name string, prompt string, negativePrompt string, outputWidth int, outputHeight int, selectedActionKeys []string, fileHeader *multipart.FileHeader) (*TaskSummaryResponse, error)
	CheckTaskOwnership(taskID, userID string) error
	GetTask(taskID string) (*TaskDetailResponse, error)
	ListTasks(userID, characterID, status string, page, pageSize int) (*TaskListResponse, error)
	DeleteTask(taskID string) error
	GetTaskSourceImage(taskID string) (fullPath string, mimeType string, err error)
	GetTaskSourceImageRef(taskID string, userID string) (security.ArtifactReference, error)
	StartTask(taskID string) (*TaskSummaryResponse, error)
	CancelTask(taskID string) error
	RetryAction(taskID, actionKey string) (*TaskActionResponse, error)
	GetFrameImage(taskID, actionKey string, frameIndex int) (fullPath string, mimeType string, err error)
	GetFrameImageRef(taskID, actionKey string, frameIndex int, userID string) (security.ArtifactReference, error)
	GetTaskTransitions(taskID string, limit int) ([]taskstate.AuditRecord, error)
}

type service struct {
	repo             Repository
	db               *gorm.DB
	providerRegistry *imageprovider.Registry
	stateStore       *StateStore
	stateEngine      *taskstate.Engine
	refAssetService  referenceasset.ReferenceAssetService
}

func NewService(repo Repository, db *gorm.DB) (Service, error) {
	registry, err := NewProviderRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to build provider registry: %w", err)
	}
	stateStore := NewStateStore(db)
	refRepo := referenceasset.NewRepository(db)
	refJournalRepo := referenceasset.NewJournalRepository(db)
	refCommitter := referenceasset.NewCommitter(refRepo, refJournalRepo)
	refAssetSvc := referenceasset.NewReferenceAssetService(refRepo, refCommitter, config.AppCfg.Storage.DataDir)
	return &service{
		repo:             repo,
		db:               db,
		providerRegistry: registry,
		stateStore:       stateStore,
		stateEngine:      taskstate.NewEngine(stateStore),
		refAssetService:  refAssetSvc,
	}, nil
}

func NewProviderRegistry() (*imageprovider.Registry, error) {
	registry := imageprovider.NewRegistry()
	for alias, canonical := range map[string]string{
		"volcengine_seedream": "seedream",
		"doubao_seedream":     "seedream",
		"ark_seedream":        "seedream",
	} {
		registry.RegisterAlias(alias, canonical)
	}
	if err := seedream.Register(registry); err != nil {
		return nil, fmt.Errorf("failed to register seedream provider: %w", err)
	}
	return registry, nil
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
		cat.Actions = append(cat.Actions, buildActionItemResponse(def))
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

	providerName := imageprovider.NormalizeProviderName(cfg.ApiType)
	if providerName == "" {
		providerName = "seedream"
	}
	provider, providerOk := s.providerRegistry.Resolve(providerName)
	if !providerOk {
		return nil, NewBusinessError(response.BusinessError, ErrCodeImageModelUnavailable, "生图提供者不可用: "+providerName)
	}

	modelConfig := imageprovider.ImageModelConfig{
		Name:      cfg.Name,
		ApiType:   cfg.ApiType,
		ApiKey:    cfg.ApiKey,
		ModelName: cfg.ModelName,
		BaseUrl:   cfg.BaseUrl,
	}

	var capabilitySnapshotJSON string
	var capabilitySnapshotHash string
	if extProvider, ok := provider.(imageprovider.ExtendedProvider); ok {
		caps, capErr := extProvider.ExtendedCapabilities(ctx, modelConfig)
		if capErr == nil {
			capJSON, _ := json.Marshal(caps)
			capabilitySnapshotJSON = string(capJSON)
			capHash := computeSHA256HexLocal(string(capJSON))
			capabilitySnapshotHash = capHash
		}
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

		ta := GenerationTaskAction{
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
		}

		if cs, ok := specs.CatalogGet(a.ActionKey); ok {
			snap, err := specs.NewSnapshotter().Freeze(cs, now)
			if err == nil {
				ta.ActionSpecSchemaVersion = cs.SchemaVersion
				ta.ActionSpecVersion = cs.Identity.DefinitionVersion
				ta.ActionSpecJSON = snap.JSON
				ta.ActionSpecHash = snap.SHA256
				ta.PlaybackModeSnapshot = string(cs.Playback.Mode)
				ta.DefaultFPSSnapshot = cs.Playback.DefaultFPS
				ta.ReturnPolicySnapshot = string(cs.Playback.ReturnPolicy)
				ta.ReturnActionKeySnapshot = cs.Playback.ReturnActionKey
				if cs.Playback.Interruptible {
					ta.InterruptibleSnapshot = 1
				}
				ta.PrioritySnapshot = cs.Playback.Priority
				ta.CooldownMSSnapshot = cs.Playback.CooldownMS
				ta.MutexGroupSnapshot = cs.Playback.MutexGroup
				ta.AnchorProfileSnapshot = string(cs.Processing.AnchorProfile)
			}
		}

		taskActions = append(taskActions, ta)
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
		GenerationPlanVersion:    1,
		ProviderKeySnapshot:      providerName,
		ModelNameSnapshot:        cfg.ModelName,
		ConfigRevisionSnapshot:   cfg.UpdatedAt,
		CapabilitySnapshotJSON:   capabilitySnapshotJSON,
		CapabilitySnapshotHash:   capabilitySnapshotHash,
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

	uploadAbsPath := filepath.Join(config.AppCfg.Storage.DataDir, imageInfo.Path)
	refAsset, err := s.refAssetService.CreateForGenerationTask(ctx, tx, referenceasset.CreateReferenceAssetRequest{
		UserID:       userID,
		CharacterID:  characterID,
		TaskID:       taskID,
		UploadPath:   uploadAbsPath,
		UploadName:   imageInfo.OriginalName,
		UploadMIME:   imageInfo.MimeType,
		UploadHash:   imageInfo.Hash,
		UploadSize:   int64(imageInfo.Size),
		UploadWidth:  imageInfo.Width,
		UploadHeight: imageInfo.Height,
	})
	if err != nil {
		tx.Rollback()
		_ = removeAllTaskDir(taskDir)
		return nil, NewBusinessError(response.OperationFailed, ErrCodeGenerationTaskCreateFailed, "参考资源创建失败")
	}

	if err := tx.Model(&GenerationTask{}).Where("id = ?", taskID).Update("reference_asset_id", refAsset.ID).Error; err != nil {
		tx.Rollback()
		_ = removeAllTaskDir(taskDir)
		return nil, NewBusinessError(response.OperationFailed, ErrCodeGenerationTaskCreateFailed, "任务创建失败")
	}
	task.ReferenceAssetID = refAsset.ID

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
		GenerationPlanVersion:    task.GenerationPlanVersion,
		ProviderKey:              task.ProviderKeySnapshot,
	}, nil
}

func (s *service) CheckTaskOwnership(taskID, userID string) error {
	task, err := s.repo.GetTaskByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewBusinessError(response.NotFound, ErrCodeGenerationTaskNotFound, "任务不存在")
		}
		return err
	}
	if task.UserID != userID {
		return NewBusinessError(response.Forbidden, ErrCodeTaskNotOwned, "任务不属于当前用户")
	}
	return nil
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
		resp := buildTaskActionResponseWithStat(&a, stat)
		actionResponses = append(actionResponses, *resp)
	}

	return &TaskDetailResponse{
		ID:                          task.ID,
		Name:                        task.Name,
		CharacterID:                 task.CharacterID,
		CharacterName:               characterName,
		ModelConfigID:               task.ModelConfigID,
		ModelName:                   modelName,
		Status:                      task.Status,
		CurrentStage:                task.CurrentStage,
		Progress:                    task.Progress,
		SelectedActionCount:         task.SelectedActionCount,
		EstimatedGenerationCount:    task.EstimatedGenerationCount,
		ReferenceImageUrl:           "/api/desktop-pets/generation-tasks/" + task.ID + "/reference-image",
		ErrorMessage:                task.ErrorMessage,
		CreatedAt:                   task.CreatedAt,
		UpdatedAt:                   task.UpdatedAt,
		StartedAt:                   task.StartedAt,
		CompletedAt:                 task.CompletedAt,
		Actions:                     actionResponses,
		SucceededActionCount:        succeededActionCount,
		FailedActionCount:           failedActionCount,
		CurrentAction:               currentAction,
		DurationSeconds:             computeTaskDurationSeconds(task.StartedAt, task.CompletedAt),
		GenerationPlanVersion:       task.GenerationPlanVersion,
		ProviderKey:                 task.ProviderKeySnapshot,
		ModelNameSnapshot:           task.ModelNameSnapshot,
		CostEstimateJSON:            task.CostEstimateJSON,
		PlannedPrimaryRequestCount:  task.PlannedPrimaryRequestCount,
		PlannedMaxProviderCallCount: task.PlannedMaxProviderCallCount,
		ActualProviderCallCount:     task.ActualProviderCallCount,
		RowVersion:                  task.RowVersion,
		StatusReason:                task.StatusReason,
		FailureStage:                task.FailureStage,
		LastTransitionAt:            task.LastTransitionAt,
		SubmittedAt:                 task.SubmittedAt,
		CancellingAt:                task.CancellingAt,
		CancelledAt:                 task.CancelledAt,
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

func (s *service) ListTasks(userID, characterID, status string, page, pageSize int) (*TaskListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	tasks, total, err := s.repo.ListTasks(userID, characterID, status, page, pageSize)
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
			StatusReason:             t.StatusReason,
			FailureStage:             t.FailureStage,
			LastTransitionAt:         t.LastTransitionAt,
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

func (s *service) GetTaskSourceImageRef(taskID string, userID string) (security.ArtifactReference, error) {
	task, err := s.repo.GetTaskByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return security.ArtifactReference{}, NewBusinessError(response.NotFound, ErrCodeGenerationTaskNotFound, "任务不存在")
		}
		return security.ArtifactReference{}, err
	}
	if task.SourceImagePath == "" {
		return security.ArtifactReference{}, NewBusinessError(response.NotFound, ErrCodeGenerationTaskNotFound, "参考图片不存在")
	}
	if strings.TrimSpace(task.SourceImageHash) == "" ||
		task.SourceImageSize <= 0 {
		return security.ArtifactReference{}, NewBusinessError(response.BusinessError, ErrCodeArtifactUntrusted, "参考图片哈希缺失，拒绝提供")
	}
	storageKey := strings.TrimPrefix(task.SourceImagePath, "desktop-pets/")
	return security.ArtifactReference{
		ArtifactID:  taskID,
		OwnerUserID: userID,
		RootKind:    security.RootGenerationArtifacts,
		StorageKey:  storageKey,
		ContentHash: task.SourceImageHash,
		ByteSize:    int64(task.SourceImageSize),
		MIME:        task.SourceImageMimeType,
	}, nil
}

func computeSHA256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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

func (s *service) GetFrameImageRef(taskID, actionKey string, frameIndex int, userID string) (security.ArtifactReference, error) {
	if frameIndex < 0 {
		return security.ArtifactReference{}, NewBusinessError(response.InvalidParams, ErrCodeFrameNotFound, "帧索引无效")
	}
	if _, err := s.repo.GetTaskByID(taskID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return security.ArtifactReference{}, NewBusinessError(response.NotFound, ErrCodeGenerationTaskNotFound, "任务不存在")
		}
		return security.ArtifactReference{}, err
	}
	actions, err := s.repo.ListActionsByTaskID(taskID)
	if err != nil {
		return security.ArtifactReference{}, err
	}
	var actionID string
	for _, a := range actions {
		if a.ActionKey == actionKey {
			actionID = a.ID
			break
		}
	}
	if actionID == "" {
		return security.ArtifactReference{}, NewBusinessError(response.NotFound, ErrCodeActionNotFound, "动作不存在")
	}
	frames, err := s.repo.ListFramesByAction(actionID)
	if err != nil {
		return security.ArtifactReference{}, err
	}
	var target *GenerationFrame
	for i := range frames {
		if frames[i].FrameIndex == frameIndex {
			target = &frames[i]
			break
		}
	}
	if target == nil {
		return security.ArtifactReference{}, NewBusinessError(response.NotFound, ErrCodeFrameNotFound, "帧不存在")
	}
	if target.Status != "succeeded" || target.ResultImagePath == "" {
		return security.ArtifactReference{}, NewBusinessError(response.NotFound, ErrCodeFrameNotFound, "帧图片不存在")
	}
	if strings.TrimSpace(target.ResultHash) == "" ||
		target.ResultSize <= 0 ||
		strings.TrimSpace(target.ResultImagePath) == "" {
		return security.ArtifactReference{}, NewBusinessError(response.BusinessError, ErrCodeArtifactUntrusted, "帧哈希缺失，拒绝提供")
	}
	storageKey := strings.TrimPrefix(target.ResultImagePath, "desktop-pets/")
	return security.ArtifactReference{
		ArtifactID:  taskID + ":" + actionKey + ":" + strconv.Itoa(frameIndex),
		OwnerUserID: userID,
		RootKind:    security.RootGenerationArtifacts,
		StorageKey:  storageKey,
		ContentHash: target.ResultHash,
		ByteSize:    int64(target.ResultSize),
		MIME:        target.ResultMimeType,
	}, nil
}

func (s *service) StartTask(taskID string) (*TaskSummaryResponse, error) {
	task, err := s.repo.GetTaskByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewBusinessError(response.NotFound, ErrCodeGenerationTaskNotFound, "任务不存在")
		}
		return nil, err
	}

	currentStatus := contracts.LifecycleStatus(task.Status)
	allowedFrom := []contracts.LifecycleStatus{
		contracts.StatusPending,
		contracts.StatusFailed,
		contracts.StatusPartiallySucceeded,
		contracts.StatusCancelled,
		contracts.StatusSucceeded,
	}
	statusAllowed := false
	for _, s := range allowedFrom {
		if currentStatus == s {
			statusAllowed = true
			break
		}
	}
	if !statusAllowed {
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

	if task.ReferenceAssetID != "" {
		_, err := s.refAssetService.ValidateForTask(context.Background(), taskID, task.UserID, task.CharacterID)
		if err != nil {
			return nil, NewBusinessError(response.BusinessError, ErrCodeReferenceImageInvalid, "参考资源验证失败")
		}
	} else {
		fullPath := filepath.Join(config.AppCfg.Storage.DataDir, task.SourceImagePath)
		if _, err := os.Stat(fullPath); err != nil {
			return nil, NewBusinessError(response.BusinessError, ErrCodeReferenceImageInvalid, "参考图片文件不存在")
		}
	}

	taskReason := contracts.ReasonGenerationTaskSubmit
	switch currentStatus {
	case contracts.StatusFailed:
		taskReason = contracts.ReasonGenerationTaskRetry
	case contracts.StatusPartiallySucceeded:
		taskReason = contracts.ReasonGenerationTaskRetryFailedSubset
	case contracts.StatusCancelled:
		taskReason = contracts.ReasonGenerationTaskRetry
	case contracts.StatusSucceeded:
		taskReason = contracts.ReasonGenerationTaskRestart
	}

	_, err = s.stateEngine.Transition(context.Background(), taskstate.TransitionRequest{
		EntityType: contracts.EntityGenerationTask,
		EntityID:   taskID,
		From:       []contracts.LifecycleStatus{currentStatus},
		To:         contracts.StatusQueued,
		Stage:      contracts.StageQueued,
		Reason:     taskReason,
		ActorType:  contracts.ActorService,
		ActorID:    task.UserID,
	})
	if err != nil {
		if taskstate.IsConflictError(err) {
			return nil, NewBusinessError(response.BusinessError, ErrCodeGenerationStateConflict, "任务当前状态不允许开始生成")
		}
		return nil, err
	}

	for _, a := range pendingActions {
		actionStatus := contracts.LifecycleStatus(a.Status)
		actionReason := contracts.ReasonGenerationActionSubmit
		switch actionStatus {
		case contracts.StatusFailed, contracts.StatusCancelled:
			actionReason = contracts.ReasonGenerationActionRetry
		}
		_, aErr := s.stateEngine.Transition(context.Background(), taskstate.TransitionRequest{
			EntityType:   contracts.EntityGenerationAction,
			EntityID:     a.ID,
			From:         []contracts.LifecycleStatus{actionStatus},
			To:           contracts.StatusQueued,
			Stage:        contracts.StageQueued,
			Reason:       actionReason,
			ActorType:    contracts.ActorService,
			ActorID:      task.UserID,
			ParentTaskID: taskID,
		})
		if aErr != nil {
			if taskstate.IsConflictError(aErr) {
				continue
			}
			return nil, aErr
		}
		supplementaryUpdates := map[string]interface{}{
			"progress":   0,
			"started_at": "",
		}
		if err := s.repo.UpdateActionStatusNoTx(a.ID, supplementaryUpdates); err != nil {
			log.Logger.Warnf("failed to reset action %s supplementary fields: %v", a.ID, err)
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

	currentStatus := contracts.LifecycleStatus(task.Status)

	if currentStatus == contracts.StatusPending || currentStatus == contracts.StatusQueued {
		_, err := s.stateEngine.Transition(context.Background(), taskstate.TransitionRequest{
			EntityType: contracts.EntityGenerationTask,
			EntityID:   taskID,
			From:       []contracts.LifecycleStatus{currentStatus},
			To:         contracts.StatusCancelled,
			Stage:      contracts.StageCancelled,
			Reason:     contracts.ReasonGenerationTaskCancelBeforeClaim,
			ActorType:  contracts.ActorService,
			ActorID:    task.UserID,
		})
		if err != nil {
			if taskstate.IsConflictError(err) {
				return NewBusinessError(response.BusinessError, ErrCodeGenerationStateConflict, "任务当前状态不允许取消")
			}
			return err
		}
		return nil
	}

	if currentStatus == contracts.StatusProcessing {
		cancellingStage := contracts.StageGenerating
		if task.CurrentStage != "" {
			potentialStage := contracts.Stage(task.CurrentStage)
			if contracts.IsAllowedStageFor(contracts.EntityGenerationTask, potentialStage) && potentialStage.IsActivityStage() {
				cancellingStage = potentialStage
			}
		}
		_, err := s.stateEngine.Transition(context.Background(), taskstate.TransitionRequest{
			EntityType: contracts.EntityGenerationTask,
			EntityID:   taskID,
			From:       []contracts.LifecycleStatus{currentStatus},
			To:         contracts.StatusCancelling,
			Stage:      cancellingStage,
			Reason:     contracts.ReasonGenerationTaskCancelRequested,
			ActorType:  contracts.ActorService,
			ActorID:    task.UserID,
		})
		if err != nil {
			if taskstate.IsConflictError(err) {
				return NewBusinessError(response.BusinessError, ErrCodeGenerationStateConflict, "任务当前状态不允许取消")
			}
			return err
		}
		now := time.Now().Format(desktopPetTimeFormat)
		if err := s.repo.SetCancelRequested(taskID, now); err != nil {
			log.Logger.Warnf("failed to set cancel requested for task %s: %v", taskID, err)
		}
		return nil
	}

	return NewBusinessError(response.BusinessError, ErrCodeGenerationStateConflict, "任务当前状态不允许取消")
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

	txStateStore := s.stateStore.WithTx(tx)
	txEngine := taskstate.NewEngine(txStateStore)

	actionStatus := contracts.LifecycleStatus(target.Status)
	_, aErr := txEngine.Transition(context.Background(), taskstate.TransitionRequest{
		EntityType:   contracts.EntityGenerationAction,
		EntityID:     target.ID,
		From:         []contracts.LifecycleStatus{actionStatus},
		To:           contracts.StatusQueued,
		Stage:        contracts.StageQueued,
		Reason:       contracts.ReasonGenerationActionRetry,
		ActorType:    contracts.ActorService,
		ActorID:      task.UserID,
		ParentTaskID: taskID,
	})
	if aErr != nil {
		tx.Rollback()
		if taskstate.IsConflictError(aErr) {
			return nil, NewBusinessError(response.BusinessError, ErrCodeGenerationStateConflict, "动作状态冲突，无法重试")
		}
		return nil, aErr
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	supplementaryUpdates := map[string]interface{}{
		"progress":   0,
		"started_at": "",
	}
	if err := s.repo.UpdateActionStatusNoTx(target.ID, supplementaryUpdates); err != nil {
		log.Logger.Warnf("failed to reset action %s supplementary fields: %v", target.ID, err)
	}

	taskStatus := contracts.LifecycleStatus(task.Status)
	if taskStatus.IsTerminal() {
		taskReason := contracts.ReasonGenerationTaskRetry
		switch taskStatus {
		case contracts.StatusPartiallySucceeded:
			taskReason = contracts.ReasonGenerationTaskRetryFailedSubset
		case contracts.StatusSucceeded:
			taskReason = contracts.ReasonGenerationTaskRestart
		}
		_, tErr := s.stateEngine.Transition(context.Background(), taskstate.TransitionRequest{
			EntityType: contracts.EntityGenerationTask,
			EntityID:   taskID,
			From:       []contracts.LifecycleStatus{taskStatus},
			To:         contracts.StatusQueued,
			Stage:      contracts.StageQueued,
			Reason:     taskReason,
			ActorType:  contracts.ActorRetryService,
			ActorID:    task.UserID,
		})
		if tErr != nil {
			if taskstate.IsConflictError(tErr) {
				return nil, NewBusinessError(response.BusinessError, ErrCodeGenerationStateConflict, "任务状态冲突，无法重试")
			}
			return nil, tErr
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
	if cfg == nil {
		return ""
	}
	apiType := strings.TrimSpace(cfg.ApiType)
	if apiType == "" {
		apiType = "seedream"
	}
	canonical := imageprovider.NormalizeProviderName(apiType)
	if _, ok := s.providerRegistry.Resolve(canonical); ok {
		return canonical
	}
	return ""
}

func (s *service) resolveProvider(cfg *imageGenConfigView) (imageprovider.ImageGenerationProvider, string, error) {
	if cfg == nil {
		return nil, "", NewBusinessError(response.BusinessError, ErrCodeImageModelUnavailable, "生图模型配置为空")
	}
	apiType := strings.TrimSpace(cfg.ApiType)
	if apiType == "" {
		apiType = "seedream"
	}
	canonical := imageprovider.NormalizeProviderName(apiType)
	provider, ok := s.providerRegistry.Resolve(canonical)
	if !ok {
		return nil, "", NewBusinessError(response.BusinessError, ErrCodeImageModelTypeUnsupported, "未知的生图模型类型: "+apiType)
	}
	return provider, canonical, nil
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
		GenerationPlanVersion:    task.GenerationPlanVersion,
		ProviderKey:              task.ProviderKeySnapshot,
		RowVersion:               task.RowVersion,
		StatusReason:             task.StatusReason,
		FailureStage:             task.FailureStage,
		LastTransitionAt:         task.LastTransitionAt,
		SubmittedAt:              task.SubmittedAt,
		CancellingAt:             task.CancellingAt,
		CancelledAt:              task.CancelledAt,
	}
}

func (s *service) buildTaskActionResponse(a *GenerationTaskAction) *TaskActionResponse {
	return buildTaskActionResponseWithStat(a, frameStat{})
}

func buildTaskActionResponseWithStat(a *GenerationTaskAction, stat frameStat) *TaskActionResponse {
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
		FrameSucceeded:           stat.succeeded,
		FrameFailed:              stat.failed,
		FrameTotal:               stat.total,
		PlaybackModeSnapshot:     a.PlaybackModeSnapshot,
		DefaultFPSSnapshot:       a.DefaultFPSSnapshot,
		ReturnPolicySnapshot:     a.ReturnPolicySnapshot,
		ReturnActionKeySnapshot:  a.ReturnActionKeySnapshot,
		InterruptibleSnapshot:    a.InterruptibleSnapshot == 1,
		PrioritySnapshot:         a.PrioritySnapshot,
		CooldownMsSnapshot:       a.CooldownMSSnapshot,
		MutexGroupSnapshot:       a.MutexGroupSnapshot,
		AnchorProfileSnapshot:    a.AnchorProfileSnapshot,
		ActionSpecHash:           a.ActionSpecHash,
		GenerationMode:           a.GenerationMode,
		ActiveAttemptID:          a.ActiveAttemptID,
		ActiveAttemptNumber:      a.ActiveAttemptNumber,
		PlannedSegmentCount:      a.PlannedSegmentCount,
		RowVersion:               a.RowVersion,
		CurrentStage:             a.CurrentStage,
		StatusReason:             a.StatusReason,
		FailureStage:             a.FailureStage,
		LastTransitionAt:         a.LastTransitionAt,
	}
}

func buildActionItemResponse(def ActionDefinition) ActionItemResponse {
	tags := []string{}
	if def.SemanticTagsJSON != "" && def.SemanticTagsJSON != "[]" {
		_ = json.Unmarshal([]byte(def.SemanticTagsJSON), &tags)
	}
	return ActionItemResponse{
		ID:                       def.ID,
		Key:                      def.ActionKey,
		Name:                     def.Name,
		Description:              def.Description,
		SupportsDefaultIdle:      def.SupportsDefaultIdle == 1,
		Recommended:              def.Recommended == 1,
		DefaultFrameCount:        def.DefaultFrameCount,
		EstimatedGenerationCount: def.EstimatedGenerationCount,
		DefinitionVersion:        def.DefinitionVersion,
		PlaybackMode:             def.PlaybackMode,
		DefaultFPS:               def.DefaultFPS,
		ReturnPolicy:             def.ReturnPolicy,
		ReturnActionKey:          def.ReturnActionKey,
		Interruptible:            def.Interruptible == 1,
		Priority:                 def.Priority,
		CooldownMs:               def.CooldownMS,
		MutexGroup:               def.MutexGroup,
		QueuePolicy:              def.QueuePolicy,
		AnchorProfile:            def.AnchorProfile,
		Tags:                     tags,
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
		if err := removeLocalDirNoSymlinks(dir); err != nil {
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

func removeLocalDirNoSymlinks(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return os.Remove(path)
	}
	if !info.IsDir() {
		return os.Remove(path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := removeLocalDirNoSymlinks(filepath.Join(path, entry.Name())); err != nil {
			return err
		}
	}
	return os.Remove(path)
}

func (s *service) GetTaskTransitions(taskID string, limit int) ([]taskstate.AuditRecord, error) {
	if _, err := s.repo.GetTaskByID(taskID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewBusinessError(response.NotFound, ErrCodeGenerationTaskNotFound, "任务不存在")
		}
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	ctx := context.Background()
	records, err := s.stateStore.ListAuditsByParent(ctx, taskID, limit)
	if err != nil {
		return nil, err
	}
	return records, nil
}
