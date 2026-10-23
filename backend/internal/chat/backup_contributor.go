package chat

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
func (c *ChatBackupContributor) ComponentIDs() []string {
	return []string{ComponentIDChatConversations, ComponentIDChatMessages}
}

type chatConversationV2 struct {
	ID                       string `json:"id"`
	SpaceID                  string `json:"spaceId"`
	ProjectID                string `json:"projectId"`
	WorkspaceID              string `json:"workspaceId"`
	WorkspaceDeviceID        string `json:"workspaceDeviceId"`
	Title                    string `json:"title"`
	Channel                  string `json:"channel"`
	Source                   string `json:"source"`
	PeerID                   string `json:"peerId"`
	ModelConfigID            int    `json:"modelConfigId"`
	ReasoningEffort          string `json:"reasoningEffort"`
	ReasoningEnabled         int    `json:"reasoningEnabled"`
	PermissionMode           string `json:"permissionMode"`
	MessageCount             int    `json:"messageCount"`
	StateVersion             string `json:"stateVersion"`
	PinnedAt                 string `json:"pinnedAt"`
	ArchivedAt               string `json:"archivedAt"`
	LastReadTurnSequence     int64  `json:"lastReadTurnSequence"`
	LastTerminalTurnSequence int64  `json:"lastTerminalTurnSequence"`
	CreatedAt                string `json:"createdAt"`
	UpdatedAt                string `json:"updatedAt"`
	Revision                 int64  `json:"revision"`
}

type chatMessageV2 struct {
	ID                   string  `json:"id"`
	ConversationID       string  `json:"conversationId"`
	CharacterID          string  `json:"characterId"`
	Sequence             int64   `json:"sequence"`
	Role                 string  `json:"role"`
	Content              string  `json:"content"`
	ReasoningContent     string  `json:"reasoningContent"`
	ReasoningDurationMS  int64   `json:"reasoningDurationMs"`
	MsgType              string  `json:"msgType"`
	ExtensionType        string  `json:"extensionType"`
	Tokens               int     `json:"tokens"`
	Source               string  `json:"source"`
	SafetyLevel          string  `json:"safetyLevel"`
	Status               string  `json:"status"`
	IncludeInCtx         int     `json:"includeInContext"`
	AudioURL             string  `json:"audioUrl"`
	AudioDuration        float64 `json:"audioDuration"`
	ImageURL             string  `json:"imageUrl"`
	VideoURL             string  `json:"videoUrl"`
	AltText              string  `json:"altText"`
	IsAnimated           int     `json:"isAnimated"`
	MediaWidth           int     `json:"width"`
	MediaHeight          int     `json:"height"`
	OriginalAsset        string  `json:"originalAssetReference"`
	FallbackAsset        string  `json:"fallbackAssetReference"`
	DeliveryGroupID      string  `json:"deliveryGroupId"`
	DeliverySequence     int     `json:"deliverySequence"`
	RequestID            string  `json:"requestId"`
	ReplyToMessageID     *string `json:"replyToMessageId"`
	ReplyToRole          *string `json:"replyToRole"`
	ReplyToExcerpt       *string `json:"replyToExcerpt"`
	CitationSnapshotJSON string  `json:"citationSnapshotJson"`
	CreatedAt            string  `json:"createdAt"`
	UpdatedAt            string  `json:"updatedAt"`
	Revision             int64   `json:"revision"`
}

