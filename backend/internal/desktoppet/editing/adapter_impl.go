package editing

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/pkg/app"
)

type generationAdapter struct {
	ctx *app.AppContext
}

func newRawGenerationAdapter(ctx *app.AppContext) GenerationAdapter {
	return &generationAdapter{ctx: ctx}
}

func (a *generationAdapter) GenerateSingleFrame(ctx context.Context, req SingleFrameGenerationRequest) (*SingleFrameGenerationResult, error) {
	if req.GenerationTaskID == "" || req.ActionKey == "" {
		return nil, fmt.Errorf("generation task ID and action key are required")
	}

	var taskAction struct {
		ID string `gorm:"column:id"`
	}
	err := a.ctx.DB.Table("desktop_pet_generation_task_actions").
		Where("task_id = ? AND action_key = ?", req.GenerationTaskID, req.ActionKey).
		First(&taskAction).Error
	if err != nil {
		return nil, fmt.Errorf("find task action: %w", err)
	}

	attemptID := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)
	err = a.ctx.DB.Table("desktop_pet_action_generation_attempts").Create(map[string]interface{}{
		"id":             attemptID,
		"task_id":        req.GenerationTaskID,
		"task_action_id": taskAction.ID,
		"attempt_number": 1,
		"mode":           "single_frame",
		"reason":         "editing_single_frame_regen",
		"status":         "pending",
		"created_at":     now,
		"updated_at":     now,
	}).Error
	if err != nil {
		return nil, fmt.Errorf("create generation attempt: %w", err)
	}

	return &SingleFrameGenerationResult{
		ProviderAttemptID: attemptID,
		ImagePath:         "",
		Width:             0,
		Height:            0,
		CostActual:        nil,
	}, nil
}

func (a *generationAdapter) GenerateFullAction(ctx context.Context, req FullActionGenerationRequest) (*FullActionGenerationResult, error) {
	if req.GenerationTaskID == "" || req.ActionKey == "" {
		return nil, fmt.Errorf("generation task ID and action key are required")
	}

	var taskAction struct {
		ID string `gorm:"column:id"`
	}
	err := a.ctx.DB.Table("desktop_pet_generation_task_actions").
		Where("task_id = ? AND action_key = ?", req.GenerationTaskID, req.ActionKey).
		First(&taskAction).Error
	if err != nil {
		return nil, fmt.Errorf("find task action: %w", err)
	}

	attemptID := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)
	err = a.ctx.DB.Table("desktop_pet_action_generation_attempts").Create(map[string]interface{}{
		"id":             attemptID,
		"task_id":        req.GenerationTaskID,
		"task_action_id": taskAction.ID,
		"attempt_number": 1,
		"mode":           "sprite_sheet",
		"reason":         "editing_full_action_regen",
		"status":         "pending",
		"created_at":     now,
		"updated_at":     now,
	}).Error
	if err != nil {
		return nil, fmt.Errorf("create generation attempt: %w", err)
	}

	return &FullActionGenerationResult{
		ProviderAttemptID:   attemptID,
		CandidateRevisionID: "",
		FrameCount:          0,
		FramePaths:          []string{},
	}, nil
}

func (a *generationAdapter) GetGenerationArtifacts(ctx context.Context, generationTaskID, actionKey string, attemptNumber int) ([]GenerationArtifactInfo, error) {
	var taskActionID string
	err := a.ctx.DB.Table("desktop_pet_generation_task_actions").
		Where("task_id = ? AND action_key = ?", generationTaskID, actionKey).
		Select("id").Scan(&taskActionID).Error
	if err != nil {
		return nil, fmt.Errorf("find task action: %w", err)
	}
	if taskActionID == "" {
		return []GenerationArtifactInfo{}, nil
	}

	query := a.ctx.DB.Table("desktop_pet_generation_artifacts").
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
	err = query.Order("segment_index ASC").Find(&artifacts).Error
	if err != nil {
		return nil, fmt.Errorf("query artifacts: %w", err)
	}

	result := make([]GenerationArtifactInfo, 0, len(artifacts))
	for _, art := range artifacts {
		if art.ArtifactType == "provider_receipt" || art.ArtifactType == "layout_manifest" {
			continue
		}
		result = append(result, GenerationArtifactInfo{
			ArtifactID:  art.ID,
			FrameIndex:  art.SegmentIndex,
			ImagePath:   art.RelativePath,
			Width:       art.Width,
			Height:      art.Height,
			ContentHash: art.Hash,
			AttemptID:   art.AttemptID,
		})
	}
	return result, nil
}

type processingAdapter struct {
	ctx *app.AppContext
}

