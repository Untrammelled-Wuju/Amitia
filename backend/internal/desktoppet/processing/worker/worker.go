// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/internal/desktoppet/processing/application"
	"github.com/u-ai/backend/internal/desktoppet/processing/commit"
	"github.com/u-ai/backend/internal/desktoppet/processing/source"
	"github.com/u-ai/backend/internal/desktoppet/taskstate"
	"github.com/u-ai/backend/log"
	_ "golang.org/x/image/webp"
	"gorm.io/gorm"
)

const (
	processingLeaseDuration      = 10 * time.Minute
	processingHeartbeatInterval  = 30 * time.Second
	processingPollInterval       = 5 * time.Second
	processingWorkerID           = "processing-worker-1"
	processingTimeFormat         = "2006-01-02 15:04:05"
	defaultBackgroundMode        = "remove_background"
	defaultLoopType              = "loop"
	maxConcurrentProcessingTasks = 2
)

type Worker struct {
	db                *gorm.DB
	repo              processing.Repository
	validator         *processing.Validator
	pipeline          *application.Pipeline
	sourceResolver    *application.RepoSourceResolver
	committer         *commit.ProcessingCommitter
	previewGenerator  *processing.PreviewGenerator
	cleanupManager    *processing.CleanupManager
	dataDir           string
	stopCh            chan struct{}
	wg                sync.WaitGroup
	lifecycleMu       sync.Mutex
	running           bool
	alive             atomic.Bool
	sem               chan struct{}
	stateEngine       *taskstate.Engine
	onActionProcessed func(taskID, actionID, actionKey string)
}

func NewWorker(db *gorm.DB, repo processing.Repository, dataDir string, pipeline *application.Pipeline, sourceResolver *application.RepoSourceResolver, committer *commit.ProcessingCommitter) *Worker {
	stateStore := desktoppet.NewStateStore(db)

	return &Worker{
		db:               db,
		repo:             repo,
		validator:        processing.NewValidator(repo, dataDir),
		pipeline:         pipeline,
		sourceResolver:   sourceResolver,
		committer:        committer,
		previewGenerator: processing.NewPreviewGenerator(dataDir),
		cleanupManager:   processing.NewCleanupManager(dataDir),
		dataDir:          dataDir,
		stopCh:           make(chan struct{}),
		sem:              make(chan struct{}, maxConcurrentProcessingTasks),
		stateEngine:      taskstate.NewEngine(stateStore),
	}
}

func (w *Worker) SetOnActionProcessed(fn func(taskID, actionID, actionKey string)) {
	w.onActionProcessed = fn
}

func (w *Worker) Start(ctx context.Context) {
	w.lifecycleMu.Lock()
	defer w.lifecycleMu.Unlock()
	if w.running {
		return
	}
	w.stopCh = make(chan struct{})
	w.running = true
	w.alive.Store(true)
	w.recoverStuckTasks(ctx)
	w.wg.Add(1)
	go w.run(ctx)
}

func (w *Worker) Stop() {
	w.lifecycleMu.Lock()
	defer w.lifecycleMu.Unlock()
	if !w.running {
		return
	}
	close(w.stopCh)
	w.wg.Wait()
	w.running = false
	w.alive.Store(false)
}

func (w *Worker) IsRunning() bool {
	w.lifecycleMu.Lock()
	defer w.lifecycleMu.Unlock()
	return w.running && w.alive.Load()
}

func (w *Worker) run(ctx context.Context) {
	defer w.wg.Done()
	defer w.alive.Store(false)
	ticker := time.NewTicker(processingPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.pollAndProcess(ctx)
		}
	}
}

func (w *Worker) pollAndProcess(ctx context.Context) {
	tasks, err := w.repo.ListQueuedProcessingTasks()
	if err != nil {
		log.Logger.Errorf("processing worker poll tasks failed: %v", err)
		return
	}
	if len(tasks) == 0 {
		return
	}
	for i := range tasks {
		select {
		case <-w.stopCh:
			return
		case <-ctx.Done():
			return
		case w.sem <- struct{}{}:
		}
		task := tasks[i]
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			defer func() { <-w.sem }()
			w.processTask(ctx, &task)
		}()
	}
}

