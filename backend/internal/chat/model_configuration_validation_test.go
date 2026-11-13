package chat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/pkg/comment/response"
	"gorm.io/gorm"
)

func TestModelConfigurationRejectsInvalidWrites(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "models.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&ModelConfig{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE model_artifact_references (
		artifact_id TEXT NOT NULL,
		model_config_id INTEGER NOT NULL,
		created_at TEXT NOT NULL,
		PRIMARY KEY (artifact_id, model_config_id)
	)`).Error; err != nil {
		t.Fatal(err)
	}
	repo := &repository{db: db}
	handler := NewHandler(&service{repo: repo})
	router := gin.New()
	router.POST("/models", handler.CreateModel)
	router.PUT("/models/:id", handler.UpdateModel)
	router.POST("/models/:id/activate", handler.ActivateModel)

	request := func(method, path, body string, code int, detail string) {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var result struct {
			Code int
			Msg  string
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("%s %s: %v: %s", method, path, err, rec.Body)
		}
		if rec.Code != http.StatusOK || result.Code != code || !strings.Contains(result.Msg, detail) {
			t.Fatalf("%s %s: HTTP %d, %s", method, path, rec.Code, rec.Body)
		}
	}

	request("POST", "/models", `{}`, response.InvalidParams, "baseUrl")
	request("POST", "/models", `{"baseUrl":"http://127.0.0.1:11434","modelName":" "}`, response.InvalidParams, "modelName")
	request("POST", "/models", `{"baseUrl":"http://%","modelName":"qa"}`, response.InvalidParams, "invalid URL escape")
	if count, err := repo.CountModels(); err != nil || count != 0 {
		t.Fatalf("invalid create wrote %d rows: %v", count, err)
	}

	request("POST", "/models", `{"apiType":"ollama","baseUrl":"http://127.0.0.1:11434","modelName":"qa"}`, response.OK, "")
	request("POST", "/models", `{"baseUrl":"file:///tmp/model","modelName":"qa","isActive":true}`, response.InvalidParams, "http/https")
	request("PUT", "/models/1", `{"baseUrl":null}`, response.InvalidParams, "baseUrl")
	request("PUT", "/models/1", `{"modelName":false}`, response.InvalidParams, "modelName")
	request("PUT", "/models/1", `{"modelName":" "}`, response.InvalidParams, "modelName")
	request("PUT", "/models/1", `{"modelName":"updated"}`, response.OK, "")

	invalid := ModelConfig{APIType: "openai", BaseURL: "http://127.0.0.1:11434"}
	if err := repo.CreateModel(&invalid); err != nil {
		t.Fatal(err)
	}
	request("POST", "/models/2/activate", `{}`, response.InvalidParams, "modelName")
	active, err := repo.GetActiveModel()
	if err != nil || active.ID != 1 || active.ModelName != "updated" {
		t.Fatalf("active model changed unexpectedly: %+v, %v", active, err)
	}
}

func TestModelConnectionPreservesRequestConstructionError(t *testing.T) {
	router := gin.New()
	router.Use(gin.Recovery())
	router.POST("/test", NewHandler(nil).TestModelStandalone)
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"apiType":"openai","baseUrl":"http://%","modelName":"qa"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	var result struct {
		Code int
		Data struct {
			Success bool
			Status  string
			Message string
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("HTTP %d: %v: %s", rec.Code, err, rec.Body)
	}
	if rec.Code != http.StatusOK || result.Code != response.OK || result.Data.Success ||
		result.Data.Status != "error" || !strings.Contains(result.Data.Message, `parse "http://%/models": invalid URL escape "%"`) {
		t.Fatalf("HTTP %d: %s", rec.Code, rec.Body)
	}
}
