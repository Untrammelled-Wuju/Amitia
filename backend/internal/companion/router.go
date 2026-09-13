// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/extension/runtimegate"
	"github.com/u-ai/backend/internal/requestidentity"
)

func RegisterCompanionRouter(r *gin.RouterGroup, svc Service) {
	handler := NewHandler(svc)

	comp := r.Group("/companion")
	comp.Use(companionCharacterScopeGuard(svc))
	{
		comp.GET("/sleep-setting", handler.GetSleepSetting)
		comp.PUT("/sleep-setting", handler.UpdateSleepSetting)
		comp.GET("/schedule", handler.GetSchedule)
		comp.GET("/schedule/conflicts", handler.GetScheduleConflicts)
		comp.GET("/schedule/today", handler.GetScheduleToday)
		comp.POST("/schedule/regenerate", handler.RegenerateSchedule)
		comp.GET("/state/life", handler.GetStateLife)
		comp.GET("/state", handler.GetState)
		comp.GET("/mind-state", handler.GetMindState)
		comp.GET("/timeline/today", handler.GetTimelineToday)
		comp.POST("/timeline/regenerate", handler.RegenerateTimeline)

		comp.GET("/fixed-events", handler.ListFixedEvents)
		comp.POST("/fixed-events", handler.CreateFixedEvent)
		comp.PUT("/fixed-events/:id", handler.UpdateFixedEvent)
		comp.DELETE("/fixed-events/:id", handler.DeleteFixedEvent)
		comp.PATCH("/fixed-events/:id/enabled", handler.ToggleFixedEventEnabled)

		comp.GET("/special-events", handler.ListSpecialEvents)
		comp.POST("/special-events", handler.CreateSpecialEvent)
		comp.PUT("/special-events/:id", handler.UpdateSpecialEvent)
		comp.DELETE("/special-events/:id", handler.DeleteSpecialEvent)
		comp.PATCH("/special-events/:id/enabled", handler.ToggleSpecialEventEnabled)

		comp.GET("/class-adjustments", handler.ListClassAdjustments)
		comp.POST("/class-adjustments", handler.CreateClassAdjustment)
		comp.PUT("/class-adjustments/:id", handler.UpdateClassAdjustment)
		comp.DELETE("/class-adjustments/:id", handler.DeleteClassAdjustment)
		comp.GET("/classes/effective", handler.GetEffectiveClasses)

		comp.GET("/lifestyle-tendency", handler.GetLifestyleTendency)
		comp.PUT("/lifestyle-tendency", handler.UpdateLifestyleTendency)
		comp.POST("/lifestyle-tendency/reset", handler.ResetLifestyleTendency)
		comp.GET("/work-profile", handler.GetWorkProfile)
		comp.PUT("/work-profile", handler.UpdateWorkProfile)

		comp.GET("/active-message/setting", activeMessagePluginGuard(), handler.GetActiveMessageSetting)
		comp.PUT("/active-message/setting", activeMessagePluginGuard(), handler.UpdateActiveMessageSetting)
		comp.GET("/active-message/tasks/today", activeMessagePluginGuard(), handler.GetActiveMessageTasksToday)
		comp.POST("/active-message/tasks/regenerate", activeMessagePluginGuard(), handler.RegenerateActiveMessageTasks)
		comp.POST("/active-message/tasks/:id/run", activeMessagePluginGuard(), handler.RunActiveMessageTask)
		comp.POST("/active-message/tasks/:id/cancel", activeMessagePluginGuard(), handler.CancelActiveMessageTask)

		comp.GET("/delayed-replies", handler.ListDelayedReplies)
		comp.POST("/delayed-replies/:id/cancel", handler.CancelDelayedReply)
		comp.POST("/delayed-replies/process", handler.ProcessDelayedReplies)

		comp.GET("/debug/overview", handler.GetDebugOverview)
		comp.POST("/debug/regenerate-all", handler.RegenerateAllDebug)
		comp.POST("/debug/process-active-messages", activeMessagePluginGuard(), handler.ProcessActiveMessagesDebug)
		comp.POST("/debug/process-delayed-replies", handler.ProcessDelayedRepliesDebug)
		comp.POST("/debug/trigger-daily-regeneration", handler.TriggerDailyRegeneration)

		comp.GET("/rule-logs", handler.GetRuleLogs)
	}
}

func activeMessagePluginGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !runtimegate.IsEnabled(runtimegate.ProactiveExtensionID) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"code": 404, "msg": "主动消息插件未启用"})
			return
		}
		c.Next()
	}
}

func companionCharacterScopeGuard(svc Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.AppCfg != nil && strings.EqualFold(strings.TrimSpace(config.AppCfg.Security.Mode), "local_single_user") {
			c.Next()
			return
		}
		characterID := strings.TrimSpace(c.Query("characterId"))
		if characterID == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "characterId is required in shared cloud mode"})
			return
		}
		owned, err := svc.CharacterOwnedBy(requestidentity.ResolveGin(c, ""), characterID)
		if err != nil || !owned {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"code": 404, "msg": "character not found"})
			return
		}
		c.Next()
	}
}
