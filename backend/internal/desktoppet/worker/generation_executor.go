// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worker

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/internal/desktoppet/generation"
	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

const (
	generationPollTimeout  = 5 * time.Minute
	generationPollInterval = 3 * time.Second
)

type GenerationExecutor struct {
	db                *gorm.DB
	repo              desktoppet.Repository
	registry          *imageprovider.Registry
	attemptFactory    *generation.AttemptFactory
	artifactPersister *generation.ArtifactPersister
	downloader        *desktoppet.ResultDownloader
}

func NewGenerationExecutor(
	db *gorm.DB,
	repo desktoppet.Repository,
	registry *imageprovider.Registry,
	attemptFactory *generation.AttemptFactory,
	artifactPersister *generation.ArtifactPersister,
	downloader *desktoppet.ResultDownloader,
) *GenerationExecutor {
	return &GenerationExecutor{
		db:                db,
		repo:              repo,
		registry:          registry,
		attemptFactory:    attemptFactory,
		artifactPersister: artifactPersister,
		downloader:        downloader,
	}
}

func (e *GenerationExecutor) Execute(
	ctx context.Context,
	task *desktoppet.GenerationTask,
	action *desktoppet.GenerationTaskAction,
	plan *generation.GenerationPlanSnapshot,
	providerName string,
	modelName string,
	provider imageprovider.ImageGenerationProvider,
	modelConfig imageprovider.ImageModelConfig,
) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	createTx := e.db.Begin()
	if createTx.Error != nil {
		return fmt.Errorf("开启 attempt 事务失败: %w", createTx.Error)
	}
	attempt, err := e.attemptFactory.CreateInitial(createTx, task.ID, action.ID, task.ExecutionID, task.WorkerID, plan)
	if err != nil {
		createTx.Rollback()
		return fmt.Errorf("创建初始 attempt 失败: %w", err)
	}
	if err := createTx.Commit().Error; err != nil {
		return fmt.Errorf("提交 attempt 事务失败: %w", err)
	}

	log.Logger.Infof("desktoppet generation executor started: task=%s action=%s attempt=%s provider=%s model=%s",
		task.ID, action.ID, attempt.ID, providerName, modelName)

	e.updateActionProgress(task, action, string(generation.AttemptStatusPending))

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if err := e.updateAttemptStatus(attempt.ID, generation.AttemptStatusPreparingReference, nil); err != nil {
		log.Logger.Warnf("desktoppet executor update attempt %s to preparing_reference failed: %v", attempt.ID, err)
	}
	e.updateActionProgress(task, action, string(generation.AttemptStatusPreparingReference))

	sourceAbsPath := filepath.Join(config.AppCfg.Storage.DataDir, task.SourceImagePath)
	referenceImages := desktoppet.SelectReferenceImages(sourceAbsPath, "", false)

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if err := e.updateAttemptStatus(attempt.ID, generation.AttemptStatusSubmitting, map[string]interface{}{
		"submitted_at": time.Now().Format(workerTimeFormat),
	}); err != nil {
		log.Logger.Warnf("desktoppet executor update attempt %s to submitting failed: %v", attempt.ID, err)
	}
	e.updateActionProgress(task, action, string(generation.AttemptStatusSubmitting))

	request := e.buildImageGenerationRequest(task, plan, referenceImages)

	desktoppet.PublishTaskEvent(task.ID, "action.progress", map[string]interface{}{
		"task_id":    task.ID,
		"action_id":  action.ID,
		"action_key": action.ActionKey,
		"stage":      string(generation.AttemptStatusSubmitting),
		"attempt_id": attempt.ID,
		"provider":   providerName,
		"model":      modelName,
	})

	submission, err := provider.Submit(ctx, modelConfig, request)
	if err != nil {
		errCode := errorCodeOf(err)
		if errCode == "" {
			errCode = desktoppet.ErrCodeImageGenerationProviderRejected
		}
		errMsg := fmt.Sprintf("提交生图请求失败: %v", err)
		e.failAttempt(attempt.ID, errCode, errMsg)
		e.failAction(action, errCode, errMsg)
		return fmt.Errorf("%s: %w", errCode, err)
	}

	if submission == nil {
		errMsg := "提供者返回空提交结果"
		e.failAttempt(attempt.ID, desktoppet.ErrCodeImageGenerationEmptyResult, errMsg)
		e.failAction(action, desktoppet.ErrCodeImageGenerationEmptyResult, errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	var result *imageprovider.ImageGenerationResult

	if submission.Status == "processing" || submission.Status == "accepted" {
		if err := e.updateAttemptStatus(attempt.ID, generation.AttemptStatusPolling, map[string]interface{}{
			"provider_request_id":   submission.RequestID,
			"provider_operation_id": submission.OperationID,
		}); err != nil {
			log.Logger.Warnf("desktoppet executor update attempt %s to polling failed: %v", attempt.ID, err)
		}
		e.updateActionProgress(task, action, string(generation.AttemptStatusPolling))

		pollResult, pollErr := e.pollProvider(ctx, provider, modelConfig, submission.OperationID)
		if pollErr != nil {
			errCode := errorCodeOf(pollErr)
			if errCode == "" {
				errCode = desktoppet.ErrCodeImageGenerationPollFailed
			}
			errMsg := fmt.Sprintf("轮询生图结果失败: %v", pollErr)
			e.failAttempt(attempt.ID, errCode, errMsg)
			e.failAction(action, errCode, errMsg)
			return fmt.Errorf("%s: %w", errCode, pollErr)
		}
		result = pollResult
	} else if submission.Status == "succeeded" || submission.Status == "failed" {
		result = submission.Result
	} else {
		if err := e.updateAttemptStatus(attempt.ID, generation.AttemptStatusUnknownSubmission, map[string]interface{}{
			"provider_request_id":   submission.RequestID,
			"provider_operation_id": submission.OperationID,
		}); err != nil {
			log.Logger.Warnf("desktoppet executor update attempt %s to unknown_submission failed: %v", attempt.ID, err)
		}
		errMsg := fmt.Sprintf("未知的提交状态: %s", submission.Status)
		e.failAttempt(attempt.ID, desktoppet.ErrCodeImageGenerationProviderRejected, errMsg)
		e.failAction(action, desktoppet.ErrCodeImageGenerationProviderRejected, errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	if result == nil {
		errMsg := "生图结果为空"
		e.failAttempt(attempt.ID, desktoppet.ErrCodeImageGenerationEmptyResult, errMsg)
		e.failAction(action, desktoppet.ErrCodeImageGenerationEmptyResult, errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	if result.Status == "failed" {
		errCode := result.ErrorCode
		if errCode == "" {
			errCode = desktoppet.ErrCodeImageGenerationProviderRejected
		}
		errMsg := result.ErrorMessage
		if errMsg == "" {
			errMsg = "提供者返回失败状态"
		}
		e.failAttempt(attempt.ID, errCode, errMsg)
		e.failAction(action, errCode, errMsg)
		return fmt.Errorf("提供者返回失败状态: %s", errMsg)
	}

	if len(result.Images) == 0 {
		errMsg := "生图结果未包含任何图片"
		e.failAttempt(attempt.ID, desktoppet.ErrCodeImageGenerationEmptyResult, errMsg)
		e.failAction(action, desktoppet.ErrCodeImageGenerationEmptyResult, errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	if err := e.updateAttemptStatus(attempt.ID, generation.AttemptStatusResultReceived, map[string]interface{}{
		"provider_request_id":   result.RequestID,
		"provider_operation_id": result.OperationID,
	}); err != nil {
		log.Logger.Warnf("desktoppet executor update attempt %s to result_received failed: %v", attempt.ID, err)
	}
	e.updateActionProgress(task, action, string(generation.AttemptStatusResultReceived))

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if err := e.updateAttemptStatus(attempt.ID, generation.AttemptStatusPersisting, nil); err != nil {
		log.Logger.Warnf("desktoppet executor update attempt %s to persisting failed: %v", attempt.ID, err)
	}
	e.updateActionProgress(task, action, string(generation.AttemptStatusPersisting))

	genResult := convertToGenerationResult(result)
	persistTx := e.db.Begin()
	if persistTx.Error != nil {
		errMsg := fmt.Sprintf("开启持久化事务失败: %v", persistTx.Error)
		e.failAttempt(attempt.ID, desktoppet.ErrCodeGenerationWorkerError, errMsg)
		e.failAction(action, desktoppet.ErrCodeGenerationWorkerError, errMsg)
		return fmt.Errorf("%s", errMsg)
	}
	persistResult, err := e.artifactPersister.Persist(generation.PersistInput{
		Tx:                  persistTx,
		TaskID:              task.ID,
		TaskActionID:        action.ID,
		AttemptID:           attempt.ID,
		Plan:                plan,
		Result:              genResult,
		SegmentIndex:        0,
		ExecutionID:         task.ExecutionID,
		ProviderRequestID:   result.RequestID,
		ProviderOperationID: result.OperationID,
	})
	if err != nil {
		persistTx.Rollback()
		errMsg := fmt.Sprintf("持久化产物失败: %v", err)
		e.failAttempt(attempt.ID, generation.ErrCodeArtifactPersistFailed, errMsg)
		e.failAction(action, generation.ErrCodeArtifactPersistFailed, errMsg)
		return fmt.Errorf("%s", errMsg)
	}
	if err := persistTx.Commit().Error; err != nil {
		errMsg := fmt.Sprintf("提交持久化事务失败: %v", err)
		e.failAttempt(attempt.ID, desktoppet.ErrCodeGenerationWorkerError, errMsg)
		e.failAction(action, desktoppet.ErrCodeGenerationWorkerError, errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	now := time.Now().Format(workerTimeFormat)
	if err := e.updateAttemptStatus(attempt.ID, generation.AttemptStatusSucceeded, map[string]interface{}{
		"completed_at": now,
	}); err != nil {
		log.Logger.Warnf("desktoppet executor update attempt %s to succeeded failed: %v", attempt.ID, err)
	}

	successProgress := generation.CalculateActionProgressFromString(string(generation.AttemptStatusSucceeded))
	action.Progress = successProgress
	_ = e.repo.UpdateActionStatusNoTx(action.ID, map[string]interface{}{
		"status":       "succeeded",
		"progress":     successProgress,
		"completed_at": now,
		"updated_at":   now,
	})
	desktoppet.PublishTaskEvent(task.ID, "action.completed", map[string]interface{}{
		"task_id":        task.ID,
		"action_id":      action.ID,
		"action_key":     action.ActionKey,
		"status":         "succeeded",
		"progress":       successProgress,
		"attempt_id":     attempt.ID,
		"provider":       providerName,
		"model":          modelName,
		"artifact_count": len(persistResult.Artifacts),
	})

	log.Logger.Infof("desktoppet generation executor succeeded: task=%s action=%s attempt=%s artifacts=%d",
		task.ID, action.ID, attempt.ID, len(persistResult.Artifacts))
	return nil
}

func (e *GenerationExecutor) pollProvider(
	ctx context.Context,
	provider imageprovider.ImageGenerationProvider,
	modelConfig imageprovider.ImageModelConfig,
	operationID string,
) (*imageprovider.ImageGenerationResult, error) {
	deadline := time.Now().Add(generationPollTimeout)
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("轮询超时 (operation=%s)", operationID)
		}
		result, err := provider.Query(ctx, modelConfig, operationID)
		if err != nil {
			return nil, err
		}
		if result == nil {
			return nil, fmt.Errorf("轮询返回空结果 (operation=%s)", operationID)
		}
		if result.Status == "succeeded" || result.Status == "failed" {
			return result, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(generationPollInterval):
		}
	}
}

func (e *GenerationExecutor) buildImageGenerationRequest(
	task *desktoppet.GenerationTask,
	plan *generation.GenerationPlanSnapshot,
	referenceImages []imageprovider.ImageInput,
) imageprovider.ImageGenerationRequest {
	width := plan.SheetWidth
	if width <= 0 {
		width = task.OutputWidth
	}
	height := plan.SheetHeight
	if height <= 0 {
		height = task.OutputHeight
	}
	outputCount := plan.OutputCount
	if outputCount <= 0 {
		outputCount = 1
	}
	return imageprovider.ImageGenerationRequest{
		RequestID:       uuid.New().String(),
		Mode:            imageprovider.GenerationMode(plan.Mode),
		Prompt:          plan.PromptSnapshot,
		NegativePrompt:  plan.NegativePromptSnapshot,
		ReferenceImages: referenceImages,
		Width:           width,
		Height:          height,
		Seed:            plan.SeedValue,
		OutputCount:     outputCount,
	}
}

func (e *GenerationExecutor) updateAttemptStatus(attemptID string, status generation.AttemptStatus, extra map[string]interface{}) error {
	now := time.Now().Format(workerTimeFormat)
	updates := map[string]interface{}{
		"status":     string(status),
		"updated_at": now,
	}
	for k, v := range extra {
		updates[k] = v
	}
	return e.db.Model(&generation.ActionGenerationAttempt{}).Where("id = ?", attemptID).Updates(updates).Error
}

func (e *GenerationExecutor) updateActionProgress(task *desktoppet.GenerationTask, action *desktoppet.GenerationTaskAction, stage string) {
	progress := generation.CalculateActionProgressFromString(stage)
	action.Progress = progress
	now := time.Now().Format(workerTimeFormat)
	_ = e.repo.UpdateActionStatusNoTx(action.ID, map[string]interface{}{
		"progress":   progress,
		"updated_at": now,
	})
	desktoppet.PublishTaskEvent(task.ID, "action.progress", map[string]interface{}{
		"task_id":    task.ID,
		"action_id":  action.ID,
		"action_key": action.ActionKey,
		"stage":      stage,
		"progress":   progress,
	})
}

func (e *GenerationExecutor) failAttempt(attemptID, code, message string) {
	now := time.Now().Format(workerTimeFormat)
	_ = e.db.Model(&generation.ActionGenerationAttempt{}).Where("id = ?", attemptID).Updates(map[string]interface{}{
		"status":        string(generation.AttemptStatusFailed),
		"error_code":    code,
		"error_message": message,
		"completed_at":  now,
		"updated_at":    now,
	}).Error
}

func (e *GenerationExecutor) failAction(action *desktoppet.GenerationTaskAction, code, message string) {
	now := time.Now().Format(workerTimeFormat)
	_ = e.repo.UpdateActionStatusNoTx(action.ID, map[string]interface{}{
		"status":        "failed",
		"error_code":    code,
		"error_message": message,
		"completed_at":  now,
		"updated_at":    now,
	})
}

func convertToGenerationResult(legacy *imageprovider.ImageGenerationResult) *imageprovider.GenerationResult {
	if legacy == nil {
		return nil
	}
	result := &imageprovider.GenerationResult{
		SubmissionState: mapLegacyStatusToSubmissionState(legacy.Status),
		OperationID:     legacy.OperationID,
		RequestID:       legacy.RequestID,
		Provider:        legacy.Provider,
		Model:           legacy.Model,
		Usage:           legacy.Usage,
		ErrorCode:       legacy.ErrorCode,
		ErrorMessage:    legacy.ErrorMessage,
		RawMetadata:     legacy.RawMetadata,
	}
	for i := range legacy.Images {
		img := &legacy.Images[i]
		result.Candidates = append(result.Candidates, imageprovider.CandidateImage{
			Index:    i,
			Bytes:    img.Bytes,
			MimeType: img.MimeType,
			Width:    img.Width,
			Height:   img.Height,
			Metadata: img.Metadata,
		})
	}
	return result
}

func mapLegacyStatusToSubmissionState(status string) imageprovider.SubmissionState {
	switch status {
	case "succeeded":
		return imageprovider.SubmissionSucceeded
	case "failed":
		return imageprovider.SubmissionFailed
	case "processing":
		return imageprovider.SubmissionProcessing
	case "accepted":
		return imageprovider.SubmissionAccepted
	default:
		return imageprovider.SubmissionUnknown
	}
}
