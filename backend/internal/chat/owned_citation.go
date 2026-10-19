package chat

import("encoding/json";"regexp";"sort";"strconv";"strings";"sync")

type ownedCitationAudit struct { citationMu sync.RWMutex; citationIDs map[int]struct{}; citationEvidence map[int]citationEvidence; webResearchUsed bool }

type citationAudit struct {
	Available       []int
	Used            []int
	Unknown         []int
	Claims          []claimCitationAudit
	MissingCitation []string
}

type citationEvidence struct {
	EvidenceID string
	RefID      string
	Title      string
	URL        string
	Text       string
}

type claimCitationAudit struct {
	Text         string
	Citations    []int
	Status       string
	SupportScore float64
}

var numericCitationPattern = regexp.MustCompile(`\[(\d{1,6})\]`)

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

func auditCitationClaims(markdown string, available map[int]struct{}, evidence map[int]citationEvidence) []claimCitationAudit {
	segments := citationClaimSegments(markdown)
	claims := make([]claimCitationAudit, 0, len(segments))
	for _, segment := range segments {
		matches := numericCitationPattern.FindAllStringSubmatch(segment, -1)
		if len(matches) == 0 {
			continue
		}
		ids := make([]int, 0, len(matches))
		seen := map[int]struct{}{}
		knownEvidence := make([]string, 0, len(matches))
		hasUnknown := false
		for _, match := range matches {
			id, err := strconv.Atoi(match[1])
			if err != nil || id <= 0 {
				continue
			}
			if _, duplicate := seen[id]; duplicate {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
			if _, ok := available[id]; !ok {
				hasUnknown = true
				continue
			}
			if item := evidence[id]; strings.TrimSpace(item.Text) != "" {
				knownEvidence = append(knownEvidence, item.Text)
			}
		}
		if len(ids) == 0 {
			continue
		}
		sort.Ints(ids)
		claimText := strings.TrimSpace(numericCitationPattern.ReplaceAllString(segment, ""))
		status := "supported"
		score := 0.0
		switch {
		case hasUnknown:
			status = "invalid_citation"
		case len(knownEvidence) == 0:
			status = "insufficient"
		default:
			score = citationLexicalSupport(claimText, strings.Join(knownEvidence, "\n"))
			if score == 0 {
				status = "unsupported"
			} else if score < 0.15 {
				status = "partially_supported"
			}
		}
		claims = append(claims, claimCitationAudit{Text: claimText, Citations: ids, Status: status, SupportScore: score})
	}
	return claims
}

var citationClaimSentencePattern = regexp.MustCompile(`(?:(?:\d+\.\d+)|[^.!?。！？\n])+(?:[.!?。！？]+|$)`)

var factualCitationSignalPattern = regexp.MustCompile(`(?i)(?:\b(?:19|20)\d{2}\b|\b\d+(?:\.\d+)+\b|\b(?:released|launch(?:ed)?|supports?|supported|available|version|published|updated|announced|according to|percent|million|billion)\b|发布|上线|支持|版本|更新|宣布|截至|目前|根据|\d+(?:\.\d+)?%|\d+年|\d+月|\d+日)`)

func auditMissingCitationClaims(markdown string) []string {
	sentences := citationSentencesOutsideCode(markdown)
	out := make([]string, 0)
	for _, sentence := range sentences {
		if numericCitationPattern.MatchString(sentence) {
			continue
		}
		plain := strings.TrimSpace(sentence)
		if plain == "" || !factualCitationSignalPattern.MatchString(plain) {
			continue
		}
		lower := strings.ToLower(plain)
		if strings.Contains(lower, "i think") || strings.Contains(lower, "in my view") || strings.Contains(plain, "我认为") || strings.Contains(plain, "我觉得") {
			continue
		}
		out = append(out, truncateCitationAuditText(plain, 320))
		if len(out) >= 12 {
			break
		}
	}
	return out
}

func citationSentencesOutsideCode(markdown string) []string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	clean := make([]string, 0, len(lines))
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
		clean = append(clean, inlineCodePattern.ReplaceAllString(line, ""))
	}
	return citationClaimSentencePattern.FindAllString(strings.Join(clean, "\n"), -1)
}

func truncateCitationAuditText(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "…"
}

var inlineCodePattern = regexp.MustCompile("`[^`]*`")
var citationTokenPattern = regexp.MustCompile(`[\p{L}\p{N}]+`)

func citationClaimSegments(markdown string) []string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	clean := make([]string, 0, len(lines))
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
		clean = append(clean, inlineCodePattern.ReplaceAllString(line, ""))
	}
	text := strings.Join(clean, "\n")
	matches := citationClaimSentencePattern.FindAllString(text, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if numericCitationPattern.MatchString(match) {
			out = append(out, strings.TrimSpace(match))
		}
	}
	return out
}

func citationLexicalSupport(claim, evidence string) float64 {
	claimTokens := citationTokens(claim)
	if len(claimTokens) == 0 {
		return 1
	}
	evidenceTokens := citationTokens(evidence)
	matched := 0
	for token := range claimTokens {
		if _, ok := evidenceTokens[token]; ok {
			matched++
		}
	}
	return float64(matched) / float64(len(claimTokens))
}

func citationTokens(value string) map[string]struct{} {
	value = strings.ToLower(value)
	matches := citationTokenPattern.FindAllString(value, -1)
	out := make(map[string]struct{}, len(matches))
	for _, token := range matches {
		token = strings.TrimSpace(token)
		if token == "" || isCitationStopToken(token) {
			continue
		}
		out[token] = struct{}{}
	}
	return out
}

func isCitationStopToken(token string) bool {
	switch token {
	case "the", "a", "an", "and", "or", "of", "to", "in", "on", "for", "is", "are", "was", "were", "be", "with", "as", "by", "from", "that", "this", "it", "its", "at":
		return true
	default:
		return false
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

func sortedCitationIDs(values map[int]struct{}) []int {
	out := make([]int, 0, len(values))
	for id := range values {
		out = append(out, id)
	}
	sort.Ints(out)
	return out
}

