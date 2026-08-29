// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/internal/desktoppet/generation"
	"github.com/u-ai/backend/internal/desktoppet/generation/activebinding"
	"github.com/u-ai/backend/internal/desktoppet/generation/commit"
	"github.com/u-ai/backend/internal/desktoppet/generationlayout"
	"github.com/u-ai/backend/internal/desktoppet/generationprompt"
	"github.com/u-ai/backend/internal/desktoppet/referenceasset"
	"github.com/u-ai/backend/internal/desktoppet/specs"
	"github.com/u-ai/backend/internal/desktoppet/taskstate"
	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

const (
	workerTimeFormat     = "2006-01-02 15:04:05"
	pollInterval         = 3 * time.Second
	heartbeatInterval    = 30 * time.Second
	leaseDuration        = 5 * time.Minute
	maxConcurrentTasks   = 4
	maxFrameRetries      = 2
	taskRecoveryInterval = 30 * time.Second
)

type Worker struct {
	db                *gorm.DB
	repo              desktoppet.Repository
	registry          *imageprovider.Registry
	downloader        *desktoppet.ResultDownloader
	stateStore        *desktoppet.StateStore
	stateEngine       *taskstate.Engine
	attemptRepo       generation.AttemptRepository
	artifactRepo      generation.ArtifactRepository
	attemptFactory    *generation.AttemptFactory
	artifactPersister *generation.ArtifactPersister
	artifactCommitter *commit.ArtifactCommitter
	refAssetRepo      referenceasset.Repository
	receiptRepo       generation.ReceiptRepository
	finalizer         *generation.GenerationFinalizer
	bindingService    *activebinding.BindingService
	recoveryWorker    *generation.RecoveryWorker
	genExecutor       *GenerationExecutor
	legacyExecutor    *LegacyFrameExecutor
	stopCh            chan struct{}
	wg                sync.WaitGroup
	sem               chan struct{}
	lifecycleMu       sync.Mutex
	lastRecoveryScan  time.Time
	running           bool
	alive             atomic.Bool
}

func NewWorker(db *gorm.DB, repo desktoppet.Repository, registry *imageprovider.Registry) *Worker {
	stateStore := desktoppet.NewStateStore(db)
	attemptRepo := generation.NewAttemptRepository(db)
	artifactRepo := generation.NewArtifactRepository(db)
	attemptFactory := generation.NewAttemptFactory(attemptRepo)
	artifactPersister := generation.NewArtifactPersister(artifactRepo)

	journalRepo := commit.NewJournalRepository(db)
	artifactCommitter := commit.NewArtifactCommitter(journalRepo, artifactRepo)
	commitFunc := makeCommitFunc(artifactCommitter)
	artifactPersister = artifactPersister.WithCommitFunc(commitFunc)

	refAssetRepo := referenceasset.NewRepository(db)
	receiptRepo := generation.NewReceiptRepository(db)

	activeBindingRepo := activebinding.NewRepository(db)
	bindingService := activebinding.NewBindingService(activeBindingRepo)
	finalizer := generation.NewGenerationFinalizer(attemptRepo, artifactRepo, bindingService)

	recoveryWorker := generation.NewRecoveryWorker(
		db, attemptRepo, artifactRepo, receiptRepo,
		generation.DefaultRecoveryWorkerConfig(),
	)

	w := &Worker{
		db:                db,
		repo:              repo,
		registry:          registry,
		downloader:        desktoppet.NewResultDownloader(),
		stateStore:        stateStore,
		stateEngine:       taskstate.NewEngine(stateStore),
		attemptRepo:       attemptRepo,
		artifactRepo:      artifactRepo,
		attemptFactory:    attemptFactory,
		artifactPersister: artifactPersister,
		artifactCommitter: artifactCommitter,
		refAssetRepo:      refAssetRepo,
		receiptRepo:       receiptRepo,
		finalizer:         finalizer,
		bindingService:    bindingService,
		recoveryWorker:    recoveryWorker,
		stopCh:            make(chan struct{}),
		sem:               make(chan struct{}, maxConcurrentTasks),
	}
	recoveryWorker.
		WithArtifactPersister(artifactPersister).
		WithFinalizer(finalizer).
		WithProviderQuery(w.queryGenerationProviderForRecovery).
		WithTerminalCallback(w.onGenerationAttemptRecovered)
	w.genExecutor = NewGenerationExecutor(
		db, repo, registry, attemptFactory, artifactPersister, artifactCommitter,
		w.downloader, refAssetRepo, receiptRepo, finalizer, bindingService,
	)
	w.legacyExecutor = NewLegacyFrameExecutor(w)
	return w
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
	w.RecoverOnStartup(ctx)
	w.recoveryWorker.Start(ctx)
	w.wg.Add(1)
	go w.pollLoop(ctx)
}

func (w *Worker) Stop() {
	w.lifecycleMu.Lock()
	defer w.lifecycleMu.Unlock()
	if !w.running {
		return
	}
	close(w.stopCh)
	w.recoveryWorker.Stop()
	w.wg.Wait()
	w.running = false
	w.alive.Store(false)
}

func (w *Worker) IsRunning() bool {
	w.lifecycleMu.Lock()
	defer w.lifecycleMu.Unlock()
	return w.running && w.alive.Load() && w.recoveryWorker.IsRunning()
}

func (w *Worker) pollLoop(ctx context.Context) {
	defer w.wg.Done()
	defer w.alive.Store(false)
	defer func() {
		if r := recover(); r != nil {
			log.Logger.Errorf("worker pollLoop panic: %v", r)
		}
	}()
	for {
		select {
		case <-w.stopCh:
			return
		case <-ctx.Done():
			return
		default:
		}
		w.pollOnce(ctx)
		select {
		case <-w.stopCh:
			return
		case <-ctx.Done():
			return
		case <-time.After(pollInterval):
		}
	}
}

func (w *Worker) pollOnce(ctx context.Context) {
	if w.lastRecoveryScan.IsZero() || time.Since(w.lastRecoveryScan) >= taskRecoveryInterval {
		w.recoverExpiredTasks(ctx)
		w.lastRecoveryScan = time.Now()
	}
	tasks, err := w.repo.ListQueuedTasks()
	if err != nil {
		log.Logger.Errorf("desktoppet worker poll tasks failed: %v", err)
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
			defer func() {
				if r := recover(); r != nil {
					log.Logger.Errorf("worker processTask(%s) panic: %v", task.ID, r)
				}
			}()
			w.processTask(ctx, &task)
		}()
	}
}

