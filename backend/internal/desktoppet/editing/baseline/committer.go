package baseline

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/editing"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/internal/desktoppet/processing/contracts"
	"gorm.io/gorm"
)

type CommitterRequest struct {
	UserID               string
	CharacterID          string
	ProcessingTaskID     string
	ProcessingActionID   string
	ProcessingAttemptID  string
	ProcessingRevisionID string
	ActionKey            string
	ActionConfigJSON     string
	ActionConfigHash     string
	ActionSpecVersion    string
	ActionSpecHash       string
	PlaybackMode         string
	FPS                  int
	FrameDurationMS      int
	LoopType             string
	AnchorJSON           string
	PromotionPolicy      string
	CreatedBy            string
	CorrelationID        string
}

type CommitterResult struct {
	ActionRevisionID string
	RevisionNumber   int64
	ActionStreamID   string
	BindingRevision  int64
	Bound            bool
	ContentHash      string
	FrameSetHash     string
}

type BaselineActionRevisionCommitter struct {
	db     *gorm.DB
	mapper *FrameAssetMapper
}

func NewBaselineActionRevisionCommitter(db *gorm.DB) *BaselineActionRevisionCommitter {
	return &BaselineActionRevisionCommitter{
		db:     db,
		mapper: NewFrameAssetMapper(db),
	}
}

