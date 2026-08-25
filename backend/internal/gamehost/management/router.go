package management

import (
	"github.com/gin-gonic/gin"
)

func RegisterGameCenterRouter(group *gin.RouterGroup, service *GameCenterManagementService) {
	handler := NewHandler(service)

	gameCenter := group.Group("/game-center")
	{
		gameCenter.GET("/health", handler.GetCenterHealth)
		gameCenter.GET("/developer-access", GetGameHostDeveloperAccess)
		gameCenter.GET("/plugins", handler.ListPlugins)
		gameCenter.GET("/plugins/:pluginId", handler.GetPlugin)
		gameCenter.GET("/plugins/:pluginId/health", handler.GetPluginHealth)

		gameCenter.GET("/runtimes", handler.ListRuntimes)
		gameCenter.GET("/runtimes/:runtimeId", handler.GetRuntime)
		gameCenter.GET("/runtimes/:runtimeId/services", handler.ListServices)
		gameCenter.GET("/runtimes/:runtimeId/health", handler.GetRuntimeHealth)
		gameCenter.GET("/runtimes/:runtimeId/handshake", handler.GetHandshakeStatus)
		gameCenter.GET("/runtimes/:runtimeId/authority", handler.GetControlAuthority)
		gameCenter.POST("/runtimes/:runtimeId/agent-context", handler.BindAgentContext)

		gameCenter.GET("/handshake", handler.GetHandshakeStatus)
		gameCenter.GET("/authority", handler.GetControlAuthority)
	}
}

func RegisterGameCenterApprovalRouter(group *gin.RouterGroup, handler *ApprovalHandler) {
	if handler == nil {
		return
	}
	gameCenter := group.Group("/game-center")
	gameCenter.GET("/approvals", handler.ListPending)
	gameCenter.POST("/approvals/:approvalId/approve", handler.Approve)
	gameCenter.POST("/approvals/:approvalId/reject", handler.Reject)
}

func RegisterGameCenterMutationRouter(group *gin.RouterGroup, mutationHandler *MutationHandler) {
	gameCenter := group.Group("/game-center")
	{
		gameCenter.POST("/plugins/install", mutationHandler.Install)
		gameCenter.POST("/extensions/:extensionId/update", mutationHandler.Update)
		gameCenter.POST("/extensions/:extensionId/enable", mutationHandler.Enable)
		gameCenter.POST("/extensions/:extensionId/disable", mutationHandler.Disable)
		gameCenter.DELETE("/extensions/:extensionId", mutationHandler.Uninstall)

		gameCenter.POST("/runtimes/:runtimeId/start", mutationHandler.StartRuntime)
		gameCenter.POST("/runtimes/:runtimeId/stop", mutationHandler.StopRuntime)
		gameCenter.POST("/runtimes/:runtimeId/restart", mutationHandler.RestartRuntime)
	}
}

func RegisterGameCenterControlRouter(group *gin.RouterGroup, controlHandler *ControlHandler) {
	gameCenter := group.Group("/game-center")
	{
		gameCenter.POST("/runtimes/:runtimeId/takeover", controlHandler.Takeover)
		gameCenter.POST("/runtimes/:runtimeId/release", controlHandler.Release)
		gameCenter.POST("/runtimes/:runtimeId/emergency-stop", controlHandler.EmergencyStop)
		gameCenter.POST("/runtimes/:runtimeId/rearm", controlHandler.Rearm)
	}
}

func RegisterRPCRouter(group *gin.RouterGroup, rpcHandler *RPCHandler) {
	gameCenter := group.Group("/game-center")
	gameCenter.Use(RequireGameHostDeveloperAccess())
	{
		gameCenter.POST("/runtimes/:runtimeId/services/:serviceId/rpc", rpcHandler.InvokeRPC)
	}
}

func RegisterGameCenterArtifactRouter(group *gin.RouterGroup, handler *ArtifactHandler) {
	gameCenter := group.Group("/game-center")
	gameCenter.Use(RequireGameHostDeveloperAccess())
	gameCenter.GET("/extensions/:extensionId/artifacts", handler.List)
	gameCenter.GET("/extensions/:extensionId/artifact-roots", handler.ListAuthorizedRoots)
	gameCenter.POST("/extensions/:extensionId/artifact-roots/authorize", handler.AuthorizeRoot)
	gameCenter.DELETE("/extensions/:extensionId/artifact-roots", handler.RevokeRoot)
	gameCenter.POST("/extensions/:extensionId/artifacts/deploy-required", handler.DeployRequired)
	gameCenter.POST("/extensions/:extensionId/artifacts/:artifactId/deploy", handler.Deploy)
	gameCenter.POST("/extensions/:extensionId/artifacts/:artifactId/verify", handler.Verify)
	gameCenter.DELETE("/extensions/:extensionId/artifacts/:artifactId", handler.Remove)
}