func (w *Worker) processTask(ctx context.Context, task *desktoppet.GenerationTask) {
	workerID := uuid.New().String()
	executionID := uuid.New().String()
	leaseExpiresAt := time.Now().Add(leaseDuration).Format(workerTimeFormat)

	claimed, err := w.repo.ClaimTask(task.ID, workerID, executionID, leaseExpiresAt)
	if err != nil {
		log.Logger.Errorf("claim task %s failed: %v", task.ID, err)
		return
	}
	if !claimed {
		return
	}

	snapshot, snapErr := w.stateEngine.GetSnapshot(ctx, contracts.EntityGenerationTask, task.ID)
	if snapErr != nil || snapshot == nil {
		log.Logger.Errorf("task %s snapshot verify failed: %v", task.ID, snapErr)
		return
	}
	if snapshot.Status != contracts.StatusProcessing || snapshot.ExecutionID != executionID {
		log.Logger.Errorf("task %s state mismatch after claim: status=%s exec=%s", task.ID, snapshot.Status, snapshot.ExecutionID)
		return
	}

	log.Logger.Infof("desktoppet worker claimed task %s (worker=%s execution=%s)", task.ID, workerID, executionID)
	desktoppet.PublishTaskEvent(task.ID, "task.claimed", map[string]interface{}{
		"task_id":      task.ID,
		"execution_id": executionID,
		"worker_id":    workerID,
	})

	task.ExecutionID = executionID
	task.WorkerID = workerID

	now := time.Now().Format(workerTimeFormat)
	owned, err := w.repo.UpdateTaskOwned(task.ID, executionID, map[string]interface{}{
		"started_at":    now,
		"current_stage": "generating",
		"updated_at":    now,
	})
	if err != nil {
		log.Logger.Errorf("update task %s owned failed: %v", task.ID, err)
		return
	}
	if !owned {
		log.Logger.Errorf("task %s ownership lost before processing", task.ID)
		return
	}

	taskCtx, taskCancel := context.WithCancel(ctx)
	defer taskCancel()

	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	w.startHeartbeat(heartbeatCtx, task.ID, executionID, workerID, taskCancel)

	results, runErr := w.runActions(taskCtx, task)

	heartbeatCancel()

	if runErr != nil {
		log.Logger.Errorf("desktoppet task %s action scan failed; leaving task non-terminal for lease recovery: %v", task.ID, runErr)
		return
	}
	w.finalizeTask(ctx, task, results)
}

func (w *Worker) startHeartbeat(ctx context.Context, taskID, executionID, workerID string, taskCancel context.CancelFunc) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Logger.Errorf("worker heartbeat(%s) panic: %v", taskID, r)
			}
		}()
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-w.stopCh:
				return
			case <-ticker.C:
				now := time.Now().Format(workerTimeFormat)
				lease := time.Now().Add(leaseDuration).Format(workerTimeFormat)
				owned, err := w.repo.RefreshLeaseOwned(taskID, executionID, lease, now)
				if err != nil {
					log.Logger.Errorf("refresh lease task %s failed: %v", taskID, err)
					continue
				}
				if !owned {
					log.Logger.Errorf("heartbeat task %s ownership lost, stopping heartbeat", taskID)
					if _, transitionErr := w.stateEngine.Transition(ctx, taskstate.TransitionRequest{
						EntityType:    contracts.EntityGenerationTask,
						EntityID:      taskID,
						From:          []contracts.LifecycleStatus{contracts.StatusProcessing},
						To:            contracts.StatusQueued,
						Stage:         contracts.StageQueued,
						Reason:        contracts.ReasonWorkerLeaseLost,
						ActorType:     contracts.ActorWorker,
						ActorID:       workerID,
						ExecutionID:   executionID,
						NeedOwnership: true,
					}); transitionErr != nil {
						log.Logger.Errorf("heartbeat task %s lease-loss transition failed: %v", taskID, transitionErr)
					}
					taskCancel()
					return
				}
			}
		}
	}()
}

type actionResult struct {
	actionID string
	status   string
}

func (w *Worker) runActions(ctx context.Context, task *desktoppet.GenerationTask) ([]actionResult, error) {
	actions, err := w.repo.ListActionsByTaskIDOrdered(task.ID)
	if err != nil {
		return nil, fmt.Errorf("list actions for task %s: %w", task.ID, err)
	}
	results := make([]actionResult, 0, len(actions))
	totalExpectedFrames := 0
	for i := range actions {
		totalExpectedFrames += actions[i].FrameCount
	}
	completedFrames := 0
	updateTaskProgress := func() {
		if totalExpectedFrames > 0 {
			now := time.Now().Format(workerTimeFormat)
			owned, updateErr := w.repo.UpdateTaskOwned(task.ID, task.ExecutionID, map[string]interface{}{
				"progress":   completedFrames * 100 / totalExpectedFrames,
				"updated_at": now,
			})
			if updateErr != nil {
				log.Logger.Errorf("persist task %s progress failed: %v", task.ID, updateErr)
				return
			}
			if !owned {
				log.Logger.Warnf("task %s ownership lost during progress update", task.ID)
			}
		}
	}
	for i := range actions {
		action := &actions[i]
		select {
		case <-w.stopCh:
			results = append(results, actionResult{actionID: action.ID, status: "skipped"})
			continue
		case <-ctx.Done():
			results = append(results, actionResult{actionID: action.ID, status: "skipped"})
			continue
		default:
		}

		if action.Status != "pending" && action.Status != "queued" {
			results = append(results, actionResult{actionID: action.ID, status: action.Status})
			completedFrames += action.Progress * action.FrameCount / 100
			updateTaskProgress()
			continue
		}

		if w.isTaskCancelled(task.ID) {
			now := time.Now().Format(workerTimeFormat)
			if err := w.repo.UpdateActionStatusNoTx(action.ID, map[string]interface{}{
				"status":       "skipped",
				"updated_at":   now,
				"completed_at": now,
			}); err != nil {
				log.Logger.Errorf("persist skipped action %s failed: %v", action.ID, err)
				results = append(results, actionResult{actionID: action.ID, status: "persistence_failed"})
				continue
			}
			results = append(results, actionResult{actionID: action.ID, status: "skipped"})
			updateTaskProgress()
			continue
		}

		status := w.runAction(ctx, task, action)
		results = append(results, actionResult{actionID: action.ID, status: status})
		completedFrames += action.Progress * action.FrameCount / 100
		updateTaskProgress()
	}
	return results, nil
}

