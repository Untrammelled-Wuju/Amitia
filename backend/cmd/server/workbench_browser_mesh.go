package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	devicemeshagent "github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/internal/workbenchbrowser"
)

func registerDeviceWorkbenchBrowserHandler(dispatcher interface {
	RegisterCancellable(string, devicemeshagent.CancellableRuntimeInvokeHandler)
}, services *AppServices) {
	dispatcher.RegisterCancellable(devicemeshagent.RemoteBrowserReadHandler, func(ctx context.Context, invocation protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
		if services == nil || services.Browser == nil || len(invocation.Input) > 2048 || invocation.SpaceID == "" || invocation.DeviceID == "" {
			return nil, errors.New("设备浏览器运行环境不可用")
		}
		if services.DeviceMesh == nil || services.DeviceMesh.LocalHandler == nil {
			return nil, errors.New("设备认证状态不可用")
		}
		identity, err := services.DeviceMesh.LocalHandler.LoadIdentity()
		if err != nil || identity == nil || identity.DeviceID != invocation.DeviceID || identity.RuntimeID != invocation.RuntimeID {
			return nil, errors.New("浏览器远程调用设备身份不匹配")
		}
		credential, err := services.DeviceMesh.LocalHandler.LoadCredential()
		if err != nil || credential == nil || credential.SpaceID != invocation.SpaceID || credential.DeviceID != invocation.DeviceID || credential.RuntimeID != invocation.RuntimeID || credential.CredentialID == "" || credential.Credential == "" || !time.Now().Before(credential.ExpiresAt) {
			return nil, errors.New("浏览器远程调用未通过配对认证")
		}
		var request workbenchbrowser.RemoteReadRequest
		if err := json.Unmarshal(invocation.Input, &request); err != nil || !request.Valid() {
			return nil, errors.New("浏览器远程观察请求无效")
		}
		result, err := workbenchbrowser.ReadAgentTab(ctx, services.Browser, invocation.SpaceID.String(), request)
		if err != nil {
			var recognized bool
			result, recognized = workbenchbrowser.RemoteReadFailureResult(err)
			if !recognized {
				return nil, err
			}
		}
		latest, err := services.DeviceMesh.LocalHandler.LoadCredential()
		if err != nil || latest == nil || latest.SpaceID != invocation.SpaceID || latest.DeviceID != invocation.DeviceID || latest.RuntimeID != invocation.RuntimeID || latest.CredentialID != credential.CredentialID || subtle.ConstantTimeCompare([]byte(latest.Credential), []byte(credential.Credential)) != 1 || !time.Now().Before(latest.ExpiresAt) {
			return nil, errors.New("浏览器远程调用期间配对认证已失效")
		}
		payload, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		return &protocol.RuntimeResultPayload{InvocationID: invocation.InvocationID, Status: "success", Result: payload}, nil
	})
}

