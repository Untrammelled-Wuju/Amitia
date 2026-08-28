package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/internal/desktoppet/editing"
	"github.com/u-ai/backend/internal/desktoppet/generation"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

const (
	editingGenerationPollInterval = 500 * time.Millisecond
	editingGenerationWaitTimeout  = 12 * time.Minute
)

type editingGenerationPort struct {
	ctx               *app.AppContext
	service           desktoppet.Service
	processingService processing.Service
}

func newEditingGenerationPort(ctx *app.AppContext, service desktoppet.Service, processingService processing.Service) editing.GenerationAdapter {
	return &editingGenerationPort{ctx: ctx, service: service, processingService: processingService}
}

type editingGenerationAttempt struct {
	ID            string `gorm:"column:id"`
	AttemptNumber int    `gorm:"column:attempt_number"`
	Status        string `gorm:"column:status"`
	ErrorCode     string `gorm:"column:error_code"`
	ErrorMessage  string `gorm:"column:error_message"`
}

type editingGenerationAction struct {
	ID                string `gorm:"column:id"`
	Status            string `gorm:"column:status"`
	NextAttemptNumber int    `gorm:"column:next_attempt_number"`
}

type editingRegenerationSubmission struct {
	ActiveAttemptID     string `gorm:"column:active_attempt_id"`
	GenerationAttemptID string `gorm:"column:generation_attempt_id"`
	ProcessingAttemptID string `gorm:"column:processing_attempt_id"`
	ProcessingTaskID    string `gorm:"column:processing_task_id"`
}

func editingSubmissionMarker(stableID string, attemptNumber int) string {
	return "editing-submit:" + stableID + ":" + strconv.Itoa(attemptNumber)
}

func parseEditingSubmissionMarker(marker, stableID string) (int, bool) {
	prefix := "editing-submit:" + stableID + ":"
	if !strings.HasPrefix(marker, prefix) {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(marker, prefix))
	return n, err == nil && n > 0
}

