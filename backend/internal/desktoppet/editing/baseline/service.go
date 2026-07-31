package baseline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/editing"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/internal/desktoppet/processing/contracts"
)

type ProcessingRevisionReader interface {
	GetProcessingRevision(revisionID string) (*processing.ProcessingRevision, error)
	GetProcessingArtifacts(revisionID string) ([]processing.ProcessingArtifactRecord, error)
}

type BaselineRevisionService interface {
	CreateFromProcessingRevision(ctx context.Context, req CreateBaselineRevisionRequest) (*editing.ActionRevision, error)
	GetRevision(ctx context.Context, userID, revisionID string) (*editing.ActionRevision, error)
	UpdateQualitySnapshot(ctx context.Context, revisionID, evaluationID, verdict string, score *float64) error
	ArchiveRevision(ctx context.Context, userID, revisionID, reason string) error
}

type service struct {
	repo       Repository
	procReader ProcessingRevisionReader
}

func NewService(repo Repository, procReader ProcessingRevisionReader) BaselineRevisionService {
	return &service{repo: repo, procReader: procReader}
}

func (s *service) CreateFromProcessingRevision(ctx context.Context, req CreateBaselineRevisionRequest) (*editing.ActionRevision, error) {
	existing, err := s.repo.GetActionRevisionBySource(req.ProcessingRevisionID, SourceTypeProcessingBaseline)
	if err != nil {
		return nil, fmt.Errorf("查询已存在的Baseline Revision失败: %w", err)
	}
	if existing != nil {
		contentHash, err := s.computeContentHashForRequest(req)
		if err != nil {
			return nil, err
		}
		if existing.ContentHash != "" && existing.ContentHash != contentHash {
			return nil, editing.ErrContentHashMismatch
		}
		return existing, nil
	}

	procRev, err := s.procReader.GetProcessingRevision(req.ProcessingRevisionID)
	if err != nil {
		return nil, fmt.Errorf("获取ProcessingRevision失败: %w", err)
	}
	if procRev == nil {
		return nil, editing.ErrRevisionNotFound
	}

	artifacts, err := s.procReader.GetProcessingArtifacts(req.ProcessingRevisionID)
	if err != nil {
		return nil, fmt.Errorf("获取ProcessingArtifacts失败: %w", err)
	}

	frameHashInfos := s.buildFrameHashInfos(artifacts, req.FrameDurationMS)
	anchor, _ := s.parseAnchor(req.AnchorJSON)
	contentHash := ComputeContentHash(req.ActionKey, req.ActionSpecVersion, req.PlaybackMode, req.FPS, anchor, frameHashInfos)
	frameSetHash := ComputeFrameSetHash(frameHashInfos)

	status := RevisionStatusReady
	hasActive, err := s.hasActiveBinding(req.ProcessingTaskID, req.ActionKey)
	if err != nil {
		return nil, fmt.Errorf("检查Active绑定失败: %w", err)
	}
	if hasActive {
		status = RevisionStatusCandidate
	}

	revisionNumber, err := s.getNextRevisionNumber(req.ProcessingTaskID, req.ActionKey)
	if err != nil {
		return nil, fmt.Errorf("获取RevisionNumber失败: %w", err)
	}

	revisionID := fmt.Sprintf("ar-%d", time.Now().UnixNano())
	now := time.Now().UTC().Format(time.RFC3339)

	frameCount := req.FrameCount
	if frameCount == 0 {
		frameCount = procRev.FrameCount
	}

	rev := &editing.ActionRevision{
		ID:                         revisionID,
		UserID:                     req.UserID,
		CharacterID:                req.CharacterID,
		ProcessingTaskID:           req.ProcessingTaskID,
		ProcessingActionID:         req.ProcessingActionID,
		ActionKey:                  req.ActionKey,
		RootRevisionID:             revisionID,
		RevisionNumber:             revisionNumber,
		RevisionType:               editing.RevisionTypeProcessed,
		Status:                     status,
		FrameCount:                 frameCount,
		DefaultFPS:                 req.FPS,
		LoopType:                   req.LoopType,
		CreatedByUserID:            req.CreatedBy,
		CreatedAt:                  now,
		UpdatedAt:                  now,
		SourceType:                 SourceTypeProcessingBaseline,
		SourceProcessingRevisionID: req.ProcessingRevisionID,
		ContentHash:                contentHash,
		ContentHashVersion:         ContentHashVersionV1,
		ActionConfigHash:           req.ActionConfigHash,
		FrameSetHash:               frameSetHash,
		Origin:                     OriginSystem,
		PlaybackMode:               req.PlaybackMode,
		AnchorJSON:                 req.AnchorJSON,
	}
	if status == RevisionStatusReady {
		rev.ReadyAt = now
	}

	err = s.repo.CreateActionRevision(rev)
	if err != nil {
		return nil, fmt.Errorf("创建ActionRevision失败: %w", err)
	}

	frames := s.buildRevisionFrames(revisionID, req.ProcessingRevisionID, artifacts, req.FrameDurationMS, now)
	if len(frames) > 0 {
		err = s.repo.CreateRevisionFrames(frames)
		if err != nil {
			return nil, fmt.Errorf("创建RevisionFrames失败: %w", err)
		}
	}

	return rev, nil
}

