package extension

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel/desktop"
)

type DesktopAPIAdapter struct {
	runtime *Runtime
}

func NewDesktopAPIAdapter(runtime *Runtime) *DesktopAPIAdapter {
	return &DesktopAPIAdapter{runtime: runtime}
}

func (a *DesktopAPIAdapter) container() *desktopCtx {
	if a.runtime == nil || a.runtime.Kernel == nil {
		return nil
	}
	c := a.runtime.Kernel.Container()
	if c == nil {
		return nil
	}
	return &desktopCtx{
		desktopAPI: c.DesktopAPI,
		updateAPI:  c.UpdateAPI,
	}
}

type desktopCtx struct {
	desktopAPI *desktop.DesktopAPI
	updateAPI  *desktop.UpdateAPI
}

func (a *DesktopAPIAdapter) RegisterRoutes(group *gin.RouterGroup) {
	desktopGroup := group.Group("/desktop")
	desktopGroup.GET("/contributions", a.listAllContributions)
	desktopGroup.GET("/contributions/:contributionId", a.getContribution)
	desktopGroup.POST("/contributions/:contributionId/enable", a.enableContribution)
	desktopGroup.POST("/contributions/:contributionId/disable", a.disableContribution)
	desktopGroup.POST("/contributions/:contributionId/invoke", a.invokeAction)
	desktopGroup.POST("/shortcuts/:contributionId/rebind", a.rebindShortcut)
	desktopGroup.GET("/conflicts", a.listConflicts)
	desktopGroup.POST("/conflicts/:conflictId/resolve", a.resolveConflict)
	desktopGroup.GET("/snapshot", a.getSnapshot)
	desktopGroup.POST("/snapshot/build", a.buildSnapshot)
	desktopGroup.GET("/contracts", a.listContracts)
	desktopGroup.GET("/permissions", a.listPermissions)
	desktopGroup.GET("/resources", a.listResources)
	desktopGroup.GET("/audit", a.listAuditEntries)
	desktopGroup.GET("/circuit/status", a.circuitStatus)
	desktopGroup.POST("/circuit/reset", a.circuitReset)

	group.GET("/:id/updates", a.listUpdates)
	group.POST("/:id/updates/check", a.checkUpdates)
	group.POST("/:id/updates/download", a.downloadUpdate)
	group.POST("/:id/updates/install", a.installUpdate)
	group.POST("/:id/updates/cancel", a.cancelUpdate)
	group.POST("/:id/updates/retry", a.retryUpdate)
	group.POST("/:id/updates/rollback", a.rollbackUpdate)
	group.GET("/updates/operations/:operationId", a.getOperation)
	group.GET("/updates/operations/:operationId/steps", a.getOperationSteps)
}

func (a *DesktopAPIAdapter) listAllContributions(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.desktopAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "desktop host unavailable"})
		return
	}
	ctx.desktopAPI.ListAllContributions(c)
}

func (a *DesktopAPIAdapter) getContribution(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.desktopAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "desktop host unavailable"})
		return
	}
	ctx.desktopAPI.GetContributionByID(c)
}

func (a *DesktopAPIAdapter) enableContribution(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.desktopAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "desktop host unavailable"})
		return
	}
	ctx.desktopAPI.EnableContributionByID(c)
}

func (a *DesktopAPIAdapter) disableContribution(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.desktopAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "desktop host unavailable"})
		return
	}
	ctx.desktopAPI.DisableContributionByID(c)
}

func (a *DesktopAPIAdapter) invokeAction(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.desktopAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "desktop host unavailable"})
		return
	}
	ctx.desktopAPI.InvokeContributionAction(c)
}

func (a *DesktopAPIAdapter) rebindShortcut(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.desktopAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "desktop host unavailable"})
		return
	}
	ctx.desktopAPI.RebindShortcutByID(c)
}

func (a *DesktopAPIAdapter) listConflicts(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.desktopAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "desktop host unavailable"})
		return
	}
	ctx.desktopAPI.ListAllConflicts(c)
}

func (a *DesktopAPIAdapter) resolveConflict(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.desktopAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "desktop host unavailable"})
		return
	}
	ctx.desktopAPI.ResolveConflictByID(c)
}

func (a *DesktopAPIAdapter) getSnapshot(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.desktopAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "desktop host unavailable"})
		return
	}
	ctx.desktopAPI.GetCurrentSnapshot(c)
}

func (a *DesktopAPIAdapter) buildSnapshot(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.desktopAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "desktop host unavailable"})
		return
	}
	ctx.desktopAPI.BuildNewSnapshot(c)
}

func (a *DesktopAPIAdapter) listContracts(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.desktopAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "desktop host unavailable"})
		return
	}
	ctx.desktopAPI.ListAllContracts(c)
}

func (a *DesktopAPIAdapter) listPermissions(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.desktopAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "desktop host unavailable"})
		return
	}
	ctx.desktopAPI.ListAllPermissions(c)
}

func (a *DesktopAPIAdapter) listResources(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.desktopAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "desktop host unavailable"})
		return
	}
	ctx.desktopAPI.ListAllResources(c)
}

func (a *DesktopAPIAdapter) listAuditEntries(c *gin.Context) {
	container := a.runtime.Kernel.Container()
	if container == nil || container.DesktopActionBridge == nil {
		c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
		return
	}
	filterType := c.Query("type")
	entries := container.DesktopActionBridge.GetAuditRecorder().ListEntries(filterType, 100)
	c.JSON(http.StatusOK, gin.H{"items": entries, "total": len(entries)})
}

func (a *DesktopAPIAdapter) circuitStatus(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.desktopAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "desktop host unavailable"})
		return
	}
	ctx.desktopAPI.CircuitStatus(c)
}

func (a *DesktopAPIAdapter) circuitReset(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.desktopAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "desktop host unavailable"})
		return
	}
	ctx.desktopAPI.CircuitReset(c)
}

func (a *DesktopAPIAdapter) listUpdates(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.updateAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "update manager unavailable"})
		return
	}
	ctx.updateAPI.ListUpdates(c)
}

func (a *DesktopAPIAdapter) checkUpdates(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.updateAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "update manager unavailable"})
		return
	}
	ctx.updateAPI.CheckUpdates(c)
}

func (a *DesktopAPIAdapter) downloadUpdate(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.updateAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "update manager unavailable"})
		return
	}
	ctx.updateAPI.DownloadUpdate(c)
}

func (a *DesktopAPIAdapter) installUpdate(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.updateAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "update manager unavailable"})
		return
	}
	ctx.updateAPI.InstallUpdate(c)
}

func (a *DesktopAPIAdapter) cancelUpdate(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.updateAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "update manager unavailable"})
		return
	}
	ctx.updateAPI.CancelUpdate(c)
}

func (a *DesktopAPIAdapter) retryUpdate(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.updateAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "update manager unavailable"})
		return
	}
	ctx.updateAPI.RetryUpdate(c)
}

func (a *DesktopAPIAdapter) rollbackUpdate(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.updateAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "update manager unavailable"})
		return
	}
	ctx.updateAPI.RollbackUpdate(c)
}

func (a *DesktopAPIAdapter) getOperation(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.updateAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "update manager unavailable"})
		return
	}
	ctx.updateAPI.GetOperation(c)
}

func (a *DesktopAPIAdapter) getOperationSteps(c *gin.Context) {
	ctx := a.container()
	if ctx == nil || ctx.updateAPI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "update manager unavailable"})
		return
	}
	ctx.updateAPI.GetOperationSteps(c)
}
