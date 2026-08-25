package management

import (
	"github.com/gin-gonic/gin"
)

func RegisterDebugRouter(group *gin.RouterGroup, debugHandler *DebugHandler) {
	debug := group.Group("/game-center-debug")
	debug.Use(RequireGameHostDeveloperAccess())
	{
		debug.GET("/residue", debugHandler.GetResidueReport)
		debug.POST("/tools/invoke", debugHandler.InvokeTool)
	}
}
