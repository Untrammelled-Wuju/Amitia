package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/internal/desktoppet/generation"
	"github.com/u-ai/backend/internal/desktoppet/generation/activebinding"
	"github.com/u-ai/backend/internal/desktoppet/generation/commit"
	"github.com/u-ai/backend/internal/desktoppet/referenceasset"
	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

var (
	errGenerationPersistence     = errors.New("generation executor persistence failure")
	errGenerationRecoveryPending = errors.New("generation attempt requires recovery")
)

const (
	generationPollTimeout  = 5 * time.Minute
	generationPollInterval = 3 * time.Second
	maxArtifactPixels      = int64(64 * 1024 * 1024)
)

type GenerationExecutor struct {
	db                *gorm.DB
	repo              desktoppet.Repository
	registry          *imageprovider.Registry
	attemptFactory    *generation.AttemptFactory
	artifactPersister *generation.ArtifactPersister
	artifactCommitter *commit.ArtifactCommitter
	downloader        *desktoppet.ResultDownloader
	refAssetRepo      referenceasset.Repository
	receiptRepo       generation.ReceiptRepository
	finalizer         *generation.GenerationFinalizer
	bindingService    *activebinding.BindingService
}

func NewGenerationExecutor(
	db *gorm.DB,
	repo desktoppet.Repository,
	registry *imageprovider.Registry,
	attemptFactory *generation.AttemptFactory,
	artifactPersister *generation.ArtifactPersister,
	artifactCommitter *commit.ArtifactCommitter,
	downloader *desktoppet.ResultDownloader,
	refAssetRepo referenceasset.Repository,
	receiptRepo generation.ReceiptRepository,
	finalizer *generation.GenerationFinalizer,
	bindingService *activebinding.BindingService,
) *GenerationExecutor {
	return &GenerationExecutor{
		db:                db,
		repo:              repo,
		registry:          registry,
		attemptFactory:    attemptFactory,
		artifactPersister: artifactPersister,
		artifactCommitter: artifactCommitter,
		downloader:        downloader,
		refAssetRepo:      refAssetRepo,
		receiptRepo:       receiptRepo,
		finalizer:         finalizer,
		bindingService:    bindingService,
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
		if errors.Is(err, generation.ErrAttemptAlreadyActive) {
			return fmt.Errorf("%w: active generation attempt already exists for action %s: %v", errGenerationRecoveryPending, action.ID, err)
		}
		return fmt.Errorf("创建初始 attempt 失败: %w", err)
	}
	if err := createTx.Commit().Error; err != nil {
		return fmt.Errorf("提交 attempt 事务失败: %w", err)
	}

	log.Logger.Infof("desktoppet generation executor started: task=%s action=%s attempt=%s provider=%s model=%s",
		task.ID, action.ID, attempt.ID, providerName, modelName)

	if err := e.updateActionProgress(task, action, string(generation.AttemptStatusPending)); err != nil {
		return fmt.Errorf("%w: persist pending action progress: %v", errGenerationPersistence, err)
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if err := e.updateAttemptStatus(attempt.ID, generation.AttemptStatusPreparingReference, nil); err != nil {
		return fmt.Errorf("%w: persist attempt preparing_reference: %v", errGenerationPersistence, err)
	}
	if err := e.updateActionProgress(task, action, string(generation.AttemptStatusPreparingReference)); err != nil {
		return fmt.Errorf("%w: persist preparing_reference action progress: %v", errGenerationPersistence, err)
	}

	referenceImages, refErr := e.resolveReferenceImages(plan, task)
	if refErr != nil {
		errMsg := fmt.Sprintf("加载参考图失败: %v", refErr)
		return e.failExecution(attempt.ID, action, desktoppet.ErrCodeImageGenerationRequestInvalid, errMsg, refErr)
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if err := e.updateAttemptStatus(attempt.ID, generation.AttemptStatusSubmitting, map[string]interface{}{
		"submitted_at": time.Now().Format(workerTimeFormat),
	}); err != nil {
		return fmt.Errorf("%w: persist attempt submitting: %v", errGenerationPersistence, err)
	}
	if err := e.updateActionProgress(task, action, string(generation.AttemptStatusSubmitting)); err != nil {
		return fmt.Errorf("%w: persist submitting action progress: %v", errGenerationPersistence, err)
	}

	request := e.buildImageGenerationRequest(task, plan, referenceImages)
	requestHash := computeRequestHash(request)

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
		return e.failExecution(attempt.ID, action, errCode, errMsg, err)
	}

	if submission == nil {
		errMsg := "提供者返回空提交结果"
		return e.failExecution(attempt.ID, action, desktoppet.ErrCodeImageGenerationEmptyResult, errMsg, nil)
	}

	receiptWriteErr := e.persistProviderReceipt(attempt.ID, providerName, modelName, submission, requestHash)
	if receiptWriteErr != nil {
		// A synchronous terminal response is already authoritative and still in
		// memory. Do not discard a paid, successfully returned image solely because
		// the auxiliary provider-receipt row could not be written; the artifact
		// commit/finalizer transaction below remains the durable source of truth.
		// Non-terminal/ambiguous submissions still fail closed into recovery.
		if submission.Result != nil && (submission.Result.Status == "succeeded" || submission.Result.Status == "failed") {
			log.Logger.Warnf("desktoppet executor provider receipt write failed for terminal response; continuing with durable result commit: attempt=%s err=%v", attempt.ID, receiptWriteErr)
		} else {
			log.Logger.Errorf("desktoppet executor persist receipt failed for attempt %s: %v", attempt.ID, receiptWriteErr)
			if err := e.updateAttemptStatus(attempt.ID, generation.AttemptStatusUnknownSubmission, map[string]interface{}{
				"provider_request_id":   submission.RequestID,
				"provider_operation_id": submission.OperationID,
			}); err != nil {
				return fmt.Errorf("%w: persist unknown_submission after receipt failure: %v", errGenerationPersistence, err)
			}
			if err := e.updateActionProgress(task, action, string(generation.AttemptStatusUnknownSubmission)); err != nil {
				return fmt.Errorf("%w: persist unknown_submission action progress after receipt failure: %v", errGenerationPersistence, err)
			}
			return fmt.Errorf("%w: %s: %v", errGenerationRecoveryPending, generation.ErrCodeProviderReceiptPersistFailed, receiptWriteErr)
		}
	}

	var result *imageprovider.ImageGenerationResult

	if submission.Status == "processing" || submission.Status == "accepted" {
		if err := e.updateAttemptStatus(attempt.ID, generation.AttemptStatusSubmitted, map[string]interface{}{
			"provider_request_id":   submission.RequestID,
			"provider_operation_id": submission.OperationID,
		}); err != nil {
			return fmt.Errorf("%w: persist attempt submitted: %v", errGenerationPersistence, err)
		}
		if err := e.updateAttemptStatus(attempt.ID, generation.AttemptStatusPolling, nil); err != nil {
			return fmt.Errorf("%w: persist attempt polling: %v", errGenerationPersistence, err)
		}
		if err := e.updateActionProgress(task, action, string(generation.AttemptStatusPolling)); err != nil {
			return fmt.Errorf("%w: persist polling action progress: %v", errGenerationPersistence, err)
		}

		pollResult, pollErr := e.pollProvider(ctx, provider, modelConfig, submission.OperationID)
		if pollErr != nil {
			errCode := errorCodeOf(pollErr)
			if errCode == "" {
				errCode = desktoppet.ErrCodeImageGenerationPollFailed
			}
			errMsg := fmt.Sprintf("轮询生图结果失败: %v", pollErr)
			return e.failExecution(attempt.ID, action, errCode, errMsg, pollErr)
		}
		result = pollResult
	} else if submission.Status == "succeeded" || submission.Status == "failed" {
		result = submission.Result
	} else {
		if err := e.updateAttemptStatus(attempt.ID, generation.AttemptStatusUnknownSubmission, map[string]interface{}{
			"provider_request_id":   submission.RequestID,
			"provider_operation_id": submission.OperationID,
		}); err != nil {
			return fmt.Errorf("%w: persist unknown_submission: %v", errGenerationPersistence, err)
		}
		if err := e.updateActionProgress(task, action, string(generation.AttemptStatusUnknownSubmission)); err != nil {
			return fmt.Errorf("%w: persist unknown_submission action progress: %v", errGenerationPersistence, err)
		}
		log.Logger.Warnf("desktoppet executor attempt %s entered unknown_submission state: submission.Status=%s", attempt.ID, submission.Status)
		errMsg := fmt.Sprintf("提交状态不确定: %s，进入 unknown_submission 等待 Recovery Worker 处理", submission.Status)
		return fmt.Errorf("%w: %s", errGenerationRecoveryPending, errMsg)
	}

	if result == nil {
		errMsg := "生图结果为空"
		return e.failExecution(attempt.ID, action, desktoppet.ErrCodeImageGenerationEmptyResult, errMsg, nil)
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
		return e.failExecution(attempt.ID, action, errCode, errMsg, nil)
	}

	if len(result.Images) == 0 {
		errMsg := "生图结果未包含任何图片"
		return e.failExecution(attempt.ID, action, desktoppet.ErrCodeImageGenerationEmptyResult, errMsg, nil)
	}

	if err := e.updateAttemptStatus(attempt.ID, generation.AttemptStatusResultReceived, map[string]interface{}{
		"provider_request_id":   result.RequestID,
		"provider_operation_id": result.OperationID,
	}); err != nil {
		return fmt.Errorf("%w: persist attempt result_received: %v", errGenerationPersistence, err)
	}
	if err := e.updateActionProgress(task, action, string(generation.AttemptStatusResultReceived)); err != nil {
		return fmt.Errorf("%w: persist result_received action progress: %v", errGenerationPersistence, err)
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if err := e.updateAttemptStatus(attempt.ID, generation.AttemptStatusPersisting, nil); err != nil {
		return fmt.Errorf("%w: persist attempt persisting: %v", errGenerationPersistence, err)
	}
	if err := e.updateActionProgress(task, action, string(generation.AttemptStatusPersisting)); err != nil {
		return fmt.Errorf("%w: persist persisting action progress: %v", errGenerationPersistence, err)
	}

	genResult := convertToGenerationResult(result)

	persistTx := e.db.Begin()
	if persistTx.Error != nil {
		errMsg := fmt.Sprintf("开启持久化事务失败: %v", persistTx.Error)
		return e.failExecution(attempt.ID, action, desktoppet.ErrCodeGenerationWorkerError, errMsg, persistTx.Error)
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
		if rollbackErr := persistTx.Rollback().Error; rollbackErr != nil {
			err = errors.Join(err, fmt.Errorf("rollback persistence transaction: %w", rollbackErr))
		}
		errMsg := fmt.Sprintf("持久化产物失败: %v", err)
		return e.failExecution(attempt.ID, action, generation.ErrCodeArtifactPersistFailed, errMsg, err)
	}

	actualCost := 0.0
	actualInputUnits := 0
	actualOutputUnits := 0
	if result.Usage != nil {
		actualInputUnits = int(result.Usage.PromptTokens)
		actualOutputUnits = int(result.Usage.CompletionTokens)
	}

	finalizeErr := e.finalizer.FinalizeAttempt(generation.FinalizeAttemptRequest{
		Tx:                persistTx,
		AttemptID:         attempt.ID,
		TaskActionID:      action.ID,
		TaskID:            task.ID,
		PrimaryArtifactID: persistResult.PrimaryArtifact.ID,
		ArtifactHash:      persistResult.PrimaryArtifact.Hash,
		ExecutionID:       task.ExecutionID,
		ActualCost:        actualCost,
		ActualInputUnits:  actualInputUnits,
		ActualOutputUnits: actualOutputUnits,
		AutoPromote:       true,
	})
	if finalizeErr != nil {
		rollbackErr := e.rollbackGenerationPersistence(persistTx, persistResult)
		combinedErr := finalizeErr
		if rollbackErr != nil {
			combinedErr = errors.Join(finalizeErr, rollbackErr)
		}
		errMsg := fmt.Sprintf("Finalize 失败: %v", combinedErr)
		return e.failExecution(attempt.ID, action, generation.ErrCodeFinalizeFailed, errMsg, combinedErr)
	}

	if err := persistTx.Commit().Error; err != nil {
		cleanupErr := e.artifactPersister.RollbackPersistedFiles("", persistResult)
		if rollbackErr := persistTx.Rollback().Error; rollbackErr != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("rollback failed persistence transaction: %w", rollbackErr))
		}
		combinedErr := err
		if cleanupErr != nil {
			combinedErr = errors.Join(err, cleanupErr)
		}
		errMsg := fmt.Sprintf("提交持久化事务失败: %v", combinedErr)
		return e.failExecution(attempt.ID, action, desktoppet.ErrCodeGenerationWorkerError, errMsg, combinedErr)
	}

	successProgress := generation.CalculateActionProgressFromString(string(generation.AttemptStatusSucceeded))
	action.Progress = successProgress
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

func (e *GenerationExecutor) rollbackGenerationPersistence(tx *gorm.DB, result *generation.PersistResult) error {
	var rollbackErrs []error
	if e.artifactPersister != nil {
		if err := e.artifactPersister.RollbackPersistedFiles("", result); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("rollback published generation artifacts: %w", err))
		}
	}
	if tx != nil {
		if err := tx.Rollback().Error; err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("rollback generation persistence transaction: %w", err))
		}
	}
	return errors.Join(rollbackErrs...)
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

