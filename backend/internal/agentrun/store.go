package agentrun

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	StatusPlanning  = "planning"
	StatusRunning   = "running"
	StatusWaiting   = "waiting"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
	StatusPaused    = "paused"

	MessageInstruction = "instruction"
	MessageProgress    = "progress"
	MessageQuestion    = "question"
	MessageResult      = "result"
	MessageError       = "error"
	MessageUser        = "user"

	MessagePending = "pending"
	MessageRead    = "read"
)

type Run struct {
	ID                string    `gorm:"primaryKey;column:id" json:"id"`
	SpaceID           string    `gorm:"column:space_id;not null;index:idx_agent_run_conversation,priority:1" json:"spaceId"`
	ConversationID    string    `gorm:"column:conversation_id;not null;index:idx_agent_run_conversation,priority:2" json:"conversationId"`
	ParentRunID       string    `gorm:"column:parent_run_id;not null;default:'';index" json:"parentRunId"`
	CoordinationID    string    `gorm:"column:coordination_id;not null;default:'';index" json:"coordinationId"`
	AssignmentID      string    `gorm:"column:assignment_id;not null;default:'';index" json:"assignmentId"`
	ParentInteraction string    `gorm:"column:parent_interaction_id;not null;default:'';index" json:"parentInteractionId"`
	ChildInteraction  string    `gorm:"column:child_interaction_id;not null;default:'';index" json:"childInteractionId"`
	CharacterID       string    `gorm:"column:character_id;not null;default:'';index" json:"characterId"`
	WorkerRole        string    `gorm:"column:worker_role;not null;default:''" json:"workerRole"`
	Objective         string    `gorm:"column:objective;not null;default:''" json:"objective"`
	Status            string    `gorm:"column:status;not null;default:'planning';index" json:"status"`
	Depth             int       `gorm:"column:depth;not null;default:0" json:"depth"`
	CreatedAt         time.Time `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt         time.Time `gorm:"column:updated_at;not null;index" json:"updatedAt"`
}

type Message struct {
	ID            string     `gorm:"primaryKey;column:id" json:"id"`
	SpaceID       string     `gorm:"column:space_id;not null;index:idx_agent_run_message_run,priority:1" json:"spaceId"`
	Conversation  string     `gorm:"column:conversation_id;not null;index" json:"conversationId"`
	SenderRunID   string     `gorm:"column:sender_run_id;not null;index" json:"senderRunId"`
	RecipientID   string     `gorm:"column:recipient_run_id;not null;index:idx_agent_run_message_run,priority:2" json:"recipientRunId"`
	Kind          string     `gorm:"column:kind;not null;default:'instruction';index" json:"kind"`
	Content       string     `gorm:"column:content;not null" json:"content"`
	Status        string     `gorm:"column:status;not null;default:'pending';index" json:"status"`
	CorrelationID string     `gorm:"column:correlation_id;not null;default:'';index" json:"correlationId"`
	CreatedAt     time.Time  `gorm:"column:created_at;not null;index" json:"createdAt"`
	ReadAt        *time.Time `gorm:"column:read_at" json:"readAt"`
}

func (Run) TableName() string {
	return "agent_runs"
}

func (Message) TableName() string {
	return "agent_run_messages"
}

type Assignment struct {
	AssignmentID     string
	ChildInteraction string
	CharacterID      string
	WorkerRole       string
	Objective        string
	Status           string
}

type Coordination struct {
	SpaceID             string
	ConversationID      string
	CoordinationID      string
	ParentInteractionID string
	ParentCharacterID   string
	Status              string
	Depth               int
	Assignments         []Assignment
}

type SendInput struct {
	SpaceID       string
	SenderRunID   string
	RecipientID   string
	Kind          string
	Content       string
	CorrelationID string
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) (*Service, error) {
	if db == nil {
		return nil, errors.New("agentrun: database unavailable")
	}
	return &Service{db: db}, nil
}

func RootRunID(coordinationID string) string {
	return "agentrun-root-" + strings.TrimSpace(coordinationID)
}

func ChildRunID(assignmentID string) string {
	return "agentrun-child-" + strings.TrimSpace(assignmentID)
}

func UserRunID(runID string) string {
	return "user:" + strings.TrimSpace(runID)
}

func normalizeStatus(value string) string {
	switch strings.TrimSpace(value) {
	case StatusPlanning, StatusRunning, StatusWaiting, StatusSucceeded, StatusFailed, StatusCancelled, StatusPaused:
		return strings.TrimSpace(value)
	case "pending":
		return StatusPlanning
	case "aggregating":
		return StatusRunning
	default:
		return StatusRunning
	}
}

func normalizeKind(value string) string {
	switch strings.TrimSpace(value) {
	case MessageInstruction, MessageProgress, MessageQuestion, MessageResult, MessageError, MessageUser:
		return strings.TrimSpace(value)
	default:
		return MessageInstruction
	}
}

func (s *Service) SyncCoordination(ctx context.Context, item Coordination) error {
	if s == nil || s.db == nil {
		return errors.New("agentrun: service unavailable")
	}
	item.SpaceID = strings.TrimSpace(item.SpaceID)
	item.ConversationID = strings.TrimSpace(item.ConversationID)
	item.CoordinationID = strings.TrimSpace(item.CoordinationID)
	if item.SpaceID == "" || item.ConversationID == "" || item.CoordinationID == "" {
		return errors.New("agentrun: coordination scope is incomplete")
	}
	now := time.Now().UTC()
	rootID := RootRunID(item.CoordinationID)
	runs := []Run{{
		ID: rootID, SpaceID: item.SpaceID, ConversationID: item.ConversationID,
		CoordinationID: item.CoordinationID, ParentInteraction: strings.TrimSpace(item.ParentInteractionID),
		CharacterID: strings.TrimSpace(item.ParentCharacterID), WorkerRole: "main",
		Objective: "主对话智能体", Status: normalizeStatus(item.Status), Depth: item.Depth, CreatedAt: now, UpdatedAt: now,
	}}
	for _, assignment := range item.Assignments {
		assignmentID := strings.TrimSpace(assignment.AssignmentID)
		if assignmentID == "" {
			continue
		}
		runs = append(runs, Run{
			ID: ChildRunID(assignmentID), SpaceID: item.SpaceID, ConversationID: item.ConversationID,
			ParentRunID: rootID, CoordinationID: item.CoordinationID, AssignmentID: assignmentID,
			ChildInteraction: strings.TrimSpace(assignment.ChildInteraction), CharacterID: strings.TrimSpace(assignment.CharacterID),
			WorkerRole: strings.TrimSpace(assignment.WorkerRole), Objective: strings.TrimSpace(assignment.Objective),
			Status: normalizeStatus(assignment.Status), Depth: item.Depth + 1, CreatedAt: now, UpdatedAt: now,
		})
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"conversation_id", "parent_run_id", "coordination_id", "assignment_id",
			"parent_interaction_id", "child_interaction_id", "character_id", "worker_role",
			"objective", "status", "depth", "updated_at",
		}),
	}).Create(&runs).Error
}

func (s *Service) Get(ctx context.Context, spaceID, runID string) (Run, error) {
	var run Run
	if s == nil || s.db == nil {
		return run, errors.New("agentrun: service unavailable")
	}
	if err := s.db.WithContext(ctx).Where("id = ? AND space_id = ?", strings.TrimSpace(runID), strings.TrimSpace(spaceID)).First(&run).Error; err != nil {
		return run, fmt.Errorf("agentrun: read run: %w", err)
	}
	return run, nil
}

func (s *Service) List(ctx context.Context, spaceID, conversationID string) ([]Run, error) {
	runs := make([]Run, 0)
	if s == nil || s.db == nil {
		return runs, errors.New("agentrun: service unavailable")
	}
	if err := s.db.WithContext(ctx).
		Where("space_id = ? AND conversation_id = ?", strings.TrimSpace(spaceID), strings.TrimSpace(conversationID)).
		Order("depth ASC, created_at ASC, id ASC").
		Find(&runs).Error; err != nil {
		return nil, fmt.Errorf("agentrun: list runs: %w", err)
	}
	return runs, nil
}

func (s *Service) ListChildren(ctx context.Context, spaceID, parentRunID string) ([]Run, error) {
	runs := make([]Run, 0)
	if s == nil || s.db == nil {
		return runs, errors.New("agentrun: service unavailable")
	}
	if err := s.db.WithContext(ctx).
		Where("space_id = ? AND parent_run_id = ?", strings.TrimSpace(spaceID), strings.TrimSpace(parentRunID)).
		Order("created_at ASC, id ASC").
		Find(&runs).Error; err != nil {
		return nil, fmt.Errorf("agentrun: list child runs: %w", err)
	}
	return runs, nil
}

func (s *Service) Send(ctx context.Context, input SendInput) (Message, error) {
	message := Message{}
	if s == nil || s.db == nil {
		return message, errors.New("agentrun: service unavailable")
	}
	input.SpaceID = strings.TrimSpace(input.SpaceID)
	input.SenderRunID = strings.TrimSpace(input.SenderRunID)
	input.RecipientID = strings.TrimSpace(input.RecipientID)
	input.Content = strings.TrimSpace(input.Content)
	if input.SpaceID == "" || input.SenderRunID == "" || input.RecipientID == "" || input.Content == "" {
		return message, errors.New("agentrun: message scope or content is empty")
	}
	if len(input.Content) > 16000 {
		return message, errors.New("agentrun: message exceeds 16000 characters")
	}
	recipient, err := s.Get(ctx, input.SpaceID, input.RecipientID)
	if err != nil {
		return message, err
	}
	sender, err := s.Get(ctx, input.SpaceID, input.SenderRunID)
	if err != nil {
		return message, err
	}
	if recipient.ConversationID != sender.ConversationID {
		return message, errors.New("agentrun: message cannot cross conversations")
	}
	if sender.ParentRunID != recipient.ID && recipient.ParentRunID != sender.ID {
		return message, errors.New("agentrun: only parent and child runs may exchange messages")
	}
	now := time.Now().UTC()
	message = Message{
		ID: uuid.NewString(), SpaceID: input.SpaceID, Conversation: recipient.ConversationID,
		SenderRunID: sender.ID, RecipientID: recipient.ID, Kind: normalizeKind(input.Kind),
		Content: input.Content, Status: MessagePending, CorrelationID: strings.TrimSpace(input.CorrelationID), CreatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&message).Error; err != nil {
		return Message{}, fmt.Errorf("agentrun: create message: %w", err)
	}
	return message, nil
}

func (s *Service) SendFromUser(ctx context.Context, spaceID, recipientID, content string) (Message, error) {
	message := Message{}
	if s == nil || s.db == nil {
		return message, errors.New("agentrun: service unavailable")
	}
	recipient, err := s.Get(ctx, spaceID, recipientID)
	if err != nil {
		return message, err
	}
	content = strings.TrimSpace(content)
	if content == "" || len(content) > 16000 {
		return message, errors.New("agentrun: user message must contain 1 to 16000 characters")
	}
	now := time.Now().UTC()
	message = Message{
		ID: uuid.NewString(), SpaceID: recipient.SpaceID, Conversation: recipient.ConversationID,
		SenderRunID: UserRunID(recipient.ID), RecipientID: recipient.ID, Kind: MessageUser,
		Content: content, Status: MessagePending, CreatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&message).Error; err != nil {
		return Message{}, fmt.Errorf("agentrun: create user message: %w", err)
	}
	return message, nil
}

func (s *Service) Inbox(ctx context.Context, spaceID, runID string, markRead bool) ([]Message, error) {
	messages := make([]Message, 0)
	if s == nil || s.db == nil {
		return messages, errors.New("agentrun: service unavailable")
	}
	run, err := s.Get(ctx, spaceID, runID)
	if err != nil {
		return nil, err
	}
	query := s.db.WithContext(ctx).Where("space_id = ? AND recipient_run_id = ?", run.SpaceID, run.ID)
	if err := query.Order("created_at ASC, id ASC").Limit(200).Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("agentrun: read inbox: %w", err)
	}
	if markRead && len(messages) > 0 {
		ids := make([]string, 0, len(messages))
		for _, message := range messages {
			if message.Status != MessageRead {
				ids = append(ids, message.ID)
			}
		}
		if len(ids) > 0 {
			readAt := time.Now().UTC()
			if err := s.db.WithContext(ctx).Model(&Message{}).Where("space_id = ? AND id IN ?", run.SpaceID, ids).Updates(map[string]any{"status": MessageRead, "read_at": readAt}).Error; err != nil {
				return nil, fmt.Errorf("agentrun: mark message read: %w", err)
			}
			for index := range messages {
				if messages[index].Status != MessageRead {
					messages[index].Status = MessageRead
					messages[index].ReadAt = &readAt
				}
			}
		}
	}
	return messages, nil
}

func (s *Service) Messages(ctx context.Context, spaceID, runID string) ([]Message, error) {
	messages := make([]Message, 0)
	if s == nil || s.db == nil {
		return messages, errors.New("agentrun: service unavailable")
	}
	run, err := s.Get(ctx, spaceID, runID)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).
		Where("space_id = ? AND conversation_id = ? AND (sender_run_id = ? OR recipient_run_id = ?)", run.SpaceID, run.ConversationID, run.ID, run.ID).
		Order("created_at ASC, id ASC").
		Limit(500).
		Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("agentrun: list messages: %w", err)
	}
	return messages, nil
}
