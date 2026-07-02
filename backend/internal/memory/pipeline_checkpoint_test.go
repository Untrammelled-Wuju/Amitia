package memory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/pipelinecheckpoint"
	"gorm.io/gorm"
)

func TestMemoryProcessAdvancesCheckpointAndSkipsProcessedMessages(t *testing.T) {
	var calls int32
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode llm request: %v", err)
		}
		if len(body.Messages) != 2 {
			t.Fatalf("unexpected llm messages: %#v", body.Messages)
		}
		if body.Messages[1].Content != "user: 我喜欢绿茶\nassistant: 记住了\n" {
			t.Fatalf("unexpected extraction content: %q", body.Messages[1].Content)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": `[{"key":"饮品","value":"喜欢绿茶","memoryType":"preference","importance":5,"confidence":90}]`,
					},
				},
			},
			"usage": map[string]int{"total_tokens": 1},
		})
	}))
	defer llm.Close()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&MemoryCandidateModel{}, &Memory{}, &pipelinecheckpoint.Record{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	stmts := []string{
		`CREATE TABLE conversations (id TEXT PRIMARY KEY, character_id TEXT)`,
		`CREATE TABLE messages (id TEXT PRIMARY KEY, conversation_id TEXT, sequence INTEGER, role TEXT, content TEXT, created_at TEXT)`,
		`CREATE TABLE model_configs (id TEXT PRIMARY KEY, base_url TEXT, api_key TEXT, model_name TEXT, temperature REAL, max_tokens INTEGER, is_active INTEGER)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}
	if err := db.Exec(`INSERT INTO conversations (id, character_id) VALUES (?, ?)`, "conv-1", "char-1").Error; err != nil {
		t.Fatalf("insert conversation: %v", err)
	}
	if err := db.Exec(`INSERT INTO messages (id, conversation_id, sequence, role, content, created_at) VALUES (?, ?, ?, ?, ?, ?)`, "m1", "conv-1", 1, "user", "我喜欢绿茶", "2026-07-02 10:00:00").Error; err != nil {
		t.Fatalf("insert message 1: %v", err)
	}
	if err := db.Exec(`INSERT INTO messages (id, conversation_id, sequence, role, content, created_at) VALUES (?, ?, ?, ?, ?, ?)`, "m2", "conv-1", 2, "assistant", "记住了", "2026-07-02 10:00:01").Error; err != nil {
		t.Fatalf("insert message 2: %v", err)
	}
	if err := db.Exec(`INSERT INTO model_configs (id, base_url, api_key, model_name, temperature, max_tokens, is_active) VALUES (?, ?, ?, ?, ?, ?, ?)`, "model-1", llm.URL, "test-key", "test-model", 0.1, 128, 1).Error; err != nil {
		t.Fatalf("insert model config: %v", err)
	}

	svc := &service{repo: &repository{db: db}, db: db}
	if err := svc.Process(context.Background(), "conv-1", nil, ""); err != nil {
		t.Fatalf("first process: %v", err)
	}
	record, err := pipelinecheckpoint.New(db).Load("conv-1", "memory")
	if err != nil {
		t.Fatalf("load checkpoint: %v", err)
	}
	if record.LastMessageSequence != 2 {
		t.Fatalf("checkpoint sequence = %d, want 2", record.LastMessageSequence)
	}
	var candidateCount int64
	if err := db.Model(&MemoryCandidateModel{}).Count(&candidateCount).Error; err != nil {
		t.Fatalf("count candidates: %v", err)
	}
	if candidateCount != 1 {
		t.Fatalf("candidate count = %d, want 1", candidateCount)
	}
	if err := svc.Process(context.Background(), "conv-1", nil, ""); err != nil {
		t.Fatalf("second process: %v", err)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("llm calls = %d, want 1", calls)
	}
	if err := db.Model(&MemoryCandidateModel{}).Count(&candidateCount).Error; err != nil {
		t.Fatalf("count candidates after second process: %v", err)
	}
	if candidateCount != 1 {
		t.Fatalf("candidate count after second process = %d, want 1", candidateCount)
	}
}
