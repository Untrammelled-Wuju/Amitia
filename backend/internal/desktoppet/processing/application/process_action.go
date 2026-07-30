package application

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	pcontracts "github.com/u-ai/backend/internal/desktoppet/processing/contracts"
	"github.com/u-ai/backend/internal/desktoppet/processing/source"
)

type RepoSourceResolver struct {
	repo        processing.Repository
	dataDir     string
	artifactRepo source.ArtifactSourceRepo
	spriteSheet *source.SpriteSheetSourceAdapter
	singleFrame *source.SingleFrameSourceAdapter
	keyframe    *source.KeyframeSourceAdapter
	legacy      *source.LegacyFrameAdapter
}

func NewRepoSourceResolver(repo processing.Repository, dataDir string, artifactRepo source.ArtifactSourceRepo) *RepoSourceResolver {
	r := &RepoSourceResolver{
		repo:        repo,
		dataDir:     dataDir,
		artifactRepo: artifactRepo,
	}
	if artifactRepo != nil {
		r.spriteSheet = source.NewSpriteSheetSourceAdapter(artifactRepo)
		r.singleFrame = source.NewSingleFrameSourceAdapter(artifactRepo)
		r.keyframe = source.NewKeyframeSourceAdapter(artifactRepo)
	}
	r.legacy = source.NewLegacyFrameAdapter(NewRepoLegacyAdapter(repo))
	return r
}

func (r *RepoSourceResolver) Resolve(ctx context.Context, req source.ResolveRequest) (*source.ProcessingSourceDescriptor, error) {
	if req.DataDir == "" {
		req.DataDir = r.dataDir
	}

	mode, err := r.resolveGenerationMode(req)
	if err != nil {
		return nil, err
	}

	switch contracts.GenerationMode(mode) {
	case contracts.GenerationModeSpriteSheet:
		if r.spriteSheet == nil {
			return nil, fmt.Errorf("sprite sheet adapter not configured")
		}
		return r.spriteSheet.Resolve(ctx, req)
	case contracts.GenerationModeSingleFrame:
		if r.singleFrame == nil {
			return nil, fmt.Errorf("single frame adapter not configured")
		}
		return r.singleFrame.Resolve(ctx, req)
	case contracts.GenerationModeKeyframe:
		if r.keyframe == nil {
			return nil, fmt.Errorf("keyframe adapter not configured")
		}
		return r.keyframe.Resolve(ctx, req)
	case contracts.GenerationModeLegacyFrame:
		return r.legacy.Resolve(ctx, req)
	default:
		return r.legacy.Resolve(ctx, req)
	}
}

func (r *RepoSourceResolver) resolveGenerationMode(req source.ResolveRequest) (string, error) {
	if r.artifactRepo == nil {
		return string(contracts.GenerationModeLegacyFrame), nil
	}

	taskActionID, err := r.artifactRepo.GetTaskActionID(req.GenerationTaskID, req.ActionKey)
	if err != nil {
		return "", fmt.Errorf("%w: %v", source.ErrSourcePlanNotFound, err)
	}

	attempt, err := r.artifactRepo.GetActiveAttemptInfo(taskActionID)
	if err != nil {
		return string(contracts.GenerationModeLegacyFrame), nil
	}
	if attempt == nil {
		return string(contracts.GenerationModeLegacyFrame), nil
	}

	if attempt.Mode != "" {
		return attempt.Mode, nil
	}

	return string(contracts.GenerationModeLegacyFrame), nil
}

type RepoLegacyAdapter struct {
	repo processing.Repository
}

func NewRepoLegacyAdapter(repo processing.Repository) *RepoLegacyAdapter {
	return &RepoLegacyAdapter{repo: repo}
}

func (a *RepoLegacyAdapter) GetGenerationTask(taskID string) (source.GenerationTaskInfo, error) {
	task, err := a.repo.GetGenerationTask(taskID)
	if err != nil {
		return source.GenerationTaskInfo{}, err
	}
	return source.GenerationTaskInfo{
		ID:     task.ID,
		UserID: task.UserID,
		Status: task.Status,
	}, nil
}

func (a *RepoLegacyAdapter) ListActionsByTaskID(taskID string) ([]source.GenerationTaskActionInfo, error) {
	actions, err := a.repo.ListActionsByTaskID(taskID)
	if err != nil {
		return nil, err
	}
	result := make([]source.GenerationTaskActionInfo, 0, len(actions))
	for _, act := range actions {
		result = append(result, source.GenerationTaskActionInfo{
			ID:             act.ID,
			ActionKey:      act.ActionKey,
			ActionName:     act.ActionNameSnapshot,
			CurrentAttempt: act.CurrentAttempt,
			FrameCount:     act.FrameCount,
			SortOrder:      act.SortOrder,
		})
	}
	return result, nil
}

