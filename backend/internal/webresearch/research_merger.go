package webresearch

import "strings"

const maxFindingEvidence = 4

// mergeResearchFindings compresses evidence into question-scoped findings.
// It is intentionally deterministic and non-generative: no extra LLM call is
// required and every summary remains traceable to stored Evidence IDs.
func mergeResearchFindings(plan ResearchPlan, citations []Citation) []ResearchFinding {
	if len(plan.Questions) == 0 {
		return nil
	}
	findings := make([]ResearchFinding, 0, len(plan.Questions))
	for _, question := range plan.Questions {
		finding := ResearchFinding{QuestionID: question.ID, Question: question.Question, Status: "insufficient"}
		matches := rankCitationsForQuestion(question.Question, citations)
		if len(matches) == 0 {
			findings = append(findings, finding)
			continue
		}
		if len(matches) > maxFindingEvidence {
			matches = matches[:maxFindingEvidence]
		}
		seenRefs := make(map[string]struct{}, len(matches))
		summaryParts := make([]string, 0, len(matches))
		for _, citation := range matches {
			if citation.EvidenceID != "" {
				finding.EvidenceIDs = appendUniqueString(finding.EvidenceIDs, citation.EvidenceID)
			}
			if citation.RefID != "" {
				finding.SourceRefs = appendUniqueString(finding.SourceRefs, citation.RefID)
				seenRefs[citation.RefID] = struct{}{}
			}
			if text := strings.TrimSpace(citation.Text); text != "" {
				summaryParts = append(summaryParts, truncateRunes(text, 420))
			}
		}
		finding.Status = "supported"
		if len(seenRefs) >= 2 {
			finding.Status = "corroborated"
		}
		finding.Summary = truncateRunes(strings.Join(summaryParts, "\n"), 1200)
		findings = append(findings, finding)
	}
	return findings
}

func rankCitationsForQuestion(question string, citations []Citation) []Citation {
	questionTokens := tokenize(question)
	type scored struct {
		citation Citation
		score    float64
	}
	items := make([]scored, 0, len(citations))
	for _, citation := range citations {
		text := citation.Title + " " + citation.Text
		score := overlapScore(questionTokens, tokenize(text))
		if strings.Contains(strings.ToLower(text), strings.ToLower(strings.TrimSpace(question))) {
			score += 0.5
		}
		if score < 0.15 {
			continue
		}
		items = append(items, scored{citation: citation, score: score})
	}
	for i := 1; i < len(items); i++ {
		current := items[i]
		j := i - 1
		for ; j >= 0 && (items[j].score < current.score || (items[j].score == current.score && items[j].citation.EvidenceID > current.citation.EvidenceID)); j-- {
			items[j+1] = items[j]
		}
		items[j+1] = current
	}
	out := make([]Citation, 0, len(items))
	for _, item := range items {
		out = append(out, item.citation)
	}
	return out
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
