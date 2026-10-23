package system

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/chat/importbatch"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type chatImportSummaryStub struct {
	chat.Service
}

func (chatImportSummaryStub) GenerateWorkshopJSON(context.Context, string, string) (string, string, string, error) {
	return `{"summary":"导入摘要"}`, "", "", nil
}

func newChatImportTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "chat-import.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&importbatch.Batch{}, &importbatch.Item{}); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE characters (id TEXT PRIMARY KEY, space_id TEXT, name TEXT, deleted_at DATETIME)`,
		`CREATE TABLE conversations (id TEXT PRIMARY KEY)`,
		`CREATE TABLE messages (id TEXT PRIMARY KEY)`,
		`INSERT INTO characters (id, space_id, name) VALUES ('role', 'space', 'Role')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	handler := NewHandler(NewService(&app.AppContext{DB: db}, runtimeprofile.ProfileLocal), db, chatImportSummaryStub{}, nil, nil, nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("actorContext", &auth.ActorContext{
			PrincipalType:  auth.PrincipalLocalUI,
			SpaceID:        "space",
			DeviceID:       "device",
			IsLocalTrusted: true,
			Permissions:    auth.OwnerDevicePermissions(),
		})
		c.Next()
	})
	registerChatImportRoutes(router.Group("/api"), handler)
	return router, db
}

func importTestRequest(t *testing.T, router *gin.Engine, path string, body any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s status=%d body=%s", path, recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response.Data
}

func TestChatImportConfirmUsesStagingOnlyAndIsIdempotent(t *testing.T) {
	router, db := newChatImportTestRouter(t)
	payload := map[string]any{
		"characterId":    "role",
		"title":          "Imported",
		"defaultRole":    "user",
		"detectedFormat": "standard",
		"items": []map[string]any{
			{"role": "user", "content": "hello", "lineNo": 1},
			{"role": "assistant", "content": "world", "lineNo": 2},
		},
	}
	first := importTestRequest(t, router, "/api/imports/confirm", payload)
	second := importTestRequest(t, router, "/api/imports/confirm", payload)
	if first["batchId"] != second["batchId"] || second["duplicate"] != true {
		t.Fatalf("first=%v second=%v", first, second)
	}
	var conversations, messages int64
	if err := db.Table("conversations").Count(&conversations).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("messages").Count(&messages).Error; err != nil {
		t.Fatal(err)
	}
	if conversations != 0 || messages != 0 {
		t.Fatalf("legacy tables written conversations=%d messages=%d", conversations, messages)
	}
	var batches, items int64
	if err := db.Model(&importbatch.Batch{}).Count(&batches).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&importbatch.Item{}).Count(&items).Error; err != nil {
		t.Fatal(err)
	}
	if batches != 1 || items != 2 {
		t.Fatalf("batches=%d items=%d", batches, items)
	}
}

func TestChatImportRoutesRemoveLegacyWritesAndFakeUpload(t *testing.T) {
	router, _ := newChatImportTestRouter(t)
	routes := map[string]bool{}
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, removed := range []string{
		"POST /api/import",
		"POST /api/imports/upload",
		"GET /api/imports/batches/:id/memory-candidates",
		"POST /api/imports/batches/:id/extract-memory-candidates",
		"POST /api/imports/batches/:id/confirm-memories",
	} {
		if routes[removed] {
			t.Fatalf("legacy route still registered: %s", removed)
		}
	}
	for _, current := range []string{
		"POST /api/imports/parse-text",
		"POST /api/imports/confirm",
		"GET /api/imports/batches",
	} {
		if !routes[current] {
			t.Fatalf("current route missing: %s", current)
		}
	}
}

func TestChatImportSummaryUsesStagedItemsOnly(t *testing.T) {
	router, db := newChatImportTestRouter(t)
	confirmed := importTestRequest(t, router, "/api/imports/confirm", map[string]any{
		"characterId": "role",
		"title":       "Imported",
		"items": []map[string]any{
			{"role": "user", "content": "hello"},
			{"role": "assistant", "content": "world"},
		},
	})
	batchID, _ := confirmed["batchId"].(string)
	importTestRequest(t, router, "/api/imports/batches/"+batchID+"/generate-summary", map[string]any{})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/imports/batches/"+batchID+"/summary", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data struct {
			Summary struct {
				Summary      string `json:"summary"`
				MessageCount int    `json:"messageCount"`
			} `json:"summary"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.Summary.Summary != "导入摘要" || response.Data.Summary.MessageCount != 2 {
		t.Fatalf("summary=%+v", response.Data.Summary)
	}
	var legacyMessages int64
	if err := db.Table("messages").Count(&legacyMessages).Error; err != nil {
		t.Fatal(err)
	}
	if legacyMessages != 0 {
		t.Fatalf("legacy messages=%d", legacyMessages)
	}
}
