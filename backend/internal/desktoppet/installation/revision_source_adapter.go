package installation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/editing"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/internal/desktoppet/specs"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type revisionSourceAdapter struct {
	ctx          *app.AppContext
	editingRepo  editing.Repository
	editingSvc   editing.Service
	processingDB *gorm.DB
	dataDir      string
}

func NewRevisionSourceAdapter(ctx *app.AppContext, editingRepo editing.Repository, editingSvc editing.Service, dataDir string) RevisionSource {
	return &revisionSourceAdapter{
		ctx:          ctx,
		editingRepo:  editingRepo,
		editingSvc:   editingSvc,
		processingDB: ctx.DB,
		dataDir:      dataDir,
	}
}

func (a *revisionSourceAdapter) GetProcessingTaskInfo(processingTaskID string) (*ProcessingTaskInfo, error) {
	var task processing.ProcessingTask
	err := a.processingDB.Table("desktop_pet_processing_tasks").
		Where("id = ?", processingTaskID).First(&task).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("processing task not found: %s", processingTaskID)
		}
		return nil, err
	}

	var genTask struct {
		ID           string `gorm:"column:id"`
		CharacterID  string `gorm:"column:character_id"`
		Name         string `gorm:"column:name"`
		UserID       string `gorm:"column:user_id"`
	}
	err = a.processingDB.Table("desktop_pet_generation_tasks").
		Where("id = ?", task.GenerationTaskID).First(&genTask).Error
	if err != nil {
		return nil, fmt.Errorf("generation task not found: %s", task.GenerationTaskID)
	}

	return &ProcessingTaskInfo{
		ID:                task.ID,
		GenerationTaskID:  task.GenerationTaskID,
		ProcessingVersion: task.ProcessingVersion,
		OutputWidth:       task.OutputWidth,
		OutputHeight:      task.OutputHeight,
		DefaultFPS:        task.DefaultFPS,
		CharacterID:       genTask.CharacterID,
		PackageName:       genTask.Name,
		UserID:            genTask.UserID,
	}, nil
}

func (a *revisionSourceAdapter) ListProcessingActions(processingTaskID string) ([]BuilderActionInfo, error) {
	var actions []processing.ProcessingAction
	err := a.processingDB.Table("desktop_pet_processing_actions").
		Where("processing_task_id = ?", processingTaskID).
		Order("created_at ASC").Find(&actions).Error
	if err != nil {
		return nil, err
	}

	result := make([]BuilderActionInfo, 0, len(actions))
	for _, action := range actions {
		supportsDefaultIdle := action.ActionKey == "idle_normal"
		if spec, ok := specs.CatalogGet(action.ActionKey); ok {
			supportsDefaultIdle = spec.Identity.SupportsDefaultIdle
		}
		result = append(result, BuilderActionInfo{
			ActionKey:           action.ActionKey,
			ActionNameSnapshot:  action.ActionNameSnapshot,
			Status:              action.Status,
			Excluded:            action.Excluded == 1,
			LoopType:            action.LoopType,
			FPS:                 action.FPS,
			FrameDurationMS:     action.FrameDurationMS,
			SupportsDefaultIdle: supportsDefaultIdle,
		})
	}
	return result, nil
}

func (a *revisionSourceAdapter) GetActiveRevisionDetail(processingTaskID, actionKey string) (*ActiveRevisionDetail, error) {
	revisionID := ""

	newBinding, err := a.editingRepo.GetActiveActionRevisionBindingByTask(processingTaskID, actionKey)
	if err != nil {
		return nil, fmt.Errorf("get active action revision binding by task: %w", err)
	}
	if newBinding != nil {
		revisionID = newBinding.ActiveActionRevisionID
	}

	if revisionID == "" {
		oldBinding, err := a.editingRepo.GetActiveRevisionBinding(processingTaskID, actionKey)
		if err != nil {
			return nil, fmt.Errorf("get active revision binding: %w", err)
		}
		if oldBinding != nil {
			revisionID = oldBinding.RevisionID
		}
	}

	if revisionID == "" {
		return nil, fmt.Errorf("no active revision for task=%s action=%s", processingTaskID, actionKey)
	}

	revision, err := a.editingRepo.GetActionRevision(revisionID)
	if err != nil {
		return nil, fmt.Errorf("get action revision: %w", err)
	}

	frames, err := a.editingRepo.ListRevisionFrames(revisionID)
	if err != nil {
		return nil, fmt.Errorf("list revision frames: %w", err)
	}

	frameInfos := make([]BuilderFrameInfo, 0, len(frames))
	for _, frame := range frames {
		asset, err := a.editingRepo.GetFrameAsset(frame.AssetID)
		if err != nil {
			continue
		}
		frameInfos = append(frameInfos, BuilderFrameInfo{
			FrameID:      frame.FrameID,
			LogicalIndex: frame.LogicalIndex,
			AssetID:      frame.AssetID,
			ContentHash:  asset.ContentHash,
			DurationMS:   frame.DurationMS,
			Width:        asset.Width,
			Height:       asset.Height,
			MimeType:     asset.MimeType,
			StoragePath:  asset.StoragePath,
			ByteSize:     asset.ByteSize,
		})
	}

	priority := 50
	if revision.PriorityOverride != nil {
		priority = *revision.PriorityOverride
	}
	cooldownMs := 0
	if revision.CooldownMSOverride != nil {
		cooldownMs = *revision.CooldownMSOverride
	}

	playbackMode := revision.PlaybackMode
	if playbackMode == "" {
		playbackMode = revision.LoopType
	}

	return &ActiveRevisionDetail{
		RevisionID:     revision.ID,
		RevisionNumber: revision.RevisionNumber,
		Status:         revision.Status,
		FrameCount:     revision.FrameCount,
		DurationMS:     revision.DurationMS,
		DefaultFPS:     revision.DefaultFPS,
		LoopType:       revision.LoopType,
		PlaybackMode:   playbackMode,
		ReturnAction:   revision.ReturnAction,
		ReturnPolicy:   "",
		Interruptible:  revision.Interruptible == 1,
		Priority:       priority,
		CooldownMs:     cooldownMs,
		MinimumPlayMs:  0,
		MaximumPlayMs:  0,
		MutexGroup:     "",
		AnchorX:        0,
		AnchorY:        0,
		AnchorJSON:     revision.AnchorJSON,
		ContentHash:    revision.ContentHash,
		QualityVerdict: revision.QualityVerdict,
		Frames:         frameInfos,
	}, nil
}

func (a *revisionSourceAdapter) GetAssetPath(assetID string) (string, error) {
	asset, err := a.editingRepo.GetFrameAsset(assetID)
	if err != nil {
		return "", fmt.Errorf("get frame asset: %w", err)
	}
	if asset.StoragePath == "" {
		return "", fmt.Errorf("asset storage path is empty: %s", assetID)
	}
	if filepath.IsAbs(asset.StoragePath) {
		return asset.StoragePath, nil
	}
	return filepath.Join(a.dataDir, asset.StoragePath), nil
}

func (a *revisionSourceAdapter) GetPackagePreviewPath(processingTaskID string, processingVersion int) (string, error) {
	previewPath := filepath.Join(a.dataDir, "desktop-pets", "generation-tasks", processingTaskID, "processed",
		fmt.Sprintf("version-%d", processingVersion), "package-preview.png")
	if _, err := os.Stat(previewPath); err != nil {
		return "", nil
	}
	return previewPath, nil
}

var _ = time.Now
var _ = context.Background
