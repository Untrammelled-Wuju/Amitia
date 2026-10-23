package configwrite_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/safety"
	"github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func configurationActionRouter(db *gorm.DB) *gin.Engine {
	router := gin.New()
	h := system.NewHandler(system.NewService(&app.AppContext{DB: db}, ""), db, nil, nil, nil, nil, nil)
	router.DELETE("/audit", h.ClearAuditLogs)
	router.DELETE("/usage", h.UsageClear)
	router.DELETE("/logs", h.LogsDelete)
	router.DELETE("/model-errors", h.LogsDeleteModelErrors)
	router.POST("/rotate", h.RotateLogs)
	router.POST("/cleanup", h.CleanupTemp)
	router.POST("/privacy/mask", h.PrivacyMask)
	router.POST("/privacy/delete", h.PrivacyDeletionRequest)
	router.POST("/privacy/cleanup", h.PrivacyDeletionCleanup)
	router.DELETE("/privacy/scan-results", h.PrivacyScanResults)
	router.POST("/shadow/start", h.ShadowModeStart)
	router.POST("/shadow/stop", h.ShadowModeStop)
	router.POST("/shadow/phase", h.ShadowModePhaseAdvance)
	router.PUT("/shadow/thresholds", h.ShadowModeUpdateThresholds)
	router.POST("/shadow/compare", h.ShadowModeCompare)
	router.POST("/shadow/load", h.ShadowModeLoadSim)
	router.POST("/shadow/longitudinal", h.ShadowModeLongitudinalSim)
	return router
}

func TestSafetyIncidentAPIsUseOwnerScopeAndRevision(t *testing.T) {
	db, svc := configurationDB(t)
	ctx, finish := configurationScope(t, svc)
	defer finish()
	if err := db.Exec(`INSERT INTO safety_incidents(id,space_id,owner_id,core_id,role_id,conversation_id,message_id,request_id,event_type,severity,description,status,policy_revision,revision,details_json,created_at,updated_at) VALUES('current','space','space','space','role','conversation','message','request','policy.violation','high','current','open',1,1,'{}','2026-10-23T00:00:00Z','2026-10-23T00:00:00Z'),('foreign','space','other','space','role','conversation','message','request','policy.violation','high','foreign','open',1,1,'{}','2026-10-23T00:00:00Z','2026-10-23T00:00:00Z')`).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		actor := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: runtimeidentity.SpaceID("space"), DeviceID: runtimeidentity.DeviceID("admin"), Permissions: []string{auth.PermSystemAdmin}}
		c.Request = c.Request.WithContext(auth.WithActor(c.Request.Context(), actor))
		c.Next()
	})
	h := system.NewHandler(system.NewService(&app.AppContext{DB: db}, ""), db, nil, nil, nil, nil, nil)
	router.GET("/safety", h.SafetyEvents)
	router.PUT("/safety/:id/handle", h.HandleSafetyEvent)
	router.DELETE("/safety", h.DeleteSafetyEvents)
	var page struct {
		Items []safety.Incident `json:"items"`
		Total int64             `json:"total"`
	}
	if code := configurationJSONResult(t, router, ctx, http.MethodGet, "/safety?status=open", `{}`, &page); code != 200 || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != "current" {
		t.Fatalf("scoped incident list=%+v code=%d", page, code)
	}
	var handled safety.Incident
	if code := configurationJSONResult(t, router, ctx, http.MethodPut, "/safety/current/handle", `{"expectedRevision":1}`, &handled); code != 200 || handled.Status != "resolved" || handled.Revision != 2 || handled.ResolvedBy != "admin" {
		t.Fatalf("incident handle=%+v code=%d", handled, code)
	}
	if err := db.Exec(`INSERT INTO safety_incidents(id,space_id,owner_id,core_id,event_type,severity,description,status,revision,details_json,created_at,updated_at) VALUES('current-open','space','space','space','policy.violation','high','current open','open',1,'{}','2026-10-23T00:00:01Z','2026-10-23T00:00:01Z')`).Error; err != nil {
		t.Fatal(err)
	}
	var deleted struct {
		Deleted int64 `json:"deleted"`
	}
	if code := configurationJSONResult(t, router, ctx, http.MethodDelete, "/safety", `{}`, &deleted); code != 200 || deleted.Deleted != 2 {
		t.Fatalf("scoped incident delete=%+v code=%d", deleted, code)
	}
	var foreign int64
	if err := db.Table("safety_incidents").Where("id='foreign'").Count(&foreign).Error; err != nil || foreign != 1 {
		t.Fatalf("foreign incident changed: %d %v", foreign, err)
	}
}

func TestConfigurationAdministratorActionsRejectExpiredScopeBeforeSQLFilesAndMemory(t *testing.T) {
	db, svc := configurationDB(t)
	ctx, finish := configurationScope(t, svc)
	defer finish()
	policy, err := svc.Get(t.Context(), "space", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GrantAdministrator(t.Context(), "space", "admin", policy.PermissionRevision, false); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO messages(id,conversation_id,role,content,tokens) VALUES('message','conversation','user','private',13)`).Error; err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Mkdir("logs", 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("logs", "fixture.log"), []byte("model error fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	router := configurationActionRouter(db)
	for _, request := range []struct{ method, path, body string }{
		{http.MethodDelete, "/audit", `{}`}, {http.MethodDelete, "/usage", `{}`},
		{http.MethodDelete, "/logs", `{}`}, {http.MethodDelete, "/model-errors", `{}`}, {http.MethodPost, "/rotate", `{}`}, {http.MethodPost, "/cleanup", `{}`},
		{http.MethodPost, "/privacy/mask", `{"ids":["message"],"confirmToken":"确认脱敏"}`}, {http.MethodPost, "/privacy/delete", `{"targetId":"message","targetType":"message"}`}, {http.MethodPost, "/privacy/cleanup", `{}`}, {http.MethodDelete, "/privacy/scan-results", `{}`},
		{http.MethodPost, "/shadow/start", `{}`}, {http.MethodPost, "/shadow/stop", `{}`}, {http.MethodPost, "/shadow/phase", `{}`}, {http.MethodPut, "/shadow/thresholds", `{"maxErrorRate":0.9}`}, {http.MethodPost, "/shadow/compare", `{}`}, {http.MethodPost, "/shadow/load", `{}`}, {http.MethodPost, "/shadow/longitudinal", `{}`},
	} {
		if code := configurationHTTPResult(t, router, ctx, request.method, request.path, request.body); code == 200 || code == 0 {
			t.Fatalf("expired administrative action succeeded: %s", request.path)
		}
	}
	var count int64
	if err := db.Table("messages").Where("id='message' AND content='private' AND tokens=13").Count(&count).Error; err != nil || count != 1 {
		t.Fatal("expired scope changed message")
	}
	if data, err := os.ReadFile(filepath.Join("logs", "fixture.log")); err != nil || string(data) != "model error fixture" {
		t.Fatal("expired action changed file")
	}
}

func TestConfigurationAdministratorActionsPropagateSQLFailuresAndRollbackCancellation(t *testing.T) {
	db, svc := configurationDB(t)
	ctx, finish := configurationScope(t, svc)
	defer finish()
	router := configurationActionRouter(db)
	if code := configurationHTTPResult(t, router, ctx, http.MethodDelete, "/audit", `{}`); code == 200 || code == 0 {
		t.Fatal("missing legacy audit table falsely reported deleted")
	}
}
