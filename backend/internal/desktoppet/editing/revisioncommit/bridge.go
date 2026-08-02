package revisioncommit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/editing"
	"github.com/u-ai/backend/internal/desktoppet/editing/baseline"
	"github.com/u-ai/backend/internal/desktoppet/editing/binding"
	"github.com/u-ai/backend/internal/desktoppet/editing/invalidation"
	"github.com/u-ai/backend/log"
)

type CommitFromProcessingRequest struct {
	UserID               string
	CharacterID          string
	ProcessingTaskID     string
	ProcessingActionID   string
	ProcessingRevisionID string
	ActionKey            string
	ActionConfigJSON     string
	ActionConfigHash     string
	ActionSpecVersion    string
	FrameCount           int
	PlaybackMode         string
	FPS                  int
	AnchorJSON           string
	FrameDurationMS      int
	LoopType             string
	PromotionPolicy      string
	CreatedBy            string
}

type RevisionBridge interface {
	CommitFromProcessingRevision(ctx context.Context, req CommitFromProcessingRequest) (*editing.ActionRevision, error)
	RecoverPending(ctx context.Context) error
}

type bridge struct {
	journalRepo BridgeJournalRepository
	baselineSvc baseline.BaselineRevisionService
	bindingSvc  binding.ActiveRevisionBindingService
	outbox      invalidation.ActionRevisionEventOutbox
}

func NewBridge(
	journalRepo BridgeJournalRepository,
	baselineSvc baseline.BaselineRevisionService,
	bindingSvc binding.ActiveRevisionBindingService,
	outbox invalidation.ActionRevisionEventOutbox,
) RevisionBridge {
	return &bridge{
		journalRepo: journalRepo,
		baselineSvc: baselineSvc,
		bindingSvc:  bindingSvc,
		outbox:      outbox,
	}
}

