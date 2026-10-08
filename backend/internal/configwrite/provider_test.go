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
	"github.com/u-ai/backend/internal/embedding_config"
	"github.com/u-ai/backend/internal/imagegen"
	"github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/internal/vision"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func configurationProviderRouter(db *gorm.DB) *gin.Engine {
	router := gin.New()
	embedding := embedding_config.NewHandler(embedding_config.NewService(embedding_config.NewRepository(db)))
	visual := vision.NewHandler(vision.NewService(vision.NewRepository(db)))
	image := imagegen.NewHandler(imagegen.NewService(imagegen.NewRepository(db)))
	for _, provider := range []struct {
		name                             string
		create, update, remove, activate gin.HandlerFunc
	}{
		{"embedding", embedding.Create, embedding.Update, embedding.Delete, embedding.Activate},
		{"vision", visual.Create, visual.Update, visual.Delete, visual.Activate},
		{"imagegen", image.Create, image.Update, image.Delete, image.Activate},
	} {
		router.POST("/"+provider.name, provider.create)
		router.PUT("/"+provider.name+"/:id", provider.update)
		router.DELETE("/"+provider.name+"/:id", provider.remove)
		router.POST("/"+provider.name+"/:id/activate", provider.activate)
	}
	appctx := &app.AppContext{DB: db}
	settings := system.NewHandler(system.NewService(appctx, ""), db, nil, nil, nil, nil, nil)
	router.PUT("/config", settings.UpdateConfig)
	router.POST("/config/import", settings.ConfigImportConfirm)
	router.PUT("/theme", settings.UpdateTheme)
	router.PUT("/mood", settings.MoodDetectionConfig)
	return router
}

func configurationHTTPResult(t *testing.T, router *gin.Engine, ctx context.Context, method, path, body string) int {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	var result struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("response %s: %v", response.Body.String(), err)
	}
	return result.Code
}

func TestConfigurationProvidersAndSettingsRejectRevocationAndAdministratorABA(t *testing.T) {
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
	if _, err := svc.GrantAdministrator(t.Context(), "space", "admin", policy.PermissionRevision, true); err != nil {
		t.Fatal(err)
	}
	router := configurationProviderRouter(db)
	for _, name := range []string{"embedding", "vision", "imagegen"} {
		for _, request := range []struct{ method, path, body string }{
			{http.MethodPost, "/" + name, `{"name":"late","isActive":1}`},
			{http.MethodPut, "/" + name + "/1", `{"name":"late"}`},
			{http.MethodDelete, "/" + name + "/1", `{}`},
			{http.MethodPost, "/" + name + "/2/activate", `{}`},
		} {
			if code := configurationHTTPResult(t, router, old, request.method, request.path, request.body); code == 200 || code == 0 {
				t.Fatalf("old provider scope survived ABA: %s %s", request.method, request.path)
			}
		}
	}
	for _, request := range []struct{ method, path, body string }{
		{http.MethodPut, "/config", `{"theme":"late","settings":{"k":"v"}}`},
		{http.MethodPost, "/config/import", `{"settings":{"k":"v"}}`},
		{http.MethodPut, "/theme", `{"theme":"late"}`},
		{http.MethodPut, "/mood", `{"enabled":true,"threshold":0.2}`},
	} {
		if code := configurationHTTPResult(t, router, old, request.method, request.path, request.body); code == 200 || code == 0 {
			t.Fatalf("old appsettings scope survived ABA: %s", request.path)
		}
	}
	for _, table := range []string{"embedding_configs", "vision_configs", "image_gen_configs"} {
		var count int64
		if err := db.Table(table).Where("id=1 AND name='previous' AND is_active=1 OR id=2 AND name='next' AND is_active=0").Count(&count).Error; err != nil || count != 2 {
			t.Fatalf("%s changed under old scope", table)
		}
	}
	var count int64
	if err := db.Table("app_settings").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("old settings scope wrote after ABA")
	}
	fresh, freshFinish := configurationScope(t, svc)
	defer freshFinish()
	for _, name := range []string{"embedding", "vision", "imagegen"} {
		if code := configurationHTTPResult(t, router, fresh, http.MethodPut, "/"+name+"/2", `{"name":"fresh"}`); code != 200 {
			t.Fatalf("fresh provider authority refused: %s %d", name, code)
		}
	}
	if code := configurationHTTPResult(t, router, fresh, http.MethodPost, "/config/import", `{"settings":{"k":"fresh"}}`); code != 200 {
		t.Fatalf("fresh settings authority refused: %d", code)
	}
}

