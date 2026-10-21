package main

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/middleware/security"
)

func registerMeshRuntimeStateRouter(mesh *gin.RouterGroup, services *AppServices, coreID string) {
	mesh.GET("/runtime-state", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		view, err := services.OwnedBusiness.RuntimeState(c.Request.Context(), business.Request{
			SpaceID:        actor.SpaceID.String(),
			DeviceID:       actor.DeviceID.String(),
			CoreID:         coreID,
			TargetDeviceID: c.Query("targetDeviceId"),
			RoleID:         c.Query("characterId"),
		})
		if err != nil {
			writeRuntimeStateError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": view})
	})
}

func writeRuntimeStateError(c *gin.Context, err error) {
	status, code := http.StatusBadRequest, "mesh.invalid_runtime_state"
	switch {
	case errors.Is(err, coordination.ErrCapabilityGrant):
		status, code = http.StatusForbidden, "mesh.target_grant_required"
	case errors.Is(err, coordination.ErrRoleRequired), errors.Is(err, coordination.ErrRoleSelection):
		status, code = http.StatusConflict, "mesh.role_required"
	case errors.Is(err, coordination.ErrWrongOwner):
		status, code = http.StatusForbidden, "mesh.wrong_owner"
	case errors.Is(err, coordination.ErrScopeExpired):
		status, code = http.StatusConflict, "mesh.scope_expired"
	case errors.Is(err, coordination.ErrResourceVersion):
		status, code = http.StatusConflict, "mesh.owned_resource_version"
	}
	c.JSON(status, gin.H{"code": code, "message": err.Error()})
}
