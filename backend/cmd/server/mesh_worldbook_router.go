package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/middleware/security"
)

func registerMeshWorldbookRouter(mesh *gin.RouterGroup, services *AppServices, coreID string) {
	admin := security.SharedCoreDataWriteOnly()
	mesh.POST("/worldbook/manage", admin, func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 320<<10)
		var input business.WorldbookMutation
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Errorf("世界书保存参数无效: %w", err).Error()})
			return
		}
		if input.ExpectedScope == nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "世界书保存参数无效或缺少原服务范围"})
			return
		}
		if _, err := uuid.Parse(input.RequestID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
		policy, err := services.DeviceMesh.Coordination.Get(c.Request.Context(), actor.SpaceID.String(), actor.DeviceID.String())
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": err.Error()})
			return
		}
		resourceOwner, roleOwner := actor.DeviceID.String(), coreID
		if policy.Coordinated {
			resourceOwner = coreID
		}
		result, err := services.OwnedBusiness.ManageWorldbook(c.Request.Context(), business.Request{
			SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID,
			TargetDeviceID: actor.DeviceID.String(), ResourceOwnerID: resourceOwner, RoleOwnerID: roleOwner, RoleID: input.RoleID, RequestID: input.RequestID,
		}, input)
		if err != nil {
			writeWorldbookError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": result})
	})
	mesh.POST("/worldbook/import", admin, func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 9<<20)
		var input business.WorldbookImport
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Errorf("世界书批量导入参数无效: %w", err).Error()})
			return
		}
		if input.ExpectedScope == nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "世界书批量导入参数无效或缺少原服务范围"})
			return
		}
		if _, err := uuid.Parse(input.RequestID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
		policy, err := services.DeviceMesh.Coordination.Get(c.Request.Context(), actor.SpaceID.String(), actor.DeviceID.String())
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": err.Error()})
			return
		}
		resourceOwner, roleOwner := actor.DeviceID.String(), coreID
		if policy.Coordinated {
			resourceOwner = coreID
		}
		result, err := services.OwnedBusiness.ImportWorldbook(c.Request.Context(), business.Request{
			SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID,
			TargetDeviceID: actor.DeviceID.String(), ResourceOwnerID: resourceOwner, RoleOwnerID: roleOwner, RoleID: input.RoleID, RequestID: input.RequestID,
		}, input)
		if err != nil {
			writeWorldbookError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": result})
	})
	mesh.POST("/worldbook/match", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 320<<10)
		var input struct {
			RequestID      string                       `json:"requestId"`
			RoleID         string                       `json:"characterId"`
			TargetDeviceID string                       `json:"targetDeviceId"`
			ExpectedScope  *coordination.ExecutionScope `json:"expectedExecutionScope"`
			Text           string                       `json:"text"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Errorf("世界书匹配参数无效: %w", err).Error()})
			return
		}
		if input.ExpectedScope == nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "世界书匹配参数无效"})
			return
		}
		if _, err := uuid.Parse(input.RequestID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
		policy, err := services.DeviceMesh.Coordination.Get(c.Request.Context(), actor.SpaceID.String(), actor.DeviceID.String())
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": err.Error()})
			return
		}
		resourceOwner, roleOwner := actor.DeviceID.String(), coreID
		if policy.Coordinated {
			resourceOwner = coreID
		}
		result, err := services.OwnedBusiness.MatchWorldbook(c.Request.Context(), business.Request{
			SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID,
			TargetDeviceID: actor.DeviceID.String(), ResourceOwnerID: resourceOwner, RoleOwnerID: roleOwner, RoleID: input.RoleID, RequestID: input.RequestID,
		}, input.RoleID, actor.DeviceID.String(), input.ExpectedScope, input.Text)
		if err != nil {
			writeWorldbookError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": result})
	})
}

func writeWorldbookError(c *gin.Context, err error) {
	status, code := http.StatusBadRequest, "mesh.invalid_worldbook"
	switch {
	case errors.Is(err, coordination.ErrCapabilityGrant):
		status, code = http.StatusForbidden, "mesh.target_grant_required"
	case errors.Is(err, coordination.ErrWrongOwner):
		status, code = http.StatusForbidden, "mesh.wrong_owner"
	case errors.Is(err, coordination.ErrScopeExpired):
		status, code = http.StatusConflict, "mesh.scope_expired"
	case errors.Is(err, coordination.ErrResourceVersion):
		status, code = http.StatusConflict, "mesh.owned_resource_version"
	case errors.Is(err, coordination.ErrRequestConflict):
		status, code = http.StatusConflict, "mesh.request_conflict"
	}
	c.JSON(status, gin.H{"code": code, "message": err.Error()})
}
