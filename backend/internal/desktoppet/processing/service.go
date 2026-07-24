// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

const (
	ErrCodeProcessingTaskAlreadyRunning = "PROCESSING_TASK_ALREADY_RUNNING"
	ErrCodeProcessingTaskStateConflict  = "PROCESSING_TASK_STATE_CONFLICT"
	ErrCodeProcessingCancelled          = "PROCESSING_CANCELLED"
	ErrCodeProcessingStorageFailed      = "PROCESSING_STORAGE_FAILED"
	ErrCodeProcessingTaskNotFound       = "PROCESSING_TASK_NOT_FOUND"
	ErrCodeProcessingActionNotFound     = "PROCESSING_ACTION_NOT_FOUND"
	ErrCodeProcessingActionNotRetryable = "PROCESSING_ACTION_NOT_RETRYABLE"
	ErrCodeProcessingInvalidAttempt     = "PROCESSING_INVALID_ATTEMPT"
	ErrCodeProcessingExcludedDefault    = "PROCESSING_EXCLUDED_DEFAULT_IDLE"
	ErrCodeProcessingPackageFailed      = "PROCESSING_PACKAGE_FAILED"
)

type ProcessingError struct {
	Code    string
	Message string
	Err     error
}

func (e *ProcessingError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *ProcessingError) Unwrap() error { return e.Err }

func NewProcessingError(code, message string) *ProcessingError {
	return &ProcessingError{Code: code, Message: message}
}

func NewProcessingErrorWithErr(code, message string, err error) *ProcessingError {
	return &ProcessingError{Code: code, Message: message, Err: err}
}

type Service interface {
	CreateProcessingTask(req *CreateProcessingTaskRequest) (*ProcessingTask, error)
	GetProcessingTask(id string) (*GetProcessingTaskResponse, error)
	CancelProcessingTask(id string) error
	RetryProcessingAction(processingTaskID, actionKey string) error
	CreatePackage(req *CreatePackageRequest) (*CreatePackageResponse, error)
	SwitchAttempt(processingTaskID, actionKey string, attemptNumber int) error
	ExcludeAction(processingTaskID, actionKey string) error
	GetProcessedFrameImage(processingTaskID, actionKey string, frameIndex int) (fullPath, mimeType string, err error)
	GetSourceFrameImage(processingTaskID, actionKey string, frameIndex int) (fullPath, mimeType string, err error)
	GetActionPreview(processingTaskID, actionKey string) (fullPath, mimeType string, err error)
}

type service struct {
	repo      Repository
	db        *gorm.DB
	ctx       *app.AppContext
	validator *Validator
	packager  *Packager
	cleanup   *CleanupManager
	dataDir   string
}

func NewService(repo Repository, db *gorm.DB, ctx *app.AppContext, dataDir string) Service {
	return &service{
		repo:      repo,
		db:        db,
		ctx:       ctx,
		dataDir:   dataDir,
		validator: NewValidator(repo, dataDir),
		packager:  NewPackager(repo, dataDir),
		cleanup:   NewCleanupManager(dataDir),
	}
}

type CreateProcessingTaskRequest struct {
	GenerationTaskID            string
	UserID                      string
	OutputWidth                 int
	OutputHeight                int
	TargetCharacterHeightRatio  float64
	AnchorMode                  string
	BackgroundMode              string
	OutputFormat                string
	DefaultFPS                  int
}

type GetProcessingTaskResponse struct {
	ProcessingTask *ProcessingTask    `json:"processingTask"`
	Actions        []ActionStatusInfo `json:"actions"`
	QualitySummary QualitySummary     `json:"qualitySummary"`
	PreviewPaths   map[string]string  `json:"previewPaths"`
}

type ActionStatusInfo struct {
	ActionKey    string   `json:"actionKey"`
	ActionName   string   `json:"actionName"`
	Status       string   `json:"status"`
	Progress     int      `json:"progress"`
	QualityLevel string   `json:"qualityLevel"`
	QualityFlags []string `json:"qualityFlags"`
	SourceAttempt int     `json:"sourceAttempt"`
	Excluded     bool     `json:"excluded"`
}

type QualitySummary struct {
	TotalActions     int `json:"totalActions"`
	SucceededActions int `json:"succeededActions"`
	FailedActions    int `json:"failedActions"`
	WarningActions   int `json:"warningActions"`
}

