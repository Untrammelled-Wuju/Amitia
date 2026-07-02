package memory

import (
	"encoding/json"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLogRetrievalPersistsRequestScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`
CREATE TABLE retrieval_logs (
	id TEXT PRIMARY KEY,
	conversation_id TEXT,
	character_id TEXT,
	request_id TEXT,
	channel TEXT,
	query_text TEXT,
	retrieved_memory_ids TEXT,
	scoring_details TEXT,
	created_at TEXT
)`).Error; err != nil {
		t.Fatalf("create retrieval_logs: %v", err)
	}

	svc := &service{db: db}
	results := []HybridSearchResult{
		{
			Memory: Memory{
				ID:         "mem-1",
				MemoryType: "profile",
			},
			Score:       0.91,
			MatchType:   "hybrid",
			MemoryLayer: "long_term",
		},
	}

	svc.logRetrieval("conv-1", "char-1", "req-1", "web", "hello", []string{"mem-1"}, results)

	var row struct {
		ConversationID     string
		CharacterID        string
		RequestID          string
		Channel            string
		QueryText          string
		RetrievedMemoryIDs string
		ScoringDetails     string
	}
	if err := db.Table("retrieval_logs").Select("conversation_id, character_id, request_id, channel, query_text, retrieved_memory_ids, scoring_details").Scan(&row).Error; err != nil {
		t.Fatalf("read retrieval log: %v", err)
	}
	if row.ConversationID != "conv-1" || row.CharacterID != "char-1" || row.RequestID != "req-1" || row.Channel != "web" {
		t.Fatalf("unexpected scope: conversation=%q character=%q request=%q channel=%q", row.ConversationID, row.CharacterID, row.RequestID, row.Channel)
	}
	if row.QueryText != "hello" {
		t.Fatalf("unexpected query text: %q", row.QueryText)
	}

	var memoryIDs []string
	if err := json.Unmarshal([]byte(row.RetrievedMemoryIDs), &memoryIDs); err != nil {
		t.Fatalf("decode memory ids: %v", err)
	}
	if len(memoryIDs) != 1 || memoryIDs[0] != "mem-1" {
		t.Fatalf("unexpected memory ids: %#v", memoryIDs)
	}

	var details []map[string]interface{}
	if err := json.Unmarshal([]byte(row.ScoringDetails), &details); err != nil {
		t.Fatalf("decode scoring details: %v", err)
	}
	if len(details) != 1 || details[0]["id"] != "mem-1" {
		t.Fatalf("unexpected scoring details: %#v", details)
	}
}
