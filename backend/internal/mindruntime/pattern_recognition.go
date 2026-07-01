package mindruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

type PatternEvent struct {
	ID         string
	Kind       string
	Summary    string
	Timestamp  time.Time
	Tags       []string
	Importance float64
}

type PatternCandidate struct {
	ID          string
	Kind        string
	Description string
	EventIDs    []string
	Count       int
	FirstSeen   time.Time
	LastSeen    time.Time
	Confidence  float64
}

type PatternRecognitionConfig struct {
	MinIndependentEvents int
	TimeWindow           time.Duration
	MinImportance        float64
	MaxPatternsPerRun    int
}

func DefaultPatternRecognitionConfig() PatternRecognitionConfig {
	return PatternRecognitionConfig{
		MinIndependentEvents: 3,
		TimeWindow:           7 * 24 * time.Hour,
		MinImportance:        0.1,
		MaxPatternsPerRun:    10,
	}
}

func RecognizePatterns(events []PatternEvent, config PatternRecognitionConfig) []PatternCandidate {
	if len(events) < config.MinIndependentEvents {
		return nil
	}
	byKind := make(map[string][]PatternEvent)
	for _, e := range events {
		kind := strings.ToLower(strings.TrimSpace(e.Kind))
		if kind == "" {
			continue
		}
		if e.Importance < config.MinImportance {
			continue
		}
		byKind[kind] = append(byKind[kind], e)
	}
	byTagGroup := make(map[string][]PatternEvent)
	for _, e := range events {
		if len(e.Tags) == 0 {
			continue
		}
		if e.Importance < config.MinImportance {
			continue
		}
		sort.Strings(e.Tags)
		tagKey := strings.Join(e.Tags, "|")
		byTagGroup[tagKey] = append(byTagGroup[tagKey], e)
	}
	candidates := make([]PatternCandidate, 0)
	for kind, kindEvents := range byKind {
		if len(kindEvents) < config.MinIndependentEvents {
			continue
		}
		filtered := filterByTimeWindow(kindEvents, config.TimeWindow)
		if len(filtered) < config.MinIndependentEvents {
			continue
		}
		candidate := buildPatternCandidate(kind, filtered, "kind")
		candidates = append(candidates, candidate)
	}
	for tagKey, tagEvents := range byTagGroup {
		if len(tagEvents) < config.MinIndependentEvents {
			continue
		}
		filtered := filterByTimeWindow(tagEvents, config.TimeWindow)
		if len(filtered) < config.MinIndependentEvents {
			continue
		}
		desc := "标签组: " + strings.ReplaceAll(tagKey, "|", ", ")
		candidate := buildPatternCandidate(desc, filtered, "tag_group")
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Confidence > candidates[j].Confidence
	})
	if len(candidates) > config.MaxPatternsPerRun {
		candidates = candidates[:config.MaxPatternsPerRun]
	}
	return candidates
}

func filterByTimeWindow(events []PatternEvent, window time.Duration) []PatternEvent {
	if window <= 0 {
		return events
	}
	if len(events) == 0 {
		return nil
	}
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})
	result := make([]PatternEvent, 0, len(events))
	for i := range events {
		group := make([]PatternEvent, 0)
		startTime := events[i].Timestamp
		for j := i; j < len(events); j++ {
			if events[j].Timestamp.Sub(startTime) <= window {
				group = append(group, events[j])
			}
		}
		if len(group) > len(result) {
			result = group
		}
	}
	return result
}

func buildPatternCandidate(description string, events []PatternEvent, sourceType string) PatternCandidate {
	eventIDs := make([]string, 0, len(events))
	for _, e := range events {
		eventIDs = append(eventIDs, e.ID)
	}
	firstSeen := events[0].Timestamp
	lastSeen := events[len(events)-1].Timestamp
	for _, e := range events {
		if e.Timestamp.Before(firstSeen) {
			firstSeen = e.Timestamp
		}
		if e.Timestamp.After(lastSeen) {
			lastSeen = e.Timestamp
		}
	}
	avgImportance := 0.0
	for _, e := range events {
		avgImportance += e.Importance
	}
	avgImportance = avgImportance / float64(len(events))
	span := lastSeen.Sub(firstSeen)
	timeNorm := 0.5
	if span > 0 {
		timeNorm = 1.0 / (1.0 + span.Hours()/24.0)
	}
	confidence := avgImportance*0.5 + timeNorm*0.3 + float64(len(events))/float64(len(events)+3)*0.2
	raw := fmt.Sprintf("pattern|%s|%s|%d", sourceType, description, len(events))
	sum := sha256.Sum256([]byte(raw))
	id := "pattern-" + hex.EncodeToString(sum[:])[:16]
	return PatternCandidate{
		ID:          id,
		Kind:        sourceType,
		Description: description,
		EventIDs:    eventIDs,
		Count:       len(events),
		FirstSeen:   firstSeen,
		LastSeen:    lastSeen,
		Confidence:  confidence,
	}
}

func IsPatternDistinct(a, b PatternCandidate) bool {
	if a.Kind != b.Kind {
		return true
	}
	shared := 0
	aSet := make(map[string]bool)
	for _, id := range a.EventIDs {
		aSet[id] = true
	}
	for _, id := range b.EventIDs {
		if aSet[id] {
			shared++
		}
	}
	overlap := float64(shared) / float64(minInt(len(a.EventIDs), len(b.EventIDs)))
	return overlap < 0.5
}

func MergePatternCandidates(candidates []PatternCandidate) []PatternCandidate {
	if len(candidates) <= 1 {
		return candidates
	}
	merged := make([]PatternCandidate, 0)
	used := make([]bool, len(candidates))
	for i := range candidates {
		if used[i] {
			continue
		}
		current := candidates[i]
		used[i] = true
		for j := i + 1; j < len(candidates); j++ {
			if used[j] {
				continue
			}
			if !IsPatternDistinct(current, candidates[j]) {
				allIDs := make(map[string]bool)
				for _, id := range current.EventIDs {
					allIDs[id] = true
				}
				for _, id := range candidates[j].EventIDs {
					allIDs[id] = true
				}
				uniqueIDs := make([]string, 0, len(allIDs))
				for id := range allIDs {
					uniqueIDs = append(uniqueIDs, id)
				}
				current.Count = len(uniqueIDs)
				current.EventIDs = uniqueIDs
				if candidates[j].Confidence > current.Confidence {
					current.Confidence = candidates[j].Confidence
				}
				if candidates[j].FirstSeen.Before(current.FirstSeen) {
					current.FirstSeen = candidates[j].FirstSeen
				}
				if candidates[j].LastSeen.After(current.LastSeen) {
					current.LastSeen = candidates[j].LastSeen
				}
				used[j] = true
			}
		}
		merged = append(merged, current)
	}
	return merged
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