func (b *bridge) CommitFromProcessingRevision(ctx context.Context, req CommitFromProcessingRequest) (*editing.ActionRevision, error) {
	existing, err := b.journalRepo.GetByProcessingRevision(req.ProcessingRevisionID)
	if err != nil {
		return nil, fmt.Errorf("查询已有桥接日志失败: %w", err)
	}
	if existing != nil && existing.Status == baseline.BridgeStatusCompleted && existing.ActionRevisionID != "" {
		rev, gErr := b.baselineSvc.GetRevision(ctx, req.UserID, existing.ActionRevisionID)
		if gErr != nil {
			return nil, fmt.Errorf("查询已完成Revision失败: %w", gErr)
		}
		if rev != nil {
			return rev, nil
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	journalID := fmt.Sprintf("bridge_%d", time.Now().UnixNano())
	journal := &editing.RevisionBridgeJournal{
		ID:                   journalID,
		ProcessingRevisionID: req.ProcessingRevisionID,
		ProcessingActionID:   req.ProcessingActionID,
		TargetActionKey:      req.ActionKey,
		Status:               baseline.BridgeStatusProcessingPublished,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := b.journalRepo.Create(journal); err != nil {
		return nil, fmt.Errorf("创建桥接日志失败: %w", err)
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, b.failJournal(journalID, fmt.Errorf("序列化请求快照失败: %w", err))
	}
	snapshot := &BridgeRequestSnapshot{
		JournalID:   journalID,
		RequestJSON: string(reqJSON),
		CreatedAt:   now,
	}
	if err := b.journalRepo.SaveRequestSnapshot(snapshot); err != nil {
		return nil, b.failJournal(journalID, fmt.Errorf("保存请求快照失败: %w", err))
	}

	createdRev, err := b.baselineSvc.CreateFromProcessingRevision(ctx, b.toBaselineRequest(req))
	if err != nil {
		return nil, b.failJournal(journalID, fmt.Errorf("创建ActionRevision失败: %w", err))
	}

	if err := b.journalRepo.UpdateActionRevisionID(journalID, createdRev.ID); err != nil {
		return nil, b.failJournal(journalID, fmt.Errorf("更新桥接日志ActionRevisionID失败: %w", err))
	}
	if err := b.journalRepo.UpdateStatus(journalID, baseline.BridgeStatusActionRevisionCreated, ""); err != nil {
		return nil, b.failJournal(journalID, fmt.Errorf("更新桥接日志状态失败: %w", err))
	}

	var (
		activatedBinding        *editing.ActiveActionRevisionBinding
		previousRevisionID      string
		expectedBindingRevision int64
	)
	shouldBind := false

	switch req.PromotionPolicy {
	case baseline.PromotionPolicyFirstRevisionOnly:
		existingBinding, gErr := b.bindingSvc.GetActiveBinding(ctx, req.UserID, req.CharacterID, req.ActionKey)
		if gErr != nil {
			return nil, b.failJournal(journalID, fmt.Errorf("查询活跃绑定失败: %w", gErr))
		}
		if existingBinding == nil {
			shouldBind = true
		} else {
			previousRevisionID = existingBinding.ActiveActionRevisionID
		}
	case baseline.PromotionPolicyAlways:
		shouldBind = true
		existingBinding, gErr := b.bindingSvc.GetActiveBinding(ctx, req.UserID, req.CharacterID, req.ActionKey)
		if gErr != nil {
			return nil, b.failJournal(journalID, fmt.Errorf("查询活跃绑定失败: %w", gErr))
		}
		if existingBinding != nil {
			previousRevisionID = existingBinding.ActiveActionRevisionID
			expectedBindingRevision = existingBinding.BindingRevision
		}
	case baseline.PromotionPolicyManual:
		shouldBind = false
	default:
		shouldBind = false
	}

	if shouldBind {
		bindReq := binding.BindActiveRevisionRequest{
			UserID:                  req.UserID,
			CharacterID:             req.CharacterID,
			ActionKey:               req.ActionKey,
			TargetRevisionID:        createdRev.ID,
			ExpectedBindingRevision: expectedBindingRevision,
			Reason:                  "processing_baseline_promotion",
			Actor:                   req.CreatedBy,
		}
		activatedBinding, err = b.bindingSvc.Bind(ctx, bindReq)
		if err != nil {
			return nil, b.failJournal(journalID, fmt.Errorf("激活绑定失败: %w", err))
		}
		if uErr := b.journalRepo.UpdateStatus(journalID, baseline.BridgeStatusBindingActivated, ""); uErr != nil {
			return nil, b.failJournal(journalID, fmt.Errorf("更新桥接日志绑定状态失败: %w", uErr))
		}
	}

	occurredAt := time.Now().UTC().Format(time.RFC3339)
	var bindingRevision int64
	if activatedBinding != nil {
		bindingRevision = activatedBinding.BindingRevision
	}

	createdEvent := baseline.ActionRevisionEvent{
		EventID:              fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		UserID:               req.UserID,
		CharacterID:          req.CharacterID,
		ActionKey:            req.ActionKey,
		ActionRevisionID:     createdRev.ID,
		PreviousRevisionID:   previousRevisionID,
		ProcessingRevisionID: req.ProcessingRevisionID,
		BindingRevision:      bindingRevision,
		Reason:               "processing_baseline_promotion",
		OccurredAt:           occurredAt,
	}
	if err := b.outbox.PublishCreated(ctx, createdEvent); err != nil {
		return nil, b.failJournal(journalID, fmt.Errorf("发布创建事件失败: %w", err))
	}

	if activatedBinding != nil {
		activatedEvent := baseline.ActionRevisionEvent{
			EventID:              fmt.Sprintf("evt_%d", time.Now().UnixNano()),
			UserID:               req.UserID,
			CharacterID:          req.CharacterID,
			ActionKey:            req.ActionKey,
			ActionRevisionID:     createdRev.ID,
			PreviousRevisionID:   previousRevisionID,
			ProcessingRevisionID: req.ProcessingRevisionID,
			BindingRevision:      activatedBinding.BindingRevision,
			Reason:               "processing_baseline_promotion",
			OccurredAt:           time.Now().UTC().Format(time.RFC3339),
		}
		if err := b.outbox.PublishActivated(ctx, activatedEvent); err != nil {
			return nil, b.failJournal(journalID, fmt.Errorf("发布激活事件失败: %w", err))
		}
	}

	if err := b.journalRepo.UpdateStatus(journalID, baseline.BridgeStatusCompleted, ""); err != nil {
		return nil, b.failJournal(journalID, fmt.Errorf("更新桥接日志完成状态失败: %w", err))
	}

	return createdRev, nil
}

func (b *bridge) RecoverPending(ctx context.Context) error {
	journals, err := b.journalRepo.ListPending()
	if err != nil {
		return fmt.Errorf("查询待恢复桥接日志失败: %w", err)
	}
	for i := range journals {
		if err := b.recoverOne(ctx, &journals[i]); err != nil {
			log.Logger.Errorf("恢复桥接日志失败: journalId=%s err=%v", journals[i].ID, err)
		}
	}
	return nil
}

func (b *bridge) recoverOne(ctx context.Context, j *editing.RevisionBridgeJournal) error {
	snap, err := b.journalRepo.GetRequestSnapshot(j.ID)
	if err != nil || snap == nil {
		log.Logger.Infof("恢复跳过: 缺少请求快照 journalId=%s", j.ID)
		return b.journalRepo.IncrementRetryCount(j.ID)
	}
	var req CommitFromProcessingRequest
	if err := json.Unmarshal([]byte(snap.RequestJSON), &req); err != nil {
		log.Logger.Errorf("恢复失败: 反序列化请求快照失败 journalId=%s err=%v", j.ID, err)
		return b.journalRepo.IncrementRetryCount(j.ID)
	}

	rev, err := b.ensureRevisionCreated(ctx, j, &req)
	if err != nil {
		log.Logger.Errorf("恢复创建ActionRevision失败: journalId=%s err=%v", j.ID, err)
		return b.journalRepo.IncrementRetryCount(j.ID)
	}
	if rev == nil {
		log.Logger.Infof("恢复跳过: 无法获取Revision journalId=%s", j.ID)
		return b.journalRepo.IncrementRetryCount(j.ID)
	}

	bound, err := b.ensureBinding(ctx, j, &req)
	if err != nil {
		log.Logger.Errorf("恢复绑定失败: journalId=%s err=%v", j.ID, err)
		return b.journalRepo.IncrementRetryCount(j.ID)
	}

	b.recoverPublishEvents(ctx, j, &req, bound)

	return b.journalRepo.UpdateStatus(j.ID, baseline.BridgeStatusCompleted, "")
}

func (b *bridge) ensureRevisionCreated(ctx context.Context, j *editing.RevisionBridgeJournal, req *CommitFromProcessingRequest) (*editing.ActionRevision, error) {
	if j.ActionRevisionID != "" {
		rev, err := b.baselineSvc.GetRevision(ctx, req.UserID, j.ActionRevisionID)
		if err != nil {
			return nil, err
		}
		if rev != nil {
			return rev, nil
		}
	}
	rev, err := b.baselineSvc.CreateFromProcessingRevision(ctx, b.toBaselineRequest(*req))
	if err != nil {
		return nil, err
	}
	if err := b.journalRepo.UpdateActionRevisionID(j.ID, rev.ID); err != nil {
		return nil, err
	}
	j.ActionRevisionID = rev.ID
	return rev, nil
}

func (b *bridge) ensureBinding(ctx context.Context, j *editing.RevisionBridgeJournal, req *CommitFromProcessingRequest) (*editing.ActiveActionRevisionBinding, error) {
	existing, err := b.bindingSvc.GetActiveBinding(ctx, req.UserID, req.CharacterID, req.ActionKey)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.ActiveActionRevisionID == j.ActionRevisionID {
		return existing, nil
	}
	switch req.PromotionPolicy {
	case baseline.PromotionPolicyFirstRevisionOnly:
		if existing != nil {
			return nil, nil
		}
	case baseline.PromotionPolicyAlways:
	case baseline.PromotionPolicyManual:
		return nil, nil
	default:
		return nil, nil
	}
	bindReq := binding.BindActiveRevisionRequest{
		UserID:                  req.UserID,
		CharacterID:             req.CharacterID,
		ActionKey:               req.ActionKey,
		TargetRevisionID:        j.ActionRevisionID,
		ExpectedBindingRevision: 0,
		Reason:                  "processing_baseline_promotion_recovery",
		Actor:                   req.CreatedBy,
	}
	if existing != nil {
		bindReq.ExpectedBindingRevision = existing.BindingRevision
	}
	bound, err := b.bindingSvc.Bind(ctx, bindReq)
	if err != nil {
		return nil, err
	}
	if uErr := b.journalRepo.UpdateStatus(j.ID, baseline.BridgeStatusBindingActivated, ""); uErr != nil {
		return nil, uErr
	}
	return bound, nil
}

func (b *bridge) recoverPublishEvents(ctx context.Context, j *editing.RevisionBridgeJournal, req *CommitFromProcessingRequest, bound *editing.ActiveActionRevisionBinding) {
	existing, _ := b.bindingSvc.GetActiveBinding(ctx, req.UserID, req.CharacterID, req.ActionKey)
	var bindingRevision int64
	var previousRevisionID string
	isActivelyBound := false
	if existing != nil {
		bindingRevision = existing.BindingRevision
		if existing.ActiveActionRevisionID == j.ActionRevisionID {
			isActivelyBound = true
		} else {
			previousRevisionID = existing.ActiveActionRevisionID
		}
	}
	if bound != nil {
		bindingRevision = bound.BindingRevision
		isActivelyBound = true
	}

	occurredAt := time.Now().UTC().Format(time.RFC3339)
	createdEvent := baseline.ActionRevisionEvent{
		EventID:              fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		UserID:               req.UserID,
		CharacterID:          req.CharacterID,
		ActionKey:            req.ActionKey,
		ActionRevisionID:     j.ActionRevisionID,
		PreviousRevisionID:   previousRevisionID,
		ProcessingRevisionID: req.ProcessingRevisionID,
		BindingRevision:      bindingRevision,
		Reason:               "processing_baseline_promotion_recovery",
		OccurredAt:           occurredAt,
	}
	if err := b.outbox.PublishCreated(ctx, createdEvent); err != nil {
		log.Logger.Errorf("恢复发布创建事件失败: journalId=%s err=%v", j.ID, err)
	}

	if isActivelyBound {
		activatedEvent := baseline.ActionRevisionEvent{
			EventID:              fmt.Sprintf("evt_%d", time.Now().UnixNano()),
			UserID:               req.UserID,
			CharacterID:          req.CharacterID,
			ActionKey:            req.ActionKey,
			ActionRevisionID:     j.ActionRevisionID,
			PreviousRevisionID:   previousRevisionID,
			ProcessingRevisionID: req.ProcessingRevisionID,
			BindingRevision:      bindingRevision,
			Reason:               "processing_baseline_promotion_recovery",
			OccurredAt:           time.Now().UTC().Format(time.RFC3339),
		}
		if err := b.outbox.PublishActivated(ctx, activatedEvent); err != nil {
			log.Logger.Errorf("恢复发布激活事件失败: journalId=%s err=%v", j.ID, err)
		}
	}
}

func (b *bridge) toBaselineRequest(req CommitFromProcessingRequest) baseline.CreateBaselineRevisionRequest {
	return baseline.CreateBaselineRevisionRequest{
		UserID:               req.UserID,
		CharacterID:          req.CharacterID,
		ProcessingTaskID:     req.ProcessingTaskID,
		ProcessingActionID:   req.ProcessingActionID,
		ProcessingRevisionID: req.ProcessingRevisionID,
		ActionKey:            req.ActionKey,
		ActionConfigJSON:     req.ActionConfigJSON,
		ActionConfigHash:     req.ActionConfigHash,
		ActionSpecVersion:    req.ActionSpecVersion,
		FrameCount:           req.FrameCount,
		PlaybackMode:         req.PlaybackMode,
		FPS:                  req.FPS,
		AnchorJSON:           req.AnchorJSON,
		FrameDurationMS:      req.FrameDurationMS,
		LoopType:             req.LoopType,
		PromotionPolicy:      req.PromotionPolicy,
		CreatedBy:            req.CreatedBy,
	}
}

func (b *bridge) failJournal(journalID string, cause error) error {
	if err := b.journalRepo.UpdateStatus(journalID, baseline.BridgeStatusFailed, cause.Error()); err != nil {
		log.Logger.Errorf("更新桥接日志失败状态失败: journalId=%s cause=%v dbErr=%v", journalID, cause, err)
	}
	return fmt.Errorf("%w: %v", editing.ErrBridgeFailed, cause)
}
