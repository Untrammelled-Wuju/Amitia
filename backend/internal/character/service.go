// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package character

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/character/card"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/internal/sync"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type CardPreviewResult struct {
	Preview    *card.CharacterCardPreview `json:"preview"`
	SourceHash string                     `json:"sourceHash"`
	Format     card.CharacterCardFormat   `json:"format"`
}

type CardImportResult struct {
	CharacterID  string `json:"characterId"`
	Name         string `json:"name"`
	SourceFormat string `json:"sourceFormat"`
}

type CardExportResult struct {
	ResourceURI string `json:"resourceUri"`
	Format      string `json:"format"`
	Filename    string `json:"filename"`
	SizeBytes   int64  `json:"sizeBytes"`
	ContentHash string `json:"contentHash"`
}

type Service interface {
	List(includeDisabled bool) ([]Character, error)
	GetByID(id string) (*Character, error)
	Create(req *CreateCharacterRequest) (*Character, error)
	Update(id string, req *UpdateCharacterRequest) (*Character, error)
	Delete(id string) error
	SetActive(id string) (*Character, error)
	ListTemplates() ([]CharacterTemplate, error)
	GetTemplateByID(id string) (*CharacterTemplate, error)
	GetRoleProfile(characterID string) (*RoleProfileResponse, error)
	UpdateRoleProfile(characterID string, updates map[string]interface{}) (*RoleProfileResponse, error)
	UpdateAvatar(id string, avatarUrl string) error
	PreviewCard(data []byte, filename string) (*CardPreviewResult, error)
	ImportCard(data []byte, filename string, confirm bool) (*CardImportResult, error)
	ExportCard(characterID string, format string) (*CardExportResult, []byte, error)
	GetCardData(characterID string) (*card.CharacterCardData, error)
	UpdateCardData(characterID string, cardData *card.CharacterCardData) error
}

type service struct {
	repo           Repository
	db             *gorm.DB
	changeRecorder sync.ChangeRecorder
}

func NewService(repo Repository, ctx *app.AppContext, recorder ...sync.ChangeRecorder) Service {
	var r sync.ChangeRecorder
	if len(recorder) > 0 {
		r = recorder[0]
	}
	return &service{repo: repo, db: ctx.DB, changeRecorder: r}
}

func (s *service) List(includeDisabled bool) ([]Character, error) {
	return s.repo.List(includeDisabled)
}

func (s *service) GetByID(id string) (*Character, error) {
	c, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("角色不存在")
	}
	return c, nil
}

func (s *service) Create(req *CreateCharacterRequest) (*Character, error) {
	return s.CreateForUser(req, requestidentity.DefaultUserID)
}

