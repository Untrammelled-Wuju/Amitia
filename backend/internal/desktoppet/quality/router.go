// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import "github.com/gin-gonic/gin"

func RegisterQualityRouter(r *gin.RouterGroup, svc QualityService) {
	handler := NewHandler(svc)

	g := r.Group("/desktop-pets/quality")
	{
		g.GET("/evaluations/:evaluationId", handler.GetEvaluation)
		g.GET("/processing-tasks/:processingTaskId/actions/:actionKey", handler.GetActiveActionQuality)
		g.POST("/evaluations/:evaluationId/reevaluate", handler.Reevaluate)
		g.GET("/processing-tasks/:processingTaskId/gate", handler.GetTaskGate)
		g.GET("/evaluations/:evaluationId/problem-frames", handler.ListProblemFrames)
		g.GET("/evaluations/:evaluationId/findings", handler.ListFindings)
	}
}