type CreatePackageRequest struct {
	ProcessingTaskID  string
	UserID            string
	DefaultAction     string
	IncludedActions   []string
	UserDefaultAction string
}

type CreatePackageResponse struct {
	PackageID   string `json:"packageId"`
	PackageHash string `json:"packageHash"`
	Status      string `json:"status"`
}

func wrapValidationError(err error) error {
	if err == nil {
		return nil
	}
	var ve *ValidationError
	if errors.As(err, &ve) {
		return NewProcessingErrorWithErr(ve.Code, ve.Message, ve)
	}
	return err
}

func (s *service) CreateProcessingTask(req *CreateProcessingTaskRequest) (*ProcessingTask, error) {
	if req == nil || req.GenerationTaskID == "" {
		return nil, NewProcessingError(ErrCodeGenerationTaskNotReady, "生成任务 ID 为空")
	}

	validation, err := s.validator.ValidateSources(req.GenerationTaskID, req.UserID)
	if err != nil {
		return nil, wrapValidationError(err)
	}

	existingTasks, err := s.repo.ListProcessingTasksByGenerationTask(req.GenerationTaskID)
	if err != nil {
		return nil, NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询历史处理任务失败", err)
	}

	maxVersion := 0
	for _, t := range existingTasks {
		if t.ProcessingVersion > maxVersion {
			maxVersion = t.ProcessingVersion
		}
		if t.Status == "queued" || t.Status == "processing" {
			return nil, NewProcessingError(ErrCodeProcessingTaskAlreadyRunning, "生成任务已有正在运行的处理任务")
		}
	}

	processingVersion := maxVersion + 1
	now := time.Now().Format(desktopPetTimeFormat)
	taskID := uuid.New().String()

	task := &ProcessingTask{
		ID:                         taskID,
		GenerationTaskID:           req.GenerationTaskID,
		ProcessingVersion:          processingVersion,
		Status:                     "pending",
		CurrentStage:               "queued",
		Progress:                   0,
		OutputWidth:                req.OutputWidth,
		OutputHeight:               req.OutputHeight,
		TargetCharacterHeightRatio: req.TargetCharacterHeightRatio,
		AnchorMode:                 req.AnchorMode,
		BackgroundMode:             req.BackgroundMode,
		OutputFormat:               req.OutputFormat,
		DefaultFPS:                 req.DefaultFPS,
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}

	invalidKeys := make(map[string]bool, len(validation.InvalidActions))
	for _, inv := range validation.InvalidActions {
		invalidKeys[inv.Action.ActionKey] = true
	}

	actions := make([]ProcessingAction, 0, len(validation.SucceededActions))
	for _, genAction := range validation.SucceededActions {
		if invalidKeys[genAction.ActionKey] {
			continue
		}
		if _, ok := validation.FramePaths[genAction.ActionKey]; !ok {
			continue
		}
		attemptNumber, err := s.validator.ResolveActiveAttempt(genAction)
		if err != nil {
			return nil, wrapValidationError(err)
		}
		actions = append(actions, ProcessingAction{
			ID:                     uuid.New().String(),
			ProcessingTaskID:       taskID,
			GenerationTaskActionID: genAction.ID,
			ActionKey:              genAction.ActionKey,
			ActionNameSnapshot:     genAction.ActionNameSnapshot,
			SourceAttemptNumber:    attemptNumber,
			Status:                 "pending",
			Progress:               0,
			SourceFrameCount:       genAction.FrameCount,
			CreatedAt:              now,
			UpdatedAt:              now,
		})
	}

	if len(actions) == 0 {
		return nil, NewProcessingError(ErrCodeNoSuccessfulActions, "没有可处理的成功动作")
	}

	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "开启事务失败", tx.Error)
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	if err := s.repo.CreateProcessingTask(tx, task); err != nil {
		return nil, NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "创建处理任务失败", err)
	}

	if err := s.repo.CreateProcessingActions(tx, actions); err != nil {
		return nil, NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "创建处理动作失败", err)
	}

	if err := s.repo.UpdateProcessingTaskStatus(tx, taskID, map[string]interface{}{
		"status":        "queued",
		"current_stage": "queued",
		"updated_at":    now,
	}); err != nil {
		return nil, NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "更新处理任务状态失败", err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "提交事务失败", err)
	}
	committed = true

	task.Status = "queued"
	task.CurrentStage = "queued"
	return task, nil
}

