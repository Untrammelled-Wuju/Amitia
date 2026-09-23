package webresearch

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

const upsertPageVersionSQL = `INSERT INTO web_page_versions (ref_id, content_hash, title, content_type, content, links_json, truncated, dynamic, first_seen_at, last_seen_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(ref_id, content_hash) DO UPDATE SET title=excluded.title, content_type=excluded.content_type, content=excluded.content, links_json=excluded.links_json, truncated=excluded.truncated, dynamic=excluded.dynamic, last_seen_at=excluded.last_seen_at`

type Store struct {
	db               *sql.DB
	mu               sync.RWMutex
	cleanupMu        sync.Mutex
	citationMu       sync.Mutex
	lastCleanup      time.Time
	references       map[string]Reference
	pages            map[string]Page
	pageVersions     map[string]map[string]PageVersion
	evidence         map[string]Evidence
	citationNumbers  map[string]map[string]int
	citationEvidence map[string]map[string]TurnCitationEvidenceBinding
}

func NewStore(db *sql.DB) *Store {
	return &Store{
		db:               db,
		references:       make(map[string]Reference),
		pages:            make(map[string]Page),
		pageVersions:     make(map[string]map[string]PageVersion),
		evidence:         make(map[string]Evidence),
		citationNumbers:  make(map[string]map[string]int),
		citationEvidence: make(map[string]map[string]TurnCitationEvidenceBinding),
	}
}

