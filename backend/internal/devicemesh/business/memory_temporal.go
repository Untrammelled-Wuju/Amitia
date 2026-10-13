package business

import (
	"context"
	"math"
	"regexp"
	"strings"
	"time"
)

type MemoryScoreCandidate struct {
	MemoryID   string  `json:"memoryId"`
	BaseScore  float64 `json:"baseScore"`
	CreatedAt  string  `json:"createdAt"`
	MemoryType string  `json:"memoryType"`
}

type MemoryTemporalPolicy interface {
	OwnedMemoryTemporalEnabled(context.Context, string) (bool, error)
}

type MemoryScoreResult struct {
	MemoryID        string  `json:"memoryId"`
	FinalScore      float64 `json:"finalScore"`
	TemporalBoost   float64 `json:"temporalBoost"`
	ValidityPenalty float64 `json:"validityPenalty"`
	ReferenceSource string  `json:"referenceSource"`
}

func RerankMemoryScores(ctx context.Context, query string, candidates []MemoryScoreCandidate, metadata map[string]MemoryTemporalMetadata, now time.Time) (map[string]MemoryScoreResult, error) {
	if !hasTemporalIntent(query) {
		results := make(map[string]MemoryScoreResult, len(candidates))
		for _, candidate := range candidates {
			results[candidate.MemoryID] = MemoryScoreResult{MemoryID: candidate.MemoryID, FinalScore: candidate.BaseScore, ValidityPenalty: 1, ReferenceSource: "none"}
		}
		return results, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	now = now.UTC()
	results := make(map[string]MemoryScoreResult, len(candidates))
	for _, candidate := range candidates {
		item, exists := metadata[candidate.MemoryID]
		var reference time.Time
		source := "none"
		if exists && item.OccurredAtUTC != nil {
			reference = item.OccurredAtUTC.UTC()
			source = "occurred_at_utc"
		}
		score := candidate.BaseScore
		if !reference.IsZero() && !reference.After(now) {
			score *= timeDecayFactor(now.Sub(reference), candidate.MemoryType)
		}
		validityPenalty := 1.0
		if exists && item.ValidToUTC != nil && now.After(*item.ValidToUTC) {
			validityPenalty = .65
		}
		if exists && item.ValidFromUTC != nil && now.Before(*item.ValidFromUTC) {
			validityPenalty = .75
		}
		resonance := temporalResonance(query, item, now)
		boost := math.Min(.18, resonance*.18)
		score = score * validityPenalty * (1 + boost)
		results[candidate.MemoryID] = MemoryScoreResult{MemoryID: candidate.MemoryID, FinalScore: roundScore(score), TemporalBoost: roundScore(boost), ValidityPenalty: validityPenalty, ReferenceSource: source}
	}
	return results, nil
}

func temporalResonance(query string, metadata MemoryTemporalMetadata, now time.Time) float64 {
	if metadata.OccurredAtUTC == nil {
		return 0
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if !hasTemporalIntent(query) {
		return 0
	}
	resonance := .35
	occurred := metadata.OccurredAtUTC.UTC()
	if occurred.Month() == now.Month() && occurred.Day() == now.Day() {
		resonance = .9
	}
	if strings.Contains(query, "去年") && occurred.Year() == now.Year()-1 {
		resonance = 1
	}
	if metadata.LocalDate != "" && strings.Contains(query, metadata.LocalDate) {
		resonance = 1
	}
	return resonance
}

func hasTemporalIntent(query string) bool {
	for _, word := range []string{"今天", "昨天", "明天", "去年", "前年", "日期", "时间", "当时", "过去", "纪念", "生日", "周年", "节日", "哪天", "什么时候"} {
		if strings.Contains(query, word) {
			return true
		}
	}
	return datePattern.MatchString(query)
}

var datePattern = regexp.MustCompile(`\b\d{4}[-/.年]\d{1,2}([-/\.月]\d{1,2}日?)?\b`)

func parseMemoryReferenceTime(value string) (time.Time, string) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, "created_at"
		}
	}
	return time.Time{}, "unknown"
}

func timeDecayFactor(age time.Duration, memoryType string) float64 {
	days := age.Hours() / 24
	halfLife := 180.0
	switch memoryType {
	case "episodic":
		halfLife = 30
	case "profile", "user_profile":
		halfLife = 90
	case "worldbook", "world_book":
		halfLife = 365
	}
	return math.Pow(2, -days/halfLife)
}

func roundScore(value float64) float64 { return math.Round(value*10000) / 10000 }

type MemoryTemporalMetadata struct {
	MemoryID          string     `json:"memoryId"`
	OccurredAtUTC     *time.Time `json:"occurredAtUtc,omitempty"`
	EndedAtUTC        *time.Time `json:"endedAtUtc,omitempty"`
	Timezone          string     `json:"timezone,omitempty"`
	LocalDate         string     `json:"localDate,omitempty"`
	Daypart           string     `json:"daypart,omitempty"`
	TemporalPrecision string     `json:"temporalPrecision"`
	ValidFromUTC      *time.Time `json:"validFromUtc,omitempty"`
	ValidToUTC        *time.Time `json:"validToUtc,omitempty"`
	AnchorIDsJSON     string     `json:"anchorIdsJson,omitempty"`
	SourceTimeText    string     `json:"sourceTimeText,omitempty"`
	CreatedAtUTC      time.Time  `json:"createdAtUtc"`
	UpdatedAtUTC      time.Time  `json:"updatedAtUtc"`
}