func (s *service) GetProcessingTask(id string) (*GetProcessingTaskResponse, error) {
	task, err := s.repo.GetProcessingTask(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewProcessingError(ErrCodeProcessingTaskNotFound, "处理任务不存在")
		}
		return nil, NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询处理任务失败", err)
	}

	actions, err := s.repo.ListProcessingActionsOrdered(id)
	if err != nil {
		return nil, NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询处理动作失败", err)
	}

	latestActions := s.dedupLatestActions(actions)

	actionInfos := make([]ActionStatusInfo, 0, len(latestActions))
	summary := QualitySummary{TotalActions: len(latestActions)}

	for _, action := range latestActions {
		info := ActionStatusInfo{
			ActionKey:    action.ActionKey,
			ActionName:   action.ActionNameSnapshot,
			Status:       action.Status,
			Progress:     action.Progress,
			SourceAttempt: action.SourceAttemptNumber,
			Excluded:     action.Excluded == 1,
			QualityFlags: []string{},
		}

		frames, ferr := s.repo.ListProcessedFramesByAction(action.ID)
		if ferr == nil && len(frames) > 0 {
			info.QualityLevel, info.QualityFlags = summarizeFrameQuality(frames)
		}

		actionInfos = append(actionInfos, info)

		if action.Excluded == 1 {
			continue
		}
		switch action.Status {
		case "succeeded":
			summary.SucceededActions++
		case "failed":
			summary.FailedActions++
		case "warning":
			summary.WarningActions++
		}
	}

	previewPaths := s.buildPreviewPaths(task, latestActions)

	return &GetProcessingTaskResponse{
		ProcessingTask: task,
		Actions:        actionInfos,
		QualitySummary: summary,
		PreviewPaths:   previewPaths,
	}, nil
}

func (s *service) CancelProcessingTask(id string) error {
	task, err := s.repo.GetProcessingTask(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewProcessingError(ErrCodeProcessingTaskNotFound, "处理任务不存在")
		}
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询处理任务失败", err)
	}

	if task.Status != "processing" && task.Status != "queued" {
		return NewProcessingError(ErrCodeProcessingTaskStateConflict, fmt.Sprintf("任务状态 %s 不可取消", task.Status))
	}

	now := time.Now().Format(desktopPetTimeFormat)
	if err := s.repo.SetProcessingCancelRequested(id, now); err != nil {
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "设置取消标记失败", err)
	}

	actions, err := s.repo.ListProcessingActions(id)
	if err != nil {
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询处理动作失败", err)
	}

	for _, action := range actions {
		if action.Status == "pending" {
			if err := s.repo.UpdateProcessingActionNoTx(action.ID, map[string]interface{}{
				"status":      "cancelled",
				"updated_at":  now,
				"completed_at": now,
			}); err != nil {
				return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "取消处理动作失败", err)
			}
		}
	}

	if err := s.repo.UpdateProcessingTaskStatusNoTx(id, map[string]interface{}{
		"status":        "cancelled",
		"current_stage": "cancelled",
		"completed_at":  now,
		"updated_at":    now,
	}); err != nil {
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "更新处理任务状态失败", err)
	}

	if cleanupErr := s.cleanup.CleanupTempDir(task.GenerationTaskID); cleanupErr != nil {
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "清理临时文件失败", cleanupErr)
	}

	return nil
}