func (w *Worker) isTaskCancelled(taskID string) bool {
	task, err := w.repo.GetTaskByID(taskID)
	if err != nil || task == nil {
		return false
	}
	return task.CancelRequestedAt != ""
}

func (w *Worker) computeTaskProgress(taskID string) (int, bool) {
	actions, err := w.repo.ListActionsByTaskIDOrdered(taskID)
	if err != nil {
		return 0, false
	}
	totalExpectedFrames := 0
	completedFrames := 0
	for _, a := range actions {
		totalExpectedFrames += a.FrameCount
		completedFrames += a.Progress * a.FrameCount / 100
	}
	if totalExpectedFrames <= 0 {
		return 0, false
	}
	return completedFrames * 100 / totalExpectedFrames, true
}

func (w *Worker) runAction(ctx context.Context, task *desktoppet.GenerationTask, action *desktoppet.GenerationTaskAction) string {
	desktoppet.PublishTaskEvent(task.ID, "action.started", map[string]interface{}{
		"task_id":     task.ID,
		"action_key":  action.ActionKey,
		"action_name": action.ActionNameSnapshot,
	})

	spec, ok := specs.SpecFromJSON(action.ActionSpecJSON)
	if !ok {
		spec, ok = specs.GetSpec(action.ActionKey)
	}
	if !ok {
		return w.failAction(action, desktoppet.ErrCodeActionNotFound, "动作规格不存在: "+action.ActionKey)
	}

	resolved, errCode, errMsg := w.resolveGenerationProvider(ctx, task)
	if resolved == nil {
		return w.failAction(action, errCode, errMsg)
	}
	providerName := resolved.ProviderName
	provider := resolved.Provider
	modelConfig := resolved.ModelConfig

	if err := provider.ValidateConfig(ctx, modelConfig); err != nil {
		return w.failAction(action, desktoppet.ErrCodeImageGenerationRequestInvalid, fmt.Sprintf("生图模型校验失败: %v", err))
	}

	capabilities, err := provider.Capabilities(ctx, modelConfig)
	if err != nil {
		return w.failAction(action, desktoppet.ErrCodeImageModelCapabilityUnsupported, fmt.Sprintf("获取模型能力失败: %v", err))
	}
	if !capabilities.SupportsReferenceImage {
		return w.failAction(action, desktoppet.ErrCodeImageModelCapabilityUnsupported, "生图模型不支持参考图")
	}

	if task.GenerationPlanVersion > 0 {
		return w.executeWithGenerationExecutor(ctx, task, action, spec, resolved.ConfigID, resolved.ModelName, resolved.ConfigRevision, providerName, provider, modelConfig)
	}

	now := time.Now().Format(workerTimeFormat)
	tx := w.db.Begin()
	if tx.Error != nil {
		return w.failAction(action, desktoppet.ErrCodeGenerationWorkerError, "开启事务失败")
	}
	if err := w.repo.IncrementActionAttempt(tx, action.ID); err != nil {
		tx.Rollback()
		return w.failAction(action, desktoppet.ErrCodeGenerationWorkerError, "递增尝试次数失败")
	}
	if err := w.repo.UpdateActionStatus(tx, action.ID, map[string]interface{}{
		"status":     "running",
		"started_at": now,
		"updated_at": now,
	}); err != nil {
		tx.Rollback()
		return w.failAction(action, desktoppet.ErrCodeGenerationWorkerError, "更新动作状态失败")
	}
	if err := tx.Commit().Error; err != nil {
		return w.failAction(action, desktoppet.ErrCodeGenerationWorkerError, "提交动作事务失败")
	}

	actionAttempt := action.AttemptNumber + 1

	frames := make([]desktoppet.GenerationFrame, 0, spec.FrameCount)
	frameTime := time.Now().Format(workerTimeFormat)
	for i := 0; i < spec.FrameCount; i++ {
		prompt := desktoppet.BuildFramePrompt(spec, i, task.Prompt)
		negPrompt := desktoppet.BuildNegativePrompt(spec, task.NegativePrompt)
		frames = append(frames, desktoppet.GenerationFrame{
			ID:                     uuid.New().String(),
			TaskID:                 task.ID,
			TaskActionID:           action.ID,
			ExecutionID:            task.ExecutionID,
			FrameIndex:             i,
			FramePhase:             framePhaseDescription(spec, i),
			Status:                 "pending",
			AttemptNumber:          0,
			GenerationAttempt:      actionAttempt,
			ProviderAttempt:        0,
			PromptSnapshot:         prompt,
			NegativePromptSnapshot: negPrompt,
			Provider:               providerName,
			Model:                  resolved.ModelName,
			CreatedAt:              frameTime,
			UpdatedAt:              frameTime,
		})
	}
	createTx := w.db.Begin()
	if createTx.Error != nil {
		return w.failAction(action, desktoppet.ErrCodeGenerationWorkerError, "创建帧事务失败")
	}
	if err := w.repo.EnsureGenerationFrames(createTx, frames); err != nil {
		createTx.Rollback()
		return w.failAction(action, desktoppet.ErrCodeGenerationWorkerError, fmt.Sprintf("创建帧失败: %v", err))
	}
	if err := createTx.Commit().Error; err != nil {
		return w.failAction(action, desktoppet.ErrCodeGenerationWorkerError, "提交帧事务失败")
	}

	successCount := 0
	failCount := 0
	previousFramePath := ""
	updateActionProgress := func() {
		if spec.FrameCount <= 0 {
			return
		}
		progress := (successCount + failCount) * 100 / spec.FrameCount
		action.Progress = progress
		progressNow := time.Now().Format(workerTimeFormat)
		if err := w.repo.UpdateActionStatusNoTx(action.ID, map[string]interface{}{
			"progress":   progress,
			"updated_at": progressNow,
		}); err != nil {
			log.Logger.Errorf("persist action %s progress failed: %v", action.ID, err)
			return
		}
		desktoppet.PublishTaskEvent(task.ID, "action.progress", map[string]interface{}{
			"task_id":     task.ID,
			"action_key":  action.ActionKey,
			"action_id":   action.ID,
			"progress":    progress,
			"frame_done":  successCount + failCount,
			"frame_total": spec.FrameCount,
		})
		if taskProgress, ok := w.computeTaskProgress(task.ID); ok {
			owned, updateErr := w.repo.UpdateTaskOwned(task.ID, task.ExecutionID, map[string]interface{}{
				"progress":   taskProgress,
				"updated_at": progressNow,
			})
			if updateErr != nil {
				log.Logger.Errorf("persist task %s progress failed: %v", task.ID, updateErr)
				return
			}
			if !owned {
				log.Logger.Warnf("task %s ownership lost during action progress", task.ID)
				return
			}
			desktoppet.PublishTaskEvent(task.ID, "task.progress", map[string]interface{}{
				"task_id":  task.ID,
				"progress": taskProgress,
			})
		}
	}
	for i := range frames {
		frame := &frames[i]
		select {
		case <-w.stopCh:
			skipNow := time.Now().Format(workerTimeFormat)
			if err := w.repo.UpdateFrame(w.db, frame.ID, map[string]interface{}{
				"status":       "skipped",
				"updated_at":   skipNow,
				"completed_at": skipNow,
			}); err != nil {
				log.Logger.Errorf("persist skipped frame %s failed: %v", frame.ID, err)
				return "persistence_failed"
			}
			failCount++
			updateActionProgress()
			continue
		case <-ctx.Done():
			skipNow := time.Now().Format(workerTimeFormat)
			if err := w.repo.UpdateFrame(w.db, frame.ID, map[string]interface{}{
				"status":       "skipped",
				"updated_at":   skipNow,
				"completed_at": skipNow,
			}); err != nil {
				log.Logger.Errorf("persist cancelled frame %s failed: %v", frame.ID, err)
				return "persistence_failed"
			}
			failCount++
			updateActionProgress()
			continue
		default:
		}

		if w.isTaskCancelled(task.ID) {
			skipNow := time.Now().Format(workerTimeFormat)
			if err := w.repo.UpdateFrame(w.db, frame.ID, map[string]interface{}{
				"status":       "skipped",
				"updated_at":   skipNow,
				"completed_at": skipNow,
			}); err != nil {
				log.Logger.Errorf("persist task-cancelled frame %s failed: %v", frame.ID, err)
				return "persistence_failed"
			}
			failCount++
			updateActionProgress()
			continue
		}

		status := w.runFrame(ctx, task, action, actionAttempt, spec, frame, previousFramePath, modelConfig, provider)
		if status == "persistence_failed" {
			return status
		}
		if status == "succeeded" {
			successCount++
			previousFramePath = frame.ResultImagePath
		} else {
			failCount++
		}
		updateActionProgress()
	}

	actionStatus := "succeeded"
	if failCount > 0 && successCount > 0 {
		actionStatus = "partially_succeeded"
	} else if failCount > 0 && successCount == 0 {
		actionStatus = "failed"
	}

	now = time.Now().Format(workerTimeFormat)
	progress := 0
	if spec.FrameCount > 0 {
		progress = (successCount + failCount) * 100 / spec.FrameCount
	}
	action.Progress = progress
	if err := w.repo.UpdateActionStatusNoTx(action.ID, map[string]interface{}{
		"status":       actionStatus,
		"progress":     progress,
		"completed_at": now,
		"updated_at":   now,
	}); err != nil {
		log.Logger.Errorf("persist terminal state for action %s failed: %v", action.ID, err)
		return "persistence_failed"
	}
	action.Status = actionStatus
	desktoppet.PublishTaskEvent(task.ID, "action.completed", map[string]interface{}{
		"task_id":         task.ID,
		"action_key":      action.ActionKey,
		"status":          actionStatus,
		"frame_succeeded": successCount,
		"frame_failed":    failCount,
		"frame_total":     spec.FrameCount,
	})

	if err := w.downloader.WriteMetadata(task.ID, action.ActionKey, actionAttempt, map[string]interface{}{
		"provider":     providerName,
		"model":        resolved.ModelName,
		"frameCount":   spec.FrameCount,
		"successCount": successCount,
		"failCount":    failCount,
		"outputWidth":  task.OutputWidth,
		"outputHeight": task.OutputHeight,
		"specVersion":  spec.Version,
	}); err != nil {
		log.Logger.Warnf("desktoppet action %s metadata write failed after durable completion: %v", action.ID, err)
	}

	log.Logger.Infof("desktoppet action %s (task=%s) finished: status=%s success=%d fail=%d", action.ID, task.ID, actionStatus, successCount, failCount)
	return actionStatus
}