func newRawProcessingAdapter(ctx *app.AppContext) ProcessingAdapter {
	return &processingAdapter{ctx: ctx}
}

func (a *processingAdapter) GetProcessingAction(ctx context.Context, processingTaskID, actionKey string) (*ProcessingActionInfo, error) {
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
	table := "desktop_pet_processing_actions"
	query := a.ctx.DB.Table(table).
		Where("processing_task_id = ? AND action_key = ?", processingTaskID, actionKey).
		Order("created_at DESC").First(&action)
	if query.Error != nil {
		return nil, query.Error
	}
	genTaskID := ""
	var genTask struct {
		ID string `gorm:"column:id"`
	}
	a.ctx.DB.Table("desktop_pet_processing_tasks").Where("id = ?", processingTaskID).Select("generation_task_id").Scan(&genTaskID)
	_ = genTask
	return &ProcessingActionInfo{
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

func (a *processingAdapter) GetProcessedFrames(ctx context.Context, processingActionID string) ([]ProcessedFrameInfo, error) {
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
	result := make([]ProcessedFrameInfo, len(frames))
	for i, f := range frames {
		result[i] = ProcessedFrameInfo{
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

func (a *processingAdapter) GetProcessingRevisionFrames(ctx context.Context, revisionID string) ([]ProcessedFrameInfo, error) {
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
	result := make([]ProcessedFrameInfo, len(artifacts))
	for i, art := range artifacts {
		result[i] = ProcessedFrameInfo{
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

func (a *processingAdapter) ImportAsBaselineRevision(ctx context.Context, processingTaskID, actionKey string) (*BaselineRevisionImport, error) {
	action, err := a.GetProcessingAction(ctx, processingTaskID, actionKey)
	if err != nil {
		return nil, err
	}
	frames, err := a.GetProcessedFrames(ctx, action.ProcessingActionID)
	if err != nil {
		return nil, err
	}
	return &BaselineRevisionImport{
		ProcessingActionID: action.ProcessingActionID,
		Frames:             frames,
		LoopType:           action.LoopType,
		FPS:                action.FPS,
		FrameDurationMS:    action.FrameDurationMS,
	}, nil
}

type qualityAdapter struct {
	ctx *app.AppContext
}

func newRawQualityAdapter(ctx *app.AppContext) QualityAdapter {
	return &qualityAdapter{ctx: ctx}
}

func (a *qualityAdapter) EvaluateRevision(ctx context.Context, revisionID string) (string, error) {
	if revisionID == "" {
		return "", fmt.Errorf("revision ID is required")
	}

	var rev ActionRevision
	err := a.ctx.DB.Table("desktop_pet_action_revisions").Where("id = ?", revisionID).First(&rev).Error
	if err != nil {
		return "", fmt.Errorf("find revision: %w", err)
	}

	if rev.Status != RevisionStatusReady {
		return "", fmt.Errorf("revision %s status is %s, expected %s for quality evaluation", revisionID, rev.Status, RevisionStatusReady)
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
		Where("id = ? AND status = ? AND manifest_hash = ?", revisionID, RevisionStatusReady, rev.ManifestHash).
		Updates(map[string]interface{}{
			"status":                RevisionStatusQualityPending,
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

func (a *qualityAdapter) GetLatestEvaluation(ctx context.Context, revisionID string) (*QualityEvaluationInfo, error) {
	var rev ActionRevision
	err := a.ctx.DB.Table("desktop_pet_action_revisions").Where("id = ?", revisionID).First(&rev).Error
	if err != nil {
		return nil, err
	}
	return &QualityEvaluationInfo{
		EvaluationID: rev.QualityEvaluationID,
		RevisionID:   revisionID,
		Verdict:      rev.QualityVerdict,
		Status:       rev.Status,
	}, nil
}

func (a *qualityAdapter) GetFindings(ctx context.Context, revisionID string) ([]QualityFindingInfo, error) {
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

	result := make([]QualityFindingInfo, 0, len(findings))
	for _, f := range findings {
		result = append(result, QualityFindingInfo{
			FindingID:   f.ID,
			Severity:    f.Severity,
			Dimension:   f.DimensionKey,
			Description: f.RuleCode + ": " + f.Description,
		})
	}
	return result, nil
}

func (a *qualityAdapter) IsGatePassed(ctx context.Context, revisionID string) (bool, string, error) {
	var rev ActionRevision
	err := a.ctx.DB.Table("desktop_pet_action_revisions").Where("id = ?", revisionID).First(&rev).Error
	if err != nil {
		return false, "", err
	}
	if rev.Status == RevisionStatusQualityPending || rev.Status == RevisionStatusBuilding {
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

func joinPath(parts ...string) string {
	return filepath.Join(parts...)
}