func (s *service) CreateForUser(req *CreateCharacterRequest, userID string) (*Character, error) {
	c := &Character{
		ID: uuid.New().String(), Name: req.Name, Identity: req.Identity,
		Personality: req.Personality, SpeakingStyle: req.SpeakingStyle,
		Avatar:            req.Avatar,
		RelationshipStyle: req.RelationshipStyle, CharacterBase: req.CharacterBase,
		BoundaryRules: req.BoundaryRules, Description: req.Description,
		Gender: req.Gender, Pronoun: req.Pronoun, SelfReference: req.SelfReference,
		GenderExpression: req.GenderExpression, LifeIdentity: req.LifeIdentity,
		Status:            "enabled",
		PersonalityConfig: string(req.PersonalityConfig),
		ChatStyleConfig:   req.ChatStyleConfig,
		SceneRules:        req.SceneRules,
		VoiceType:         req.VoiceType, VoiceSpeed: req.VoiceSpeed, VoicePitch: req.VoicePitch,
		VoiceVolume: req.VoiceVolume, CustomVoiceID: req.CustomVoiceID,
	}
	if c.Name == "" {
		c.Name = "新角色"
	}
	if c.Gender == "" {
		c.Gender = "UNSPECIFIED"
	}
	if c.Pronoun == "" {
		c.Pronoun = "TA"
	}
	if c.SelfReference == "" {
		c.SelfReference = "我"
	}
	if c.LifeIdentity == "" {
		c.LifeIdentity = "CUSTOM"
	}
	if c.PersonalityConfig == "" || c.PersonalityConfig == "null" {
		c.PersonalityConfig = "{}"
	}
	if c.ChatStyleConfig == "" {
		c.ChatStyleConfig = "{}"
	}
	if c.SceneRules == "" {
		c.SceneRules = "{}"
	}
	if c.VoiceType == "" {
		c.VoiceType = "zh_female_vv_uranus_bigtts"
	}
	if c.VoiceSpeed == 0 {
		c.VoiceSpeed = 1.0
	}
	if c.VoicePitch == 0 && req.VoicePitch == 0 {
		c.VoicePitch = 0
	}
	if c.VoiceVolume == 0 {
		c.VoiceVolume = 1.0
	}
	c.Revision = 1
	if req.IsDefault {
		c.IsDefault = 1
	}
	var count int64
	if err := s.db.Table("characters").Count(&count).Error; err == nil && count == 0 {
		c.IsDefault = 1
	}

	var createErr error
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if c.IsDefault == 1 {
			var previousDefaults []Character
			if err := tx.Where("is_default = 1 AND deleted_at IS NULL").Find(&previousDefaults).Error; err != nil {
				return err
			}
			for i := range previousDefaults {
				previous := &previousDefaults[i]
				newRevision := previous.Revision + 1
				result := tx.Model(&Character{}).Where("id = ? AND revision = ?", previous.ID, previous.Revision).Updates(map[string]interface{}{"is_default": 0, "revision": newRevision, "updated_at": time.Now().Format("2006-01-02 15:04:05")})
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected == 0 {
					return fmt.Errorf("默认角色版本冲突")
				}
				previous.IsDefault, previous.Revision = 0, newRevision
				if err := s.recordCharacterChangeTx(tx, previous, sync.OpUpdate, userID); err != nil {
					return err
				}
			}
		}
		if err := tx.Create(c).Error; err != nil {
			return err
		}
		convID := uuid.New().String()
		now := time.Now().Format("2006-01-02 15:04:05")
		if err := tx.Exec("INSERT INTO conversations (id, character_id, title, channel, source, created_at, updated_at, revision) VALUES (?, ?, ?, 'web', 'system', ?, ?, 1)", convID, c.ID, c.Name, now, now).Error; err != nil {
			return err
		}
		if err := tx.Table("characters").Where("id = ?", c.ID).Update("conversation_id", convID).Error; err != nil {
			return err
		}
		if s.changeRecorder != nil {
			payload, marshalErr := json.Marshal(map[string]interface{}{"id": c.ID, "name": c.Name, "meta": characterSyncMeta(c)})
			if marshalErr != nil {
				return marshalErr
			}
			if _, recErr := s.changeRecorder.RecordChange(tx, sync.EntityTypeCharacter, sync.EntityID(c.ID), sync.OpCreate, 1, sync.MutationID("business_character_"+c.ID+"_create_"+uuid.NewString()), normalizeCharacterChangeUserID(userID), sync.ScopeDevice, payload); recErr != nil {
				createErr = recErr
				return recErr
			}
			conversationPayload, marshalErr := json.Marshal(map[string]string{"id": convID, "characterId": c.ID, "title": c.Name, "channel": "web", "source": "system"})
			if marshalErr != nil {
				return marshalErr
			}
			if _, recErr := s.changeRecorder.RecordChange(tx, sync.EntityTypeConversation, sync.EntityID(convID), sync.OpCreate, 1, sync.MutationID("business_conversation_"+convID+"_create_"+uuid.NewString()), normalizeCharacterChangeUserID(userID), sync.ScopeDevice, conversationPayload); recErr != nil {
				createErr = recErr
				return recErr
			}
		}
		return nil
	})
	if err != nil {
		if createErr != nil {
			return nil, fmt.Errorf("创建角色失败: %w", createErr)
		}
		return nil, fmt.Errorf("创建角色失败: %w", err)
	}

	presetRules := []struct {
		Name, Channel, RuleType, ScheduleCron, PromptTemplate string
		MaxPerDay, RandomMinutes                              int
	}{
		{"早安问候", "all", "cron", "0 8 * * *", "早上好！新的一天开始了，有什么计划吗？", 20, 30},
		{"晚安提醒", "all", "cron", "0 22 * * *", "夜深了，早点休息哦。今天过得怎么样？", 20, 30},
		{"学习打卡", "all", "cron", "0 19 * * *", "今天的学习任务完成了吗？需要我帮你复习一下吗？", 20, 30},
		{"工作间歇", "all", "cron", "0 15 * * 1-5", "工作累了就起来活动一下，喝杯水休息一会吧。", 20, 30},
		{"午饭时间", "all", "cron", "0 12 * * *", "到午饭时间啦，别忘了按时吃饭哦！", 20, 15},
		{"晚间闲聊", "all", "cron", "0 20 * * *", "晚上好！放松一下，想聊点什么吗？", 20, 45},
	}
	for _, p := range presetRules {
		s.db.Exec("INSERT INTO proactive_rules (name, enabled, channel, character_id, rule_type, schedule_cron, max_per_day, prompt_template, random_minutes, created_at, updated_at) VALUES (?, 0, ?, ?, ?, ?, ?, ?, ?, datetime('now', 'localtime'), datetime('now', 'localtime'))",
			p.Name, p.Channel, c.ID, p.RuleType, p.ScheduleCron, p.MaxPerDay, p.PromptTemplate, p.RandomMinutes)
	}
	return c, nil
}

