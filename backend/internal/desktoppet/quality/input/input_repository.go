// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package input

import (
	"context"
	"encoding/json"
	"path/filepath"

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
		var transforms []quality.Transform
		if f.TransformJSON != "" {
			_ = json.Unmarshal([]byte(f.TransformJSON), &transforms)
		}
		if transforms == nil {
			transforms = []quality.Transform{}
		}

		var measurements map[string]float64
		if f.MetadataJSON != "" {
			_ = json.Unmarshal([]byte(f.MetadataJSON), &measurements)
		}
		if measurements == nil {
			measurements = map[string]float64{}
		}

		frame := quality.QualityFrameInput{
			FrameRevisionID: f.ID,
			FrameArtifactID: f.AssetID,
			FrameIndex:      f.LogicalIndex,
			Anchor: quality.Point{
				X: f.AnchorX,
				Y: f.AnchorY,
			},
			CoordinateSpace: f.AnchorSpace,
			TransformChain:  transforms,
			Measurements:    measurements,
			SubjectBox:      quality.Rect{},
		}

		if asset, ok := assetMap[f.AssetID]; ok {
			frame.ContentHash = asset.ContentHash
			frame.RelativePath = asset.StoragePath
			frame.MimeType = asset.MimeType
			frame.Width = asset.Width
			frame.Height = asset.Height
			if asset.StoragePath != "" {
				frame.AbsolutePath = filepath.Join(r.dataDir, asset.StoragePath)
			}
		}

		frames = append(frames, frame)
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
	}, nil
}