func (w *Worker) executeWithGenerationExecutor(
	ctx context.Context,
	task *desktoppet.GenerationTask,
	action *desktoppet.GenerationTaskAction,
	spec specs.ActionGenerationSpec,
	configID int,
	modelName string,
	configRevision string,
	providerName string,
	provider imageprovider.ImageGenerationProvider,
	modelConfig imageprovider.ImageModelConfig,
) string {
	layoutPlanner := generationlayout.DefaultPlanner(spec.FrameCount)
	layoutResult, err := layoutPlanner.Plan()
	if err != nil {
		return w.failAction(action, desktoppet.ErrCodeGenerationWorkerError, fmt.Sprintf("布局规划失败: %v", err))
	}

	framePhases := make([]generationprompt.FramePhaseInput, len(spec.FramePhases))
	for i, p := range spec.FramePhases {
		framePhases[i] = generationprompt.FramePhaseInput{Index: p.Index, Description: p.Description}
	}

	rows, columns := generationlayout.RecommendGrid(spec.FrameCount)
	promptDoc := generationprompt.PromptDocument{
		SchemaVersion:          1,
		CharacterIdentity:      action.ActionNameSnapshot,
		ArtStyle:               "动漫风格",
		CameraConstraint:       spec.CameraConstraint,
		PoseConstraint:         spec.PoseConstraint,
		MotionDescription:      spec.MotionDescription,
		ContinuityConstraint:   spec.ContinuityConstraint,
		UserPrompt:             task.Prompt,
		PromptFragment:         spec.PromptFragment,
		NegativePromptFragment: spec.NegativePromptFragment,
		FramePhases:            framePhases,
		GridLayout: generationprompt.GridLayoutInput{
			Rows:         rows,
			Columns:      columns,
			CellCount:    spec.FrameCount,
			ReadingOrder: "从左到右，从上到下",
		},
		BackgroundStrategy: "透明背景",
		UserNegativePrompt: task.NegativePrompt,
	}

	promptSnapshot, err := generationprompt.BuildSheetPrompt(promptDoc)
	if err != nil {
		return w.failAction(action, desktoppet.ErrCodeGenerationWorkerError, fmt.Sprintf("Prompt 构建失败: %v", err))
	}

	var caps imageprovider.ProviderCapabilities
	if extProvider, ok := provider.(imageprovider.ExtendedProvider); ok {
		caps, err = extProvider.ExtendedCapabilities(ctx, modelConfig)
		if err != nil {
			return w.failAction(action, desktoppet.ErrCodeImageModelCapabilityUnsupported, fmt.Sprintf("获取扩展能力失败: %v", err))
		}
	} else {
		basicCaps, err := provider.Capabilities(ctx, modelConfig)
		if err != nil {
			return w.failAction(action, desktoppet.ErrCodeImageModelCapabilityUnsupported, fmt.Sprintf("获取能力失败: %v", err))
		}
		caps = imageprovider.ProviderCapabilities{
			Provider:               providerName,
			Model:                  modelName,
			SupportedModes:         []imageprovider.GenerationMode{imageprovider.ModeSpriteSheet, imageprovider.ModeLegacyFrame},
			SupportsReferenceImage: basicCaps.SupportsReferenceImage,
			SupportsMultipleImages: basicCaps.SupportsMultipleImages,
			SupportsNegativePrompt: basicCaps.SupportsNegativePrompt,
			SupportsSeed:           basicCaps.SupportsSeed,
			SupportsAsyncOperation: basicCaps.SupportsAsyncOperation,
			SupportsCancellation:   basicCaps.SupportsCancellation,
			MaxReferenceImages:     basicCaps.MaxReferenceImages,
			MaxOutputImages:        basicCaps.MaxOutputImages,
		}
	}

	capabilityHash := computeSHA256Hex(fmt.Sprintf("%s|%s|%s", providerName, modelName, caps.CapabilityVersion))

	now := time.Now().Format(workerTimeFormat)
	runTx := w.db.Begin()
	if runTx.Error != nil {
		return w.failAction(action, desktoppet.ErrCodeGenerationWorkerError, "开启事务失败")
	}
	if err := w.repo.IncrementActionAttempt(runTx, action.ID); err != nil {
		runTx.Rollback()
		return w.failAction(action, desktoppet.ErrCodeGenerationWorkerError, "递增尝试次数失败")
	}
	if err := w.repo.UpdateActionStatus(runTx, action.ID, map[string]interface{}{
		"status":     "running",
		"started_at": now,
		"updated_at": now,
	}); err != nil {
		runTx.Rollback()
		return w.failAction(action, desktoppet.ErrCodeGenerationWorkerError, "更新动作状态失败")
	}
	if err := runTx.Commit().Error; err != nil {
		return w.failAction(action, desktoppet.ErrCodeGenerationWorkerError, "提交动作事务失败")
	}

	planner := generation.NewActionGenerationPlanner(action.ActionKey, spec.FrameCount).
		WithMode(string(imageprovider.ModeSpriteSheet)).
		WithProvider(providerName).
		WithModel(modelName).
		WithCapabilities(caps).
		WithCapabilityHash(capabilityHash).
		WithLayoutResult(layoutResult).
		WithPromptSnapshot(&promptSnapshot).
		WithSeedPolicy(generation.SeedPolicyRandom).
		WithOutputCount(1).
		WithConfig(configID, configRevision)

	plan, err := planner.Plan()
	if err != nil {
		return w.failAction(action, desktoppet.ErrCodeGenerationWorkerError, fmt.Sprintf("生成计划失败: %v", err))
	}

	execErr := w.genExecutor.Execute(ctx, task, action, plan, providerName, modelName, provider, modelConfig)
	if execErr != nil {
		log.Logger.Errorf("desktoppet generation executor failed: task=%s action=%s: %v", task.ID, action.ID, execErr)
		if errors.Is(execErr, errGenerationPersistence) || errors.Is(execErr, errGenerationRecoveryPending) || errors.Is(execErr, context.Canceled) || errors.Is(execErr, context.DeadlineExceeded) {
			return "persistence_failed"
		}
		return "failed"
	}

	log.Logger.Infof("desktoppet action %s (task=%s) completed via generation executor", action.ID, task.ID)
	return "succeeded"
}