func (s *service) Update(id string, req *UpdateCharacterRequest) (*Character, error) {
	return s.UpdateForUser(id, req, requestidentity.DefaultUserID)
}

func (s *service) UpdateForUser(id string, req *UpdateCharacterRequest, userID string) (*Character, error) {
	_, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("角色不存在")
	}
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Identity != nil {
		updates["identity"] = *req.Identity
	}
	if req.Personality != nil {
		updates["personality"] = *req.Personality
	}
	if req.SpeakingStyle != nil {
		updates["speaking_style"] = *req.SpeakingStyle
	}
	if req.RelationshipStyle != nil {
		updates["relationship_style"] = *req.RelationshipStyle
	}
	if req.CharacterBase != nil {
		updates["character_base"] = *req.CharacterBase
	}
	if req.BoundaryRules != nil {
		updates["boundary_rules"] = *req.BoundaryRules
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if req.Gender != nil {
		updates["gender"] = *req.Gender
	}
	if req.Pronoun != nil {
		updates["pronoun"] = *req.Pronoun
	}
	if req.SelfReference != nil {
		updates["self_reference"] = *req.SelfReference
	}
	if req.GenderExpression != nil {
		updates["gender_expression"] = *req.GenderExpression
	}
	if req.LifeIdentity != nil {
		updates["life_identity"] = *req.LifeIdentity
	}
	if req.VoiceConfigID != nil {
		updates["voice_config_id"] = *req.VoiceConfigID
	}
	if req.VoiceType != nil {
		updates["voice_type"] = *req.VoiceType
	}
	if req.VoiceSpeed != nil {
		updates["voice_speed"] = *req.VoiceSpeed
	}
	if req.VoicePitch != nil {
		updates["voice_pitch"] = *req.VoicePitch
	}
	if req.VoiceVolume != nil {
		updates["voice_volume"] = *req.VoiceVolume
	}
	if req.CustomVoiceID != nil {
		updates["custom_voice_id"] = *req.CustomVoiceID
	}
	if req.VoiceMode != nil {
		updates["voice_mode"] = *req.VoiceMode
	}
	if req.Emotion != nil {
		updates["emotion"] = *req.Emotion
	}
	if req.EmotionScale != nil {
		updates["emotion_scale"] = *req.EmotionScale
	}
	if req.SilenceDuration != nil {
		updates["silence_duration"] = *req.SilenceDuration
	}
	if req.PersonalityConfig != nil {
		updates["personality_config"] = string(*req.PersonalityConfig)
	}
	setExclusiveDefault := req.IsDefault != nil && *req.IsDefault
	if req.IsDefault != nil {
		if *req.IsDefault {
			updates["is_default"] = 1
		} else {
			updates["is_default"] = 0
		}
	}
	if req.ChatStyleConfig != nil {
		updates["chat_style_config"] = *req.ChatStyleConfig
	}
	if req.SceneRules != nil {
		updates["scene_rules"] = *req.SceneRules
	}
	if req.Avatar != nil {
		updates["avatar"] = *req.Avatar
	}
	if len(updates) == 0 {
		return nil, fmt.Errorf("没有可更新的字段")
	}
	var updateErr error
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var current Character
		if err := tx.Where("id = ?", id).First(&current).Error; err != nil {
			return err
		}
		if setExclusiveDefault {
			var previousDefaults []Character
			if err := tx.Where("is_default = 1 AND id <> ? AND deleted_at IS NULL", id).Find(&previousDefaults).Error; err != nil {
				return err
			}
			for i := range previousDefaults {
				previous := &previousDefaults[i]
				newPreviousRevision := previous.Revision + 1
				result := tx.Model(&Character{}).Where("id = ? AND revision = ?", previous.ID, previous.Revision).Updates(map[string]interface{}{"is_default": 0, "revision": newPreviousRevision, "updated_at": time.Now().Format("2006-01-02 15:04:05")})
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected == 0 {
					return fmt.Errorf("默认角色版本冲突")
				}
				previous.IsDefault, previous.Revision = 0, newPreviousRevision
				if err := s.recordCharacterChangeTx(tx, previous, sync.OpUpdate, userID); err != nil {
					return err
				}
			}
		}
		newRevision := current.Revision + 1
		updates["revision"] = newRevision
		result := tx.Model(&Character{}).Where("id = ? AND revision = ? AND deleted_at IS NULL", id, current.Revision).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("角色版本冲突")
		}
		if s.changeRecorder != nil {
			payload, marshalErr := json.Marshal(map[string]interface{}{"id": id, "revision": newRevision, "meta": updates})
			if marshalErr != nil {
				return marshalErr
			}
			if _, recErr := s.changeRecorder.RecordChange(tx, sync.EntityTypeCharacter, sync.EntityID(id), sync.OpUpdate, newRevision, sync.MutationID("business_character_"+id+"_update_"+uuid.NewString()), normalizeCharacterChangeUserID(userID), sync.ScopeDevice, payload); recErr != nil {
				updateErr = recErr
				return recErr
			}
		}
		return nil
	})
	if err != nil {
		if updateErr != nil {
			return nil, fmt.Errorf("更新失败: %w", updateErr)
		}
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	var result Character
	if err := s.db.Where("id = ?", id).First(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *service) Delete(id string) error {
	return s.DeleteForUser(id, requestidentity.DefaultUserID)
}

func (s *service) DeleteForUser(id string, userID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var current Character
		if err := tx.Where("id = ?", id).First(&current).Error; err != nil {
			return err
		}
		result := tx.Model(&Character{}).Where("id = ? AND revision = ? AND deleted_at IS NULL", id, current.Revision).Updates(map[string]interface{}{
			"deleted_at": time.Now().Format("2006-01-02 15:04:05"),
			"updated_at": time.Now().Format("2006-01-02 15:04:05"),
			"revision":   current.Revision + 1,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("角色版本冲突")
		}
		if s.changeRecorder != nil {
			payload, marshalErr := json.Marshal(map[string]string{"id": id})
			if marshalErr != nil {
				return marshalErr
			}
			if _, recErr := s.changeRecorder.RecordChange(tx, sync.EntityTypeCharacter, sync.EntityID(id), sync.OpDelete, current.Revision+1, sync.MutationID("business_character_"+id+"_delete_"+uuid.NewString()), normalizeCharacterChangeUserID(userID), sync.ScopeDevice, payload); recErr != nil {
				return recErr
			}
		}
		return nil
	})
}

