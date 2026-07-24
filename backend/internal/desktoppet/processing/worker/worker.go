// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval/local"
	"github.com/u-ai/backend/log"
	_ "golang.org/x/image/webp"
	"gorm.io/gorm"
)

const (
	processingLeaseDuration     = 10 * time.Minute
	processingHeartbeatInterval = 30 * time.Second
	processingPollInterval      = 5 * time.Second
	processingWorkerID          = "processing-worker-1"
	processingTimeFormat        = "2006-01-02 15:04:05"
	defaultBackgroundMode       = "remove_background"
	defaultLoopType             = "loop"
	maxConcurrentProcessingTasks = 2
)

type Worker struct {
	db               *gorm.DB
	repo             processing.Repository
	validator        *processing.Validator
	bgRegistry       backgroundremoval.Registry
	subjectDetector  *processing.SubjectDetector
	driftDetector    *processing.DriftDetector
	qualityChecker   *processing.QualityChecker
	loopChecker      *processing.LoopChecker
	resourceWriter   *processing.ResourceWriter
	previewGenerator *processing.PreviewGenerator
	cleanupManager   *processing.CleanupManager
	manifestBuilder  *processing.ManifestBuilder
	dataDir          string
	stopCh           chan struct{}
	wg               sync.WaitGroup
	sem              chan struct{}
}

func NewWorker(db *gorm.DB, repo processing.Repository, dataDir string) *Worker {
	bgRegistry := backgroundremoval.NewRegistry()
	bgRegistry.Register(local.NewLocalProvider())

	return &Worker{
		db:               db,
		repo:             repo,
		validator:        processing.NewValidator(repo, dataDir),
		bgRegistry:       bgRegistry,
		subjectDetector:  processing.NewSubjectDetector(),
		driftDetector:    processing.NewDriftDetector(),
		qualityChecker:   processing.NewQualityChecker(),
		loopChecker:      processing.NewLoopChecker(),
		resourceWriter:   processing.NewResourceWriter(dataDir),
		previewGenerator: processing.NewPreviewGenerator(dataDir),
		cleanupManager:   processing.NewCleanupManager(dataDir),
		manifestBuilder:  processing.NewManifestBuilder(dataDir),
		dataDir:          dataDir,
		stopCh:           make(chan struct{}),
		sem:              make(chan struct{}, maxConcurrentProcessingTasks),
	}
}

func (w *Worker) Start(ctx context.Context) {
	w.recoverStuckTasks(ctx)
	w.wg.Add(1)
	go w.run(ctx)
}