func (s *Store) EnsureSchema(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS web_references (ref_id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL, turn_id TEXT, invocation_id TEXT, kind TEXT NOT NULL, url TEXT NOT NULL, canonical_url TEXT NOT NULL, title TEXT, snippet TEXT, provider TEXT, query_text TEXT, rank_value INTEGER NOT NULL DEFAULT 0, published_at DATETIME, created_at DATETIME NOT NULL, expires_at DATETIME NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_web_references_conversation ON web_references(conversation_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_web_references_canonical ON web_references(canonical_url)`,
		`CREATE TABLE IF NOT EXISTS web_pages (ref_id TEXT PRIMARY KEY, source_ref_id TEXT, conversation_id TEXT NOT NULL, url TEXT NOT NULL, canonical_url TEXT NOT NULL, title TEXT, content_type TEXT NOT NULL, content TEXT NOT NULL, content_hash TEXT NOT NULL, links_json TEXT NOT NULL, truncated INTEGER NOT NULL DEFAULT 0, dynamic INTEGER NOT NULL DEFAULT 0, fetched_at DATETIME NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_web_pages_conversation ON web_pages(conversation_id, fetched_at)`,
		`CREATE INDEX IF NOT EXISTS idx_web_pages_hash ON web_pages(content_hash)`,
		`CREATE TABLE IF NOT EXISTS web_page_versions (ref_id TEXT NOT NULL, content_hash TEXT NOT NULL, title TEXT, content_type TEXT NOT NULL, content TEXT NOT NULL, links_json TEXT NOT NULL, truncated INTEGER NOT NULL DEFAULT 0, dynamic INTEGER NOT NULL DEFAULT 0, first_seen_at DATETIME NOT NULL, last_seen_at DATETIME NOT NULL, PRIMARY KEY(ref_id, content_hash))`,
		`CREATE INDEX IF NOT EXISTS idx_web_page_versions_ref ON web_page_versions(ref_id, first_seen_at)`,
		`CREATE TABLE IF NOT EXISTS web_evidence (evidence_id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL, page_ref_id TEXT NOT NULL, page_content_hash TEXT NOT NULL DEFAULT '', query_text TEXT, text_value TEXT NOT NULL, text_hash TEXT NOT NULL, locator_json TEXT NOT NULL, relevance REAL NOT NULL DEFAULT 0, created_at DATETIME NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_web_evidence_page ON web_evidence(page_ref_id, relevance DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_web_evidence_conversation ON web_evidence(conversation_id, created_at)`,
		`CREATE TABLE IF NOT EXISTS web_turn_citations (turn_id TEXT NOT NULL, ref_id TEXT NOT NULL, citation_number INTEGER NOT NULL, created_at DATETIME NOT NULL, PRIMARY KEY(turn_id, ref_id), UNIQUE(turn_id, citation_number))`,
		`CREATE INDEX IF NOT EXISTS idx_web_turn_citations_turn ON web_turn_citations(turn_id, citation_number)`,
		`CREATE TABLE IF NOT EXISTS web_turn_citation_evidence (turn_id TEXT NOT NULL, citation_number INTEGER NOT NULL, ref_id TEXT NOT NULL, evidence_id TEXT NOT NULL, created_at DATETIME NOT NULL, PRIMARY KEY(turn_id, evidence_id))`,
		`CREATE INDEX IF NOT EXISTS idx_web_turn_citation_evidence_number ON web_turn_citation_evidence(turn_id, citation_number)`,
		`CREATE INDEX IF NOT EXISTS idx_web_turn_citation_evidence_ref ON web_turn_citation_evidence(ref_id)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	if err := ensureEvidencePageContentHashColumn(ctx, s.db); err != nil {
		return err
	}
	return nil
}

func ensureEvidencePageContentHashColumn(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(web_evidence)`)
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, pk int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if strings.EqualFold(strings.TrimSpace(name), "page_content_hash") {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = db.ExecContext(ctx, `ALTER TABLE web_evidence ADD COLUMN page_content_hash TEXT NOT NULL DEFAULT ''`)
	return err
}

func (s *Store) MaybeCleanupExpired(ctx context.Context, now time.Time) error {
	if s == nil {
		return nil
	}
	now = now.UTC()
	s.cleanupMu.Lock()
	defer s.cleanupMu.Unlock()
	if !s.lastCleanup.IsZero() && now.Sub(s.lastCleanup) < 10*time.Minute {
		return nil
	}

	expiredRefs := make(map[string]struct{})
	s.mu.Lock()
	for refID, ref := range s.references {
		if !ref.ExpiresAt.IsZero() && !ref.ExpiresAt.After(now) {
			expiredRefs[refID] = struct{}{}
			delete(s.references, refID)
		}
	}
	for refID, page := range s.pages {
		_, pageExpired := expiredRefs[refID]
		_, sourceExpired := expiredRefs[page.SourceRefID]
		if pageExpired || sourceExpired {
			delete(s.pages, refID)
			delete(s.pageVersions, refID)
			for evidenceID, item := range s.evidence {
				if item.PageRefID == refID {
					delete(s.evidence, evidenceID)
				}
			}
		}
	}
	if len(expiredRefs) > 0 {
		for turnID, byRef := range s.citationNumbers {
			for refID := range byRef {
				if _, expired := expiredRefs[refID]; expired {
					delete(byRef, refID)
				}
			}
			if len(byRef) == 0 {
				delete(s.citationNumbers, turnID)
			}
		}
		for turnID, byEvidence := range s.citationEvidence {
			for evidenceID, binding := range byEvidence {
				if _, expired := expiredRefs[binding.RefID]; expired {
					delete(byEvidence, evidenceID)
				}
			}
			if len(byEvidence) == 0 {
				delete(s.citationEvidence, turnID)
			}
		}
	}
	s.mu.Unlock()

	if s.db == nil {
		s.lastCleanup = now
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM web_evidence WHERE page_ref_id IN (
		SELECT p.ref_id
		FROM web_pages p
		LEFT JOIN web_references page_ref ON page_ref.ref_id = p.ref_id
		LEFT JOIN web_references source_ref ON source_ref.ref_id = p.source_ref_id
		WHERE (page_ref.expires_at IS NOT NULL AND page_ref.expires_at <= ?)
		   OR (source_ref.expires_at IS NOT NULL AND source_ref.expires_at <= ?)
	)`, now, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM web_page_versions WHERE ref_id IN (
		SELECT p.ref_id
		FROM web_pages p
		LEFT JOIN web_references page_ref ON page_ref.ref_id = p.ref_id
		LEFT JOIN web_references source_ref ON source_ref.ref_id = p.source_ref_id
		WHERE (page_ref.expires_at IS NOT NULL AND page_ref.expires_at <= ?)
		   OR (source_ref.expires_at IS NOT NULL AND source_ref.expires_at <= ?)
	)`, now, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM web_pages WHERE ref_id IN (SELECT ref_id FROM web_references WHERE expires_at <= ?) OR source_ref_id IN (SELECT ref_id FROM web_references WHERE expires_at <= ?)`, now, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM web_turn_citation_evidence WHERE ref_id IN (SELECT ref_id FROM web_references WHERE expires_at <= ?)`, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM web_turn_citations WHERE ref_id IN (SELECT ref_id FROM web_references WHERE expires_at <= ?)`, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM web_references WHERE expires_at <= ?`, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.lastCleanup = now
	return nil
}

func (s *Store) PutReference(ctx context.Context, ref Reference) error {
	if s == nil {
		return errors.New("nil web store")
	}
	if s.db != nil {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO web_references (ref_id, conversation_id, turn_id, invocation_id, kind, url, canonical_url, title, snippet, provider, query_text, rank_value, published_at, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(ref_id) DO UPDATE SET conversation_id=excluded.conversation_id, turn_id=excluded.turn_id, invocation_id=excluded.invocation_id, kind=excluded.kind, url=excluded.url, canonical_url=excluded.canonical_url, title=excluded.title, snippet=excluded.snippet, provider=excluded.provider, query_text=excluded.query_text, rank_value=excluded.rank_value, published_at=excluded.published_at, expires_at=excluded.expires_at`, ref.RefID, ref.ConversationID, ref.TurnID, ref.InvocationID, ref.Kind, ref.URL, ref.CanonicalURL, ref.Title, ref.Snippet, ref.Provider, ref.Query, ref.Rank, nullableTime(ref.PublishedAt), ref.CreatedAt, ref.ExpiresAt); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.references[ref.RefID] = ref
	s.mu.Unlock()
	return nil
}

func (s *Store) GetReference(ctx context.Context, conversationID, refID string) (Reference, error) {
	if s == nil {
		return Reference{}, errors.New("nil web store")
	}
	s.mu.RLock()
	ref, ok := s.references[refID]
	s.mu.RUnlock()
	if ok {
		if ref.ConversationID != conversationID {
			return Reference{}, newError(ErrReferenceScope, "reference belongs to another conversation", false, nil)
		}
		if !ref.ExpiresAt.IsZero() && time.Now().UTC().After(ref.ExpiresAt) {
			return Reference{}, newError(ErrReferenceNotFound, "reference expired", false, nil)
		}
		return ref, nil
	}
	if s.db == nil {
		return Reference{}, newError(ErrReferenceNotFound, "reference not found", false, nil)
	}
	var published sql.NullTime
	row := s.db.QueryRowContext(ctx, `SELECT ref_id, conversation_id, turn_id, invocation_id, kind, url, canonical_url, title, snippet, provider, query_text, rank_value, published_at, created_at, expires_at FROM web_references WHERE ref_id = ?`, refID)
	if err := row.Scan(&ref.RefID, &ref.ConversationID, &ref.TurnID, &ref.InvocationID, &ref.Kind, &ref.URL, &ref.CanonicalURL, &ref.Title, &ref.Snippet, &ref.Provider, &ref.Query, &ref.Rank, &published, &ref.CreatedAt, &ref.ExpiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Reference{}, newError(ErrReferenceNotFound, "reference not found", false, nil)
		}
		return Reference{}, err
	}
	if published.Valid {
		value := published.Time.UTC()
		ref.PublishedAt = &value
	}
	if ref.ConversationID != conversationID {
		return Reference{}, newError(ErrReferenceScope, "reference belongs to another conversation", false, nil)
	}
	if !ref.ExpiresAt.IsZero() && time.Now().UTC().After(ref.ExpiresAt) {
		return Reference{}, newError(ErrReferenceNotFound, "reference expired", false, nil)
	}
	s.mu.Lock()
	s.references[ref.RefID] = ref
	s.mu.Unlock()
	return ref, nil
}

func (s *Store) PutPage(ctx context.Context, page Page) error {
	if s == nil {
		return errors.New("nil web store")
	}
	links, err := json.Marshal(page.Links)
	if err != nil {
		return err
	}
	seenAt := page.FetchedAt.UTC()
	if seenAt.IsZero() {
		seenAt = nowUTC()
	}
	if s.db != nil {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err := tx.ExecContext(ctx, `INSERT INTO web_pages (ref_id, source_ref_id, conversation_id, url, canonical_url, title, content_type, content, content_hash, links_json, truncated, dynamic, fetched_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(ref_id) DO UPDATE SET source_ref_id=excluded.source_ref_id, conversation_id=excluded.conversation_id, url=excluded.url, canonical_url=excluded.canonical_url, title=excluded.title, content_type=excluded.content_type, content=excluded.content, content_hash=excluded.content_hash, links_json=excluded.links_json, truncated=excluded.truncated, dynamic=excluded.dynamic, fetched_at=excluded.fetched_at`, page.RefID, page.SourceRefID, page.ConversationID, page.URL, page.CanonicalURL, page.Title, page.ContentType, page.Content, page.ContentHash, string(links), boolInt(page.Truncated), boolInt(page.Dynamic), page.FetchedAt); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, upsertPageVersionSQL, page.RefID, page.ContentHash, page.Title, page.ContentType, page.Content, string(links), boolInt(page.Truncated), boolInt(page.Dynamic), seenAt, seenAt); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.pages[page.RefID] = page
	s.recordPageVersionLocked(page, seenAt)
	s.mu.Unlock()
	return nil
}

func (s *Store) GetPage(ctx context.Context, conversationID, refID string) (Page, error) {
	if s == nil {
		return Page{}, errors.New("nil web store")
	}
	s.mu.RLock()
	page, ok := s.pages[refID]
	s.mu.RUnlock()
	if ok {
		if page.ConversationID != conversationID {
			return Page{}, newError(ErrReferenceScope, "page belongs to another conversation", false, nil)
		}
		return page, nil
	}
	if s.db == nil {
		return Page{}, newError(ErrReferenceNotFound, "page not found", false, nil)
	}
	var links string
	var truncated int
	var dynamic int
	row := s.db.QueryRowContext(ctx, `SELECT ref_id, source_ref_id, conversation_id, url, canonical_url, title, content_type, content, content_hash, links_json, truncated, dynamic, fetched_at FROM web_pages WHERE ref_id = ?`, refID)
	if err := row.Scan(&page.RefID, &page.SourceRefID, &page.ConversationID, &page.URL, &page.CanonicalURL, &page.Title, &page.ContentType, &page.Content, &page.ContentHash, &links, &truncated, &dynamic, &page.FetchedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Page{}, newError(ErrReferenceNotFound, "page not found", false, nil)
		}
		return Page{}, err
	}
	if page.ConversationID != conversationID {
		return Page{}, newError(ErrReferenceScope, "page belongs to another conversation", false, nil)
	}
	page.Truncated = truncated != 0
	page.Dynamic = dynamic != 0
	_ = json.Unmarshal([]byte(links), &page.Links)
	s.mu.Lock()
	s.pages[page.RefID] = page
	s.mu.Unlock()
	return page, nil
}

func (s *Store) PutPageArtifact(ctx context.Context, page Page, ref Reference, items []Evidence) error {
	if s == nil {
		return errors.New("nil web store")
	}
	if page.RefID == "" || ref.RefID == "" || page.RefID != ref.RefID {
		return errors.New("page artifact requires matching page and reference ids")
	}
	if page.ConversationID == "" || ref.ConversationID != page.ConversationID {
		return errors.New("page artifact conversation mismatch")
	}
	links, err := json.Marshal(page.Links)
	if err != nil {
		return err
	}
	locators := make([]string, len(items))
	for i := range items {
		if items[i].ConversationID != page.ConversationID || items[i].PageRefID != page.RefID {
			return errors.New("page artifact evidence scope mismatch")
		}
		if strings.TrimSpace(items[i].PageContentHash) == "" {
			items[i].PageContentHash = page.ContentHash
		}
		if page.ContentHash != "" && items[i].PageContentHash != page.ContentHash {
			return errors.New("page artifact evidence content version mismatch")
		}
		encoded, err := json.Marshal(items[i].Locator)
		if err != nil {
			return err
		}
		locators[i] = string(encoded)
	}

	if s.db != nil {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err := tx.ExecContext(ctx, `INSERT INTO web_pages (ref_id, source_ref_id, conversation_id, url, canonical_url, title, content_type, content, content_hash, links_json, truncated, dynamic, fetched_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(ref_id) DO UPDATE SET source_ref_id=excluded.source_ref_id, conversation_id=excluded.conversation_id, url=excluded.url, canonical_url=excluded.canonical_url, title=excluded.title, content_type=excluded.content_type, content=excluded.content, content_hash=excluded.content_hash, links_json=excluded.links_json, truncated=excluded.truncated, dynamic=excluded.dynamic, fetched_at=excluded.fetched_at`, page.RefID, page.SourceRefID, page.ConversationID, page.URL, page.CanonicalURL, page.Title, page.ContentType, page.Content, page.ContentHash, string(links), boolInt(page.Truncated), boolInt(page.Dynamic), page.FetchedAt); err != nil {
			return err
		}
		seenAt := page.FetchedAt.UTC()
		if seenAt.IsZero() {
			seenAt = nowUTC()
		}
		if _, err := tx.ExecContext(ctx, upsertPageVersionSQL, page.RefID, page.ContentHash, page.Title, page.ContentType, page.Content, string(links), boolInt(page.Truncated), boolInt(page.Dynamic), seenAt, seenAt); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO web_references (ref_id, conversation_id, turn_id, invocation_id, kind, url, canonical_url, title, snippet, provider, query_text, rank_value, published_at, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(ref_id) DO UPDATE SET conversation_id=excluded.conversation_id, turn_id=excluded.turn_id, invocation_id=excluded.invocation_id, kind=excluded.kind, url=excluded.url, canonical_url=excluded.canonical_url, title=excluded.title, snippet=excluded.snippet, provider=excluded.provider, query_text=excluded.query_text, rank_value=excluded.rank_value, published_at=excluded.published_at, expires_at=excluded.expires_at`, ref.RefID, ref.ConversationID, ref.TurnID, ref.InvocationID, ref.Kind, ref.URL, ref.CanonicalURL, ref.Title, ref.Snippet, ref.Provider, ref.Query, ref.Rank, nullableTime(ref.PublishedAt), ref.CreatedAt, ref.ExpiresAt); err != nil {
			return err
		}
		for i, item := range items {
			if _, err := tx.ExecContext(ctx, `INSERT INTO web_evidence (evidence_id, conversation_id, page_ref_id, page_content_hash, query_text, text_value, text_hash, locator_json, relevance, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(evidence_id) DO UPDATE SET page_content_hash=excluded.page_content_hash, query_text=excluded.query_text, text_value=excluded.text_value, text_hash=excluded.text_hash, locator_json=excluded.locator_json, relevance=excluded.relevance`, item.ID, item.ConversationID, item.PageRefID, item.PageContentHash, item.Query, item.Text, item.TextHash, locators[i], item.Relevance, item.CreatedAt); err != nil {
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	// Publish to the in-memory read-through cache only after the durable write
	// succeeds, so callers never observe an artifact that cannot survive restart.
	s.mu.Lock()
	s.pages[page.RefID] = page
	seenAt := page.FetchedAt.UTC()
	if seenAt.IsZero() {
		seenAt = nowUTC()
	}
	s.recordPageVersionLocked(page, seenAt)
	s.references[ref.RefID] = ref
	for _, item := range items {
		s.evidence[item.ID] = item
	}
	s.mu.Unlock()
	return nil
}

func (s *Store) recordPageVersionLocked(page Page, seenAt time.Time) {
	if strings.TrimSpace(page.RefID) == "" || strings.TrimSpace(page.ContentHash) == "" {
		return
	}
	versions := s.pageVersions[page.RefID]
	if versions == nil {
		versions = make(map[string]PageVersion)
		s.pageVersions[page.RefID] = versions
	}
	version, exists := versions[page.ContentHash]
	if !exists {
		version = PageVersion{RefID: page.RefID, ContentHash: page.ContentHash, FirstSeenAt: seenAt}
	}
	version.Title = page.Title
	version.ContentType = page.ContentType
	version.Content = page.Content
	version.Links = append([]Link(nil), page.Links...)
	version.Truncated = page.Truncated
	version.Dynamic = page.Dynamic
	version.LastSeenAt = seenAt
	versions[page.ContentHash] = version
}

func (s *Store) PageVersions(ctx context.Context, refID string) ([]PageVersion, error) {
	if s == nil {
		return nil, errors.New("nil web store")
	}
	refID = strings.TrimSpace(refID)
	if refID == "" {
		return nil, nil
	}
	s.mu.RLock()
	cached := s.pageVersions[refID]
	items := make([]PageVersion, 0, len(cached))
	for _, version := range cached {
		copyVersion := version
		copyVersion.Links = append([]Link(nil), version.Links...)
		items = append(items, copyVersion)
	}
	s.mu.RUnlock()
	if len(items) == 0 && s.db != nil {
		rows, err := s.db.QueryContext(ctx, `SELECT ref_id, content_hash, title, content_type, content, links_json, truncated, dynamic, first_seen_at, last_seen_at FROM web_page_versions WHERE ref_id = ? ORDER BY first_seen_at ASC, content_hash ASC`, refID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var version PageVersion
			var links string
			var truncated, dynamic int
			if err := rows.Scan(&version.RefID, &version.ContentHash, &version.Title, &version.ContentType, &version.Content, &links, &truncated, &dynamic, &version.FirstSeenAt, &version.LastSeenAt); err != nil {
				return nil, err
			}
			version.Truncated = truncated != 0
			version.Dynamic = dynamic != 0
			_ = json.Unmarshal([]byte(links), &version.Links)
			items = append(items, version)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		if len(items) > 0 {
			s.mu.Lock()
			for _, version := range items {
				if s.pageVersions[refID] == nil {
					s.pageVersions[refID] = make(map[string]PageVersion)
				}
				s.pageVersions[refID][version.ContentHash] = version
			}
			s.mu.Unlock()
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].FirstSeenAt.Equal(items[j].FirstSeenAt) {
			return items[i].FirstSeenAt.Before(items[j].FirstSeenAt)
		}
		return items[i].ContentHash < items[j].ContentHash
	})
	return items, nil
}

func (s *Store) PutEvidence(ctx context.Context, items []Evidence) error {
	if s == nil {
		return errors.New("nil web store")
	}
	if s.db != nil && len(items) > 0 {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		for _, item := range items {
			locator, err := json.Marshal(item.Locator)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO web_evidence (evidence_id, conversation_id, page_ref_id, page_content_hash, query_text, text_value, text_hash, locator_json, relevance, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(evidence_id) DO UPDATE SET page_content_hash=excluded.page_content_hash, query_text=excluded.query_text, text_value=excluded.text_value, text_hash=excluded.text_hash, locator_json=excluded.locator_json, relevance=excluded.relevance`, item.ID, item.ConversationID, item.PageRefID, item.PageContentHash, item.Query, item.Text, item.TextHash, string(locator), item.Relevance, item.CreatedAt); err != nil {
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	s.mu.Lock()
	for _, item := range items {
		s.evidence[item.ID] = item
	}
	s.mu.Unlock()
	return nil
}

func (s *Store) EvidenceForPage(ctx context.Context, conversationID, pageRefID string) ([]Evidence, error) {
	if s == nil {
		return nil, errors.New("nil web store")
	}
	items := make([]Evidence, 0)
	s.mu.RLock()
	for _, item := range s.evidence {
		if item.ConversationID == conversationID && item.PageRefID == pageRefID {
			items = append(items, item)
		}
	}
	s.mu.RUnlock()
	if len(items) > 0 || s.db == nil {
		sortEvidence(items)
		return items, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT evidence_id, conversation_id, page_ref_id, page_content_hash, query_text, text_value, text_hash, locator_json, relevance, created_at FROM web_evidence WHERE conversation_id = ? AND page_ref_id = ? ORDER BY relevance DESC, created_at ASC`, conversationID, pageRefID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item Evidence
		var locator string
		if err := rows.Scan(&item.ID, &item.ConversationID, &item.PageRefID, &item.PageContentHash, &item.Query, &item.Text, &item.TextHash, &locator, &item.Relevance, &item.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(locator), &item.Locator)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) EvidenceForPageQuery(ctx context.Context, conversationID, pageRefID, query string) ([]Evidence, error) {
	return s.EvidenceForPageVersionQuery(ctx, conversationID, pageRefID, "", query)
}

func (s *Store) EvidenceForPageVersionQuery(ctx context.Context, conversationID, pageRefID, pageContentHash, query string) ([]Evidence, error) {
	if s == nil {
		return nil, errors.New("nil web store")
	}
	query = normalizedQuery(query)
	pageContentHash = strings.TrimSpace(pageContentHash)
	items := make([]Evidence, 0)
	s.mu.RLock()
	for _, item := range s.evidence {
		if item.ConversationID != conversationID || item.PageRefID != pageRefID || normalizedQuery(item.Query) != query {
			continue
		}
		if pageContentHash != "" && item.PageContentHash != pageContentHash {
			continue
		}
		items = append(items, item)
	}
	s.mu.RUnlock()
	if len(items) > 0 || s.db == nil {
		sortEvidence(items)
		return items, nil
	}
	querySQL := `SELECT evidence_id, conversation_id, page_ref_id, page_content_hash, query_text, text_value, text_hash, locator_json, relevance, created_at FROM web_evidence WHERE conversation_id = ? AND page_ref_id = ? AND query_text = ?`
	args := []any{conversationID, pageRefID, query}
	if pageContentHash != "" {
		querySQL += ` AND page_content_hash = ?`
		args = append(args, pageContentHash)
	}
	querySQL += ` ORDER BY relevance DESC, created_at ASC`
	rows, err := s.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item Evidence
		var locator string
		if err := rows.Scan(&item.ID, &item.ConversationID, &item.PageRefID, &item.PageContentHash, &item.Query, &item.Text, &item.TextHash, &locator, &item.Relevance, &item.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(locator), &item.Locator)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) > 0 {
		s.mu.Lock()
		for _, item := range items {
			s.evidence[item.ID] = item
		}
		s.mu.Unlock()
	}
	return items, nil
}

func (s *Store) PageForSource(ctx context.Context, conversationID, sourceRefID string) (Page, bool, error) {
	if s == nil {
		return Page{}, false, errors.New("nil web store")
	}
	s.mu.RLock()
	var latest Page
	foundMemory := false
	for _, page := range s.pages {
		if page.ConversationID == conversationID && page.SourceRefID == sourceRefID {
			if !foundMemory || page.FetchedAt.After(latest.FetchedAt) {
				latest = page
				foundMemory = true
			}
		}
	}
	s.mu.RUnlock()
	if foundMemory {
		return latest, true, nil
	}
	if s.db == nil {
		return Page{}, false, nil
	}
	var refID string
	err := s.db.QueryRowContext(ctx, `SELECT ref_id FROM web_pages WHERE conversation_id = ? AND source_ref_id = ? ORDER BY fetched_at DESC LIMIT 1`, conversationID, sourceRefID).Scan(&refID)
	if errors.Is(err, sql.ErrNoRows) {
		return Page{}, false, nil
	}
	if err != nil {
		return Page{}, false, err
	}
	page, err := s.GetPage(ctx, conversationID, refID)
	if err != nil {
		return Page{}, false, err
	}
	return page, true, nil
}

func (s *Store) CitationRegistryForTurn(ctx context.Context, turnID string) ([]TurnCitationBinding, error) {
	if s == nil {
		return nil, errors.New("nil web store")
	}
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return nil, errors.New("citation registry requires turn_id")
	}
	if s.db == nil {
		s.mu.RLock()
		byRef := s.citationNumbers[turnID]
		items := make([]TurnCitationBinding, 0, len(byRef))
		for refID, number := range byRef {
			items = append(items, TurnCitationBinding{TurnID: turnID, RefID: refID, CitationNumber: number})
		}
		s.mu.RUnlock()
		sortCitationBindings(items)
		return items, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT turn_id, ref_id, citation_number, created_at FROM web_turn_citations WHERE turn_id = ? ORDER BY citation_number ASC`, turnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]TurnCitationBinding, 0)
	for rows.Next() {
		var item TurnCitationBinding
		if err := rows.Scan(&item.TurnID, &item.RefID, &item.CitationNumber, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) > 0 {
		s.mu.Lock()
		if s.citationNumbers[turnID] == nil {
			s.citationNumbers[turnID] = make(map[string]int, len(items))
		}
		for _, item := range items {
			s.citationNumbers[turnID][item.RefID] = item.CitationNumber
		}
		s.mu.Unlock()
	}
	return items, nil
}

func (s *Store) BindCitationEvidence(ctx context.Context, turnID string, citationNumber int, refID, evidenceID string) error {
	if s == nil {
		return errors.New("nil web store")
	}
	turnID = strings.TrimSpace(turnID)
	refID = strings.TrimSpace(refID)
	evidenceID = strings.TrimSpace(evidenceID)
	if turnID == "" || citationNumber <= 0 || refID == "" || evidenceID == "" {
		return errors.New("citation evidence binding requires turn_id, citation_number, ref_id and evidence_id")
	}
	createdAt := time.Now().UTC()
	if s.db != nil {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO web_turn_citation_evidence (turn_id, citation_number, ref_id, evidence_id, created_at) VALUES (?, ?, ?, ?, ?) ON CONFLICT(turn_id, evidence_id) DO UPDATE SET citation_number=excluded.citation_number, ref_id=excluded.ref_id`, turnID, citationNumber, refID, evidenceID, createdAt); err != nil {
			return err
		}
	}
	s.mu.Lock()
	if s.citationEvidence[turnID] == nil {
		s.citationEvidence[turnID] = make(map[string]TurnCitationEvidenceBinding)
	}
	s.citationEvidence[turnID][evidenceID] = TurnCitationEvidenceBinding{TurnID: turnID, CitationNumber: citationNumber, RefID: refID, EvidenceID: evidenceID, CreatedAt: createdAt}
	s.mu.Unlock()
	return nil
}

func (s *Store) CitationEvidenceForTurn(ctx context.Context, turnID string) ([]TurnCitationEvidenceBinding, error) {
	if s == nil {
		return nil, errors.New("nil web store")
	}
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return nil, errors.New("citation evidence registry requires turn_id")
	}
	if s.db == nil {
		s.mu.RLock()
		byEvidence := s.citationEvidence[turnID]
		items := make([]TurnCitationEvidenceBinding, 0, len(byEvidence))
		for _, item := range byEvidence {
			items = append(items, item)
		}
		s.mu.RUnlock()
		sort.Slice(items, func(i, j int) bool {
			if items[i].CitationNumber != items[j].CitationNumber {
				return items[i].CitationNumber < items[j].CitationNumber
			}
			return items[i].EvidenceID < items[j].EvidenceID
		})
		return items, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT turn_id, citation_number, ref_id, evidence_id, created_at FROM web_turn_citation_evidence WHERE turn_id = ? ORDER BY citation_number ASC, evidence_id ASC`, turnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]TurnCitationEvidenceBinding, 0)
	for rows.Next() {
		var item TurnCitationEvidenceBinding
		if err := rows.Scan(&item.TurnID, &item.CitationNumber, &item.RefID, &item.EvidenceID, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) > 0 {
		s.mu.Lock()
		if s.citationEvidence[turnID] == nil {
			s.citationEvidence[turnID] = make(map[string]TurnCitationEvidenceBinding, len(items))
		}
		for _, item := range items {
			s.citationEvidence[turnID][item.EvidenceID] = item
		}
		s.mu.Unlock()
	}
	return items, nil
}

func sortCitationBindings(items []TurnCitationBinding) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].CitationNumber < items[i].CitationNumber ||
				(items[j].CitationNumber == items[i].CitationNumber && items[j].RefID < items[i].RefID) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func sortEvidence(items []Evidence) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Relevance > items[i].Relevance {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func (s *Store) GetOrAssignCitationNumber(ctx context.Context, turnID, refID string) (int, error) {
	if s == nil {
		return 0, errors.New("nil web store")
	}
	turnID = strings.TrimSpace(turnID)
	refID = strings.TrimSpace(refID)
	if turnID == "" || refID == "" {
		return 0, errors.New("citation numbering requires turn_id and ref_id")
	}

	// Citation numbers are allocated serially per Store so concurrent web.run calls
	// in the same turn cannot race and assign the same display number.
	s.citationMu.Lock()
	defer s.citationMu.Unlock()

	if s.db == nil {
		s.mu.RLock()
		if byRef := s.citationNumbers[turnID]; byRef != nil {
			if number := byRef[refID]; number > 0 {
				s.mu.RUnlock()
				return number, nil
			}
		}
		s.mu.RUnlock()
	}

	number := 0
	if s.db != nil {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return 0, err
		}
		defer tx.Rollback()

		err = tx.QueryRowContext(ctx, `SELECT citation_number FROM web_turn_citations WHERE turn_id = ? AND ref_id = ?`, turnID, refID).Scan(&number)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}
		if errors.Is(err, sql.ErrNoRows) {
			if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(citation_number), 0) + 1 FROM web_turn_citations WHERE turn_id = ?`, turnID).Scan(&number); err != nil {
				return 0, err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO web_turn_citations (turn_id, ref_id, citation_number, created_at) VALUES (?, ?, ?, ?)`, turnID, refID, number, time.Now().UTC()); err != nil {
				return 0, err
			}
		}
		if err := tx.Commit(); err != nil {
			return 0, err
		}
	} else {
		s.mu.RLock()
		byRef := s.citationNumbers[turnID]
		for _, existing := range byRef {
			if existing >= number {
				number = existing + 1
			}
		}
		s.mu.RUnlock()
		if number == 0 {
			number = 1
		}
	}

	if s.db == nil {
		s.mu.Lock()
		if s.citationNumbers[turnID] == nil {
			s.citationNumbers[turnID] = make(map[string]int)
		}
		s.citationNumbers[turnID][refID] = number
		s.mu.Unlock()
	}
	return number, nil
}
