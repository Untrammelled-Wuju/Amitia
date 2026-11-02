package business

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type memoryRankingWeights struct {
	Semantic      float64 `json:"semantic"`
	Keyword       float64 `json:"keyword"`
	Importance    float64 `json:"importance"`
	Confidence    float64 `json:"confidence"`
	Strength      float64 `json:"strength"`
	Recency       float64 `json:"recency"`
	Verification  float64 `json:"verification"`
	Reinforcement float64 `json:"reinforcement"`
	Utility       float64 `json:"utility"`
	Validity      float64 `json:"validity"`
}

type memoryRankingProfile struct {
	Weights     memoryRankingWeights `json:"weights"`
	SampleCount int                  `json:"sampleCount"`
	UpdatedAt   string               `json:"updatedAt"`
}

type memoryRetrievalFeatures struct {
	Semantic      float64 `json:"semantic"`
	Keyword       float64 `json:"keyword"`
	Importance    float64 `json:"importance"`
	Confidence    float64 `json:"confidence"`
	Strength      float64 `json:"strength"`
	Recency       float64 `json:"recency"`
	Verification  float64 `json:"verification"`
	Reinforcement float64 `json:"reinforcement"`
	Utility       float64 `json:"utility"`
	Validity      float64 `json:"validity"`
}

func defaultMemoryRankingWeights() memoryRankingWeights {
	return memoryRankingWeights{Semantic: .44, Keyword: .16, Importance: .10, Confidence: .06, Strength: .08, Recency: .04, Verification: .04, Reinforcement: .02, Utility: .03, Validity: .03}
}

func (w memoryRankingWeights) normalize() memoryRankingWeights {
	values := []*float64{&w.Semantic, &w.Keyword, &w.Importance, &w.Confidence, &w.Strength, &w.Recency, &w.Verification, &w.Reinforcement, &w.Utility, &w.Validity}
	sum := 0.0
	for _, value := range values {
		*value = math.Max(.005, math.Min(.5, *value))
		sum += *value
	}
	if sum <= 0 {
		return defaultMemoryRankingWeights()
	}
	for _, value := range values {
		*value /= sum
	}
	return w
}

func (w memoryRankingWeights) vector() []float64 {
	return []float64{w.Semantic, w.Keyword, w.Importance, w.Confidence, w.Strength, w.Recency, w.Verification, w.Reinforcement, w.Utility, w.Validity}
}

func (f memoryRetrievalFeatures) vector() []float64 {
	return []float64{f.Semantic, f.Keyword, f.Importance, f.Confidence, f.Strength, f.Recency, f.Verification, f.Reinforcement, f.Utility, f.Validity}
}

func (e *Engine) memoryRankingProfile(ctx context.Context, scope coordination.ExecutionScope) (memoryRankingProfile, *coordination.Resource, error) {
	profile := memoryRankingProfile{Weights: defaultMemoryRankingWeights()}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return profile, nil, nil
	}
	resource, err := port.Resource(ctx, scope, "checkpoint", "memory-ranking-profile/"+scope.RoleID)
	if err != nil || resource == nil || resource.Deleted {
		return profile, resource, err
	}
	if err := json.Unmarshal(resource.Body, &profile); err != nil {
		return memoryRankingProfile{}, resource, err
	}
	profile.Weights = profile.Weights.normalize()
	return profile, resource, nil
}

func (e *Engine) memoryRankingProfileMutation(ctx context.Context, scope coordination.ExecutionScope, memoryDocument map[string]json.RawMessage, feedback string, now time.Time) (*coordination.Mutation, error) {
	profile, resource, err := e.memoryRankingProfile(ctx, scope)
	if err != nil {
		return nil, err
	}
	features := memoryRetrievalFeatures{Semantic: .5, Keyword: .5}
	if raw, exists := memoryDocument["lastRetrievalFeatures"]; exists {
		if err := json.Unmarshal(raw, &features); err != nil {
			return nil, err
		}
	}
	weights, featureValues := profile.Weights.vector(), features.vector()
	predicted := .5
	for index := range weights {
		predicted += weights[index] * (featureValues[index] - .5)
	}
	predicted = math.Max(0, math.Min(1, predicted))
	label := 1.0
	if feedback == "not_helpful" {
		label = 0
	}
	rate := .08
	for index := range weights {
		weights[index] += rate * (label - predicted) * (featureValues[index] - .5)
	}
	profile.SampleCount++
	profile.UpdatedAt = now.UTC().Format(time.RFC3339Nano)
	profile.Weights = memoryRankingWeights{
		Semantic: weights[0], Keyword: weights[1], Importance: weights[2], Confidence: weights[3], Strength: weights[4],
		Recency: weights[5], Verification: weights[6], Reinforcement: weights[7], Utility: weights[8], Validity: weights[9],
	}.normalize()
	revision := int64(0)
	if resource != nil && !resource.Deleted {
		revision = resource.Revision
	}
	mutation := coordination.Mutation{Kind: "checkpoint", ID: "memory-ranking-profile/" + scope.RoleID, RoleID: scope.RoleID, ExpectedRevision: revision, Body: body(profile)}
	return &mutation, nil
}
