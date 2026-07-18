package emote

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/delivery"
	"gorm.io/gorm"
)

type EventPublisher func(message *chat.Message, channel string)

type Service struct {
	repo          *Repository
	semantic      *SemanticService
	deliveryStore *delivery.SQLiteDeliveryStore
	publish       EventPublisher
}

func NewService(db *gorm.DB, deliveryStore *delivery.SQLiteDeliveryStore) *Service {
	repo := NewRepository(db)
	return &Service{repo: repo, semantic: NewSemanticService(repo), deliveryStore: deliveryStore}
}

func (s *Service) SetPublisher(publisher EventPublisher) { s.publish = publisher }

func (s *Service) Repository() *Repository { return s.repo }

func (s *Service) List(groupID, view, query string, page, pageSize int) ([]Emote, int64, error) {
	return s.repo.List(groupID, view, query, page, pageSize)
}

func (s *Service) Get(id string) (*Emote, error) { return s.repo.Get(id) }

func (s *Service) Import(header *multipart.FileHeader, config ImportConfig) ImportResult {
	result := ImportResult{SourceName: header.Filename}
	id := uuid.New().String()
	asset, err := ProcessUpload(header, id)
	if err != nil {
		if fileErr, ok := err.(*FileError); ok {
			result.ErrorCode, result.ErrorMessage = fileErr.Code, fileErr.Message
		} else {
			result.ErrorCode, result.ErrorMessage = "invalid_image", err.Error()
		}
		result.Status = "failed"
		return result
	}
	if existing, duplicateErr := s.repo.GetByHash(asset.Hash); duplicateErr == nil {
		_ = removeAssetDirectory(&Emote{ID: id})
		if len(config.GroupIDs) > 0 {
			_ = s.repo.AddToGroup(config.GroupIDs[0], []string{existing.ID})
			for _, groupID := range config.GroupIDs[1:] {
				_ = s.repo.AddToGroup(groupID, []string{existing.ID})
			}
		}
		result.Status = "duplicate"
		result.EmoteID = existing.ID
		result.DuplicateEmoteID = existing.ID
		return result
	}
	name := strings.TrimSpace(config.Name)
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(header.Filename), filepath.Ext(header.Filename))
	}
	meaning := strings.TrimSpace(config.Meaning)
	aiEnabled := config.AIEnabled && meaning != ""
	roleScope := config.RoleScope
	if roleScope == "" {
		roleScope = RoleScopeAll
	}
	if err := s.validateScope(roleScope, config.CharacterIDs); err != nil {
		_ = removeAssetDirectory(&Emote{ID: id})
		result.Status = "failed"
		result.ErrorCode = "invalid_role_scope"
		result.ErrorMessage = err.Error()
		return result
	}
	keywords, _ := json.Marshal(normalizeKeywords(config.Keywords))
	now := time.Now().Format("2006-01-02 15:04:05")
	item := &Emote{ID: id, Name: name, Meaning: meaning, Keywords: string(keywords), OriginalFilename: filepath.Base(header.Filename), FilePath: asset.FilePath, ThumbnailPath: asset.ThumbnailPath, FallbackPath: asset.FallbackPath, MimeType: asset.Mime, FileExtension: asset.Ext, FileSize: asset.Size, Width: asset.Width, Height: asset.Height, IsAnimated: boolInt(asset.Animated), DurationMS: asset.DurationMS, FrameCount: asset.FrameCount, FileHash: asset.Hash, Enabled: 1, AIEnabled: boolInt(aiEnabled), RoleScope: roleScope, VectorStatus: "disabled", CreatedAt: now, UpdatedAt: now}
	if config.FolderGroup != "" && len(config.GroupIDs) == 0 {
		groupID, groupErr := s.ensureGroup(config.FolderGroup)
		if groupErr == nil {
			config.GroupIDs = []string{groupID}
		}
	}
	if err := s.repo.Create(item, config.GroupIDs, config.CharacterIDs); err != nil {
		_ = removeAssetDirectory(item)
		result.Status = "failed"
		result.ErrorCode = "storage_failed"
		result.ErrorMessage = err.Error()
		return result
	}
	if aiEnabled {
		if err := s.semantic.Sync(item); err != nil {
			item.VectorStatus = "failed"
			item.VectorError = err.Error()
			_ = s.repo.DB().Model(item).Updates(map[string]interface{}{"vector_status": "failed", "vector_error": err.Error()}).Error
		} else {
			item.VectorStatus = "ready"
			_ = s.repo.DB().Model(item).Updates(map[string]interface{}{"vector_status": "ready", "vector_error": ""}).Error
		}
	}
	result.Status = "success"
	result.EmoteID = id
	result.AIWasDisabled = config.AIEnabled && !aiEnabled
	return result
}

