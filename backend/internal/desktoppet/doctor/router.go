// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package doctor

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func RegisterRouter(r *gin.RouterGroup, db *gorm.DB, ext *extension.Runtime) {
	doc := NewDoctor(db, ext)

	r.GET("/desktop-pets/doctor/status", func(c *gin.Context) {
		report := doc.RunChecks()
		if report.Status == "blocked" {
			c.JSON(503, gin.H{
				"code": 503,
				"msg":  "system has blocking issues",
				"data": report,
			})
			return
		}
		c.JSON(200, gin.H{
			"code": 200,
			"msg":  "ok",
			"data": report,
		})
	})

	r.GET("/desktop-pets/doctor/components", func(c *gin.Context) {
		report := doc.RunChecks()
		components := make([]gin.H, 0)
		for _, f := range report.Findings {
			components = append(components, gin.H{
				"category": f.Category,
				"code":     f.Code,
				"severity": f.Severity,
				"message":  f.Message,
			})
		}
		c.JSON(200, gin.H{
			"code": 200,
			"msg":  "ok",
			"data": gin.H{
				"status":     report.Status,
				"mode":       report.Mode,
				"components": components,
			},
		})
	})
}

func RegisterRouterWithContext(r *gin.RouterGroup, ctx *app.AppContext, ext *extension.Runtime) {
	RegisterRouter(r, ctx.DB, ext)
}
