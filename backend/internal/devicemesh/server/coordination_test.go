package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	meshaudit "github.com/u-ai/backend/internal/devicemesh/audit"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRemoteAdministratorPolicyReadDoesNotDowngradeCoordination(t *testing.T) {
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
		`CREATE TABLE kernel_devices(device_id TEXT PRIMARY KEY,space_id TEXT,trust_state TEXT,created_at TEXT,last_seen_at TEXT)`,
	} {
		if _, err := db.Exec(schema); err != nil {
			t.Fatal(err)
		}
	}
	policies := coordination.NewService(db)
	policy, err := policies.InitializeCoordinated(t.Context(), "space", "phone", 1, "center-role")
	if err != nil {
		t.Fatal(err)
	}
	policy, err = policies.GrantAdministrator(t.Context(), "space", "phone", policy.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
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
	router.GET("/coordination/me", makePolicyHandler(&RouterDeps{
		Coordination:              policies,
		LocalCoreDeviceID:         "core",
		Coordinated:               false,
		BusinessCoordinationReady: true,
	}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/coordination/me", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("%d %s", response.Code, response.Body.String())
	}
	var result struct {
		Policy         coordination.Policy `json:"policy"`
		CoreDataAccess struct {
			Write bool `json:"write"`
		} `json:"coreDataAccess"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Policy.Coordinated || !result.Policy.Administrator || !result.CoreDataAccess.Write {
		t.Fatalf("remote administrator was downgraded: %+v", result)
	}
	stored, err := policies.Get(t.Context(), "space", "phone")
	if err != nil {
		t.Fatal(err)
	}
	if !stored.Coordinated || !stored.Administrator || stored.ModeRevision != policy.ModeRevision || stored.PermissionRevision != policy.PermissionRevision {
		t.Fatalf("stored policy changed during read: before=%+v after=%+v", policy, stored)
	}
}
