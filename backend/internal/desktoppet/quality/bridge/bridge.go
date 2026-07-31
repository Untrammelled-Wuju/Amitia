// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package bridge

import (
	"context"
	"path/filepath"

	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/internal/desktoppet/quality"
	"gorm.io/gorm"
)

type ProcessingBridge struct {
	db      *gorm.DB
	dataDir string
}

func NewProcessingBridge(db *gorm.DB, dataDir string) *ProcessingBridge {
	return &ProcessingBridge{
		db:      db,
		dataDir: dataDir,
	}
}

func (b *ProcessingBridge) GetActionMetadata(ctx context.Context, processingActionID string) (*quality.ActionMetadata, error) {
	var action processing.ProcessingAction
	err := b.db.WithContext(ctx).
		Where("id = ?", processingActionID).
		First(&action).Error
	if err != nil {
		return nil, err
	}

	var task processing.ProcessingTask
	err = b.db.WithContext(ctx).
		Where("id = ?", action.ProcessingTaskID).
		First(&task).Error
	if err != nil {
		return nil, err
	}

	canvasWidth := task.OutputWidth
	canvasHeight := task.OutputHeight

	frameCount := action.ProcessedFrameCount
	if frameCount <= 0 {
		frameCount = action.SourceFrameCount
	}

	return &quality.ActionMetadata{
		ActionKey:      action.ActionKey,
		LoopType:       action.LoopType,
		PlaybackMode:   action.PlaybackMode,
		AnchorProfile:  action.AnchorProfile,
		ActionSpecHash: action.ActionSpecHash,
		CanvasWidth:    canvasWidth,
		CanvasHeight:   canvasHeight,
		FrameCount:     frameCount,
		RevisionHash:   action.ActionSpecHash,
	}, nil
}

func (b *ProcessingBridge) GetActiveActionRevisionID(ctx context.Context, processingActionID string) (string, error) {
	var revisionID string
	err := b.db.WithContext(ctx).
		Table("desktop_pet_action_revisions").
		Where("processing_action_id = ? AND status IN ?", []string{"ready", "active"}).
		Order("created_at DESC").
		Limit(1).
		Select("id").
		Scan(&revisionID).Error
	if err != nil {
		return "", err
	}
	return revisionID, nil
}

func (b *ProcessingBridge) ListFrameData(ctx context.Context, processingActionID string) ([]quality.FrameData, error) {
	var frames []processing.ProcessedFrame
	err := b.db.WithContext(ctx).
		Where("processing_action_id = ?", processingActionID).
		Order("frame_index ASC").
		Find(&frames).Error
	if err != nil {
		return nil, err
	}

	result := make([]quality.FrameData, 0, len(frames))
	for _, f := range frames {
		absPath := f.ProcessedPath
		if absPath != "" && !filepath.IsAbs(absPath) {
			absPath = filepath.Join(b.dataDir, absPath)
		}
		result = append(result, quality.FrameData{
			FrameIndex:    f.FrameIndex,
			FilePath:      absPath,
			Width:         f.Width,
			Height:        f.Height,
			AlphaCoverage: f.AlphaCoverage,
			SubjectBox:    f.SubjectBox,
			AnchorX:       f.AnchorX,
			AnchorY:       f.AnchorY,
			ContentHash:   f.ContentHash,
			Status:        f.Status,
		})
	}
	return result, nil
}
