package chat

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func scenarioRepository(t *testing.T) *repository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "routes.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&ModelConfig{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE model_scenario_routes (scenario TEXT PRIMARY KEY, model_config_id INTEGER NOT NULL DEFAULT 0)`).Error; err != nil {
		t.Fatal(err)
	}
	for _, cfg := range []ModelConfig{{ID: 1, Name: "default", IsActive: 1}, {ID: 2, Name: "assigned"}, {ID: 3, Name: "explicit"}} {
		if err := db.Create(&cfg).Error; err != nil {
			t.Fatal(err)
		}
	}
	return &repository{db: db}
}

func TestRepositoryModelRoutesRoundTrip(t *testing.T) {
	repo := scenarioRepository(t)
	routes, err := repo.GetModelRoutes()
	if err != nil || len(routes) != 3 || routes[0]["modelConfigId"] != 0 {
		t.Fatalf("catalog: %v %v", routes, err)
	}
	if err := repo.UpdateModelRoutes([]map[string]interface{}{{"scenario": "chat", "modelConfigId": 2}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateModelRoutes([]map[string]interface{}{{"scenario": "summary", "modelConfigId": 3}}); err != nil {
		t.Fatal(err)
	}
	model := &service{repo: repo}
	for _, test := range []struct {
		scenario       string
		explicit, want int
	}{{"chat", 0, 2}, {"chat", 3, 3}, {"summary", 0, 3}, {"memory_extract", 0, 1}} {
		cfg, err := model.modelForScenario(test.scenario, test.explicit)
		if err != nil || cfg.ID != test.want {
			t.Fatalf("%+v got %v %v", test, cfg, err)
		}
	}
	invalid := [][]map[string]interface{}{
		{{"scenario": "chat", "modelConfigId": 999}},
		{{"scenario": "safety_rewrite", "modelConfigId": 2}},
		{{"scenario": "chat", "modelConfigId": 1.5}},
		{{"scenario": "chat", "modelConfigId": -1}},
		{{"scenario": "chat", "modelConfigId": 1}, {"scenario": "chat", "modelConfigId": 2}},
		{{"scenario": "chat", "modelConfigId": 1}, {"scenario": "summary", "modelConfigId": 999}},
	}
	for _, update := range invalid {
		if repo.UpdateModelRoutes(update) == nil {
			t.Fatalf("accepted %v", update)
		}
		cfg, err := model.modelForScenario("chat", 0)
		if err != nil || cfg.ID != 2 {
			t.Fatal("failed update changed existing route")
		}
	}
	if err := repo.UpdateModelRoutes([]map[string]interface{}{{"scenario": "chat", "modelConfigId": nil}}); err != nil {
		t.Fatal(err)
	}
	cfg, err := model.modelForScenario("chat", 0)
	if err != nil || cfg.ID != 1 {
		t.Fatal("clear did not restore default")
	}
	if err := repo.DeleteModel(3); err != nil {
		t.Fatal(err)
	}
	cfg, err = model.modelForScenario("summary", 0)
	if err != nil || cfg.ID != 1 {
		t.Fatal("delete did not clear route")
	}
	if _, err := model.modelForScenario("chat", 999); err == nil {
		t.Fatal("invalid explicit model fell back silently")
	}
}

func TestModelScenarioHTTPContract(t *testing.T) {
	repo := scenarioRepository(t)
	handler := NewHandler(&service{repo: repo})
	router := gin.New()
	router.GET("/api/model/routes", handler.GetModelRoutes)
	router.PUT("/api/model/routes", handler.UpdateModelRoutes)
	server := httptest.NewServer(router)
	defer server.Close()
	for _, body := range []string{`{"routes":[{"scenario":"chat","modelConfigId":2}]}`, `{"routes":[{"scenario":"summary","modelConfigId":3}]}`} {
		request, _ := http.NewRequest(http.MethodPut, server.URL+"/api/model/routes", bytes.NewBufferString(body))
		request.Header.Set("Content-Type", "application/json")
		result, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		var payload struct{ Code int }
		err = json.NewDecoder(result.Body).Decode(&payload)
		result.Body.Close()
		if err != nil || payload.Code != 200 {
			t.Fatalf("save: %v %+v", err, payload)
		}
	}
	response, err := http.Get(server.URL + "/api/model/routes")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var payload struct {
		Data []struct {
			Scenario      string
			ModelConfigID int `json:"modelConfigId"`
			Label         string
		}
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data) != 3 || payload.Data[0].ModelConfigID != 2 || payload.Data[1].ModelConfigID != 3 || payload.Data[0].Label == "" {
		t.Fatalf("wire contract: %+v", payload)
	}
}