func (s *Service) Update(id string, req UpdateRequest) (*Emote, error) {
	item, err := s.repo.Get(id)
	if err != nil {
		return nil, err
	}
	wasAvailableForAI := item.AIEnabled == 1 && item.Enabled == 1
	semanticChanged := false
	if req.Name != nil {
		item.Name = strings.TrimSpace(*req.Name)
		semanticChanged = true
	}
	if req.Meaning != nil {
		item.Meaning = strings.TrimSpace(*req.Meaning)
		semanticChanged = true
	}
	if req.Keywords != nil {
		raw, _ := json.Marshal(normalizeKeywords(req.Keywords))
		item.Keywords = string(raw)
		semanticChanged = true
	}
	if req.Enabled != nil {
		item.Enabled = boolInt(*req.Enabled)
	}
	if req.AIEnabled != nil {
		item.AIEnabled = boolInt(*req.AIEnabled)
	}
	if item.AIEnabled == 1 && strings.TrimSpace(item.Meaning) == "" {
		item.AIEnabled = 0
	}
	replaceCharacters := req.RoleScope != nil || req.CharacterIDs != nil
	if req.RoleScope != nil {
		item.RoleScope = *req.RoleScope
	}
	if err := s.validateScope(item.RoleScope, req.CharacterIDs); err != nil {
		return nil, err
	}
	item.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	if err := s.repo.Update(item, req.GroupIDs, req.CharacterIDs, req.GroupIDs != nil, replaceCharacters); err != nil {
		return nil, err
	}
	if item.AIEnabled != 1 || item.Enabled != 1 {
		_ = s.semantic.Delete(item.ID)
		item.VectorStatus = "disabled"
	} else if semanticChanged || !wasAvailableForAI {
		if err := s.semantic.Sync(item); err != nil {
			item.VectorStatus = "failed"
			item.VectorError = err.Error()
		} else {
			item.VectorStatus = "ready"
			item.VectorError = ""
		}
	}
	_ = s.repo.DB().Model(item).Updates(map[string]interface{}{"vector_status": item.VectorStatus, "vector_error": item.VectorError}).Error
	return s.repo.Get(id)
}

func (s *Service) Delete(id string) error {
	item, err := s.repo.Get(id)
	if err != nil {
		return err
	}
	if err = s.repo.SoftDelete(id); err != nil {
		return err
	}
	_ = s.semantic.Delete(id)
	go func(deleted Emote) {
		timer := time.NewTimer(10 * time.Minute)
		defer timer.Stop()
		<-timer.C
		_ = removeAssetDirectory(&deleted)
	}(*item)
	return nil
}

func (s *Service) Groups() ([]Group, error) { return s.repo.ListGroups() }

func (s *Service) CreateGroup(name string) (*Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("分组名称不能为空")
	}
	var maxOrder int
	s.repo.DB().Table("emote_groups").Select("COALESCE(MAX(sort_order), -1)").Scan(&maxOrder)
	now := time.Now().Format("2006-01-02 15:04:05")
	group := &Group{ID: uuid.New().String(), Name: name, SortOrder: maxOrder + 1, CreatedAt: now, UpdatedAt: now}
	return group, s.repo.CreateGroup(group)
}

func (s *Service) UpdateGroup(id string, name *string, cover *string) error {
	updates := map[string]interface{}{"updated_at": time.Now().Format("2006-01-02 15:04:05")}
	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if trimmed == "" {
			return fmt.Errorf("分组名称不能为空")
		}
		updates["name"] = trimmed
	}
	if cover != nil {
		if *cover == "" {
			updates["cover_emote_id"] = nil
		} else {
			updates["cover_emote_id"] = *cover
		}
	}
	return s.repo.UpdateGroup(id, updates)
}

func (s *Service) DeleteGroup(id string) error { return s.repo.DeleteGroup(id) }

func (s *Service) ReorderGroups(ids []string) error { return s.repo.ReorderGroups(ids) }

func (s *Service) AddToGroup(groupID string, ids []string) error {
	return s.repo.AddToGroup(groupID, ids)
}

func (s *Service) RemoveFromGroup(groupID, id string) error {
	return s.repo.RemoveFromGroup(groupID, id)
}

func (s *Service) GetSettings(characterID string) (CharacterSettings, error) {
	if !s.repo.CharacterExists(characterID) {
		return CharacterSettings{}, gorm.ErrRecordNotFound
	}
	return s.repo.GetSettings(characterID)
}

