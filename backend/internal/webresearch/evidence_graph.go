package webresearch

import (
	"regexp"
	"sort"
	"strings"
)

var versionValuePattern = regexp.MustCompile(`(?i)\b(?:v(?:ersion)?\s*)?(\d+\.\d+(?:\.\d+)?)\b`)
var yearValuePattern = regexp.MustCompile(`\b(?:19|20)\d{2}\b`)

func buildEvidenceGraphSummary(output *ToolOutput) *EvidenceGraphSummary {
	if output == nil || (len(output.Search) == 0 && len(output.Pages) == 0 && len(output.Citations) == 0) {
		return nil
	}
	sources := make(map[string]struct{})
	searchSourceKeys := make(map[string]string)
	for _, hit := range output.Search {
		key := strings.TrimSpace(hit.URL)
		if key == "" {
			key = strings.TrimSpace(hit.RefID)
		}
		if key != "" {
			sources[key] = struct{}{}
			if refID := strings.TrimSpace(hit.RefID); refID != "" {
				searchSourceKeys[refID] = key
			}
		}
	}
	for _, page := range output.Pages {
		key := searchSourceKeys[strings.TrimSpace(page.SourceRefID)]
		if key == "" {
			key = strings.TrimSpace(page.CanonicalURL)
		}
		if key == "" {
			key = strings.TrimSpace(page.SourceRefID)
		}
		if key != "" {
			sources[key] = struct{}{}
		}
	}
	conflicts := detectEvidenceConflicts(output.Citations)
	claims := buildEvidenceClaims(output)
	return &EvidenceGraphSummary{
		SourceCount:   len(sources),
		PageCount:     len(output.Pages),
		EvidenceCount: len(output.Citations),
		ClaimCount:    len(claims),
		CitationCount: len(output.Citations),
		ConflictCount: len(conflicts),
		Claims:        claims,
		Conflicts:     conflicts,
	}
}

func buildEvidenceClaims(output *ToolOutput) []EvidenceClaim {
	if output == nil || output.Research == nil || len(output.Research.Findings) == 0 {
		return nil
	}
	citationByEvidence := make(map[string]int, len(output.Citations))
	for _, citation := range output.Citations {
		if citation.EvidenceID != "" && citation.Index > 0 {
			citationByEvidence[citation.EvidenceID] = citation.Index
		}
	}
	claims := make([]EvidenceClaim, 0, len(output.Research.Findings))
	for _, finding := range output.Research.Findings {
		text := strings.TrimSpace(finding.Summary)
		if text == "" {
			text = strings.TrimSpace(finding.Question)
		}
		if text == "" {
			continue
		}
		claim := EvidenceClaim{
			ID:          finding.QuestionID,
			Text:        text,
			Status:      finding.Status,
			EvidenceIDs: append([]string(nil), finding.EvidenceIDs...),
		}
		seenCitation := make(map[int]struct{})
		for _, evidenceID := range finding.EvidenceIDs {
			index := citationByEvidence[evidenceID]
			if index <= 0 {
				continue
			}
			if _, exists := seenCitation[index]; exists {
				continue
			}
			seenCitation[index] = struct{}{}
			claim.CitationIndexes = append(claim.CitationIndexes, index)
		}
		sort.Ints(claim.CitationIndexes)
		claims = append(claims, claim)
	}
	return claims
}

func detectEvidenceConflicts(citations []Citation) []EvidenceConflict {
	if len(citations) < 2 {
		return nil
	}
	const maxConflicts = 12
	conflicts := make([]EvidenceConflict, 0)
	for i := 0; i < len(citations); i++ {
		left := citations[i]
		if strings.TrimSpace(left.Text) == "" {
			continue
		}
		for j := i + 1; j < len(citations); j++ {
			right := citations[j]
			if left.RefID == right.RefID || strings.TrimSpace(right.Text) == "" {
				continue
			}
			leftScope := extractEvidenceScope(left.Text)
			rightScope := extractEvidenceScope(right.Text)
			if scopesHaveDifferentFactDomain(leftScope, rightScope) {
				continue
			}
			kind, reason, confidence := evidenceConflictPair(left.Text, right.Text)
			if kind == "" {
				continue
			}
			resolution := "compare primary sources within the same version, platform, region, and publication window"
			if scopesHaveDifferentYears(leftScope, rightScope) {
				kind = "temporal_change"
				reason = "similar evidence reports different status in different time scopes; the older source may be stale rather than contradictory"
				confidence = clamp01(confidence * 0.85)
				resolution = "prefer the newer primary source for current-state claims while retaining the older evidence as historical context"
			}
			conflicts = append(conflicts, EvidenceConflict{
				Kind:            kind,
				LeftEvidenceID:  left.EvidenceID,
				RightEvidenceID: right.EvidenceID,
				LeftRefID:       left.RefID,
				RightRefID:      right.RefID,
				Reason:          reason,
				Confidence:      confidence,
				LeftScope:       leftScope,
				RightScope:      rightScope,
				ResolutionHint:  resolution,
			})
			if len(conflicts) >= maxConflicts {
				return conflicts
			}
		}
	}
	return conflicts
}

