package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/gamemode"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"gorm.io/gorm"
)

func newGameModeRouterTestService(t *testing.T) *gamemode.Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "game-mode-router.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := gamemode.NewService(db)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func newGameModeTestRouter(service *gamemode.Service, actor *auth.ActorContext) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if actor != nil {
			c.Set("actorContext", actor)
		}
		c.Next()
	})
	registerGameModeRoutes(router.Group("/api"), &AppServices{GameMode: service})
	return router
}

func performGameModeRequest(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestGameModeRouterFencesSecondDeviceAndStaleRevision(t *testing.T) {
	service := newGameModeRouterTestService(t)
	deviceA := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "space-a", DeviceID: "device-a", RequestID: "request-a"}
	routerA := newGameModeTestRouter(service, deviceA)

	initial := performGameModeRequest(routerA, http.MethodGet, "/api/game-mode", "")
	if initial.Code != http.StatusOK {
		t.Fatalf("initial status=%d body=%s", initial.Code, initial.Body.String())
	}
	acquired := performGameModeRequest(routerA, http.MethodPost, "/api/game-mode/acquire", `{"gamePluginId":"game-a","gameTitle":"Game A"}`)
	if acquired.Code != http.StatusOK {
		t.Fatalf("acquire status=%d body=%s", acquired.Code, acquired.Body.String())
	}
	var acquireBody struct {
		Data gamemode.Session `json:"data"`
	}
	if err := json.Unmarshal(acquired.Body.Bytes(), &acquireBody); err != nil {
		t.Fatal(err)
	}
	if acquireBody.Data.DeviceID != "device-a" || acquireBody.Data.State != gamemode.StateActive {
		t.Fatalf("unexpected acquire result: %+v", acquireBody.Data)
	}

	deviceB := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "space-a", DeviceID: "device-b", RequestID: "request-b"}
	routerB := newGameModeTestRouter(service, deviceB)
	visible := performGameModeRequest(routerB, http.MethodGet, "/api/game-mode", "")
	if visible.Code != http.StatusOK || !bytes.Contains(visible.Body.Bytes(), []byte(`"owned":false`)) || !bytes.Contains(visible.Body.Bytes(), []byte(`"deviceId":"device-a"`)) {
		t.Fatalf("device B cannot see owner correctly: status=%d body=%s", visible.Code, visible.Body.String())
	}
	blocked := performGameModeRequest(routerB, http.MethodPost, "/api/game-mode/acquire", `{"gamePluginId":"game-b"}`)
	if blocked.Code != http.StatusConflict || !bytes.Contains(blocked.Body.Bytes(), []byte("occupied")) {
		t.Fatalf("device B acquired occupied mode: status=%d body=%s", blocked.Code, blocked.Body.String())
	}

	stale := performGameModeRequest(routerA, http.MethodPost, "/api/game-mode/update", `{"sessionId":"`+acquireBody.Data.SessionID+`","state":"paused","expectedRevision":0}`)
	if stale.Code != http.StatusOK {
		t.Fatalf("first update status=%d body=%s", stale.Code, stale.Body.String())
	}
	stale = performGameModeRequest(routerA, http.MethodPost, "/api/game-mode/update", `{"sessionId":"`+acquireBody.Data.SessionID+`","state":"active","expectedRevision":1}`)
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale update status=%d body=%s", stale.Code, stale.Body.String())
	}
}

func TestGameModeRouterForceReleaseRequiresAdministrator(t *testing.T) {
	service := newGameModeRouterTestService(t)
	deviceA := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "space-a", DeviceID: "device-a"}
	routerA := newGameModeTestRouter(service, deviceA)
	acquired := performGameModeRequest(routerA, http.MethodPost, "/api/game-mode/acquire", `{}`)
	var acquireBody struct {
		Data gamemode.Session `json:"data"`
	}
	if err := json.Unmarshal(acquired.Body.Bytes(), &acquireBody); err != nil {
		t.Fatal(err)
	}

	deviceB := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "space-a", DeviceID: "device-b"}
	routerB := newGameModeTestRouter(service, deviceB)
	forbidden := performGameModeRequest(routerB, http.MethodPost, "/api/game-mode/release", `{"sessionId":"`+acquireBody.Data.SessionID+`","expectedRevision":1,"force":true}`)
	if forbidden.Code != http.StatusConflict {
		t.Fatalf("non-admin force release status=%d body=%s", forbidden.Code, forbidden.Body.String())
	}

	adminB := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "space-a", DeviceID: "device-b", Permissions: []string{auth.PermSystemAdmin}}
	adminRouter := newGameModeTestRouter(service, adminB)
	released := performGameModeRequest(adminRouter, http.MethodPost, "/api/game-mode/release", `{"sessionId":"`+acquireBody.Data.SessionID+`","expectedRevision":1,"force":true}`)
	if released.Code != http.StatusOK || !bytes.Contains(released.Body.Bytes(), []byte(`"state":"inactive"`)) {
		t.Fatalf("admin force release status=%d body=%s", released.Code, released.Body.String())
	}
}