func (w *Worker) runFrame(ctx context.Context, task *desktoppet.GenerationTask, action *desktoppet.GenerationTaskAction, actionAttempt int, spec specs.ActionGenerationSpec, frame *desktoppet.GenerationFrame, previousFramePath string, modelConfig imageprovider.ImageModelConfig, provider imageprovider.ImageGenerationProvider) string {
	now := time.Now().Format(workerTimeFormat)
	updates := map[string]interface{}{
		"status":     "running",
		"started_at": now,
		"updated_at": now,
	}
	if previousFramePath != "" {
		updates["previous_frame_path"] = previousFramePath
	}
	if err := w.repo.UpdateFrame(w.db, frame.ID, updates); err != nil {
		log.Logger.Errorf("persist frame %s running state failed: %v", frame.ID, err)
		return "persistence_failed"
	}
	desktoppet.PublishTaskEvent(task.ID, "frame.started", map[string]interface{}{
		"task_id":     task.ID,
		"action_key":  action.ActionKey,
		"frame_index": frame.FrameIndex,
		"frame_phase": frame.FramePhase,
	})

	sourceAbsPath := filepath.Join(config.AppCfg.Storage.DataDir, task.SourceImagePath)
	prevAbsPath := ""
	if previousFramePath != "" {
		prevAbsPath = filepath.Join(config.AppCfg.Storage.DataDir, previousFramePath)
	}
	referenceImages := desktoppet.SelectReferenceImages(sourceAbsPath, prevAbsPath, false)

	request := imageprovider.ImageGenerationRequest{
		Prompt:          frame.PromptSnapshot,
		NegativePrompt:  frame.NegativePromptSnapshot,
		ReferenceImages: referenceImages,
		Width:           task.OutputWidth,
		Height:          task.OutputHeight,
		OutputCount:     1,
	}

	frameProviderName := imageprovider.NormalizeProviderName(modelConfig.ApiType)
	if frameProviderName == "" {
		frameProviderName = "seedream"
	}
	callLogID := uuid.New().String()
	callLogStartedAt := time.Now().Format(workerTimeFormat)
	if err := w.repo.CreateCallLog(&desktoppet.GenerationCallLog{
		ID:               callLogID,
		TaskID:           task.ID,
		TaskActionID:     action.ID,
		FrameID:          frame.ID,
		ExecutionID:      task.ExecutionID,
		Provider:         frameProviderName,
		Model:            modelConfig.ModelName,
		RequestStartedAt: callLogStartedAt,
		RequestStatus:    "submitted",
		AttemptNumber:    actionAttempt,
	}); err != nil {
		log.Logger.Warnf("create generation call log %s failed: %v", callLogID, err)
	}

	result, errCode, errMsg := w.submitWithRetry(ctx, provider, modelConfig, request, frame)
	if result == nil {
		if !w.markFrameFailed(task, action, frame, errCode, errMsg) {
			return "persistence_failed"
		}
		w.finalizeCallLog(callLogID, "failed", time.Now().Format(workerTimeFormat), "unknown", errCode, errMsg, "")
		return "failed"
	}

	if len(result.Images) == 0 {
		if !w.markFrameFailed(task, action, frame, desktoppet.ErrCodeImageGenerationEmptyResult, "未返回任何图片") {
			return "persistence_failed"
		}
		w.finalizeCallLog(callLogID, "failed", time.Now().Format(workerTimeFormat), serializeUsage(result.Usage), desktoppet.ErrCodeImageGenerationEmptyResult, "未返回任何图片", result.RequestID)
		return "failed"
	}

	img := result.Images[0]
	path, width, height, size, hash, err := w.downloader.DownloadAndSave(img.Bytes, img.MimeType, task.ID, action.ActionKey, actionAttempt, frame.FrameIndex)
	if err != nil {
		if !w.markFrameFailed(task, action, frame, desktoppet.ErrCodeImageResultSaveFailed, err.Error()) {
			return "persistence_failed"
		}
		w.finalizeCallLog(callLogID, "failed", time.Now().Format(workerTimeFormat), serializeUsage(result.Usage), desktoppet.ErrCodeImageResultSaveFailed, err.Error(), result.RequestID)
		return "failed"
	}

	now = time.Now().Format(workerTimeFormat)
	if err := w.repo.UpdateFrame(w.db, frame.ID, map[string]interface{}{
		"status":                "succeeded",
		"result_image_path":     path,
		"result_mime_type":      img.MimeType,
		"result_width":          width,
		"result_height":         height,
		"result_size":           size,
		"result_hash":           hash,
		"provider":              result.Provider,
		"model":                 result.Model,
		"provider_request_id":   result.RequestID,
		"provider_operation_id": result.OperationID,
		"completed_at":          now,
		"updated_at":            now,
	}); err != nil {
		log.Logger.Errorf("persist terminal success for frame %s failed: %v", frame.ID, err)
		return "persistence_failed"
	}
	w.finalizeCallLog(callLogID, "succeeded", now, serializeUsage(result.Usage), "", "", result.RequestID)
	frame.ResultImagePath = path
	desktoppet.PublishTaskEvent(task.ID, "frame.succeeded", map[string]interface{}{
		"task_id":     task.ID,
		"action_key":  action.ActionKey,
		"frame_index": frame.FrameIndex,
	})
	return "succeeded"
}

