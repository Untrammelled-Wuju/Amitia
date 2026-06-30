// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package character

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

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
}

type service struct {
	repo Repository
	db   *gorm.DB
}

func NewService(repo Repository, ctx *app.AppContext) Service {
	return &service{repo: repo, db: ctx.DB}
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
	c := &Character{
		ID: uuid.New().String(), Name: req.Name, Identity: req.Identity,
		Personality: req.Personality, SpeakingStyle: req.SpeakingStyle,
		RelationshipStyle: req.RelationshipStyle, SystemPrompt: req.SystemPrompt,
		BoundaryRules: req.BoundaryRules, Description: req.Description,
		Gender: req.Gender, Pronoun: req.Pronoun, SelfReference: req.SelfReference,
		GenderExpression: req.GenderExpression, LifeIdentity: req.LifeIdentity,
		Status: "enabled", PersonalityConfig: "{}", ChatStyleConfig: "{}", SceneRules: "{}",
		VoiceType: req.VoiceType, VoiceSpeed: req.VoiceSpeed, VoicePitch: req.VoicePitch,
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
	if req.IsDefault {
		s.db.Table("characters").Where("is_default = 1").Update("is_default", 0)
		c.IsDefault = 1
	}
	if err := s.repo.Create(c); err != nil {
		return nil, fmt.Errorf("创建角色失败: %w", err)
	}
	// Auto-create preset proactive rules for the new character
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
	if req.SystemPrompt != nil {
		updates["system_prompt"] = *req.SystemPrompt
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
		updates["personality_config"] = *req.PersonalityConfig
	}
	if req.IsDefault != nil {
		if *req.IsDefault {
			s.db.Table("characters").Where("is_default = 1").Update("is_default", 0)
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
	if err := s.repo.Update(id, updates); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	return s.repo.FindByID(id)
}

func (s *service) Delete(id string) error { return s.repo.Delete(id) }

func (s *service) SetActive(id string) (*Character, error) {
	_, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("角色不存在")
	}
	if err := s.repo.SetActive(id); err != nil {
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
	if err := s.repo.Update(targetID, updates); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	return s.GetRoleProfile(targetID)
}
