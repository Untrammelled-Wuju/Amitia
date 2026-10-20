package knowledge

import (
	"regexp"
	"strings"
)

const (
	AnswerStateSufficient   = "sufficient"
	AnswerStateInsufficient = "insufficient"
	AnswerStateAmbiguous    = "ambiguous"
)

type sufficiencyDecision struct {
	Sufficient bool
	State      string
	Reason     string
}

// evaluateSufficiency is deliberately deterministic. It does not call an LLM
// to decide whether retrieval is good enough; this keeps the no-answer gate
// auditable and prevents a second model call from inventing coverage.
func evaluateSufficiency(query string, evidence []Evidence, ambiguous bool) sufficiencyDecision {
	if len(evidence) == 0 {
		return sufficiencyDecision{State: AnswerStateInsufficient, Reason: "no_evidence"}
	}
	qualityFloor := 1.0 / (rrfK + 1.0)
	if evidence[0].FusionScore < qualityFloor {
		return sufficiencyDecision{State: AnswerStateInsufficient, Reason: "low_quality"}
	}
	// A comparison is not covered by two chunks from the same document. It must
	// contain evidence from at least two canonical sources; otherwise the model
	// is missing one side of the comparison.
	if isComparisonQuery(query) && distinctSourceCount(evidence) < 2 {
		return sufficiencyDecision{State: AnswerStateInsufficient, Reason: "comparison_source_coverage"}
	}
	if required := explicitLocatorRequirements(query); len(required) > 0 && !evidenceCoversLocatorRequirements(evidence, required) {
		return sufficiencyDecision{State: AnswerStateInsufficient, Reason: "locator_coverage"}
	}
	if ambiguous {
		return sufficiencyDecision{Sufficient: true, State: AnswerStateAmbiguous, Reason: "conflicting_evidence"}
	}
	return sufficiencyDecision{Sufficient: true, State: AnswerStateSufficient, Reason: "evidence_covered"}
}

func distinctEvidenceCount(evidence []Evidence) int {
	seen := map[string]struct{}{}
	for _, item := range evidence {
		key := strings.TrimSpace(item.SourceID) + "\x00" + strings.TrimSpace(item.ID)
		if key == "\x00" {
			continue
		}
		seen[key] = struct{}{}
	}
	return len(seen)
}

func distinctSourceCount(evidence []Evidence) int {
	seen := map[string]struct{}{}
	for _, item := range evidence {
		if id := strings.TrimSpace(item.SourceID); id != "" {
			seen[id] = struct{}{}
		}
	}
	return len(seen)
}

var (
	zhPageRE = regexp.MustCompile(`第\s*([0-9]{1,7})\s*页`)
	enPageRE = regexp.MustCompile(`(?i)\bpage\s*#?\s*([0-9]{1,7})\b`)
	zhRowRE  = regexp.MustCompile(`第\s*([0-9]{1,9})\s*(?:行|列)`)
	enRowRE  = regexp.MustCompile(`(?i)\brow\s*#?\s*([0-9]{1,9})\b`)
)

func explicitLocatorRequirements(query string) []string {
	out := make([]string, 0, 4)
	seen := map[string]struct{}{}
	appendMatch := func(prefix string, matches [][]string) {
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			key := prefix + "=" + strings.TrimSpace(match[1])
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, key)
		}
	}
	appendMatch("page", zhPageRE.FindAllStringSubmatch(query, 4))
	appendMatch("page", enPageRE.FindAllStringSubmatch(query, 4))
	appendMatch("row", zhRowRE.FindAllStringSubmatch(query, 4))
	appendMatch("row", enRowRE.FindAllStringSubmatch(query, 4))
	return out
}

func evidenceCoversLocatorRequirements(evidence []Evidence, required []string) bool {
	if len(required) == 0 {
		return true
	}
	covered := map[string]struct{}{}
	for _, ev := range evidence {
		keys := ev.CoverageKeys
		if len(keys) == 0 {
			keys = coverageKeysForLocator(ev.Locator)
		}
		for _, key := range keys {
			covered[normalizeCoverageKey(key)] = struct{}{}
		}
	}
	for _, requirement := range required {
		normalized := normalizeCoverageKey(requirement)
		if _, ok := covered[normalized]; ok {
			continue
		}
		// Parser locators are not fully standardized yet; pageNumber and rowStart
		// are accepted as equivalent coverage aliases without weakening the value.
		parts := strings.SplitN(normalized, "=", 2)
		if len(parts) != 2 {
			return false
		}
		aliases := []string{}
		switch parts[0] {
		case "page":
			aliases = []string{"pagenumber=" + parts[1]}
		case "row":
			aliases = []string{"rowstart=" + parts[1], "rowend=" + parts[1]}
		}
		matched := false
		for _, alias := range aliases {
			if _, ok := covered[alias]; ok {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func normalizeCoverageKey(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
}

func isComparisonQuery(query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return false
	}
	markers := []string{
		"对比", "比较", "区别", "差异", "不同", "优缺点", "哪个好", "哪一个", "分别", "异同",
		" vs ", " versus ", "compare", "comparison", "difference", "differences", "pros and cons",
	}
	for _, marker := range markers {
		if strings.Contains(q, marker) {
			return true
		}
	}
	return false
}

// ApplySufficiency re-runs the deterministic evidence gate after canonical
// reads have expanded or replaced retrieval snippets. It is intentionally the
// same gate used by Search so orchestration cannot invent a separate notion of
// "enough evidence".
func ApplySufficiency(query string, response *SearchResponse) {
	if response == nil {
		return
	}
	decision := evaluateSufficiency(query, response.Evidence, response.Ambiguous)
	response.Sufficient = decision.Sufficient
	response.AnswerState = decision.State
	response.SufficiencyReason = decision.Reason
}