func evidenceConflictPair(left, right string) (string, string, float64) {
	leftTokens := tokenize(strings.ToLower(left))
	rightTokens := tokenize(strings.ToLower(right))
	overlap := overlapScore(leftTokens, rightTokens)
	if overlap < 0.35 {
		return "", "", 0
	}

	leftVersions := distinctVersionValues(left)
	rightVersions := distinctVersionValues(right)
	if len(leftVersions) == 1 && len(rightVersions) == 1 && leftVersions[0] != rightVersions[0] && overlap >= 0.45 {
		return "value_conflict", "highly similar evidence reports different version values", clamp01(0.55 + overlap*0.35)
	}

	leftPolarity, leftTopic := evidencePolarity(left)
	rightPolarity, rightTopic := evidencePolarity(right)
	if leftPolarity != 0 && rightPolarity != 0 && leftPolarity != rightPolarity && leftTopic != "" && leftTopic == rightTopic && overlap >= 0.40 {
		return "status_conflict", "similar evidence makes opposing availability/release/support claims", clamp01(0.60 + overlap*0.30)
	}
	return "", "", 0
}

func distinctVersionValues(value string) []string {
	matches := versionValuePattern.FindAllStringSubmatch(value, -1)
	seen := map[string]struct{}{}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		version := strings.TrimSpace(match[1])
		if version == "" {
			continue
		}
		if _, ok := seen[version]; ok {
			continue
		}
		seen[version] = struct{}{}
		out = append(out, version)
	}
	sort.Strings(out)
	return out
}

func evidencePolarity(value string) (int, string) {
	lower := strings.ToLower(value)
	topics := []struct {
		name     string
		positive []string
		negative []string
	}{
		{name: "release", positive: []string{" released", "is released", "available now", "正式发布", "已发布", "已经发布"}, negative: []string{"not released", "unreleased", "not yet released", "尚未发布", "未发布"}},
		{name: "support", positive: []string{" supports ", "is supported", " support for ", "支持"}, negative: []string{"does not support", "doesn't support", "not supported", "unsupported", "不支持", "暂不支持"}},
		{name: "availability", positive: []string{"is available", "now available", "可用", "已上线"}, negative: []string{"not available", "unavailable", "不可用", "未上线"}},
	}
	for _, topic := range topics {
		for _, marker := range topic.negative {
			if strings.Contains(lower, marker) {
				return -1, topic.name
			}
		}
		for _, marker := range topic.positive {
			if strings.Contains(lower, marker) {
				return 1, topic.name
			}
		}
	}
	return 0, ""
}

func extractEvidenceScope(value string) EvidenceScope {
	lower := strings.ToLower(value)
	platformMarkers := []string{"windows", "linux", "macos", "android", "ios", "web", "desktop", "mobile"}
	regionMarkers := []string{"中国", "china", "美国", "united states", "usa", "europe", "eu", "global", "全球"}
	return EvidenceScope{
		Platforms: collectScopeMarkers(lower, platformMarkers),
		Regions:   collectScopeMarkers(lower, regionMarkers),
		Years:     distinctStringMatches(yearValuePattern.FindAllString(lower, -1)),
	}
}

func collectScopeMarkers(value string, markers []string) []string {
	out := make([]string, 0)
	for _, marker := range markers {
		if strings.Contains(value, marker) {
			out = append(out, marker)
		}
	}
	sort.Strings(out)
	return out
}

func distinctStringMatches(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func scopesHaveDifferentFactDomain(left, right EvidenceScope) bool {
	return disjointNonEmptyScopes(left.Platforms, right.Platforms) || disjointNonEmptyScopes(left.Regions, right.Regions)
}

func scopesHaveDifferentYears(left, right EvidenceScope) bool {
	return disjointNonEmptyScopes(left.Years, right.Years)
}

func disjointNonEmptyScopes(left, right []string) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(left))
	for _, value := range left {
		set[value] = struct{}{}
	}
	for _, value := range right {
		if _, ok := set[value]; ok {
			return false
		}
	}
	return true
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
