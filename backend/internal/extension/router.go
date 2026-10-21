package extension

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterRouter(group *gin.RouterGroup, ctx *app.AppContext, runtime *Runtime) {
	handler := NewHandler()
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
	petPluginAPI := NewPetPluginAPI(runtime)
	extensions := group.Group("/extensions")
	extensions.GET("/openapi.json", handler.OpenAPI)

	registerRetiredLegacyRoutes(extensions, retiredExtensionLegacyRoutes)
	registerRetiredLegacyRoutes(group, retiredRootLegacyRoutes)
	registerSelfHostedExtensionRegistryRoutes(group, runtime)

	extensions.Use(extensionAuth())
	registerExtensionPackageRoutes(extensions, runtime)
	registerPackageDistributionRoutes(extensions, runtime)
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
	petPluginAPI.RegisterRoutes(extensions)
}

func extensionAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.SpaceID == "" {
			writeAuthProblem(c, "Space principal required")
			return
		}
		c.Next()
	}
}

func authenticatedSpaceID(c *gin.Context) string {
	actor := security.GetActor(c)
	if actor == nil || actor.SpaceID == "" {
		return ""
	}
	return actor.SpaceID.String()
}

func writeAuthProblem(c *gin.Context, detail string) {
	c.Header("Content-Type", "application/problem+json")
	c.Abort()
	c.JSON(http.StatusUnauthorized, ProblemDetail{Type: "https://errors.amitia.dev/extensions/unauthorized", Title: "Unauthorized", Status: http.StatusUnauthorized, Detail: detail, Instance: c.Request.URL.Path, Code: "UNAUTHORIZED", TraceID: c.GetString("trace_request_id")})
}
