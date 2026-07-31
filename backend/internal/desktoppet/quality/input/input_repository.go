// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package input

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/quality"
	"gorm.io/gorm"
)

type actionRevisionRow struct {
	ID                         string `gorm:"column:id"`
	UserID                     string `gorm:"column:user_id"`
	CharacterID                string `gorm:"column:character_id"`
	ProcessingTaskID           string `gorm:"column:processing_task_id"`
	ProcessingActionID         string `gorm:"column:processing_action_id"`
	ActionKey                  string `gorm:"column:action_key"`
	ContentHash                string `gorm:"column:content_hash"`
	SourceProcessingRevisionID string `gorm:"column:source_processing_revision_id"`
	PlaybackMode               string `gorm:"column:playback_mode"`
	DefaultFPS                 int    `gorm:"column:default_fps"`
	FrameCount                 int    `gorm:"column:frame_count"`
}

func (actionRevisionRow) TableName() string { return "desktop_pet_action_revisions" }

type actionRevisionFrameRow struct {
	ID            string  `gorm:"column:id"`
	RevisionID    string  `gorm:"column:revision_id"`
	AssetID       string  `gorm:"column:asset_id"`
	LogicalIndex  int     `gorm:"column:logical_index"`
	AnchorX       float64 `gorm:"column:anchor_x"`
	AnchorY       float64 `gorm:"column:anchor_y"`
	AnchorSpace   string  `gorm:"column:anchor_space"`
	TransformJSON string  `gorm:"column:transform_json"`
	MetadataJSON  string  `gorm:"column:metadata_json"`
}

func (actionRevisionFrameRow) TableName() string { return "desktop_pet_action_revision_frames" }

type frameAssetRow struct {
	ID          string `gorm:"column:id"`
	ContentHash string `gorm:"column:content_hash"`
	StoragePath string `gorm:"column:storage_path"`
	MimeType    string `gorm:"column:mime_type"`
	Width       int    `gorm:"column:width"`
	Height      int    `gorm:"column:height"`
	Status      string `gorm:"column:status"`
}

func (frameAssetRow) TableName() string { return "desktop_pet_frame_assets" }

type InputRepository struct {
	db      *gorm.DB
	dataDir string
}

func NewInputRepository(db *gorm.DB, dataDir string) *InputRepository {
	return &InputRepository{db: db, dataDir: dataDir}
}

func (r *InputRepository) safePath(framePath string) (string, error) {
	if framePath == "" {
		return "", fmt.Errorf("frame asset missing: empty storage path")
	}
	cleaned := filepath.Clean(framePath)
	if strings.HasPrefix(cleaned, "..") {
		return "", fmt.Errorf("path escape detected: %s", framePath)
	}
	absDataDir, err := filepath.Abs(r.dataDir)
	if err != nil {
		return "", err
	}
	absPath := filepath.Join(absDataDir, cleaned)
	if !strings.HasPrefix(absPath, absDataDir+string(filepath.Separator)) && absPath != absDataDir {
		return "", fmt.Errorf("path escape detected: %s", framePath)
	}
	return absPath, nil
}