func (w *Worker) submitWithRetry(ctx context.Context, provider imageprovider.ImageGenerationProvider, cfg imageprovider.ImageModelConfig, req imageprovider.ImageGenerationRequest, frame *desktoppet.GenerationFrame) (*imageprovider.ImageGenerationResult, string, string) {
	var lastErrCode string
	var lastErrMsg string

	for attempt := 0; attempt <= maxFrameRetries; attempt++ {
		if attempt > 0 {
			if err := w.repo.UpdateFrame(w.db, frame.ID, map[string]interface{}{
				"provider_attempt": attempt,
			}); err != nil {
				log.Logger.Errorf("persist provider attempt for frame %s failed: %v", frame.ID, err)
				return nil, desktoppet.ErrCodeGenerationWorkerError, "持久化 provider attempt 失败"
			}
			backoff := time.Duration(attempt) * time.Second
			select {
			case <-time.After(backoff):
			case <-w.stopCh:
				return nil, desktoppet.ErrCodeGenerationWorkerError, "worker 已停止"
			case <-ctx.Done():
				return nil, desktoppet.ErrCodeGenerationWorkerError, "任务已取消"
			}
		}

		submission, err := provider.Submit(ctx, cfg, req)
		if err != nil {
			lastErrCode = errorCodeOf(err)
			lastErrMsg = err.Error()
			if lastErrCode == "" {
				lastErrCode = desktoppet.ErrCodeImageGenerationProviderRejected
			}
			if isNonRetriableError(lastErrCode) {
				return nil, lastErrCode, lastErrMsg
			}
			if isTransientError(lastErrCode) && attempt < maxFrameRetries {
				log.Logger.Warnf("desktoppet frame %s submit failed (attempt %d/%d): %s, retrying", frame.ID, attempt+1, maxFrameRetries+1, lastErrCode)
				continue
			}
			return nil, lastErrCode, lastErrMsg
		}

		if submission == nil || submission.Result == nil {
			lastErrCode = desktoppet.ErrCodeImageGenerationEmptyResult
			lastErrMsg = "提供者返回空结果"
			if attempt < maxFrameRetries {
				log.Logger.Warnf("desktoppet frame %s submit returned nil result (attempt %d/%d), retrying", frame.ID, attempt+1, maxFrameRetries+1)
				continue
			}
			return nil, lastErrCode, lastErrMsg
		}

		result := submission.Result
		if result.Status != "succeeded" {
			lastErrCode = result.ErrorCode
			if lastErrCode == "" {
				lastErrCode = desktoppet.ErrCodeImageGenerationProviderRejected
			}
			lastErrMsg = result.ErrorMessage
			if lastErrMsg == "" {
				lastErrMsg = fmt.Sprintf("提供者返回状态: %s", result.Status)
			}
			if isNonRetriableError(lastErrCode) {
				return nil, lastErrCode, lastErrMsg
			}
			if isTransientError(lastErrCode) && attempt < maxFrameRetries {
				log.Logger.Warnf("desktoppet frame %s submit non-success (attempt %d/%d): %s, retrying", frame.ID, attempt+1, maxFrameRetries+1, lastErrCode)
				continue
			}
			return nil, lastErrCode, lastErrMsg
		}

		return result, "", ""
	}

	if lastErrCode == "" {
		lastErrCode = desktoppet.ErrCodeImageGenerationProviderRejected
		lastErrMsg = "提交失败且无具体错误"
	}
	return nil, lastErrCode, lastErrMsg
}

