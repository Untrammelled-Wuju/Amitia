package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	authctx "github.com/u-ai/backend/internal/auth"
	devicemeshagent "github.com/u-ai/backend/internal/devicemesh/agent"
	deviceruntimeprotocol "github.com/u-ai/backend/internal/deviceruntime/protocol"
	extensionkernel "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/management"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

const (
	gameHostManagementInvokeHandler = "gamehost.management.http"
	// Device Mesh frames are capped at 1 MiB. Keep proxied HTTP bodies below
	// half that limit so the surrounding invocation/result envelopes remain
	// bounded after JSON encoding. Game Center management endpoints are JSON
	// control-plane calls, not artifact transport.
	maxGameCenterProxyBody = 256 << 10
)

type gameCenterProxyRequest struct {
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	RawQuery    string            `json:"rawQuery,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        []byte            `json:"body,omitempty"`
	ActorType   string            `json:"actorType,omitempty"`
	Roles       []string          `json:"roles,omitempty"`
	Permissions []string          `json:"permissions,omitempty"`
}

type gameCenterProxyResponse struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
}

func registerCloudGameCenterGateway(apiGroup *gin.RouterGroup, services *AppServices) {
	if apiGroup == nil || services == nil || services.RuntimeProfile != runtimeprofile.ProfileCloudCore {
		return
	}
	handler := func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.SpaceID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "unauthorized"})
			return
		}
		if services.DeviceMesh == nil || services.DeviceMesh.DeviceReg == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "device mesh unavailable"})
			return
		}

		targetDeviceID := strings.TrimSpace(c.GetHeader("X-Amitia-Target-Device-ID"))
		if targetDeviceID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code": 400,
				"msg":  "target device is required",
				"data": gin.H{"requiredHeader": "X-Amitia-Target-Device-ID"},
			})
			return
		}
		deviceID := runtimeidentity.ParseDeviceID(targetDeviceID)
		spaceID := runtimeidentity.SpaceID(actor.SpaceID)
		if err := services.DeviceMesh.DeviceReg.RequireDeviceOwnedBy(c.Request.Context(), spaceID, deviceID); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "target device is not owned by current user"})
			return
		}

		body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxGameCenterProxyBody+1))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "failed to read request body"})
			return
		}
		if len(body) > maxGameCenterProxyBody {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"code": 413, "msg": "game center request too large"})
			return
		}
		proxyReq := gameCenterProxyRequest{
			Method:      c.Request.Method,
			Path:        c.Request.URL.Path,
			RawQuery:    c.Request.URL.RawQuery,
			ActorType:   string(actor.PrincipalType),
			Roles:       append([]string(nil), actor.Capabilities...),
			Permissions: append([]string(nil), actor.Permissions...),
			Headers: map[string]string{
				"Accept":       c.GetHeader("Accept"),
				"Content-Type": c.GetHeader("Content-Type"),
			},
			Body: body,
		}
		payload, err := json.Marshal(proxyReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "failed to encode game center request"})
			return
		}
		invokeCtx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
		defer cancel()
		result, err := services.DeviceMesh.InvokeDeviceHandler(
			invokeCtx,
			spaceID,
			deviceID,
			gameHostManagementInvokeHandler,
			payload,
			90*time.Second,
		)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": err.Error(), "data": gin.H{"targetDeviceId": targetDeviceID}})
			return
		}
		if len(result.Structured) == 0 {
			c.JSON(http.StatusBadGateway, gin.H{"code": 502, "msg": "device returned an empty game center response"})
			return
		}
		var proxyResp gameCenterProxyResponse
		if err := json.Unmarshal(result.Structured, &proxyResp); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"code": 502, "msg": "invalid game center response from device"})
			return
		}
		for key, value := range proxyResp.Headers {
			switch http.CanonicalHeaderKey(key) {
			case "Content-Type", "Cache-Control", "ETag":
				if strings.TrimSpace(value) != "" {
					c.Header(key, value)
				}
			}
		}
		contentType := proxyResp.Headers["Content-Type"]
		if contentType == "" {
			contentType = "application/json; charset=utf-8"
		}
		status := proxyResp.StatusCode
		if status < 100 || status > 599 {
			status = http.StatusBadGateway
		}
		c.Data(status, contentType, []byte(proxyResp.Body))
	}

	apiGroup.Any("/game-center", handler)
	apiGroup.Any("/game-center/*proxyPath", handler)
}

func newGameHostManagementInvokeHandler(services *AppServices) devicemeshagent.CancellableRuntimeInvokeHandler {
	localHandler, buildErr := buildDeviceGameCenterHTTPHandler(services)
	return func(ctx context.Context, invoke deviceruntimeprotocol.RuntimeInvokePayload) (*deviceruntimeprotocol.RuntimeResultPayload, error) {
		if buildErr != nil {
			return nil, buildErr
		}
		if localHandler == nil {
			return nil, fmt.Errorf("gamehost management bridge unavailable")
		}
		var proxyReq gameCenterProxyRequest
		if err := json.Unmarshal(invoke.Input, &proxyReq); err != nil {
			return nil, fmt.Errorf("decode gamehost management request: %w", err)
		}
		if proxyReq.Path != "/api/game-center" && !strings.HasPrefix(proxyReq.Path, "/api/game-center/") {
			return nil, fmt.Errorf("gamehost management path is outside game-center namespace")
		}
		if len(proxyReq.Body) > maxGameCenterProxyBody {
			return nil, fmt.Errorf("gamehost management request exceeds size limit")
		}
		switch strings.ToUpper(strings.TrimSpace(proxyReq.Method)) {
		case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		default:
			return nil, fmt.Errorf("unsupported gamehost management method")
		}

		url := "http://device-agent.local" + proxyReq.Path
		if strings.TrimSpace(proxyReq.RawQuery) != "" {
			url += "?" + proxyReq.RawQuery
		}
		req := httptest.NewRequest(strings.ToUpper(proxyReq.Method), url, bytes.NewReader(proxyReq.Body))
		if ctx == nil {
			ctx = context.Background()
		}
		req = req.WithContext(ctx)
		for key, value := range proxyReq.Headers {
			if strings.TrimSpace(value) != "" {
				req.Header.Set(key, value)
			}
		}
		req.Header.Set("X-Amitia-Mesh-Space-ID", invoke.SpaceID.String())
		req.Header.Set("X-Amitia-Mesh-Device-ID", invoke.DeviceID.String())
		req.Header.Set("X-Amitia-Mesh-Runtime-ID", invoke.RuntimeID.String())
		req.Header.Set("X-Amitia-Mesh-Actor-Type", proxyReq.ActorType)
		req.Header.Set("X-Amitia-Mesh-Roles", strings.Join(proxyReq.Roles, ","))
		req.Header.Set("X-Amitia-Mesh-Permissions", strings.Join(proxyReq.Permissions, ","))
		recorder := httptest.NewRecorder()
		localHandler.ServeHTTP(recorder, req)
		if recorder.Body.Len() > maxGameCenterProxyBody {
			return nil, fmt.Errorf("gamehost management response exceeds device-mesh control-plane limit")
		}

		proxyResp := gameCenterProxyResponse{
			StatusCode: recorder.Code,
			Headers:    map[string]string{},
			Body:       recorder.Body.String(),
		}
		for _, key := range []string{"Content-Type", "Cache-Control", "ETag"} {
			if value := recorder.Header().Get(key); value != "" {
				proxyResp.Headers[key] = value
			}
		}
		encoded, err := json.Marshal(proxyResp)
		if err != nil {
			return nil, fmt.Errorf("encode gamehost management response: %w", err)
		}
		return &deviceruntimeprotocol.RuntimeResultPayload{
			InvocationID:         invoke.InvocationID,
			RuntimeSessionID:     invoke.RuntimeSessionID,
			ConnectionGeneration: invoke.ConnectionGeneration,
			DeviceID:             invoke.DeviceID,
			RuntimeID:            invoke.RuntimeID,
			Status:               string(capability.ToolResultStatusSuccess),
			Result:               encoded,
			CompletedAt:          time.Now().UTC(),
		}, nil
	}
}

func splitTrustedMeshClaims(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func buildDeviceGameCenterHTTPHandler(services *AppServices) (http.Handler, error) {
	if services == nil || services.KernelContainer == nil || services.KernelContainer.GameHost == nil {
		return nil, fmt.Errorf("device GameHost is not available")
	}
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(func(c *gin.Context) {
		actorType := authctx.PrincipalType(strings.TrimSpace(c.GetHeader("X-Amitia-Mesh-Actor-Type")))
		if actorType == "" {
			actorType = authctx.PrincipalDeviceRuntime
		}
		actor := &authctx.ActorContext{
			PrincipalType:  actorType,
			SpaceID:        runtimeidentity.SpaceID(strings.TrimSpace(c.GetHeader("X-Amitia-Mesh-Space-ID"))),
			DeviceID:       runtimeidentity.DeviceID(strings.TrimSpace(c.GetHeader("X-Amitia-Mesh-Device-ID"))),
			RuntimeID:      runtimeidentity.RuntimeID(strings.TrimSpace(c.GetHeader("X-Amitia-Mesh-Runtime-ID"))),
			Capabilities:   splitTrustedMeshClaims(c.GetHeader("X-Amitia-Mesh-Roles")),
			Permissions:    splitTrustedMeshClaims(c.GetHeader("X-Amitia-Mesh-Permissions")),
			AuthMethod:     "device_mesh",
			IsLocalTrusted: false,
		}
		c.Set("actorContext", actor)
		c.Request = c.Request.WithContext(authctx.WithActor(c.Request.Context(), actor))
		c.Next()
	})
	apiGroup := engine.Group("/api")
	registerDeviceGameCenterManagementRoutes(apiGroup, services)
	return engine, nil
}

func registerDeviceGameCenterManagementRoutes(apiGroup *gin.RouterGroup, services *AppServices) {
	container := services.KernelContainer
	kernelReader := management.NewKernelReaderWithContributions(container.DefinitionRepository, container.InstallationRepository, container.ContributionRepository)
	gameCenterSvc := management.NewProductionService(container.GameHost, kernelReader)
	management.RegisterGameCenterRouter(apiGroup, gameCenterSvc)
	if container.GameHost.ArtifactManager != nil {
		management.RegisterGameCenterArtifactRouter(apiGroup, management.NewArtifactHandler(container.GameHost.ArtifactManager))
	}
	if services.Extension == nil {
		return
	}

	kernelMutation := management.NewKernelMutationFromFuncs(management.KernelMutationOptions{
		EnableFn: func(ctx context.Context, extensionID string) error {
			return services.Extension.Kernel.Enable(ctx, extensionID)
		},
		DisableFn: func(ctx context.Context, extensionID string) error {
			return services.Extension.Kernel.Disable(ctx, extensionID)
		},
	})
	var upgradeCoordinator management.PackageUpgradeCoordinator
	if container.GameHost.UpgradeCoordinator != nil {
		upgradeCoordinator = &management.GameHostUpgradeCoordinatorAdapter{UC: container.GameHost.UpgradeCoordinator}
	}
	packageSvc := management.NewProductionPackageMutationServiceFromKernelReader(
		kernelReader,
		management.NewGameHostPluginRegistryFromContainer(container.GameHost),
		kernelMutation,
		upgradeCoordinator,
	)
	runtimeSvc := management.NewProductionRuntimeMutationService(container.GameHost)
	management.RegisterGameCenterMutationRouter(apiGroup, management.NewMutationHandler(packageSvc, runtimeSvc))

	if container.GameHost.TakeoverService != nil {
		controlHandler := management.NewControlHandlerFromFuncs(management.ControlServiceOptions{
			SupportsControlFn: func(ctx context.Context, runtimeID string) (bool, error) {
				runtimeRef, err := container.GameHost.RuntimeManager.GetRuntime(domain.RuntimeInstanceID(runtimeID))
				if err != nil {
					return false, err
				}
				plugin, err := container.GameHost.PluginRegistry.Get(ctx, runtimeRef.PluginID)
				if err != nil {
					return false, err
				}
				for _, feature := range plugin.Capabilities {
					if feature == domain.HostFeatureRealtimeControl {
						return true, nil
					}
				}
				return false, nil
			},
			TakeoverFn: func(ctx context.Context, runtimeID string) (management.TakeoverResult, error) {
				_, err := container.GameHost.TakeoverService.Takeover(ctx, control.TakeoverRequest{RuntimeID: domain.RuntimeInstanceID(runtimeID), Actor: "cloud_game_center_user"})
				return management.TakeoverResult{Success: err == nil}, err
			},
			ReleaseFn: func(ctx context.Context, runtimeID string, targetMode string, expectedEpoch uint64) (management.ReleaseResult, error) {
				_, err := container.GameHost.TakeoverService.Release(ctx, control.ReleaseRequest{
					RuntimeID: domain.RuntimeInstanceID(runtimeID), TargetMode: domain.ControlMode(targetMode), Actor: "cloud_game_center_user",
					ExpectedEpoch: expectedEpoch, UseExpected: expectedEpoch > 0,
				})
				return management.ReleaseResult{Success: err == nil}, err
			},
			EmergencyStopFn: func(ctx context.Context, runtimeID string) (control.EmergencyStopResult, error) {
				return container.GameHost.EmergencyStopService.Execute(ctx, domain.RuntimeInstanceID(runtimeID))
			},
			RearmFn: func(ctx context.Context, runtimeID string) error {
				return container.GameHost.EmergencyStopService.ClearEmergencyLatch(ctx, domain.RuntimeInstanceID(runtimeID), "cloud_game_center_user")
			},
		})
		management.RegisterGameCenterControlRouter(apiGroup, controlHandler)
	}

	if container.GameHost.ControlPlane != nil {
		rpcInvoker := management.NewControlPlaneRPCInvoker(
			container.GameHost.ControlPlane,
			management.NewGameHostTopologyStore(container.GameHost.RuntimeTopologyStore),
			management.NewGameHostPluginRegistry(container.GameHost.PluginRegistry),
			container.GameHost.RuntimeManager,
		)
		management.RegisterRPCRouter(apiGroup, management.NewRPCHandler(rpcInvoker))
		debugHandler := management.NewDebugHandler(container.GameHost, func(ctx context.Context, toolID string, input json.RawMessage, scope management.DebugToolScope) (any, bool) {
			if container.ToolFacade == nil {
				return nil, false
			}
			result, ok := container.ToolFacade.ExecuteTool(ctx, capability.CapabilityID(toolID), input, extensionkernel.InvocationScope{
				SpaceID: scope.SpaceID, CharacterID: scope.CharacterID, ConversationID: scope.ConversationID, Channel: scope.Channel,
				SessionID: scope.SessionID, RequestID: scope.RequestID, ToolCallID: scope.ToolCallID,
			}, scope.ToolCallID, scope.RequestID)
			return result, ok
		})
		management.RegisterDebugRouter(apiGroup, debugHandler)
	}
}