func (w *Worker) processTask(ctx context.Context, task *processing.ProcessingTask) {
	executionID := "processing-" + uuid.New().String()
	leaseExpiresAt := time.Now().Add(processingLeaseDuration).Format(processingTimeFormat)
	now := time.Now().Format(processingTimeFormat)
	progress := ProgressValidatingSources

	_, err := w.stateEngine.Transition(ctx, taskstate.TransitionRequest{
		EntityType:      contracts.EntityProcessingTask,
		EntityID:        task.ID,
		From:            []contracts.LifecycleStatus{contracts.StatusQueued},
		To:              contracts.StatusProcessing,
		Stage:           contracts.StageValidatingSources,
		Reason:          contracts.ReasonProcessingTaskClaim,
		ActorType:       contracts.ActorWorker,
		ActorID:         processingWorkerID,
		ExecutionID:     executionID,
		WorkerID:        processingWorkerID,
		LeaseExpiresAt:  leaseExpiresAt,
		LastHeartbeatAt: now,
		Progress:        &progress,
		NeedOwnership:   false,
	})
	if err != nil {
		if taskstate.IsConflictError(err) {
			return
		}
		log.Logger.Errorf("claim processing task %s failed: %v", task.ID, err)
		return
	}
	log.Logger.Infof("processing worker claimed task %s (execution=%s)", task.ID, executionID)

	w.publishProgress(task.ID, ProgressValidatingSources, StageValidatingSources)

	processingCtx, processingCancel := context.WithCancel(ctx)
	defer processingCancel()

	heartbeatCtx, heartbeatCancel := context.WithCancel(processingCtx)
	w.startHeartbeat(heartbeatCtx, task.ID, executionID, processingCancel)
	defer heartbeatCancel()

	genTask, err := w.repo.GetGenerationTask(task.GenerationTaskID)
	if err != nil {
		log.Logger.Errorf("get generation task %s failed: %v", task.GenerationTaskID, err)
		w.finalizeTask(task.ID, err, executionID)
		return
	}

	processErr := w.runProcessingStages(processingCtx, task, genTask.UserID, executionID)

	w.finalizeTask(task.ID, processErr, executionID)
}

func (w *Worker) runProcessingStages(ctx context.Context, task *processing.ProcessingTask, userID string, executionID string) error {
	if err := w.updateStage(task.ID, executionID, StageValidatingSources, ProgressValidatingSources); err != nil {
		return err
	}
	sourceVal, err := w.validator.ValidateProcessingSources(task.GenerationTaskID, userID)
	if err != nil {
		return fmt.Errorf("validate processing sources failed: %w", err)
	}

	actions, err := w.repo.ListProcessingActionsOrdered(task.ID)
	if err != nil {
		return fmt.Errorf("list processing actions failed: %w", err)
	}
	if len(actions) == 0 {
		return fmt.Errorf("no actions to process")
	}

	seenKeys := make(map[string]bool, len(actions))
	for _, a := range actions {
		if seenKeys[a.ActionKey] {
			return fmt.Errorf("duplicate action key: %s", a.ActionKey)
		}
		seenKeys[a.ActionKey] = true
	}

	totalActions := len(actions)
	for i := range actions {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if w.isCancelled(task.ID) {
			break
		}

		taskProgress := ProgressValidatingSources + (ProgressPreview-ProgressValidatingSources)*i/totalActions
		if err := w.updateProgress(task.ID, executionID, taskProgress); err != nil {
			return err
		}

		action := &actions[i]
		validatedAttempt, ok := sourceVal.SourceAttempts[action.ActionKey]
		if !ok || validatedAttempt <= 0 {
			err := fmt.Errorf("validated source attempt missing for action %s", action.ActionKey)
			log.Logger.Errorf("processing action %s source freeze failed: %v", action.ActionKey, err)
			if failErr := w.failAction(action, err); failErr != nil {
				return errors.Join(err, failErr)
			}
			continue
		}
		if validatedAttempt != action.SourceAttemptNumber {
			err := fmt.Errorf("source generation attempt drift for action %s: frozen=%d current=%d", action.ActionKey, action.SourceAttemptNumber, validatedAttempt)
			log.Logger.Errorf("processing action %s source freeze failed: %v", action.ActionKey, err)
			if failErr := w.failAction(action, err); failErr != nil {
				return errors.Join(err, failErr)
			}
			continue
		}
		if err := w.processAction(ctx, task, action, sourceVal, executionID); err != nil {
			log.Logger.Errorf("processing action %s failed: %v", action.ActionKey, err)
			if failErr := w.failAction(action, err); failErr != nil {
				return errors.Join(err, failErr)
			}
			continue
		}
	}

	if w.isCancelled(task.ID) {
		return fmt.Errorf("cancelled")
	}

	if err := w.updateStage(task.ID, executionID, StagePreview, ProgressPreview); err != nil {
		return err
	}

	if err := w.updateStage(task.ID, executionID, StagePackaging, ProgressManifest); err != nil {
		return err
	}

	if err := w.updateProgress(task.ID, executionID, ProgressPackage); err != nil {
		return err
	}
	return nil
}

