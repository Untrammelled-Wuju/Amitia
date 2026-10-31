package system

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func newRuntimeModulesTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB.Close()
	})
	originalCfg := config.AppCfg
	config.AppCfg = &config.Config{Security: config.SecurityRuntimeConfig{Mode: "local_single_user"}}
	t.Cleanup(func() { config.AppCfg = originalCfg })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("actorContext", &auth.ActorContext{
			PrincipalType:  auth.PrincipalLocalUI,
			SpaceID:        runtimeidentity.SpaceID(requestidentity.CanonicalSpaceID()),
			IsLocalTrusted: true,
			Permissions:    auth.OwnerDevicePermissions(),
		})
		c.Next()
	})
	group := router.Group("")
	RegisterSystemRouter(group, &app.AppContext{DB: db}, NewService(&app.AppContext{DB: db}, ""), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	return router
}

func TestRuntimeModulesHealthRoute(t *testing.T) {
	router := newRuntimeModulesTestRouter(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/runtime/modules/health", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data = %T", body["data"])
	}
	if _, ok := data["healthy"].(bool); !ok {
		t.Fatalf("healthy = %T", data["healthy"])
	}
	modules, ok := data["modules"].([]interface{})
	if !ok {
		t.Fatalf("modules = %T", data["modules"])
	}
	if len(modules) == 0 {
		t.Fatal("modules is empty")
	}
}

func TestLegacyConfigMigrationRoutesRemoved(t *testing.T) {
	router := newRuntimeModulesTestRouter(t)
	registered := make(map[string]bool)
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = true
	}
	for _, route := range []string{
		http.MethodPost + " /config/export",
		http.MethodPost + " /config/import/preview",
		http.MethodPost + " /config/import/confirm",
	} {
		if registered[route] {
			t.Fatalf("legacy route still registered: %s", route)
		}
	}
	for _, route := range []string{
		http.MethodPost + " /data-portability/exports/preview",
		http.MethodPost + " /data-portability/exports",
		http.MethodPost + " /data-portability/imports/inspect",
	} {
		if !registered[route] {
			t.Fatalf("single data portability route missing: %s", route)
		}
	}
}

func TestRuntimeHealthRouteKeepsResponseShape(t *testing.T) {
	router := newRuntimeModulesTestRouter(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/runtime/health", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data = %T", body["data"])
	}
	if _, ok := data["health"].(bool); !ok {
		t.Fatalf("health = %T", data["health"])
	}
	if _, ok := data["database"].(string); !ok {
		t.Fatalf("database = %T", data["database"])
	}
	if _, ok := data["model"].(string); !ok {
		t.Fatalf("model = %T", data["model"])
	}
	if _, exists := data["modules"]; exists {
		t.Fatal("runtime health response unexpectedly contains modules")
	}
}