func (s *Service) SaveSettings(characterID string, settings CharacterSettings) (CharacterSettings, error) {
	if !s.repo.CharacterExists(characterID) {
		return settings, gorm.ErrRecordNotFound
	}
	if settings.BaseProbability < 0 || settings.BaseProbability > 0.30 || settings.MaxProbability < 0 || settings.MaxProbability > 0.50 || settings.BaseProbability > settings.MaxProbability || settings.MaxPerHour < 0 || settings.MaxPerHour > 20 || settings.MinReplyGap < 0 || settings.MinReplyGap > 20 || settings.SameEmoteCooldownMinutes < 0 || settings.SameEmoteCooldownMinutes > 1440 {
		return settings, fmt.Errorf("角色表情设置超出允许范围")
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	existing, _ := s.repo.GetSettings(characterID)
	settings.CharacterID = characterID
	settings.CreatedAt = existing.CreatedAt
	if settings.CreatedAt == "" {
		settings.CreatedAt = now
	}
	settings.UpdatedAt = now
	return settings, s.repo.SaveSettings(&settings)
}

func (s *Service) validateScope(scope string, characterIDs []string) error {
	if scope != RoleScopeAll && scope != RoleScopeSelected {
		return fmt.Errorf("角色范围无效")
	}
	if scope == RoleScopeSelected && len(uniqueStrings(characterIDs)) == 0 {
		return fmt.Errorf("指定角色范围至少需要一个角色")
	}
	for _, id := range uniqueStrings(characterIDs) {
		if !s.repo.CharacterExists(id) {
			return fmt.Errorf("角色不存在: %s", id)
		}
	}
	return nil
}

func (s *Service) ensureGroup(name string) (string, error) {
	groups, err := s.repo.ListGroups()
	if err != nil {
		return "", err
	}
	for _, group := range groups {
		if strings.EqualFold(group.Name, strings.TrimSpace(name)) {
			return group.ID, nil
		}
	}
	group, err := s.CreateGroup(name)
	if err != nil {
		return "", err
	}
	return group.ID, nil
}

func normalizeKeywords(values []string) []string {
	out := []string{}
	for _, value := range values {
		for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '，' || r == ';' || r == '；' || r == '\n' }) {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return uniqueStrings(out)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (s *Service) ManualSend(conversationID, characterID, emoteID string, replyTo *string) (*chat.Message, error) {
	item, err := s.repo.Get(emoteID)
	if err != nil {
		return nil, err
	}
	if item.Enabled != 1 || item.DeletedAt != nil {
		return nil, errors.New("emote_not_found")
	}
	var conversation chat.Conversation
	if err = s.repo.DB().Where("id = ?", conversationID).First(&conversation).Error; err != nil {
		return nil, err
	}
	if conversation.CharacterID != characterID {
		return nil, errors.New("conversation_character_mismatch")
	}
	if conversation.Channel != "web" && conversation.Channel != "wechat" && conversation.Channel != "qq" {
		return nil, errors.New("platform_unsupported")
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	messageID := uuid.New().String()
	responseID := "manual-" + messageID
	alt := "[表情：" + item.Name + "]"
	message := &chat.Message{ID: messageID, ConversationID: conversationID, Role: "user", Content: alt, MsgType: "emote", Source: "manual", Status: "sending", ImageUrl: item.FilePath, EmoteID: item.ID, AltText: alt, IsAnimated: item.IsAnimated, MediaWidth: item.Width, MediaHeight: item.Height, OriginalAsset: item.FilePath, FallbackAsset: item.FallbackPath, ResponseGroupID: responseID, DeliverySequence: 1, ReplyToMessageID: replyTo, CreatedAt: now, UpdatedAt: now}
	deliveryKey := delivery.GenerateDeliveryID(responseID, conversation.Channel, conversation.PeerID, item.ID)
	err = s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(message).Error; err != nil {
			return err
		}
		record := SendRecord{ID: uuid.New().String(), EmoteID: &item.ID, CharacterID: characterID, ConversationID: conversationID, MessageID: messageID, ResponseID: responseID, Platform: conversation.Channel, TriggerType: TriggerManual, TriggerHit: 1, DecisionReason: "manual", SendMode: SendModeManual, DeliveryKey: deliveryKey, Status: "queued", CreatedAt: now}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		intent := newEmoteIntent(responseID, conversation.Channel, conversation.PeerID, deliveryKey, item, messageID, conversationID, characterID)
		return createIntentTx(tx, intent)
	})
	if err != nil {
		return nil, err
	}
	if s.publish != nil {
		s.publish(message, conversation.Channel)
	}
	return message, nil
}

func newEmoteIntent(responseID, channel, peerID, key string, item *Emote, messageID, conversationID, characterID string) delivery.DeliveryIntent {
	payload, _ := json.Marshal(map[string]interface{}{"messageId": messageID, "conversationId": conversationID, "characterId": characterID, "emoteId": item.ID, "altText": "[表情：" + item.Name + "]", "originalPath": item.FilePath, "fallbackPath": item.FallbackPath, "mimeType": item.MimeType, "isAnimated": item.IsAnimated == 1})
	intent := delivery.NewDeliveryIntent(responseID, channel, peerID, "emote", payload)
	intent.ID = key
	intent.MaxRetries = 3
	return intent
}

func createIntentTx(tx *gorm.DB, intent delivery.DeliveryIntent) error {
	result := tx.Exec("INSERT OR IGNORE INTO delivery_intents (id, interaction_id, channel, peer_id, content_type, payload, status, created_at, max_retries) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)", intent.ID, intent.InteractionID, intent.Channel, intent.PeerID, intent.ContentType, string(intent.Payload), string(intent.Status), intent.CreatedAt.Format("2006-01-02 15:04:05"), intent.MaxRetries)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("delivery_duplicate")
	}
	return nil
}