func TestConfigurationProvidersActivationCancellationAndMissingTargetKeepPrevious(t *testing.T) {
	for _, provider := range []struct {
		table    string
		activate func(*gorm.DB, int) error
	}{
		{"embedding_configs", func(db *gorm.DB, id int) error { return embedding_config.NewRepository(db).Activate(id) }},
		{"vision_configs", func(db *gorm.DB, id int) error { return vision.NewRepository(db).Activate(id) }},
		{"image_gen_configs", func(db *gorm.DB, id int) error { return imagegen.NewRepository(db).Activate(id) }},
	} {
		t.Run(provider.table, func(t *testing.T) {
			db, svc := configurationDB(t)
			guarded, finish := configurationScope(t, svc)
			defer finish()
			if err := provider.activate(db.WithContext(guarded), 999); err == nil {
				t.Fatal("missing activation target succeeded")
			}
			ctx, cancel := context.WithCancel(guarded)
			defer cancel()
			if err := db.Callback().Update().After("gorm:update").Register("test:provider-cancel", func(tx *gorm.DB) {
				if tx.Statement.Table == provider.table {
					cancel()
				}
			}); err != nil {
				t.Fatal(err)
			}
			if err := provider.activate(db.WithContext(ctx), 2); err == nil {
				t.Fatal("partial activation survived cancellation")
			}
			var count int64
			if err := db.Table(provider.table).Where("id=1 AND is_active=1").Count(&count).Error; err != nil || count != 1 {
				t.Fatal("previous active not rolled back")
			}
		})
	}
}

func TestConfigurationAppSettingsImportRollsBackWholeBatch(t *testing.T) {
	for _, failure := range []string{"database", "canceled"} {
		t.Run(failure, func(t *testing.T) {
			db, svc := configurationDB(t)
			guarded, finish := configurationScope(t, svc)
			defer finish()
			ctx, cancel := context.WithCancel(guarded)
			defer cancel()
			if err := db.Exec(`INSERT INTO app_settings(key,value,revision) VALUES('seed','previous',5)`).Error; err != nil {
				t.Fatal(err)
			}
			if failure == "database" {
				if err := db.Exec(`CREATE TRIGGER reject_setting BEFORE INSERT ON app_settings WHEN NEW.key='zzz' BEGIN SELECT RAISE(ABORT,'injected setting failure'); END`).Error; err != nil {
					t.Fatal(err)
				}
			} else {
				if err := db.Callback().Create().After("gorm:create").Register("test:settings-cancel", func(tx *gorm.DB) {
					if tx.Statement.Table == "app_settings" {
						cancel()
					}
				}); err != nil {
					t.Fatal(err)
				}
			}
			if code := configurationHTTPResult(t, configurationProviderRouter(db), ctx, http.MethodPost, "/config/import", `{"settings":{"aaa":"partial","seed":"changed","zzz":"fail"}}`); code == 200 || code == 0 {
				t.Fatal("failed settings batch reported success")
			}
			var count int64
			if err := db.Table("app_settings").Count(&count).Error; err != nil || count != 1 {
				t.Fatal("partial setting inserted")
			}
			if err := db.Table("app_settings").Where("key='seed' AND value='previous' AND revision=5").Count(&count).Error; err != nil || count != 1 {
				t.Fatal("setting value/revision changed despite rollback")
			}
		})
	}
}

func TestConfigurationProviderRoutesRequireAdministrator(t *testing.T) {
	router := gin.New()
	group := router.Group("/api", func(c *gin.Context) {
		c.Set("actorContext", &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, Permissions: auth.StandardPermissions()})
		c.Next()
	})
	ctx := &app.AppContext{}
	embedding_config.RegisterEmbeddingConfigRouter(group, ctx)
	vision.RegisterVisionRouter(group, ctx)
	imagegen.RegisterImageGenRouter(group, ctx)
	for _, name := range []string{"embedding", "vision", "imagegen"} {
		for _, request := range []struct{ method, path string }{{http.MethodPost, "/configs"}, {http.MethodPut, "/configs/1"}, {http.MethodDelete, "/configs/1"}, {http.MethodPost, "/configs/1/activate"}} {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(request.method, "/api/"+name+request.path, strings.NewReader(`{"name":"must not write"}`)))
			if response.Code != 403 {
				t.Fatalf("unguarded mutation %s %s: %d", request.method, name+request.path, response.Code)
			}
		}
	}
}

func TestConfigurationVisionRepositoryRechecksMainModelInsideWriteTransaction(t *testing.T) {
	db, svc := configurationDB(t)
	ctx, finish := configurationScope(t, svc)
	defer finish()
	if err := db.Exec(`UPDATE model_configs SET capabilities_json='{"supportsImage":true}' WHERE id=1`).Error; err != nil {
		t.Fatal(err)
	}
	repo := vision.NewRepository(db.WithContext(ctx))
	for _, write := range []func() error{
		func() error { return repo.Create(&vision.VisionConfig{Name: "blocked", IsActive: 1}) },
		func() error { return repo.Update(1, map[string]interface{}{"name": "blocked"}) },
		func() error { return repo.Delete(1) },
		func() error { return repo.Activate(2) },
	} {
		if err := write(); err == nil || !strings.Contains(err.Error(), vision.MainModelVisionNotice) {
			t.Fatalf("vision takeover constraint bypassed: %v", err)
		}
	}
	var count int64
	if err := db.Table("vision_configs").Where("id=1 AND name='previous' AND is_active=1 OR id=2 AND name='next' AND is_active=0").Count(&count).Error; err != nil || count != 2 {
		t.Fatal("main-model takeover changed independent config")
	}
}