func (s *service) updateCharacterFieldsForUser(id string, updates map[string]interface{}, userID string) (*Character, error) {
	var updated Character
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var current Character
		if err := tx.Where("id = ? AND deleted_at IS NULL", id).First(&current).Error; err != nil {
			return err
		}
		newRevision := current.Revision + 1
		copyUpdates := make(map[string]interface{}, len(updates)+2)
		for k, v := range updates {
			copyUpdates[k] = v
		}
		copyUpdates["revision"] = newRevision
		copyUpdates["updated_at"] = time.Now().Format("2006-01-02 15:04:05")
		result := tx.Model(&Character{}).Where("id = ? AND revision = ?", id, current.Revision).Updates(copyUpdates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("角色版本冲突")
		}
		if err := tx.Where("id = ?", id).First(&updated).Error; err != nil {
			return err
		}
		return s.recordCharacterChangeTx(tx, &updated, sync.OpUpdate, userID)
	})
	return &updated, err
}

func (s *service) recordCharacterChangeTx(tx *gorm.DB, c *Character, op sync.OperationType, userID string) error {
	if s.changeRecorder == nil || c == nil {
		return nil
	}
	payload, err := json.Marshal(map[string]interface{}{"id": c.ID, "name": c.Name, "revision": c.Revision, "meta": characterSyncMeta(c)})
	if err != nil {
		return err
	}
	_, err = s.changeRecorder.RecordChange(tx, sync.EntityTypeCharacter, sync.EntityID(c.ID), op, c.Revision, sync.MutationID("business_character_"+c.ID+"_"+string(op)+"_"+uuid.NewString()), normalizeCharacterChangeUserID(userID), sync.ScopeDevice, payload)
	return err
}

