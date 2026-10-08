package configwrite_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/securityaudit"
	"github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/pkg/app"
)

func TestConfigurationAuditAdministratorUsesCanonicalCoreRowsAndCurrentSpace(t *testing.T) {
	db, svc := configurationDB(t)
	baseline, err := os.ReadFile(filepath.Join("..", "migration", "baseline.sql"))
	if err != nil {
		t.Fatal(err)
	}
	ddl := regexp.MustCompile(`(?s)CREATE TABLE IF NOT EXISTS security_audit_events\s*\(.*?\);`).Find(baseline)
	if err := db.Exec(string(ddl)).Error; err != nil {
		t.Fatal(err)
	}
	repo := securityaudit.NewRepository(db)
	for _, event := range []securityaudit.AuditEvent{
		{EventID: "first", SpaceID: "space", EventType: "pair-approved", ReasonCode: "device-trusted", Outcome: "success", OccurredAt: "2026-10-08T01:00:00Z"},
		{EventID: "second", SpaceID: "space", EventType: "admin-revoked", ReasonCode: "permission-changed", Outcome: "success", OccurredAt: "2026-10-08T02:00:00Z"},
		{EventID: "private-other-realm", SpaceID: "other", EventType: "private", Outcome: "success", OccurredAt: "2026-10-08T03:00:00Z"},
	} {
		if err := repo.Insert(&event); err != nil {
			t.Fatal(err)
		}
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		actor := &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: runtimeidentity.SpaceID("space"), Permissions: []string{auth.PermSystemAdmin}}
		c.Set("actorContext", actor)
		c.Request = c.Request.WithContext(auth.WithActor(c.Request.Context(), actor))
		c.Next()
	})
	h := system.NewHandler(system.NewService(&app.AppContext{DB: db}, ""), db, nil, nil, nil, nil, nil)
	router.GET("/audit", h.AuditLogs)
	router.GET("/stats", h.AuditStats)
	router.DELETE("/audit", h.ClearAuditLogs)
	ctx, finish := configurationScope(t, svc)
	defer finish()
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/audit?limit=200&spaceId=other", nil).WithContext(ctx))
	var logs struct {
		Code int `json:"code"`
		Data []struct {
			ID     string `json:"id"`
			Time   string `json:"time"`
			Rule   string `json:"ruleId"`
			Action string `json:"action"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &logs); err != nil || logs.Code != 200 || len(logs.Data) != 2 {
		t.Fatalf("canonical audit list failed: %s", response.Body.String())
	}
	if logs.Data[0].ID != "second" || logs.Data[0].Action != "admin-revoked" || logs.Data[0].Rule != "permission-changed" {
		t.Fatal("Web audit compatibility lost canonical event identity")
	}
	if code := configurationHTTPResult(t, router, ctx, http.MethodDelete, "/audit", `{}`); code != 200 {
		t.Fatalf("canonical clear rejected: %d", code)
	}
	events, err := repo.ListSpaceEvents("space", 50, "")
	if err != nil || len(events) != 0 {
		t.Fatal("canonical security API still sees cleared Core data")
	}
	events, err = repo.ListSpaceEvents("other", 50, "")
	if err != nil || len(events) != 1 {
		t.Fatal("Core administrator deletion crossed authorization realm")
	}
	if err := repo.Insert(&securityaudit.AuditEvent{EventID: "remaining", SpaceID: "space", EventType: "fixture", Outcome: "success"}); err != nil {
		t.Fatal(err)
	}
	policy, err := svc.Get(t.Context(), "space", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GrantAdministrator(t.Context(), "space", "admin", policy.PermissionRevision, false); err != nil {
		t.Fatal(err)
	}
	if code := configurationHTTPResult(t, router, ctx, http.MethodDelete, "/audit", `{}`); code == 200 || code == 0 {
		t.Fatal("expired admin cleared canonical audit")
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/audit", strings.NewReader(`{}`)).WithContext(ctx))
	if !strings.Contains(response.Body.String(), `"code":409`) {
		t.Fatal("expired administrator read canonical private data", response.Body.String())
	}
}
