package webresearch

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCitationRegistryForTurnIsStableAndSorted(t *testing.T) {
	store := NewStore(nil)
	ctx := context.Background()
	first, err := store.GetOrAssignCitationNumber(ctx, "turn-1", "ref-b")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.GetOrAssignCitationNumber(ctx, "turn-1", "ref-a")
	if err != nil {
		t.Fatal(err)
	}
	again, err := store.GetOrAssignCitationNumber(ctx, "turn-1", "ref-b")
	if err != nil {
		t.Fatal(err)
	}
	if first != 1 || second != 2 || again != first {
		t.Fatalf("citation assignment = %d,%d,%d", first, second, again)
	}
	items, err := store.CitationRegistryForTurn(ctx, "turn-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].CitationNumber != 1 || items[0].RefID != "ref-b" || items[1].CitationNumber != 2 {
		t.Fatalf("registry = %+v", items)
	}
}

func TestCleanupExpiredRemovesInMemoryCitationBindings(t *testing.T) {
	store := NewStore(nil)
	ctx := context.Background()
	now := time.Now().UTC()
	ref := Reference{RefID: "ref-expired", ConversationID: "conv", Kind: "page", URL: "https://example.com", CanonicalURL: "https://example.com", CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Minute)}
	if err := store.PutReference(ctx, ref); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetOrAssignCitationNumber(ctx, "turn-1", ref.RefID); err != nil {
		t.Fatal(err)
	}
	if err := store.MaybeCleanupExpired(ctx, now); err != nil {
		t.Fatal(err)
	}
	items, err := store.CitationRegistryForTurn(ctx, "turn-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("stale citation bindings remain: %+v", items)
	}
}