func (r *InputRepository) LoadActionRevisionInput(ctx context.Context, userID string, actionRevisionID string) (*quality.QualityActionInput, error) {
	var rev actionRevisionRow
	err := r.db.WithContext(ctx).
		Where("id = ?", actionRevisionID).
		First(&rev).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, quality.ErrActionRevisionNotFound
		}
		return nil, err
	}

	if rev.UserID != userID {
		return nil, fmt.Errorf("user mismatch: action revision belongs to %s, request from %s", rev.UserID, userID)
	}

	var frameRows []actionRevisionFrameRow
	err = r.db.WithContext(ctx).
		Where("revision_id = ?", actionRevisionID).
		Order("logical_index ASC").
		Find(&frameRows).Error
	if err != nil {
		return nil, err
	}

	assetIDs := make([]string, 0, len(frameRows))
	for _, f := range frameRows {
		if f.AssetID != "" {
			assetIDs = append(assetIDs, f.AssetID)
		}
	}

	assetMap := make(map[string]frameAssetRow)
	if len(assetIDs) > 0 {
		var assets []frameAssetRow
		err = r.db.WithContext(ctx).
			Where("id IN ?", assetIDs).
			Find(&assets).Error
		if err != nil {
			return nil, err
		}
		for _, a := range assets {
			assetMap[a.ID] = a
		}
	}

	frames := make([]quality.QualityFrameInput, 0, len(frameRows))
	for _, f := range frameRows {
		if f.AssetID == "" {
			return nil, fmt.Errorf("frame asset missing: frame %d has no asset ID", f.LogicalIndex)
		}

		var transforms []quality.Transform
		if f.TransformJSON != "" {
			if err := json.Unmarshal([]byte(f.TransformJSON), &transforms); err != nil {
				return nil, fmt.Errorf("transform JSON invalid (frame %d): %w", f.LogicalIndex, err)
			}
		}
		if transforms == nil {
			transforms = []quality.Transform{}
		}

		var measurements map[string]float64
		if f.MetadataJSON != "" {
			if err := json.Unmarshal([]byte(f.MetadataJSON), &measurements); err != nil {
				return nil, fmt.Errorf("metadata JSON invalid (frame %d): %w", f.LogicalIndex, err)
			}
		}
		if measurements == nil {
			measurements = map[string]float64{}
		}

		asset, ok := assetMap[f.AssetID]
		if !ok || asset.ID == "" {
			return nil, fmt.Errorf("frame asset not found: frame %d asset %s", f.LogicalIndex, f.AssetID)
		}

		if asset.ContentHash == "" {
			return nil, fmt.Errorf("frame asset missing: frame %d has empty content hash", f.LogicalIndex)
		}

		frame := quality.QualityFrameInput{
			FrameRevisionID: f.ID,
			FrameArtifactID: f.AssetID,
			FrameIndex:      f.LogicalIndex,
			ContentHash:     asset.ContentHash,
			MimeType:        asset.MimeType,
			Width:           asset.Width,
			Height:          asset.Height,
			Anchor: quality.Point{
				X: f.AnchorX,
				Y: f.AnchorY,
			},
			CoordinateSpace: f.AnchorSpace,
			TransformChain:  transforms,
			Measurements:    measurements,
			SubjectBox:      quality.Rect{},
		}

		if asset.StoragePath != "" {
			relPath := asset.StoragePath
			if absPath, pathErr := r.safePath(relPath); pathErr == nil {
				frame.AbsolutePath = absPath
				frame.RelativePath = relPath
			} else {
				frame.RelativePath = relPath
			}
		}

		frames = append(frames, frame)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	snapshot := &quality.EvaluationInputSnapshot{
		ID:                   fmt.Sprintf("is-%d", time.Now().UnixNano()),
		UserID:               rev.UserID,
		CharacterID:          rev.CharacterID,
		ActionStreamID:       rev.ID,
		ActionRevisionID:     rev.ID,
		ActionContentHash:    rev.ContentHash,
		ProcessingRevisionID: rev.SourceProcessingRevisionID,
		ActionKey:            rev.ActionKey,
		PlaybackMode:         rev.PlaybackMode,
		FPS:                  rev.DefaultFPS,
		ExpectedFrameCount:   rev.FrameCount,
		CreatedAt:            now,
	}

	return &quality.QualityActionInput{
		UserID:               rev.UserID,
		CharacterID:          rev.CharacterID,
		ProcessingTaskID:     rev.ProcessingTaskID,
		ProcessingActionID:   rev.ProcessingActionID,
		ActionKey:            rev.ActionKey,
		ActionRevisionID:     rev.ID,
		ActionContentHash:    rev.ContentHash,
		ProcessingRevisionID: rev.SourceProcessingRevisionID,
		PlaybackMode:         rev.PlaybackMode,
		FPS:                  rev.DefaultFPS,
		ExpectedFrameCount:   rev.FrameCount,
		Frames:               frames,
		InputSource:          quality.InputSourceNewBridge,
		InputSnapshotID:      snapshot.ID,
		InputSnapshot:        snapshot,
	}, nil
}
