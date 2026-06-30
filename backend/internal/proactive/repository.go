// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package proactive

import (
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Repository interface {
	ListRules() ([]ProactiveRule, error)
	ListRulesByCharacter(characterID string) ([]ProactiveRule, error)
	FindRuleByID(id int) (*ProactiveRule, error)
	CreateRule(r *ProactiveRule) error
	UpdateRule(id int, updates map[string]interface{}) error
	DeleteRule(id int) error
	ToggleRule(id int) (*ProactiveRule, error)
	ResetDailyCounts() error

	ListReminders() ([]Reminder, error)
	CreateReminder(r *Reminder) error
	UpdateReminder(id int, updates map[string]interface{}) error
	DeleteReminder(id int) error
	ToggleReminder(id int) (*Reminder, error)
	PendingReminders() ([]Reminder, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(ctx *app.AppContext) Repository {
	return &repository{db: ctx.DB}
}

func (r *repository) ListRules() ([]ProactiveRule, error) {
	var rules []ProactiveRule
	err := r.db.Order("id").Find(&rules).Error
	if rules == nil {
		rules = []ProactiveRule{}
	}
	return rules, err
}

func (r *repository) ListRulesByCharacter(characterID string) ([]ProactiveRule, error) {
	var rules []ProactiveRule
	err := r.db.Where("character_id = ?", characterID).Order("id").Find(&rules).Error
	if rules == nil {
		rules = []ProactiveRule{}
	}
	return rules, err
}

func (r *repository) FindRuleByID(id int) (*ProactiveRule, error) {
	var rule ProactiveRule
	err := r.db.First(&rule, id).Error
	return &rule, err
}

func (r *repository) CreateRule(rule *ProactiveRule) error {
	return r.db.Create(rule).Error
}

func (r *repository) UpdateRule(id int, updates map[string]interface{}) error {
	return r.db.Model(&ProactiveRule{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) DeleteRule(id int) error {
	return r.db.Delete(&ProactiveRule{}, id).Error
}

func (r *repository) ToggleRule(id int) (*ProactiveRule, error) {
	r.db.Exec("UPDATE proactive_rules SET enabled = CASE WHEN enabled = 1 THEN 0 ELSE 1 END, updated_at = datetime('now', 'localtime') WHERE id = ?", id)
	return r.FindRuleByID(id)
}

func (r *repository) ResetDailyCounts() error {
	return r.db.Model(&ProactiveRule{}).Where("sent_count_today > 0").Update("sent_count_today", 0).Error
}

func (r *repository) ListReminders() ([]Reminder, error) {
	var items []Reminder
	err := r.db.Order("remind_at ASC").Find(&items).Error
	if items == nil {
		items = []Reminder{}
	}
	return items, err
}

func (r *repository) CreateReminder(rem *Reminder) error {
	return r.db.Create(rem).Error
}

func (r *repository) UpdateReminder(id int, updates map[string]interface{}) error {
	return r.db.Model(&Reminder{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) DeleteReminder(id int) error {
	return r.db.Delete(&Reminder{}, id).Error
}

func (r *repository) ToggleReminder(id int) (*Reminder, error) {
	r.db.Exec("UPDATE reminders SET enabled = CASE WHEN enabled = 1 THEN 0 ELSE 1 END, updated_at = datetime('now', 'localtime') WHERE id = ?", id)
	var rem Reminder
	r.db.First(&rem, id)
	return &rem, nil
}

func (r *repository) PendingReminders() ([]Reminder, error) {
	var items []Reminder
	err := r.db.Where("enabled = 1 AND remind_at <= datetime('now', 'localtime', '+5 minutes')").
		Order("remind_at ASC").Limit(20).Find(&items).Error
	if items == nil {
		items = []Reminder{}
	}
	return items, err
}
