package management

import (
	"github.com/gin-gonic/gin"
)

func RegisterGameCenterRouter(group *gin.RouterGroup, service *GameCenterManagementService) {
	handler := NewHandler(service)

	gameCenter := group.Group("/game-center")
	{
		gameCenter.GET("/plugins", handler.ListPlugins)
		gameCenter.GET("/plugins/:pluginId", handler.GetPlugin)
		gameCenter.GET("/plugins/:pluginId/health", handler.GetPluginHealth)

		gameCenter.GET("/runtimes", handler.ListRuntimes)
		gameCenter.GET("/runtimes/:runtimeId", handler.GetRuntime)
		gameCenter.GET("/runtimes/:runtimeId/services", handler.ListServices)
		gameCenter.GET("/runtimes/:runtimeId/health", handler.GetRuntimeHealth)
		gameCenter.GET("/runtimes/:runtimeId/handshake", handler.GetHandshakeStatus)
		gameCenter.GET("/runtimes/:runtimeId/authority", handler.GetControlAuthority)

		gameCenter.GET("/handshake", handler.GetHandshakeStatus)
		gameCenter.GET("/authority", handler.GetControlAuthority)
	}
}
