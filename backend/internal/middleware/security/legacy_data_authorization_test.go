package security_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/credential"
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/profile"
	"github.com/u-ai/backend/internal/worldbook"
	"github.com/u-ai/backend/pkg/app"
)

func registerLegacyDataTestRoutes(router *gin.Engine) {
	group := router.Group("/api")
	memory.RegisterMemoryRouter(group, &app.AppContext{}, nil)
	profile.RegisterProfileRouter(group, nil)
	episodic.RegisterEpisodicRouter(group, nil)
	worldbook.RegisterWorldBookRouter(group, nil)
	group.GET("/unrelated", func(c *gin.Context) { c.Status(http.StatusNoContent) })
}

func TestLegacyDataRegisteredRoutesRequireCurrentCoreAdministration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
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
	registerLegacyDataTestRoutes(router)
	request := func(method, path, expectedPolicy string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader("{"))
		req.Header.Set("Authorization", "AmitiaDevice "+raw)
		req.Header.Set("X-Amitia-Device-ID", "device")
		req.Header.Set("Content-Type", "application/json")
		if expectedPolicy != "" {
			req.Header.Set(security.ExpectedCoreHeader, "space")
			req.Header.Set(security.ExpectedConfigurationPolicyHeader, expectedPolicy)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	assertAllDenied := func(stage string) {
		t.Helper()
		for _, route := range router.Routes() {
			if route.Path == "/api/unrelated" {
				continue
			}
			path := strings.ReplaceAll(route.Path, ":id", "existing-resource")
			response := request(route.Method, path, "")
			if response.Code != http.StatusForbidden {
				t.Fatalf("%s %s %s: got %d %s", stage, route.Method, path, response.Code, response.Body.String())
			}
		}
		if response := request(http.MethodGet, "/api/unrelated", ""); response.Code != http.StatusNoContent {
			t.Fatalf("route guard leaked into unrelated API: %d", response.Code)
		}
	}
	assertAllDenied("ordinary")
	policy, err := policies.ChangeMode(t.Context(), "space", "device", 1, true, "role")
	if err != nil {
		t.Fatal(err)
	}
	assertAllDenied("coordinated-without-admin")
	policy, err = policies.GrantAdministrator(t.Context(), "space", "device", policy.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	originalPolicy := fmt.Sprintf("%d:%d:%d", policy.ProviderEpoch, policy.ModeRevision, policy.PermissionRevision)
	for _, path := range []string{"/api/memories", "/api/memory-candidates/generate", "/api/profiles", "/api/episodic", "/api/world-book"} {
		response := request(http.MethodPost, path, originalPolicy)
		var payload map[string]any
		expectedMessage := "unexpected EOF"
		if path == "/api/memory-candidates/generate" {
			expectedMessage = "conversationId不能为空"
		}
		if path == "/api/episodic" {
			expectedMessage = "缺少必要参数"
		}
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &payload) != nil || !strings.Contains(fmt.Sprint(payload["msg"]), expectedMessage) {
			t.Fatalf("valid administrator did not reach actual JSON handler %s: %d %s", path, response.Code, response.Body.String())
		}
	}
	policy, err = policies.GrantAdministrator(t.Context(), "space", "device", policy.PermissionRevision, false)
	if err != nil {
		t.Fatal(err)
	}
	assertAllDenied("revoked")
	policy, err = policies.GrantAdministrator(t.Context(), "space", "device", policy.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/memories", "/api/profiles", "/api/episodic", "/api/world-book"} {
		if response := request(http.MethodPost, path, originalPolicy); response.Code != http.StatusConflict {
			t.Fatalf("regrant revived old intent %s: %d", path, response.Code)
		}
	}
	if _, err := policies.ChangeMode(t.Context(), "space", "device", policy.ModeRevision, false, "source-role"); err != nil {
		t.Fatal(err)
	}
	assertAllDenied("mode-off")
}

func TestLegacyDataRegisteredRoutesPreserveLocalCoreAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, method := range []string{security.AuthMethodLocalToken, security.AuthMethodLocalAdminToken, security.AuthMethodDesktopSession} {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("actorContext", &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: "space", IsLocalTrusted: true, AuthMethod: method, Permissions: auth.OwnerDevicePermissions()})
			c.Next()
		})
		registerLegacyDataTestRoutes(router)
		for _, path := range []string{"/api/memories", "/api/profiles", "/api/episodic", "/api/world-book"} {
			request := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{"))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			expectedMessage := "unexpected EOF"
			if path == "/api/episodic" {
				expectedMessage = "缺少必要参数"
			}
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), expectedMessage) {
				t.Fatalf("canonical administrator rejected %s: %d %s", path, response.Code, response.Body.String())
			}
		}
	}
}