func (s *service) GetRevision(ctx context.Context, userID, revisionID string) (*editing.ActionRevision, error) {
	return s.repo.GetActionRevisionForUser(userID, revisionID)
}

func (s *service) UpdateQualitySnapshot(ctx context.Context, revisionID, evaluationID, verdict string, score *float64) error {
	return s.repo.UpdateActionRevisionQuality(revisionID, evaluationID, verdict)
}

func (s *service) ArchiveRevision(ctx context.Context, userID, revisionID, reason string) error {
	_, err := s.repo.GetActionRevisionForUser(userID, revisionID)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	return s.repo.DB().Model(&editing.ActionRevision{}).Where("id = ?", revisionID).Updates(map[string]any{
		"status":          RevisionStatusArchived,
		"archived_at":     now,
		"archived_reason": reason,
		"updated_at":      now,
	}).Error
}

func (s *service) computeContentHashForRequest(req CreateBaselineRevisionRequest) (string, error) {
	_, err := s.procReader.GetProcessingRevision(req.ProcessingRevisionID)
	if err != nil {
		return "", fmt.Errorf("获取ProcessingRevision失败: %w", err)
	}
	artifacts, err := s.procReader.GetProcessingArtifacts(req.ProcessingRevisionID)
	if err != nil {
		return "", fmt.Errorf("获取ProcessingArtifacts失败: %w", err)
	}
	frameHashInfos := s.buildFrameHashInfos(artifacts, req.FrameDurationMS)
	anchor, _ := s.parseAnchor(req.AnchorJSON)
	return ComputeContentHash(req.ActionKey, req.ActionSpecVersion, req.PlaybackMode, req.FPS, anchor, frameHashInfos), nil
}

func (s *service) buildFrameHashInfos(artifacts []processing.ProcessingArtifactRecord, frameDurationMS int) []FrameHashInfo {
	infos := make([]FrameHashInfo, 0, len(artifacts))
	for _, a := range artifacts {
		if a.ArtifactKind != contracts.ArtifactKindFrame {
			continue
		}
		idx := 0
		if a.FrameIndex != nil {
			idx = *a.FrameIndex
		}
		infos = append(infos, FrameHashInfo{
			Index:        idx,
			ContentHash:  a.ContentHash,
			DurationMS:   frameDurationMS,
			RelativePath: a.RelativePath,
		})
	}
	return infos
}

func (s *service) parseAnchor(anchorJSON string) (AnchorInfo, error) {
	var anchor AnchorInfo
	if anchorJSON == "" {
		return anchor, nil
	}
	err := json.Unmarshal([]byte(anchorJSON), &anchor)
	return anchor, err
}

func (s *service) buildRevisionFrames(revisionID, processingRevisionID string, artifacts []processing.ProcessingArtifactRecord, frameDurationMS int, now string) []editing.ActionRevisionFrame {
	frames := make([]editing.ActionRevisionFrame, 0, len(artifacts))
	base := time.Now().UnixNano()
	seq := 0
	for _, a := range artifacts {
		if a.ArtifactKind != contracts.ArtifactKindFrame {
			continue
		}
		idx := 0
		if a.FrameIndex != nil {
			idx = *a.FrameIndex
		}
		frames = append(frames, editing.ActionRevisionFrame{
			ID:               fmt.Sprintf("rf-%d-%d", base, seq),
			RevisionID:       revisionID,
			FrameID:          a.ID,
			LogicalIndex:     idx,
			DurationMS:       frameDurationMS,
			SourceRevisionID: processingRevisionID,
			CreatedAt:        now,
		})
		seq++
	}
	return frames
}

func (s *service) getNextRevisionNumber(processingTaskID, actionKey string) (int, error) {
	var maxNum int
	err := s.repo.DB().Model(&editing.ActionRevision{}).
		Where("processing_task_id = ? AND action_key = ?", processingTaskID, actionKey).
		Select("COALESCE(MAX(revision_number), 0)").Scan(&maxNum).Error
	if err != nil {
		return 0, err
	}
	return maxNum + 1, nil
}

func (s *service) hasActiveBinding(processingTaskID, actionKey string) (bool, error) {
	var count int64
	err := s.repo.DB().Model(&editing.ActiveRevisionBinding{}).
		Where("processing_task_id = ? AND action_key = ?", processingTaskID, actionKey).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
