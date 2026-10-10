package sync

import (
	"context"
	"encoding/json"
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
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLegacySyncRemoteAuthorityAndLocalContinuity(t *testing.T) {
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
		`CREATE TABLE sync_changes(change_id TEXT PRIMARY KEY,seq INTEGER,space_id TEXT,scope TEXT,entity_type TEXT,entity_id TEXT,operation TEXT,revision INTEGER,mutation_id TEXT,origin_device TEXT,payload BLOB,checksum TEXT,created_at DATETIME)`,
		`CREATE TABLE sync_cursors(device_id TEXT,space_id TEXT,scope TEXT,last_applied INTEGER,last_pushed INTEGER,updated_at DATETIME,PRIMARY KEY(space_id,scope,device_id))`,
		`CREATE TABLE sync_sequence(id INTEGER PRIMARY KEY,seq INTEGER)`,
		`CREATE TABLE sync_mutation_claims(space_id TEXT,scope TEXT,mutation_id TEXT,status TEXT,created_at DATETIME,PRIMARY KEY(space_id,scope,mutation_id))`,
		`CREATE TABLE conversations(id TEXT PRIMARY KEY,space_id TEXT,title TEXT,revision INTEGER,updated_at DATETIME,deleted_at DATETIME)`,
		`INSERT INTO conversations(id,space_id,title,revision) VALUES('core-chat','core-space','original',1)`,
	} {
		if err := db.Exec(query).Error; err != nil {
			t.Fatal(err)
		}
	}
	registry := host_registry.NewRegistry(raw)
	for _, device := range []host_registry.DeviceRecord{
		{SpaceID: "core-space", DeviceID: "device", TrustState: host_registry.DeviceTrustTrusted},
		{SpaceID: "core-space", DeviceID: "victim", TrustState: host_registry.DeviceTrustTrusted},
	} {
		if _, err := registry.EnsureDevice(t.Context(), device); err != nil {
			t.Fatal(err)
		}
	}
	credentials := credential.NewService(credential.NewRepository(raw), 3600)
	_, token, err := credentials.Exchange(t.Context(), &credential.ExchangeTicketView{SpaceID: "core-space", DeviceID: "device", RuntimeID: "runtime", Status: "active", ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	policies := coordination.NewService(raw)
	service := NewService(db, NewBusinessApplier(db))
	if _, err := service.ChangeLog.Append(EntityTypeMessage, "core-message", OpCreate, 1, "seed", "core-device", "core-space", ScopeDevice, []byte(`{"content":"private Core chat"}`)); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(service, registry, policies)
	remote := gin.New()
	handler.RegisterRoutes(remote.Group("/api"), security.AuthenticationMiddleware(security.AuthConfig{Mode: "cloud_core", SpaceID: "core-space", DeviceCredentials: credentials, DeviceRegistry: registry, Coordination: policies}))
	local := gin.New()
	handler.RegisterRoutes(local.Group("/api"), func(c *gin.Context) {
		c.Set("actorContext", &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: "core-space", DeviceID: "device", IsLocalTrusted: true, Permissions: auth.OwnerDevicePermissions()})
	})
	request := func(router *gin.Engine, method, path, body, core, policy string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "AmitiaDevice "+token)
		if core != "" {
			req.Header.Set(security.ExpectedCoreHeader, core)
		}
		if policy != "" {
			req.Header.Set(security.ExpectedConfigurationPolicyHeader, policy)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	payload, _ := json.Marshal([]byte(`{"title":"local update"}`))
	pushBody := `{"deviceId":"victim","mutations":[{"mutationId":"mutation","entityType":"conversation","entityId":"core-chat","operation":"update","baseRevision":1,"payload":` + string(payload) + `}]}`
	blocked := func(stage string) {
		t.Helper()
		for _, route := range []struct{ method, path, body string }{
			{"POST", "/api/v1/sync/pull", `{"deviceId":"device"}`},
			{"POST", "/api/v1/sync/pull", `{"deviceId":"victim","scope":"global"}`},
			{"POST", "/api/v1/sync/push", pushBody},
			{"POST", "/api/v1/sync/ack", `{"deviceId":"victim","lastApplied":999}`},
			{"GET", "/api/v1/sync/gap?cursor=999", ""},
		} {
			rec := request(remote, route.method, route.path, route.body, "", "")
			if rec.Code != 403 || !strings.Contains(rec.Body.String(), "legacy_sync_remote_disabled") {
				t.Fatalf("%s %s %d %s", stage, route.path, rec.Code, rec.Body.String())
			}
		}
		var title string
		db.Table("conversations").Select("title").Scan(&title)
		if title != "original" {
			t.Fatal("denied remote mutation changed Core chat")
		}
		var count int64
		db.Model(&SyncCursor{}).Count(&count)
		if count != 0 {
			t.Fatal("denied remote request changed cursors")
		}
	}
	blocked("ordinary")
	if rec := request(remote, "GET", "/api/v1/sync/status?deviceId=device", "", "", ""); rec.Code != 200 {
		t.Fatalf("self status %d %s", rec.Code, rec.Body.String())
	}
	if rec := request(remote, "GET", "/api/v1/sync/status?deviceId=victim", "", "", ""); rec.Code != 403 {
		t.Fatalf("ordinary cross-device %d", rec.Code)
	}
	policy, err := policies.ChangeMode(t.Context(), "core-space", "device", 1, true, "role")
	if err != nil {
		t.Fatal(err)
	}
	policy, err = policies.GrantAdministrator(t.Context(), "core-space", "device", policy.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	stamp := func(policy coordination.Policy) string {
		return fmt.Sprintf("%d:%d:%d", policy.ProviderEpoch, policy.ModeRevision, policy.PermissionRevision)
	}
	original := stamp(policy)
	blocked("administrator")
	for _, scenario := range []struct {
		target, core, stamp string
		status              int
	}{
		{"victim", "core-space", original, 200},
		{"device", "core-space", original, 200},
		{"victim", "", "", 409},
		{"victim", "old-core", original, 409},
		{"device", "old-core", original, 409},
	} {
		rec := request(remote, "GET", "/api/v1/sync/status?deviceId="+scenario.target, "", scenario.core, scenario.stamp)
		if rec.Code != scenario.status {
			t.Fatalf("intent %+v: %d %s", scenario, rec.Code, rec.Body.String())
		}
	}
	policy, err = policies.GrantAdministrator(t.Context(), "core-space", "device", policy.PermissionRevision, false)
	if err != nil {
		t.Fatal(err)
	}
	policy, err = policies.GrantAdministrator(t.Context(), "core-space", "device", policy.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"device", "victim"} {
		rec := request(remote, "GET", "/api/v1/sync/status?deviceId="+target, "", "core-space", original)
		if rec.Code != 409 {
			t.Fatalf("ABA %s %d %s", target, rec.Code, rec.Body.String())
		}
	}
	if rec := request(remote, "GET", "/api/v1/sync/status?deviceId=victim", "", "core-space", stamp(policy)); rec.Code != 200 {
		t.Fatalf("fresh admin %d", rec.Code)
	}
	for _, route := range []struct{ method, path, body string }{
		{"POST", "/api/v1/sync/pull", `{"deviceId":"victim"}`},
		{"POST", "/api/v1/sync/push", pushBody},
		{"POST", "/api/v1/sync/ack", `{"deviceId":"victim","lastApplied":2}`},
		{"GET", "/api/v1/sync/status?deviceId=victim", ""},
		{"GET", "/api/v1/sync/gap?cursor=999", ""},
	} {
		rec := request(local, route.method, route.path, route.body, "", "")
		if rec.Code != 200 {
			t.Fatalf("local %s %d %s", route.path, rec.Code, rec.Body.String())
		}
	}
	var title string
	db.Table("conversations").Select("title").Scan(&title)
	if title != "local update" {
		t.Fatal("local Core mutation no longer works")
	}
	if rec := request(local, "GET", "/api/v1/sync/status?deviceId=victim", "", "old-core", stamp(policy)); rec.Code != 409 {
		t.Fatalf("local old Core intent %d %s", rec.Code, rec.Body.String())
	}
	late := gin.New()
	late.GET("/status", security.AuthenticationMiddleware(security.AuthConfig{Mode: "cloud_core", SpaceID: "core-space", DeviceCredentials: credentials, DeviceRegistry: registry, Coordination: policies}), handler.statusAuthority, func(c *gin.Context) {
		policy, err = policies.GrantAdministrator(c.Request.Context(), "core-space", "device", policy.PermissionRevision, false)
		if err != nil {
			t.Fatal(err)
		}
		c.Next()
	}, handler.HandleStatus)
	if rec := request(late, "GET", "/status?deviceId=victim", "", "core-space", stamp(policy)); rec.Code != 409 || strings.Contains(rec.Body.String(), "lastApplied") {
		t.Fatalf("revoked during status %d %s", rec.Code, rec.Body.String())
	}
	policy, err = policies.GrantAdministrator(t.Context(), "core-space", "device", policy.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	revokeAfterRead := true
	if err := db.Callback().Query().After("gorm:query").Register("sync_test_revoke_after_read", func(tx *gorm.DB) {
		if revokeAfterRead && tx.Statement.Table == "sync_cursors" {
			revokeAfterRead = false
			policy, err = policies.GrantAdministrator(context.Background(), "core-space", "device", policy.PermissionRevision, false)
			if err != nil {
				t.Fatal(err)
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	if rec := request(remote, "GET", "/api/v1/sync/status?deviceId=victim", "", "core-space", stamp(policy)); rec.Code != 409 || strings.Contains(rec.Body.String(), "lastApplied") {
		t.Fatalf("revoked after status read %d %s", rec.Code, rec.Body.String())
	}
	if revokeAfterRead {
		t.Fatal("status read revocation hook was not exercised")
	}
}