func (w *Worker) processAction(ctx context.Context, task *processing.ProcessingTask, action *processing.ProcessingAction, sourceVal *processing.SourceValidationResult, executionID string) (retErr error) {
	tx := w.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("begin transaction for attempt failed: %w", tx.Error)
	}
	attempt, err := w.repo.BeginProcessingActionAttempt(tx, action.ID, action.RowVersion, executionID, action.SourceAttemptNumber)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("begin processing action attempt failed: %w", err)
	}
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit attempt creation failed: %w", err)
	}
	w.publishActionEvent(task.ID, action.ActionKey, "started")
	defer func() {
		if retErr == nil {
			return
		}
		now := time.Now().Format(processingTimeFormat)
		result := w.db.Model(&processing.ProcessingActionAttempt{}).
			Where("id = ? AND status != ?", attempt.ID, "committed").
			Updates(map[string]interface{}{
				"status":        "failed",
				"error_message": retErr.Error(),
				"completed_at":  now,
				"updated_at":    now,
			})
		if result.Error != nil {
			retErr = errors.Join(retErr, fmt.Errorf("persist processing attempt %s failure: %w", attempt.ID, result.Error))
		}
	}()

	configSnapshot := application.BuildConfigSnapshot(task)

	resolveReq := source.ResolveRequest{
		ProcessingTaskID:              task.ID,
		ProcessingActionID:            action.ID,
		ActionKey:                     action.ActionKey,
		GenerationTaskID:              task.GenerationTaskID,
		UserID:                        sourceVal.Task.UserID,
		SourceAttemptID:               attempt.ID,
		SourceGenerationAttemptNumber: action.SourceAttemptNumber,
		CandidateIndex:                0,
		DataDir:                       w.dataDir,
	}
	sourceDesc, err := w.sourceResolver.Resolve(ctx, resolveReq)
	if err != nil {
		return fmt.Errorf("resolve source descriptor failed: %w", err)
	}

	if err := w.updateActionProgress(action, StageBackgroundRemoval, ProgressBackgroundRemoval); err != nil {
		return fmt.Errorf("persist background-removal progress failed: %w", err)
	}

	pipelineReq := application.ProcessActionRequest{
		Context:             ctx,
		SourceDescriptor:    sourceDesc,
		ConfigSnapshot:      configSnapshot,
		ProcessingTaskID:    task.ID,
		ProcessingActionID:  action.ID,
		ProcessingAttemptID: attempt.ID,
		ActionKey:           action.ActionKey,
		GenerationTaskID:    task.GenerationTaskID,
		ExecutionID:         executionID,
		ProcessingVersion:   task.ProcessingVersion,
	}
	result, err := w.pipeline.ProcessAction(pipelineReq)
	if err != nil {
		return fmt.Errorf("pipeline process action failed: %w", err)
	}

	if err := w.updateActionProgress(action, StageWriteFrames, ProgressWriteFrames); err != nil {
		return fmt.Errorf("persist write-frames progress failed: %w", err)
	}

	configSnapshotJSON, err := json.Marshal(configSnapshot)
	if err != nil {
		return fmt.Errorf("marshal config snapshot failed: %w", err)
	}

	if err := w.updateActionProgress(action, StageActionJSON, ProgressActionJSON); err != nil {
		return fmt.Errorf("persist action-json progress failed: %w", err)
	}
	if err := w.writeActionJSON(task, action, result.FrameCount); err != nil {
		w.cleanupDerivedActionResources(task, action)
		return fmt.Errorf("write action json failed: %w", err)
	}

	if err := w.updateActionProgress(action, StagePreview, ProgressPreview); err != nil {
		w.cleanupDerivedActionResources(task, action)
		return fmt.Errorf("persist preview progress failed: %w", err)
	}
	previewImgs, err := w.loadPipelineFrames(result)
	if err != nil {
		w.cleanupDerivedActionResources(task, action)
		return fmt.Errorf("load pipeline frames for preview failed: %w", err)
	}
	if _, err := w.previewGenerator.GenerateActionPreview(task.GenerationTaskID, task.ProcessingVersion, action.ActionKey, previewImgs); err != nil {
		w.cleanupDerivedActionResources(task, action)
		return fmt.Errorf("generate action preview failed: %w", err)
	}

	characterID := task.CharacterID
	if characterID == "" {
		characterID = sourceVal.Task.CharacterID
	}
	if characterID == "" {
		return fmt.Errorf("processing task %s character id is empty", task.ID)
	}

	commitReq := &commit.CommitRequest{
		Ctx:                        ctx,
		UserID:                     sourceVal.Task.UserID,
		CharacterID:                characterID,
		ProcessingTaskID:           task.ID,
		ProcessingActionID:         action.ID,
		ProcessingAttemptID:        attempt.ID,
		ActionKey:                  action.ActionKey,
		SourceManifestID:           "",
		SourceGenerationAttemptID:  sourceDesc.Artifact.AttemptID,
		SourceGenerationArtifactID: sourceDesc.Artifact.ArtifactID,
		SourceArtifactContentHash:  sourceDesc.Artifact.ContentHash,
		ConfigSnapshot:             string(configSnapshotJSON),
		ConfigHash:                 configSnapshot.ConfigHash,
		PipelineVersion:            configSnapshot.PipelineVersion,
		PipelineResult:             result,
		ExpectedActionRowVersion:   action.RowVersion + 1,
		ExecutionID:                executionID,
		LeaseOwner:                 processingWorkerID,
	}

	if _, err := w.committer.Commit(commitReq); err != nil {
		w.cleanupDerivedActionResources(task, action)
		return fmt.Errorf("commit processing result failed: %w", err)
	}

	w.publishActionEvent(action.ProcessingTaskID, action.ActionKey, "succeeded")

	if w.onActionProcessed != nil {
		w.onActionProcessed(task.ID, action.ID, action.ActionKey)
	}

	return nil
}

