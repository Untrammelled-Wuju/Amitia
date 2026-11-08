package main

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh"
	"github.com/u-ai/backend/internal/devicemesh/capabilities"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

func gameCenterBridgeTestServices(t *testing.T) (*AppServices, *coordination.Service) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "game-center-bridge.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := sqlite.Migrate(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	registry := host_registry.NewRegistry(db)
	for _, deviceID := range []string{"device-a", "device-b"} {
		record, err := registry.EnsureDevice(t.Context(), host_registry.DeviceRecord{
			SpaceID:    "space-a",
			DeviceID:   runtimeidentity.DeviceID(deviceID),
			TrustState: host_registry.DeviceTrustTrusted,
		})
		if err != nil {
			t.Fatal(err)
		}
		if record.TrustState != host_registry.DeviceTrustTrusted {
			t.Fatalf("device %s is not trusted: %+v", deviceID, record)
		}
	}
	service := coordination.NewService(db)
	return &AppServices{
		RuntimeProfile: runtimeprofile.ProfileCloudCore,
		DeviceMesh: &devicemesh.Runtime{
			DB:           db,
			DeviceReg:    registry,
			Coordination: service,
		},
	}, service
}

func gameCenterBridgeTestRouter(services *AppServices, actor *auth.ActorContext) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("actorContext", actor)
		c.Next()
	})
	registerCloudGameCenterGateway(router.Group("/api"), services)
	return router
}

func TestGameCenterBridgeRequiresExplicitCrossDeviceGrantAndExposure(t *testing.T) {
	services, coordinationService := gameCenterBridgeTestServices(t)
	router := gameCenterBridgeTestRouter(services, &auth.ActorContext{
		PrincipalType: auth.PrincipalTrustedDevice,
		SpaceID:       "space-a",
		DeviceID:      "device-a",
	})
	call := func() *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, "/api/game-center/plugins", bytes.NewBuffer(nil))
		request.Header.Set("X-Amitia-Target-Device-ID", "device-b")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}

	denied := call()
	if denied.Code != http.StatusForbidden || !bytes.Contains(denied.Body.Bytes(), []byte("mesh.cross_device_game_forbidden")) {
		t.Fatalf("ungranted cross-device call status=%d body=%s", denied.Code, denied.Body.String())
	}
	if _, err := coordinationService.SetCapabilityGrant(t.Context(), "space-a", "device-a", "device-b", "game.execute", 0, true); err != nil {
		t.Fatal(err)
	}
	authorized := call()
	if authorized.Code == http.StatusForbidden {
		t.Fatalf("explicit grant was not honored: %s", authorized.Body.String())
	}
	if authorized.Code != http.StatusServiceUnavailable {
		t.Fatalf("authorized target should reach invocation runtime, status=%d body=%s", authorized.Code, authorized.Body.String())
	}

	withoutGame := make([]string, 0, len(capabilities.IDs()))
	for _, capability := range capabilities.IDs() {
		if capability != "game.execute" {
			withoutGame = append(withoutGame, capability)
		}
	}
	if _, err := coordinationService.SetCapabilityExposure(t.Context(), "space-a", "device-b", 2, withoutGame); err != nil {
		t.Fatal(err)
	}
	revoked := call()
	if revoked.Code != http.StatusForbidden || !bytes.Contains(revoked.Body.Bytes(), []byte("mesh.cross_device_game_forbidden")) {
		t.Fatalf("closed target exposure did not revoke access: status=%d body=%s", revoked.Code, revoked.Body.String())
	}
}

func TestGameCenterBehaviorInvocationPathRequiresLease(t *testing.T) {
	if !gameCenterBehaviorInvocation(http.MethodPost, "/api/game-center/behaviors/invoke") {
		t.Fatal("behavior invoke path was not recognized")
	}
	for _, testCase := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/game-center/behaviors/invoke"},
		{http.MethodPost, "/api/game-center/behaviors"},
		{http.MethodPost, "/api/game-center/runtimes/runtime/start"},
	} {
		if gameCenterBehaviorInvocation(testCase.method, testCase.path) {
			t.Fatalf("unexpected lease-protected path: %s %s", testCase.method, testCase.path)
		}
	}
}
