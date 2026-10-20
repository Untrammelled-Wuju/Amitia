package knowledge

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxBenchmarkCases         = 100
	maxBenchmarkLabelsPerCase = 64
	maxBenchmarkCitations     = 64
	maxBenchmarkScenarioRunes = 96
)

type benchmarkSearchFunc func(context.Context, string, SearchRequest) (SearchResponse, error)

func validateBenchmarkRequest(req BenchmarkRequest) error {
	if len(req.Cases) == 0 {
		return fmt.Errorf("benchmark cases required")
	}
	if len(req.Cases) > maxBenchmarkCases {
		return fmt.Errorf("benchmark cases exceed limit %d", maxBenchmarkCases)
	}
	if req.K < 0 || req.K > 30 {
		return fmt.Errorf("benchmark k must be between 1 and 30")
	}
	if err := validateBenchmarkGate(req.Gate); err != nil {
		return err
	}
	for i, item := range req.Cases {
		query := strings.TrimSpace(item.Query)
		if query == "" {
			return fmt.Errorf("benchmark case %d query required", i+1)
		}
		if utf8.RuneCountInString(query) > 4096 {
			return fmt.Errorf("benchmark case %d query is too long", i+1)
		}
		if utf8.RuneCountInString(strings.TrimSpace(item.Scenario)) > maxBenchmarkScenarioRunes {
			return fmt.Errorf("benchmark case %d scenario is too long", i+1)
		}
		if len(item.Relevant) > maxBenchmarkLabelsPerCase {
			return fmt.Errorf("benchmark case %d relevant labels exceed limit %d", i+1, maxBenchmarkLabelsPerCase)
		}
		if len(item.CitationEvidenceIDs) > maxBenchmarkCitations {
			return fmt.Errorf("benchmark case %d citation labels exceed limit %d", i+1, maxBenchmarkCitations)
		}
		if !item.ExpectedNoAnswer && len(normalizedBenchmarkLabels(item.Relevant)) == 0 {
			return fmt.Errorf("benchmark case %d must provide relevant labels unless expectedNoAnswer=true", i+1)
		}
		if item.Limit < 0 || item.Limit > 30 {
			return fmt.Errorf("benchmark case %d limit must be between 1 and 30", i+1)
		}
		if len(item.SourceIDs) > 64 {
			return fmt.Errorf("benchmark case %d sourceIds exceed limit 64", i+1)
		}
		if err := validateBenchmarkCaseExpectation(item.Expect); err != nil {
			return fmt.Errorf("benchmark case %d: %w", i+1, err)
		}
		limit := item.Limit
		if limit <= 0 {
			limit = req.K
		}
		if limit <= 0 {
			limit = 8
		}
		searchReq := SearchRequest{Query: item.Query, ContextualQuery: item.ContextualQuery, RetrievalLevel: item.RetrievalLevel, Limit: limit, TemporalMode: item.TemporalMode, AsOf: item.AsOf}
		if _, err := buildQueryPlan(searchReq, limit, defaultRuntimeSettings()); err != nil {
			return fmt.Errorf("benchmark case %d: %w", i+1, err)
		}
		if _, err := resolveTemporalQuery(searchReq); err != nil {
			return fmt.Errorf("benchmark case %d: %w", i+1, err)
		}
	}
	return nil
}

func validateBenchmarkGate(gate BenchmarkGate) error {
	for name, value := range map[string]*float64{
		"minRecallAtK":        gate.MinRecallAtK,
		"minMrr":              gate.MinMRR,
		"minNdcgAtK":          gate.MinNDCGAtK,
		"minContextRecall":    gate.MinContextRecall,
		"minNoAnswerAccuracy": gate.MinNoAnswerAccuracy,
		"maxErrorRate":        gate.MaxErrorRate,
	} {
		if value != nil && (*value < 0 || *value > 1) {
			return fmt.Errorf("benchmark gate %s must be between 0 and 1", name)
		}
	}
	if gate.MaxP95LatencyMs < 0 {
		return fmt.Errorf("benchmark gate maxP95LatencyMs must be non-negative")
	}
	return nil
}

func validateBenchmarkCaseExpectation(expect BenchmarkCaseExpectation) error {
	for name, value := range map[string]*float64{
		"minRecallAtK":     expect.MinRecallAtK,
		"minContextRecall": expect.MinContextRecall,
	} {
		if value != nil && (*value < 0 || *value > 1) {
			return fmt.Errorf("expect.%s must be between 0 and 1", name)
		}
	}
	if expect.MaxLatencyMs < 0 {
		return fmt.Errorf("expect.maxLatencyMs must be non-negative")
	}
	if level := strings.TrimSpace(expect.ExpectedRetrievalLevel); level != "" && !isKnownRetrievalLevel(level) {
		return fmt.Errorf("expect.expectedRetrievalLevel is invalid")
	}
	if state := strings.TrimSpace(expect.ExpectedAnswerState); state != "" && !isKnownAnswerState(state) {
		return fmt.Errorf("expect.expectedAnswerState is invalid")
	}
	return nil
}

