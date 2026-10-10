package system

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

func TestLegacyWebChatRegisteredCoreBoundary(t *testing.T) {
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
		`CREATE TABLE conversations(id TEXT PRIMARY KEY,space_id TEXT,title TEXT,deleted_at DATETIME)`,
		`INSERT INTO conversations VALUES('core-chat','core-space','private Core conversation',NULL)`,
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
	RegisterSystemRouter(router.Group("/api"), &app.AppContext{DB: db}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	RegisterVoiceEntryRouter(router.Group("/api"), db, nil, nil, nil)
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
	isLegacy := func(path string) bool {
		return strings.HasPrefix(path, "/api/web-chat/") || path == "/api/proactive-sse" || path == "/api/voice/upload" || path == "/api/image/upload" || path == "/api/video/upload" || path == "/api/voice/transcribe" || strings.HasPrefix(path, "/api/voice/session") || path == "/api/voice/turn"
	}
	assertBoundary := func(stage, core, stamp string, status int) {
		t.Helper()
		count := 0
		for _, route := range router.Routes() {
			if !isLegacy(route.Path) {
				continue
			}
			count++
			path := route.Path
			for _, parameter := range []string{":id", ":turnId", ":approvalId"} {
				path = strings.ReplaceAll(path, parameter, "existing")
			}
			rec := request(route.Method, path, core, stamp)
			if rec.Code != status {
				t.Fatalf("%s %s %s: %d %s", stage, route.Method, path, rec.Code, rec.Body.String())
			}
		}
		if count != 34 {
			t.Fatalf("legacy route count %d", count)
		}
	}
	assertBoundary("ordinary", "", "", 403)
	if rec := request("GET", "/api/health", "", ""); rec.Code != 503 || !strings.Contains(rec.Body.String(), "服务尚未就绪") {
		t.Fatalf("public health polluted %d %s", rec.Code, rec.Body.String())
	}
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
	for _, scenario := range []struct {
		method, path string
		status       int
		body         string
	}{
		{"GET", "/api/web-chat/conversations/core-chat", 200, "private Core conversation"},
		{"POST", "/api/voice/upload", 200, "缺少音频文件"},
		{"POST", "/api/image/upload", 200, "缺少图片文件"},
		{"POST", "/api/video/upload", 200, "缺少视频文件"},
		{"POST", "/api/voice/transcribe", 200, "缺少audioUrl"},
		{"POST", "/api/voice/session", 200, "invalid request"},
		{"POST", "/api/voice/turn", 200, "invalid request"},
	} {
		rec := request(scenario.method, scenario.path, "core-space", stamp)
		if rec.Code != scenario.status || !strings.Contains(rec.Body.String(), scenario.body) {
			t.Fatalf("admin %+v %d %s", scenario, rec.Code, rec.Body.String())
		}
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
	if rec := request("GET", "/api/web-chat/conversations/core-chat", "", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), "private Core conversation") {
		t.Fatalf("local %d %s", rec.Code, rec.Body.String())
	}
}
