// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package proactive

import (
	"context"

	"github.com/u-ai/backend/pkg/sse"
	"gorm.io/gorm"
)

type ProactiveDispatcher interface {
	DispatchProactiveMessage(ctx context.Context, userID, characterID, conversationID, channel, prompt, requestID string) (string, error)
}

type scopedProactiveService interface {
	ListRulesForUser(characterID, userID string) ([]map[string]interface{}, error)
	FindRuleForUser(id int, userID string) (*ProactiveRule, error)
	CreateRuleForUser(req *CreateRuleRequest, userID string) (*ProactiveRule, error)
	UpdateRuleForUser(id int, updates map[string]interface{}, userID string) (*ProactiveRule, error)
	DeleteRuleForUser(id int, userID string) error
	ToggleRuleForUser(id int, userID string) (*ProactiveRule, error)
	DeleteRulesByCharacterForUser(characterID, userID string) error
	CreateRuleDirectForUser(rule *ProactiveRule, userID string) error
	ListRemindersForUser(userID string) ([]Reminder, error)
	FindReminderForUser(id int, userID string) (*Reminder, error)
	CreateReminderForUser(req *CreateReminderRequest, userID string) (*Reminder, error)
	UpdateReminderForUser(id int, updates map[string]interface{}, userID string) (*Reminder, error)
	DeleteReminderForUser(id int, userID string) error
	ToggleReminderForUser(id int, userID string) (*Reminder, error)
	PendingRemindersForUser(userID string) ([]Reminder, error)
	ListTriggerHistoryForUser(page, pageSize int, state, userID string) ([]TriggerHistory, int64, error)
}

type Handler struct {
	service Service
	db      *gorm.DB
	compSvc ProactiveDispatcher
}

func NewHandler(srv Service, db *gorm.DB, compSvc ProactiveDispatcher) *Handler {
	return &Handler{service: srv, db: db, compSvc: compSvc}
}

func (h *Handler) broadcastReminderChange(userID string) {
	owner := normalizeProactiveOwner(userID)
	sse.Global.BroadcastToUser(owner, "changed", map[string]interface{}{"type": "reminder", "userId": owner})
}
