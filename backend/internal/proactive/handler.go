// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package proactive

import (
	"context"

	"github.com/u-ai/backend/pkg/sse"
	"gorm.io/gorm"
)

type ProactiveDispatcher interface {
	DispatchProactiveMessage(ctx context.Context, characterID, conversationID, channel, prompt, requestID string) (string, error)
}

type Handler struct {
	service Service
	db      *gorm.DB
	compSvc ProactiveDispatcher
}

func NewHandler(srv Service, db *gorm.DB, compSvc ProactiveDispatcher) *Handler {
	return &Handler{service: srv, db: db, compSvc: compSvc}
}

func (h *Handler) broadcastReminderChange() {
	sse.Global.Broadcast("changed", map[string]interface{}{"type": "reminder"})
}
