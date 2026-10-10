package feedback

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/credential"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLegacyFeedbackRegisteredCoreBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	raw.SetMaxOpenConns(1)
	t.Cleanup(func() { raw.Close() })
	if err := kernelsqlite.Migrate(t.Context(), raw); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`CREATE TABLE conversations(id TEXT PRIMARY KEY,space_id TEXT,deleted_at DATETIME)`,
		`CREATE TABLE messages(id TEXT PRIMARY KEY,conversation_id TEXT,role TEXT,deleted_at DATETIME)`,
		`CREATE TABLE message_feedback(id INTEGER PRIMARY KEY,message_id TEXT,feedback_type TEXT,reason TEXT,created_at TEXT)`,
		`INSERT INTO conversations VALUES('core-chat','core-space',NULL)`,
		`INSERT INTO messages VALUES('core-message','core-chat','assistant',NULL)`,
		`INSERT INTO message_feedback VALUES(1,'core-message','good','private Core feedback','2026-10-10')`,
	} {
		if err := db.Exec(query).Error; err != nil {
			t.Fatal(err)
		}
	}
	registry := host_registry.NewRegistry(raw)
	if _, err := registry.EnsureDevice(t.Context(), host_registry.DeviceRecord{SpaceID: "core-space", DeviceID: "device", TrustState: host_registry.DeviceTrustTrusted}); err != nil {
		t.Fatal(err)
	}
	credentials := credential.NewService(credential.NewRepository(raw), 3600)
	_, token, err := credentials.Exchange(t.Context(), &credential.ExchangeTicketView{SpaceID: "core-space", DeviceID: "device", RuntimeID: "runtime", Status: "active", ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	policies := coordination.NewService(raw)
	authenticate := security.AuthenticationMiddleware(security.AuthConfig{Mode: "cloud_core", SpaceID: "core-space", DeviceCredentials: credentials, DeviceRegistry: registry, Coordination: policies})
	local := false
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if local {
			c.Set("actorContext", &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: "core-space", DeviceID: "device", IsLocalTrusted: true, Permissions: auth.OwnerDevicePermissions()})
			c.Set("spaceId", "core-space")
			c.Next()
		} else {
			authenticate(c)
		}
	})
	RegisterFeedbackRouter(router.Group("/api"), &app.AppContext{DB: db})
	router.GET("/api/device-mesh/v1/business/boundary-probe", func(c *gin.Context) { c.Status(204) })
	request := func(method, path, core, stamp string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader("{"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "AmitiaDevice "+token)
		if core != "" {
			req.Header.Set(security.ExpectedCoreHeader, core)
		}
		if stamp != "" {
			req.Header.Set(security.ExpectedConfigurationPolicyHeader, stamp)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	assertBoundary := func(stage, core, stamp string, status int) {
		t.Helper()
		count := 0
		for _, route := range router.Routes() {
			if route.Path == "/api/device-mesh/v1/business/boundary-probe" {
				continue
			}
			count++
			path := strings.ReplaceAll(route.Path, ":id", "existing")
			rec := request(route.Method, path, core, stamp)
			if rec.Code != status {
				t.Fatalf("%s %s %s %d %s", stage, route.Method, path, rec.Code, rec.Body.String())
			}
		}
		if count != 5 {
			t.Fatalf("route count %d", count)
		}
	}
	assertBoundary("ordinary", "", "", 403)
	if rec := request("GET", "/api/device-mesh/v1/business/boundary-probe", "", ""); rec.Code != 204 {
		t.Fatalf("owned group polluted %d", rec.Code)
	}
	policy, err := policies.ChangeMode(t.Context(), "core-space", "device", 1, true, "role")
	if err != nil {
		t.Fatal(err)
	}
	assertBoundary("coordinated-without-admin", "", "", 403)
	policy, err = policies.GrantAdministrator(t.Context(), "core-space", "device", policy.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	stamp := fmt.Sprintf("%d:%d:%d", policy.ProviderEpoch, policy.ModeRevision, policy.PermissionRevision)
	assertBoundary("old-Core", "old-core", stamp, 409)
	if rec := request("GET", "/api/messages/core-message/feedback", "core-space", stamp); rec.Code != 200 || !strings.Contains(rec.Body.String(), "private Core feedback") {
		t.Fatalf("admin %d %s", rec.Code, rec.Body.String())
	}
	if rec := request("POST", "/api/messages/core-message/feedback", "core-space", stamp); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"code":400`) {
		t.Fatalf("admin original Create handler %d %s", rec.Code, rec.Body.String())
	}
	policy, err = policies.GrantAdministrator(t.Context(), "core-space", "device", policy.PermissionRevision, false)
	if err != nil {
		t.Fatal(err)
	}
	assertBoundary("revoked-admin", "core-space", stamp, 403)
	policy, err = policies.GrantAdministrator(t.Context(), "core-space", "device", policy.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	assertBoundary("regranted-old-intent", "core-space", stamp, 409)
	local = true
	if rec := request("GET", "/api/messages/core-message/feedback", "", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), "private Core feedback") {
		t.Fatalf("local %d %s", rec.Code, rec.Body.String())
	}
}