func normalizeCharacterChangeUserID(userID string) string {
	return requestidentity.NormalizeUserID(userID)
}

func characterSyncMeta(c *Character) map[string]interface{} {
	if c == nil {
		return nil
	}
	return map[string]interface{}{
		"name": c.Name, "identity": c.Identity, "personality": c.Personality,
		"speaking_style": c.SpeakingStyle, "relationship_style": c.RelationshipStyle,
		"character_base": c.CharacterBase, "boundary_rules": c.BoundaryRules,
		"description": c.Description, "status": c.Status, "is_active": c.IsActive,
		"sort_order": c.SortOrder, "gender": c.Gender, "pronoun": c.Pronoun,
		"self_reference": c.SelfReference, "gender_expression": c.GenderExpression,
		"life_identity": c.LifeIdentity, "voice_config_id": c.VoiceConfigID,
		"voice_type": c.VoiceType, "voice_speed": c.VoiceSpeed, "voice_pitch": c.VoicePitch,
		"voice_volume": c.VoiceVolume, "custom_voice_id": c.CustomVoiceID, "voice_mode": c.VoiceMode,
		"emotion": c.Emotion, "emotion_scale": c.EmotionScale, "silence_duration": c.SilenceDuration,
		"personality_config": c.PersonalityConfig, "is_default": c.IsDefault,
		"chat_style_config": c.ChatStyleConfig, "scene_rules": c.SceneRules,
		"avatar": c.Avatar, "conversation_id": c.ConversationID,
		"gender_label": c.GenderLabel, "user_addressing_style": c.UserAddressingStyle,
		"base_prompt": c.BasePrompt, "generated_prompt": c.GeneratedPrompt,
		"personality_sliders": c.PersonalitySliders, "card_data_json": c.CardDataJSON,
	}
}

func (s *service) SetActive(id string) (*Character, error) {
	return s.SetActiveForUser(id, requestidentity.DefaultUserID)
}

func (s *service) SetActiveForUser(id string, userID string) (*Character, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var chars []Character
		if err := tx.Where("deleted_at IS NULL AND (is_active = 1 OR id = ?)", id).Find(&chars).Error; err != nil {
			return err
		}
		found := false
		for i := range chars {
			c := &chars[i]
			if c.ID == id {
				found = true
			}
			desired := 0
			if c.ID == id {
				desired = 1
			}
			if c.IsActive == desired {
				continue
			}
			newRevision := c.Revision + 1
			result := tx.Model(&Character{}).Where("id = ? AND revision = ? AND deleted_at IS NULL", c.ID, c.Revision).Updates(map[string]interface{}{"is_active": desired, "revision": newRevision, "updated_at": time.Now().Format("2006-01-02 15:04:05")})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return fmt.Errorf("角色版本冲突")
			}
			c.IsActive, c.Revision = desired, newRevision
			if err := s.recordCharacterChangeTx(tx, c, sync.OpUpdate, userID); err != nil {
				return err
			}
		}
		if !found {
			return fmt.Errorf("角色不存在")
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("设置活跃失败: %w", err)
	}
	return s.repo.FindByID(id)
}

func (s *service) ListTemplates() ([]CharacterTemplate, error) { return s.repo.ListTemplates() }

func (s *service) GetTemplateByID(id string) (*CharacterTemplate, error) {
	t, err := s.repo.FindTemplateByID(id)
	if err != nil {
		return nil, fmt.Errorf("模板不存在")
	}
	return t, nil
}

