// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package proactive

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterProactiveRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	RegisterProactiveRouterWithCompanion(r, ctx, nil)
}

func RegisterProactiveRouterWithCompanion(r *gin.RouterGroup, ctx *app.AppContext, compSvc ProactiveDispatcher) {
	repo := NewRepository(ctx)
	svc := NewService(repo, ctx)
	handler := NewHandler(svc, ctx.DB, compSvc)

	r.GET("/proactive/rules", handler.ListRules)
	r.POST("/proactive/rules", handler.CreateRule)
	r.PUT("/proactive/rules/:id", handler.UpdateRule)
	r.DELETE("/proactive/rules/:id", handler.DeleteRule)
	r.POST("/proactive/rules/:id/toggle", handler.ToggleRule)
	r.GET("/proactive/status", handler.Status)
	r.GET("/proactive/reminders", handler.ListReminders)
	r.POST("/proactive/reminders", handler.CreateReminder)
	r.PUT("/proactive/reminders/:id", handler.UpdateReminder)
	r.DELETE("/proactive/reminders/:id", handler.DeleteReminder)
	r.POST("/proactive/reminders/:id/toggle", handler.ToggleReminder)
	r.POST("/proactive/reminders/test/:id", handler.TestReminder)
	r.POST("/proactive/reminders/:id/trigger", handler.TriggerReminder)
	r.POST("/proactive/reminders/cancel-latest", handler.CancelLatestReminder)
	r.GET("/proactive/reminders/status", handler.ReminderStatus)
	r.GET("/proactive/reminders/pending", handler.PendingReminders)
	r.DELETE("/proactive/reminders", handler.CancelRemindersByQuery)
	r.GET("/proactive/history", handler.ListTriggerHistory)
	r.GET("/proactive/queue-summary", handler.QueueSummary)
	r.GET("/proactive/prospective", handler.Prospective)
	r.POST("/proactive/rules/test/:id", handler.TestRule)
	r.POST("/proactive/rules/:id/trigger", handler.TriggerRule)
	r.POST("/proactive/presets/reset", handler.ResetPresets)
	r.GET("/proactive/rules/:id/messages", handler.RuleMessages)
	r.GET("/proactive/settings/cleanup", handler.GetCleanupConfig)
	r.POST("/proactive/settings/cleanup", handler.SetCleanupConfig)
}
