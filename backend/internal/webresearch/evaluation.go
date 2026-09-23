package webresearch

import (
	"math"
	"sort"
	"strings"
)

// RetrievalMetrics contains the offline quality metrics used by the production
// release benchmark. Gold relevance values are graded (0 = irrelevant).
type RetrievalMetrics struct {
	RecallAtK       float64 `json:"recall_at_k"`
	MRR             float64 `json:"mrr"`
	NDCGAtK         float64 `json:"ndcg_at_k"`
	DomainDiversity float64 `json:"domain_diversity"`
}

// EvaluateRetrieval scores ranked search hits against canonical-URL relevance.
// It never performs network I/O, so the same evaluator is usable in CI and in
// an offline benchmark runner fed by captured provider results.
func EvaluateRetrieval(hits []SearchHit, gold map[string]float64, k int) RetrievalMetrics {
	if k <= 0 || k > len(hits) {
		k = len(hits)
	}
	goldRelevant := 0
	for _, grade := range gold {
		if grade > 0 {
			goldRelevant++
		}
	}
	matched := make(map[string]struct{})
	firstRelevantRank := 0
	dcg := 0.0
	domains := make(map[string]struct{})
	for i := 0; i < k; i++ {
		hit := hits[i]
		canonical := canonicalizeURL(hit.URL)
		grade := relevanceForCanonicalURL(gold, canonical)
		if grade > 0 {
			matched[canonical] = struct{}{}
			if firstRelevantRank == 0 {
				firstRelevantRank = i + 1
			}
		}
		dcg += discountedGain(grade, i+1)
		if domain := domainOf(hit.URL); domain != "" {
			domains[domain] = struct{}{}
		}
	}
	idealGrades := make([]float64, 0, len(gold))
	for _, grade := range gold {
		if grade > 0 {
			idealGrades = append(idealGrades, grade)
		}
	}
	sort.Slice(idealGrades, func(i, j int) bool { return idealGrades[i] > idealGrades[j] })
	idcg := 0.0
	for i := 0; i < len(idealGrades) && i < k; i++ {
		idcg += discountedGain(idealGrades[i], i+1)
	}
	metrics := RetrievalMetrics{}
	if goldRelevant > 0 {
		metrics.RecallAtK = float64(len(matched)) / float64(goldRelevant)
	}
	if firstRelevantRank > 0 {
		metrics.MRR = 1 / float64(firstRelevantRank)
	}
	if idcg > 0 {
		metrics.NDCGAtK = dcg / idcg
	}
	if k > 0 {
		metrics.DomainDiversity = float64(len(domains)) / float64(k)
	}
	return metrics
}

func discountedGain(grade float64, rank int) float64 {
	if grade <= 0 || rank <= 0 {
		return 0
	}
	return (math.Pow(2, grade) - 1) / math.Log2(float64(rank)+1)
}

func relevanceForCanonicalURL(gold map[string]float64, canonical string) float64 {
	if canonical == "" {
		return 0
	}
	for raw, grade := range gold {
		if canonicalizeURL(raw) == canonical {
			return grade
		}
	}
	return 0
}

// CitationBenchmarkMetrics measures mapping quality independently from prose
// quality. CorrectEvidence contains the citation/evidence IDs that are valid
// for the gold claims in one benchmark case.
type CitationBenchmarkMetrics struct {
	Precision   float64 `json:"precision"`
	Recall      float64 `json:"recall"`
	Correctness float64 `json:"correctness"`
	InvalidRate float64 `json:"invalid_rate"`
}

func EvaluateCitationIDs(predicted, gold []string) CitationBenchmarkMetrics {
	goldSet := stringSet(gold)
	predictedSet := stringSet(predicted)
	correct := 0
	for id := range predictedSet {
		if _, ok := goldSet[id]; ok {
			correct++
		}
	}
	metrics := CitationBenchmarkMetrics{}
	if len(predictedSet) > 0 {
		metrics.Precision = float64(correct) / float64(len(predictedSet))
		metrics.InvalidRate = float64(len(predictedSet)-correct) / float64(len(predictedSet))
	}
	if len(goldSet) > 0 {
		metrics.Recall = float64(correct) / float64(len(goldSet))
	}
	if len(predictedSet) > 0 || len(goldSet) > 0 {
		denom := maxInt(len(predictedSet), len(goldSet))
		if denom > 0 {
			metrics.Correctness = float64(correct) / float64(denom)
		}
	}
	return metrics
}

// DeepResearchBenchmarkMetrics scores the structural properties of a research
// run without asking a second LLM to grade it.
type DeepResearchBenchmarkMetrics struct {
	QuestionCoverage float64 `json:"question_coverage"`
	SourceCoverage   float64 `json:"source_coverage"`
	ConflictHandling float64 `json:"conflict_handling"`
}

func EvaluateDeepResearch(summary *ResearchSummary, graph *EvidenceGraphSummary, requiredSources []string) DeepResearchBenchmarkMetrics {
	metrics := DeepResearchBenchmarkMetrics{}
	if summary == nil {
		return metrics
	}
	if summary.Plan != nil {
		required := 0
		covered := 0
		byID := make(map[string]ResearchFinding, len(summary.Findings))
		for _, finding := range summary.Findings {
			byID[finding.QuestionID] = finding
		}
		for _, question := range summary.Plan.Questions {
			if !question.Required {
				continue
			}
			required++
			if finding, ok := byID[question.ID]; ok && finding.Status != "insufficient" {
				covered++
			}
		}
		if required > 0 {
			metrics.QuestionCoverage = float64(covered) / float64(required)
		}
	}
	if len(requiredSources) > 0 {
		seen := make(map[string]struct{})
		for _, finding := range summary.Findings {
			for _, ref := range finding.SourceRefs {
				seen[strings.ToLower(strings.TrimSpace(ref))] = struct{}{}
			}
		}
		matched := 0
		for _, source := range requiredSources {
			if _, ok := seen[strings.ToLower(strings.TrimSpace(source))]; ok {
				matched++
			}
		}
		metrics.SourceCoverage = float64(matched) / float64(len(requiredSources))
	}
	if graph == nil || graph.ConflictCount == 0 {
		metrics.ConflictHandling = 1
	} else {
		handled := 0
		for _, conflict := range graph.Conflicts {
			if strings.TrimSpace(conflict.ResolutionHint) != "" {
				handled++
			}
		}
		metrics.ConflictHandling = float64(handled) / float64(graph.ConflictCount)
	}
	return metrics
}

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = struct{}{}
		}
	}
	return out
}
