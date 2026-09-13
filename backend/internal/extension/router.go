package extension

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/user"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterRouter(group *gin.RouterGroup, ctx *app.AppContext, runtime *Runtime) {
	userService := user.NewService(user.NewRepository(ctx), ctx)
	handler := NewHandler()
	agentSkillHandler := NewAgentSkillHandler(runtime.AgentSkills, handler)
	kernelAPI := NewKernelAPI(runtime)
	devModeAPI := NewDevModeAPI(runtime)
	hookAPI := NewHookAPI(runtime)
	trustedServiceAPI := NewTrustedServiceAPI(runtime)
	taskAPI := NewTaskAPI(runtime)
	eventAPI := NewEventAPI(runtime)
	scheduleAPI := NewScheduleAPI(runtime)
	workflowAPI := NewWorkflowAPI(runtime)
	uiAPI := NewUIAPI(runtime)
	desktopAPIAdapter := NewDesktopAPIAdapter(runtime)
	devConsoleAPI := NewDevConsoleAPI(runtime)
	updateAPI := NewUpdateAPI(runtime)
	canaryAPI := NewCanaryAPI(runtime)
	desktopPetPluginAPI := NewDesktopPetPluginAPI(runtime)
	extensions := group.Group("/extensions")
	extensions.GET("/openapi.json", handler.OpenAPI)

	registerRetiredLegacyRoutes(extensions, retiredExtensionLegacyRoutes)
	registerRetiredLegacyRoutes(group, retiredRootLegacyRoutes)

	extensions.Use(extensionAuth(userService))
	registerExtensionPackageRoutes(extensions, runtime)
	kernelAPI.RegisterRoutes(extensions)
	devModeAPI.RegisterRoutes(extensions)
	hookAPI.RegisterRoutes(extensions)
	trustedServiceAPI.RegisterRoutes(extensions)
	taskAPI.RegisterRoutes(extensions)
	eventAPI.RegisterRoutes(extensions)
	scheduleAPI.RegisterRoutes(extensions)
	workflowAPI.RegisterRoutes(extensions)
	uiAPI.RegisterRoutes(extensions, group)
	desktopAPIAdapter.RegisterRoutes(extensions)
	devConsoleAPI.RegisterRoutes(group)
	updateAPI.RegisterRoutes(extensions)
	canaryAPI.RegisterRoutes(extensions)
	desktopPetPluginAPI.RegisterRoutes(extensions)
	extensions.POST("/agent-skills/import/preview", agentSkillHandler.Preview)
	extensions.POST("/agent-skills/import/install", agentSkillHandler.Install)
	extensions.GET("/agent-skills", agentSkillHandler.List)
	extensions.GET("/agent-skills/metrics", agentSkillHandler.Metrics)
	extensions.GET("/agent-skills/:id", agentSkillHandler.Get)
	extensions.POST("/agent-skills/:id/enable", agentSkillHandler.Enable)
	extensions.POST("/agent-skills/:id/disable", agentSkillHandler.Disable)
	extensions.DELETE("/agent-skills/:id", agentSkillHandler.Remove)
	extensions.GET("/agent-skills/:id/compatibility", agentSkillHandler.Compatibility)
	extensions.GET("/agent-skills/:id/resources", agentSkillHandler.Resources)
	extensions.GET("/agent-skills/:id/resources/content", agentSkillHandler.ResourceContent)
	extensions.GET("/agent-skills/:id/assets/content", agentSkillHandler.AssetContent)
	extensions.GET("/agent-skills/:id/activations", agentSkillHandler.Activations)
}

func extensionAuth(service user.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if actor := security.GetActor(c); actor != nil && actor.UserID != "" && actor.IsLocalTrusted {
			c.Set(authenticatedUserKey, string(actor.UserID))
			c.Next()
			return
		}
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			writeAuthProblem(c, "Authentication required")
			return
		}
		me, err := service.GetMe(strings.TrimPrefix(auth, "Bearer "))
		if err != nil || me == nil || me.ID == 0 {
			writeAuthProblem(c, "Invalid authentication token")
			return
		}
		c.Set(authenticatedUserKey, me.ID)
		c.Next()
	}
}

func writeAuthProblem(c *gin.Context, detail string) {
	c.Header("Content-Type", "application/problem+json")
	c.Abort()
	c.JSON(http.StatusUnauthorized, ProblemDetail{Type: "https://errors.amitia.dev/extensions/unauthorized", Title: "Unauthorized", Status: http.StatusUnauthorized, Detail: detail, Instance: c.Request.URL.Path, Code: "UNAUTHORIZED", TraceID: c.GetString("trace_request_id")})
}