func (s *service) RetryProcessingAction(processingTaskID, actionKey string) error {
	_, err := s.repo.GetProcessingTask(processingTaskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewProcessingError(ErrCodeProcessingTaskNotFound, "处理任务不存在")
		}
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询处理任务失败", err)
	}

	actions, err := s.repo.ListProcessingActionsOrdered(processingTaskID)
	if err != nil {
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询处理动作失败", err)
	}

	var latestAction *ProcessingAction
	for i := len(actions) - 1; i >= 0; i-- {
		if actions[i].ActionKey == actionKey {
			latestAction = &actions[i]
			break
		}
	}
	if latestAction == nil {
		return NewProcessingError(ErrCodeProcessingActionNotFound, "处理动作不存在")
	}

	if latestAction.Status != "failed" && latestAction.Status != "succeeded" {
		return NewProcessingError(ErrCodeProcessingActionNotRetryable, fmt.Sprintf("动作状态 %s 不可重试", latestAction.Status))
	}

	now := time.Now().Format(desktopPetTimeFormat)
	newAction := &ProcessingAction{
		ID:                     uuid.New().String(),
		ProcessingTaskID:       processingTaskID,
		GenerationTaskActionID: latestAction.GenerationTaskActionID,
		ActionKey:              latestAction.ActionKey,
		ActionNameSnapshot:     latestAction.ActionNameSnapshot,
		SourceAttemptNumber:    latestAction.SourceAttemptNumber,
		Status:                 "pending",
		Progress:               0,
		SourceFrameCount:       latestAction.SourceFrameCount,
		LoopType:               latestAction.LoopType,
		FPS:                    latestAction.FPS,
		FrameDurationMS:        latestAction.FrameDurationMS,
		AnchorType:             latestAction.AnchorType,
		AnchorX:                latestAction.AnchorX,
		AnchorY:                latestAction.AnchorY,
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	tx := s.db.Begin()
	if tx.Error != nil {
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "开启事务失败", tx.Error)
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	if err := s.repo.CreateProcessingActions(tx, []ProcessingAction{*newAction}); err != nil {
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "创建重试动作记录失败", err)
	}

	if err := s.repo.UpdateProcessingTaskStatus(tx, processingTaskID, map[string]interface{}{
		"status":        "queued",
		"current_stage": "queued",
		"error_code":    "",
		"error_message": "",
		"updated_at":    now,
	}); err != nil {
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "更新处理任务状态失败", err)
	}

	if err := tx.Commit().Error; err != nil {
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "提交事务失败", err)
	}
	committed = true

	return nil
}

func (s *service) CreatePackage(req *CreatePackageRequest) (*CreatePackageResponse, error) {
	if req == nil || req.ProcessingTaskID == "" {
		return nil, NewProcessingError(ErrCodeProcessingTaskNotFound, "处理任务 ID 为空")
	}

	task, err := s.repo.GetProcessingTask(req.ProcessingTaskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewProcessingError(ErrCodeProcessingTaskNotFound, "处理任务不存在")
		}
		return nil, NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询处理任务失败", err)
	}

	processingActions, err := s.repo.ListProcessingActions(req.ProcessingTaskID)
	if err != nil {
		return nil, NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询处理动作失败", err)
	}

	excludedKeys := make(map[string]bool)
	for _, pa := range processingActions {
		if pa.Excluded == 1 {
			excludedKeys[pa.ActionKey] = true
		}
	}

	succeededActions, err := s.repo.ListSucceededActions(task.GenerationTaskID)
	if err != nil {
		return nil, NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询成功动作失败", err)
	}

	availableActions := make([]desktoppet.GenerationTaskAction, 0, len(succeededActions))
	for _, a := range succeededActions {
		if !excludedKeys[a.ActionKey] {
			availableActions = append(availableActions, a)
		}
	}

	if len(availableActions) == 0 {
		return nil, NewProcessingError(ErrCodeNoSuccessfulActions, "没有可打包的成功动作")
	}

	genTask, err := s.repo.GetGenerationTask(task.GenerationTaskID)
	if err != nil {
		return nil, NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询生成任务失败", err)
	}

	defaultAction := req.DefaultAction
	if defaultAction == "" {
		selector := NewDefaultActionSelector(req.UserDefaultAction)
		selected, err := selector.SelectDefaultAction(availableActions)
		if err != nil {
			return nil, NewProcessingError(ErrCodeDefaultIdleActionUnavailable, "选择默认待机动作失败: "+err.Error())
		}
		defaultAction = selected
	} else {
		found := false
		for _, a := range availableActions {
			if a.ActionKey == defaultAction {
				if a.SupportsDefaultIdle != 1 {
					return nil, NewProcessingError(ErrCodePackageDefaultActionInvalid, fmt.Sprintf("动作 %s 不支持默认待机", defaultAction))
				}
				found = true
				break
			}
		}
		if !found {
			return nil, NewProcessingError(ErrCodePackageDefaultActionInvalid, fmt.Sprintf("默认动作 %s 不在可用动作中", defaultAction))
		}
	}

	includedActions := req.IncludedActions
	if len(includedActions) == 0 {
		excludedList := make([]string, 0, len(excludedKeys))
		for k := range excludedKeys {
			excludedList = append(excludedList, k)
		}
		included, err := SelectIncludedActions(succeededActions, defaultAction, excludedList)
		if err != nil {
			return nil, NewProcessingError(ErrCodeProcessingPackageFailed, "选择包含动作失败: "+err.Error())
		}
		includedActions = included
	} else {
		includedSet := make(map[string]bool, len(includedActions))
		for _, k := range includedActions {
			includedSet[k] = true
		}
		if !includedSet[defaultAction] {
			return nil, NewProcessingError(ErrCodePackageDefaultActionInvalid, fmt.Sprintf("默认动作 %s 不在包含动作中", defaultAction))
		}
		availableMap := make(map[string]bool, len(availableActions))
		for _, a := range availableActions {
			availableMap[a.ActionKey] = true
		}
		for _, k := range includedActions {
			if !availableMap[k] {
				return nil, NewProcessingError(ErrCodeProcessingPackageFailed, fmt.Sprintf("动作 %s 不在可用动作中", k))
			}
		}
	}

	buildReq := &PackageBuildRequest{
		ProcessingTaskID:  req.ProcessingTaskID,
		UserID:            req.UserID,
		CharacterID:       genTask.CharacterID,
		GenerationTaskID:  task.GenerationTaskID,
		PackageName:       genTask.Name,
		DefaultAction:     defaultAction,
		IncludedActions:   includedActions,
		CanvasWidth:       task.OutputWidth,
		CanvasHeight:      task.OutputHeight,
		ProcessingVersion: task.ProcessingVersion,
		UserDefaultAction: req.UserDefaultAction,
		SucceededActions:  availableActions,
	}

	result, err := s.packager.BuildPackage(buildReq)
	if err != nil {
		return nil, err
	}

	return &CreatePackageResponse{
		PackageID:   result.Package.ID,
		PackageHash: result.PackageHash,
		Status:      result.Package.Status,
	}, nil
}

