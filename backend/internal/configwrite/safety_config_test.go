package configwrite_test

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/safety"
	"github.com/u-ai/backend/internal/system"
)

func TestConfigurationSafetyHandlersRejectABAAndPropagatePersistenceFailure(t *testing.T) {
	db, svc := configurationDB(t)
	ctx, finish := configurationScope(t, svc)
	defer finish()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		actor := &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: runtimeidentity.SpaceID("space"), Permissions: []string{auth.PermSystemAdmin}}
		c.Request = c.Request.WithContext(auth.WithActor(c.Request.Context(), actor))
		c.Next()
	})
	h := safety.NewHandler(db)
	router.PUT("/config", h.PutConfig)
	router.GET("/config", h.GetConfig)
	router.PUT("/bdi", h.PutBdiConfig)
	router.GET("/bdi", h.GetBdiConfig)
	if code := configurationHTTPResult(t, router, ctx, http.MethodPut, "/config", `{"violationAction":"block"}`); code != 200 {
		t.Fatalf("initial configuration failed: %d", code)
	}
	if code := configurationHTTPResult(t, router, ctx, http.MethodPut, "/config", `{"violationAction":"warn"}`); code != 200 {
		t.Fatalf("configuration update failed: %d", code)
	}
	value, revision, err := system.NewSettingsStore(db).Get("safety_config")
	if err != nil || revision != 2 {
		t.Fatalf("safety configuration lost CAS revision: %d %v", revision, err)
	}
	if err := db.Exec(`CREATE TRIGGER reject_safety_bdi BEFORE INSERT ON app_settings WHEN NEW.key='safety_bdi_config' BEGIN SELECT RAISE(ABORT,'fixture persistence failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if code := configurationHTTPResult(t, router, ctx, http.MethodPut, "/bdi", `{}`); code == 200 || code == 0 {
		t.Fatal("failed safety persistence falsely succeeded")
	}
	policy, err := svc.Get(t.Context(), "space", "admin")
	if err != nil {
		t.Fatal(err)
	}
	policy, err = svc.GrantAdministrator(t.Context(), "space", "admin", policy.PermissionRevision, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GrantAdministrator(t.Context(), "space", "admin", policy.PermissionRevision, true); err != nil {
		t.Fatal(err)
	}
	for _, request := range []struct{ method, path, body string }{
		{http.MethodPut, "/config", `{"violationAction":"allow"}`}, {http.MethodPut, "/bdi", `{}`}, {http.MethodGet, "/config", `{}`}, {http.MethodGet, "/bdi", `{}`},
	} {
		if code := configurationHTTPResult(t, router, ctx, request.method, request.path, request.body); code == 200 || code == 0 {
			t.Fatal("expired safety configuration request succeeded", request.path)
		}
	}
	after, afterRevision, err := system.NewSettingsStore(db).Get("safety_config")
	if err != nil || after != value || afterRevision != revision {
		t.Fatal("expired safety request changed authoritative Core settings")
	}
}
