package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestConversationCompressorWaitsForTokenPressureAndCommitsAtomically(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode summary request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"用户要完成代码测试；之前已检查文件，还需验证结果"}}]}`))
	}))
	defer server.Close()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "compactor.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&Message{}, &ModelConfig{}, &ConversationSummary{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE model_scenario_routes (scenario TEXT PRIMARY KEY, model_config_id INTEGER NOT NULL DEFAULT 0)`).Error; err != nil {
		t.Fatal(err)
	}
	cfg := ModelConfig{Name: "test", IsActive: 1, APIType: "openai", Protocol: "openai_chat", BaseURL: server.URL, ModelName: "mock", ContextWindow: 1000, MaxOutputTokens: 200, MaxTokens: 256}
	if err := db.Create(&cfg).Error; err != nil {
		t.Fatal(err)
	}
	add := func(from, to int) {
		t.Helper()
		for i := from; i < to; i++ {
			role := "user"
			if i%2 == 1 {
				role = "assistant"
			}
			m := Message{ID: "msg-" + string(rune('a'+i)), ConversationID: "conv-a", Role: role, Sequence: int64(i + 1), Content: strings.Repeat("content ", 40), IncludeInCtx: 1}
			if err := db.Create(&m).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	compressor := NewCompressor(db)
	add(0, 4)
	compressor.MaybeCompress(context.Background(), "conv-a")
	if requests != 0 {
		t.Fatalf("compressor must not run before threshold; requests=%d", requests)
	}
	var untouched int64
	db.Model(&Message{}).Where("include_in_context = 1").Count(&untouched)
	if untouched != 4 {
		t.Fatalf("prematurely deactivated messages: %d", untouched)
	}
	add(4, 20)
	compressor.MaybeCompress(context.Background(), "conv-a")
	if requests != 1 {
		t.Fatalf("expected one summary request after 95%% token pressure, got %d", requests)
	}
	var summary ConversationSummary
	if err := db.Where("conversation_id = ?", "conv-a").First(&summary).Error; err != nil {
		t.Fatal(err)
	}
	if summary.RoundStart != 1 || summary.RoundEnd < 1 {
		t.Fatalf("invalid compression boundary: %+v", summary)
	}
	var active []Message
	if err := db.Where("conversation_id = ? AND include_in_context = 1", "conv-a").Order("sequence ASC").Find(&active).Error; err != nil {
		t.Fatal(err)
	}
	if len(active) < 8 || len(active) >= 20 {
		t.Fatalf("expected oldest messages compacted with recent history retained, got %d", len(active))
	}
	if active[len(active)-1].Sequence != 20 {
		t.Fatal("latest user conversation must be retained")
	}
	if err := db.Unscoped().Model(&Message{}).Where("conversation_id = ?", "conv-a").Count(&untouched).Error; err != nil {
		t.Fatal(err)
	}
	if untouched != 20 {
		t.Fatalf("original conversation messages must remain in the database, got %d", untouched)
	}
}

func TestOllamaConversationSummaryUsesContextWindowNotOutputTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Options struct {
				Context int `json:"num_ctx"`
				Output  int `json:"num_predict"`
			} `json:"options"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode ollama summary: %v", err)
		}
		if request.Options.Context != 8192 || request.Options.Output != 512 {
			t.Errorf("context and output budgets must be independent: %+v", request.Options)
		}
		_, _ = w.Write([]byte(`{"message":{"content":"summary"}}`))
	}))
	defer server.Close()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "ollama.db")), &gorm.Config{})
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
	if err := db.Create(&ModelConfig{Name: "test", IsActive: 1, APIType: "ollama", BaseURL: server.URL, ModelName: "mock", Temperature: 0.2, MaxTokens: 512, ContextWindow: 8192}).Error; err != nil {
		t.Fatal(err)
	}
	result := NewCompressor(db).generateSummary(context.Background(), "history", "")
	if result != "summary" {
		t.Fatalf("ollama summary not parsed: %q", result)
	}
}
