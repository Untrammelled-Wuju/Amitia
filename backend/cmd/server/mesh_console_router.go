package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension"
	mcpapi "github.com/u-ai/backend/internal/extension/mcpruntime/api"
	"github.com/u-ai/backend/internal/middleware/security"
)

func registerMeshConsoleRouter(mesh *gin.RouterGroup, services *AppServices, coreID string) {
	console := mesh.Group("/console")
	console.Use(meshConsoleGuard(services))
	if services.MCPRuntime != nil {
		mcpapi.RegisterConsoleRouter(console, services.MCPRuntime.API)
	}
	extension.RegisterConsoleAgentSkillRoutes(console, func() *extension.Runtime { return services.Extension })
}

func meshConsoleGuard(services *AppServices) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		policy, err := services.DeviceMesh.Coordination.Get(c.Request.Context(), actor.SpaceID.String(), actor.DeviceID.String())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"code": "mesh.policy_unavailable", "message": err.Error()})
			return
		}
		isConsole, err := services.DeviceMesh.Coordination.IsCoreConsole(c.Request.Context(), actor.SpaceID.String(), actor.DeviceID.String())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"code": "mesh.policy_unavailable", "message": err.Error()})
			return
		}
		if !policy.Administrator && !isConsole {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "mesh.administrator_required", "message": "在统筹中心执行该操作需要统筹中心管理员授权"})
			return
		}
		c.Next()
	}
}
