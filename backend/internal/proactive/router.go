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
	proactiveGroup.GET("/history", handler.ListTriggerHistory)
	proactiveGroup.GET("/queue-summary", handler.QueueSummary)
	proactiveGroup.GET("/prospective", handler.Prospective)
	proactiveGroup.POST("/rules/test/:id", handler.TestRule)
	proactiveGroup.POST("/rules/:id/trigger", handler.TriggerRule)
	proactiveGroup.POST("/presets/reset", handler.ResetPresets)
	proactiveGroup.GET("/rules/:id/messages", handler.RuleMessages)
	return handler
}

func RegisterProactiveRouterWithPlugin(r *gin.RouterGroup, ctx *app.AppContext, compSvc ProactiveDispatcher, pluginExecutor PluginToolExecutor) *Handler {
	repo := NewRepository(ctx)
	svc := NewService(repo, ctx)
	reminderHandler := NewHandler(svc, ctx.DB, compSvc)
	pluginHandler := NewPluginHandler(pluginExecutor, ctx.DB)
	proactiveGroup := r.Group("/proactive")
	proactiveGroup.Use(proactivePluginGuard())

	proactiveGroup.GET("/rules", pluginHandler.ListRules)
	proactiveGroup.POST("/rules", pluginHandler.CreateRule)
	proactiveGroup.PUT("/rules/:id", pluginHandler.UpdateRule)
	proactiveGroup.DELETE("/rules/:id", pluginHandler.DeleteRule)
	proactiveGroup.POST("/rules/:id/toggle", pluginHandler.ToggleRule)
	proactiveGroup.POST("/rules/test/:id", pluginHandler.TestRule)
	proactiveGroup.POST("/rules/:id/trigger", pluginHandler.TriggerRule)
	proactiveGroup.GET("/rules/:id/messages", pluginHandler.RuleMessages)
	proactiveGroup.POST("/presets/reset", pluginHandler.ResetPresets)
	proactiveGroup.GET("/status", pluginHandler.Status)
	proactiveGroup.GET("/history", pluginHandler.ListTriggerHistory)
	proactiveGroup.GET("/queue-summary", pluginHandler.QueueSummary)
	proactiveGroup.GET("/settings", pluginHandler.GetSettings)
	proactiveGroup.PUT("/settings", pluginHandler.UpdateSettings)
	return reminderHandler
}

func RegisterRemindersRouter(r *gin.RouterGroup, h *Handler) {
	reminders := r.Group("/reminders")
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