func isGenerationActionTerminal(status string) bool {
	switch status {
	case "succeeded", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func (a *editingGenerationPort) loadAttempt(id string) (*editingGenerationAttempt, error) {
	var attempt editingGenerationAttempt
	err := a.ctx.DB.Table("desktop_pet_action_generation_attempts").Where("id = ?", id).First(&attempt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (a *editingGenerationPort) loadAttemptByNumber(taskActionID string, attemptNumber int) (*editingGenerationAttempt, error) {
	var attempt editingGenerationAttempt
	err := a.ctx.DB.Table("desktop_pet_action_generation_attempts").
		Where("task_action_id = ? AND attempt_number = ?", taskActionID, attemptNumber).
		First(&attempt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (a *editingGenerationPort) waitAttempt(ctx context.Context, attempt *editingGenerationAttempt) (*editingGenerationAttempt, error) {
	if attempt == nil {
		return nil, errors.New("generation attempt is nil")
	}
	waitCtx := ctx
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		waitCtx, cancel = context.WithTimeout(ctx, editingGenerationWaitTimeout)
		defer cancel()
	}
	ticker := time.NewTicker(editingGenerationPollInterval)
	defer ticker.Stop()
	for {
		latest, err := a.loadAttempt(attempt.ID)
		if err != nil {
			return nil, err
		}
		if latest != nil {
			switch generation.AttemptStatus(latest.Status) {
			case generation.AttemptStatusSucceeded:
				return latest, nil
			case generation.AttemptStatusFailed,
				generation.AttemptStatusFailedConfirmed,
				generation.AttemptStatusManualReview,
				generation.AttemptStatusCancelled,
				generation.AttemptStatusCancelNotSupported,
				generation.AttemptStatusCancelledAfterProviderCompletion:
				message := strings.TrimSpace(latest.ErrorMessage)
				if message == "" {
					message = "generation attempt ended without a usable artifact"
				}
				if latest.ErrorCode != "" {
					return nil, fmt.Errorf("generation attempt %s failed [%s]: %s", latest.ID, latest.ErrorCode, message)
				}
				return nil, fmt.Errorf("generation attempt %s failed: %s", latest.ID, message)
			}
		}
		select {
		case <-waitCtx.Done():
			return nil, fmt.Errorf("wait generation attempt %s: %w", attempt.ID, waitCtx.Err())
		case <-ticker.C:
		}
	}
}

func (a *editingGenerationPort) submitAndWait(ctx context.Context, jobID, generationTaskID, actionKey, stableSubmissionID string) (*editingGenerationAttempt, error) {
	if jobID == "" || generationTaskID == "" || actionKey == "" || stableSubmissionID == "" {
		return nil, errors.New("job ID, generation task ID, action key and stable submission ID are required")
	}
	if a.service == nil {
		return nil, errors.New("desktop pet generation service is unavailable")
	}

	var action editingGenerationAction
	if err := a.ctx.DB.Table("desktop_pet_generation_task_actions").
		Where("task_id = ? AND action_key = ?", generationTaskID, actionKey).
		First(&action).Error; err != nil {
		return nil, fmt.Errorf("find task action: %w", err)
	}

	var submission editingRegenerationSubmission
	if err := a.ctx.DB.Table("desktop_pet_regeneration_jobs").
		Where("id = ?", jobID).
		First(&submission).Error; err != nil {
		return nil, fmt.Errorf("load regeneration submission state: %w", err)
	}
	if submission.GenerationAttemptID != "" {
		attempt, err := a.loadAttempt(submission.GenerationAttemptID)
		if err != nil {
			return nil, err
		}
		if attempt == nil {
			return nil, fmt.Errorf("recorded generation attempt %s does not exist", submission.GenerationAttemptID)
		}
		return a.waitAttempt(ctx, attempt)
	}

	expectedAttempt := 0
	markerExists := false
	if submission.ActiveAttemptID != "" {
		var ok bool
		expectedAttempt, ok = parseEditingSubmissionMarker(submission.ActiveAttemptID, stableSubmissionID)
		if !ok {
			return nil, fmt.Errorf("regeneration job %s is already bound to another generation submission", jobID)
		}
		markerExists = true
	} else {
		expectedAttempt = action.NextAttemptNumber
		if expectedAttempt <= 0 {
			expectedAttempt = 1
		}
		marker := editingSubmissionMarker(stableSubmissionID, expectedAttempt)
		result := a.ctx.DB.Table("desktop_pet_regeneration_jobs").
			Where("id = ? AND (active_attempt_id = '' OR active_attempt_id IS NULL)", jobID).
			Updates(map[string]any{"active_attempt_id": marker, "updated_at": time.Now().UTC().Format(time.RFC3339)})
		if result.Error != nil {
			return nil, fmt.Errorf("reserve regeneration submission: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return a.submitAndWait(ctx, jobID, generationTaskID, actionKey, stableSubmissionID)
		}
	}

	attempt, err := a.loadAttemptByNumber(action.ID, expectedAttempt)
	if err != nil {
		return nil, err
	}
	if attempt == nil {
		// A persisted marker means a previous process may have queued the retry and
		// crashed before the Generation Worker created its attempt. Never queue a
		// second retry while the action is already active.
		if !markerExists || isGenerationActionTerminal(action.Status) {
			if _, err := a.service.RetryAction(generationTaskID, actionKey); err != nil {
				// If the retry raced with another claimant, re-read before surfacing the
				// conflict. The stable expected attempt is authoritative.
				if existing, readErr := a.loadAttemptByNumber(action.ID, expectedAttempt); readErr == nil && existing != nil {
					attempt = existing
				} else {
					return nil, fmt.Errorf("queue canonical generation retry: %w", err)
				}
			}
		}
	}

	if attempt == nil {
		waitCtx := ctx
		var cancel context.CancelFunc
		if _, ok := ctx.Deadline(); !ok {
			waitCtx, cancel = context.WithTimeout(ctx, editingGenerationWaitTimeout)
			defer cancel()
		}
		ticker := time.NewTicker(editingGenerationPollInterval)
		defer ticker.Stop()
		for attempt == nil {
			attempt, err = a.loadAttemptByNumber(action.ID, expectedAttempt)
			if err != nil {
				return nil, err
			}
			if attempt != nil {
				break
			}
			select {
			case <-waitCtx.Done():
				return nil, fmt.Errorf("wait canonical generation attempt %d: %w", expectedAttempt, waitCtx.Err())
			case <-ticker.C:
			}
		}
	}

	if err := a.ctx.DB.Table("desktop_pet_regeneration_jobs").Where("id = ?", jobID).Updates(map[string]any{
		"active_attempt_id":     attempt.ID,
		"generation_attempt_id": attempt.ID,
		"provider_attempt_id":   attempt.ID,
		"updated_at":            time.Now().UTC().Format(time.RFC3339),
	}).Error; err != nil {
		return nil, fmt.Errorf("persist generation attempt identity: %w", err)
	}
	return a.waitAttempt(ctx, attempt)
}

func (a *editingGenerationPort) findProcessingTaskForAttempt(generationTaskID, actionKey string, attemptNumber int) (*processing.ProcessingTask, error) {
	var task processing.ProcessingTask
	err := a.ctx.DB.Table("desktop_pet_processing_tasks AS t").
		Joins("JOIN desktop_pet_processing_actions AS a ON a.processing_task_id = t.id").
		Where("t.generation_task_id = ? AND a.action_key = ? AND a.source_attempt_number = ?", generationTaskID, actionKey, attemptNumber).
		Order("t.processing_version DESC").
		Select("t.*").
		First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (a *editingGenerationPort) ensureProcessingTask(ctx context.Context, jobID, generationTaskID, actionKey, userID string, attemptNumber int) (*processing.ProcessingTask, error) {
	if a.processingService == nil {
		return nil, errors.New("desktop pet processing service is unavailable")
	}
	var submission editingRegenerationSubmission
	if err := a.ctx.DB.WithContext(ctx).Table("desktop_pet_regeneration_jobs").Where("id = ?", jobID).First(&submission).Error; err != nil {
		return nil, err
	}
	if submission.ProcessingAttemptID != "" {
		resp, err := a.processingService.GetProcessingTask(submission.ProcessingAttemptID)
		if err == nil && resp != nil && resp.ProcessingTask != nil {
			return resp.ProcessingTask, nil
		}
	}
	if existing, err := a.findProcessingTaskForAttempt(generationTaskID, actionKey, attemptNumber); err != nil {
		return nil, err
	} else if existing != nil {
		_ = a.ctx.DB.Table("desktop_pet_regeneration_jobs").Where("id = ?", jobID).Updates(map[string]any{
			"processing_attempt_id": existing.ID,
			"updated_at":            time.Now().UTC().Format(time.RFC3339),
		}).Error
		return existing, nil
	}

	var source processing.ProcessingTask
	if submission.ProcessingTaskID != "" {
		_ = a.ctx.DB.Table("desktop_pet_processing_tasks").Where("id = ?", submission.ProcessingTaskID).First(&source).Error
	}
	created, err := a.processingService.CreateProcessingTask(&processing.CreateProcessingTaskRequest{
		GenerationTaskID:           generationTaskID,
		UserID:                     userID,
		OutputWidth:                source.OutputWidth,
		OutputHeight:               source.OutputHeight,
		TargetCharacterHeightRatio: source.TargetCharacterHeightRatio,
		AnchorMode:                 source.AnchorMode,
		BackgroundMode:             source.BackgroundMode,
		OutputFormat:               source.OutputFormat,
		DefaultFPS:                 source.DefaultFPS,
	})
	if err != nil {
		// A crash after task creation but before job bookkeeping is recovered by
		// adopting the processing task that already points at this generation attempt.
		if existing, findErr := a.findProcessingTaskForAttempt(generationTaskID, actionKey, attemptNumber); findErr == nil && existing != nil {
			created = existing
		} else {
			return nil, fmt.Errorf("create processing task for regeneration: %w", err)
		}
	}
	if err := a.ctx.DB.Table("desktop_pet_regeneration_jobs").Where("id = ?", jobID).Updates(map[string]any{
		"processing_attempt_id": created.ID,
		"updated_at":            time.Now().UTC().Format(time.RFC3339),
	}).Error; err != nil {
		return nil, err
	}
	return created, nil
}

func (a *editingGenerationPort) waitProcessedFrames(ctx context.Context, task *processing.ProcessingTask, actionKey string, attemptNumber int) ([]editing.GenerationArtifactInfo, error) {
	if task == nil {
		return nil, errors.New("processing task is nil")
	}
	waitCtx := ctx
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		waitCtx, cancel = context.WithTimeout(ctx, editingGenerationWaitTimeout)
		defer cancel()
	}
	ticker := time.NewTicker(editingGenerationPollInterval)
	defer ticker.Stop()
	for {
		resp, err := a.processingService.GetProcessingTask(task.ID)
		if err != nil {
			return nil, err
		}
		if resp != nil && resp.ProcessingTask != nil {
			status := resp.ProcessingTask.Status
			switch status {
			case "succeeded", "partially_succeeded":
				var actionStatus string
				for _, action := range resp.Actions {
					if action.ActionKey == actionKey {
						actionStatus = action.Status
						break
					}
				}
				if actionStatus != "succeeded" {
					return nil, fmt.Errorf("processing action %s ended with status %s", actionKey, actionStatus)
				}
				var rows []struct {
					ID            string `gorm:"column:id"`
					FrameIndex    int    `gorm:"column:frame_index"`
					ProcessedPath string `gorm:"column:processed_path"`
					Width         int    `gorm:"column:width"`
					Height        int    `gorm:"column:height"`
					ContentHash   string `gorm:"column:content_hash"`
				}
				if err := a.ctx.DB.WithContext(ctx).Table("desktop_pet_processed_frames AS f").
					Joins("JOIN desktop_pet_processing_actions AS a ON a.id = f.processing_action_id").
					Where("a.processing_task_id = ? AND a.action_key = ? AND a.source_attempt_number = ? AND f.status = ?", task.ID, actionKey, attemptNumber, "succeeded").
					Order("f.frame_index ASC").Find(&rows).Error; err != nil {
					return nil, err
				}
				if len(rows) == 0 {
					return nil, fmt.Errorf("processing task %s produced no frames for %s", task.ID, actionKey)
				}
				out := make([]editing.GenerationArtifactInfo, 0, len(rows))
				for _, row := range rows {
					path := row.ProcessedPath
					if !filepath.IsAbs(path) {
						path = filepath.Join(config.AppCfg.Storage.DataDir, filepath.FromSlash(path))
					}
					out = append(out, editing.GenerationArtifactInfo{
						ArtifactID: row.ID, FrameIndex: row.FrameIndex, ImagePath: path,
						Width: row.Width, Height: row.Height, ContentHash: row.ContentHash,
					})
				}
				return out, nil
			case "failed", "cancelled":
				return nil, fmt.Errorf("processing task %s ended with status %s: %s", task.ID, status, resp.ProcessingTask.ErrorMessage)
			}
		}
		select {
		case <-waitCtx.Done():
			return nil, fmt.Errorf("wait processing task %s: %w", task.ID, waitCtx.Err())
		case <-ticker.C:
		}
	}
}

func (a *editingGenerationPort) generateProcessedFrames(ctx context.Context, jobID, generationTaskID, actionKey, userID, stableSubmissionID string) (*editingGenerationAttempt, []editing.GenerationArtifactInfo, error) {
	attempt, err := a.submitAndWait(ctx, jobID, generationTaskID, actionKey, stableSubmissionID)
	if err != nil {
		return nil, nil, err
	}
	processingTask, err := a.ensureProcessingTask(ctx, jobID, generationTaskID, actionKey, userID, attempt.AttemptNumber)
	if err != nil {
		return nil, nil, err
	}
	frames, err := a.waitProcessedFrames(ctx, processingTask, actionKey, attempt.AttemptNumber)
	if err != nil {
		return nil, nil, err
	}
	for i := range frames {
		frames[i].AttemptID = attempt.ID
	}
	return attempt, frames, nil
}

func (a *editingGenerationPort) GenerateSingleFrame(ctx context.Context, req editing.SingleFrameGenerationRequest) (*editing.SingleFrameGenerationResult, error) {
	attempt, frames, err := a.generateProcessedFrames(ctx, req.JobID, req.GenerationTaskID, req.ActionKey, req.UserID, req.AttemptID)
	if err != nil {
		return nil, err
	}
	var selected *editing.GenerationArtifactInfo
	for i := range frames {
		if frames[i].FrameIndex == req.FrameIndex {
			selected = &frames[i]
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("processed regeneration attempt %s has no frame index %d", attempt.ID, req.FrameIndex)
	}
	return &editing.SingleFrameGenerationResult{
		ProviderAttemptID: attempt.ID,
		ImagePath:         selected.ImagePath,
		Width:             selected.Width,
		Height:            selected.Height,
	}, nil
}

func (a *editingGenerationPort) GenerateFullAction(ctx context.Context, req editing.FullActionGenerationRequest) (*editing.FullActionGenerationResult, error) {
	attempt, frames, err := a.generateProcessedFrames(ctx, req.JobID, req.GenerationTaskID, req.ActionKey, req.UserID, req.AttemptID)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(frames))
	for _, frame := range frames {
		if frame.ImagePath != "" {
			paths = append(paths, frame.ImagePath)
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("processed regeneration attempt %s completed without frames", attempt.ID)
	}
	return &editing.FullActionGenerationResult{
		ProviderAttemptID: attempt.ID,
		FrameCount:        len(paths),
		FramePaths:        paths,
	}, nil
}

func (a *editingGenerationPort) GetGenerationArtifacts(ctx context.Context, generationTaskID, actionKey string, attemptNumber int) ([]editing.GenerationArtifactInfo, error) {
	var taskActionID string
	err := a.ctx.DB.WithContext(ctx).Table("desktop_pet_generation_task_actions").
		Where("task_id = ? AND action_key = ?", generationTaskID, actionKey).
		Select("id").Scan(&taskActionID).Error
	if err != nil {
		return nil, fmt.Errorf("find task action: %w", err)
	}
	if taskActionID == "" {
		return []editing.GenerationArtifactInfo{}, nil
	}
	query := a.ctx.DB.WithContext(ctx).Table("desktop_pet_generation_artifacts").
		Where("task_action_id = ?", taskActionID)
	if attemptNumber > 0 {
		query = query.Where("attempt_id IN (?)",
			a.ctx.DB.Table("desktop_pet_action_generation_attempts").
				Select("id").
				Where("task_action_id = ? AND attempt_number = ?", taskActionID, attemptNumber))
	}
	var artifacts []struct {
		ID           string `gorm:"column:id"`
		AttemptID    string `gorm:"column:attempt_id"`
		ArtifactType string `gorm:"column:artifact_type"`
		Status       string `gorm:"column:status"`
		RelativePath string `gorm:"column:relative_path"`
		Hash         string `gorm:"column:hash"`
		Width        int    `gorm:"column:width"`
		Height       int    `gorm:"column:height"`
		SegmentIndex int    `gorm:"column:segment_index"`
	}
	if err := query.Order("segment_index ASC").Find(&artifacts).Error; err != nil {
		return nil, fmt.Errorf("query artifacts: %w", err)
	}
	result := make([]editing.GenerationArtifactInfo, 0, len(artifacts))
	for _, art := range artifacts {
		if art.ArtifactType == "provider_receipt" || art.ArtifactType == "layout_manifest" || art.RelativePath == "" {
			continue
		}
		path := art.RelativePath
		if !filepath.IsAbs(path) {
			path = filepath.Join(config.AppCfg.Storage.DataDir, filepath.FromSlash(path))
		}
		result = append(result, editing.GenerationArtifactInfo{
			ArtifactID:  art.ID,
			FrameIndex:  art.SegmentIndex,
			ImagePath:   path,
			Width:       art.Width,
			Height:      art.Height,
			ContentHash: art.Hash,
			AttemptID:   art.AttemptID,
		})
	}
	return result, nil
}

type editingProcessingPort struct {
	ctx *app.AppContext
}

func newEditingProcessingPort(ctx *app.AppContext) editing.ProcessingAdapter {
	return &editingProcessingPort{ctx: ctx}
}

func (a *editingProcessingPort) GetProcessingAction(ctx context.Context, processingTaskID, actionKey string) (*editing.ProcessingActionInfo, error) {
	var action struct {
		ID                  string `gorm:"column:id"`
		ProcessingTaskID    string `gorm:"column:processing_task_id"`
		GenerationTaskID    string `gorm:"column:generation_task_id"`
		ActionKey           string `gorm:"column:action_key"`
		ActionNameSnapshot  string `gorm:"column:action_name_snapshot"`
		SourceAttemptNumber int    `gorm:"column:source_attempt_number"`
		Status              string `gorm:"column:status"`
		LoopType            string `gorm:"column:loop_type"`
		FPS                 int    `gorm:"column:fps"`
		FrameDurationMS     int    `gorm:"column:frame_duration_ms"`
		Excluded            int    `gorm:"column:excluded"`
	}
	query := a.ctx.DB.Table("desktop_pet_processing_actions").
		Where("processing_task_id = ? AND action_key = ?", processingTaskID, actionKey).
		Order("created_at DESC").First(&action)
	if query.Error != nil {
		return nil, query.Error
	}
	genTaskID := ""
	a.ctx.DB.Table("desktop_pet_processing_tasks").Where("id = ?", processingTaskID).Select("generation_task_id").Scan(&genTaskID)
	return &editing.ProcessingActionInfo{
		ProcessingActionID:  action.ID,
		ProcessingTaskID:    action.ProcessingTaskID,
		GenerationTaskID:    genTaskID,
		ActionKey:           action.ActionKey,
		ActionNameSnapshot:  action.ActionNameSnapshot,
		SourceAttemptNumber: action.SourceAttemptNumber,
		Status:              action.Status,
		LoopType:            action.LoopType,
		FPS:                 action.FPS,
		FrameDurationMS:     action.FrameDurationMS,
		Excluded:            action.Excluded != 0,
	}, nil
}

func (a *editingProcessingPort) GetProcessedFrames(ctx context.Context, processingActionID string) ([]editing.ProcessedFrameInfo, error) {
	var frames []struct {
		ID            string  `gorm:"column:id"`
		FrameIndex    int     `gorm:"column:frame_index"`
		ProcessedPath string  `gorm:"column:processed_path"`
		SourcePath    string  `gorm:"column:source_path"`
		Width         int     `gorm:"column:width"`
		Height        int     `gorm:"column:height"`
		ContentHash   string  `gorm:"column:content_hash"`
		AnchorX       float64 `gorm:"column:anchor_x"`
		AnchorY       float64 `gorm:"column:anchor_y"`
		QualityFlags  string  `gorm:"column:quality_flags"`
	}
	err := a.ctx.DB.Table("desktop_pet_processed_frames").
		Where("processing_action_id = ? AND status = ?", processingActionID, "succeeded").
		Order("frame_index ASC").Find(&frames).Error
	if err != nil {
		return nil, err
	}
	result := make([]editing.ProcessedFrameInfo, len(frames))
	for i, f := range frames {
		result[i] = editing.ProcessedFrameInfo{
			FrameID:       fmt.Sprintf("frm-legacy-%s-%d", processingActionID, f.FrameIndex),
			FrameIndex:    f.FrameIndex,
			ProcessedPath: f.ProcessedPath,
			SourcePath:    f.SourcePath,
			Width:         f.Width,
			Height:        f.Height,
			ContentHash:   f.ContentHash,
			AnchorX:       f.AnchorX,
			AnchorY:       f.AnchorY,
			QualityFlags:  f.QualityFlags,
		}
	}
	return result, nil
}

func (a *editingProcessingPort) GetProcessingRevisionFrames(ctx context.Context, revisionID string) ([]editing.ProcessedFrameInfo, error) {
	var artifacts []struct {
		FrameIndex   int    `gorm:"column:frame_index"`
		RelativePath string `gorm:"column:relative_path"`
		Width        int    `gorm:"column:width"`
		Height       int    `gorm:"column:height"`
		ContentHash  string `gorm:"column:content_hash"`
	}
	err := a.ctx.DB.Table("desktop_pet_processing_artifacts").
		Where("revision_id = ? AND artifact_kind = ?", revisionID, "frame").
		Order("frame_index ASC").Find(&artifacts).Error
	if err != nil {
		return nil, err
	}
	result := make([]editing.ProcessedFrameInfo, len(artifacts))
	for i, art := range artifacts {
		result[i] = editing.ProcessedFrameInfo{
			FrameID:       fmt.Sprintf("frm-prev-%s-%d", revisionID, art.FrameIndex),
			FrameIndex:    art.FrameIndex,
			ProcessedPath: art.RelativePath,
			Width:         art.Width,
			Height:        art.Height,
			ContentHash:   art.ContentHash,
		}
	}
	return result, nil
}

type editingQualityPort struct {
	ctx *app.AppContext
}

func newEditingQualityPort(ctx *app.AppContext) editing.QualityAdapter {
	return &editingQualityPort{ctx: ctx}
}

func (a *editingQualityPort) EvaluateRevision(ctx context.Context, revisionID string) (string, error) {
	if revisionID == "" {
		return "", fmt.Errorf("revision ID is required")
	}
	var rev editing.ActionRevision
	err := a.ctx.DB.Table("desktop_pet_action_revisions").Where("id = ?", revisionID).First(&rev).Error
	if err != nil {
		return "", fmt.Errorf("find revision: %w", err)
	}
	if rev.Status != editing.RevisionStatusReady {
		return "", fmt.Errorf("revision %s status is %s, expected %s for quality evaluation", revisionID, rev.Status, editing.RevisionStatusReady)
	}
	if rev.ManifestHash == "" {
		return "", fmt.Errorf("revision %s manifest hash is empty, content not finalized", revisionID)
	}
	evaluationID := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)
	err = a.ctx.DB.Table("desktop_pet_quality_evaluations").Create(map[string]interface{}{
		"id":                   evaluationID,
		"processing_task_id":   rev.ProcessingTaskID,
		"processing_action_id": rev.ProcessingActionID,
		"action_revision_id":   revisionID,
		"action_key":           rev.ActionKey,
		"execution_status":     "pending",
		"quality_mode":         "standard",
		"is_active":            1,
		"created_at":           now,
		"updated_at":           now,
	}).Error
	if err != nil {
		return "", fmt.Errorf("create quality evaluation: %w", err)
	}
	result := a.ctx.DB.Table("desktop_pet_action_revisions").
		Where("id = ? AND status = ? AND manifest_hash = ?", revisionID, editing.RevisionStatusReady, rev.ManifestHash).
		Updates(map[string]interface{}{
			"status":                editing.RevisionStatusQualityPending,
			"quality_evaluation_id": evaluationID,
			"updated_at":            now,
		})
	if result.Error != nil {
		return "", fmt.Errorf("cas update action revision: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return "", fmt.Errorf("cas update failed: revision %s was modified by a newer edit", revisionID)
	}
	return evaluationID, nil
}

func (a *editingQualityPort) GetLatestEvaluation(ctx context.Context, revisionID string) (*editing.QualityEvaluationInfo, error) {
	var rev editing.ActionRevision
	err := a.ctx.DB.Table("desktop_pet_action_revisions").Where("id = ?", revisionID).First(&rev).Error
	if err != nil {
		return nil, err
	}
	return &editing.QualityEvaluationInfo{
		EvaluationID: rev.QualityEvaluationID,
		RevisionID:   revisionID,
		Verdict:      rev.QualityVerdict,
		Status:       rev.Status,
	}, nil
}

func (a *editingQualityPort) GetFindings(ctx context.Context, revisionID string) ([]editing.QualityFindingInfo, error) {
	var findings []struct {
		ID               string `gorm:"column:id"`
		Severity         string `gorm:"column:severity"`
		DimensionKey     string `gorm:"column:dimension_key"`
		RuleCode         string `gorm:"column:rule_code"`
		FrameIndexesJSON string `gorm:"column:frame_indexes_json"`
		Description      string `gorm:"column:description"`
	}
	err := a.ctx.DB.Table("desktop_pet_quality_findings").
		Joins("JOIN desktop_pet_quality_evaluations ON desktop_pet_quality_findings.evaluation_id = desktop_pet_quality_evaluations.id").
		Where("desktop_pet_quality_evaluations.action_revision_id = ?", revisionID).
		Order("desktop_pet_quality_findings.severity DESC").
		Find(&findings).Error
	if err != nil {
		return nil, err
	}
	result := make([]editing.QualityFindingInfo, 0, len(findings))
	for _, f := range findings {
		result = append(result, editing.QualityFindingInfo{
			FindingID:   f.ID,
			Severity:    f.Severity,
			Dimension:   f.DimensionKey,
			Description: f.RuleCode + ": " + f.Description,
		})
	}
	return result, nil
}

func (a *editingQualityPort) IsGatePassed(ctx context.Context, revisionID string) (bool, string, error) {
	var rev editing.ActionRevision
	err := a.ctx.DB.Table("desktop_pet_action_revisions").Where("id = ?", revisionID).First(&rev).Error
	if err != nil {
		return false, "", err
	}
	if rev.Status == editing.RevisionStatusQualityPending || rev.Status == editing.RevisionStatusBuilding {
		return false, "quality_pending", nil
	}
	var eval struct {
		ExecutionStatus string `gorm:"column:execution_status"`
		Verdict         string `gorm:"column:verdict"`
	}
	if rev.QualityEvaluationID != "" {
		a.ctx.DB.Table("desktop_pet_quality_evaluations").
			Where("id = ?", rev.QualityEvaluationID).
			First(&eval)
	}
	if eval.ExecutionStatus == "evaluation_failed" {
		return false, "evaluation_failed", nil
	}
	if eval.Verdict == "rejected" || rev.QualityVerdict == "rejected" {
		return false, "rejected", nil
	}
	if eval.Verdict == "needs_review" || rev.QualityVerdict == "needs_review" {
		return false, "needs_review", nil
	}
	if eval.ExecutionStatus != "" && eval.ExecutionStatus != "succeeded" {
		return false, eval.ExecutionStatus, nil
	}
	return true, rev.QualityVerdict, nil
}
