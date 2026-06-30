// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package proactive

import (
	"fmt"

	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Service interface {
	ListRules(characterID string) ([]map[string]interface{}, error)
	CreateRule(req *CreateRuleRequest) (*ProactiveRule, error)
	UpdateRule(id int, updates map[string]interface{}) (*ProactiveRule, error)
	DeleteRule(id int) error
	ToggleRule(id int) (*ProactiveRule, error)
	DeleteRulesByCharacter(characterID string) error
	CreateRuleDirect(rule *ProactiveRule) error
	ListReminders() ([]Reminder, error)
	CreateReminder(req *CreateReminderRequest) (*Reminder, error)
	UpdateReminder(id int, updates map[string]interface{}) (*Reminder, error)
	DeleteReminder(id int) error
	ToggleReminder(id int) (*Reminder, error)
	PendingReminders() ([]Reminder, error)
}

type service struct {
	repo Repository
	db   *gorm.DB
}

func NewService(repo Repository, ctx *app.AppContext) Service {
	return &service{repo: repo, db: ctx.DB}
}

func (s *service) ListRules(characterID string) ([]map[string]interface{}, error) {
	presetNames := map[string]bool{
		"早安问候": true, "晚安提醒": true,
		"工作间歇": true, "午饭时间": true, "晚间闲聊": true,
		"早安心情": true, "午间日常": true, "傍晚时光": true, "睡前分享": true,
	}
	var rules []ProactiveRule
	var err error
	if characterID != "" {
		rules, err = s.repo.ListRulesByCharacter(characterID)
	} else {
		rules, err = s.repo.ListRules()
	}
	if err != nil {
		return nil, err
	}
	result := make([]map[string]interface{}, len(rules))
	for i, r := range rules {
		result[i] = map[string]interface{}{
			"id": r.ID, "name": r.Name, "enabled": r.Enabled,
			"channel": r.Channel, "conversationId": r.ConversationID,
			"characterId": r.CharacterID, "ruleType": r.RuleType,
			"scheduleCron": r.ScheduleCron, "quietStart": r.QuietStart,
			"quietEnd": r.QuietEnd, "maxPerDay": r.MaxPerDay,
			"lastSentAt": r.LastSentAt, "sentCountToday": r.SentCountToday,
			"promptTemplate": r.PromptTemplate, "randomMinutes": r.RandomMinutes,
			"createdAt": r.CreatedAt, "updatedAt": r.UpdatedAt,
			"_isSystem": presetNames[r.Name],
		}
	}
	return result, nil
}

func (s *service) CreateRule(req *CreateRuleRequest) (*ProactiveRule, error) {
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
	rule := &ProactiveRule{
		Name: req.Name, Enabled: enabled, Channel: req.Channel,
		ConversationID: req.ConversationID, CharacterID: req.CharacterID,
		RuleType: req.RuleType, ScheduleCron: req.ScheduleCron,
		QuietStart: req.QuietStart, QuietEnd: req.QuietEnd,
		MaxPerDay: req.MaxPerDay, PromptTemplate: req.PromptTemplate,
		RandomMinutes: req.RandomMinutes,
	}
	if err := s.repo.CreateRule(rule); err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}
	return rule, nil
}

func (s *service) UpdateRule(id int, updates map[string]interface{}) (*ProactiveRule, error) {
	if len(updates) == 0 {
		return nil, fmt.Errorf("没有可更新的字段")
	}
	if err := s.repo.UpdateRule(id, updates); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	return s.repo.FindRuleByID(id)
}

func (s *service) DeleteRule(id int) error                   { return s.repo.DeleteRule(id) }
func (s *service) ToggleRule(id int) (*ProactiveRule, error) { return s.repo.ToggleRule(id) }
func (s *service) DeleteRulesByCharacter(characterID string) error {
	return s.db.Where("character_id = ?", characterID).Delete(&ProactiveRule{}).Error
}
func (s *service) CreateRuleDirect(rule *ProactiveRule) error { return s.repo.CreateRule(rule) }

func (s *service) ListReminders() ([]Reminder, error) {
	items, err := s.repo.ListReminders()
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ConversationID != "" {
			var title, charID string
			s.db.Raw("SELECT title, character_id FROM conversations WHERE id = ? LIMIT 1", items[i].ConversationID).Row().Scan(&title, &charID)
			items[i].ConversationTitle = title
			if items[i].CharacterID == "" && charID != "" {
				items[i].CharacterID = charID
			}
		}
		if items[i].CharacterID != "" {
			s.db.Raw("SELECT name FROM characters WHERE id = ? LIMIT 1", items[i].CharacterID).Row().Scan(&items[i].CharacterName)
		}
	}
	return items, nil
}

func (s *service) CreateReminder(req *CreateReminderRequest) (*Reminder, error) {
	if req.Channel == "" {
		req.Channel = "web"
	}
	if req.RepeatRule == "" {
		req.RepeatRule = "none"
	}
	rem := &Reminder{
		Title: req.Title, Content: req.Content, Channel: req.Channel,
		ConversationID: req.ConversationID, CharacterID: req.CharacterID,
		RemindAt: req.RemindAt, RepeatRule: req.RepeatRule,
	}
	if err := s.repo.CreateReminder(rem); err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}
	return rem, nil
}

func (s *service) UpdateReminder(id int, updates map[string]interface{}) (*Reminder, error) {
	if len(updates) == 0 {
		return nil, fmt.Errorf("没有可更新的字段")
	}
	if err := s.repo.UpdateReminder(id, updates); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	var rem Reminder
	s.db.First(&rem, id)
	return &rem, nil
}

func (s *service) DeleteReminder(id int) error              { return s.repo.DeleteReminder(id) }
func (s *service) ToggleReminder(id int) (*Reminder, error) { return s.repo.ToggleReminder(id) }
func (s *service) PendingReminders() ([]Reminder, error)    { return s.repo.PendingReminders() }
