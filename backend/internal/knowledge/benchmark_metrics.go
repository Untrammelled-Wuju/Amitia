package knowledge

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

type benchmarkObservedEvidence struct {
	EvidenceID     string
	SourceID       string
	SourceRevision int64
}

type benchmarkMetricSet struct {
	RecallAtK        float64
	MRR              float64
	NDCGAtK          float64
	ContextPrecision float64
	ContextRecall    float64
}

func normalizedBenchmarkLabels(labels []BenchmarkRelevantEvidence) []BenchmarkRelevantEvidence {
	out := make([]BenchmarkRelevantEvidence, 0, len(labels))
	indexByKey := map[string]int{}
	for _, label := range labels {
		label.EvidenceID = strings.TrimSpace(label.EvidenceID)
		label.SourceID = strings.TrimSpace(label.SourceID)
		if label.EvidenceID == "" && label.SourceID == "" {
			continue
		}
		if label.Grade < 1 {
			label.Grade = 1
		}
		if label.Grade > 3 {
			label.Grade = 3
		}
		key := label.EvidenceID + "\x00" + label.SourceID + "\x00"
		if label.SourceRevision > 0 {
			key += strconv.FormatInt(label.SourceRevision, 10)
		}
		if existing, ok := indexByKey[key]; ok {
			if label.Grade > out[existing].Grade {
				out[existing].Grade = label.Grade
			}
			continue
		}
		indexByKey[key] = len(out)
		out = append(out, label)
	}
	return out
}

func benchmarkLabelMatches(label BenchmarkRelevantEvidence, ev benchmarkObservedEvidence) bool {
	if label.EvidenceID != "" && label.EvidenceID != ev.EvidenceID {
		return false
	}
	if label.SourceID != "" && label.SourceID != ev.SourceID {
		return false
	}
	if label.SourceRevision > 0 && label.SourceRevision != ev.SourceRevision {
		return false
	}
	return true
}

func consumeBestBenchmarkLabel(labels []BenchmarkRelevantEvidence, used []bool, ev benchmarkObservedEvidence) (int, int) {
	bestIndex, bestGrade := -1, 0
	for i, label := range labels {
		if used[i] || !benchmarkLabelMatches(label, ev) {
			continue
		}
		if label.Grade > bestGrade {
			bestIndex, bestGrade = i, label.Grade
		}
	}
	if bestIndex >= 0 {
		used[bestIndex] = true
	}
	return bestIndex, bestGrade
}

func evaluateBenchmarkRanking(observed []benchmarkObservedEvidence, labels []BenchmarkRelevantEvidence, k int) benchmarkMetricSet {
	labels = normalizedBenchmarkLabels(labels)
	if k <= 0 {
		k = 8
	}
	if len(labels) == 0 {
		return benchmarkMetricSet{}
	}
	if k > len(observed) {
		k = len(observed)
	}

	usedTopK := make([]bool, len(labels))
	matchedTopK := 0
	firstRelevantRank := 0
	dcg := 0.0
	for i := 0; i < k; i++ {
		idx, grade := consumeBestBenchmarkLabel(labels, usedTopK, observed[i])
		if idx < 0 {
			continue
		}
		matchedTopK++
		if firstRelevantRank == 0 {
			firstRelevantRank = i + 1
		}
		dcg += (math.Pow(2, float64(grade)) - 1) / math.Log2(float64(i)+2)
	}
	grades := make([]int, 0, len(labels))
	for _, label := range labels {
		grades = append(grades, label.Grade)
	}
	sort.SliceStable(grades, func(i, j int) bool { return grades[i] > grades[j] })
	idealLimit := k
	if idealLimit > len(grades) {
		idealLimit = len(grades)
	}
	idcg := 0.0
	for i := 0; i < idealLimit; i++ {
		idcg += (math.Pow(2, float64(grades[i])) - 1) / math.Log2(float64(i)+2)
	}

	usedContext := make([]bool, len(labels))
	matchedContextLabels := 0
	matchedContextEvidence := 0
	for _, ev := range observed {
		idx, _ := consumeBestBenchmarkLabel(labels, usedContext, ev)
		if idx >= 0 {
			matchedContextLabels++
			matchedContextEvidence++
		}
	}

	metrics := benchmarkMetricSet{
		RecallAtK:     float64(matchedTopK) / float64(len(labels)),
		ContextRecall: float64(matchedContextLabels) / float64(len(labels)),
	}
	if firstRelevantRank > 0 {
		metrics.MRR = 1 / float64(firstRelevantRank)
	}
	if idcg > 0 {
		metrics.NDCGAtK = dcg / idcg
	}
	if len(observed) > 0 {
		metrics.ContextPrecision = float64(matchedContextEvidence) / float64(len(observed))
	}
	return metrics
}

func evaluateBenchmarkCitations(citationIDs []string, observed []benchmarkObservedEvidence, labels []BenchmarkRelevantEvidence) (precision, recall, groundedness float64, ok bool) {
	labels = normalizedBenchmarkLabels(labels)
	if len(citationIDs) == 0 {
		return 0, 0, 0, false
	}
	observedByID := make(map[string]benchmarkObservedEvidence, len(observed))
	for _, ev := range observed {
		if id := strings.TrimSpace(ev.EvidenceID); id != "" {
			observedByID[id] = ev
		}
	}
	seenCitation := map[string]struct{}{}
	usedLabels := make([]bool, len(labels))
	total, relevant, grounded := 0, 0, 0
	for _, raw := range citationIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, exists := seenCitation[id]; exists {
			continue
		}
		seenCitation[id] = struct{}{}
		total++
		ev, retrieved := observedByID[id]
		if !retrieved {
			continue
		}
		idx, _ := consumeBestBenchmarkLabel(labels, usedLabels, ev)
		if idx >= 0 {
			relevant++
			grounded++
		}
	}
	if total == 0 {
		return 0, 0, 0, false
	}
	precision = float64(relevant) / float64(total)
	groundedness = float64(grounded) / float64(total)
	if len(labels) > 0 {
		recall = float64(relevant) / float64(len(labels))
	}
	return precision, recall, groundedness, true
}

func benchmarkPercentile(values []int64, percentile float64) int64 {
	if len(values) == 0 {
		return 0
	}
	copyValues := append([]int64(nil), values...)
	sort.Slice(copyValues, func(i, j int) bool { return copyValues[i] < copyValues[j] })
	if percentile <= 0 {
		return copyValues[0]
	}
	if percentile >= 1 {
		return copyValues[len(copyValues)-1]
	}
	index := int(math.Ceil(percentile*float64(len(copyValues)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(copyValues) {
		index = len(copyValues) - 1
	}
	return copyValues[index]
}
