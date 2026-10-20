package chat

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
)

type ownedCitationAudit struct {
	citationMu       sync.RWMutex
	citationIDs      map[int]struct{}
	citationEvidence map[int]citationEvidence
	webResearchUsed  bool
}

func (r *ownedCitationAudit) captureCitationIDs(toolName, result string) {
	if r == nil || (toolName != "web_run" && toolName != "web.run") {
		return
	}
	r.citationMu.Lock()
	r.webResearchUsed = true
	r.citationMu.Unlock()
	if strings.TrimSpace(result) == "" {
		return
	}
	var payload struct {
		Citations []struct {
			Index      int    `json:"index"`
			EvidenceID string `json:"evidence_id"`
			RefID      string `json:"ref_id"`
			Title      string `json:"title"`
			URL        string `json:"url"`
			Text       string `json:"text"`
		} `json:"citations"`
	}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		return
	}
	r.citationMu.Lock()
	defer r.citationMu.Unlock()
	if r.citationIDs == nil {
		r.citationIDs = make(map[int]struct{})
	}
	if r.citationEvidence == nil {
		r.citationEvidence = make(map[int]citationEvidence)
	}
	for _, citation := range payload.Citations {
		if citation.Index > 0 {
			r.citationIDs[citation.Index] = struct{}{}
			r.citationEvidence[citation.Index] = citationEvidence{
				EvidenceID: strings.TrimSpace(citation.EvidenceID),
				RefID:      strings.TrimSpace(citation.RefID),
				Title:      strings.TrimSpace(citation.Title),
				URL:        strings.TrimSpace(citation.URL),
				Text:       strings.TrimSpace(citation.Text),
			}
		}
	}
}

func (r *ownedCitationAudit) AuditCitationMarkers(markdown string) citationAudit {
	if r == nil {
		return citationAudit{}
	}
	r.citationMu.RLock()
	webResearchUsed := r.webResearchUsed
	availableSet := make(map[int]struct{}, len(r.citationIDs))
	for id := range r.citationIDs {
		availableSet[id] = struct{}{}
	}
	evidenceByCitation := make(map[int]citationEvidence, len(r.citationEvidence))
	for id, evidence := range r.citationEvidence {
		evidenceByCitation[id] = evidence
	}
	r.citationMu.RUnlock()
	usedSet := make(map[int]struct{})
	unknownSet := make(map[int]struct{})
	for _, match := range citationMarkersOutsideCode(markdown) {
		id, err := strconv.Atoi(match)
		if err != nil || id <= 0 {
			continue
		}
		if _, ok := availableSet[id]; ok {
			usedSet[id] = struct{}{}
		} else {
			unknownSet[id] = struct{}{}
		}
	}
	missing := []string(nil)
	if webResearchUsed {
		missing = auditMissingCitationClaims(markdown)
	}
	return citationAudit{
		Available:       sortedCitationIDs(availableSet),
		Used:            sortedCitationIDs(usedSet),
		Unknown:         sortedCitationIDs(unknownSet),
		Claims:          auditCitationClaims(markdown, availableSet, evidenceByCitation),
		MissingCitation: missing,
	}
}

func citationMarkersOutsideCode(markdown string) []string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	out := make([]string, 0)
	inFence := false
	fence := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			current := trimmed[:3]
			if !inFence {
				inFence = true
				fence = current
			} else if current == fence {
				inFence = false
				fence = ""
			}
			continue
		}
		if inFence {
			continue
		}
		out = append(out, citationMarkersOutsideInlineCode(line)...)
	}
	return out
}

func citationMarkersOutsideInlineCode(line string) []string {
	out := make([]string, 0)
	start := 0
	inCode := false
	for i := 0; i <= len(line); i++ {
		if i < len(line) && line[i] != '`' {
			continue
		}
		if !inCode && i > start {
			segment := line[start:i]
			for _, match := range numericCitationPattern.FindAllStringSubmatch(segment, -1) {
				if len(match) > 1 {
					out = append(out, match[1])
				}
			}
		}
		if i == len(line) {
			break
		}
		inCode = !inCode
		start = i + 1
	}
	return out
}
