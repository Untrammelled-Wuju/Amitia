package chat

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/knowledge"
	"gorm.io/gorm"
)

func TestCitationSnapshotRestrictsUntrustedSourceMetadata(t *testing.T) {
	recorder := &ownedCitationAudit{}
	locator := map[string]any{
		"page": 42, "row": float64(8), "sheet": "2", "kind": "pdf_page",
		"path": "/home/user/secrets/notes.pdf", "accessToken": "private", "metadata": map[string]any{"password": "private"},
		"field": "C:\\private\\key", "injected": strings.Repeat("X", 10000),
	}
	recorder.registerKnowledgeCitation(1, knowledge.Evidence{ID: "e1", SourceID: "source-a", Locator: locator}, "confirmed evidence")
	locator["page"] = 9000
	recorder.captureCitationIDs("web_run", `{"citations":[{"index":1,"url":"https://example.com/article?q=ok&api_key=secret&access_token=private#part","title":"Public"},{"index":2,"url":"file:///tmp/private","title":"Private"}]}`)
	got := recorder.BuildCitationSnapshot("Document [K1]. Website [1]. Unsafe link [2].")
	if len(got) != 3 {
		t.Fatalf("expected three citations, got %+v", got)
	}
	if got[0].Locator["page"] != 42 || got[0].Locator["row"] != 8 || got[0].Locator["sheet"] != 2 {
		t.Fatalf("safe canonical locator was lost: %+v", got[0].Locator)
	}
	if _, exposed := got[0].Locator["path"]; exposed {
		t.Fatalf("absolute path leaked into snapshot: %+v", got[0].Locator)
	}
	if _, exposed := got[0].Locator["field"]; exposed {
		t.Fatalf("sensitive field locator leaked: %+v", got[0].Locator)
	}
	if got[1].URL != "https://example.com/article?q=ok#part" || got[2].URL != "" {
		t.Fatalf("unsafe citation URL was retained: %+v", got)
	}
}

func TestCitationSnapshotDefensiveLegacyHydration(t *testing.T) {
	entries := []CitationSnapshot{
		{Marker: "[K1]", Kind: "knowledge", Index: 1, URL: "javascript:alert(1)", Locator: map[string]any{"path": "/private", "page": 9}, Title: strings.Repeat("t", 300), Excerpt: strings.Repeat("字", 2000)},
		{Marker: "[K1]", Kind: "knowledge", Index: 1, Excerpt: "duplicate"},
		{Marker: "[K9]", Kind: "knowledge", Index: 5, Excerpt: "inconsistent"},
	}
	encoded, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	message := &Message{CitationSnapshotJSON: string(encoded)}
	if err := message.AfterFind(nil); err != nil {
		t.Fatal(err)
	}
	if len(message.CitationSnapshot) != 1 || message.CitationSnapshot[0].URL != "" || len(message.CitationSnapshot[0].Locator) != 1 {
		t.Fatalf("legacy snapshot not sanitized: %+v", message.CitationSnapshot)
	}
	if len([]rune(message.CitationSnapshot[0].Excerpt)) != 1200 || len([]rune(message.CitationSnapshot[0].Title)) != 200 {
		t.Fatal("legacy snapshot exceeded output size bounds")
	}
	message.CitationSnapshotJSON = strings.Repeat("!", maxCitationSnapshotJSONBytes+1)
	if err := message.AfterFind(nil); err != nil || len(message.CitationSnapshot) != 0 {
		t.Fatal("oversized legacy snapshot should be dropped without blocking history")
	}
}

