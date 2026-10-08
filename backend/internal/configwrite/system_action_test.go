package configwrite_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func configurationActionRouter(db *gorm.DB) *gin.Engine {
	router := gin.New()
	h := system.NewHandler(system.NewService(&app.AppContext{DB: db}, ""), db, nil, nil, nil, nil, nil)
	router.DELETE("/audit", h.ClearAuditLogs)
	router.DELETE("/safety", h.DeleteSafetyEvents)
	router.PUT("/safety/:id", h.HandleSafetyEvent)
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
	if err := db.Exec(`INSERT INTO safety_events(id,handled) VALUES('event',0)`).Error; err != nil {
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
		{http.MethodDelete, "/audit", `{}`}, {http.MethodDelete, "/safety", `{}`}, {http.MethodPut, "/safety/event", `{}`}, {http.MethodDelete, "/usage", `{}`},
		{http.MethodDelete, "/logs", `{}`}, {http.MethodDelete, "/model-errors", `{}`}, {http.MethodPost, "/rotate", `{}`}, {http.MethodPost, "/cleanup", `{}`},
		{http.MethodPost, "/privacy/mask", `{"ids":["message"],"confirmToken":"确认脱敏"}`}, {http.MethodPost, "/privacy/delete", `{"targetId":"message","targetType":"message"}`}, {http.MethodPost, "/privacy/cleanup", `{}`}, {http.MethodDelete, "/privacy/scan-results", `{}`},
		{http.MethodPost, "/shadow/start", `{}`}, {http.MethodPost, "/shadow/stop", `{}`}, {http.MethodPost, "/shadow/phase", `{}`}, {http.MethodPut, "/shadow/thresholds", `{"maxErrorRate":0.9}`}, {http.MethodPost, "/shadow/compare", `{}`}, {http.MethodPost, "/shadow/load", `{}`}, {http.MethodPost, "/shadow/longitudinal", `{}`},
	} {
		if code := configurationHTTPResult(t, router, ctx, request.method, request.path, request.body); code == 200 || code == 0 {
			t.Fatalf("expired administrative action succeeded: %s", request.path)
		}
	}
	var count int64
	if err := db.Table("safety_events").Where("id='event' AND handled=0").Count(&count).Error; err != nil || count != 1 {
		t.Fatal("expired scope changed safety record")
	}
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
	if code := configurationHTTPResult(t, router, ctx, http.MethodPut, "/safety/missing", `{}`); code == 200 || code == 0 {
		t.Fatal("missing safety event reported handled")
	}
	if err := db.Exec(`INSERT INTO safety_events(id,handled) VALUES('event',0)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TRIGGER reject_handled BEFORE UPDATE ON safety_events BEGIN SELECT RAISE(ABORT,'injected error'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if code := configurationHTTPResult(t, router, ctx, http.MethodPut, "/safety/event", `{}`); code == 200 || code == 0 {
		t.Fatal("SQL failure reported handled")
	}
	var count int64
	if err := db.Table("safety_events").Where("id='event' AND handled=0").Count(&count).Error; err != nil || count != 1 {
		t.Fatal("SQL failure changed original event")
	}
}