func (s *service) GetRoleProfile(characterID string) (*RoleProfileResponse, error) {
	var c *Character
	var err error
	if characterID != "" {
		c, err = s.repo.FindByID(characterID)
	} else {
		c, err = s.repo.GetActive()
	}
	if err != nil {
		return nil, fmt.Errorf("没有可用角色")
	}
	return &RoleProfileResponse{ID: c.ID, CharacterID: c.ID, RoleName: c.Name, Gender: c.Gender, GenderLabel: c.GenderLabel, Pronoun: c.Pronoun, SelfReference: c.SelfReference, GenderExpression: c.GenderExpression, LifeIdentity: c.LifeIdentity, UserAddressingStyle: c.UserAddressingStyle}, nil
}

func (s *service) UpdateRoleProfile(characterID string, updates map[string]interface{}) (*RoleProfileResponse, error) {
	return s.UpdateRoleProfileForUser(characterID, updates, requestidentity.DefaultUserID)
}

func (s *service) UpdateRoleProfileForUser(characterID string, updates map[string]interface{}, userID string) (*RoleProfileResponse, error) {
	var targetID string
	if characterID != "" {
		targetID = characterID
	} else {
		active, err := s.repo.GetActive()
		if err != nil {
			return nil, fmt.Errorf("没有可用角色")
		}
		targetID = active.ID
	}

	fieldMap := map[string]string{
		"roleName":            "name",
		"gender":              "gender",
		"genderLabel":         "gender_label",
		"pronoun":             "pronoun",
		"selfReference":       "self_reference",
		"genderExpression":    "gender_expression",
		"lifeIdentity":        "life_identity",
		"userAddressingStyle": "user_addressing_style",
	}
	dbUpdates := make(map[string]interface{})
	for k, v := range updates {
		if col, ok := fieldMap[k]; ok {
			dbUpdates[col] = v
		}
	}
	if len(dbUpdates) == 0 {
		return s.GetRoleProfile(targetID)
	}
	if _, err := s.updateCharacterFieldsForUser(targetID, dbUpdates, userID); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	return s.GetRoleProfile(targetID)
}
func (s *service) UpdateAvatar(id string, avatarUrl string) error {
	return s.UpdateAvatarForUser(id, avatarUrl, requestidentity.DefaultUserID)
}

func (s *service) UpdateAvatarForUser(id string, avatarURL string, userID string) error {
	_, err := s.updateCharacterFieldsForUser(id, map[string]interface{}{"avatar": avatarURL}, userID)
	return err
}

func (s *service) PreviewCard(data []byte, filename string) (*CardPreviewResult, error) {
	parser := card.NewCardParser()
	c, _, err := parser.Parse(data, filename)
	if err != nil {
		return nil, err
	}

	sourceHash := card.ComputeSourceHash(data)
	format, _ := card.DetectFormat(data, filename)

	return &CardPreviewResult{
		Preview:    c.BuildPreview(),
		SourceHash: sourceHash,
		Format:     format,
	}, nil
}

func (s *service) ImportCard(data []byte, filename string, confirm bool) (*CardImportResult, error) {
	return s.ImportCardForUser(data, filename, confirm, requestidentity.DefaultUserID)
}