func (a *RepoLegacyAdapter) ListFramesByAction(actionID string, attemptNumber int) ([]source.GenerationFrameInfo, error) {
	frames, err := a.repo.ListFramesByAction(actionID)
	if err != nil {
		return nil, err
	}
	result := make([]source.GenerationFrameInfo, 0, len(frames))
	for _, f := range frames {
		if f.GenerationAttempt != attemptNumber || f.Status != string(contracts.StatusSucceeded) {
			continue
		}
		result = append(result, source.GenerationFrameInfo{
			ID:              f.ID,
			FrameIndex:      f.FrameIndex,
			ResultImagePath: f.ResultImagePath,
			ResultHash:      f.ResultHash,
			Status:          f.Status,
			AttemptNumber:   f.AttemptNumber,
		})
	}
	return result, nil
}

type RepoArtifactSourceAdapter struct {
	repo processing.Repository
}

func NewRepoArtifactSourceAdapter(repo processing.Repository) *RepoArtifactSourceAdapter {
	return &RepoArtifactSourceAdapter{repo: repo}
}

func (a *RepoArtifactSourceAdapter) GetTaskActionID(taskID, actionKey string) (string, error) {
	actions, err := a.repo.ListActionsByTaskID(taskID)
	if err != nil {
		return "", err
	}
	for _, act := range actions {
		if act.ActionKey == actionKey {
			return act.ID, nil
		}
	}
	return "", fmt.Errorf("task action not found: taskID=%s actionKey=%s", taskID, actionKey)
}

func (a *RepoArtifactSourceAdapter) GetActiveAttemptInfo(taskActionID string) (*source.AttemptInfo, error) {
	db := a.repo.DB()
	var attempt struct {
		ID     string `gorm:"column:id"`
		Mode   string `gorm:"column:mode"`
		Status string `gorm:"column:status"`
	}
	err := db.Table("desktop_pet_action_generation_attempts").
		Where("task_action_id = ? AND status IN ?", taskActionID, []string{
			"pending", "preparing_reference", "building_prompt", "waiting_rate_limit",
			"submitting", "polling", "result_received", "persisting", "unknown_submission",
			"succeeded",
		}).
		Order("attempt_number DESC").
		First(&attempt).Error
	if err != nil {
		return nil, err
	}
	return &source.AttemptInfo{
		ID:     attempt.ID,
		Mode:   attempt.Mode,
		Status: attempt.Status,
	}, nil
}

func (a *RepoArtifactSourceAdapter) GetPrimaryArtifact(attemptID string) (*source.ArtifactInfo, error) {
	db := a.repo.DB()
	var artifact struct {
		ID           string `gorm:"column:id"`
		TaskID       string `gorm:"column:task_id"`
		TaskActionID string `gorm:"column:task_action_id"`
		AttemptID    string `gorm:"column:attempt_id"`
		ArtifactType string `gorm:"column:artifact_type"`
		Status       string `gorm:"column:status"`
		RelativePath string `gorm:"column:relative_path"`
		Hash         string `gorm:"column:hash"`
		Width        int    `gorm:"column:width"`
		Height       int    `gorm:"column:height"`
		LayoutJSON   string `gorm:"column:layout_json"`
		IsPrimary    int    `gorm:"column:is_primary"`
	}
	err := db.Table("desktop_pet_generation_artifacts").
		Where("attempt_id = ? AND is_primary = 1", attemptID).
		Order("segment_index ASC").
		First(&artifact).Error
	if err != nil {
		return nil, err
	}
	return &source.ArtifactInfo{
		ArtifactID:   artifact.ID,
		TaskID:       artifact.TaskID,
		TaskActionID: artifact.TaskActionID,
		AttemptID:    artifact.AttemptID,
		ArtifactType: artifact.ArtifactType,
		Status:       artifact.Status,
		RelativePath: artifact.RelativePath,
		Hash:         artifact.Hash,
		Width:        artifact.Width,
		Height:       artifact.Height,
		LayoutJSON:   artifact.LayoutJSON,
		IsPrimary:    artifact.IsPrimary == 1,
	}, nil
}

func (a *RepoArtifactSourceAdapter) GetTaskUserID(taskID string) (string, error) {
	task, err := a.repo.GetGenerationTask(taskID)
	if err != nil {
		return "", err
	}
	return task.UserID, nil
}

func BuildConfigSnapshot(task *processing.ProcessingTask) *pcontracts.ProcessingConfigSnapshot {
	return pcontracts.NewDefaultConfigSnapshot(
		task.OutputWidth,
		task.OutputHeight,
		task.TargetCharacterHeightRatio,
		task.AnchorMode,
		task.BackgroundMode,
		"",
		"",
	)
}