func (w *Worker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

func (w *Worker) run(ctx context.Context) {
	defer w.wg.Done()
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

	claimed, err := w.repo.ClaimProcessingTask(task.ID, processingWorkerID, executionID, leaseExpiresAt)
	if err != nil {
		log.Logger.Errorf("claim processing task %s failed: %v", task.ID, err)
		return
	}
	if !claimed {
		return
	}
	log.Logger.Infof("processing worker claimed task %s (execution=%s)", task.ID, executionID)

	now := time.Now().Format(processingTimeFormat)
	_ = w.repo.UpdateProcessingTaskStatusNoTx(task.ID, map[string]interface{}{
		"started_at":  now,
		"updated_at":  now,
		"progress":    ProgressValidatingSources,
	})
	w.publishProgress(task.ID, ProgressValidatingSources, StageValidatingSources)

	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	w.startHeartbeat(heartbeatCtx, task.ID)
	defer heartbeatCancel()

	genTask, err := w.repo.GetGenerationTask(task.GenerationTaskID)
	if err != nil {
		log.Logger.Errorf("get generation task %s failed: %v", task.GenerationTaskID, err)
		w.finalizeTask(task.ID, err)
		return
	}

	processErr := w.runProcessingStages(ctx, task, genTask.UserID)

	w.finalizeTask(task.ID, processErr)
}

func (w *Worker) runProcessingStages(ctx context.Context, task *processing.ProcessingTask, userID string) error {
	w.updateStage(task.ID, StageValidatingSources, ProgressValidatingSources)
	source, err := w.validator.ValidateSources(task.GenerationTaskID, userID)
	if err != nil {
		return err
	}

	actions := w.createProcessingActions(task, source)
	if len(actions) == 0 {
		return fmt.Errorf("no actions to process")
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
		w.updateProgress(task.ID, taskProgress)

		action := &actions[i]
		if err := w.processAction(ctx, task, action, source); err != nil {
			log.Logger.Errorf("processing action %s failed: %v", action.ActionKey, err)
			w.failAction(action, err)
			continue
		}
	}

	if w.isCancelled(task.ID) {
		return fmt.Errorf("cancelled")
	}

	w.updateStage(task.ID, StagePreview, ProgressPreview)

	w.updateStage(task.ID, StagePackaging, ProgressManifest)
	if err := w.buildPackage(task, source); err != nil {
		log.Logger.Errorf("build package for task %s failed: %v", task.ID, err)
		return err
	}

	w.updateProgress(task.ID, ProgressPackage)
	return nil
}

func (w *Worker) createProcessingActions(task *processing.ProcessingTask, source *processing.SourceValidationResult) []processing.ProcessingAction {
	actions := make([]processing.ProcessingAction, 0, len(source.SucceededActions))
	for _, genAction := range source.SucceededActions {
		anchor := processing.DefaultAnchorForActionKey(genAction.ActionKey)
		fps := task.DefaultFPS
		if fps <= 0 {
			fps = processing.DefaultFPSForAction(genAction.ActionKey)
		}

		attempt := genAction.CurrentAttempt
		if attempt <= 0 {
			attempt = 1
		}

		frameInfos := source.FramePaths[genAction.ActionKey]

		action := processing.ProcessingAction{
			ID:                     "pa-" + uuid.New().String(),
			ProcessingTaskID:       task.ID,
			GenerationTaskActionID: genAction.ID,
			ActionKey:              genAction.ActionKey,
			ActionNameSnapshot:     genAction.ActionNameSnapshot,
			SourceAttemptNumber:    attempt,
			Status:                 "pending",
			SourceFrameCount:       len(frameInfos),
			FPS:                    fps,
			FrameDurationMS:        1000 / fps,
			AnchorType:             string(anchor.Type),
			AnchorX:                anchor.X,
			AnchorY:                anchor.Y,
			LoopType:               defaultLoopType,
			Excluded:               0,
		}
		actions = append(actions, action)
	}

	if len(actions) == 0 {
		return nil
	}
	if err := w.repo.CreateProcessingActions(w.db, actions); err != nil {
		log.Logger.Errorf("create processing actions failed: %v", err)
		return nil
	}
	return actions
}

func (w *Worker) processAction(ctx context.Context, task *processing.ProcessingTask, action *processing.ProcessingAction, source *processing.SourceValidationResult) error {
	w.publishActionEvent(task.ID, action.ActionKey, "started")

	frames, err := w.loadSourceFrames(action, source)
	if err != nil {
		return err
	}

	w.updateActionProgress(action, StageBackgroundRemoval, ProgressBackgroundRemoval)
	provider, err := w.bgRegistry.Get(backgroundremoval.BackgroundMode(task.BackgroundMode))
	if err != nil {
		mode := backgroundremoval.BackgroundMode(defaultBackgroundMode)
		provider, err = w.bgRegistry.Get(mode)
		if err != nil {
			return fmt.Errorf("get background provider failed: %w", err)
		}
	}
	processedImgs, boxes, err := w.removeBackgrounds(ctx, provider, frames, task.BackgroundMode)
	if err != nil {
		return err
	}

	w.updateActionProgress(action, StageSubjectDetection, ProgressSubjectDetection)

	w.updateActionProgress(action, StageScaling, ProgressScaling)
	maxBox := processing.MaxSubjectBox(boxes)
	scale := processing.ComputeSequenceScale(maxBox, w.canvasConfig(task))

	w.updateActionProgress(action, StageAnchor, ProgressAnchor)
	anchor := processing.DefaultAnchorForActionKey(action.ActionKey)

	w.updateActionProgress(action, StageCanvas, ProgressCanvas)
	normalizedImgs := w.normalizeFrames(processedImgs, scale, anchor, task)

	w.updateActionProgress(action, StageAlignment, ProgressAlignment)
	alignedImgs, _ := w.alignFrames(normalizedImgs, boxes, anchor)

	w.updateActionProgress(action, StageQuality, ProgressQuality)
	qualityResults := w.qualityChecker.CheckFrames(alignedImgs, boxes)

	w.updateActionProgress(action, StageLoop, ProgressLoop)
	loopResult := w.loopChecker.CheckLoop(action.ActionKey, alignedImgs, boxes)
	finalImgs := loopResult.AdjustedFrames
	if len(finalImgs) == 0 {
		finalImgs = alignedImgs
	}

	w.updateActionProgress(action, StageWriteFrames, ProgressWriteFrames)
	relPaths, err := w.resourceWriter.WriteActionFrames(task.GenerationTaskID, task.ProcessingVersion, action.ActionKey, finalImgs)
	if err != nil {
		return fmt.Errorf("write action frames failed: %w", err)
	}

	if err := w.persistProcessedFrames(task, action, source, finalImgs, relPaths, qualityResults); err != nil {
		log.Logger.Errorf("persist processed frames for action %s failed: %v", action.ActionKey, err)
	}

	w.updateActionProgress(action, StageActionJSON, ProgressActionJSON)
	fps := action.FPS
	if fps <= 0 {
		fps = task.DefaultFPS
	}
	actionJSON := processing.BuildActionJSON(action.ActionKey, action.ActionNameSnapshot, len(finalImgs), fps, anchor, action.LoopType)
	if err := w.resourceWriter.WriteActionJSON(task.GenerationTaskID, task.ProcessingVersion, actionJSON); err != nil {
		return fmt.Errorf("write action json failed: %w", err)
	}

	w.updateActionProgress(action, StagePreview, ProgressPreview)
	if _, err := w.previewGenerator.GenerateActionPreview(task.GenerationTaskID, task.ProcessingVersion, action.ActionKey, finalImgs); err != nil {
		return fmt.Errorf("generate action preview failed: %w", err)
	}

	w.succeedAction(action)
	return nil
}

func (w *Worker) loadSourceFrames(action *processing.ProcessingAction, source *processing.SourceValidationResult) ([]image.Image, error) {
	frameInfos, ok := source.FramePaths[action.ActionKey]
	if !ok || len(frameInfos) == 0 {
		return nil, fmt.Errorf("action %s has no source frames", action.ActionKey)
	}

	frames := make([]image.Image, 0, len(frameInfos))
	for _, info := range frameInfos {
		if info.AbsPath == "" {
			return nil, fmt.Errorf("frame %s has empty path", info.Frame.ID)
		}
		f, err := os.Open(info.AbsPath)
		if err != nil {
			return nil, fmt.Errorf("open frame %s failed: %w", info.Frame.ID, err)
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("decode frame %s failed: %w", info.Frame.ID, err)
		}
		frames = append(frames, img)
	}
	return frames, nil
}

func (w *Worker) removeBackgrounds(ctx context.Context, provider backgroundremoval.BackgroundRemovalProvider, frames []image.Image, modeStr string) ([]image.Image, []backgroundremoval.SubjectBox, error) {
	mode := backgroundremoval.BackgroundMode(modeStr)
	if mode == "" {
		mode = backgroundremoval.ModeRemoveBackground
	}

	processedImgs := make([]image.Image, 0, len(frames))
	boxes := make([]backgroundremoval.SubjectBox, 0, len(frames))

	for i, img := range frames {
		result, err := provider.RemoveBackground(ctx, backgroundremoval.ImageInput{
			Image: img,
			Mode:  mode,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("remove background for frame %d failed: %w", i, err)
		}
		processedImgs = append(processedImgs, result.Image)
		boxes = append(boxes, result.SubjectBox)
	}
	return processedImgs, boxes, nil
}

func (w *Worker) normalizeFrames(imgs []image.Image, scale float64, anchor processing.Anchor, task *processing.ProcessingTask) []image.Image {
	cfg := w.canvasConfig(task)
	normalized := make([]image.Image, 0, len(imgs))
	for _, img := range imgs {
		normalized = append(normalized, processing.NormalizeCanvas(img, scale, anchor.X, anchor.Y, cfg))
	}
	return normalized
}

func (w *Worker) alignFrames(imgs []image.Image, boxes []backgroundremoval.SubjectBox, anchor processing.Anchor) ([]image.Image, []processing.AlignmentResult) {
	results, err := w.driftDetector.AlignFrames(imgs, boxes, anchor)
	if err != nil {
		log.Logger.Errorf("align frames failed: %v", err)
		return imgs, nil
	}
	aligned := make([]image.Image, 0, len(results))
	for _, r := range results {
		if r.AlignedImage != nil {
			aligned = append(aligned, r.AlignedImage)
		}
	}
	if len(aligned) != len(imgs) {
		log.Logger.Errorf("aligned frame count mismatch: got %d, want %d", len(aligned), len(imgs))
		return imgs, results
	}
	return aligned, results
}

func (w *Worker) buildPackage(task *processing.ProcessingTask, source *processing.SourceValidationResult) error {
	selector := processing.NewDefaultActionSelector("")
	defaultAction, err := selector.SelectDefaultAction(source.SucceededActions)
	if err != nil {
		return fmt.Errorf("select default action failed: %w", err)
	}

	processingActions, err := w.repo.ListProcessingActions(task.ID)
	if err != nil {
		return fmt.Errorf("list processing actions failed: %w", err)
	}

	includedActions := make([]string, 0, len(processingActions))
	for _, pa := range processingActions {
		if pa.Status == "succeeded" && pa.Excluded == 0 {
			includedActions = append(includedActions, pa.ActionKey)
		}
	}

	if len(includedActions) == 0 {
		return fmt.Errorf("no succeeded actions to package")
	}

	defaultIdleFrames, err := w.loadProcessedFrames(task.GenerationTaskID, task.ProcessingVersion, defaultAction)
	if err != nil {
		return fmt.Errorf("load default idle frames failed: %w", err)
	}

	if _, err := w.previewGenerator.GeneratePackagePreview(task.GenerationTaskID, task.ProcessingVersion, defaultIdleFrames); err != nil {
		return fmt.Errorf("generate package preview failed: %w", err)
	}

	packager := processing.NewPackager(w.repo, w.dataDir)
	req := &processing.PackageBuildRequest{
		ProcessingTaskID:  task.ID,
		UserID:            source.Task.UserID,
		CharacterID:       source.Task.CharacterID,
		GenerationTaskID:  task.GenerationTaskID,
		PackageName:       source.Task.Name,
		DefaultAction:     defaultAction,
		IncludedActions:   includedActions,
		CanvasWidth:       task.OutputWidth,
		CanvasHeight:      task.OutputHeight,
		ProcessingVersion: task.ProcessingVersion,
		SucceededActions:  source.SucceededActions,
	}

	if _, err := packager.BuildPackage(req); err != nil {
		return fmt.Errorf("build package failed: %w", err)
	}

	return nil
}

func (w *Worker) loadProcessedFrames(generationTaskID string, processingVersion int, actionKey string) ([]image.Image, error) {
	dir := filepath.Join(w.dataDir, "desktop-pets", "generation-tasks", generationTaskID, "processed",
		fmt.Sprintf("version-%d", processingVersion), "actions", actionKey, "frames")

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	frames := make([]image.Image, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		f, err := os.Open(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			return nil, err
		}
		frames = append(frames, img)
	}
	return frames, nil
}

func (w *Worker) persistProcessedFrames(
	task *processing.ProcessingTask,
	action *processing.ProcessingAction,
	source *processing.SourceValidationResult,
	finalImgs []image.Image,
	relPaths []string,
	qualityResults []processing.FrameQualityResult,
) error {
	if len(finalImgs) == 0 || len(relPaths) == 0 {
		return nil
	}
	if len(finalImgs) != len(relPaths) {
		return fmt.Errorf("finalImgs count %d != relPaths count %d", len(finalImgs), len(relPaths))
	}

	frameInfos := source.FramePaths[action.ActionKey]

	actionRelDir := filepath.ToSlash(filepath.Join(
		"desktop-pets", "generation-tasks", task.GenerationTaskID,
		"processed", fmt.Sprintf("version-%d", task.ProcessingVersion),
		"actions", action.ActionKey,
	))

	now := time.Now().Format(processingTimeFormat)
	frames := make([]processing.ProcessedFrame, 0, len(finalImgs))

	for i, relPath := range relPaths {
		fullRelPath := filepath.ToSlash(filepath.Join(actionRelDir, relPath))
		absPath := filepath.Join(w.dataDir, filepath.FromSlash(fullRelPath))

		hash, err := computeFileSHA256(absPath)
		if err != nil {
			return fmt.Errorf("compute hash for frame %d failed: %w", i, err)
		}

		var sourceFrameID string
		var sourcePath string
		if i < len(frameInfos) {
			sourceFrameID = frameInfos[i].Frame.ID
			sourcePath = frameInfos[i].Frame.ResultImagePath
		}

		var qualityFlags string
		if i < len(qualityResults) && len(qualityResults[i].QualityFlags) > 0 {
			qualityFlags = strings.Join(qualityResults[i].QualityFlags, ",")
		}

		bounds := finalImgs[i].Bounds()
		width := bounds.Dx()
		height := bounds.Dy()
		if width <= 0 {
			width = task.OutputWidth
		}
		if height <= 0 {
			height = task.OutputHeight
		}

		frames = append(frames, processing.ProcessedFrame{
			ID:                 "pf-" + uuid.New().String(),
			ProcessingActionID: action.ID,
			FrameIndex:         i,
			SourceFrameID:      sourceFrameID,
			SourcePath:         sourcePath,
			ProcessedPath:      fullRelPath,
			Status:             "succeeded",
			Width:              width,
			Height:             height,
			ContentHash:        hash,
			QualityFlags:       qualityFlags,
			CreatedAt:          now,
			UpdatedAt:          now,
		})
	}

	if err := w.repo.CreateProcessedFrames(w.db, frames); err != nil {
		return fmt.Errorf("create processed frames failed: %w", err)
	}
	return nil
}

func computeFileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (w *Worker) startHeartbeat(ctx context.Context, taskID string) {
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
				if err := w.repo.UpdateProcessingHeartbeat(taskID, now); err != nil {
					log.Logger.Errorf("processing heartbeat task %s failed: %v", taskID, err)
				}
				if err := w.repo.RefreshProcessingLease(taskID, lease, now); err != nil {
					log.Logger.Errorf("refresh processing lease task %s failed: %v", taskID, err)
				}
			}
		}
	}()
}