func (s *service) SwitchAttempt(processingTaskID, actionKey string, attemptNumber int) error {
	task, err := s.repo.GetProcessingTask(processingTaskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewProcessingError(ErrCodeProcessingTaskNotFound, "处理任务不存在")
		}
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询处理任务失败", err)
	}

	actions, err := s.repo.ListProcessingActionsOrdered(processingTaskID)
	if err != nil {
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询处理动作失败", err)
	}

	var latestAction *ProcessingAction
	for i := len(actions) - 1; i >= 0; i-- {
		if actions[i].ActionKey == actionKey {
			latestAction = &actions[i]
			break
		}
	}
	if latestAction == nil {
		return NewProcessingError(ErrCodeProcessingActionNotFound, "处理动作不存在")
	}

	genActions, err := s.repo.ListSucceededActions(task.GenerationTaskID)
	if err != nil {
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询成功动作失败", err)
	}

	var genAction *desktoppet.GenerationTaskAction
	for i := range genActions {
		if genActions[i].ID == latestAction.GenerationTaskActionID {
			genAction = &genActions[i]
			break
		}
	}
	if genAction == nil {
		return NewProcessingError(ErrCodeProcessingActionNotFound, "关联生成动作不存在")
	}

	maxAttempt := genAction.AttemptNumber
	if maxAttempt <= 0 {
		maxAttempt = latestAction.SourceAttemptNumber
	}
	if attemptNumber < 1 || attemptNumber > maxAttempt {
		return NewProcessingError(ErrCodeProcessingInvalidAttempt, fmt.Sprintf("attempt %d 超出范围 [1, %d]", attemptNumber, maxAttempt))
	}

	now := time.Now().Format(desktopPetTimeFormat)
	tx := s.db.Begin()
	if tx.Error != nil {
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "开启事务失败", tx.Error)
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	if err := s.repo.UpdateProcessingActionAttempt(tx, latestAction.ID, attemptNumber); err != nil {
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "更新源尝试编号失败", err)
	}

	if err := s.repo.UpdateProcessingAction(tx, latestAction.ID, map[string]interface{}{
		"status":       "pending",
		"progress":     0,
		"error_code":   "",
		"error_message": "",
		"started_at":   "",
		"completed_at": "",
		"updated_at":   now,
	}); err != nil {
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "重置动作状态失败", err)
	}

	if err := s.repo.UpdateProcessingTaskStatus(tx, processingTaskID, map[string]interface{}{
		"status":        "queued",
		"current_stage": "queued",
		"error_code":    "",
		"error_message": "",
		"updated_at":    now,
	}); err != nil {
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "更新处理任务状态失败", err)
	}

	if err := tx.Commit().Error; err != nil {
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "提交事务失败", err)
	}
	committed = true

	return nil
}

