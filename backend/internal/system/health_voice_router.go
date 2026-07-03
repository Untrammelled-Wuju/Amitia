// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

func RegisterHealthRouter(r *gin.RouterGroup, cbRegistry *mindruntime.CircuitBreakerRegistry, dataLifecycle *mindruntime.DataLifecycleCoordinator, reconciliation *mindruntime.ReconciliationEngine) {
	r.GET("/health/circuit-breakers", func(c *gin.Context) {
		reports := cbRegistry.AllHealthReports()
		result := make([]gin.H, 0, len(reports))
		for _, report := range reports {
			result = append(result, gin.H{
				"name":    report.Name,
				"healthy": report.Healthy,
				"state":   string(report.CircuitBreaker.Status()),
				"message": report.CheckMessage,
			})
		}
		util.SuccessResponse(c, result)
	})

	r.POST("/health/circuit-breakers/:name/reset", func(c *gin.Context) {
		name := c.Param("name")
		cb := cbRegistry.Get(name)
		if cb == nil {
			util.ErrorResponse(c, response.NotFound, "breaker not found", nil)
			return
		}
		cb.Reset()
		util.SuccessMsgResponse(c, "breaker reset", gin.H{"name": name, "state": string(cb.Status())})
	})

	r.GET("/health/data-lifecycle", func(c *gin.Context) {
		stats := dataLifecycle.Stats()
		util.SuccessResponse(c, stats)
	})

	r.GET("/health/reconciliation", func(c *gin.Context) {
		scans := reconciliation.AllScans()
		util.SuccessResponse(c, gin.H{
			"status": string(reconciliation.Status()),
			"scans":  scans,
		})
	})

	r.POST("/health/reconciliation/run", func(c *gin.Context) {
		var body struct {
			Target   string `json:"target"`
			Strategy string `json:"strategy"`
		}
		c.ShouldBindJSON(&body)
		target := mindruntime.ReconciliationTarget(body.Target)
		strategy := mindruntime.ReconciliationStrategy(body.Strategy)
		if strategy == "" {
			strategy = mindruntime.StrategyManualConfirm
		}
		scan, err := reconciliation.RunScan(c.Request.Context(), target, strategy, "")
		if err != nil {
			util.ErrorResponse(c, response.InternalError, err.Error(), nil)
			return
		}
		util.SuccessMsgResponse(c, "scan started", scan)
	})
}

func RegisterVoiceEntryRouter(r *gin.RouterGroup, voiceEntry *interaction.VoiceEntry) {
	r.POST("/voice/session", func(c *gin.Context) {
		var body struct {
			SessionID      string `json:"sessionId"`
			ConversationID string `json:"conversationId"`
			CharacterID    string `json:"characterId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			util.ErrorResponse(c, response.InvalidParams, "invalid request", nil)
			return
		}
		session := voiceEntry.CreateSession(body.SessionID, body.ConversationID, body.CharacterID)
		util.SuccessMsgResponse(c, "session created", gin.H{
			"sessionId":      session.SessionID,
			"conversationId": session.ConversationID,
			"characterId":    session.CharacterID,
			"state":          string(session.GetState()),
		})
	})

	r.POST("/voice/turn", func(c *gin.Context) {
		var req interaction.VoiceTurnRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			util.ErrorResponse(c, response.InvalidParams, "invalid request", nil)
			return
		}
		result, err := voiceEntry.HandleTurn(c.Request.Context(), &req)
		if err != nil {
			util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
			return
		}
		if result == nil {
			util.SuccessResponse(c, gin.H{"status": "interim"})
			return
		}
		reply := ""
		if result.Response != nil {
			reply = result.Response.Reply
		}
		util.SuccessResponse(c, gin.H{
			"status":  "final",
			"reply":   reply,
			"message": result.Response,
		})
	})

	r.POST("/voice/session/:id/interrupt", func(c *gin.Context) {
		sessionID := c.Param("id")
		var body struct {
			Policy string `json:"policy"`
		}
		c.ShouldBindJSON(&body)
		policy := interaction.VoiceInterruptPolicy(body.Policy)
		if policy == "" {
			policy = interaction.VoiceInterruptPolicyImmediate
		}
		if err := voiceEntry.SetInterruptPolicy(sessionID, policy); err != nil {
			util.ErrorResponse(c, response.NotFound, err.Error(), nil)
			return
		}
		util.SuccessMsgResponse(c, "interrupt policy set", nil)
	})

	r.DELETE("/voice/session/:id", func(c *gin.Context) {
		sessionID := c.Param("id")
		voiceEntry.RemoveSession(sessionID)
		util.SuccessMsgResponse(c, "session removed", nil)
	})

	r.GET("/voice/session/:id", func(c *gin.Context) {
		sessionID := c.Param("id")
		session := voiceEntry.GetSession(sessionID)
		if session == nil {
			util.ErrorResponse(c, response.NotFound, "session not found", nil)
			return
		}
		util.SuccessResponse(c, gin.H{
			"sessionId":      session.SessionID,
			"conversationId": session.ConversationID,
			"characterId":    session.CharacterID,
			"state":          string(session.GetState()),
			"currentText":    session.GetCurrentText(),
		})
	})
}
