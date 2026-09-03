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
	handler := NewHandler(runtime.Service)
	workshopHandler := NewWorkshopHandler(runtime.Workshop, handler)
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
	extensions.GET("/capabilities", handler.Capabilities)
	extensions.GET("/skills", handler.ListSkills)
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
	extensions.GET("/workshop/metrics", workshopHandler.Metrics)
	extensions.POST("/workshop/instructions/generate", workshopHandler.GenerateInstruction)
	extensions.GET("/workshop/sessions", workshopHandler.ListSessions)
	extensions.POST("/workshop/sessions", workshopHandler.CreateSession)
	extensions.GET("/workshop/sessions/:id", workshopHandler.GetSession)
	extensions.POST("/workshop/sessions/:id/archive", workshopHandler.Archive)
	extensions.GET("/workshop/sessions/:id/revisions", workshopHandler.ListRevisions)
	extensions.GET("/workshop/sessions/:id/revisions/:revision", workshopHandler.GetRevision)
	extensions.POST("/workshop/sessions/:id/generate", workshopHandler.Generate)
	extensions.POST("/workshop/sessions/:id/revisions/:revision/validate", workshopHandler.Validate)
	extensions.POST("/workshop/sessions/:id/revisions/:revision/permissions/confirm", workshopHandler.ConfirmPermissions)
	extensions.POST("/workshop/sessions/:id/revisions/:revision/test", workshopHandler.Test)
	extensions.POST("/workshop/sessions/:id/revisions/:revision/install", workshopHandler.Install)
	extensions.GET("/workshop/sessions/:id/tests", workshopHandler.ListTests)
	extensions.GET("/workshop/sessions/:id/artifact", workshopHandler.GetArtifact)
	extensions.POST("/workshop/sessions/:id/export", workshopHandler.Export)
	extensions.GET("/workshop/tests/:testRunId", workshopHandler.GetTest)
	extensions.GET("/plugins", handler.ListPlugins)
	extensions.GET("/plugins/:id", handler.GetPlugin)
	extensions.POST("/plugins/:id/enable", handler.EnablePlugin)
	extensions.POST("/plugins/:id/disable", handler.DisablePlugin)
	extensions.POST("/plugins/:id/reload", handler.ReloadPlugin)
	extensions.GET("/plugins/:id/config", handler.GetPluginConfig)
	extensions.PUT("/plugins/:id/config", handler.UpdatePluginConfig)
	extensions.POST("/plugins/:id/config/reset", handler.ResetPluginConfig)
	extensions.GET("/plugins/:id/permissions", handler.GetPluginPermissions)
	extensions.PUT("/plugins/:id/permissions", handler.UpdatePluginPermissions)
	extensions.GET("/plugins/:id/health", handler.GetPluginHealth)
	extensions.POST("/plugins/:id/circuit/reset", handler.ResetPluginCircuit)
	extensions.GET("/plugins/:id/state", handler.GetPluginState)
	extensions.GET("/plugins/:id/surface", handler.GetPluginSurface)
	extensions.GET("/plugins/:id/schedules", handler.GetPluginSchedules)
	extensions.POST("/plugins/:id/schedules/:scheduleId/pause", handler.PausePluginSchedule)
	extensions.POST("/plugins/:id/schedules/:scheduleId/resume", handler.ResumePluginSchedule)
	extensions.GET("/plugins/:id/events", handler.GetPluginEvents)
	extensions.GET("/plugins/:id/events/dead-letter", handler.GetPluginDeadLetters)
	extensions.POST("/plugins/:id/events/:eventId/retry", handler.RetryPluginEvent)
	extensions.POST("/plugins/:id/surface/actions/:actionId", handler.ExecutePluginSurfaceAction)
	extensions.GET("/skills/:id", handler.GetSkill)
	extensions.POST("/skills/:id/enable", handler.EnableSkill)
	extensions.POST("/skills/:id/disable", handler.DisableSkill)
	extensions.GET("/skills/:id/permissions", handler.GetPermissions)
	extensions.PUT("/skills/:id/permissions", handler.UpdatePermissions)
	extensions.GET("/skills/:id/config", handler.GetConfig)
	extensions.PUT("/skills/:id/config", handler.UpdateConfig)
	extensions.POST("/skills/:id/config/reset", handler.ResetConfig)
	extensions.POST("/skills/:id/execute", handler.Execute)
	extensions.POST("/skills/:id/workshop/fork", workshopHandler.Fork)
	extensions.POST("/skills/:id/versions/:version/rollback", workshopHandler.Rollback)
	extensions.GET("/runs", handler.ListRuns)
	extensions.GET("/runs/:runId", handler.GetRun)
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