func (e *GenerationExecutor) updateActionProgress(task *desktoppet.GenerationTask, action *desktoppet.GenerationTaskAction, stage string) error {
	progress := generation.CalculateActionProgressFromString(stage)
	now := time.Now().Format(workerTimeFormat)
	if err := e.repo.UpdateActionStatusNoTx(action.ID, map[string]interface{}{
		"progress":   progress,
		"updated_at": now,
	}); err != nil {
		return err
	}
	action.Progress = progress
	desktoppet.PublishTaskEvent(task.ID, "action.progress", map[string]interface{}{
		"task_id":    task.ID,
		"action_id":  action.ID,
		"action_key": action.ActionKey,
		"stage":      stage,
		"progress":   progress,
	})
	return nil
}

func (e *GenerationExecutor) failAttempt(attemptID, code, message string) error {
	now := time.Now().Format(workerTimeFormat)
	return e.db.Model(&generation.ActionGenerationAttempt{}).Where("id = ?", attemptID).Updates(map[string]interface{}{
		"status":        string(generation.AttemptStatusFailed),
		"error_code":    code,
		"error_message": message,
		"completed_at":  now,
		"updated_at":    now,
	}).Error
}

func (e *GenerationExecutor) failAction(action *desktoppet.GenerationTaskAction, code, message string) error {
	now := time.Now().Format(workerTimeFormat)
	return e.repo.UpdateActionStatusNoTx(action.ID, map[string]interface{}{
		"status":        "failed",
		"error_code":    code,
		"error_message": message,
		"completed_at":  now,
		"updated_at":    now,
	})
}

func (e *GenerationExecutor) failExecution(attemptID string, action *desktoppet.GenerationTaskAction, code, message string, cause error) error {
	var persistenceErrs []error
	if err := e.failAttempt(attemptID, code, message); err != nil {
		persistenceErrs = append(persistenceErrs, fmt.Errorf("attempt %s: %w", attemptID, err))
	}
	if err := e.failAction(action, code, message); err != nil {
		persistenceErrs = append(persistenceErrs, fmt.Errorf("action %s: %w", action.ID, err))
	}
	if len(persistenceErrs) > 0 {
		return fmt.Errorf("%w: %v", errGenerationPersistence, errors.Join(persistenceErrs...))
	}
	action.Status = "failed"
	if cause != nil {
		return fmt.Errorf("%s: %w", code, cause)
	}
	if code != "" {
		return fmt.Errorf("%s: %s", code, message)
	}
	return errors.New(message)
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
