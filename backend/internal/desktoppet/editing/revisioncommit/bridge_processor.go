package revisioncommit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/editing"
	"github.com/u-ai/backend/internal/desktoppet/editing/baseline"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/log"
)

const (
	MaxInboxRetryAttempts  = 5
	MaxOutboxRetryAttempts = 5
	LeaseDuration          = 60 * time.Second
)

type ProcessingRevisionReader interface {
	GetProcessingRevision(revisionID string) (*processing.ProcessingRevision, error)
	GetProcessingArtifacts(revisionID string) ([]processing.ProcessingArtifactRecord, error)
}

type InboxEntryPayload struct {
	UserID               string `json:"userId"`
	CharacterID          string `json:"characterId"`
	ProcessingTaskID     string `json:"processingTaskId"`
	ProcessingActionID   string `json:"processingActionId"`
	ProcessingAttemptID  string `json:"processingAttemptId"`
	ProcessingRevisionID string `json:"processingRevisionId"`
	ActionKey            string `json:"actionKey"`
	ActionConfigJSON     string `json:"actionConfigJson"`
	ActionConfigHash     string `json:"actionConfigHash"`
	ActionSpecVersion    string `json:"actionSpecVersion"`
	ActionSpecHash       string `json:"actionSpecHash"`
	PlaybackMode         string `json:"playbackMode"`
	FPS                  int    `json:"fps"`
	FrameDurationMS      int    `json:"frameDurationMs"`
	LoopType             string `json:"loopType"`
	AnchorJSON           string `json:"anchorJson"`
	PromotionPolicy      string `json:"promotionPolicy"`
	CreatedBy            string `json:"createdBy"`
}

type BridgeProcessor struct {
	inboxRepo   BridgeInboxRepository
	journalRepo BridgeJournalRepository
	committer   *baseline.BaselineActionRevisionCommitter
	procReader  ProcessingRevisionReader
	outboxRepo  OutboxRepository
	eventPub    EventPublisher
	workerID    string
}

type EventPublisher interface {
	Publish(ctx context.Context, eventType string, payload []byte) error
}

func NewBridgeProcessor(
	inboxRepo BridgeInboxRepository,
	journalRepo BridgeJournalRepository,
	committer *baseline.BaselineActionRevisionCommitter,
	procReader ProcessingRevisionReader,
	outboxRepo OutboxRepository,
	eventPub EventPublisher,
	workerID string,
) *BridgeProcessor {
	return &BridgeProcessor{
		inboxRepo:   inboxRepo,
		journalRepo: journalRepo,
		committer:   committer,
		procReader:  procReader,
		outboxRepo:  outboxRepo,
		eventPub:    eventPub,
		workerID:    workerID,
	}
}