func (s *service) ImportCardForUser(data []byte, filename string, confirm bool, userID string) (*CardImportResult, error) {
	parser := card.NewCardParser()
	c, _, err := parser.Parse(data, filename)
	if err != nil {
		return nil, err
	}

	if err := c.ValidateForImport(); err != nil {
		return nil, err
	}

	mapping := c.ToCharacterMapping()
	name := c.SanitizeName()

	char := &Character{
		ID:            uuid.New().String(),
		Name:          name,
		Description:   mapping.Description,
		Personality:   mapping.Personality,
		BasePrompt:    mapping.SystemPrompt,
		Status:        "enabled",
		CardDataJSON:  "",
		CharacterBase: mapping.Scenario,
	}

	cardData := c.ToCardData()
	cardDataBytes, err := json.Marshal(cardData)
	if err != nil {
		return nil, fmt.Errorf("序列化卡片数据失败: %w", err)
	}
	char.CardDataJSON = string(cardDataBytes)

	char.Revision = 1
	convID := uuid.New().String()
	now := time.Now().Format("2006-01-02 15:04:05")
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(char).Error; err != nil {
			return err
		}
		if err := tx.Exec("INSERT INTO conversations (id, character_id, title, channel, source, created_at, updated_at, revision) VALUES (?, ?, ?, 'web', 'system', ?, ?, 1)", convID, char.ID, char.Name, now, now).Error; err != nil {
			return err
		}
		if err := tx.Model(&Character{}).Where("id = ?", char.ID).Update("conversation_id", convID).Error; err != nil {
			return err
		}
		char.ConversationID = convID
		if err := s.recordCharacterChangeTx(tx, char, sync.OpCreate, userID); err != nil {
			return err
		}
		if s.changeRecorder != nil {
			payload, marshalErr := json.Marshal(map[string]string{"id": convID, "characterId": char.ID, "title": char.Name, "channel": "web", "source": "system"})
			if marshalErr != nil {
				return marshalErr
			}
			_, recErr := s.changeRecorder.RecordChange(tx, sync.EntityTypeConversation, sync.EntityID(convID), sync.OpCreate, 1, sync.MutationID("business_conversation_"+convID+"_create_"+uuid.NewString()), normalizeCharacterChangeUserID(userID), sync.ScopeDevice, payload)
			if recErr != nil {
				return recErr
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("创建角色失败: %w", err)
	}

	return &CardImportResult{
		CharacterID:  char.ID,
		Name:         char.Name,
		SourceFormat: string(c.SourceFormat),
	}, nil
}

func (s *service) ExportCard(characterID string, format string) (*CardExportResult, []byte, error) {
	char, err := s.repo.FindByID(characterID)
	if err != nil {
		return nil, nil, fmt.Errorf("角色不存在")
	}

	var cardData card.CharacterCardData
	if char.CardDataJSON != "" && char.CardDataJSON != "{}" {
		if err := json.Unmarshal([]byte(char.CardDataJSON), &cardData); err != nil {
			return nil, nil, fmt.Errorf("解析卡片数据失败: %w", err)
		}
	}

	exporter := card.NewExporter("data")
	input := card.ExportInput{
		Name:               char.Name,
		Description:        char.Description,
		Personality:        char.Personality,
		Scenario:           cardData.Scenario,
		FirstMessage:       cardData.FirstMessage,
		AlternateGreetings: cardData.AlternateGreetings,
		ExampleMessages:    cardData.ExampleMessages,
		SystemPrompt:       cardData.SystemPrompt,
		PostHistory:        cardData.PostHistoryInstructions,
		Creator:            cardData.Creator,
		CreatorNotes:       cardData.CreatorNotes,
		CharacterVersion:   cardData.CharacterVersion,
		Tags:               cardData.Tags,
		Nickname:           cardData.Nickname,
		GroupOnlyGreetings: cardData.GroupOnlyGreetings,
		Source:             "Amitia",
		Extensions:         cardData.ExternalExtensions,
		AvatarURL:          char.Avatar,
		SourceFormat:       cardData.SourceFormat,
	}

	result, data, err := exporter.Export(input, format)
	if err != nil {
		return nil, nil, err
	}

	return &CardExportResult{
		ResourceURI: result.ResourceURI,
		Format:      result.Format,
		Filename:    result.Filename,
		SizeBytes:   result.SizeBytes,
		ContentHash: result.ContentHash,
	}, data, nil
}

func (s *service) GetCardData(characterID string) (*card.CharacterCardData, error) {
	char, err := s.repo.FindByID(characterID)
	if err != nil {
		return nil, fmt.Errorf("角色不存在")
	}

	if char.CardDataJSON == "" || char.CardDataJSON == "{}" {
		return &card.CharacterCardData{}, nil
	}

	var cardData card.CharacterCardData
	if err := json.Unmarshal([]byte(char.CardDataJSON), &cardData); err != nil {
		return nil, fmt.Errorf("解析卡片数据失败: %w", err)
	}
	return &cardData, nil
}

func (s *service) UpdateCardData(characterID string, cardData *card.CharacterCardData) error {
	_, err := s.repo.FindByID(characterID)
	if err != nil {
		return fmt.Errorf("角色不存在")
	}

	data, err := json.Marshal(cardData)
	if err != nil {
		return fmt.Errorf("序列化卡片数据失败: %w", err)
	}

	return s.repo.Update(characterID, map[string]interface{}{"card_data_json": string(data)})
}
