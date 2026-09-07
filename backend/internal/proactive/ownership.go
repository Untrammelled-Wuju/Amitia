// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package proactive

import (
	"fmt"
	"strings"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/requestidentity"
	"gorm.io/gorm"
)

func proactiveLocalSingleUserMode() bool {
	return config.AppCfg != nil && strings.EqualFold(strings.TrimSpace(config.AppCfg.Security.Mode), "local_single_user")
}

func normalizeProactiveOwner(userID string) string { return requestidentity.NormalizeUserID(userID) }

func proactiveOwnerQuery(db *gorm.DB, userID string) *gorm.DB {
	owner := normalizeProactiveOwner(userID)
	if proactiveLocalSingleUserMode() {
		return db.Where("(user_id = ? OR user_id = '' OR user_id IS NULL OR user_id = ?)", owner, requestidentity.DefaultUserID)
	}
	return db.Where("user_id = ?", owner)
}

func (s *service) validateProactiveScope(userID, characterID, conversationID string) error {
	owner := normalizeProactiveOwner(userID)
	if strings.TrimSpace(characterID) != "" {
		var count int64
		q := proactiveOwnerQuery(s.db.Table("characters").Where("id = ? AND deleted_at IS NULL", strings.TrimSpace(characterID)), owner)
		if err := q.Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
	}
	if strings.TrimSpace(conversationID) != "" {
		var count int64
		q := proactiveOwnerQuery(s.db.Table("conversations").Where("id = ? AND deleted_at IS NULL", strings.TrimSpace(conversationID)), owner)
		if err := q.Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
	}
	return nil
}

func (s *service) ListRulesForUser(characterID, userID string) ([]map[string]interface{}, error) {
	presetNames := map[string]bool{"早安问候": true, "晚安提醒": true, "工作间歇": true, "午饭时间": true, "晚间闲聊": true, "早安心情": true, "午间日常": true, "傍晚时光": true, "睡前分享": true}
	var rules []ProactiveRule
	q := proactiveOwnerQuery(s.db.Model(&ProactiveRule{}), userID)
	if strings.TrimSpace(characterID) != "" {
		q = q.Where("character_id = ?", strings.TrimSpace(characterID))
	}
	if err := q.Order("id").Find(&rules).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]interface{}, len(rules))
	for i, r := range rules {
		result[i] = map[string]interface{}{"id": r.ID, "name": r.Name, "enabled": r.Enabled, "channel": r.Channel, "conversationId": r.ConversationID, "characterId": r.CharacterID, "ruleType": r.RuleType, "scheduleCron": r.ScheduleCron, "quietStart": r.QuietStart, "quietEnd": r.QuietEnd, "maxPerDay": r.MaxPerDay, "lastSentAt": r.LastSentAt, "sentCountToday": r.SentCountToday, "promptTemplate": r.PromptTemplate, "randomMinutes": r.RandomMinutes, "createdAt": r.CreatedAt, "updatedAt": r.UpdatedAt, "_isSystem": presetNames[r.Name]}
	}
	return result, nil
}

func (s *service) FindRuleForUser(id int, userID string) (*ProactiveRule, error) {
	var rule ProactiveRule
	if err := proactiveOwnerQuery(s.db.Model(&ProactiveRule{}).Where("id = ?", id), userID).First(&rule).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

func (s *service) CreateRuleForUser(req *CreateRuleRequest, userID string) (*ProactiveRule, error) {
	if err := s.validateProactiveScope(userID, req.CharacterID, req.ConversationID); err != nil {
		return nil, fmt.Errorf("规则作用域无效")
	}
	if req.Channel == "" {
		req.Channel = "web"
	}
	if req.RuleType == "" {
		req.RuleType = "cron"
	}
	if req.MaxPerDay == 0 {
		req.MaxPerDay = 1
	}
	if req.RandomMinutes == 0 {
		req.RandomMinutes = 30
	}
	enabled := 1
	if req.Enabled != nil && !*req.Enabled {
		enabled = 0
	}
	rule := &ProactiveRule{UserID: normalizeProactiveOwner(userID), Name: req.Name, Enabled: enabled, Channel: req.Channel, ConversationID: req.ConversationID, CharacterID: req.CharacterID, RuleType: req.RuleType, ScheduleCron: req.ScheduleCron, QuietStart: req.QuietStart, QuietEnd: req.QuietEnd, MaxPerDay: req.MaxPerDay, PromptTemplate: req.PromptTemplate, RandomMinutes: req.RandomMinutes}
	if err := s.db.Create(rule).Error; err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}
	return rule, nil
}