func (w *Worker) finalizeTask(ctx context.Context, task *desktoppet.GenerationTask, results []actionResult) {
	for _, r := range results {
		if r.status == "persistence_failed" {
			log.Logger.Errorf("desktoppet task %s has an unpersisted child terminal state; leaving task non-terminal for lease recovery", task.ID)
			return
		}
	}

	successCount := 0
	failedCount := 0
	skippedCount := 0
	for _, r := range results {
		switch r.status {
		case "succeeded", "partially_succeeded":
			successCount++
		case "failed":
			failedCount++
		case "skipped":
			skippedCount++
		}
	}

	currentTask, _ := w.repo.GetTaskByID(task.ID)
	cancelled := currentTask != nil && currentTask.CancelRequestedAt != ""

	finalProgress := 100
	actions, err := w.repo.ListActionsByTaskIDOrdered(task.ID)
	if err == nil {
		totalExpectedFrames := 0
		completedFrames := 0
		for _, a := range actions {
			totalExpectedFrames += a.FrameCount
			completedFrames += a.Progress * a.FrameCount / 100
		}
		if totalExpectedFrames > 0 {
			finalProgress = completedFrames * 100 / totalExpectedFrames
		}
	}

	snap := taskstate.GenerationSnapshot{
		TaskStatus:                  contracts.StatusProcessing,
		CancelRequested:             cancelled,
		HasActiveChildren:           false,
		AllRequiredActionsSucceeded: failedCount == 0 && skippedCount == 0 && successCount > 0,
		AllSucceededArtifactsValid:  true,
		HasAtLeastOneCompleteAction: successCount > 0,
		AllowPartialResult:          true,
		ActualProgress:              finalProgress,
	}
	decision := taskstate.AggregateGenerationTask(snap)

	if decision.Status == contracts.StatusProcessing || decision.Status == contracts.StatusCancelling {
		log.Logger.Warnf("desktoppet task %s aggregate returned non-terminal status %s, skip finalization", task.ID, decision.Status)
		return
	}

	fromStatus := contracts.StatusProcessing
	if currentTask != nil && currentTask.Status == "cancelling" {
		fromStatus = contracts.StatusCancelling
	}

	req := taskstate.TransitionRequest{
		EntityType:    contracts.EntityGenerationTask,
		EntityID:      task.ID,
		From:          []contracts.LifecycleStatus{fromStatus},
		To:            decision.Status,
		Stage:         decision.Stage,
		Reason:        decision.Reason,
		Progress:      &decision.Progress,
		ActorType:     contracts.ActorFinalizer,
		ActorID:       task.WorkerID,
		ExecutionID:   task.ExecutionID,
		NeedOwnership: true,
	}
	if decision.Status == contracts.StatusFailed {
		req.ErrorCode = desktoppet.ErrCodeGenerationWorkerError
		req.ErrorMessage = "所有动作均失败"
		req.FailureStage = decision.FailureStage
	}

	_, tErr := w.stateEngine.Transition(ctx, req)
	if tErr != nil {
		log.Logger.Errorf("finalize task %s engine transition failed: %v", task.ID, tErr)
		return
	}

	taskStatus := string(decision.Status)
	log.Logger.Infof("desktoppet task %s finalized: status=%s success=%d failed=%d skipped=%d", task.ID, taskStatus, successCount, failedCount, skippedCount)
	desktoppet.PublishTaskEvent(task.ID, "task.completed", map[string]interface{}{
		"task_id":                task.ID,
		"status":                 taskStatus,
		"progress":               finalProgress,
		"succeeded_action_count": successCount,
		"failed_action_count":    failedCount,
	})
}

func (w *Worker) RecoverOnStartup(ctx context.Context) {
	w.recoverExpiredTasks(ctx)
	w.lastRecoveryScan = time.Now()
}