func isKnownRetrievalLevel(level string) bool {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case RetrievalLevelNone, RetrievalLevelFast, RetrievalLevelDeep, RetrievalLevelAgentic:
		return true
	default:
		return false
	}
}

func isKnownAnswerState(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case AnswerStateSufficient, AnswerStateInsufficient, AnswerStateAmbiguous:
		return true
	default:
		return false
	}
}

func benchmarkScenario(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "default"
	}
	return value
}

func runRetrievalBenchmark(ctx context.Context, tenantID string, req BenchmarkRequest, settings runtimeSettings, search benchmarkSearchFunc) (BenchmarkResponse, error) {
	if err := validateBenchmarkRequest(req); err != nil {
		return BenchmarkResponse{}, err
	}
	if search == nil {
		return BenchmarkResponse{}, fmt.Errorf("benchmark retrieval runtime unavailable")
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return BenchmarkResponse{}, fmt.Errorf("tenant scope required")
	}
	k := req.K
	if k <= 0 {
		k = 8
	}
	response := BenchmarkResponse{
		Summary: BenchmarkSummary{
			ConfigHash:       settings.ConfigHash,
			K:                k,
			Cases:            len(req.Cases),
			GroundednessMode: "label_overlap",
			ScenarioCounts:   map[string]int{},
			Passed:           true,
		},
		Results: make([]BenchmarkCaseResult, 0, len(req.Cases)),
	}
	var (
		positiveCount, noAnswerCount, noAnswerCorrect, citationCases int
		recallSum, mrrSum, ndcgSum, precisionSum, contextRecallSum   float64
		citationPrecisionSum, citationRecallSum, groundednessSum     float64
		latencies                                                    []int64
	)
	for index, item := range req.Cases {
		if err := ctx.Err(); err != nil {
			return response, err
		}
		caseID := strings.TrimSpace(item.ID)
		if caseID == "" {
			caseID = fmt.Sprintf("case-%03d", index+1)
		}
		scenario := benchmarkScenario(item.Scenario)
		response.Summary.ScenarioCounts[scenario]++
		limit := item.Limit
		if limit <= 0 {
			limit = k
		}
		if limit < k {
			limit = k
		}
		searchReq := SearchRequest{
			Query: item.Query, ContextualQuery: item.ContextualQuery, RetrievalLevel: item.RetrievalLevel,
			Limit: limit, SourceIDs: append([]string(nil), item.SourceIDs...), TemporalMode: item.TemporalMode,
			AsOf: item.AsOf, SkipTrace: true,
		}
		started := time.Now()
		searchResp, searchErr := search(ctx, tenantID, searchReq)
		latency := time.Since(started).Milliseconds()
		result := BenchmarkCaseResult{
			ID: caseID, Scenario: scenario, LatencyMs: latency, RetrievalLevel: searchResp.RetrievalLevel,
			AnswerState: searchResp.AnswerState, EvidenceIDs: []string{}, Passed: true,
		}
		if searchErr != nil {
			result.Error = searchErr.Error()
			result.Passed = false
			result.Violations = append(result.Violations, "search_error")
			response.Summary.ErrorCases++
			response.Summary.FailedCases++
			response.Results = append(response.Results, result)
			continue
		}
		response.Summary.SuccessfulCases++
		latencies = append(latencies, latency)
		observed := make([]benchmarkObservedEvidence, 0, len(searchResp.Evidence))
		for _, ev := range searchResp.Evidence {
			observed = append(observed, benchmarkObservedEvidence{EvidenceID: ev.ID, SourceID: ev.SourceID, SourceRevision: ev.SourceRevision})
			result.EvidenceIDs = append(result.EvidenceIDs, ev.ID)
		}
		if item.ExpectedNoAnswer {
			noAnswerCount++
			result.NoAnswerCorrect = !searchResp.Sufficient && searchResp.AnswerState == AnswerStateInsufficient
			if !result.NoAnswerCorrect {
				result.Violations = append(result.Violations, "expected_no_answer")
			}
		} else {
			metrics := evaluateBenchmarkRanking(observed, item.Relevant, k)
			result.RecallAtK = metrics.RecallAtK
			result.MRR = metrics.MRR
			result.NDCGAtK = metrics.NDCGAtK
			result.ContextPrecision = metrics.ContextPrecision
			result.ContextRecall = metrics.ContextRecall
			positiveCount++
			recallSum += metrics.RecallAtK
			mrrSum += metrics.MRR
			ndcgSum += metrics.NDCGAtK
			precisionSum += metrics.ContextPrecision
			contextRecallSum += metrics.ContextRecall
		}
		if result.NoAnswerCorrect {
			noAnswerCorrect++
		}
		if cp, cr, grounded, ok := evaluateBenchmarkCitations(item.CitationEvidenceIDs, observed, item.Relevant); ok {
			result.CitationPrecision = floatPtr(cp)
			result.CitationRecall = floatPtr(cr)
			result.Groundedness = floatPtr(grounded)
			citationCases++
			citationPrecisionSum += cp
			citationRecallSum += cr
			groundednessSum += grounded
		}
		applyBenchmarkCaseExpectation(&result, item.Expect)
		if len(result.Violations) > 0 {
			result.Passed = false
			response.Summary.FailedCases++
		}
		response.Results = append(response.Results, result)
	}
	response.Summary.PositiveCases = positiveCount
	response.Summary.NoAnswerCases = noAnswerCount
	response.Summary.CitationObservedCases = citationCases
	if positiveCount > 0 {
		denom := float64(positiveCount)
		response.Summary.RecallAtK = recallSum / denom
		response.Summary.MRR = mrrSum / denom
		response.Summary.NDCGAtK = ndcgSum / denom
		response.Summary.ContextPrecision = precisionSum / denom
		response.Summary.ContextRecall = contextRecallSum / denom
	}
	if noAnswerCount > 0 {
		response.Summary.NoAnswerAccuracy = float64(noAnswerCorrect) / float64(noAnswerCount)
	}
	if citationCases > 0 {
		denom := float64(citationCases)
		response.Summary.CitationPrecision = citationPrecisionSum / denom
		response.Summary.CitationRecall = citationRecallSum / denom
		response.Summary.Groundedness = groundednessSum / denom
	}
	if response.Summary.Cases > 0 {
		response.Summary.ErrorRate = float64(response.Summary.ErrorCases) / float64(response.Summary.Cases)
	}
	response.Summary.P50LatencyMs = benchmarkPercentile(latencies, 0.50)
	response.Summary.P95LatencyMs = benchmarkPercentile(latencies, 0.95)
	applyBenchmarkGate(&response.Summary, req.Gate)
	if response.Summary.FailedCases > 0 {
		response.Summary.Passed = false
		response.Summary.Violations = appendUniqueString(response.Summary.Violations, "case_expectation_failed")
	}
	return response, nil
}

