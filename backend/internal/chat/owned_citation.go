package chat

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/u-ai/backend/internal/knowledge"
)

type ownedCitationAuditContextKey struct{}

func withOwnedCitationAudit(ctx context.Context, audit *ownedCitationAudit) context.Context {
	if audit == nil {
		return ctx
	}
	return context.WithValue(ctx, ownedCitationAuditContextKey{}, audit)
}

func ownedCitationAuditFromContext(ctx context.Context) *ownedCitationAudit {
	audit, _ := ctx.Value(ownedCitationAuditContextKey{}).(*ownedCitationAudit)
	return audit
}

type ownedCitationAudit struct {
	citationMu                sync.RWMutex
	knowledgeCitationEvidence map[int]citationEvidence
}

func (r *ownedCitationAudit) registerKnowledgeCitation(index int, ev knowledge.Evidence, shownText string) {
	if r == nil || index <= 0 || index > 64 {
		return
	}
	r.citationMu.Lock()
	defer r.citationMu.Unlock()
	if r.knowledgeCitationEvidence == nil {
		r.knowledgeCitationEvidence = make(map[int]citationEvidence)
	}
	r.knowledgeCitationEvidence[index] = citationEvidence{
		EvidenceID: ev.ID, RefID: ev.ID, Title: ev.Title, Text: shownText,
		SourceID: ev.SourceID, SourceRevision: ev.SourceRevision,
		UnitID: ev.UnitID, Locator: safeCitationLocator(ev.Locator), Checksum: ev.Checksum,
	}
}

func (r *ownedCitationAudit) BuildCitationSnapshot(markdown string) []CitationSnapshot {
	if r == nil || strings.TrimSpace(markdown) == "" {
		return []CitationSnapshot{}
	}
	r.citationMu.RLock()
	knowledgeItems := make(map[int]citationEvidence, len(r.knowledgeCitationEvidence))
	for index, ev := range r.knowledgeCitationEvidence {
		knowledgeItems[index] = ev
	}
	r.citationMu.RUnlock()
	seen := make(map[string]bool)
	out := make([]CitationSnapshot, 0, 8)
	add := func(kind string, index int, ev citationEvidence) {
		if len(out) >= maxCitationSnapshotEntries {
			return
		}
		key := fmt.Sprintf("%s:%d", kind, index)
		if seen[key] {
			return
		}
		seen[key] = true
		raw := strings.TrimSpace(ev.Text)
		sum := sha256.Sum256([]byte(raw))
		marker := fmt.Sprintf("[%d]", index)
		if kind == "knowledge" {
			marker = fmt.Sprintf("[K%d]", index)
		}
		out = append(out, CitationSnapshot{
			Marker: marker, Kind: kind, Index: index, EvidenceID: ev.EvidenceID,
			RefID: ev.RefID, SourceID: ev.SourceID, SourceRevision: ev.SourceRevision,
			UnitID: ev.UnitID, Title: citationExcerpt(ev.Title, 200), URL: ev.URL,
			Locator: safeCitationLocator(ev.Locator), Excerpt: citationExcerpt(raw, 1200),
			ContentHash: fmt.Sprintf("%x", sum[:]), Checksum: ev.Checksum,
		})
	}
	for _, index := range citationMarkersInProse(markdown, knowledgeMarkerRE) {
		if ev, ok := knowledgeItems[index]; ok {
			add("knowledge", index, ev)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Index < out[j].Index
	})
	return normalizeCitationSnapshotRecords(out)
}

func (r *ownedCitationAudit) AuditCitationMarkers(markdown string) citationAudit {
	if r == nil {
		return citationAudit{}
	}
	r.citationMu.RLock()
	r.citationMu.RUnlock()
	availableSet := map[int]struct{}{}
	evidenceByCitation := map[int]citationEvidence{}
	usedSet := make(map[int]struct{})
	unknownSet := make(map[int]struct{})
	for _, id := range citationMarkersInProse(markdown, numericCitationPattern) {
		if _, ok := availableSet[id]; ok {
			usedSet[id] = struct{}{}
		} else {
			unknownSet[id] = struct{}{}
		}
	}
	return citationAudit{
		Available:       sortedCitationIDs(availableSet),
		Used:            sortedCitationIDs(usedSet),
		Unknown:         sortedCitationIDs(unknownSet),
		Claims:          auditCitationClaims(markdown, availableSet, evidenceByCitation),
		MissingCitation: nil,
	}
}