func (w *Worker) updateStage(taskID, stage string, progress int) {
	now := time.Now().Format(processingTimeFormat)
	_ = w.repo.UpdateProcessingTaskStatusNoTx(taskID, map[string]interface{}{
		"current_stage": stage,
		"progress":      progress,
		"updated_at":    now,
	})
	w.publishProgress(taskID, progress, stage)
}

func (w *Worker) updateProgress(taskID string, progress int) {
	now := time.Now().Format(processingTimeFormat)
	_ = w.repo.UpdateProcessingTaskStatusNoTx(taskID, map[string]interface{}{
		"progress":   progress,
		"updated_at": now,
	})
	w.publishProgress(taskID, progress, "")
}

func (w *Worker) updateActionProgress(action *processing.ProcessingAction, stage string, progress int) {
	now := time.Now().Format(processingTimeFormat)
	_ = w.repo.UpdateProcessingActionNoTx(action.ID, map[string]interface{}{
		"progress":   progress,
		"updated_at": now,
	})
	w.publishActionProgress(action.ProcessingTaskID, action.ActionKey, stage, progress)
}

func (w *Worker) succeedAction(action *processing.ProcessingAction) {
	now := time.Now().Format(processingTimeFormat)
	_ = w.repo.UpdateProcessingActionNoTx(action.ID, map[string]interface{}{
		"status":       "succeeded",
		"progress":     100,
		"completed_at": now,
		"updated_at":   now,
	})
	w.publishActionEvent(action.ProcessingTaskID, action.ActionKey, "succeeded")
}