func TestCitationSnapshotsEnforceConversationSpaceOwner(t *testing.T) {
	svc := newProjectTestService(t)
	previousConfig := config.AppCfg
	config.AppCfg = &config.Config{Security: config.SecurityRuntimeConfig{Mode: "cloud"}}
	t.Cleanup(func() { config.AppCfg = previousConfig })
	for _, conv := range []Conversation{
		{ID: "conversation-A", SpaceID: "space-A", Title: "Owner A", Channel: "web"},
		{ID: "conversation-B", SpaceID: "space-B", Title: "Owner B", Channel: "web"},
	} {
		if err := svc.db.Create(&conv).Error; err != nil {
			t.Fatal(err)
		}
	}
	message := &Message{ID: "cited-msg-A", ConversationID: "conversation-A", Role: "assistant", Content: "Answer [K1]", CitationSnapshot: []CitationSnapshot{
		{Marker: "[K1]", Kind: "knowledge", Index: 1, SourceID: "knowledge-owned-by-A", Excerpt: "private evidence"},
	}}
	if err := svc.repo.CreateMessage(message); err != nil {
		t.Fatal(err)
	}
	rows, total, err := svc.GetMessagesForSpace("conversation-A", "space-A", 1, 50)
	if err != nil || total != 1 || len(rows) != 1 || len(rows[0].CitationSnapshot) != 1 {
		t.Fatalf("owner should retain historical citations: total=%d rows=%+v err=%v", total, rows, err)
	}
	if _, _, err := svc.GetMessagesForSpace("conversation-A", "space-B", 1, 50); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("foreign space accessed historical citations: %v", err)
	}
	if _, _, err := svc.GetMessagesBeforeForSpace("conversation-A", "space-B", 0, 50); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("foreign space accessed paginated citations: %v", err)
	}
}

func TestCitationMarkersIgnoreFencedAndVariableBacktickCode(t *testing.T) {
	recorder := &ownedCitationAudit{}
	for index := 1; index <= 8; index++ {
		recorder.registerKnowledgeCitation(index, knowledge.Evidence{ID: "ev"}, "evidence")
	}
	markdown := "Visible [K1] and escaped \\[K2]. ` [K3] ` and `` [K4] ``.\n" +
		"````md\n[K5]\n```\n[K6]\n````\n" +
		"    [K7]\nFinal [K8]"
	got := recorder.BuildCitationSnapshot(markdown)
	if len(got) != 2 || got[0].Marker != "[K1]" || got[1].Marker != "[K8]" {
		t.Fatalf("unexpected citation markers from code blocks: %+v", got)
	}
}

func TestPhase20SpreadsheetAndDocumentLocatorRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input map[string]any
		want  map[string]any
	}{
		{name: "excel", input: map[string]any{"sheetName": "年度汇总 2026", "rowStart": float64(146), "rowEnd": 147, "path": "/etc/private"}, want: map[string]any{"sheet": "年度汇总 2026", "row": 146}},
		{name: "numeric-sheet-name", input: map[string]any{"sheetName": "2026", "rowStart": 32}, want: map[string]any{"sheet": "2026", "row": 32}},
		{name: "pdf", input: map[string]any{"pageNumber": float64(37), "password": "private"}, want: map[string]any{"page": 37}},
		{name: "docx", input: map[string]any{"paragraphIndex": int64(86), "field": "body"}, want: map[string]any{"paragraph": 86, "field": "body"}},
		{name: "unsafe-sheet", input: map[string]any{"sheetName": "C:\\private\\file", "rowStart": 2}, want: map[string]any{"row": 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &ownedCitationAudit{}
			recorder.registerKnowledgeCitation(1, knowledge.Evidence{ID: "ev", SourceID: "src", Locator: tc.input}, "evidence")
			snapshot := recorder.BuildCitationSnapshot("According to [K1]")
			if len(snapshot) != 1 || !reflect.DeepEqual(snapshot[0].Locator, tc.want) {
				t.Fatalf("locator lost or leaked: %+v want %+v", snapshot, tc.want)
			}
			encoded, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			message := Message{CitationSnapshotJSON: string(encoded)}
			if err := message.AfterFind(nil); err != nil || len(message.CitationSnapshot) != 1 || !reflect.DeepEqual(message.CitationSnapshot[0].Locator, tc.want) {
				t.Fatalf("stored locator not stable: %+v err %v", message.CitationSnapshot, err)
			}
		})
	}
}