func (s *service) ExcludeAction(processingTaskID, actionKey string) error {
	task, err := s.repo.GetProcessingTask(processingTaskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewProcessingError(ErrCodeProcessingTaskNotFound, "处理任务不存在")
		}
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询处理任务失败", err)
	}

	actions, err := s.repo.ListProcessingActionsOrdered(processingTaskID)
	if err != nil {
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询处理动作失败", err)
	}

	var latestAction *ProcessingAction
	for i := len(actions) - 1; i >= 0; i-- {
		if actions[i].ActionKey == actionKey {
			latestAction = &actions[i]
			break
		}
	}
	if latestAction == nil {
		return NewProcessingError(ErrCodeProcessingActionNotFound, "处理动作不存在")
	}

	if latestAction.Excluded == 1 {
		return nil
	}

	genActions, err := s.repo.ListSucceededActions(task.GenerationTaskID)
	if err != nil {
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询成功动作失败", err)
	}

	var targetGenAction *desktoppet.GenerationTaskAction
	for i := range genActions {
		if genActions[i].ID == latestAction.GenerationTaskActionID {
			targetGenAction = &genActions[i]
			break
		}
	}

	if targetGenAction != nil && targetGenAction.SupportsDefaultIdle == 1 {
		otherIdleAvailable := false
		processingActionMap := make(map[string]*ProcessingAction)
		for i := range actions {
			if actions[i].ActionKey == actionKey {
				continue
			}
			processingActionMap[actions[i].ActionKey] = &actions[i]
		}

		for i := range genActions {
			ga := &genActions[i]
			if ga.ActionKey == actionKey {
				continue
			}
			if ga.SupportsDefaultIdle != 1 {
				continue
			}
			pa, ok := processingActionMap[ga.ActionKey]
			if ok && pa.Excluded == 1 {
				continue
			}
			otherIdleAvailable = true
			break
		}

		if !otherIdleAvailable {
			return NewProcessingError(ErrCodeProcessingExcludedDefault, "禁止排除唯一默认待机动作")
		}
	}

	if err := s.repo.SetActionExcluded(latestAction.ID, true); err != nil {
		return NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "设置排除标记失败", err)
	}

	return nil
}

func (s *service) dedupLatestActions(actions []ProcessingAction) []ProcessingAction {
	latestMap := make(map[string]ProcessingAction)
	order := make([]string, 0, len(actions))
	for _, a := range actions {
		if _, exists := latestMap[a.ActionKey]; !exists {
			order = append(order, a.ActionKey)
		}
		latestMap[a.ActionKey] = a
	}
	result := make([]ProcessingAction, 0, len(order))
	for _, key := range order {
		result = append(result, latestMap[key])
	}
	return result
}

func summarizeFrameQuality(frames []ProcessedFrame) (string, []string) {
	if len(frames) == 0 {
		return QualityLevelNormal, []string{}
	}

	allFlags := make(map[string]bool)
	hasFailed := false
	hasWarning := false

	for _, f := range frames {
		flags := parseQualityFlags(f.QualityFlags)
		for _, flag := range flags {
			allFlags[flag] = true
		}
		switch f.Status {
		case "failed":
			hasFailed = true
		}
		if IsAutoFail(flags) {
			hasFailed = true
		} else if len(flags) > 0 {
			hasWarning = true
		}
	}

	flags := make([]string, 0, len(allFlags))
	for flag := range allFlags {
		flags = append(flags, flag)
	}

	if hasFailed {
		return QualityLevelFailed, flags
	}
	if hasWarning {
		return QualityLevelWarning, flags
	}
	return QualityLevelNormal, flags
}