func (w *Worker) failAction(action *processing.ProcessingAction, err error) {
	now := time.Now().Format(processingTimeFormat)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	_ = w.repo.UpdateProcessingActionNoTx(action.ID, map[string]interface{}{
		"status":        "failed",
		"progress":      100,
		"error_message": errMsg,
		"completed_at":  now,
		"updated_at":    now,
	})
	w.publishActionEvent(action.ProcessingTaskID, action.ActionKey, "failed")
}

func (w *Worker) isCancelled(taskID string) bool {
	task, err := w.repo.GetProcessingTask(taskID)
	if err != nil || task == nil {
		return false
	}
	return task.CancelRequestedAt != ""
}

func (w *Worker) finalizeTask(taskID string, processErr error) {
	task, err := w.repo.GetProcessingTask(taskID)
	if err != nil {
		log.Logger.Errorf("get processing task for finalize failed: %v", err)
		return
	}

	actions, err := w.repo.ListProcessingActions(taskID)
	if err != nil {
		log.Logger.Errorf("list processing actions for finalize failed: %v", err)
		actions = nil
	}

	succeeded, failed := 0, 0
	for _, a := range actions {
		switch a.Status {
		case "succeeded":
			succeeded++
		case "failed":
			failed++
		}
	}

	now := time.Now().Format(processingTimeFormat)
	updates := map[string]interface{}{
		"completed_at": now,
		"updated_at":   now,
		"progress":     100,
		"current_stage": StageCompleted,
	}

	cancelled := w.isCancelled(taskID)
	total := len(actions)

	switch {
	case cancelled && succeeded > 0:
		updates["status"] = "partially_succeeded"
	case cancelled:
		updates["status"] = "cancelled"
	case total > 0 && succeeded == total:
		updates["status"] = "succeeded"
	case succeeded > 0:
		updates["status"] = "partially_succeeded"
	default:
		updates["status"] = "failed"
		if processErr != nil {
			updates["error_message"] = processErr.Error()
		}
	}

	if processErr != nil {
		if status, ok := updates["status"].(string); ok && status == "failed" {
			if _, hasMsg := updates["error_message"]; !hasMsg {
				updates["error_message"] = processErr.Error()
			}
		}
	}

	_ = w.repo.UpdateProcessingTaskStatusNoTx(taskID, updates)

	statusStr, _ := updates["status"].(string)
	w.publishCompleted(taskID, statusStr, succeeded, failed, total)

	if err := w.cleanupManager.CleanupTempDir(task.GenerationTaskID); err != nil {
		log.Logger.Errorf("cleanup temp dir for task %s failed: %v", task.GenerationTaskID, err)
	}
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
		err := w.repo.UpdateProcessingTaskStatusNoTx(task.ID, map[string]interface{}{
			"status":            "queued",
			"current_stage":     "queued",
			"execution_id":      "",
			"worker_id":         "",
			"lease_expires_at":  "",
			"last_heartbeat_at": "",
		})
		if err != nil {
			log.Logger.Errorf("recover processing task %s failed: %v", task.ID, err)
			continue
		}
		log.Logger.Infof("recovered processing task: %s", task.ID)
	}
}

func (w *Worker) canvasConfig(task *processing.ProcessingTask) processing.CanvasConfig {
	cfg := processing.CanvasConfig{
		OutputWidth:                task.OutputWidth,
		OutputHeight:               task.OutputHeight,
		TargetCharacterHeightRatio: task.TargetCharacterHeightRatio,
	}
	defaults := processing.DefaultCanvasConfig()
	if cfg.OutputWidth <= 0 {
		cfg.OutputWidth = defaults.OutputWidth
	}
	if cfg.OutputHeight <= 0 {
		cfg.OutputHeight = defaults.OutputHeight
	}
	if cfg.TargetCharacterHeightRatio <= 0 {
		cfg.TargetCharacterHeightRatio = defaults.TargetCharacterHeightRatio
	}
	return cfg
}
