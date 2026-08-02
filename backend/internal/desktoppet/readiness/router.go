// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package readiness

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func RegisterRouter(r *gin.RouterGroup, db *gorm.DB, ext *extension.Runtime) {
	svc := NewStartupReadinessService(db, ext)

	r.GET("/desktop-pets/readiness/complete", func(c *gin.Context) {
		snapshot := svc.Snapshot()
		httpStatus := 200
		if snapshot.OverallStatus == StatusBlocked {
			httpStatus = 503
		} else if snapshot.OverallStatus == StatusDegraded {
			httpStatus = 200
		}
		c.JSON(httpStatus, gin.H{
			"code": httpStatus,
			"msg":  string(snapshot.OverallStatus),
			"data": snapshot,
		})
	})

	r.GET("/desktop-pets/readiness/startup", func(c *gin.Context) {
		snapshot := svc.Snapshot()
		ready := snapshot.BlockingCount == 0
		result := gin.H{
			"ready":         ready,
			"overallStatus": snapshot.OverallStatus,
			"timestamp":     snapshot.Timestamp,
		}
		if !ready {
			blocking := make([]string, 0)
			for name, check := range snapshot.Checks {
				if check.Status == StatusBlocked && check.Required {
					blocking = append(blocking, name)
				}
			}
			result["blocking"] = blocking
			c.JSON(503, gin.H{
				"code": 503,
				"msg":  "system not ready",
				"data": result,
			})
			return
		}
		c.JSON(200, gin.H{
			"code": 200,
			"msg":  "system is ready",
			"data": result,
		})
	})
}

func RegisterRouterWithContext(r *gin.RouterGroup, ctx *app.AppContext, ext *extension.Runtime) {
	RegisterRouter(r, ctx.DB, ext)
}
