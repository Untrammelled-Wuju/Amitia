package security

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	meshaudit "github.com/u-ai/backend/internal/devicemesh/audit"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestDeviceManagementIntentOriginalPolicyAndOrdinarySelfGrant(t *testing.T) {
	for _, scenario := range []struct {
		name, core, policy string
		snapshot, changed  bool
		status             int
	}{
		{"ordinary_self_grant", "space", "1:1:1", true, false, 200},
		{"missing_headers", "", "", true, false, 409},
		{"foreign_core", "other", "1:1:1", true, false, 409},
		{"old_permission", "space", "1:1:1", true, true, 409},
		{"missing_snapshot", "space", "1:1:1", false, false, 409},
		{"noncanonical", "space", "01:1:1", true, false, 409},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db"))
			if err != nil {
				t.Fatal(err)
			}
			db.SetMaxOpenConns(1)
			t.Cleanup(func() { db.Close() })
			for _, schema := range []string{coordination.PolicySchema, coordination.CapabilityGrantsSchema, coordination.ProviderSchema, coordination.RemoteAuthoritySchema, coordination.CancelledAuthoritySchema, coordination.SourceAuthoritySchema, meshaudit.Schema, `CREATE TABLE kernel_devices(device_id TEXT PRIMARY KEY,space_id TEXT,trust_state TEXT,created_at TEXT,last_seen_at TEXT)`} {
				if _, err := db.Exec(schema); err != nil {
					t.Fatal(err)
				}
			}
			for _, id := range []string{"a", "b"} {
				if _, err := db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES(?,'space','trusted','now','now')`, id); err != nil {
					t.Fatal(err)
				}
			}
			svc := coordination.NewService(db)
			original, err := svc.Get(t.Context(), "space", "a")
			if err != nil {
				t.Fatal(err)
			}
			if scenario.changed {
				if _, err := svc.ChangeMode(t.Context(), "space", "a", 1, true, "role"); err != nil {
					t.Fatal(err)
				}
				if _, err := svc.ChangeMode(t.Context(), "space", "a", 2, false, "role"); err != nil {
					t.Fatal(err)
				}
			}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("actorContext", &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "space", DeviceID: "a", Permissions: auth.StandardPermissions(), RequestID: "intent"})
				if scenario.snapshot {
					c.Set(configurationPolicyContextKey, original)
				}
			})
			writes := 0
			router.PUT("/grant", func(c *gin.Context) {
				finish, valid := BeginDeviceManagementIntent(c, svc)
				if !valid {
					return
				}
				defer finish()
				grant, err := svc.SetCapabilityGrant(c.Request.Context(), "space", "b", "a", "ai.chat", 0, true)
				if err != nil {
					c.Status(409)
					return
				}
				if grant.Revision != 1 || !grant.Allowed {
					t.Fatal("invalid grant acknowledgement")
				}
				writes++
				c.Status(200)
			})
			req := httptest.NewRequest(http.MethodPut, "/grant", nil)
			req.Header.Set(ExpectedCoreHeader, scenario.core)
			req.Header.Set(ExpectedConfigurationPolicyHeader, scenario.policy)
			out := httptest.NewRecorder()
			router.ServeHTTP(out, req)
			if out.Code != scenario.status || writes != map[bool]int{true: 1, false: 0}[scenario.status == 200] {
				t.Fatalf("status %d writes %d, expected %d", out.Code, writes, scenario.status)
			}
			if scenario.status == 200 {
				policy, err := svc.Get(t.Context(), "space", "a")
				if err != nil || policy.PermissionRevision != 2 || policy.Administrator || policy.Coordinated {
					t.Fatalf("ordinary grant changed wrong authority: %+v %v", policy, err)
				}
			}
		})
	}
}