func TestGameModeRouterOwnerReleaseSurvivesRecoveryRevisionAdvance(t *testing.T) {
	service := newGameModeRouterTestService(t)
	deviceA := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "space-a", DeviceID: "device-a"}
	routerA := newGameModeTestRouter(service, deviceA)
	acquired := performGameModeRequest(routerA, http.MethodPost, "/api/game-mode/acquire", `{"gamePluginId":"game-a"}`)
	if acquired.Code != http.StatusOK {
		t.Fatalf("acquire status=%d body=%s", acquired.Code, acquired.Body.String())
	}
	var acquireBody struct {
		Data gamemode.Session `json:"data"`
	}
	if err := json.Unmarshal(acquired.Body.Bytes(), &acquireBody); err != nil {
		t.Fatal(err)
	}
	if _, err := service.MarkUnknown(t.Context(), "space-a", "device-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.MarkReady(t.Context(), "space-a", "device-a"); err != nil {
		t.Fatal(err)
	}
	released := performGameModeRequest(routerA, http.MethodPost, "/api/game-mode/release", `{"sessionId":"`+acquireBody.Data.SessionID+`","expectedRevision":`+jsonNumber(acquireBody.Data.Revision)+`}`)
	if released.Code != http.StatusOK || !bytes.Contains(released.Body.Bytes(), []byte(`"state":"inactive"`)) {
		t.Fatalf("release recovered session status=%d body=%s", released.Code, released.Body.String())
	}
}

func TestReconcileGameModePresenceMarksExpiredOwnerUnknown(t *testing.T) {
	service := newGameModeRouterTestService(t)
	acquired, err := service.Acquire(t.Context(), gamemode.AcquireInput{
		SpaceID:  "space-a",
		DeviceID: "device-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close sqlite: %v", err)
		}
	})
	if err := kernelsqlite.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	registry := host_registry.NewRegistry(db)
	now := time.Now().UTC()
	if err := registry.RegisterEntry(t.Context(), &host_registry.RuntimeEntry{
		EntryID:         "runtime-entry",
		Kind:            host_registry.RegistryEntryKindRuntime,
		SpaceID:         runtimeidentity.ParseSpaceID("space-a"),
		DeviceID:        runtimeidentity.ParseDeviceID("device-a"),
		RuntimeID:       runtimeidentity.ParseRuntimeID("runtime-a"),
		Platform:        runtimeidentity.Platform("android"),
		AuthenticatedAt: now.Add(-5 * time.Minute),
		LastHeartbeat:   now.Add(-91 * time.Second),
		PresenceState:   host_registry.PresenceStateReady,
		CreatedAt:       now.Add(-5 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	services := &AppServices{
		GameMode: service,
		KernelContainer: &kernel.Container{
			DeviceRegistry: registry,
		},
	}
	if err := reconcileGameModePresence(t.Context(), services, runtimeidentity.ParseSpaceID("space-a")); err != nil {
		t.Fatal(err)
	}
	current, err := service.Get(t.Context(), "space-a")
	if err != nil {
		t.Fatal(err)
	}
	if current.State != gamemode.StateUnknown || current.Revision != acquired.Revision+1 {
		t.Fatalf("stale game mode owner was not reconciled: %+v", current)
	}
}

func TestGameModeRouterRejectsMissingIdentityAndInvalidBody(t *testing.T) {
	service := newGameModeRouterTestService(t)
	unauthorized := performGameModeRequest(newGameModeTestRouter(service, nil), http.MethodGet, "/api/game-mode", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("missing identity status=%d body=%s", unauthorized.Code, unauthorized.Body.String())
	}
	actor := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "space-a", DeviceID: "device-a"}
	invalid := performGameModeRequest(newGameModeTestRouter(service, actor), http.MethodPost, "/api/game-mode/acquire", "{")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid body status=%d body=%s", invalid.Code, invalid.Body.String())
	}
}

func jsonNumber(value int64) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
