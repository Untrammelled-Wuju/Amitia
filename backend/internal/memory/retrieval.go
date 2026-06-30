// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"math"
	"sort"
	"strings"
	"time"
)

type RetrievalScorer struct{}

func (rs *RetrievalScorer) halflifeDays(memoryType string) int {
	switch memoryType {
	case "episodic":
		return 30
	case "profile", "user_profile":
		return 90
	case "fact", "structured_fact", "custom":
		return 180
	case "worldbook", "world_book":
		return 365
	default:
		return 180
	}
}

func (rs *RetrievalScorer) TimeDecay(createdAt string, memoryType string, baseScore float64) float64 {
	t, err := time.Parse("2006-01-02 15:04:05", createdAt)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05Z", createdAt)
		if err != nil {
			return baseScore
		}
	}
	ageHours := time.Since(t).Hours()
	ageDays := ageHours / 24
	halflife := float64(rs.halflifeDays(memoryType))
	decay := math.Pow(2, -ageDays/halflife)
	return baseScore * decay
}

func (rs *RetrievalScorer) FrequencyBoost(useCount int, baseScore float64) float64 {
	boost := math.Log(1 + float64(useCount))
	return baseScore * (1 + boost*0.1)
}

type scoredResult struct {
	id        string
	score     float64
	value     string
	createdAt string
	memory    Memory
	rest      HybridSearchResult
}

func (rs *RetrievalScorer) JaccardDedup(results []scoredResult, threshold float64) []scoredResult {
	if len(results) <= 1 {
		return results
	}

	tokenSets := make([]map[string]bool, len(results))
	for i, r := range results {
		tokens := strings.Fields(strings.ToLower(r.value))
		set := make(map[string]bool, len(tokens))
		for _, t := range tokens {
			set[t] = true
		}
		tokenSets[i] = set
	}

	kept := make([]bool, len(results))
	for i := range kept {
		kept[i] = true
	}

	for i := 0; i < len(results); i++ {
		if !kept[i] {
			continue
		}
		for j := i + 1; j < len(results); j++ {
			if !kept[j] {
				continue
			}
			sim := jaccardSetSimilarity(tokenSets[i], tokenSets[j])
			if sim > threshold {
				if results[i].createdAt >= results[j].createdAt {
					kept[j] = false
				} else {
					kept[i] = false
					break
				}
			}
		}
	}

	var deduped []scoredResult
	for i, k := range kept {
		if k {
			deduped = append(deduped, results[i])
		}
	}
	return deduped
}

func jaccardSetSimilarity(a, b map[string]bool) float64 {
	intersection := 0
	union := 0
	for k := range a {
		if b[k] {
			intersection++
		}
		union++
	}
	for k := range b {
		if !a[k] {
			union++
		}
	}
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func (rs *RetrievalScorer) Pipeline(vectorResults []VectorSearchResult) []HybridSearchResult {
	results := make([]scoredResult, 0, len(vectorResults))
	for _, r := range vectorResults {
		baseScore := float64(r.Score)
		baseScore = rs.TimeDecay(r.Memory.CreatedAt, r.Memory.MemoryType, baseScore)
		baseScore = rs.FrequencyBoost(r.Memory.UseCount, baseScore)
		results = append(results, scoredResult{
			id:        r.Memory.ID,
			score:     baseScore,
			value:     r.Memory.Value,
			createdAt: r.Memory.CreatedAt,
			memory:    r.Memory,
			rest: HybridSearchResult{
				Memory:         r.Memory,
				Score:          baseScore,
				VectorScore:    float64(r.Score),
				MatchType:      "vector",
				CollectionName: r.CollectionName,
				MemoryLayer:    r.MemoryLayer,
			},
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	results = rs.JaccardDedup(results, 0.85)

	out := make([]HybridSearchResult, 0, len(results))
	for _, r := range results {
		hr := r.rest
		hr.Score = math.Round(r.score*10000) / 10000
		hr.VectorScore = math.Round(hr.VectorScore*10000) / 10000
		out = append(out, hr)
	}
	return out
}
