// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package user

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterUserRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	repo := NewRepository(ctx)
	svc := NewService(repo, ctx)
	handler := NewHandler(svc)

	authGroup := r.Group("/auth")
	{
		authGroup.GET("/status", handler.Status)
		authGroup.POST("/setup", handler.Setup)
		authGroup.POST("/login", handler.Login)
		authGroup.GET("/me", handler.Me)
		authGroup.POST("/logout", handler.Logout)
		authGroup.GET("/sessions", handler.ListSessions)
		authGroup.DELETE("/sessions/:id", handler.RevokeSession)
		authGroup.DELETE("/sessions", handler.RevokeAllSessions)
		authGroup.POST("/change-password", handler.ChangePassword)
	}
}