func (p *BridgeProcessor) SubmitToInbox(ctx context.Context, eventID string, payload InboxEntryPayload) error {
	existing, err := p.inboxRepo.GetByEventID(eventID)
	if err != nil {
		return fmt.Errorf("查询Inbox去重失败: %w", err)
	}
	if existing != nil {
		log.Logger.Infof("Inbox条目已存在，跳过: eventId=%s status=%s", eventID, existing.Status)
		return nil
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化Inbox Payload失败: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	entry := &editing.ActionRevisionBridgeInbox{
		ID:                   fmt.Sprintf("inbox-%d", time.Now().UnixNano()),
		EventID:              eventID,
		ProcessingRevisionID: payload.ProcessingRevisionID,
		PayloadJSON:          string(payloadJSON),
		Status:               baseline.InboxStatusReceived,
		ReceivedAt:           now,
	}
	return p.inboxRepo.Create(entry)
}

func (p *BridgeProcessor) ProcessPending(ctx context.Context, maxCount int) error {
	entries, err := p.inboxRepo.ListPending(maxCount)
	if err != nil {
		return fmt.Errorf("查询待处理Inbox条目失败: %w", err)
	}

	for i := range entries {
		if err := p.processOne(ctx, &entries[i]); err != nil {
			log.Logger.Errorf("处理Inbox条目失败: id=%s err=%v", entries[i].ID, err)
		}
	}
	return nil
}

func (p *BridgeProcessor) processOne(ctx context.Context, entry *editing.ActionRevisionBridgeInbox) error {
	acquired, err := p.inboxRepo.AcquireLease(entry.ID, p.workerID, LeaseDuration)
	if err != nil {
		return fmt.Errorf("获取Inbox租约失败: %w", err)
	}
	if !acquired {
		return nil
	}

	var payload InboxEntryPayload
	if err := json.Unmarshal([]byte(entry.PayloadJSON), &payload); err != nil {
		_ = p.inboxRepo.MarkFailedTerminal(entry.ID, fmt.Sprintf("反序列化Payload失败: %v", err))
		return err
	}

	if err := p.processPayload(ctx, entry, &payload); err != nil {
		_ = p.inboxRepo.IncrementAttemptCount(entry.ID)
		if entry.AttemptCount+1 >= MaxInboxRetryAttempts {
			_ = p.inboxRepo.MarkFailedTerminal(entry.ID, err.Error())
		} else {
			_ = p.inboxRepo.UpdateStatus(entry.ID, baseline.InboxStatusFailedRetryable, err.Error())
		}
		return err
	}

	return p.inboxRepo.MarkCompleted(entry.ID)
}

func (p *BridgeProcessor) processPayload(ctx context.Context, entry *editing.ActionRevisionBridgeInbox, payload *InboxEntryPayload) error {
	existingJournal, err := p.journalRepo.GetByProcessingRevision(payload.ProcessingRevisionID)
	if err != nil {
		return fmt.Errorf("查询已有Journal失败: %w", err)
	}
	if existingJournal != nil && existingJournal.Status == baseline.BridgeStatusCompleted && existingJournal.ActionRevisionID != "" {
		log.Logger.Infof("ProcessingRevision已桥接完成，跳过: id=%s revisionId=%s", payload.ProcessingRevisionID, existingJournal.ActionRevisionID)
		return nil
	}

	procRev, err := p.procReader.GetProcessingRevision(payload.ProcessingRevisionID)
	if err != nil {
		return fmt.Errorf("获取ProcessingRevision失败: %w", err)
	}
	if procRev == nil {
		return fmt.Errorf("ProcessingRevision不存在: %s", payload.ProcessingRevisionID)
	}

	artifacts, err := p.procReader.GetProcessingArtifacts(payload.ProcessingRevisionID)
	if err != nil {
		return fmt.Errorf("获取ProcessingArtifacts失败: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	journalID := fmt.Sprintf("bridge-%d", time.Now().UnixNano())
	journal := &editing.RevisionBridgeJournal{
		ID:                   journalID,
		ProcessingRevisionID: payload.ProcessingRevisionID,
		ProcessingActionID:   payload.ProcessingActionID,
		ActionRevisionID:     "",
		TargetActionKey:      payload.ActionKey,
		Status:               baseline.BridgeStatusReceived,
		EventID:              entry.EventID,
		UserID:               payload.UserID,
		CharacterID:          payload.CharacterID,
		ActionKey:            payload.ActionKey,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := p.journalRepo.Create(journal); err != nil {
		return fmt.Errorf("创建Journal失败: %w", err)
	}

	_ = p.journalRepo.UpdateStatus(journalID, baseline.BridgeStatusCommitting, "")

	commitReq := baseline.CommitterRequest{
		UserID:               payload.UserID,
		CharacterID:          payload.CharacterID,
		ProcessingTaskID:     payload.ProcessingTaskID,
		ProcessingActionID:   payload.ProcessingActionID,
		ProcessingAttemptID:  payload.ProcessingAttemptID,
		ProcessingRevisionID: payload.ProcessingRevisionID,
		ActionKey:            payload.ActionKey,
		ActionConfigJSON:     payload.ActionConfigJSON,
		ActionConfigHash:     payload.ActionConfigHash,
		ActionSpecVersion:    payload.ActionSpecVersion,
		ActionSpecHash:       payload.ActionSpecHash,
		PlaybackMode:         payload.PlaybackMode,
		FPS:                  payload.FPS,
		FrameDurationMS:      payload.FrameDurationMS,
		LoopType:             payload.LoopType,
		AnchorJSON:           payload.AnchorJSON,
		PromotionPolicy:      payload.PromotionPolicy,
		CreatedBy:            payload.CreatedBy,
		CorrelationID:        entry.EventID,
	}

	result, err := p.committer.Commit(commitReq, procRev, artifacts)
	if err != nil {
		_ = p.journalRepo.UpdateStatus(journalID, baseline.BridgeStatusFailedRetryable, err.Error())
		return fmt.Errorf("Committer提交失败: %w", err)
	}

	_ = p.journalRepo.UpdateActionRevisionID(journalID, result.ActionRevisionID)
	_ = p.journalRepo.UpdateStatus(journalID, baseline.BridgeStatusCompleted, "")

	log.Logger.Infof("Bridge处理完成: journalId=%s revisionId=%s streamId=%s bindingRev=%d",
		journalID, result.ActionRevisionID, result.ActionStreamID, result.BindingRevision)

	return nil
}

func (p *BridgeProcessor) PublishOutbox(ctx context.Context, maxCount int) error {
	records, err := p.outboxRepo.ListPending(maxCount)
	if err != nil {
		return fmt.Errorf("查询待发布Outbox事件失败: %w", err)
	}

	for i := range records {
		if err := p.publishOne(ctx, &records[i]); err != nil {
			log.Logger.Errorf("发布Outbox事件失败: id=%s err=%v", records[i].ID, err)
		}
	}
	return nil
}

func (p *BridgeProcessor) publishOne(ctx context.Context, record *editing.ActionRevisionEventOutboxRecord) error {
	acquired, err := p.outboxRepo.AcquireLease(record.ID, p.workerID, LeaseDuration)
	if err != nil {
		return fmt.Errorf("获取Outbox租约失败: %w", err)
	}
	if !acquired {
		return nil
	}

	if p.eventPub == nil {
		return p.outboxRepo.MarkPublished(record.ID)
	}

	if err := p.eventPub.Publish(ctx, record.EventType, []byte(record.PayloadJSON)); err != nil {
		_ = p.outboxRepo.IncrementAttemptCount(record.ID)
		if record.AttemptCount+1 >= MaxOutboxRetryAttempts {
			_ = p.outboxRepo.MarkFailed(record.ID, err.Error())
		} else {
			_ = p.outboxRepo.MarkFailed(record.ID, err.Error())
		}
		return err
	}

	return p.outboxRepo.MarkPublished(record.ID)
}
