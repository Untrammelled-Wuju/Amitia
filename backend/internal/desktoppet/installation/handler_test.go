// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	desktoppetAuth "github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/desktoppet/installation/desired"
	"github.com/u-ai/backend/internal/desktoppet/installation/projection"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRegisterRoutesUsesReleaseInstallationSurfaceOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api")
	RegisterRoutes(group, nil, nil, nil)

	routes := map[string]bool{}
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	for _, expected := range []string{
		"POST /api/desktop-pets/pets/:petId/releases/:releaseId/install",
		"GET /api/desktop-pets/installations",
		"POST /api/desktop-pets/installations/:installationId/upgrade",
		"POST /api/desktop-pets/installations/:installationId/downgrade",
		"POST /api/desktop-pets/installations/:installationId/switch",
		"POST /api/desktop-pets/installations/:installationId/repair",
		"GET /api/desktop-pets/operations/:operationId",
	} {
		if !routes[expected] {
			t.Fatalf("missing canonical route %s", expected)
		}
	}

	for _, retired := range []string{
		"POST /api/desktop-pets/packages/:packageId/install",
	} {
		if routes[retired] {
			t.Fatalf("retired route still registered: %s", retired)
		}
	}
}

func TestInstallReleaseRequiresDeviceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("actorContext", &desktoppetAuth.ActorContext{
			PrincipalType: desktoppetAuth.PrincipalLocalUI,
			SpaceID:       "test-user",
			Permissions:   desktoppetAuth.StandardPermissions(),
		})
		c.Next()
	})
	handler := NewCoordinatorHandler(nil, nil, nil)
	router.POST("/api/desktop-pets/pets/:petId/releases/:releaseId/install", handler.InstallRelease)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/desktop-pets/pets/pet-one/releases/release-one/install",
		strings.NewReader(`{}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "DEVICE_ID_REQUIRED") {
		t.Fatalf("missing device error: %s", response.Body.String())
	}
}

func TestInstallationRuntimeViewSeparatesDesiredAndAppliedState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Installation{}, &desired.RuntimeDesiredState{}, &projection.InstallationRuntimeProjection{}); err != nil {
		t.Fatalf("migrate runtime view schema: %v", err)
	}
	if err := db.Create(&desired.RuntimeDesiredState{
		ID:               "desired-one",
		SpaceID:          "space-one",
		DeviceID:         "device-one",
		RuntimeID:        "runtime-one",
		InstallationID:   "installation-one",
		PetID:            "pet-one",
		ReleaseID:        "release-one",
		DesiredEnabled:   true,
		DesiredVisible:   true,
		DesiredActionKey: "wave",
		DesiredRevision:  3,
		UpdatedAt:        "2026-10-23T12:00:00Z",
	}).Error; err != nil {
		t.Fatalf("create desired state: %v", err)
	}
	if err := db.Create(&projection.InstallationRuntimeProjection{
		ID:                      "projection-one",
		SpaceID:                 "space-one",
		DeviceID:                "device-one",
		RuntimeID:               "runtime-one",
		InstallationID:          "installation-one",
		PetID:                   "pet-one",
		AppliedDesiredRevision:  2,
		AppliedSettingsRevision: 4,
		ActualReleaseID:         "release-one",
		ActualVisible:           1,
		ActualActionKey:         "idle_normal",
		ActualHealth:            "healthy",
		RuntimeSyncState:        "pending",
		LastHeartbeatAt:         "2026-10-23T12:00:01Z",
		UpdatedAt:               "2026-10-23T12:00:01Z",
	}).Error; err != nil {
		t.Fatalf("create runtime projection: %v", err)
	}
	handler := NewCoordinatorHandler(nil, NewRepository(db, nil), nil)
	view, err := handler.loadRuntimeView(context.Background(), "space-one", "device-one", "installation-one")
	if err != nil {
		t.Fatalf("load runtime view: %v", err)
	}
	if view == nil || view.DesiredRevision != 3 || view.AppliedDesiredRevision != 2 || !view.Pending || view.DesiredActionKey != "wave" || view.ActualActionKey != "idle_normal" {
		t.Fatalf("runtime view mixed desired and applied state: %+v", view)
	}
	if err := db.Model(&projection.InstallationRuntimeProjection{}).Where("id = ?", "projection-one").Updates(map[string]any{
		"applied_desired_revision": 3,
		"actual_action_key":        "wave",
		"runtime_sync_state":       "applied",
	}).Error; err != nil {
		t.Fatalf("apply runtime projection: %v", err)
	}
	view, err = handler.loadRuntimeView(context.Background(), "space-one", "device-one", "installation-one")
	if err != nil {
		t.Fatalf("reload runtime view: %v", err)
	}
	if view == nil || view.Pending || view.AppliedDesiredRevision != 3 || view.ActualActionKey != "wave" {
		t.Fatalf("applied runtime view is incorrect: %+v", view)
	}
}