func (w *Worker) writeActionJSON(task *processing.ProcessingTask, action *processing.ProcessingAction, frameCount int) error {
	fps := action.FPS
	if fps <= 0 {
		fps = task.DefaultFPS
	}
	loopType := action.LoopType
	if action.PlaybackMode == "loop" || action.PlaybackMode == "ping_pong" {
		loopType = "loop"
	} else if action.PlaybackMode == "once" || action.PlaybackMode == "hold" {
		loopType = "once"
	}
	anchor := processing.DefaultAnchorForActionKey(action.ActionKey)
	actionJSON := processing.BuildActionJSON(action.ActionKey, action.ActionNameSnapshot, frameCount, fps, anchor, loopType)
	processing.EnrichActionJSONFromSpec(actionJSON, action)

	actionsDir := filepath.Join(w.dataDir, "desktop-pets", "generation-tasks", task.GenerationTaskID, "processed",
		fmt.Sprintf("version-%d", task.ProcessingVersion), "actions", action.ActionKey)
	if err := os.MkdirAll(actionsDir, 0755); err != nil {
		return fmt.Errorf("create actions dir failed: %w", err)
	}

	finalPath := filepath.Join(actionsDir, "action.json")
	tmpPath := filepath.Join(actionsDir, ".action.json.tmp")

	data, err := json.MarshalIndent(actionJSON, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal action json failed: %w", err)
	}

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write tmp action json failed: %w", err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename action json failed: %w", err)
	}

	return nil
}

