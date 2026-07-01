package mindruntime

import (
	"strings"
	"time"
)

type DayEvent struct {
	ID         string
	Kind       string
	Summary    string
	Importance float64
	IsThread   bool
	IsPlan     bool
	Timestamp  time.Time
}

type DayEvents struct {
	Date   string
	Events []DayEvent
}

type ConsolidationResult struct {
	CompressedEvents  []DayEvent
	PreservedEvents   []DayEvent
	MemoryAbstractions []MemoryAbstraction
	GeneratedSummaries []MemorySummaryRecord
	Date              string
	CharacterID       string
}

type ConsolidationConfig struct {
	ImportanceThreshold float64
	MaxCompressedEvents int
	Retention           time.Duration
}

func DefaultConsolidationConfig() ConsolidationConfig {
	return ConsolidationConfig{
		ImportanceThreshold: 0.5,
		MaxCompressedEvents: 100,
		Retention:           30 * 24 * time.Hour,
	}
}

func RunDayConsolidation(dayEvents DayEvents, characterID string, config ConsolidationConfig) ConsolidationResult {
	result := ConsolidationResult{
		Date:        strings.TrimSpace(dayEvents.Date),
		CharacterID: strings.TrimSpace(characterID),
	}

	ordinary, important := partitionDayEvents(dayEvents.Events, config.ImportanceThreshold)

	result.PreservedEvents = preservedEvents(important)
	result.CompressedEvents = compressOrdinaryEvents(ordinary, config.MaxCompressedEvents)
	result.MemoryAbstractions = buildDayAbstractions(dayEvents, config)
	result.GeneratedSummaries = buildDaySummaries(dayEvents, characterID, config)

	return result
}

func partitionDayEvents(events []DayEvent, threshold float64) (ordinary, important []DayEvent) {
	ordinary = make([]DayEvent, 0)
	important = make([]DayEvent, 0)
	for _, e := range events {
		if e.IsThread || e.IsPlan {
			important = append(important, e)
		} else if e.Importance >= threshold {
			important = append(important, e)
		} else {
			ordinary = append(ordinary, e)
		}
	}
	return
}

func preservedEvents(important []DayEvent) []DayEvent {
	if len(important) == 0 {
		return nil
	}
	result := make([]DayEvent, len(important))
	copy(result, important)
	return result
}

func compressOrdinaryEvents(ordinary []DayEvent, maxEvents int) []DayEvent {
	if len(ordinary) == 0 {
		return nil
	}
	if len(ordinary) <= maxEvents {
		result := make([]DayEvent, len(ordinary))
		copy(result, ordinary)
		return result
	}
	result := make([]DayEvent, maxEvents)
	copy(result, ordinary[:maxEvents])
	return result
}

func buildDayAbstractions(dayEvents DayEvents, config ConsolidationConfig) []MemoryAbstraction {
	abstractions := make([]MemoryAbstraction, 0)
	byKind := make(map[string][]DayEvent)
	for _, e := range dayEvents.Events {
		kind := strings.ToLower(strings.TrimSpace(e.Kind))
		if kind == "" {
			kind = "general"
		}
		byKind[kind] = append(byKind[kind], e)
	}
	for kind, events := range byKind {
		if len(events) < 3 {
			continue
		}
		sourceIDs := make([]string, 0, len(events))
		for _, e := range events {
			sourceIDs = append(sourceIDs, e.ID)
		}
		abstraction := MemoryAbstraction{
			SourceIDs: sourceIDs,
			Topic:     kind,
			Abstract:  buildAbstractSummary(dayEvents.Date, kind, len(events)),
		}
		abstractions = append(abstractions, abstraction)
	}
	return abstractions
}

func buildDaySummaries(dayEvents DayEvents, characterID string, config ConsolidationConfig) []MemorySummaryRecord {
	summaries := make([]MemorySummaryRecord, 0)
	byTopic := make(map[string][]string)
	for _, e := range dayEvents.Events {
		topic := strings.ToLower(strings.TrimSpace(e.Kind))
		if topic == "" {
			topic = "general"
		}
		byTopic[topic] = append(byTopic[topic], e.ID)
	}
	for topic, ids := range byTopic {
		keys := make([]string, 0)
		record := BuildMemorySummaryRecord(
			topic,
			characterID,
			"day-consolidation",
			len(ids),
			ids,
			keys,
			config.Retention,
		)
		summaries = append(summaries, record)
	}
	return summaries
}

func buildAbstractSummary(date, kind string, count int) string {
	return date + " 的 " + kind + " 事件 (" + "" + ")"
}