func registerCloudWorkbenchBrowserGateway(group *gin.RouterGroup, services *AppServices) {
	if group == nil || services == nil || services.RuntimeProfile != runtimeprofile.ProfileCloudCore {
		return
	}
	group.POST("/device-mesh/v1/workbench/browser/read", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store, private")
		c.Header("X-Content-Type-Options", "nosniff")
		actor := security.GetActor(c)
		if actor == nil || actor.SpaceID == "" || actor.DeviceID == "" || !actor.HasPermission(auth.PermWorkbenchBrowserControl) || (actor.PrincipalType != auth.PrincipalTrustedDevice && actor.PrincipalType != auth.PrincipalLocalUI) {
			c.JSON(http.StatusForbidden, gin.H{"error": "没有远程浏览器观察权限", "errorCode": "access_denied", "retryable": false})
			return
		}
		if services.DeviceMesh == nil || services.DeviceMesh.DeviceReg == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Device Mesh 未就绪"})
			return
		}
		var request struct {
			TargetDeviceID string `json:"targetDeviceId"`
			workbenchbrowser.RemoteReadRequest
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式无效"})
			return
		}
		target := runtimeidentity.ParseDeviceID(strings.TrimSpace(request.TargetDeviceID))
		if target == "" || !request.RemoteReadRequest.Valid() || len(request.TargetDeviceID) > 160 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "必须指定有效的目标设备、会话、观察引用与读取类型"})
			return
		}
		space := runtimeidentity.SpaceID(actor.SpaceID)
		if err := services.DeviceMesh.DeviceReg.RequireTrustedDevice(c.Request.Context(), space, actor.DeviceID); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "当前设备未获信任", "errorCode": "access_denied", "retryable": false})
			return
		}
		if err := services.DeviceMesh.DeviceReg.RequireTrustedDevice(c.Request.Context(), space, target); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "目标设备未配对或已撤销信任", "errorCode": "access_denied", "retryable": false})
			return
		}
		if services.DeviceMesh.Hub == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "设备通道不可用"})
			return
		}
		originalConnection, connected := services.DeviceMesh.Hub.GetByDevice(space, target)
		if !connected {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "目标设备离线", "errorCode": "device_offline", "retryable": true})
			return
		}
		input, err := json.Marshal(request.RemoteReadRequest)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求编码失败"})
			return
		}
		deadline := 15 * time.Second
		requestContext, cancel := context.WithTimeout(c.Request.Context(), deadline)
		defer cancel()
		result, err := services.DeviceMesh.InvokeDeviceHandlerWithRuntimeType(requestContext, space, target, capability.RuntimeTypeBrowser, devicemeshagent.RemoteBrowserReadHandler, input, deadline)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "设备浏览器观察暂时不可用", "errorCode": "device_unavailable", "retryable": true})
			return
		}
		if err := services.DeviceMesh.DeviceReg.RequireTrustedDevice(c.Request.Context(), space, actor.DeviceID); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "当前设备的信任授权已撤销", "errorCode": "access_denied", "retryable": false})
			return
		}
		if err := services.DeviceMesh.DeviceReg.RequireTrustedDevice(c.Request.Context(), space, target); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "目标设备的信任授权已撤销", "errorCode": "access_denied", "retryable": false})
			return
		}
		currentConnection, connected := services.DeviceMesh.Hub.GetByDevice(space, target)
		if !connected || currentConnection != originalConnection {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "目标设备连接已变化，请重新观察", "errorCode": "device_reconnected", "retryable": true})
			return
		}
		if len(result.Structured) == 0 || len(result.Structured) > 4<<20 || !json.Valid(result.Structured) {
			c.JSON(http.StatusBadGateway, gin.H{"error": "目标设备返回了无效的浏览器数据", "errorCode": "invalid_device_response", "retryable": true})
			return
		}
		var reply workbenchbrowser.RemoteReadResult
		if err := json.Unmarshal(result.Structured, &reply); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "浏览器观察数据解码失败", "errorCode": "invalid_device_response", "retryable": true})
			return
		}
		if reply.ErrorCode != "" {
			failure, valid := workbenchbrowser.NormalizeRemoteReadFailure(reply)
			if !valid {
				c.JSON(http.StatusBadGateway, gin.H{"error": "目标设备返回了无效的失败信息", "errorCode": "invalid_device_response", "retryable": true})
				return
			}
			status := remoteBrowserReadHTTPStatus(failure.ErrorCode)
			c.JSON(status, gin.H{"error": failure.Error, "errorCode": failure.ErrorCode, "retryable": failure.Retryable})
			return
		}
		if !workbenchbrowser.ValidRemoteReadResult(request.Operation, reply) {
			c.JSON(http.StatusBadGateway, gin.H{"error": "目标设备未返回有效观察状态", "errorCode": "invalid_device_response", "retryable": true})
			return
		}
		c.JSON(http.StatusOK, reply)
	})
}

func remoteBrowserReadHTTPStatus(code workbenchbrowser.RemoteReadErrorCode) int {
	switch code {
	case workbenchbrowser.RemoteReadInvalidRequest:
		return http.StatusBadRequest
	case workbenchbrowser.RemoteReadCapabilityRevoked:
		return http.StatusGone
	case workbenchbrowser.RemoteReadUnsupported:
		return http.StatusNotImplemented
	case workbenchbrowser.RemoteReadResourceExceeded:
		return http.StatusBadGateway
	default:
		return http.StatusServiceUnavailable
	}
}
