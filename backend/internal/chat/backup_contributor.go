package chat

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
)

const (
	ComponentIDChatConversations = "chat.conversations"
	ComponentIDChatMessages      = "chat.messages"
)

type ChatBackupContributor struct {
	DB *gorm.DB
}

func NewChatBackupContributor(db *gorm.DB) *ChatBackupContributor {
	return &ChatBackupContributor{DB: db}
}

func (c *ChatBackupContributor) ID() string             { return "chat" }
func (c *ChatBackupContributor) Name() string           { return "Chat" }
func (c *ChatBackupContributor) Dependencies() []string { return []string{"character"} }

type chatConversationV2 struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Channel      string `json:"channel"`
	Source       string `json:"source"`
	PeerID       string `json:"peerId"`
	MessageCount int    `json:"messageCount"`
	StateVersion string `json:"stateVersion"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type chatMessageV2 struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversationId"`
	CharacterID    string `json:"characterId"`
	Sequence       int64  `json:"sequence"`
	Role           string `json:"role"`
	Content        string `json:"content"`
	MsgType        string `json:"msgType"`
	Status         string `json:"status"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

func (c *ChatBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	var convCount int64
	c.DB.WithContext(ctx).Model(&Conversation{}).Where("COALESCE(project_id, '') = ''").Count(&convCount)
	var msgCount int64
	c.DB.WithContext(ctx).Model(&Message{}).
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("COALESCE(conversations.project_id, '') = ''").
		Count(&msgCount)

	return []dataportability.BackupComponentPlan{
		{
			ID:            ComponentIDChatConversations,
			Kind:          dataportability.KindNDJSON,
			LogicalName:   "chat.conversations.v2",
			Required:      true,
			SourceOfTruth: true,
			ItemCount:     convCount,
			EstimatedSize: convCount * 512,
		},
		{
			ID:            ComponentIDChatMessages,
			Kind:          dataportability.KindNDJSON,
			LogicalName:   "chat.messages.v2",
			Required:      true,
			SourceOfTruth: true,
			ItemCount:     msgCount,
			EstimatedSize: msgCount * 1024,
		},
	}, nil
}

func (c *ChatBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	convComp, err := out.CreateComponent(ComponentIDChatConversations, "chat.conversations.v2", dataportability.KindNDJSON)
	if err != nil {
		return fmt.Errorf("export: create conversations component: %w", err)
	}
	defer convComp.Close()

	rows, err := c.DB.WithContext(ctx).Model(&Conversation{}).Where("COALESCE(project_id, '') = ''").Rows()
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var conv Conversation
		if err := c.DB.ScanRows(rows, &conv); err != nil {
			continue
		}
		rec := chatConversationV2{
			ID:           conv.ID,
			Title:        conv.Title,
			Channel:      conv.Channel,
			Source:       conv.Source,
			PeerID:       conv.PeerID,
			MessageCount: conv.MessageCount,
			StateVersion: conv.StateVersion,
			CreatedAt:    conv.CreatedAt,
			UpdatedAt:    conv.UpdatedAt,
		}
		data, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		convComp.Write(data)
		convComp.Write([]byte("\n"))
	}

	msgComp, err := out.CreateComponent(ComponentIDChatMessages, "chat.messages.v2", dataportability.KindNDJSON)
	if err != nil {
		return fmt.Errorf("export: create messages component: %w", err)
	}
	defer msgComp.Close()

	msgRows, err := c.DB.WithContext(ctx).Model(&Message{}).
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("COALESCE(conversations.project_id, '') = ''").
		Order("messages.conversation_id, messages.sequence").
		Rows()
	if err != nil {
		return err
	}
	defer msgRows.Close()
	for msgRows.Next() {
		var msg Message
		if err := c.DB.ScanRows(msgRows, &msg); err != nil {
			continue
		}
		rec := chatMessageV2{
			ID:             msg.ID,
			ConversationID: msg.ConversationID,
			CharacterID:    msg.CharacterID,
			Sequence:       msg.Sequence,
			Role:           msg.Role,
			Content:        msg.Content,
			MsgType:        msg.MsgType,
			Status:         msg.Status,
			CreatedAt:      msg.CreatedAt,
			UpdatedAt:      msg.UpdatedAt,
		}
		data, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		msgComp.Write(data)
		msgComp.Write([]byte("\n"))
	}
	return nil
}

func (c *ChatBackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	previews := make([]dataportability.ImportComponentPreview, 0, 2)

	convPreview := dataportability.ImportComponentPreview{
		ComponentID: ComponentIDChatConversations,
		Kind:        dataportability.KindNDJSON,
		LogicalName: "chat.conversations.v2",
		Collisions:  make([]dataportability.ComponentCollision, 0),
		Warnings:    make([]string, 0),
	}
	convRC, err := in.ReadComponent(ComponentIDChatConversations + ".v2")
	if err == nil {
		defer convRC.Close()
		scanner := bufio.NewScanner(convRC)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			convPreview.ItemCount++
			var rec chatConversationV2
			if err := json.Unmarshal(line, &rec); err != nil {
				continue
			}
			var existing struct{ ID string }
			c.DB.WithContext(ctx).Model(&Conversation{}).Select("id").Where("id = ?", rec.ID).Scan(&existing)
			if existing.ID != "" {
				convPreview.Collisions = append(convPreview.Collisions, dataportability.ComponentCollision{
					SourceID:   rec.ID,
					TargetID:   existing.ID,
					EntityType: "conversation",
					Policy:     dataportability.CollisionDuplicate,
				})
			}
		}
	}
	previews = append(previews, convPreview)

	msgPreview := dataportability.ImportComponentPreview{
		ComponentID: ComponentIDChatMessages,
		Kind:        dataportability.KindNDJSON,
		LogicalName: "chat.messages.v2",
	}
	msgRC, err := in.ReadComponent(ComponentIDChatMessages + ".v2")
	if err == nil {
		defer msgRC.Close()
		scanner := bufio.NewScanner(msgRC)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			msgPreview.ItemCount++
		}
	}
	previews = append(previews, msgPreview)

	return previews, nil
}

func (c *ChatBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	opts := dataportability.RestoreOptions{
		OperationID:        req.OperationID,
		Purpose:            dataportability.RestorePurposeOrdinary,
		CharacterPolicy:    req.CharacterPolicy,
		DefaultCharacterID: req.DefaultCharacterID,
		ActivateImported:   req.ActivateImported,
		IdentityMap:        req.IdentityMap,
		SecretProvider:     req.SecretProvider,
	}
	return c.RestoreChats(ctx, in, opts)
}

func (c *ChatBackupContributor) RestoreChats(ctx context.Context, in dataportability.BackupReader, opts dataportability.RestoreOptions) error {
	if opts.IdentityMap == nil {
		return dataportability.ErrImportIdentityMapMissing
	}
	idMap := opts.IdentityMap

	convRC, err := in.ReadComponent(ComponentIDChatConversations + ".v2")
	if err != nil {
		return fmt.Errorf("restore: conversations component missing: %w", err)
	}
	defer convRC.Close()

	conversations := make([]chatConversationV2, 0)
	scanner := bufio.NewScanner(convRC)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec chatConversationV2
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		conversations = append(conversations, rec)
	}

	for _, rec := range conversations {
		newID := rec.ID
		var existing struct{ ID string }
		c.DB.WithContext(ctx).Model(&Conversation{}).Select("id").Where("id = ?", rec.ID).Scan(&existing)
		if existing.ID != "" {
			switch opts.CharacterPolicy {
			case dataportability.CollisionSkip:
				idMap.AddConversation(rec.ID, existing.ID)
				continue
			case dataportability.CollisionReplace:
				newID = rec.ID
			default:
				newID = uuid.New().String()
			}
		}
		conv := Conversation{
			ID:           newID,
			Title:        rec.Title,
			Channel:      rec.Channel,
			Source:       rec.Source,
			PeerID:       rec.PeerID,
			MessageCount: rec.MessageCount,
			StateVersion: rec.StateVersion,
			CreatedAt:    rec.CreatedAt,
			UpdatedAt:    rec.UpdatedAt,
		}
		if err := c.DB.WithContext(ctx).Create(&conv).Error; err != nil {
			idMap.AddConversation(rec.ID, newID)
			continue
		}
		idMap.AddConversation(rec.ID, newID)
	}

	msgRC, err := in.ReadComponent(ComponentIDChatMessages + ".v2")
	if err != nil {
		return nil
	}
	defer msgRC.Close()

	msgScanner := bufio.NewScanner(msgRC)
	for msgScanner.Scan() {
		line := msgScanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec chatMessageV2
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		newConvID := idMap.RemapConversationRef(rec.ConversationID)
		newMsgID := uuid.New().String()
		msg := Message{
			ID:             newMsgID,
			ConversationID: newConvID,
			CharacterID:    idMap.RemapCharacterRef(rec.CharacterID),
			Sequence:       rec.Sequence,
			Role:           rec.Role,
			Content:        rec.Content,
			MsgType:        rec.MsgType,
			Status:         rec.Status,
			CreatedAt:      rec.CreatedAt,
			UpdatedAt:      rec.UpdatedAt,
		}
		c.DB.WithContext(ctx).Create(&msg)
		idMap.AddMessage(rec.ID, newMsgID)
	}

	return nil
}

var _ dataportability.ChatRestorePort = (*ChatBackupContributor)(nil)
