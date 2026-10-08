package security_test

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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

func TestPairedDeviceRequiresExplicitCurrentCoreAdministration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
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
	router.GET("/configuration", security.SharedCoreAdminOnly(), func(c *gin.Context) { c.JSON(200, gin.H{"dataSource": "same-core-data"}) })
	expectedPolicy := ""
	configurationPath := "/configuration"
	request := func(identity string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, configurationPath, nil)
		req.Header.Set("Authorization", "AmitiaDevice "+raw)
		if expectedPolicy != "" {
			req.Header.Set(security.ExpectedCoreHeader, "space")
			req.Header.Set(security.ExpectedConfigurationPolicyHeader, expectedPolicy)
		}
		if identity != "" {
			req.Header.Set("X-Amitia-Device-ID", identity)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		return recorder
	}
	if response := request("device"); response.Code != 403 {
		t.Fatalf("pairing granted administration: %d %s", response.Code, response.Body.String())
	}
	p, err := policies.ChangeMode(t.Context(), "space", "device", 1, true, "role")
	if err != nil {
		t.Fatal(err)
	}
	if response := request("device"); response.Code != 403 {
		t.Fatalf("coordination alone granted administration: %d", response.Code)
	}
	p, err = policies.GrantAdministrator(t.Context(), "space", "device", p.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	if response := request("device"); response.Code != 200 || response.Body.String() != `{"dataSource":"same-core-data"}` {
		t.Fatalf("administrator not using canonical data: %d %s", response.Code, response.Body.String())
	}
	if response := request("other-device"); response.Body.String() == `{"dataSource":"same-core-data"}` {
		t.Fatal("spoofed identity authorized")
	}
	expectedPolicy = fmt.Sprintf("%d:%d:%d", p.ProviderEpoch, p.ModeRevision, p.PermissionRevision)
	if response := request("device"); response.Code != 200 {
		t.Fatalf("authenticated current policy was rejected: %d", response.Code)
	}
	p, err = policies.GrantAdministrator(t.Context(), "space", "device", p.PermissionRevision, false)
	if err != nil {
		t.Fatal(err)
	}
	if response := request("device"); response.Code != 403 {
		t.Fatal("revoked configuration intent retained access")
	}
	p, err = policies.GrantAdministrator(t.Context(), "space", "device", p.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	if response := request("device"); response.Code != 409 {
		t.Fatalf("administrator regrant revived old configuration intent: %d", response.Code)
	}
	expectedPolicy = fmt.Sprintf("%d:%d:%d", p.ProviderEpoch, p.ModeRevision, p.PermissionRevision)
	if response := request("device"); response.Code != 200 {
		t.Fatal("refreshed administrator configuration intent was rejected")
	}
	lateWrites := 0
	router.GET("/configuration/long", security.SharedCoreAdminOnly(), func(c *gin.Context) {
		if _, ok := coordination.FromContext(c.Request.Context()); !ok {
			t.Error("configuration lacks current authority scope")
		}
		p, err = policies.GrantAdministrator(t.Context(), "space", "device", p.PermissionRevision, false)
		if err != nil {
			t.Fatal(err)
		}
		if coordination.ValidateCurrent(c.Request.Context()) != nil && c.Request.Context().Err() != nil {
			c.Status(409)
			return
		}
		lateWrites++
		c.Status(200)
	})
	configurationPath = "/configuration/long"
	if response := request("device"); response.Code != 409 || lateWrites != 0 {
		t.Fatal("revocation did not cancel in-flight configuration")
	}
	configurationPath = "/configuration"
	p, err = policies.GrantAdministrator(t.Context(), "space", "device", p.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	expectedPolicy = ""
	p, err = policies.ChangeMode(t.Context(), "space", "device", p.ModeRevision, false, "device-role")
	if err != nil {
		t.Fatal(err)
	}
	if response := request("device"); response.Code != 403 {
		t.Fatalf("mode off retained administration: %d", response.Code)
	}
	if _, err := policies.ChangeMode(t.Context(), "space", "device", p.ModeRevision, true, "role"); err != nil {
		t.Fatal(err)
	}
	if response := request("device"); response.Code != 403 {
		t.Fatal("mode on restored old administrator")
	}
}

func TestAdministrationCannotBypassAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, actor := range []*auth.ActorContext{nil, {PrincipalType: auth.PrincipalTrustedDevice, Permissions: auth.StandardPermissions()}, {PrincipalType: auth.PrincipalLocalUI, IsLocalTrusted: true, Permissions: auth.StandardPermissions()}} {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			if actor != nil {
				c.Set("actorContext", actor)
			}
			c.Next()
		})
		router.GET("/config", security.SharedCoreAdminOnly(), func(c *gin.Context) { c.Status(200) })
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/config", nil))
		if response.Code != 403 {
			t.Fatalf("actor without administrator permission was allowed: %+v %d", actor, response.Code)
		}
	}
}