func (s *service) UpdateRuleForUser(id int, updates map[string]interface{}, userID string) (*ProactiveRule, error) {
	if len(updates) == 0 {
		return nil, fmt.Errorf("没有可更新的字段")
	}
	if rule, err := s.FindRuleForUser(id, userID); err != nil {
		return nil, err
	} else {
		characterID := rule.CharacterID
		conversationID := rule.ConversationID
		if v, ok := updates["characterId"].(string); ok {
			characterID = v
			delete(updates, "characterId")
			updates["character_id"] = v
		}
		if v, ok := updates["conversationId"].(string); ok {
			conversationID = v
			delete(updates, "conversationId")
			updates["conversation_id"] = v
		}
		if err := s.validateProactiveScope(userID, characterID, conversationID); err != nil {
			return nil, fmt.Errorf("规则作用域无效")
		}
	}
	delete(updates, "user_id")
	delete(updates, "userId")
	delete(updates, "id")
	result := proactiveOwnerQuery(s.db.Model(&ProactiveRule{}).Where("id = ?", id), userID).Updates(updates)
	if result.Error != nil {
		return nil, fmt.Errorf("更新失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return s.FindRuleForUser(id, userID)
}

func (s *service) DeleteRuleForUser(id int, userID string) error {
	result := proactiveOwnerQuery(s.db.Where("id = ?", id), userID).Delete(&ProactiveRule{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *service) ToggleRuleForUser(id int, userID string) (*ProactiveRule, error) {
	result := proactiveOwnerQuery(s.db.Model(&ProactiveRule{}).Where("id = ?", id), userID).Updates(map[string]interface{}{"enabled": gorm.Expr("CASE WHEN enabled = 1 THEN 0 ELSE 1 END"), "updated_at": gorm.Expr("datetime('now', 'localtime')")})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return s.FindRuleForUser(id, userID)
}

func (s *service) DeleteRulesByCharacterForUser(characterID, userID string) error {
	return proactiveOwnerQuery(s.db.Where("character_id = ?", characterID), userID).Delete(&ProactiveRule{}).Error
}

func (s *service) CreateRuleDirectForUser(rule *ProactiveRule, userID string) error {
	if rule == nil {
		return fmt.Errorf("规则不能为空")
	}
	rule.UserID = normalizeProactiveOwner(userID)
	if err := s.validateProactiveScope(userID, rule.CharacterID, rule.ConversationID); err != nil {
		return err
	}
	return s.db.Create(rule).Error
}

func (s *service) ListRemindersForUser(userID string) ([]Reminder, error) {
	var items []Reminder
	if err := proactiveOwnerQuery(s.db.Model(&Reminder{}), userID).Order("remind_at ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ConversationID != "" {
			var title, charID string
			proactiveOwnerQuery(s.db.Table("conversations").Where("id = ?", items[i].ConversationID), userID).Select("title, character_id").Row().Scan(&title, &charID)
			items[i].ConversationTitle = title
			if items[i].CharacterID == "" {
				items[i].CharacterID = charID
			}
		}
		if items[i].CharacterID != "" {
			proactiveOwnerQuery(s.db.Table("characters").Where("id = ?", items[i].CharacterID), userID).Select("name").Row().Scan(&items[i].CharacterName)
		}
	}
	if items == nil {
		items = []Reminder{}
	}
	return items, nil
}

func (s *service) FindReminderForUser(id int, userID string) (*Reminder, error) {
	var rem Reminder
	if err := proactiveOwnerQuery(s.db.Model(&Reminder{}).Where("id = ?", id), userID).First(&rem).Error; err != nil {
		return nil, err
	}
	return &rem, nil
}

func (s *service) CreateReminderForUser(req *CreateReminderRequest, userID string) (*Reminder, error) {
	if err := s.validateProactiveScope(userID, req.CharacterID, req.ConversationID); err != nil {
		return nil, fmt.Errorf("提醒作用域无效")
	}
	if req.Channel == "" {
		req.Channel = "web"
	}
	if req.RepeatRule == "" {
		req.RepeatRule = "none"
	}
	rem := &Reminder{UserID: normalizeProactiveOwner(userID), Title: req.Title, Content: req.Content, Channel: req.Channel, ConversationID: req.ConversationID, CharacterID: req.CharacterID, RemindAt: req.RemindAt, RepeatRule: req.RepeatRule}
	if err := s.db.Create(rem).Error; err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}
	return rem, nil
}

func (s *service) UpdateReminderForUser(id int, updates map[string]interface{}, userID string) (*Reminder, error) {
	if len(updates) == 0 {
		return nil, fmt.Errorf("没有可更新的字段")
	}
	rem, err := s.FindReminderForUser(id, userID)
	if err != nil {
		return nil, err
	}
	characterID, conversationID := rem.CharacterID, rem.ConversationID
	if v, ok := updates["characterId"].(string); ok {
		characterID = v
		delete(updates, "characterId")
		updates["character_id"] = v
	}
	if v, ok := updates["conversationId"].(string); ok {
		conversationID = v
		delete(updates, "conversationId")
		updates["conversation_id"] = v
	}
	if err := s.validateProactiveScope(userID, characterID, conversationID); err != nil {
		return nil, fmt.Errorf("提醒作用域无效")
	}
	delete(updates, "user_id")
	delete(updates, "userId")
	delete(updates, "id")
	result := proactiveOwnerQuery(s.db.Model(&Reminder{}).Where("id = ?", id), userID).Updates(updates)
	if result.Error != nil {
		return nil, fmt.Errorf("更新失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return s.FindReminderForUser(id, userID)
}

func (s *service) DeleteReminderForUser(id int, userID string) error {
	result := proactiveOwnerQuery(s.db.Where("id = ?", id), userID).Delete(&Reminder{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *service) ToggleReminderForUser(id int, userID string) (*Reminder, error) {
	result := proactiveOwnerQuery(s.db.Model(&Reminder{}).Where("id = ?", id), userID).Updates(map[string]interface{}{"enabled": gorm.Expr("CASE WHEN enabled = 1 THEN 0 ELSE 1 END"), "updated_at": gorm.Expr("datetime('now', 'localtime')")})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return s.FindReminderForUser(id, userID)
}

func (s *service) PendingRemindersForUser(userID string) ([]Reminder, error) {
	var items []Reminder
	err := proactiveOwnerQuery(s.db.Model(&Reminder{}).Where("enabled = 1 AND remind_at <= datetime('now', 'localtime', '+5 minutes')"), userID).Order("remind_at ASC").Limit(20).Find(&items).Error
	if items == nil {
		items = []Reminder{}
	}
	return items, err
}

func (s *service) ListTriggerHistoryForUser(page, pageSize int, state, userID string) ([]TriggerHistory, int64, error) {
	var items []TriggerHistory
	var total int64
	q := proactiveOwnerQuery(s.db.Model(&TriggerHistory{}), userID)
	if state != "" {
		q = q.Where("state = ?", state)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error
	if items == nil {
		items = []TriggerHistory{}
	}
	return items, total, err
}