func newSQLiteWebResearchStore(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	name := strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	gormDB, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db, err := gormDB.DB()
	if err != nil {
		t.Fatalf("unwrap sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewStore(db), db
}

func TestEnsureSchemaFreshInstallIsIdempotent(t *testing.T) {
	store, db := newSQLiteWebResearchStore(t)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("fresh schema: %v", err)
	}
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("repeat schema: %v", err)
	}
	for _, table := range []string{"web_references", "web_pages", "web_page_versions", "web_evidence", "web_turn_citations", "web_turn_citation_evidence"} {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil {
			t.Fatalf("inspect %s: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("table %s count=%d, want 1", table, count)
		}
	}
}

func TestEnsureSchemaCompletesPartialUpgradeWithoutTouchingExistingChatData(t *testing.T) {
	store, db := newSQLiteWebResearchStore(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `CREATE TABLE messages (id TEXT PRIMARY KEY, content TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO messages(id, content) VALUES('m1', 'keep me')`); err != nil {
		t.Fatal(err)
	}
	// Simulate an interrupted earlier upgrade where only the first web table exists.
	if _, err := db.ExecContext(ctx, `CREATE TABLE web_references (ref_id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL, turn_id TEXT, invocation_id TEXT, kind TEXT NOT NULL, url TEXT NOT NULL, canonical_url TEXT NOT NULL, title TEXT, snippet TEXT, provider TEXT, query_text TEXT, rank_value INTEGER NOT NULL DEFAULT 0, published_at DATETIME, created_at DATETIME NOT NULL, expires_at DATETIME NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("resume schema: %v", err)
	}
	var content string
	if err := db.QueryRowContext(ctx, `SELECT content FROM messages WHERE id='m1'`).Scan(&content); err != nil {
		t.Fatalf("read pre-existing chat row: %v", err)
	}
	if content != "keep me" {
		t.Fatalf("existing chat data changed: %q", content)
	}
	for _, table := range []string{"web_pages", "web_page_versions", "web_evidence", "web_turn_citations", "web_turn_citation_evidence"} {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("missing resumed table %s", table)
		}
	}
}

func TestCitationRegistryRecoversAfterStoreRestart(t *testing.T) {
	store, db := newSQLiteWebResearchStore(t)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	ref := Reference{RefID: "ref-restart", ConversationID: "conv", Kind: "page", URL: "https://example.com/a", CanonicalURL: "https://example.com/a", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := store.PutReference(ctx, ref); err != nil {
		t.Fatal(err)
	}
	number, err := store.GetOrAssignCitationNumber(ctx, "turn-restart", ref.RefID)
	if err != nil {
		t.Fatal(err)
	}
	if number != 1 {
		t.Fatalf("citation number=%d, want 1", number)
	}

	restarted := NewStore(db)
	if err := restarted.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	items, err := restarted.CitationRegistryForTurn(ctx, "turn-restart")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].RefID != ref.RefID || items[0].CitationNumber != number {
		t.Fatalf("recovered registry=%+v", items)
	}
	again, err := restarted.GetOrAssignCitationNumber(ctx, "turn-restart", ref.RefID)
	if err != nil {
		t.Fatal(err)
	}
	if again != number {
		t.Fatalf("citation number changed after restart: %d -> %d", number, again)
	}
}

func TestCleanupExpiredRemovesDurableWebArtifacts(t *testing.T) {
	store, db := newSQLiteWebResearchStore(t)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	ref := Reference{RefID: "ref-old", ConversationID: "conv", Kind: "page", URL: "https://example.com/old", CanonicalURL: "https://example.com/old", CreatedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(-time.Minute)}
	page := Page{RefID: ref.RefID, SourceRefID: ref.RefID, ConversationID: "conv", URL: ref.URL, CanonicalURL: ref.CanonicalURL, ContentType: "text/plain", Content: "expired evidence text", ContentHash: "hash-old", FetchedAt: now.Add(-time.Hour)}
	evidence := Evidence{ID: "ev-old", ConversationID: "conv", PageRefID: ref.RefID, Query: "old", Text: "expired evidence text", TextHash: "ev-hash-old", Locator: EvidenceLocator{Kind: "html_block", BlockIndex: 0}, Relevance: 1, CreatedAt: now.Add(-time.Hour)}
	if err := store.PutPageArtifact(ctx, page, ref, []Evidence{evidence}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetOrAssignCitationNumber(ctx, "turn-old", ref.RefID); err != nil {
		t.Fatal(err)
	}
	if err := store.MaybeCleanupExpired(ctx, now); err != nil {
		t.Fatal(err)
	}
	for table, want := range map[string]int{"web_references": 0, "web_pages": 0, "web_page_versions": 0, "web_evidence": 0, "web_turn_citations": 0, "web_turn_citation_evidence": 0} {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != want {
			t.Fatalf("%s count=%d, want %d", table, count, want)
		}
	}
}

func TestEnsureSchemaMigratesEvidenceContentHashColumnWithoutLosingRows(t *testing.T) {
	store, db := newSQLiteWebResearchStore(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `CREATE TABLE web_evidence (evidence_id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL, page_ref_id TEXT NOT NULL, query_text TEXT, text_value TEXT NOT NULL, text_hash TEXT NOT NULL, locator_json TEXT NOT NULL, relevance REAL NOT NULL DEFAULT 0, created_at DATETIME NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO web_evidence(evidence_id, conversation_id, page_ref_id, query_text, text_value, text_hash, locator_json, relevance, created_at) VALUES('old-ev','conv','page','q','old text','th','{}',1,?)`, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}
	var hash, text string
	if err := db.QueryRowContext(ctx, `SELECT page_content_hash, text_value FROM web_evidence WHERE evidence_id='old-ev'`).Scan(&hash, &text); err != nil {
		t.Fatalf("read migrated evidence: %v", err)
	}
	if hash != "" || text != "old text" {
		t.Fatalf("migrated row changed unexpectedly: hash=%q text=%q", hash, text)
	}
}

func TestEvidenceBindsToImmutablePageVersion(t *testing.T) {
	store, _ := newSQLiteWebResearchStore(t)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	ref := Reference{RefID: "ref-versioned", ConversationID: "conv", Kind: "page", URL: "https://example.com/doc", CanonicalURL: "https://example.com/doc", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	pageV1 := Page{RefID: ref.RefID, SourceRefID: ref.RefID, ConversationID: "conv", URL: ref.URL, CanonicalURL: ref.CanonicalURL, ContentType: "text/plain", Content: "version one", ContentHash: "hash-v1", FetchedAt: now}
	evV1 := Evidence{ID: "ev-v1", ConversationID: "conv", PageRefID: ref.RefID, Query: "version", Text: "version one", TextHash: "ev-hash-v1", Locator: EvidenceLocator{Kind: "html_block"}, Relevance: 1, CreatedAt: now}
	if err := store.PutPageArtifact(ctx, pageV1, ref, []Evidence{evV1}); err != nil {
		t.Fatal(err)
	}
	pageV2 := pageV1
	pageV2.Content = "version two"
	pageV2.ContentHash = "hash-v2"
	pageV2.FetchedAt = now.Add(time.Minute)
	evV2 := Evidence{ID: "ev-v2", ConversationID: "conv", PageRefID: ref.RefID, Query: "version", Text: "version two", TextHash: "ev-hash-v2", Locator: EvidenceLocator{Kind: "html_block"}, Relevance: 1, CreatedAt: now.Add(time.Minute)}
	if err := store.PutPageArtifact(ctx, pageV2, ref, []Evidence{evV2}); err != nil {
		t.Fatal(err)
	}

	versions, err := store.PageVersions(ctx, ref.RefID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 || versions[0].ContentHash != "hash-v1" || versions[1].ContentHash != "hash-v2" {
		t.Fatalf("versions=%+v", versions)
	}
	v1Evidence, err := store.EvidenceForPageVersionQuery(ctx, "conv", ref.RefID, "hash-v1", "version")
	if err != nil {
		t.Fatal(err)
	}
	v2Evidence, err := store.EvidenceForPageVersionQuery(ctx, "conv", ref.RefID, "hash-v2", "version")
	if err != nil {
		t.Fatal(err)
	}
	if len(v1Evidence) != 1 || v1Evidence[0].ID != "ev-v1" || v1Evidence[0].PageContentHash != "hash-v1" {
		t.Fatalf("v1 evidence=%+v", v1Evidence)
	}
	if len(v2Evidence) != 1 || v2Evidence[0].ID != "ev-v2" || v2Evidence[0].PageContentHash != "hash-v2" {
		t.Fatalf("v2 evidence=%+v", v2Evidence)
	}
}

func TestCitationEvidenceBindingsRecoverAfterRestart(t *testing.T) {
	store, db := newSQLiteWebResearchStore(t)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	ref := Reference{RefID: "ref-evidence", ConversationID: "conv", Kind: "page", URL: "https://example.com/evidence", CanonicalURL: "https://example.com/evidence", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := store.PutReference(ctx, ref); err != nil {
		t.Fatal(err)
	}
	number, err := store.GetOrAssignCitationNumber(ctx, "turn-evidence", ref.RefID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.BindCitationEvidence(ctx, "turn-evidence", number, ref.RefID, "ev-1"); err != nil {
		t.Fatal(err)
	}
	if err := store.BindCitationEvidence(ctx, "turn-evidence", number, ref.RefID, "ev-2"); err != nil {
		t.Fatal(err)
	}

	restarted := NewStore(db)
	if err := restarted.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	items, err := restarted.CitationEvidenceForTurn(ctx, "turn-evidence")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].CitationNumber != number || items[0].EvidenceID != "ev-1" || items[1].EvidenceID != "ev-2" {
		t.Fatalf("citation evidence registry=%+v", items)
	}
}

func TestCleanupExpiredRemovesCitationEvidenceBindings(t *testing.T) {
	store, db := newSQLiteWebResearchStore(t)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	ref := Reference{RefID: "ref-expired-evidence", ConversationID: "conv", Kind: "page", URL: "https://example.com/expired", CanonicalURL: "https://example.com/expired", CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Minute)}
	if err := store.PutReference(ctx, ref); err != nil {
		t.Fatal(err)
	}
	number, err := store.GetOrAssignCitationNumber(ctx, "turn-expired-evidence", ref.RefID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.BindCitationEvidence(ctx, "turn-expired-evidence", number, ref.RefID, "ev-expired"); err != nil {
		t.Fatal(err)
	}
	if err := store.MaybeCleanupExpired(ctx, now); err != nil {
		t.Fatal(err)
	}
	items, err := store.CitationEvidenceForTurn(ctx, "turn-expired-evidence")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("stale citation evidence bindings remain: %+v", items)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM web_turn_citation_evidence`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("durable citation evidence count=%d, want 0", count)
	}
}