func applyBenchmarkCaseExpectation(result *BenchmarkCaseResult, expect BenchmarkCaseExpectation) {
	if result == nil {
		return
	}
	if want := strings.TrimSpace(expect.ExpectedRetrievalLevel); want != "" && !strings.EqualFold(want, result.RetrievalLevel) {
		result.Violations = append(result.Violations, "retrieval_level_mismatch")
	}
	if want := strings.TrimSpace(expect.ExpectedAnswerState); want != "" && !strings.EqualFold(want, result.AnswerState) {
		result.Violations = append(result.Violations, "answer_state_mismatch")
	}
	if expect.MinRecallAtK != nil && result.RecallAtK < *expect.MinRecallAtK {
		result.Violations = append(result.Violations, "recall_below_min")
	}
	if expect.MinContextRecall != nil && result.ContextRecall < *expect.MinContextRecall {
		result.Violations = append(result.Violations, "context_recall_below_min")
	}
	if expect.MaxLatencyMs > 0 && result.LatencyMs > expect.MaxLatencyMs {
		result.Violations = append(result.Violations, "latency_above_max")
	}
}

func applyBenchmarkGate(summary *BenchmarkSummary, gate BenchmarkGate) {
	if summary == nil {
		return
	}
	if gate.MinRecallAtK != nil && summary.RecallAtK < *gate.MinRecallAtK {
		summary.Violations = append(summary.Violations, "recall_below_min")
	}
	if gate.MinMRR != nil && summary.MRR < *gate.MinMRR {
		summary.Violations = append(summary.Violations, "mrr_below_min")
	}
	if gate.MinNDCGAtK != nil && summary.NDCGAtK < *gate.MinNDCGAtK {
		summary.Violations = append(summary.Violations, "ndcg_below_min")
	}
	if gate.MinContextRecall != nil && summary.ContextRecall < *gate.MinContextRecall {
		summary.Violations = append(summary.Violations, "context_recall_below_min")
	}
	if gate.MinNoAnswerAccuracy != nil && summary.NoAnswerAccuracy < *gate.MinNoAnswerAccuracy {
		summary.Violations = append(summary.Violations, "no_answer_accuracy_below_min")
	}
	if gate.MaxP95LatencyMs > 0 && summary.P95LatencyMs > gate.MaxP95LatencyMs {
		summary.Violations = append(summary.Violations, "p95_latency_above_max")
	}
	if gate.MaxErrorRate != nil && summary.ErrorRate > *gate.MaxErrorRate {
		summary.Violations = append(summary.Violations, "error_rate_above_max")
	}
	if len(summary.Violations) > 0 {
		summary.Passed = false
	}
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func floatPtr(value float64) *float64 { return &value }