func (w *Worker) loadPipelineFrames(result *application.ProcessActionResult) ([]image.Image, error) {
	if result == nil || result.WorkDir == nil {
		return nil, fmt.Errorf("pipeline result/work directory is nil")
	}
	if len(result.Frames) == 0 {
		return nil, fmt.Errorf("pipeline result has no frames")
	}

	frames := append([]application.PipelineFrameResult(nil), result.Frames...)
	sort.Slice(frames, func(i, j int) bool { return frames[i].Index < frames[j].Index })
	imgs := make([]image.Image, 0, len(frames))
	for expectedIndex, frame := range frames {
		if frame.Index != expectedIndex {
			return nil, fmt.Errorf("pipeline frame index is not contiguous: expected=%d actual=%d", expectedIndex, frame.Index)
		}
		name := filepath.Base(frame.FileName)
		if name == "." || name == "" || name != frame.FileName {
			return nil, fmt.Errorf("invalid pipeline frame file name: %s", frame.FileName)
		}
		path := filepath.Join(result.WorkDir.FramesDir, name)
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open pipeline frame %d: %w", frame.Index, err)
		}
		img, _, decodeErr := image.Decode(f)
		closeErr := f.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode pipeline frame %d: %w", frame.Index, decodeErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close pipeline frame %d: %w", frame.Index, closeErr)
		}
		imgs = append(imgs, img)
	}
	return imgs, nil
}

func (w *Worker) cleanupDerivedActionResources(task *processing.ProcessingTask, action *processing.ProcessingAction) {
	if task == nil || action == nil || task.GenerationTaskID == "" || action.ActionKey == "" {
		return
	}
	cleanupManager := processing.NewCleanupManager(w.dataDir)
	if err := cleanupManager.CleanupActionResources(task.GenerationTaskID, task.ProcessingVersion, action.ActionKey); err != nil {
		log.Logger.Errorf("cleanup derived processing action resources %s failed: %v", action.ID, err)
	}
}

func (w *Worker) startHeartbeat(ctx context.Context, taskID, executionID string, leaseLostCancel context.CancelFunc) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(processingHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-w.stopCh:
				return
			case <-ticker.C:
				now := time.Now().Format(processingTimeFormat)
				lease := time.Now().Add(processingLeaseDuration).Format(processingTimeFormat)
				ok, err := w.repo.RefreshProcessingLeaseOwned(taskID, executionID, lease, now)
				if err != nil {
					log.Logger.Errorf("refresh processing lease task %s failed: %v", taskID, err)
					continue
				}
				if !ok {
					log.Logger.Warnf("processing lease lost for task %s, stopping heartbeat", taskID)
					_, tErr := w.stateEngine.Transition(context.Background(), taskstate.TransitionRequest{
						EntityType:    contracts.EntityProcessingTask,
						EntityID:      taskID,
						From:          []contracts.LifecycleStatus{contracts.StatusProcessing},
						To:            contracts.StatusQueued,
						Stage:         contracts.StageQueued,
						Reason:        contracts.ReasonWorkerLeaseLost,
						ActorType:     contracts.ActorWorker,
						ActorID:       processingWorkerID,
						ExecutionID:   executionID,
						NeedOwnership: true,
					})
					if tErr != nil {
						log.Logger.Errorf("requeue processing task %s after lease lost failed: %v", taskID, tErr)
					}
					leaseLostCancel()
					return
				}
			}
		}
	}()
}

func (w *Worker) updateStage(taskID, executionID, stage string, progress int) error {
	now := time.Now().Format(processingTimeFormat)
	owned, err := w.repo.UpdateProcessingTaskOwned(taskID, executionID, map[string]interface{}{
		"current_stage": stage,
		"progress":      progress,
		"updated_at":    now,
	})
	if err != nil {
		return fmt.Errorf("processing task %s updateStage persistence failed: %w", taskID, err)
	}
	if !owned {
		return fmt.Errorf("processing task %s ownership lost during updateStage", taskID)
	}
	w.publishProgress(taskID, progress, stage)
	return nil
}

func (w *Worker) updateProgress(taskID, executionID string, progress int) error {
	now := time.Now().Format(processingTimeFormat)
	owned, err := w.repo.UpdateProcessingTaskOwned(taskID, executionID, map[string]interface{}{
		"progress":   progress,
		"updated_at": now,
	})
	if err != nil {
		return fmt.Errorf("processing task %s progress persistence failed: %w", taskID, err)
	}
	if !owned {
		return fmt.Errorf("processing task %s ownership lost during updateProgress", taskID)
	}
	w.publishProgress(taskID, progress, "")
	return nil
}

