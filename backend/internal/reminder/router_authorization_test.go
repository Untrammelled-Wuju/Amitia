package reminder

import (
	"database/sql"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/credential"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/middleware/security"
)

func TestLegacyReminderRegisteredRoutesRequireCurrentAdministration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if err := sqlite.Migrate(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	registry := host_registry.NewRegistry(db)
	if _, err := registry.EnsureDevice(t.Context(), host_registry.DeviceRecord{SpaceID: "space", DeviceID: "device", TrustState: host_registry.DeviceTrustTrusted}); err != nil {
		t.Fatal(err)
	}
	credentials := credential.NewService(credential.NewRepository(db), 3600)
	_, raw, err := credentials.Exchange(t.Context(), &credential.ExchangeTicketView{SpaceID: "space", DeviceID: "device", RuntimeID: "runtime", Status: "active", ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	policies := coordination.NewService(db)
	router := gin.New()
	router.Use(security.AuthenticationMiddleware(security.AuthConfig{Mode: "cloud_core", SpaceID: "space", DeviceCredentials: credentials, DeviceRegistry: registry, Coordination: policies}))
	registerReminderRoutes(router.Group("/api"), NewHandler(NewService(nil, nil)))
	router.GET("/api/unrelated", func(c *gin.Context) { c.Status(204) })
	request := func(method, path, expected string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader("{"))
		req.Header.Set("Authorization", "AmitiaDevice "+raw)
		req.Header.Set("X-Amitia-Device-ID", "device")
		req.Header.Set("Content-Type", "application/json")
		if expected != "" {
			req.Header.Set(security.ExpectedCoreHeader, "space")
			req.Header.Set(security.ExpectedConfigurationPolicyHeader, expected)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	denyAll := func(stage string) {
		t.Helper()
		for _, route := range router.Routes() {
			if route.Path == "/api/unrelated" {
				continue
			}
			path := strings.ReplaceAll(route.Path, ":id", "existing-resource")
			if response := request(route.Method, path, ""); response.Code != 403 {
				t.Fatalf("%s %s %s: got %d %s", stage, route.Method, path, response.Code, response.Body.String())
			}
		}
		if response := request("GET", "/api/unrelated", ""); response.Code != 204 {
			t.Fatal("guard escaped dedicated route group")
		}
	}
	if response := request("GET", "/api/reminders/prospective", ""); response.Code != 403 {
		t.Fatalf("ordinary device entered Core handler: %d %s", response.Code, response.Body.String())
	}
	denyAll("ordinary paired device sharing Core SpaceID")
	policy, err := policies.ChangeMode(t.Context(), "space", "device", 1, true, "role")
	if err != nil {
		t.Fatal(err)
	}
	denyAll("coordinated without administration")
	policy, err = policies.GrantAdministrator(t.Context(), "space", "device", policy.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	original := fmt.Sprintf("%d:%d:%d", policy.ProviderEpoch, policy.ModeRevision, policy.PermissionRevision)
	for _, route := range []struct{ method, path string }{{"GET", "/api/reminders/prospective"}, {"POST", "/api/reminders"}, {"PUT", "/api/reminders/existing-resource"}} {
		if response := request(route.method, route.path, original); response.Code != 200 || strings.Contains(response.Body.String(), "管理员") {
			t.Fatalf("administrator did not enter handler: %d %s", response.Code, response.Body.String())
		}
	}
	policy, err = policies.GrantAdministrator(t.Context(), "space", "device", policy.PermissionRevision, false)
	if err != nil {
		t.Fatal(err)
	}
	denyAll("revoked")
	policy, err = policies.GrantAdministrator(t.Context(), "space", "device", policy.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range []struct{ method, path string }{{"GET", "/api/reminders/prospective"}, {"POST", "/api/reminders"}, {"PUT", "/api/reminders/existing-resource"}} {
		if response := request(route.method, route.path, original); response.Code != 409 {
			t.Fatalf("regrant revived old policy: %d %s", response.Code, response.Body.String())
		}
	}
	if _, err := policies.ChangeMode(t.Context(), "space", "device", policy.ModeRevision, false, "source-role"); err != nil {
		t.Fatal(err)
	}
	denyAll("mode disabled")
}

func TestLegacyReminderRegisteredRoutesPreserveCoreOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, principal := range []auth.PrincipalType{auth.PrincipalLocalUI, auth.PrincipalTrustedDevice} {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("actorContext", &auth.ActorContext{PrincipalType: principal, SpaceID: "space", IsLocalTrusted: principal == auth.PrincipalLocalUI, Permissions: auth.OwnerDevicePermissions()})
			c.Next()
		})
		registerReminderRoutes(router.Group("/api"), NewHandler(NewService(nil, nil)))
		for _, route := range []struct{ method, path string }{{"GET", "/api/reminders/prospective"}, {"POST", "/api/reminders"}, {"PUT", "/api/reminders/existing-resource"}} {
			req := httptest.NewRequest(route.method, route.path, strings.NewReader("{"))
			req.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != 200 || strings.Contains(response.Body.String(), "管理员") {
				t.Fatalf("Core owner rejected: %d %s", response.Code, response.Body.String())
			}
		}
	}
}