func (c *BaselineActionRevisionCommitter) Commit(req CommitterRequest, procRev *processing.ProcessingRevision, artifacts []processing.ProcessingArtifactRecord) (*CommitterResult, error) {
	result := &CommitterResult{}

	err := c.db.Transaction(func(tx *gorm.DB) error {
		stream, err := c.getOrCreateStream(tx, req.UserID, req.CharacterID, req.ActionKey, req.ProcessingTaskID)
		if err != nil {
			return fmt.Errorf("获取或创建ActionStream失败: %w", err)
		}
		result.ActionStreamID = stream.ID

		revisionNumber, err := c.allocateRevisionNumber(tx, stream.ID, stream.NextRevisionNumber)
		if err != nil {
			return fmt.Errorf("分配RevisionNumber失败: %w", err)
		}
		result.RevisionNumber = revisionNumber

		mappings, err := c.mapper.MapArtifactsToAssets(tx, req.UserID, req.CharacterID, req.ProcessingRevisionID, artifacts)
		if err != nil {
			return fmt.Errorf("映射FrameAssets失败: %w", err)
		}

		frameHashInfos := c.mapper.BuildFrameHashInfos(mappings, req.FrameDurationMS)
		anchor, err := parseAnchor(req.AnchorJSON)
		if err != nil {
			return fmt.Errorf("解析Anchor失败: %w", err)
		}

		cfgHashInfo := ActionConfigHashInfo{
			ActionKey:        req.ActionKey,
			ActionSpecHash:   req.ActionSpecHash,
			ActionConfigHash: req.ActionConfigHash,
			PlaybackMode:     req.PlaybackMode,
			FPS:              req.FPS,
			Anchor:           anchor,
		}

		contentHash, err := ComputeContentHashV2(cfgHashInfo, frameHashInfos)
		if err != nil {
			return fmt.Errorf("计算ContentHash V2失败: %w", err)
		}
		frameSetHash, err := ComputeFrameSetHashV2(frameHashInfos)
		if err != nil {
			return fmt.Errorf("计算FrameSetHash V2失败: %w", err)
		}
		result.ContentHash = contentHash
		result.FrameSetHash = frameSetHash

		now := time.Now().UTC().Format(time.RFC3339)
		revisionID := fmt.Sprintf("ar-%d", time.Now().UnixNano())

		frameCount := len(mappings)

		rev := &editing.ActionRevision{
			ID:                         revisionID,
			UserID:                     req.UserID,
			CharacterID:                req.CharacterID,
			ProcessingTaskID:           req.ProcessingTaskID,
			ProcessingActionID:         req.ProcessingActionID,
			ActionKey:                  req.ActionKey,
			RootRevisionID:             revisionID,
			RevisionNumber:             int(revisionNumber),
			RevisionType:               editing.RevisionTypeProcessed,
			Status:                     RevisionStatusCommitting,
			FrameCount:                 frameCount,
			DefaultFPS:                 req.FPS,
			LoopType:                   req.LoopType,
			CreatedByUserID:            req.CreatedBy,
			CreatedAt:                  now,
			UpdatedAt:                  now,
			SourceType:                 SourceTypeProcessingBaseline,
			SourceProcessingRevisionID: req.ProcessingRevisionID,
			ContentHash:                contentHash,
			ContentHashVersion:         ContentHashVersionV2,
			ActionConfigHash:           req.ActionConfigHash,
			FrameSetHash:               frameSetHash,
			Origin:                     OriginSystem,
			PlaybackMode:               req.PlaybackMode,
			AnchorJSON:                 req.AnchorJSON,
			ActionStreamID:             stream.ID,
			SourceProcessingTaskID:     req.ProcessingTaskID,
			SourceProcessingActionID:   req.ProcessingActionID,
			SourceProcessingAttemptID:  req.ProcessingAttemptID,
			ParentActionRevisionID:     "",
			RootActionRevisionID:       revisionID,
			ActionConfigSnapshotJSON:   req.ActionConfigJSON,
			ActionSpecHash:             req.ActionSpecHash,
		}

		if err := tx.Create(rev).Error; err != nil {
			return fmt.Errorf("创建ActionRevision失败: %w", err)
		}

		frames := c.mapper.BuildRevisionFrames(
			revisionID, req.ProcessingRevisionID, req.ProcessingAttemptID,
			mappings, req.FrameDurationMS, now,
		)
		if len(frames) > 0 {
			if err := tx.CreateInBatches(frames, 100).Error; err != nil {
				return fmt.Errorf("创建RevisionFrames失败: %w", err)
			}
		}

		bound, bindingRevision, err := c.activateBinding(tx, stream, revisionID, req, now)
		if err != nil {
			return fmt.Errorf("激活Binding失败: %w", err)
		}
		result.Bound = bound
		result.BindingRevision = bindingRevision

		if err := c.recordBindingHistory(tx, stream.ID, bindingRevision, "", revisionID, req, now); err != nil {
			return fmt.Errorf("记录BindingHistory失败: %w", err)
		}

		if err := c.createOutboxEvents(tx, stream.ID, revisionID, "", req.ProcessingRevisionID, bindingRevision, req, now); err != nil {
			return fmt.Errorf("创建Outbox事件失败: %w", err)
		}

		if err := tx.Model(rev).Where("id = ?", revisionID).Updates(map[string]any{
			"status":     RevisionStatusReady,
			"ready_at":   now,
			"updated_at": now,
		}).Error; err != nil {
			return fmt.Errorf("更新Revision状态失败: %w", err)
		}

		result.ActionRevisionID = revisionID
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *BaselineActionRevisionCommitter) getOrCreateStream(tx *gorm.DB, userID, characterID, actionKey, processingTaskID string) (*editing.ActionStream, error) {
	streamKey := fmt.Sprintf("%s:%s:%s", userID, characterID, actionKey)

	var stream editing.ActionStream
	err := tx.Where("stream_key = ?", streamKey).First(&stream).Error
	if err == nil {
		return &stream, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	stream = editing.ActionStream{
		ID:                   fmt.Sprintf("as-%d", time.Now().UnixNano()),
		UserID:               userID,
		CharacterID:          characterID,
		ActionKey:            actionKey,
		RootProcessingTaskID: processingTaskID,
		StreamKey:            streamKey,
		NextRevisionNumber:   1,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	if err := tx.Create(&stream).Error; err != nil {
		var existing editing.ActionStream
		findErr := tx.Where("stream_key = ?", streamKey).First(&existing).Error
		if findErr != nil {
			return nil, fmt.Errorf("创建ActionStream后查找失败: %w (原错误: %v)", findErr, err)
		}
		return &existing, nil
	}
	return &stream, nil
}

func (c *BaselineActionRevisionCommitter) allocateRevisionNumber(tx *gorm.DB, streamID string, expectedNext int64) (int64, error) {
	result := tx.Model(&editing.ActionStream{}).
		Where("id = ? AND next_revision_number = ?", streamID, expectedNext).
		Updates(map[string]any{
			"next_revision_number": expectedNext + 1,
			"updated_at":           time.Now().UTC().Format(time.RFC3339),
		})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		var stream editing.ActionStream
		if err := tx.Where("id = ?", streamID).First(&stream).Error; err != nil {
			return 0, fmt.Errorf("CAS失败后查询ActionStream失败: %w", err)
		}
		return c.allocateRevisionNumber(tx, streamID, stream.NextRevisionNumber)
	}
	return expectedNext, nil
}

func (c *BaselineActionRevisionCommitter) activateBinding(tx *gorm.DB, stream *editing.ActionStream, revisionID string, req CommitterRequest, now string) (bool, int64, error) {
	var existing editing.ActiveActionRevisionBinding
	err := tx.Where("action_stream_id = ?", stream.ID).First(&existing).Error

	if err == nil {
		newBindingRevision := existing.BindingRevision + 1
		updateResult := tx.Model(&editing.ActiveActionRevisionBinding{}).
			Where("id = ? AND binding_revision = ?", existing.ID, existing.BindingRevision).
			Updates(map[string]any{
				"active_action_revision_id": revisionID,
				"binding_revision":          newBindingRevision,
				"bound_reason":              req.PromotionPolicy,
				"bound_by":                  req.CreatedBy,
				"bound_at":                  now,
				"updated_at":                now,
			})
		if updateResult.Error != nil {
			return false, 0, updateResult.Error
		}
		if updateResult.RowsAffected == 0 {
			return false, 0, editing.ErrActionRevisionBindingConflict
		}
		return true, newBindingRevision, nil
	}

	if err != gorm.ErrRecordNotFound {
		return false, 0, err
	}

	binding := &editing.ActiveActionRevisionBinding{
		ID:                     fmt.Sprintf("ab-%d", time.Now().UnixNano()),
		ActionStreamID:         stream.ID,
		UserID:                 stream.UserID,
		CharacterID:            stream.CharacterID,
		ActionKey:              stream.ActionKey,
		ActiveActionRevisionID: revisionID,
		BindingRevision:        1,
		BoundReason:            req.PromotionPolicy,
		BoundBy:                req.CreatedBy,
		BoundAt:                now,
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	if err := tx.Create(binding).Error; err != nil {
		var existingBinding editing.ActiveActionRevisionBinding
		findErr := tx.Where("action_stream_id = ?", stream.ID).First(&existingBinding).Error
		if findErr != nil {
			return false, 0, fmt.Errorf("创建Binding后查找失败: %w (原错误: %v)", findErr, err)
		}

		newBindingRevision := existingBinding.BindingRevision + 1
		updateResult := tx.Model(&editing.ActiveActionRevisionBinding{}).
			Where("id = ? AND binding_revision = ?", existingBinding.ID, existingBinding.BindingRevision).
			Updates(map[string]any{
				"active_action_revision_id": revisionID,
				"binding_revision":          newBindingRevision,
				"bound_reason":              req.PromotionPolicy,
				"bound_by":                  req.CreatedBy,
				"bound_at":                  now,
				"updated_at":                now,
			})
		if updateResult.Error != nil {
			return false, 0, updateResult.Error
		}
		return true, newBindingRevision, nil
	}

	return true, 1, nil
}

func (c *BaselineActionRevisionCommitter) recordBindingHistory(tx *gorm.DB, streamID string, bindingRevision int64, previousRevisionID, newRevisionID string, req CommitterRequest, now string) error {
	history := &editing.ActionRevisionBindingHistory{
		ID:                 fmt.Sprintf("bh-%d", time.Now().UnixNano()),
		ActionStreamID:     streamID,
		BindingRevision:    bindingRevision,
		PreviousRevisionID: previousRevisionID,
		NewRevisionID:      newRevisionID,
		Reason:             req.PromotionPolicy,
		Actor:              req.CreatedBy,
		OccurredAt:         now,
		CorrelationID:      req.CorrelationID,
	}
	return tx.Create(history).Error
}

func (c *BaselineActionRevisionCommitter) createOutboxEvents(tx *gorm.DB, streamID, revisionID, previousRevisionID, processingRevisionID string, bindingRevision int64, req CommitterRequest, now string) error {
	eventID := fmt.Sprintf("evt-%d", time.Now().UnixNano())

	createdPayload, _ := json.Marshal(map[string]any{
		"actionRevisionId":     revisionID,
		"actionStreamId":       streamID,
		"revisionNumber":       req.ProcessingRevisionID,
		"processingRevisionId": processingRevisionID,
		"actionKey":            req.ActionKey,
		"userId":               req.UserID,
		"characterId":          req.CharacterID,
		"contentHash":          "",
		"bindingRevision":      bindingRevision,
		"occurredAt":           now,
	})

	createdEvent := &editing.ActionRevisionEventOutboxRecord{
		ID:                   fmt.Sprintf("eo-%d", time.Now().UnixNano()),
		EventID:              eventID,
		EventType:            EventActionRevisionCreated,
		AggregateType:        "action_revision",
		AggregateID:          revisionID,
		AggregateSequence:    1,
		ActionStreamID:       streamID,
		ActionRevisionID:     revisionID,
		PreviousRevisionID:   previousRevisionID,
		ProcessingRevisionID: processingRevisionID,
		PayloadJSON:          string(createdPayload),
		Status:               OutboxStatusPending,
		AvailableAt:          now,
		CreatedAt:            now,
	}
	if err := tx.Create(createdEvent).Error; err != nil {
		return err
	}

	if req.PromotionPolicy == PromotionPolicyAlways || req.PromotionPolicy == PromotionPolicyFirstRevisionOnly {
		activatedEventID := fmt.Sprintf("evt-%d", time.Now().UnixNano()+1)
		activatedPayload, _ := json.Marshal(map[string]any{
			"actionRevisionId":   revisionID,
			"actionStreamId":     streamID,
			"bindingRevision":    bindingRevision,
			"previousRevisionId": previousRevisionID,
			"actionKey":          req.ActionKey,
			"userId":             req.UserID,
			"characterId":        req.CharacterID,
			"occurredAt":         now,
		})

		activatedEvent := &editing.ActionRevisionEventOutboxRecord{
			ID:                   fmt.Sprintf("eo-%d", time.Now().UnixNano()+1),
			EventID:              activatedEventID,
			EventType:            EventActionRevisionActivated,
			AggregateType:        "action_revision",
			AggregateID:          revisionID,
			AggregateSequence:    2,
			ActionStreamID:       streamID,
			ActionRevisionID:     revisionID,
			PreviousRevisionID:   previousRevisionID,
			ProcessingRevisionID: processingRevisionID,
			PayloadJSON:          string(activatedPayload),
			Status:               OutboxStatusPending,
			AvailableAt:          now,
			CreatedAt:            now,
		}
		if err := tx.Create(activatedEvent).Error; err != nil {
			return err
		}
	}

	return nil
}

func parseAnchor(anchorJSON string) (AnchorInfo, error) {
	var anchor AnchorInfo
	if anchorJSON == "" {
		return anchor, nil
	}
	err := json.Unmarshal([]byte(anchorJSON), &anchor)
	if err != nil {
		return anchor, fmt.Errorf("解析Anchor JSON失败: %w", err)
	}
	return anchor, nil
}

var _ = contracts.ArtifactKindFrame