func (w *Worker) updateActionProgress(action *processing.ProcessingAction, stage string, progress int) error {
	now := time.Now().Format(processingTimeFormat)
	if err := w.repo.UpdateProcessingActionNoTx(action.ID, map[string]interface{}{
		"progress":   progress,
		"updated_at": now,
	}); err != nil {
		return fmt.Errorf("processing action %s progress persistence failed: %w", action.ID, err)
	}
	w.publishActionProgress(action.ProcessingTaskID, action.ActionKey, stage, progress)
	return nil
}

func (w *Worker) failAction(action *processing.ProcessingAction, cause error) error {
	now := time.Now().Format(processingTimeFormat)
	errMsg := ""
	if cause != nil {
		errMsg = cause.Error()
	}
	if updateErr := w.repo.UpdateProcessingActionNoTx(action.ID, map[string]interface{}{
		"status":        "failed",
		"progress":      100,
		"error_message": errMsg,
		"completed_at":  now,
		"updated_at":    now,
	}); updateErr != nil {
		return fmt.Errorf("processing action %s terminal failure persistence failed: %w", action.ID, updateErr)
	}
	action.Status = "failed"
	w.publishActionEvent(action.ProcessingTaskID, action.ActionKey, "failed")
	return nil
}

func (w *Worker) isCancelled(taskID string) bool {
	task, err := w.repo.GetProcessingTask(taskID)
	if err != nil || task == nil {
		return false
	}
	return task.CancelRequestedAt != ""
}

func (w *Worker) finalizeTask(taskID string, processErr error, executionID string) {
	task, err := w.repo.GetProcessingTask(taskID)
	if err != nil {
		log.Logger.Errorf("get processing task for finalize failed: %v", err)
		return
	}

	defer func() {
		if err := w.cleanupManager.CleanupTempDir(task.GenerationTaskID); err != nil {
			log.Logger.Errorf("cleanup temp dir for task %s failed: %v", task.GenerationTaskID, err)
		}
	}()

	actions, err := w.repo.ListProcessingActions(taskID)
	if err != nil {
		log.Logger.Errorf("list processing actions for finalize failed: %v", err)
		return
	}

	succeeded, failed, hasActiveChildren := 0, 0, false
	for _, a := range actions {
		switch a.Status {
		case "succeeded":
			succeeded++
		case "failed":
			failed++
		case "processing", "queued":
			hasActiveChildren = true
		}
	}
	total := len(actions)

	snapshot := w.buildProcessingSnapshot(task, actions, succeeded, hasActiveChildren)
	decision := taskstate.AggregateProcessingTask(snapshot)

	if processErr != nil && decision.Status == contracts.StatusFailed {
		decision.ErrorMessage = processErr.Error()
	}

	currentStatus := contracts.LifecycleStatus(task.Status)

	if decision.Status == currentStatus && !currentStatus.IsTerminal() {
		log.Logger.Warnf(
			"processing task %s aggregate remains non-terminal (%s); suppressing completed event and leaving task for recovery",
			taskID,
			decision.Status,
		)
		return
	}

	req := taskstate.TransitionRequest{
		EntityType:    contracts.EntityProcessingTask,
		EntityID:      taskID,
		From:          []contracts.LifecycleStatus{currentStatus},
		To:            decision.Status,
		Stage:         decision.Stage,
		Reason:        decision.Reason,
		ActorType:     contracts.ActorFinalizer,
		ActorID:       processingWorkerID,
		ExecutionID:   executionID,
		Progress:      &decision.Progress,
		ErrorCode:     decision.ErrorCode,
		ErrorMessage:  decision.ErrorMessage,
		FailureStage:  decision.FailureStage,
		NeedOwnership: true,
	}

	_, err = w.stateEngine.Transition(context.Background(), req)
	if err != nil {
		if taskstate.IsOwnershipLostError(err) {
			log.Logger.Warnf("processing task %s ownership lost during finalize", taskID)
			return
		}
		log.Logger.Errorf("finalize processing task %s failed: %v", taskID, err)
		return
	}

	w.publishCompleted(taskID, string(decision.Status), succeeded, failed, total)
}

