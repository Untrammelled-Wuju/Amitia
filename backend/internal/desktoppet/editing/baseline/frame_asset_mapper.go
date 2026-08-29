package baseline

import (
	"fmt"
	"github.com/google/uuid"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/editing"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/internal/desktoppet/processing/contracts"
	"gorm.io/gorm"
)

type FrameAssetMapping struct {
	Artifact processing.ProcessingArtifactRecord
	Asset    *editing.FrameAsset
	Created  bool
}

type FrameAssetMapper struct {
	db *gorm.DB
}

func NewFrameAssetMapper(db *gorm.DB) *FrameAssetMapper {
	return &FrameAssetMapper{db: db}
}

func (m *FrameAssetMapper) MapArtifactsToAssets(
	tx *gorm.DB,
	userID, characterID string,
	processingRevisionID string,
	artifacts []processing.ProcessingArtifactRecord,
) ([]FrameAssetMapping, error) {
	mappings := make([]FrameAssetMapping, 0, len(artifacts))
	now := time.Now().UTC().Format(time.RFC3339)

	for _, art := range artifacts {
		if art.ArtifactKind != contracts.ArtifactKindFrame {
			continue
		}

		existing, err := m.findExistingAsset(tx, art.ID)
		if err != nil {
			return nil, fmt.Errorf("查找已有FrameAsset失败(artifact=%s): %w", art.ID, err)
		}

		if existing != nil {
			mappings = append(mappings, FrameAssetMapping{
				Artifact: art,
				Asset:    existing,
				Created:  false,
			})
			continue
		}

		asset := &editing.FrameAsset{
			ID:                         fmt.Sprintf("fa-%s", art.ID),
			UserID:                     userID,
			CharacterID:                characterID,
			ContentHash:                art.ContentHash,
			StoragePath:                art.RelativePath,
			StorageKey:                 art.RelativePath,
			MimeType:                   art.MimeType,
			Width:                      art.Width,
			Height:                     art.Height,
			ByteSize:                   art.ByteSize,
			SourceType:                 editing.AssetSourceProcessed,
			SourceRefID:                art.ID,
			SourceProcessingRevisionID: processingRevisionID,
			SourceProcessingArtifactID: art.ID,
			OriginalHash:               art.ContentHash,
			CreatedBy:                  "system",
			Status:                     editing.AssetStatusReady,
			CreatedAt:                  now,
		}

		if err := tx.Create(asset).Error; err != nil {
			return nil, fmt.Errorf("创建FrameAsset失败(artifact=%s): %w", art.ID, err)
		}

		mappings = append(mappings, FrameAssetMapping{
			Artifact: art,
			Asset:    asset,
			Created:  true,
		})
	}

	return mappings, nil
}

func (m *FrameAssetMapper) findExistingAsset(tx *gorm.DB, artifactID string) (*editing.FrameAsset, error) {
	var asset editing.FrameAsset
	err := tx.Where("source_processing_artifact_id = ? AND source_type = ?",
		artifactID, editing.AssetSourceProcessed).First(&asset).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &asset, nil
}

func (m *FrameAssetMapper) BuildFrameHashInfos(mappings []FrameAssetMapping, frameDurationMS int) []FrameHashInfo {
	infos := make([]FrameHashInfo, 0, len(mappings))
	for _, mp := range mappings {
		idx := 0
		if mp.Artifact.FrameIndex != nil {
			idx = *mp.Artifact.FrameIndex
		}
		infos = append(infos, FrameHashInfo{
			Index:       idx,
			ContentHash: mp.Asset.ContentHash,
			DurationMS:  frameDurationMS,
		})
	}
	return infos
}

func (m *FrameAssetMapper) BuildRevisionFrames(
	revisionID, processingRevisionID, processingAttemptID string,
	mappings []FrameAssetMapping,
	frameDurationMS int,
	now string,
) []editing.ActionRevisionFrame {
	frames := make([]editing.ActionRevisionFrame, 0, len(mappings))
    for _, mp := range mappings {
		idx := 0
		if mp.Artifact.FrameIndex != nil {
			idx = *mp.Artifact.FrameIndex
		}
		frames = append(frames, editing.ActionRevisionFrame{
			ID:                              "rf-" + uuid.NewString(),
			RevisionID:                      revisionID,
			FrameID:                         mp.Artifact.ID,
			AssetID:                         mp.Asset.ID,
			LogicalIndex:                    idx,
			DurationMS:                      frameDurationMS,
			SourceFrameID:                   mp.Artifact.ID,
			SourceRevisionID:                processingRevisionID,
			SourceAttemptID:                 processingAttemptID,
			SourceProcessingFrameArtifactID: mp.Artifact.ID,
			SourceProcessingRevisionID:      processingRevisionID,
			SourceProcessingAttemptID:       processingAttemptID,
			AnchorX:                         0.5,
			AnchorY:                         0.9,
			AnchorSpace:                     editing.AnchorSpaceNormalizedCanvas,
			CreatedAt:                       now,
		})
	}
	return frames
}
