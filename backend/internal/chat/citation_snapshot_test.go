package chat

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/knowledge"
	"gorm.io/gorm"
)

func TestPhase17FreezeOnlyUsedKnowledgeAndWebReferences(t *testing.T) {
	r := &ownedCitationAudit{}
	r.registerKnowledgeCitation(1, knowledge.Evidence{
		ID: "ev-1", SourceID: "src-a", SourceRevision: 9,
		UnitID: "u-37", Title: "a.pdf", Locator: map[string]any{"page": 37},
		Checksum: "original-checksum",
	}, strings.Repeat("甲", 2500))
	r.registerKnowledgeCitation(2, knowledge.Evidence{ID: "ev-2", SourceID: "src-b", SourceRevision: 1}, "unused")
	r.captureCitationIDs("web_run", `{"citations":[{"index":1,"evidence_id":"web-1","url":"https://example.org","title":"web","text":"source text"}]}`)
	got := r.BuildCitationSnapshot("引用 [K1] 与 [1]。`[K2]` 只是示例。\n```md\n[K2]\n```\n[K999] 未找到")
	if len(got) != 2 {
		t.Fatalf("expected two used citations only, got %+v", got)
	}
	if got[0].Marker != "[K1]" || got[0].Kind != "knowledge" || got[0].SourceRevision != 9 || got[0].UnitID != "u-37" {
		t.Fatalf("knowledge snapshot incorrect: %+v", got[0])
	}
	if len([]rune(got[0].Excerpt)) != 1200 || got[0].ContentHash == "" {
		t.Fatalf("unbounded/missing excerpt checksum: %+v", got[0])
	}
	if got[1].Kind != "web" || got[1].EvidenceID != "web-1" {
		t.Fatalf("web citation lost: %+v", got[1])
	}
	got[0].Locator["page"] = 99
	snapshots := r.BuildCitationSnapshot("[K1]")
	if snapshots[0].Locator["page"] != 37 {
		t.Fatal("snapshot mutated source locator")
	}
}

func TestPhase17MessageSnapshotDatabaseRoundTrip(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "citations.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Message{}); err != nil {
		t.Fatal(err)
	}
	snapshot := CitationSnapshot{Marker: "[K3]", Kind: "knowledge", Index: 3,
		EvidenceID: "ev-3", SourceID: "source-a", SourceRevision: 3, UnitID: "unit-3",
		Locator: map[string]any{"page": 25}, Excerpt: "a preserved line"}
	message := &Message{ID: "msg-1", ConversationID: "conv-1", Role: "assistant", Content: "answer [K3]",
		CitationSnapshot: []CitationSnapshot{snapshot}, Sequence: 1}
	if err := db.Create(message).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.CitationSnapshotJSON, "unit-3") {
		t.Fatalf("snapshot was not serialized: %s", message.CitationSnapshotJSON)
	}
	var loaded Message
	if err := db.First(&loaded, "id = ?", "msg-1").Error; err != nil {
		t.Fatal(err)
	}
	if len(loaded.CitationSnapshot) != 1 || loaded.CitationSnapshot[0].SourceID != "source-a" {
		t.Fatalf("history did not hydrate snapshot: %+v", loaded.CitationSnapshot)
	}
	body, err := json.Marshal(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"citationSnapshot"`) || strings.Contains(string(body), "citation_snapshot_json") {
		t.Fatalf("history DTO did not expose frozen citations correctly: %s", body)
	}
	if err := db.Model(&Message{}).Where("id = ?", "msg-1").Update("citation_snapshot_json", "broken").Error; err != nil {
		t.Fatal(err)
	}
	var historical Message
	if err := db.First(&historical, "id = ?", "msg-1").Error; err != nil || len(historical.CitationSnapshot) != 0 {
		t.Fatalf("malformed old snapshot should not break history: %v %+v", err, historical.CitationSnapshot)
	}
}