func (c *ChatBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	if req.EffectiveScope() != dataportability.ScopeAll && req.EffectiveScope() != dataportability.ScopeCharacter {
		return nil, nil
	}
	var convCount int64
	convQuery := c.DB.WithContext(ctx).Model(&Conversation{}).Where("deleted_at IS NULL")
	if req.EffectiveScope() == dataportability.ScopeCharacter && req.CharacterID != "" {
		convQuery = convQuery.Where("EXISTS (SELECT 1 FROM messages m WHERE m.conversation_id = conversations.id AND m.character_id = ? AND m.deleted_at IS NULL)", req.CharacterID)
	}
	convQuery.Count(&convCount)
	var msgCount int64
	msgQuery := c.DB.WithContext(ctx).Model(&Message{}).
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("conversations.deleted_at IS NULL").
		Where("messages.deleted_at IS NULL")
	if req.EffectiveScope() == dataportability.ScopeCharacter && req.CharacterID != "" {
		msgQuery = msgQuery.Where("messages.character_id = ?", req.CharacterID)
	}
	msgQuery.Count(&msgCount)

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
	if req.EffectiveScope() != dataportability.ScopeAll && req.EffectiveScope() != dataportability.ScopeCharacter {
		return nil
	}
	convComp, err := out.CreateComponent(ComponentIDChatConversations, "chat.conversations.v2", dataportability.KindNDJSON)
	if err != nil {
		return fmt.Errorf("export: create conversations component: %w", err)
	}
	defer convComp.Close()

	convQuery := c.DB.WithContext(ctx).Model(&Conversation{}).Where("deleted_at IS NULL")
	if req.EffectiveScope() == dataportability.ScopeCharacter && req.CharacterID != "" {
		convQuery = convQuery.Where("EXISTS (SELECT 1 FROM messages m WHERE m.conversation_id = conversations.id AND m.character_id = ? AND m.deleted_at IS NULL)", req.CharacterID)
	}
	rows, err := convQuery.Rows()
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
			ID:                       conv.ID,
			SpaceID:                  conv.SpaceID,
			ProjectID:                conv.ProjectID,
			WorkspaceID:              conv.WorkspaceID,
			WorkspaceDeviceID:        conv.WorkspaceDeviceID,
			Title:                    conv.Title,
			Channel:                  conv.Channel,
			Source:                   conv.Source,
			PeerID:                   conv.PeerID,
			ModelConfigID:            conv.ModelConfigID,
			ReasoningEffort:          conv.ReasoningEffort,
			ReasoningEnabled:         conv.ReasoningEnabled,
			PermissionMode:           conv.PermissionMode,
			MessageCount:             conv.MessageCount,
			StateVersion:             conv.StateVersion,
			PinnedAt:                 conv.PinnedAt,
			ArchivedAt:               conv.ArchivedAt,
			LastReadTurnSequence:     conv.LastReadTurnSequence,
			LastTerminalTurnSequence: conv.LastTerminalTurnSequence,
			CreatedAt:                conv.CreatedAt,
			UpdatedAt:                conv.UpdatedAt,
			Revision:                 conv.Revision,
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

	msgQuery := c.DB.WithContext(ctx).Model(&Message{}).
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("conversations.deleted_at IS NULL").
		Where("messages.deleted_at IS NULL")
	if req.EffectiveScope() == dataportability.ScopeCharacter && req.CharacterID != "" {
		msgQuery = msgQuery.Where("messages.character_id = ?", req.CharacterID)
	}
	msgRows, err := msgQuery.Order("messages.conversation_id, messages.sequence").Rows()
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
			ID:                   msg.ID,
			ConversationID:       msg.ConversationID,
			CharacterID:          msg.CharacterID,
			Sequence:             msg.Sequence,
			Role:                 msg.Role,
			Content:              msg.Content,
			ReasoningContent:     msg.ReasoningContent,
			ReasoningDurationMS:  msg.ReasoningDurationMS,
			MsgType:              msg.MsgType,
			ExtensionType:        msg.ExtensionType,
			Tokens:               msg.Tokens,
			Source:               msg.Source,
			SafetyLevel:          msg.SafetyLevel,
			Status:               msg.Status,
			IncludeInCtx:         msg.IncludeInCtx,
			AudioURL:             msg.AudioUrl,
			AudioDuration:        msg.AudioDuration,
			ImageURL:             msg.ImageUrl,
			VideoURL:             msg.VideoUrl,
			AltText:              msg.AltText,
			IsAnimated:           msg.IsAnimated,
			MediaWidth:           msg.MediaWidth,
			MediaHeight:          msg.MediaHeight,
			OriginalAsset:        msg.OriginalAsset,
			FallbackAsset:        msg.FallbackAsset,
			DeliveryGroupID:      msg.DeliveryGroupID,
			DeliverySequence:     msg.DeliverySequence,
			RequestID:            msg.RequestID,
			ReplyToMessageID:     msg.ReplyToMessageID,
			ReplyToRole:          msg.ReplyToRole,
			ReplyToExcerpt:       msg.ReplyToExcerpt,
			CitationSnapshotJSON: msg.CitationSnapshotJSON,
			CreatedAt:            msg.CreatedAt,
			UpdatedAt:            msg.UpdatedAt,
			Revision:             msg.Revision,
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
	convRC, err := in.ReadComponent(ComponentIDChatConversations)
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
	msgRC, err := in.ReadComponent(ComponentIDChatMessages)
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

	convRC, err := in.ReadComponent(ComponentIDChatConversations)
	if err != nil {
		return fmt.Errorf("restore: conversations component missing: %w", err)
	}
	conversations := make([]chatConversationV2, 0)
	scanner := bufio.NewScanner(convRC)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec chatConversationV2
		if err := json.Unmarshal(line, &rec); err != nil {
			convRC.Close()
			return fmt.Errorf("%w: conversation record", dataportability.ErrImportPackageInvalid)
		}
		conversations = append(conversations, rec)
	}
	if err := scanner.Err(); err != nil {
		convRC.Close()
		return err
	}
	convRC.Close()

	msgRC, err := in.ReadComponent(ComponentIDChatMessages)
	if err != nil {
		return fmt.Errorf("restore: messages component missing: %w", err)
	}
	messages := make([]chatMessageV2, 0)
	msgScanner := bufio.NewScanner(msgRC)
	msgScanner.Buffer(make([]byte, 64<<10), 8<<20)
	for msgScanner.Scan() {
		line := msgScanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec chatMessageV2
		if err := json.Unmarshal(line, &rec); err != nil {
			msgRC.Close()
			return fmt.Errorf("%w: message record", dataportability.ErrImportPackageInvalid)
		}
		messages = append(messages, rec)
	}
	if err := msgScanner.Err(); err != nil {
		msgRC.Close()
		return err
	}
	msgRC.Close()

	return c.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, rec := range conversations {
			newID := rec.ID
			var existing struct{ ID string }
			if err := tx.Model(&Conversation{}).Select("id").Where("id = ?", rec.ID).Scan(&existing).Error; err != nil {
				return err
			}
			if existing.ID != "" {
				switch opts.CharacterPolicy {
				case dataportability.CollisionSkip:
					idMap.AddConversation(rec.ID, existing.ID)
					continue
				case dataportability.CollisionReplace:
					newID = rec.ID
				case dataportability.CollisionDuplicate:
					newID = uuid.New().String()
				default:
					return dataportability.ErrImportCollisionPolicyInvalid
				}
			}
			conv := Conversation{
				ID:                       newID,
				SpaceID:                  rec.SpaceID,
				ProjectID:                rec.ProjectID,
				WorkspaceID:              rec.WorkspaceID,
				WorkspaceDeviceID:        rec.WorkspaceDeviceID,
				Title:                    rec.Title,
				Channel:                  rec.Channel,
				Source:                   rec.Source,
				PeerID:                   rec.PeerID,
				ModelConfigID:            rec.ModelConfigID,
				ReasoningEffort:          rec.ReasoningEffort,
				ReasoningEnabled:         rec.ReasoningEnabled,
				PermissionMode:           rec.PermissionMode,
				MessageCount:             rec.MessageCount,
				StateVersion:             rec.StateVersion,
				PinnedAt:                 rec.PinnedAt,
				ArchivedAt:               rec.ArchivedAt,
				LastReadTurnSequence:     rec.LastReadTurnSequence,
				LastTerminalTurnSequence: rec.LastTerminalTurnSequence,
				CreatedAt:                rec.CreatedAt,
				UpdatedAt:                rec.UpdatedAt,
				Revision:                 rec.Revision,
			}
			query := tx
			if opts.CharacterPolicy == dataportability.CollisionReplace {
				query = query.Clauses(clause.OnConflict{UpdateAll: true})
			}
			if err := query.Create(&conv).Error; err != nil {
				return err
			}
			idMap.AddConversation(rec.ID, newID)
		}

		for _, rec := range messages {
			newConvID := idMap.RemapConversationRef(rec.ConversationID)
			if newConvID == "" {
				return dataportability.ErrImportMessageRefInvalid
			}
			newMsgID := rec.ID
			var existing struct{ ID string }
			if err := tx.Model(&Message{}).Select("id").Where("id = ?", rec.ID).Scan(&existing).Error; err != nil {
				return err
			}
			if existing.ID != "" {
				switch opts.CharacterPolicy {
				case dataportability.CollisionSkip:
					idMap.AddMessage(rec.ID, existing.ID)
					continue
				case dataportability.CollisionReplace:
					newMsgID = rec.ID
				case dataportability.CollisionDuplicate:
					newMsgID = uuid.New().String()
				default:
					return dataportability.ErrImportCollisionPolicyInvalid
				}
			}
			citationJSON := rec.CitationSnapshotJSON
			if citationJSON == "" {
				citationJSON = "[]"
			}
			msg := Message{
				ID:                   newMsgID,
				ConversationID:       newConvID,
				CharacterID:          idMap.RemapCharacterRef(rec.CharacterID),
				Sequence:             rec.Sequence,
				Role:                 rec.Role,
				Content:              rec.Content,
				ReasoningContent:     rec.ReasoningContent,
				ReasoningDurationMS:  rec.ReasoningDurationMS,
				MsgType:              rec.MsgType,
				ExtensionType:        rec.ExtensionType,
				Tokens:               rec.Tokens,
				Source:               rec.Source,
				SafetyLevel:          rec.SafetyLevel,
				Status:               rec.Status,
				IncludeInCtx:         rec.IncludeInCtx,
				AudioUrl:             rec.AudioURL,
				AudioDuration:        rec.AudioDuration,
				ImageUrl:             rec.ImageURL,
				VideoUrl:             rec.VideoURL,
				AltText:              rec.AltText,
				IsAnimated:           rec.IsAnimated,
				MediaWidth:           rec.MediaWidth,
				MediaHeight:          rec.MediaHeight,
				OriginalAsset:        rec.OriginalAsset,
				FallbackAsset:        rec.FallbackAsset,
				DeliveryGroupID:      rec.DeliveryGroupID,
				DeliverySequence:     rec.DeliverySequence,
				RequestID:            rec.RequestID,
				ReplyToMessageID:     rec.ReplyToMessageID,
				ReplyToRole:          rec.ReplyToRole,
				ReplyToExcerpt:       rec.ReplyToExcerpt,
				CitationSnapshotJSON: citationJSON,
				CreatedAt:            rec.CreatedAt,
				UpdatedAt:            rec.UpdatedAt,
				Revision:             rec.Revision,
			}
			query := tx
			if opts.CharacterPolicy == dataportability.CollisionReplace {
				query = query.Clauses(clause.OnConflict{UpdateAll: true})
			}
			if err := query.Create(&msg).Error; err != nil {
				return err
			}
			idMap.AddMessage(rec.ID, newMsgID)
		}
		return nil
	})
}
