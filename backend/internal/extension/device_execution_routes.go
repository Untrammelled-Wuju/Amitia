// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package extension

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
	"github.com/u-ai/backend/internal/middleware/security"
)

// RegisterDeviceExecutionPackageRoutes exposes only package lifecycle routes
// that must mutate the device-local extension kernel. The caller is expected
// to protect the group with Desktop Session authentication. Cloud Core remains
// the business/control plane; large .amitiax artifacts never traverse the
// Device Mesh runtime-invocation channel.
func RegisterDeviceExecutionPackageRoutes(group *gin.RouterGroup, runtime *Runtime) {
	if group == nil {
		return
	}
	group.Use(func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.UserID == "" || !actor.IsLocalTrusted {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "device-local desktop session required"})
			return
		}
		c.Set(authenticatedUserKey, string(actor.UserID))
		c.Next()
	})

	registerExtensionPackageRoutes(group, runtime)
	kernelAPI := NewKernelAPI(runtime)
	kernel := group.Group("/kernel")
	kernel.POST("/extensions/uninstall/preview", kernelAPI.previewUninstall)
	kernel.POST("/extensions/uninstall/confirm", kernelAPI.confirmUninstall)
	kernel.POST("/extensions/uninstall", kernelAPI.uninstall)
	kernel.POST("/extensions/resume-uninstall", kernelAPI.resumeUninstall)
}

// RegisterDeviceExecutionWorkflowRoutes exposes the device-local Workflow API
// with least-privilege authentication. Builder/CRUD remains Desktop Session
// only; trusted native producers may use the root local token only for the
// structured event ingress and runtime telemetry/audio endpoints.
func RegisterDeviceExecutionWorkflowRoutes(group *gin.RouterGroup, runtime *Runtime) {
	if group == nil || runtime == nil {
		return
	}
	api := NewWorkflowAPIForLocation(runtime, workflow.WorkflowLocationLocal)
	trustedActor := func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.UserID == "" || !actor.IsLocalTrusted {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "device-local authenticated session required"})
			return
		}
		c.Set(authenticatedUserKey, string(actor.UserID))
		c.Next()
	}

	management := group.Group("")
	management.Use(security.RequireAuthMethod(security.AuthMethodDesktopSession), trustedActor)
	api.registerWorkflowManagementRoutes(management)

	events := group.Group("")
	events.Use(security.RequireAuthMethod(security.AuthMethodDesktopSession, security.AuthMethodLocalToken), trustedActor)
	api.registerWorkflowEventRoute(events)

	runtimeProducer := group.Group("")
	runtimeProducer.Use(security.RequireAuthMethod(security.AuthMethodLocalToken), trustedActor)
	api.registerWorkflowRuntimeProducerRoutes(runtimeProducer)
}
