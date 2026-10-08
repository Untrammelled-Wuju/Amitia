package configwrite_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/asr"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/configwrite"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/tts"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func configurationDB(t *testing.T) (*gorm.DB, *coordination.Service) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "configs.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	raw.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = raw.Close() })
	baseline, err := os.ReadFile(filepath.Join("..", "migration", "baseline.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"model_configs", "tts_configs", "asr_configs", "tts_cloned_voices", "model_scenario_routes", "embedding_configs", "vision_configs", "image_gen_configs", "app_settings", "search_api_keys", "search_credential_cleanup", "messages", "safety_events"} {
		statement := regexp.MustCompile(`(?s)CREATE TABLE IF NOT EXISTS ` + regexp.QuoteMeta(table) + `\s*\(.*?\);`).Find(baseline)
		if len(statement) == 0 {
			t.Fatalf("production baseline missing %s", table)
		}
		if err := db.Exec(string(statement)).Error; err != nil {
			t.Fatal(err)
		}
	}
	kernelDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db"))
	if err != nil {
		t.Fatal(err)
	}
	kernelDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = kernelDB.Close() })
	for _, statement := range []string{coordination.PolicySchema, coordination.ProviderSchema, coordination.RemoteAuthoritySchema, coordination.CancelledAuthoritySchema, coordination.SourceAuthoritySchema, `CREATE TABLE kernel_devices(device_id TEXT PRIMARY KEY,space_id TEXT NOT NULL,trust_state TEXT NOT NULL,created_at TEXT NOT NULL,last_seen_at TEXT NOT NULL)`} {
		if _, err := kernelDB.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := kernelDB.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES('admin','space','trusted','now','now')`); err != nil {
		t.Fatal(err)
	}
	svc := coordination.NewService(kernelDB)
	if err := svc.InitializeCoreConsole(t.Context(), "space", "admin"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"model_configs", "tts_configs", "asr_configs", "embedding_configs", "vision_configs", "image_gen_configs"} {
		if err := db.Exec("INSERT INTO " + table + "(id,name,is_active) VALUES(1,'previous',1),(2,'next',0)").Error; err != nil {
			t.Fatal(err)
		}
	}
	return db, svc
}

func configurationScope(t *testing.T, svc *coordination.Service) (context.Context, func()) {
	t.Helper()
	ctx, _, finish, err := svc.Begin(t.Context(), "space", "admin", "", "space", "", "config-request")
	if err != nil {
		t.Fatal(err)
	}
	return ctx, finish
}

func TestConfigurationRepositoriesRejectRevokedAdministratorWrites(t *testing.T) {
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
	model := chat.NewRepository(&app.AppContext{DB: db.WithContext(ctx)})
	voice := tts.NewRepository(db.WithContext(ctx))
	speech := asr.NewRepository(db.WithContext(ctx))
	operations := []struct {
		name  string
		write func() error
	}{
		{"model-create", func() error { return model.CreateModel(&chat.ModelConfig{Name: "late", IsActive: 1}) }},
		{"model-update", func() error { return model.UpdateModel(1, map[string]interface{}{"name": "late"}) }},
		{"model-delete", func() error { return model.DeleteModel(1) }},
		{"model-active", func() error { return model.ActivateModel(2) }},
		{"model-routes", func() error {
			return model.UpdateModelRoutes([]map[string]interface{}{{"scenario": "chat", "modelConfigId": 2}})
		}},
		{"tts-create", func() error { return voice.Create(&tts.TtsConfig{Name: "late", IsActive: 1}) }},
		{"tts-update", func() error { return voice.Update(1, map[string]interface{}{"name": "late"}) }},
		{"tts-delete", func() error { return voice.Delete(1) }},
		{"tts-active", func() error { return voice.Activate(2) }},
		{"tts-clone", func() error { return voice.UpsertClonedVoice(&tts.ClonedVoice{SpaceID: "space", SpeakerID: "late"}) }},
		{"asr-create", func() error { return speech.Create(&asr.AsrConfig{Name: "late", IsActive: 1}) }},
		{"asr-update", func() error { return speech.Update(1, map[string]interface{}{"name": "late"}) }},
		{"asr-delete", func() error { return speech.Delete(1) }},
		{"asr-active", func() error { return speech.Activate(2) }},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			if err := operation.write(); err == nil {
				t.Fatal("revoked administrator persisted configuration")
			}
		})
	}
	for _, table := range []string{"model_configs", "tts_configs", "asr_configs"} {
		var count int64
		if err := db.Table(table).Where("id=1 AND name='previous' AND is_active=1 OR id=2 AND name='next' AND is_active=0").Count(&count).Error; err != nil || count != 2 {
			t.Fatalf("%s changed: %d %v", table, count, err)
		}
	}
}

func TestConfigurationActivationRollsBackCanceledTransaction(t *testing.T) {
	for _, table := range []string{"model_configs", "tts_configs", "asr_configs"} {
		t.Run(table, func(t *testing.T) {
			db, svc := configurationDB(t)
			guarded, finish := configurationScope(t, svc)
			defer finish()
			ctx, cancel := context.WithCancel(guarded)
			defer cancel()
			if err := db.Callback().Update().After("gorm:update").Register("test:cancel-active", func(tx *gorm.DB) {
				if tx.Statement.Table == table {
					cancel()
				}
			}); err != nil {
				t.Fatal(err)
			}
			var err error
			switch table {
			case "model_configs":
				err = chat.NewRepository(&app.AppContext{DB: db.WithContext(ctx)}).ActivateModel(2)
			case "tts_configs":
				err = tts.NewRepository(db.WithContext(ctx)).Activate(2)
			case "asr_configs":
				err = asr.NewRepository(db.WithContext(ctx)).Activate(2)
			}
			if err == nil {
				t.Fatal("canceled partial activation succeeded")
			}
			var active int64
			if err := db.Table(table).Where("id=1 AND is_active=1").Count(&active).Error; err != nil || active != 1 {
				t.Fatalf("previous active lost: %d %v", active, err)
			}
		})
	}
}

func TestConfigurationGuardAndTransactionAreAtomicOnSingleConnection(t *testing.T) {
	db, svc := configurationDB(t)
	ctx, finish := configurationScope(t, svc)
	defer finish()
	errSentinel := errors.New("rollback")
	if err := configwrite.Transaction(db.WithContext(ctx), func(tx *gorm.DB) error {
		if err := tx.Exec("UPDATE model_configs SET name='partial' WHERE id=1").Error; err != nil {
			return err
		}
		return errSentinel
	}); !errors.Is(err, errSentinel) {
		t.Fatalf("transaction error: %v", err)
	}
	if err := tts.NewRepository(db.WithContext(ctx)).Activate(999); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing target: %v", err)
	}
	if err := asr.NewRepository(db.WithContext(ctx)).Activate(999); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing target: %v", err)
	}
	for _, table := range []string{"model_configs", "tts_configs", "asr_configs"} {
		var count int64
		if err := db.Table(table).Where("id=1 AND name='previous' AND is_active=1").Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("rollback failed: %s %d %v", table, count, err)
		}
	}
}

func TestConfigurationHTTPHandlersKeepTheRevocableRequestScope(t *testing.T) {
	db, svc := configurationDB(t)
	ctx, finish := configurationScope(t, svc)
	defer finish()
	policy, err := svc.Get(t.Context(), "space", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ChangeMode(t.Context(), "space", "admin", policy.ModeRevision, false, ""); err != nil {
		t.Fatal(err)
	}
	appctx := &app.AppContext{DB: db}
	models := chat.NewHandler(chat.NewService(chat.NewRepository(appctx), appctx, nil, nil, nil, nil, nil, nil, nil, nil))
	voices := tts.NewHandler(tts.NewService(tts.NewRepository(db)))
	transcription := asr.NewHandler(asr.NewService(asr.NewRepository(db)))
	router := gin.New()
	router.POST("/models", models.CreateModel)
	router.PUT("/models/:id", models.UpdateModel)
	router.POST("/models/:id/activate", models.ActivateModel)
	router.DELETE("/models/:id", models.DeleteModel)
	router.PUT("/routes", models.UpdateModelRoutes)
	router.POST("/tts", voices.Create)
	router.PUT("/tts/:id", voices.Update)
	router.POST("/tts/:id/activate", voices.Activate)
	router.DELETE("/tts/:id", voices.Delete)
	router.POST("/asr", transcription.Create)
	router.PUT("/asr/:id", transcription.Update)
	router.POST("/asr/:id/activate", transcription.Activate)
	router.DELETE("/asr/:id", transcription.Delete)
	for _, request := range []struct{ method, path, body string }{
		{http.MethodPost, "/models", `{"name":"late"}`}, {http.MethodPut, "/models/1", `{"name":"late"}`}, {http.MethodPost, "/models/2/activate", `{}`}, {http.MethodDelete, "/models/1", `{}`}, {http.MethodPut, "/routes", `{"routes":[]}`},
		{http.MethodPost, "/tts", `{"name":"late"}`}, {http.MethodPut, "/tts/1", `{"name":"late"}`}, {http.MethodPost, "/tts/2/activate", `{}`}, {http.MethodDelete, "/tts/1", `{}`},
		{http.MethodPost, "/asr", `{"name":"late"}`}, {http.MethodPut, "/asr/1", `{"name":"late"}`}, {http.MethodPost, "/asr/2/activate", `{}`}, {http.MethodDelete, "/asr/1", `{}`},
	} {
		t.Run(request.method+request.path, func(t *testing.T) {
			r := httptest.NewRequest(request.method, request.path, strings.NewReader(request.body)).WithContext(ctx)
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			var result struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Code == 200 || result.Code == 0 {
				t.Fatalf("late handler reported success: %s %v", w.Body.String(), err)
			}
		})
	}
	for _, table := range []string{"model_configs", "tts_configs", "asr_configs"} {
		var count int64
		if err := db.Table(table).Where("id=1 AND name='previous' AND is_active=1 OR id=2 AND name='next' AND is_active=0").Count(&count).Error; err != nil || count != 2 {
			t.Fatalf("%s changed through handler: %d %v", table, count, err)
		}
	}
}
