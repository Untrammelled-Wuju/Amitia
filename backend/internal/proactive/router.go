// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package proactive

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterProactiveRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	repo := NewRepository(ctx)
	svc := NewService(repo, ctx)
	handler := NewHandler(svc, ctx.DB)

	r.GET("/proactive/rules", handler.ListRules)
	r.POST("/proactive/rules", handler.CreateRule)
	r.PUT("/proactive/rules/:id", handler.UpdateRule)
	r.DELETE("/proactive/rules/:id", handler.DeleteRule)
	r.POST("/proactive/rules/:id/toggle", handler.ToggleRule)
	r.POST("/proactive/rules/:id/test", handler.TestRule)
	r.POST("/proactive/rules/:id/trigger", handler.TriggerRule)
	r.POST("/proactive/rules/reset-presets", handler.ResetPresets)
	r.GET("/proactive/rules/:id/messages", handler.RuleMessages)
	r.GET("/proactive/status", handler.Status)

	r.GET("/reminders", handler.ListReminders)
	r.POST("/reminders", handler.CreateReminder)
	r.PUT("/reminders/:id", handler.UpdateReminder)
	r.DELETE("/reminders/:id", handler.DeleteReminder)
	r.POST("/reminders/:id/toggle", handler.ToggleReminder)
	r.POST("/reminders/:id/test", handler.TestReminder)
	r.POST("/reminders/:id/trigger", handler.TriggerReminder)
	r.POST("/reminders/cancel-by-query", handler.CancelRemindersByQuery)
	r.POST("/reminders/cancel-latest", handler.CancelLatestReminder)
	r.GET("/reminders/status", handler.ReminderStatus)
	r.GET("/reminders/pending", handler.PendingReminders)
	r.GET("/reminders/cleanup-config", handler.GetCleanupConfig)
	r.PUT("/reminders/cleanup-config", handler.SetCleanupConfig)
}
