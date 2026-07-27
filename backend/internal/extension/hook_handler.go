package extension

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel/hook"
)

type HookAPI struct {
	runtime *Runtime
}

func NewHookAPI(runtime *Runtime) *HookAPI {
	return &HookAPI{runtime: runtime}
}

func (api *HookAPI) RegisterRoutes(group *gin.RouterGroup) {
	hooks := group.Group("/hooks")
	hooks.GET("/points", api.listPoints)
	hooks.GET("/contributions", api.listContributions)
	hooks.GET("/contributions/:id", api.getContribution)
	hooks.POST("/contributions/:id/enable", api.enableContribution)
	hooks.POST("/contributions/:id/disable", api.disableContribution)
	hooks.GET("/contributions/:id/circuit", api.getCircuit)
	hooks.POST("/contributions/:id/circuit/reset", api.resetCircuit)
	hooks.GET("/points/:pointId/contributions", api.listByPoint)
}

func (api *HookAPI) service(c *gin.Context) *hook.Service {
	if api.runtime == nil || api.runtime.Kernel == nil {
		return nil
	}
	container := api.runtime.Kernel.Container()
	if container == nil {
		return nil
	}
	return container.HookService
}

func (api *HookAPI) listPoints(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "hook service unavailable"})
		return
	}
	points, err := svc.ReadModel.ListHookPoints(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"points": points, "total": len(points)})
}

func (api *HookAPI) listContributions(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "hook service unavailable"})
		return
	}
	ctx := c.Request.Context()
	extensionID := c.Query("extensionId")

	var contribs []hook.HookContributionSummary
	var err error
	if extensionID != "" {
		contribs, err = svc.ReadModel.ListContributions(ctx, extensionID)
	} else {
		all, listErr := svc.ContribStore.List(ctx)
		if listErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": listErr.Error()})
			return
		}
		contribs = make([]hook.HookContributionSummary, 0, len(all))
		for _, contrib := range all {
			contribs = append(contribs, svc.ReadModel.GetSummary(contrib))
		}
		err = nil
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contributions": contribs, "total": len(contribs)})
}

func (api *HookAPI) listByPoint(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "hook service unavailable"})
		return
	}
	pointID := c.Param("pointId")
	contribs, err := svc.ReadModel.ListContributionsByPoint(c.Request.Context(), pointID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contributions": contribs, "total": len(contribs)})
}

func (api *HookAPI) getContribution(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "hook service unavailable"})
		return
	}
	id := c.Param("id")
	contrib, err := svc.ReadModel.GetContribution(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, contrib)
}

func (api *HookAPI) enableContribution(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "hook service unavailable"})
		return
	}
	id := c.Param("id")
	if err := svc.Lifecycle.EnableContribution(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contributionId": id, "enabled": true})
}

func (api *HookAPI) disableContribution(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "hook service unavailable"})
		return
	}
	id := c.Param("id")
	if err := svc.Lifecycle.DisableContribution(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contributionId": id, "enabled": false})
}

func (api *HookAPI) getCircuit(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "hook service unavailable"})
		return
	}
	id := c.Param("id")
	stats, err := svc.ReadModel.GetCircuitStats(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (api *HookAPI) resetCircuit(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "hook service unavailable"})
		return
	}
	id := c.Param("id")
	if err := svc.ReadModel.ResetCircuit(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contributionId": id, "reset": true})
}