func parseQualityFlags(s string) []string {
	if s == "" {
		return nil
	}
	return splitQualityFlags(s)
}

func splitQualityFlags(s string) []string {
	var result []string
	current := ""
	for _, ch := range s {
		if ch == ',' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func (s *service) buildPreviewPaths(task *ProcessingTask, actions []ProcessingAction) map[string]string {
	paths := make(map[string]string)
	for _, action := range actions {
		if action.Excluded == 1 {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(
			"desktop-pets", "generation-tasks", task.GenerationTaskID,
			"processed", fmt.Sprintf("version-%d", task.ProcessingVersion),
			"actions", action.ActionKey, "preview.png",
		))
		paths[action.ActionKey] = rel
	}
	return paths
}

func (s *service) resolveProcessingFrame(processingTaskID, actionKey string, frameIndex int) (*ProcessingTask, *ProcessingAction, []ProcessedFrame, error) {
	task, err := s.repo.GetProcessingTask(processingTaskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, NewProcessingError(ErrCodeProcessingTaskNotFound, "处理任务不存在")
		}
		return nil, nil, nil, NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询处理任务失败", err)
	}

	actions, err := s.repo.ListProcessingActionsOrdered(processingTaskID)
	if err != nil {
		return nil, nil, nil, NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询处理动作失败", err)
	}

	var latestAction *ProcessingAction
	for i := len(actions) - 1; i >= 0; i-- {
		if actions[i].ActionKey == actionKey {
			latestAction = &actions[i]
			break
		}
	}
	if latestAction == nil {
		return nil, nil, nil, NewProcessingError(ErrCodeProcessingActionNotFound, "处理动作不存在")
	}

	frames, err := s.repo.ListProcessedFramesByAction(latestAction.ID)
	if err != nil {
		return nil, nil, nil, NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询处理帧失败", err)
	}

	if frameIndex < 0 || frameIndex >= len(frames) {
		return nil, nil, nil, NewProcessingError(ErrCodeProcessingInvalidAttempt, fmt.Sprintf("帧索引 %d 超出范围 [0, %d]", frameIndex, len(frames)-1))
	}

	return task, latestAction, frames, nil
}

func (s *service) GetProcessedFrameImage(processingTaskID, actionKey string, frameIndex int) (fullPath, mimeType string, err error) {
	_, _, frames, err := s.resolveProcessingFrame(processingTaskID, actionKey, frameIndex)
	if err != nil {
		return "", "", err
	}

	frame := frames[frameIndex]
	if frame.ProcessedPath == "" {
		return "", "", NewProcessingError(ErrCodeProcessingTaskNotFound, "处理帧图片不存在")
	}

	fullPath = filepath.Join(s.dataDir, frame.ProcessedPath)
	return fullPath, "image/png", nil
}

func (s *service) GetSourceFrameImage(processingTaskID, actionKey string, frameIndex int) (fullPath, mimeType string, err error) {
	_, _, frames, err := s.resolveProcessingFrame(processingTaskID, actionKey, frameIndex)
	if err != nil {
		return "", "", err
	}

	frame := frames[frameIndex]
	if frame.SourcePath == "" {
		return "", "", NewProcessingError(ErrCodeProcessingTaskNotFound, "源帧图片不存在")
	}

	fullPath = filepath.Join(s.dataDir, frame.SourcePath)
	return fullPath, "image/png", nil
}

func (s *service) GetActionPreview(processingTaskID, actionKey string) (fullPath, mimeType string, err error) {
	task, err := s.repo.GetProcessingTask(processingTaskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", NewProcessingError(ErrCodeProcessingTaskNotFound, "处理任务不存在")
		}
		return "", "", NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询处理任务失败", err)
	}

	_, err = s.repo.GetProcessingActionByActionKey(processingTaskID, actionKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", NewProcessingError(ErrCodeProcessingActionNotFound, "处理动作不存在")
		}
		return "", "", NewProcessingErrorWithErr(ErrCodeProcessingStorageFailed, "查询处理动作失败", err)
	}

	fullPath = filepath.Join(s.dataDir, "desktop-pets", "generation-tasks", task.GenerationTaskID,
		"processed", fmt.Sprintf("version-%d", task.ProcessingVersion),
		"actions", actionKey, "preview.png")
	return fullPath, "image/png", nil
}
