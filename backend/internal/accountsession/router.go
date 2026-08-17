package accountsession

import (
	"github.com/gin-gonic/gin"
)

func RegisterPublicRoutes(group *gin.RouterGroup, handler *Handler) {
	group.POST("/auth/login", handler.Login)
	group.POST("/auth/refresh", handler.Refresh)
	group.POST("/auth/logout/revoke", handler.RevokeRefresh)
}

func RegisterAuthenticatedRoutes(group *gin.RouterGroup, handler *Handler) {
	group.POST("/auth/logout", handler.Logout)
	group.POST("/auth/logout-all", handler.LogoutAll)
	group.GET("/auth/sessions", handler.ListSessions)
	group.DELETE("/auth/sessions/:sessionId", handler.RevokeSession)
	group.DELETE("/auth/sessions", handler.RevokeOtherSessions)
}
