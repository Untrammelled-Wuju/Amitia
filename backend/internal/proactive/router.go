// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package proactive

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/runtimegate"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterProactiveRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	RegisterProactiveRouterWithCompanion(r, ctx, nil)
}

func RegisterProactiveRouterWithCompanion(r *gin.RouterGroup, ctx *app.AppContext, compSvc ProactiveDispatcher) *Handler {
	repo := NewRepository(ctx)
	svc := NewService(repo, ctx)
	handler := NewHandler(svc, ctx.DB, compSvc)
	proactiveGroup := r.Group("/proactive")
	proactiveGroup.Use(proactivePluginGuard())

	proactiveGroup.GET("/rules", handler.ListRules)
	proactiveGroup.POST("/rules", handler.CreateRule)
	proactiveGroup.PUT("/rules/:id", handler.UpdateRule)
	proactiveGroup.DELETE("/rules/:id", handler.DeleteRule)
	proactiveGroup.POST("/rules/:id/toggle", handler.ToggleRule)
	proactiveGroup.GET("/status", handler.Status)
	proactiveGroup.GET("/reminders", handler.ListReminders)
	proactiveGroup.POST("/reminders", handler.CreateReminder)
	proactiveGroup.PUT("/reminders/:id", handler.UpdateReminder)
	proactiveGroup.DELETE("/reminders/:id", handler.DeleteReminder)
	proactiveGroup.POST("/reminders/:id/toggle", handler.ToggleReminder)
	proactiveGroup.POST("/reminders/test/:id", handler.TestReminder)
	proactiveGroup.POST("/reminders/:id/trigger", handler.TriggerReminder)
	proactiveGroup.POST("/reminders/cancel-latest", handler.CancelLatestReminder)
	proactiveGroup.GET("/reminders/status", handler.ReminderStatus)
	proactiveGroup.GET("/reminders/pending", handler.PendingReminders)
	proactiveGroup.DELETE("/reminders", handler.CancelRemindersByQuery)
	proactiveGroup.GET("/history", handler.ListTriggerHistory)
	proactiveGroup.GET("/queue-summary", handler.QueueSummary)
	proactiveGroup.GET("/prospective", handler.Prospective)
	proactiveGroup.POST("/rules/test/:id", handler.TestRule)
	proactiveGroup.POST("/rules/:id/trigger", handler.TriggerRule)
	proactiveGroup.POST("/presets/reset", handler.ResetPresets)
	proactiveGroup.GET("/rules/:id/messages", handler.RuleMessages)
	proactiveGroup.GET("/settings/cleanup", security.SharedCoreAdminOnly(), handler.GetCleanupConfig)
	proactiveGroup.POST("/settings/cleanup", security.SharedCoreAdminOnly(), handler.SetCleanupConfig)
	return handler
}

func RegisterRemindersRouter(r *gin.RouterGroup, h *Handler) {
	reminders := r.Group("/reminders")
	reminders.Use(proactivePluginGuard())
	reminders.GET("", h.ListReminders)
	reminders.POST("", h.CreateReminder)
	reminders.PUT("/:id", h.UpdateReminder)
	reminders.DELETE("/:id", h.DeleteReminder)
	reminders.POST("/:id/toggle", h.ToggleReminder)
	reminders.POST("/:id/test", h.TestReminder)
	reminders.POST("/:id/trigger", h.TriggerReminder)
	reminders.GET("/status", h.ReminderStatus)
	reminders.GET("/cleanup-config", security.SharedCoreAdminOnly(), h.GetCleanupConfig)
	reminders.PUT("/cleanup-config", security.SharedCoreAdminOnly(), h.SetCleanupConfig)
	reminders.POST("/clear-backpressure", security.SharedCoreAdminOnly(), h.ClearBackpressure)
	reminders.GET("/stream", h.RemindersStream)
	reminders.GET("/prospective", h.Prospective)
	reminders.GET("/queue-summary", h.QueueSummary)
	reminders.GET("/trigger-history", h.ListTriggerHistory)
}

func proactivePluginGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !runtimegate.IsEnabled(runtimegate.ProactiveExtensionID) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"code": 404, "msg": "主动消息插件未启用"})
			return
		}
		c.Next()
	}
}
