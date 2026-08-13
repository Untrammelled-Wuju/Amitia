package character

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB.Close()
	})
	if err := db.AutoMigrate(&Character{}, &CharacterTemplate{}); err != nil {
		t.Fatal(err)
	}
	db.Exec(`CREATE TABLE IF NOT EXISTS conversations (id text, character_id text, title text, channel text, source text, created_at text, updated_at text)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS proactive_rules (id integer primary key, name text, enabled integer, channel text, character_id text, rule_type text, schedule_cron text, max_per_day integer, prompt_template text, random_minutes integer, created_at text, updated_at text)`)
	return db
}

func TestE2ECharacterReadAfterWrite(t *testing.T) {
	db := newTestDB(t)
	ctx := app.NewAppContext(db, nil)
	repo := NewRepository(ctx)
	svc := NewService(repo, ctx)

	empty, err := svc.List(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected 0 characters, got %d", len(empty))
	}

	marker := "amitia-runtime-e2e-" + t.Name()
	created, err := svc.Create(&CreateCharacterRequest{
		Name:     marker,
		Identity: "E2E测试角色",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatal("created character has empty ID")
	}
	if created.Name != marker {
		t.Fatalf("expected name %q, got %q", marker, created.Name)
	}

	list, err := svc.List(false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range list {
		if c.ID == created.ID && c.Name == marker {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("created character not found in list after write")
	}

	got, err := svc.GetByID(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != marker || got.Identity != "E2E测试角色" {
		t.Fatalf("getById mismatch: %+v", got)
	}
}

func TestE2ECharacterStorageReopen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "reopen.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Character{}, &CharacterTemplate{}); err != nil {
		t.Fatal(err)
	}
	db.Exec(`CREATE TABLE IF NOT EXISTS conversations (id text, character_id text, title text, channel text, source text, created_at text, updated_at text)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS proactive_rules (id integer primary key, name text, enabled integer, channel text, character_id text, rule_type text, schedule_cron text, max_per_day integer, prompt_template text, random_minutes integer, created_at text, updated_at text)`)
	ctx := app.NewAppContext(db, nil)
	repo := NewRepository(ctx)
	svc := NewService(repo, ctx)

	marker := "amitia-runtime-e2e-persist-" + t.Name()
	created, err := svc.Create(&CreateCharacterRequest{
		Name:     marker,
		Identity: "持久化测试角色",
	})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	db2, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB2, err := db2.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB2.Close()
	})
	ctx2 := app.NewAppContext(db2, nil)
	repo2 := NewRepository(ctx2)
	svc2 := NewService(repo2, ctx2)

	list, err := svc2.List(true)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range list {
		if c.ID == created.ID && c.Name == marker {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("character not found after storage reopen")
	}
}

func TestE2ECharacterHandlerReadAndWrite(t *testing.T) {
	db := newTestDB(t)
	ctx := app.NewAppContext(db, nil)
	repo := NewRepository(ctx)
	svc := NewService(repo, ctx)
	handler := NewHandler(svc)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	r := router.Group("/api")
	r.GET("/characters", handler.List)
	r.GET("/characters/:id", handler.Get)
	r.POST("/characters", handler.Create)
	r.PUT("/characters/:id", handler.Update)
	r.DELETE("/characters/:id", handler.Delete)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/characters", nil)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("List expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var listResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if listResp["code"].(float64) != 200 {
		t.Fatalf("List expected code 200, got %v", listResp["code"])
	}

	marker := "amitia-runtime-e2e-handler-" + t.Name()
	createBody := map[string]interface{}{
		"name":     marker,
		"identity": "Handler测试角色",
	}
	bodyBytes, _ := json.Marshal(createBody)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/characters", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("Create expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var createResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &createResp); err != nil {
		t.Fatal(err)
	}
	if createResp["code"].(float64) != 200 {
		t.Fatalf("Create expected code 200, got %v", createResp["code"])
	}
	dataMap, ok := createResp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Create response missing data: %v", createResp)
	}
	createdID, ok := dataMap["id"].(string)
	if !ok || createdID == "" {
		t.Fatalf("Create response missing id: %v", createResp)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/characters/"+createdID, nil)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("Get expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var getResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &getResp); err != nil {
		t.Fatal(err)
	}
	getData, ok := getResp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Get response missing data: %v", getResp)
	}
	if getData["name"] != marker {
		t.Fatalf("expected name %q, got %v", marker, getData["name"])
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/characters", nil)
	router.ServeHTTP(w, req)
	var listResp2 map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &listResp2); err != nil {
		t.Fatal(err)
	}
	listData, ok := listResp2["data"].([]interface{})
	if !ok {
		t.Fatalf("List response missing data array: %v", listResp2)
	}
	found := false
	for _, item := range listData {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if m["id"] == createdID && m["name"] == marker {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("created character not found in list after Get")
	}
}

func TestE2ECharacterValidation(t *testing.T) {
	db := newTestDB(t)
	ctx := app.NewAppContext(db, nil)
	repo := NewRepository(ctx)
	svc := NewService(repo, ctx)
	handler := NewHandler(svc)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	r := router.Group("/api")
	r.POST("/characters", handler.Create)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/characters", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("Expected 200 (gin wraps error in JSON), got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["code"].(float64) == 200 {
		t.Fatalf("Expected non-200 code for invalid request, got 200")
	}
}

func TestE2ECharacterDelete(t *testing.T) {
	db := newTestDB(t)
	ctx := app.NewAppContext(db, nil)
	repo := NewRepository(ctx)
	svc := NewService(repo, ctx)

	created, err := svc.Create(&CreateCharacterRequest{
		Name:     "amitia-runtime-e2e-delete-" + t.Name(),
		Identity: "待删除角色",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = svc.Delete(created.ID)
	if err != nil {
		t.Fatal(err)
	}

	got, err := svc.GetByID(created.ID)
	if err == nil && got != nil {
		t.Fatalf("expected deleted character to not be found")
	}
}
