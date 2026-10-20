package knowledge

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	RetrievalLevelAuto    = "auto"
	RetrievalLevelNone    = "none"
	RetrievalLevelFast    = "fast"
	RetrievalLevelDeep    = "deep"
	RetrievalLevelAgentic = "agentic"

	QueryComplexitySimple     = "simple"
	QueryComplexityComplex    = "complex"
	QueryComplexityComparison = "comparison"
)

type queryPlan struct {
	OriginalQuery              string
	EffectiveQuery             string
	QueryHistory               []string
	Complexity                 string
	Level                      string
	Timeout                    time.Duration
	CandidateLimit             int
	UseDense                   bool
	UseRerank                  bool
	ParentExpansionTokenBudget int
	MaxAgenticRounds           int
}

func buildQueryPlan(req SearchRequest, resultLimit int, runtime ...runtimeSettings) (queryPlan, error) {
	settings := loadRuntimeSettings()
	if len(runtime) > 0 {
		settings = runtime[0]
	}
	original := strings.TrimSpace(req.Query)
	if original == "" {
		return queryPlan{}, fmt.Errorf("query required")
	}
	if utf8.RuneCountInString(original) > 4096 {
		return queryPlan{}, fmt.Errorf("query is too long")
	}
	contextual := strings.TrimSpace(req.ContextualQuery)
	if utf8.RuneCountInString(contextual) > 8192 {
		return queryPlan{}, fmt.Errorf("contextualQuery is too long")
	}

	effective := original
	if contextual != "" && isShortReferentialQuery(original) {
		// Contextual query may only supplement the original. It never replaces
		// the user's literal wording, which remains the first retrieval query.
		effective = contextual + "\nFollow-up: " + original
	}
	complexity := classifyQueryComplexity(original, effective)
	level := strings.ToLower(strings.TrimSpace(req.RetrievalLevel))
	if level == "" {
		level = RetrievalLevelAuto
	}
	switch level {
	case RetrievalLevelAuto:
		if complexity == QueryComplexitySimple {
			level = RetrievalLevelFast
		} else {
			level = RetrievalLevelDeep
		}
	case RetrievalLevelNone, RetrievalLevelFast, RetrievalLevelDeep, RetrievalLevelAgentic:
	default:
		return queryPlan{}, fmt.Errorf("retrievalLevel must be auto, none, fast, deep, or agentic")
	}

	plan := queryPlan{
		OriginalQuery: original, EffectiveQuery: effective, Complexity: complexity, Level: level,
		MaxAgenticRounds: 1,
	}
	switch level {
	case RetrievalLevelNone:
		plan.Timeout = 0
		plan.CandidateLimit = 0
		plan.QueryHistory = []string{original}
		return plan, nil
	case RetrievalLevelFast:
		plan.Timeout = settings.FastTimeout
		plan.CandidateLimit = clampCandidateBudget(resultLimit*4, settings.FastMinCandidates, settings.FastMaxCandidates)
		plan.UseDense = true
		plan.UseRerank = false
		plan.ParentExpansionTokenBudget = settings.FastParentTokens
	case RetrievalLevelDeep:
		plan.Timeout = settings.DeepTimeout
		plan.CandidateLimit = clampCandidateBudget(resultLimit*8, settings.DeepMinCandidates, settings.DeepMaxCandidates)
		plan.UseDense = true
		plan.UseRerank = true
		plan.ParentExpansionTokenBudget = settings.DeepParentTokens
	case RetrievalLevelAgentic:
		plan.Timeout = settings.AgenticTimeout
		plan.CandidateLimit = clampCandidateBudget(resultLimit*10, settings.AgenticMinCandidates, settings.AgenticMaxCandidates)
		plan.UseDense = true
		plan.UseRerank = true
		plan.ParentExpansionTokenBudget = settings.AgenticParentTokens
		plan.MaxAgenticRounds = settings.MaxAgenticRounds
	}

	history := []string{original}
	if effective != original {
		history = appendUniqueQuery(history, effective)
	}
	if level == RetrievalLevelDeep || level == RetrievalLevelAgentic {
		for _, sub := range decomposeQuery(original, complexity, 4) {
			history = appendUniqueQuery(history, sub)
		}
	}
	// AGENTIC is deliberately bounded. Without an LLM planner inside the
	// retrieval package, rounds are deterministic refinements of the same
	// user-grounded query rather than unconstrained autonomous generation.
	if level == RetrievalLevelAgentic && len(history) < plan.MaxAgenticRounds {
		normalized := normalizeQueryWhitespace(effective)
		if normalized != "" {
			history = appendUniqueQuery(history, normalized)
		}
	}
	plan.QueryHistory = history
	return plan, nil
}

