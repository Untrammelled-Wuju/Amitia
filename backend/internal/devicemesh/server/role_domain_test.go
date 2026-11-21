package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	meshaudit "github.com/u-ai/backend/internal/devicemesh/audit"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
)

func newRoleDomainRouter(t *testing.T) (*gin.Engine, *coordination.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, schema := range []string{
		coordination.PolicySchema,
		coordination.CapabilityGrantsSchema,
		coordination.CapabilityExposureSchema,
		coordination.CapabilityExposureStateSchema,
		coordination.ProviderSchema,
		coordination.RemoteAuthoritySchema,
		coordination.CancelledAuthoritySchema,
		coordination.SourceAuthoritySchema,
		meshaudit.Schema,
		`CREATE TABLE kernel_devices(device_id TEXT PRIMARY KEY,space_id TEXT NOT NULL,platform TEXT NOT NULL DEFAULT '',label TEXT NOT NULL DEFAULT '',reported_name TEXT NOT NULL DEFAULT '',pending_name TEXT NOT NULL DEFAULT '',name_revision INTEGER NOT NULL DEFAULT 0,trust_state TEXT NOT NULL DEFAULT 'pending',created_at TEXT NOT NULL,trusted_at TEXT,last_seen_at TEXT NOT NULL,revision INTEGER NOT NULL DEFAULT 1)`,
	} {
		if _, err := db.Exec(schema); err != nil {
			t.Fatal(err)
		}
	}
	for _, device := range []string{"phone", "tablet"} {
		if _, err := db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES(?,'space','trusted','now','now')`, device); err != nil {
			t.Fatal(err)
		}
	}
	policies := coordination.NewService(db)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		actor := &auth.ActorContext{
			PrincipalType: auth.PrincipalTrustedDevice,
			SpaceID:       "space",
			DeviceID:      "phone",
			Permissions:   auth.OwnerDevicePermissions(),
		}
		c.Set("actorContext", actor)
		c.Request = c.Request.WithContext(auth.WithActor(c.Request.Context(), actor))
	})
	deps := &RouterDeps{Coordination: policies, DeviceReg: host_registry.NewRegistry(db)}
	router.PUT("/devices/:deviceId/role", makeRoleDomainHandler(deps))
	return router, policies
}

func TestRoleDomainHandlerPersistsOwnDeviceSelection(t *testing.T) {
	router, policies := newRoleDomainRouter(t)
	body := `{"domain":"center","role":"core-role","expectedRevision":1}`
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/devices/phone/role", strings.NewReader(body)))
	if response.Code != http.StatusOK {
		t.Fatalf("%d %s", response.Code, response.Body.String())
	}
	var result struct {
		Policy coordination.Policy `json:"policy"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Policy.Coordinated || result.Policy.CenterRole != "core-role" || result.Policy.SelectedRole != "core-role" {
		t.Fatalf("policy: %+v", result.Policy)
	}
	stored, err := policies.Get(t.Context(), "space", "phone")
	if err != nil {
		t.Fatal(err)
	}
	if stored.CenterRole != "core-role" || !stored.Coordinated {
		t.Fatalf("stored: %+v", stored)
	}
}

func TestRoleDomainHandlerRejectsForeignDeviceAndStaleRevision(t *testing.T) {
	router, _ := newRoleDomainRouter(t)
	foreign := httptest.NewRecorder()
	router.ServeHTTP(foreign, httptest.NewRequest(http.MethodPut, "/devices/tablet/role", strings.NewReader(`{"domain":"local","role":"role","expectedRevision":1}`)))
	if foreign.Code != http.StatusForbidden && foreign.Code != http.StatusConflict {
		t.Fatalf("foreign device accepted: %d %s", foreign.Code, foreign.Body.String())
	}
	stale := httptest.NewRecorder()
	router.ServeHTTP(stale, httptest.NewRequest(http.MethodPut, "/devices/phone/role", strings.NewReader(`{"domain":"local","role":"role","expectedRevision":9}`)))
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale revision accepted: %d %s", stale.Code, stale.Body.String())
	}
	invalid := httptest.NewRecorder()
	router.ServeHTTP(invalid, httptest.NewRequest(http.MethodPut, "/devices/phone/role", strings.NewReader(`{"domain":"remote","role":"role","expectedRevision":1}`)))
	if invalid.Code != http.StatusConflict {
		t.Fatalf("invalid domain accepted: %d %s", invalid.Code, invalid.Body.String())
	}
}
