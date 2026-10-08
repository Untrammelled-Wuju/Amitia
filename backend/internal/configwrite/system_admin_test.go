package configwrite_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/internal/timeoutpolicy"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func configurationAdministratorRouter(db *gorm.DB) *gin.Engine {
	router := gin.New()
	handler := system.NewHandler(system.NewService(&app.AppContext{DB: db}, ""), db, nil, nil, nil, nil, nil)
	router.PUT("/audit", handler.UpdateAuditSettings)
	router.PUT("/security", handler.UpdateSecurityAccessConfig)
	router.PUT("/runtime", handler.UpdateRuntimeMode)
	router.PUT("/long-running", handler.UpdateLongRunningConfig)
	router.PUT("/update", handler.UpdateConfig_Update)
	router.PUT("/timeout", handler.UpdateTimeoutSettings)
	router.POST("/setup/finish", handler.SetupFinish)
	router.POST("/setup/reset", handler.SetupReset)
	router.POST("/setup/step", handler.SetupStep)
	router.POST("/onboarding/complete", handler.OnboardingComplete)
	router.POST("/onboarding/reset", handler.OnboardingReset)
	router.POST("/release-check", handler.ReleaseCheckRun)
	return router
}

func TestConfigurationAdministratorSettingsKeepRevokedRequestAfterABA(t *testing.T) {
	db, svc := configurationDB(t)
	old, finish := configurationScope(t, svc)
	defer finish()
	policy, err := svc.Get(t.Context(), "space", "admin")
	if err != nil {
		t.Fatal(err)
	}
	policy, err = svc.GrantAdministrator(t.Context(), "space", "admin", policy.PermissionRevision, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.GrantAdministrator(t.Context(), "space", "admin", policy.PermissionRevision, true); err != nil {
		t.Fatal(err)
	}
	router := configurationAdministratorRouter(db)
	for _, request := range []struct{ method, path, body string }{
		{http.MethodPut, "/audit", `{"enabled":false,"retentionDays":3,"logActions":false}`},
		{http.MethodPut, "/security", `{"requireAuth":false,"allowedOrigins":"late","rateLimit":false}`},
		{http.MethodPut, "/runtime", `{"mode":"cloud-web","publicBaseUrl":"late"}`},
		{http.MethodPut, "/long-running", `{"maxTasks":9,"timeoutMinutes":90}`},
		{http.MethodPut, "/update", `{"autoCheck":false}`},
		{http.MethodPut, "/timeout", `{"disabled":false,"seconds":120}`},
		{http.MethodPost, "/setup/finish", `{}`},
		{http.MethodPost, "/setup/reset", `{}`},
		{http.MethodPost, "/setup/step", `{"step":"late"}`},
		{http.MethodPost, "/onboarding/complete", `{}`},
		{http.MethodPost, "/onboarding/reset", `{}`},
		{http.MethodPost, "/release-check", `{}`},
	} {
		if code := configurationHTTPResult(t, router, old, request.method, request.path, request.body); code == 200 || code == 0 {
			t.Fatalf("administrator mutation survived ABA: %s", request.path)
		}
	}
	var count int64
	if err := db.Table("app_settings").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("revoked settings changed: count=%d err=%v", count, err)
	}
	current, currentFinish := configurationScope(t, svc)
	defer currentFinish()
	if code := configurationHTTPResult(t, router, current, http.MethodPut, "/audit", `{"enabled":false,"retentionDays":3,"logActions":false}`); code != 200 {
		t.Fatalf("current administrator rejected: %d", code)
	}
}

func TestConfigurationAdministratorMalformedSettingsDoNotWrite(t *testing.T) {
	db, _ := configurationDB(t)
	router := configurationAdministratorRouter(db)
	for _, path := range []string{"/audit", "/security", "/runtime", "/long-running", "/update"} {
		if code := configurationHTTPResult(t, router, t.Context(), http.MethodPut, path, `{`); code != 400 {
			t.Fatalf("malformed administrator setting %s returned %d", path, code)
		}
	}
	if code := configurationHTTPResult(t, router, t.Context(), http.MethodPost, "/setup/step", `{`); code != 400 {
		t.Fatalf("malformed setup step returned %d", code)
	}
	var count int64
	if err := db.Table("app_settings").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("malformed administrator settings wrote data")
	}
}

func TestConfigurationAdministratorSettingsRollBackMultiKeyAndTimeoutSideEffects(t *testing.T) {
	for _, operation := range []struct{ method, path, body, failureKey string }{
		{http.MethodPut, "/audit", `{"enabled":false,"retentionDays":3,"logActions":false}`, "audit_retention_days"},
		{http.MethodPut, "/security", `{"requireAuth":false,"allowedOrigins":"changed","rateLimit":false}`, "require_auth"},
		{http.MethodPut, "/runtime", `{"mode":"cloud-web","publicBaseUrl":"changed"}`, "runtime_mode"},
		{http.MethodPost, "/setup/finish", `{}`, "setup_step"},
	} {
		t.Run(operation.path, func(t *testing.T) {
			db, svc := configurationDB(t)
			ctx, finish := configurationScope(t, svc)
			defer finish()
			if err := db.Exec(`CREATE TRIGGER reject_admin_setting BEFORE INSERT ON app_settings WHEN NEW.key='` + operation.failureKey + `' BEGIN SELECT RAISE(ABORT,'injected configuration failure'); END`).Error; err != nil {
				t.Fatal(err)
			}
			if code := configurationHTTPResult(t, configurationAdministratorRouter(db), ctx, operation.method, operation.path, operation.body); code == 200 || code == 0 {
				t.Fatal("failed administrator batch reported success")
			}
			var count int64
			if err := db.Table("app_settings").Count(&count).Error; err != nil || count != 0 {
				t.Fatal("administrator setting partially committed")
			}
		})
	}
	db, svc := configurationDB(t)
	guarded, finish := configurationScope(t, svc)
	defer finish()
	ctx, cancel := context.WithCancel(guarded)
	defer cancel()
	restore := timeoutpolicy.Configure(timeoutpolicy.Default())
	defer restore()
	if err := db.Callback().Create().After("gorm:create").Register("test:timeout-cancel", func(tx *gorm.DB) {
		if tx.Statement.Table == "app_settings" {
			cancel()
		}
	}); err != nil {
		t.Fatal(err)
	}
	request := configurationHTTPResult(t, configurationAdministratorRouter(db), ctx, http.MethodPut, "/timeout", `{"disabled":true,"seconds":120}`)
	if request == 200 || request == 0 {
		t.Fatal("canceled timeout save reported success")
	}
	current, _ := timeoutpolicy.Current()
	if current != timeoutpolicy.Default() {
		t.Fatal("uncommitted timeout changed process configuration")
	}
	var count int64
	if err := db.Table("app_settings").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("canceled timeout remained persisted")
	}
}