func (w *Worker) recoverExpiredTasks(ctx context.Context) {
	tasks, err := w.repo.ListRecoverableTasks()
	if err != nil {
		log.Logger.Errorf("desktoppet recover list tasks failed: %v", err)
		return
	}
	recovered := 0
	deferred := 0
	for i := range tasks {
		task := &tasks[i]
		if task.GenerationPlanVersion > 0 {
			hasRecoverableAttempt, checkErr := w.taskHasRecoverableGenerationAttempts(task.ID)
			if checkErr != nil {
				log.Logger.Errorf("desktoppet inspect recoverable attempts for task %s failed: %v", task.ID, checkErr)
				continue
			}
			if hasRecoverableAttempt {
				deferred++
				continue
			}
		}

		now := time.Now().Format(workerTimeFormat)
		applied := false
		err := w.db.Transaction(func(tx *gorm.DB) error {
			result := tx.Model(&desktoppet.GenerationTask{}).
				Where("id = ? AND execution_id = ? AND lease_expires_at = ? AND status = ?", task.ID, task.ExecutionID, task.LeaseExpiresAt, "processing").
				Updates(map[string]interface{}{
					"status":             "queued",
					"current_stage":      "queued",
					"status_reason":      string(contracts.ReasonSystemLeaseExpired),
					"worker_id":          "",
					"execution_id":       "",
					"lease_expires_at":   "",
					"last_heartbeat_at":  "",
					"completed_at":       "",
					"error_code":         "",
					"error_message":      "",
					"last_transition_at": now,
					"updated_at":         now,
					"row_version":        gorm.Expr("row_version + 1"),
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return nil
			}
			if err := tx.Model(&desktoppet.GenerationTaskAction{}).
				Where("task_id = ? AND status = ?", task.ID, "running").
				Updates(map[string]interface{}{
					"status":        "pending",
					"progress":      0,
					"error_code":    "",
					"error_message": "",
					"started_at":    "",
					"completed_at":  "",
					"updated_at":    now,
				}).Error; err != nil {
				return err
			}
			if err := tx.Model(&desktoppet.GenerationFrame{}).
				Where("task_id = ? AND status = ?", task.ID, "running").
				Updates(map[string]interface{}{
					"status":        "pending",
					"error_code":    "",
					"error_message": "",
					"started_at":    "",
					"completed_at":  "",
					"updated_at":    now,
				}).Error; err != nil {
				return err
			}
			applied = true
			return nil
		})
		if err != nil {
			log.Logger.Errorf("desktoppet recover task %s transaction failed: %v", task.ID, err)
			continue
		}
		if !applied {
			continue
		}
		recovered++
		if err := w.stateStore.WriteAudit(ctx, taskstate.AuditRecord{
			ID:          taskstate.NewAuditID(),
			EntityType:  contracts.EntityGenerationTask,
			EntityID:    task.ID,
			FromStatus:  contracts.StatusProcessing,
			ToStatus:    contracts.StatusQueued,
			ToStage:     contracts.StageQueued,
			ReasonCode:  contracts.ReasonSystemLeaseExpired,
			ActorType:   contracts.ActorRecovery,
			ActorID:     "system",
			ExecutionID: task.ExecutionID,
			CreatedAt:   now,
		}); err != nil {
			log.Logger.Errorf("desktoppet recover task %s audit write failed: %v", task.ID, err)
		}
		log.Logger.Infof("desktoppet recovered expired task %s and its running children atomically", task.ID)
	}
	if recovered > 0 || deferred > 0 {
		log.Logger.Infof("desktoppet recovery scan complete: recovered=%d deferred_to_generation_recovery=%d", recovered, deferred)
	}
}

func (w *Worker) taskHasRecoverableGenerationAttempts(taskID string) (bool, error) {
	actions, err := w.repo.ListActionsByTaskIDOrdered(taskID)
	if err != nil {
		return false, err
	}
	for i := range actions {
		action := &actions[i]
		if action.Status != "running" {
			continue
		}
		attempts, err := w.attemptRepo.ListAttemptsByActionID(action.ID)
		if err != nil {
			return false, err
		}
		for j := len(attempts) - 1; j >= 0; j-- {
			attempt := &attempts[j]
			if action.CurrentAttempt > 0 && attempt.AttemptNumber != action.CurrentAttempt {
				continue
			}
			if attempt.IsActive() || attempt.IsTerminal() {
				return true, nil
			}
		}
	}
	return false, nil
}

func (w *Worker) onGenerationAttemptRecovered(ctx context.Context, attempt *generation.ActionGenerationAttempt) error {
	if attempt == nil || attempt.TaskID == "" {
		return nil
	}
	task, err := w.repo.GetTaskByID(attempt.TaskID)
	if err != nil {
		return fmt.Errorf("load recovered generation task %s: %w", attempt.TaskID, err)
	}
	if task == nil || task.Status != "processing" || task.ExecutionID != attempt.ExecutionID {
		return nil
	}

	actions, err := w.repo.ListActionsByTaskIDOrdered(task.ID)
	if err != nil {
		return fmt.Errorf("list actions before recovered task requeue: %w", err)
	}
	for i := range actions {
		if actions[i].Status == "running" {
			return nil
		}
	}

	_, err = w.stateEngine.Transition(ctx, taskstate.TransitionRequest{
		EntityType:    contracts.EntityGenerationTask,
		EntityID:      task.ID,
		From:          []contracts.LifecycleStatus{contracts.StatusProcessing},
		To:            contracts.StatusQueued,
		Stage:         contracts.StageQueued,
		Reason:        contracts.ReasonSystemRecovered,
		ActorType:     contracts.ActorRecovery,
		ActorID:       "generation-recovery-worker",
		ExecutionID:   attempt.ExecutionID,
		NeedOwnership: true,
		AttemptID:     attempt.ID,
	})
	if err != nil {
		latest, loadErr := w.repo.GetTaskByID(task.ID)
		if loadErr == nil && latest != nil && latest.Status != "processing" {
			return nil
		}
		return fmt.Errorf("requeue recovered generation task %s: %w", task.ID, err)
	}
	log.Logger.Infof("generation recovery converged task %s after attempt %s; task requeued", task.ID, attempt.ID)
	return nil
}

func (w *Worker) markFrameFailed(task *desktoppet.GenerationTask, action *desktoppet.GenerationTaskAction, frame *desktoppet.GenerationFrame, code, message string) bool {
	now := time.Now().Format(workerTimeFormat)
	if err := w.repo.UpdateFrame(w.db, frame.ID, map[string]interface{}{
		"status":        "failed",
		"error_code":    code,
		"error_message": message,
		"completed_at":  now,
		"updated_at":    now,
	}); err != nil {
		log.Logger.Errorf("persist terminal failure for frame %s failed: %v", frame.ID, err)
		return false
	}
	desktoppet.PublishTaskEvent(task.ID, "frame.failed", map[string]interface{}{
		"task_id":       task.ID,
		"action_key":    action.ActionKey,
		"frame_index":   frame.FrameIndex,
		"error_code":    code,
		"error_message": message,
	})
	return true
}

func (w *Worker) failAction(action *desktoppet.GenerationTaskAction, code, message string) string {
	now := time.Now().Format(workerTimeFormat)
	if err := w.repo.UpdateActionStatusNoTx(action.ID, map[string]interface{}{
		"status":        "failed",
		"error_code":    code,
		"error_message": message,
		"completed_at":  now,
		"updated_at":    now,
	}); err != nil {
		log.Logger.Errorf("failed to persist terminal failure for action %s: %v", action.ID, err)
		return "persistence_failed"
	}
	action.Status = "failed"
	return "failed"
}

func (w *Worker) finalizeCallLog(logID, status, completedAt, usage, errCode, errMsg, providerRequestID string) {
	if err := w.db.Model(&desktoppet.GenerationCallLog{}).Where("id = ?", logID).Updates(map[string]interface{}{
		"request_completed_at": completedAt,
		"request_status":       status,
		"usage":                usage,
		"error_code":           errCode,
		"error_message":        errMsg,
		"provider_request_id":  providerRequestID,
	}).Error; err != nil {
		log.Logger.Warnf("failed to finalize call log %s: %v", logID, err)
	}
}

func serializeUsage(usage *imageprovider.GenerationUsage) string {
	if usage == nil {
		return "unknown"
	}
	data, err := json.Marshal(usage)
	if err != nil {
		return "unknown"
	}
	return string(data)
}

func isTransientError(code string) bool {
	switch code {
	case desktoppet.ErrCodeImageGenerationTimeout,
		desktoppet.ErrCodeImageGenerationRateLimited,
		desktoppet.ErrCodeImageGenerationProviderRejected,
		desktoppet.ErrCodeImageResultDownloadFailed:
		return true
	default:
		return false
	}
}

func isNonRetriableError(code string) bool {
	switch code {
	case desktoppet.ErrCodeImageGenerationAuthFailed,
		desktoppet.ErrCodeImageGenerationRequestInvalid,
		desktoppet.ErrCodeImageModelCapabilityUnsupported,
		desktoppet.ErrCodeImageModelCredentialMissing:
		return true
	default:
		return false
	}
}

type codedError interface {
	ErrorCode() string
}

func errorCodeOf(err error) string {
	var ce codedError
	if errors.As(err, &ce) {
		return ce.ErrorCode()
	}
	return ""
}

func computeSHA256Hex(data string) string {
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

func framePhaseDescription(spec specs.ActionGenerationSpec, frameIndex int) string {
	if len(spec.FramePhases) == 0 {
		return ""
	}
	if frameIndex < 0 || frameIndex >= len(spec.FramePhases) {
		return strings.TrimSpace(spec.FramePhases[len(spec.FramePhases)-1].Description)
	}
	return strings.TrimSpace(spec.FramePhases[frameIndex].Description)
}