func (w *Worker) buildProcessingSnapshot(task *processing.ProcessingTask, actions []processing.ProcessingAction, succeeded int, hasActiveChildren bool) taskstate.ProcessingSnapshot {
	total := len(actions)
	allActionsSucceeded := total > 0 && succeeded == total
	hasAtLeastOneSucceeded := succeeded > 0

	pkg, pkgErr := w.repo.GetPackageByProcessingTaskID(task.ID)
	packageExists := pkgErr == nil && pkg != nil
	packageReady := false
	packagePathValid := false
	manifestValid := false
	hashValid := false
	includedActionsMatch := false

	if packageExists {
		packageReady = pkg.Status == "succeeded" || pkg.Status == "ready"
		packagePathValid = pkg.PackagePath != "" && pathExists(filepath.Join(w.dataDir, pkg.PackagePath))
		manifestValid = pkg.ManifestPath != "" && fileExists(filepath.Join(w.dataDir, pkg.ManifestPath))
		hashValid = pkg.PackageHash != ""
		includedActionsMatch = checkIncludedActionsMatch(pkg.IncludedActions, actions)
	}

	cancelRequested := task.CancelRequestedAt != ""
	actualProgress := task.Progress
	if actualProgress == 0 {
		actualProgress = 100
	}

	return taskstate.ProcessingSnapshot{
		TaskStatus:                   contracts.LifecycleStatus(task.Status),
		CancelRequested:              cancelRequested,
		HasActiveChildren:            hasActiveChildren,
		AllActionsSucceeded:          allActionsSucceeded,
		HasAtLeastOneActionSucceeded: hasAtLeastOneSucceeded,
		PackageExists:                packageExists,
		PackageReady:                 packageReady,
		PackagePathValid:             packagePathValid,
		ManifestValid:                manifestValid,
		HashValid:                    hashValid,
		IncludedActionsMatch:         includedActionsMatch,
		AllowPartialResult:           true,
		ActualProgress:               actualProgress,
	}
}

func checkIncludedActionsMatch(includedActionsJSON string, actions []processing.ProcessingAction) bool {
	var included []string
	if err := json.Unmarshal([]byte(includedActionsJSON), &included); err != nil {
		return false
	}
	if len(included) == 0 {
		return false
	}
	includedSet := make(map[string]bool, len(included))
	for _, key := range included {
		includedSet[key] = true
	}
	for _, a := range actions {
		if a.Excluded == 1 {
			continue
		}
		if a.Status == "succeeded" {
			if !includedSet[a.ActionKey] {
				return false
			}
		}
	}
	return true
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func pathExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (w *Worker) recoverStuckTasks(ctx context.Context) {
	tasks, err := w.repo.ListRecoverableProcessingTasks()
	if err != nil {
		log.Logger.Errorf("list recoverable processing tasks failed: %v", err)
		return
	}
	for _, task := range tasks {
		if ctx.Err() != nil {
			return
		}
		_, err := w.stateEngine.Transition(ctx, taskstate.TransitionRequest{
			EntityType:    contracts.EntityProcessingTask,
			EntityID:      task.ID,
			From:          []contracts.LifecycleStatus{contracts.StatusProcessing},
			To:            contracts.StatusQueued,
			Stage:         contracts.StageQueued,
			Reason:        contracts.ReasonSystemLeaseExpired,
			ActorType:     contracts.ActorRecovery,
			ExecutionID:   task.ExecutionID,
			NeedOwnership: true,
		})
		if err != nil {
			if taskstate.IsConflictError(err) {
				continue
			}
			log.Logger.Errorf("recover processing task %s failed: %v", task.ID, err)
			continue
		}
		log.Logger.Infof("recovered processing task: %s", task.ID)

		actions, aErr := w.repo.ListProcessingActions(task.ID)
		if aErr != nil {
			log.Logger.Errorf("list processing actions for recovery %s failed: %v", task.ID, aErr)
			continue
		}
		for _, action := range actions {
			if action.Status == "processing" || action.Status == "queued" {
				if rErr := w.repo.ResetProcessingActionToPending(action.ID); rErr != nil {
					log.Logger.Errorf("reset processing action %s to pending failed: %v", action.ID, rErr)
				}
			}
		}
	}
}
