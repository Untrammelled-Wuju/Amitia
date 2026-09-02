package workspace

import (
	"context"
	"sort"
	"strings"
	"unicode"
)

// ContextSearchRequest describes an intent-oriented source search. Unlike the
// exact workspace.search tool it ranks code windows by the overlap between the
// user's natural-language intent, identifiers, file path and nearby lines.
// This keeps the feature local and deterministic without requiring an external
// embedding service.
type ContextSearchRequest struct {
	WorkspaceID  string   `json:"workspaceId"`
	Query        string   `json:"query"`
	IncludeGlobs []string `json:"includeGlobs,omitempty"`
	ExcludeGlobs []string `json:"excludeGlobs,omitempty"`
	MaxResults   int      `json:"maxResults"`
	ContextLines int      `json:"contextLines"`
}

type ContextSearchMatch struct {
	Path       string   `json:"path"`
	Line       int      `json:"line"`
	Score      float64  `json:"score"`
	LineText   string   `json:"lineText"`
	Context    []string `json:"context"`
	Matched    []string `json:"matchedTerms"`
	FileSHA256 string   `json:"fileSha256"`
}

type ContextSearchResult struct {
	WorkspaceID string               `json:"workspaceId"`
	Matches     []ContextSearchMatch `json:"matches"`
	Total       int                  `json:"total"`
	Truncated   bool                 `json:"truncated"`
}

func ContextSearch(ctx context.Context, svc *Service, req ContextSearchRequest) (*ContextSearchResult, error) {
	if svc == nil {
		return nil, ErrOperationUnsupported
	}
	req.WorkspaceID = strings.TrimSpace(req.WorkspaceID)
	req.Query = strings.TrimSpace(req.Query)
	if req.WorkspaceID == "" || req.Query == "" {
		return &ContextSearchResult{WorkspaceID: req.WorkspaceID, Matches: []ContextSearchMatch{}}, nil
	}
	if req.MaxResults <= 0 {
		req.MaxResults = 40
	}
	if req.MaxResults > 200 {
		req.MaxResults = 200
	}
	if req.ContextLines <= 0 {
		req.ContextLines = 3
	}
	if req.ContextLines > 10 {
		req.ContextLines = 10
	}

	queryTerms := contextTokens(req.Query)
	queryLower := strings.ToLower(req.Query)
	if len(queryTerms) == 0 {
		queryTerms = []string{queryLower}
	}

	walker := &defaultPreciseEditingService{service: svc}
	candidates := make([]ContextSearchMatch, 0, req.MaxResults*3)
	err := walker.walkWorkspace(ctx, req.WorkspaceID, func(filePath string, content []byte) error {
		if len(req.IncludeGlobs) > 0 && !matchesAnyGlob(filePath, req.IncludeGlobs) {
			return nil
		}
		if len(req.ExcludeGlobs) > 0 && matchesAnyGlob(filePath, req.ExcludeGlobs) {
			return nil
		}
		if !likelyText(content) {
			return nil
		}
		lines := splitLines(string(content))
		if len(lines) == 0 {
			return nil
		}
		fileHash := ComputeSHA256(content)
		pathLower := strings.ToLower(filePath)
		pathTerms := tokenSet(contextTokens(pathLower))
		for i, line := range lines {
			start := i - req.ContextLines
			if start < 0 {
				start = 0
			}
			end := i + req.ContextLines + 1
			if end > len(lines) {
				end = len(lines)
			}
			window := strings.Join(lines[start:end], "\n")
			score, matched := scoreContextWindow(queryLower, queryTerms, strings.ToLower(line), strings.ToLower(window), pathLower, pathTerms)
			if score <= 0 {
				continue
			}
			contextLines := append([]string(nil), lines[start:end]...)
			candidates = append(candidates, ContextSearchMatch{
				Path:       filePath,
				Line:       i + 1,
				Score:      score,
				LineText:   line,
				Context:    contextLines,
				Matched:    matched,
				FileSHA256: fileHash,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			if candidates[i].Path == candidates[j].Path {
				return candidates[i].Line < candidates[j].Line
			}
			return candidates[i].Path < candidates[j].Path
		}
		return candidates[i].Score > candidates[j].Score
	})

	// Collapse heavily overlapping windows from the same file so a single
	// symbol does not consume the whole result budget.
	selected := make([]ContextSearchMatch, 0, req.MaxResults)
	for _, candidate := range candidates {
		duplicate := false
		for _, existing := range selected {
			if existing.Path == candidate.Path && absInt(existing.Line-candidate.Line) <= req.ContextLines {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		selected = append(selected, candidate)
		if len(selected) >= req.MaxResults {
			break
		}
	}
	return &ContextSearchResult{
		WorkspaceID: req.WorkspaceID,
		Matches:     selected,
		Total:       len(selected),
		Truncated:   len(candidates) > len(selected),
	}, nil
}

func scoreContextWindow(query string, terms []string, line, window, path string, pathTerms map[string]struct{}) (float64, []string) {
	score := 0.0
	matched := make([]string, 0, len(terms))
	if len(query) >= 3 {
		if strings.Contains(line, query) {
			score += 14
		} else if strings.Contains(window, query) {
			score += 8
		}
	}
	lineTokens := tokenSet(contextTokens(line))
	windowTokens := tokenSet(contextTokens(window))
	for _, term := range terms {
		termScore := 0.0
		if _, ok := lineTokens[term]; ok {
			termScore += 4
		} else if strings.Contains(line, term) {
			termScore += 2.5
		}
		if _, ok := windowTokens[term]; ok {
			termScore += 1.5
		}
		if _, ok := pathTerms[term]; ok || strings.Contains(path, term) {
			termScore += 1.25
		}
		if termScore > 0 {
			score += termScore
			matched = append(matched, term)
		}
	}
	if len(terms) > 1 && len(matched) == len(terms) {
		score += 5
	}
	return score, matched
}

func contextTokens(value string) []string {
	value = splitIdentifierBoundaries(value)
	fields := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	seen := make(map[string]struct{}, len(fields))
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if len(field) < 2 || isContextStopWord(field) {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		out = append(out, field)
	}
	return out
}

func splitIdentifierBoundaries(value string) string {
	runes := []rune(value)
	var b strings.Builder
	b.Grow(len(value) + 16)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1])) {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func tokenSet(tokens []string) map[string]struct{} {
	out := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		out[token] = struct{}{}
	}
	return out
}

func likelyText(content []byte) bool {
	if len(content) == 0 {
		return true
	}
	limit := len(content)
	if limit > 8192 {
		limit = 8192
	}
	for _, b := range content[:limit] {
		if b == 0 {
			return false
		}
	}
	return true
}

func isContextStopWord(word string) bool {
	switch word {
	case "the", "a", "an", "and", "or", "of", "to", "in", "on", "for", "with", "from", "is", "are", "this", "that", "how", "where", "what", "which", "find", "code", "代码", "查找", "搜索", "实现", "相关":
		return true
	default:
		return false
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
