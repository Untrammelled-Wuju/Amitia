package mindruntime

import (
	"strings"
	"time"
)

type MemorySummaryRecord struct {
	ID              string    `json:"id"`
	Topic           string    `json:"topic"`
	CharacterID     string    `json:"characterId"`
	TotalEntries    int       `json:"totalEntries"`
	MemoryIDs       []string  `json:"memoryIds,omitempty"`
	SummarizedKeys  []string  `json:"summarizedKeys,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	ExpiresAt       time.Time `json:"expiresAt,omitempty"`
	SourceRequestID string    `json:"sourceRequestId,omitempty"`
}

type SummarizerConfig struct {
	DefaultRetention time.Duration
}

func DefaultSummarizerConfig() SummarizerConfig {
	return SummarizerConfig{
		DefaultRetention: 24 * time.Hour,
	}
}

func BuildMemorySummaryRecord(topic, characterID, sourceRequestID string, totalEntries int, memoryIDs, summarizedKeys []string, retention time.Duration) MemorySummaryRecord {
	now := time.Now().UTC()
	record := MemorySummaryRecord{
		ID:              summaryRecordID(topic, characterID, now),
		Topic:           strings.TrimSpace(topic),
		CharacterID:     strings.TrimSpace(characterID),
		TotalEntries:    totalEntries,
		MemoryIDs:       normalizeStringSlice(memoryIDs),
		SummarizedKeys:  normalizeStringSlice(summarizedKeys),
		CreatedAt:       now,
		SourceRequestID: strings.TrimSpace(sourceRequestID),
	}
	if retention > 0 {
		record.ExpiresAt = now.Add(retention)
	}
	return record
}

func (r MemorySummaryRecord) Expired(now time.Time) bool {
	if r.ExpiresAt.IsZero() {
		return false
	}
	return !now.UTC().Before(r.ExpiresAt)
}

func (r MemorySummaryRecord) CoversTopic(topic string) bool {
	return strings.EqualFold(strings.TrimSpace(topic), r.Topic)
}

func (r MemorySummaryRecord) CoversCharacter(characterID string) bool {
	return strings.EqualFold(strings.TrimSpace(characterID), r.CharacterID)
}

func summaryRecordID(topic, characterID string, now time.Time) string {
	clean1 := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(topic)), " ", "_")
	clean2 := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(characterID)), " ", "_")
	ts := now.Format("20060102T150405")
	return "mem-summary-" + clean1 + "-" + clean2 + "-" + ts
}

func normalizeStringSlice(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	result := make([]string, 0, len(items))
	for _, item := range items {
		v := strings.TrimSpace(item)
		if v != "" && !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
