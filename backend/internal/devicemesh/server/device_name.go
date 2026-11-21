package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func makeReportDeviceNameHandler(deps *RouterDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := security.GetActor(c)
		deviceID := runtimeidentity.ParseDeviceID(c.Param("deviceId"))
		if actor == nil || actor.DeviceID == "" || actor.DeviceID != deviceID {
			c.JSON(http.StatusForbidden, gin.H{"code": "mesh.device_identity_mismatch", "message": "设备名只能由该设备自身上报"})
			return
		}
		var req struct {
			Name string `json:"name"`
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "mesh.invalid_device_name", "message": err.Error()})
			return
		}
		record, err := deps.DeviceReg.ReportDeviceName(c.Request.Context(), deviceID, req.Name)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "mesh.invalid_device_name", "message": err.Error()})
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{"device": deviceNamePayload(record)})
	}
}

func makeConfirmDeviceNameHandler(deps *RouterDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || !actor.HasPermission(auth.PermSystemAdmin) {
			c.JSON(http.StatusForbidden, gin.H{"code": "mesh.administrator_required", "message": "设备改名只能由统筹中心管理员确认"})
			return
		}
		deviceID := runtimeidentity.ParseDeviceID(c.Param("deviceId"))
		finish, valid := security.BeginDeviceManagementIntent(c, deps.Coordination)
		if !valid {
			return
		}
		defer finish()
		if err := deps.DeviceReg.RequireDeviceOwnedBy(c.Request.Context(), actor.SpaceID, deviceID); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"code": "mesh.device_not_found", "message": err.Error()})
			return
		}
		record, err := deps.DeviceReg.ConfirmDeviceName(c.Request.Context(), actor.SpaceID, deviceID)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"code": "mesh.device_name_not_pending", "message": err.Error()})
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{"device": deviceNamePayload(record)})
	}
}

func deviceNamePayload(record *host_registry.DeviceRecord) gin.H {
	if record == nil {
		return gin.H{}
	}
	return gin.H{
		"deviceId":     record.DeviceID.String(),
		"label":        record.Label,
		"reportedName": record.ReportedName,
		"pendingName":  record.PendingName,
		"nameRevision": record.NameRevision,
	}
}
