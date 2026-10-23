package configwrite_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/safety"
)

func TestScopedSafetyConfigurationUsesRevisionAndPropagatesPersistenceFailure(t *testing.T) {
	db, svc := configurationDB(t)
	ctx, finish := configurationScope(t, svc)
	defer finish()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		actor := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: runtimeidentity.SpaceID("space"), DeviceID: runtimeidentity.DeviceID("admin"), Permissions: []string{auth.PermSystemAdmin}}
		c.Request = c.Request.WithContext(auth.WithActor(c.Request.Context(), actor))
		c.Next()
	})
	h := safety.NewHandler(db)
	router.PUT("/config", h.PutConfig)
	router.GET("/config", h.GetConfig)
	router.PUT("/bdi", h.PutBdiConfig)
	router.GET("/bdi", h.GetBdiConfig)
	var current safety.SafetyConfig
	if code := configurationJSONResult(t, router, ctx, http.MethodGet, "/config", `{}`, &current); code != 200 || current.Revision != 1 {
		t.Fatalf("initial configuration revision=%d code=%d", current.Revision, code)
	}
	current.ViolationAction = "rewrite"
	current.ExpectedRevision = current.Revision
	if code := configurationJSONResult(t, router, ctx, http.MethodPut, "/config", mustJSON(t, current), &current); code != 200 || current.Revision != 2 {
		t.Fatalf("configuration update revision=%d code=%d", current.Revision, code)
	}
	if code := configurationJSONResult(t, router, ctx, http.MethodPut, "/config", mustJSON(t, current), &current); code != 200 || current.Revision != 3 {
		t.Fatalf("second configuration update revision=%d code=%d", current.Revision, code)
	}
	stale := current
	stale.Revision = 1
	stale.ExpectedRevision = 1
	if code := configurationJSONResult(t, router, ctx, http.MethodPut, "/config", mustJSON(t, stale), &current); code != 409 {
		t.Fatalf("stale safety configuration accepted: %d", code)
	}
	if err := db.Exec(`CREATE TRIGGER reject_safety_policy BEFORE UPDATE ON safety_policies BEGIN SELECT RAISE(ABORT,'fixture persistence failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	current.ExpectedRevision = current.Revision
	if code := configurationJSONResult(t, router, ctx, http.MethodPut, "/bdi", `{"expectedRevision":3}`, &current); code == 200 || code == 0 {
		t.Fatal("failed safety persistence falsely succeeded")
	}
	policy, err := svc.Get(t.Context(), "space", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GrantAdministrator(t.Context(), "space", "admin", policy.PermissionRevision, false); err != nil {
		t.Fatal(err)
	}
	policy, err = svc.Get(t.Context(), "space", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GrantAdministrator(t.Context(), "space", "admin", policy.PermissionRevision, true); err != nil {
		t.Fatal(err)
	}
	for _, request := range []struct{ method, path, body string }{
		{http.MethodPut, "/config", `{"violationAction":"audit_only","expectedRevision":3}`},
		{http.MethodPut, "/bdi", `{"expectedRevision":3}`},
		{http.MethodGet, "/config", `{}`},
		{http.MethodGet, "/bdi", `{}`},
	} {
		if code := configurationHTTPResult(t, router, ctx, request.method, request.path, request.body); code == 200 || code == 0 {
			t.Fatal("expired safety configuration request succeeded", request.path)
		}
	}
	var revision int64
	if err := db.Table("safety_policies").Select("revision").Where("space_id='space' AND owner_id='space'").Scan(&revision).Error; err != nil || revision != 3 {
		t.Fatalf("stale or expired request changed safety policy: %d %v", revision, err)
	}
}

func configurationJSONResult(t *testing.T, router *gin.Engine, ctx context.Context, method, path, body string, target any) int {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	var result struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("response %s: %v", response.Body.String(), err)
	}
	if len(result.Data) > 0 && string(result.Data) != "null" {
		if err := json.Unmarshal(result.Data, target); err != nil {
			t.Fatalf("response data %s: %v", string(result.Data), err)
		}
	}
	return result.Code
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