func clampCandidateBudget(v, low, high int) int {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func appendUniqueQuery(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if strings.EqualFold(strings.TrimSpace(existing), value) {
			return values
		}
	}
	return append(values, value)
}

func normalizeQueryWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func isShortReferentialQuery(query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" || utf8.RuneCountInString(q) > 24 {
		return false
	}
	markers := []string{"这个", "那个", "它", "他们", "它们", "上述", "前者", "后者", "这点", "那点", "继续", "这个呢", "那个呢", "what about", "how about", "this", "that", " it ", "they", "them"}
	padded := " " + q + " "
	for _, marker := range markers {
		if strings.Contains(padded, marker) {
			return true
		}
	}
	return false
}

func classifyQueryComplexity(original, effective string) string {
	if isComparisonQuery(original) {
		return QueryComplexityComparison
	}
	q := strings.TrimSpace(effective)
	if utf8.RuneCountInString(q) >= 120 {
		return QueryComplexityComplex
	}
	separators := strings.Count(q, "？") + strings.Count(q, "?") + strings.Count(q, ";") + strings.Count(q, "；")
	complexMarkers := []string{"分别", "同时", "并且", "以及", "原因和", "步骤和", "为什么以及", "先", "然后", "最后", "analyze and", "explain and", "compare and"}
	lower := strings.ToLower(q)
	for _, marker := range complexMarkers {
		if strings.Contains(lower, marker) {
			return QueryComplexityComplex
		}
	}
	if separators >= 2 {
		return QueryComplexityComplex
	}
	return QueryComplexitySimple
}

var comparisonSplitRE = regexp.MustCompile(`(?i)\s+(?:vs\.?|versus)\s+`)
var chineseComparisonRE = regexp.MustCompile(`^\s*(.+?)\s*(?:和|与)\s*(.+?)(?:有什么|有何)?(?:区别|差异|异同|优缺点)\s*[？?]*\s*$`)

func decomposeQuery(query, complexity string, max int) []string {
	if max <= 0 || (complexity != QueryComplexityComparison && complexity != QueryComplexityComplex) {
		return nil
	}
	query = strings.TrimSpace(query)
	out := []string{}
	if complexity == QueryComplexityComparison {
		if parts := comparisonSplitRE.Split(query, 3); len(parts) == 2 {
			left, right := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			if left != "" && right != "" {
				out = appendUniqueQuery(out, left)
				out = appendUniqueQuery(out, right)
			}
		}
		if len(out) == 0 {
			if matches := chineseComparisonRE.FindStringSubmatch(query); len(matches) == 3 {
				out = appendUniqueQuery(out, matches[1])
				out = appendUniqueQuery(out, matches[2])
			}
		}
		// For Chinese comparisons, split only on explicit comparison syntax;
		// avoid treating every ordinary conjunction as a decomposition signal.
		for _, marker := range []string{"相比", "对比", "比较"} {
			if idx := strings.Index(query, marker); idx > 0 {
				left := strings.TrimSpace(query[:idx])
				right := strings.TrimSpace(query[idx+len(marker):])
				right = strings.TrimLeft(right, "：:，, ")
				if left != "" && right != "" {
					out = appendUniqueQuery(out, left)
					out = appendUniqueQuery(out, right)
				}
				break
			}
		}
	}
	// Complex multi-clause questions are decomposed only at strong clause
	// boundaries. The original query is always searched separately as fallback.
	if len(out) == 0 {
		parts := strings.FieldsFunc(query, func(r rune) bool {
			return r == ';' || r == '；' || r == '\n'
		})
		if len(parts) > 1 {
			for _, part := range parts {
				out = appendUniqueQuery(out, part)
			}
		}
	}
	if len(out) > max {
		out = out[:max]
	}
	return out
}
