package security

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestCoreConfigurationIntentCannotRetargetAdministratorWrite(t *testing.T) {
	for _, scenario := range []struct {
		name, expected string
		administrator  bool
		status         int
	}{
		{"old_core_form", "core-b", true, 409},
		{"current_core_form", "core-c", true, 200},
		{"legacy_administrator", "", true, 200},
		{"ordinary_cannot_gain_permission", "core-c", false, 403},
		{"ambiguous_core", "core-c ", true, 409},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			router := gin.New()
			permissions := auth.StandardPermissions()
			if scenario.administrator {
				permissions = auth.OwnerDevicePermissions()
			}
			router.Use(func(c *gin.Context) {
				c.Set("actorContext", &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: runtimeidentity.SpaceID("core-c"), Permissions: permissions})
				c.Next()
			})
			writes := 0
			router.PUT("/api/model/configs/same-id", SharedCoreAdminOnly(), func(c *gin.Context) { writes++; c.Status(200) })
			req := httptest.NewRequest(http.MethodPut, "/api/model/configs/same-id", nil)
			if scenario.expected != "" {
				req.Header.Set(ExpectedCoreHeader, scenario.expected)
			}
			out := httptest.NewRecorder()
			router.ServeHTTP(out, req)
			if out.Code != scenario.status {
				t.Fatalf("status=%d expected=%d", out.Code, scenario.status)
			}
			if writes != 0 && scenario.status != 200 {
				t.Fatal("rejected configuration reached mutation handler")
			}
		})
	}
}

func TestCoreConfigurationIntentRejectsOldPolicyAfterAdministratorRegrant(t *testing.T) {
	for _, scenario := range []struct {
		name, policy string
		current      coordination.Policy
		status       int
	}{
		{"current", "1:2:3", coordination.Policy{Coordinated: true, Administrator: true, ProviderEpoch: 1, ModeRevision: 2, PermissionRevision: 3}, 200},
		{"regranted", "1:2:3", coordination.Policy{Coordinated: true, Administrator: true, ProviderEpoch: 1, ModeRevision: 2, PermissionRevision: 5}, 409},
		{"mode_reenabled", "1:2:3", coordination.Policy{Coordinated: true, Administrator: true, ProviderEpoch: 1, ModeRevision: 4, PermissionRevision: 5}, 409},
		{"same_core_new_provider", "1:2:3", coordination.Policy{Coordinated: true, Administrator: true, ProviderEpoch: 2, ModeRevision: 2, PermissionRevision: 3}, 409},
		{"malformed", "01:2:3", coordination.Policy{Coordinated: true, Administrator: true, ProviderEpoch: 1, ModeRevision: 2, PermissionRevision: 3}, 409},
		{"missing_authenticated_policy", "1:2:3", coordination.Policy{}, 409},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("actorContext", &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "core-c", Permissions: auth.OwnerDevicePermissions()})
				if scenario.current.ProviderEpoch > 0 {
					c.Set(configurationPolicyContextKey, scenario.current)
				}
				c.Next()
			})
			writes := 0
			router.PUT("/configuration", SharedCoreAdminOnly(), func(c *gin.Context) { writes++; c.Status(200) })
			req := httptest.NewRequest(http.MethodPut, "/configuration", nil)
			req.Header.Set(ExpectedCoreHeader, "core-c")
			req.Header.Set(ExpectedConfigurationPolicyHeader, scenario.policy)
			out := httptest.NewRecorder()
			router.ServeHTTP(out, req)
			if out.Code != scenario.status || writes != map[bool]int{true: 1, false: 0}[scenario.status == 200] {
				t.Fatalf("policy status=%d writes=%d expected=%d", out.Code, writes, scenario.status)
			}
		})
	}
}
