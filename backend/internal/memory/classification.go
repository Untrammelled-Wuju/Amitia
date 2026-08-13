// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import "strings"

type MemoryLayer string

const (
	MemoryLayerFact      MemoryLayer = "fact"
	MemoryLayerProfile   MemoryLayer = "profile"
	MemoryLayerEpisodic  MemoryLayer = "episodic"
	MemoryLayerWorking   MemoryLayer = "working"
	MemoryLayerWorldbook MemoryLayer = "worldbook"
	MemoryLayerGraph     MemoryLayer = "graph"
)

type MemoryType string

const (
	MemoryTypePersonalInfo MemoryType = "personal_info"
	MemoryTypeHobby        MemoryType = "hobby"
	MemoryTypePreference   MemoryType = "preference"
	MemoryTypeFact         MemoryType = "fact"
	MemoryTypePlan         MemoryType = "plan"
	MemoryTypeHabit        MemoryType = "habit"
	MemoryTypeRelationship MemoryType = "relationship"
	MemoryTypeCustom       MemoryType = "custom"
)

var validMemoryTypes = map[MemoryType]bool{
	MemoryTypePersonalInfo: true,
	MemoryTypeHobby:        true,
	MemoryTypePreference:   true,
	MemoryTypeFact:         true,
	MemoryTypePlan:         true,
	MemoryTypeHabit:        true,
	MemoryTypeRelationship: true,
	MemoryTypeCustom:       true,
}

var MemoryTypeAliases = map[string]MemoryType{
	"nickname":   MemoryTypePersonalInfo,
	"bio":        MemoryTypePersonalInfo,
	"personal":   MemoryTypePersonalInfo,
	"likes":      MemoryTypePreference,
	"preference": MemoryTypePreference,
	"routine":    MemoryTypeHabit,
	"event":      MemoryTypeFact,
	"goal":       MemoryTypePlan,
	"relation":   MemoryTypeRelationship,
	"other":      MemoryTypeCustom,
}

var validLayers = map[MemoryLayer]bool{
	MemoryLayerFact:      true,
	MemoryLayerProfile:   true,
	MemoryLayerEpisodic:  true,
	MemoryLayerWorking:   true,
	MemoryLayerWorldbook: true,
	MemoryLayerGraph:     true,
}

type MemoryTimeBasis string

const (
	TimeBasisOccurred MemoryTimeBasis = "occurred"
	TimeBasisValidity MemoryTimeBasis = "validity"
	TimeBasisCreated  MemoryTimeBasis = "created"
	TimeBasisUpdated  MemoryTimeBasis = "updated"
	TimeBasisLastUsed MemoryTimeBasis = "last_used"
)

var validTimeBasis = map[MemoryTimeBasis]bool{
	TimeBasisOccurred: true,
	TimeBasisValidity: true,
	TimeBasisCreated:  true,
	TimeBasisUpdated:  true,
	TimeBasisLastUsed: true,
}

type TemporalPrecision string

const (
	TemporalPrecisionExact      TemporalPrecision = "exact"
	TemporalPrecisionMinute     TemporalPrecision = "minute"
	TemporalPrecisionHour       TemporalPrecision = "hour"
	TemporalPrecisionDay        TemporalPrecision = "day"
	TemporalPrecisionWeek       TemporalPrecision = "week"
	TemporalPrecisionMonth      TemporalPrecision = "month"
	TemporalPrecisionYear       TemporalPrecision = "year"
	TemporalPrecisionApproximate TemporalPrecision = "approximate"
	TemporalPrecisionRange      TemporalPrecision = "range"
	TemporalPrecisionUnknown    TemporalPrecision = "unknown"
)

var validPrecisions = map[TemporalPrecision]bool{
	TemporalPrecisionExact:      true,
	TemporalPrecisionMinute:     true,
	TemporalPrecisionHour:       true,
	TemporalPrecisionDay:        true,
	TemporalPrecisionWeek:       true,
	TemporalPrecisionMonth:      true,
	TemporalPrecisionYear:       true,
	TemporalPrecisionApproximate: true,
	TemporalPrecisionRange:      true,
	TemporalPrecisionUnknown:    true,
}

type MemoryClassification struct {
	Layer          MemoryLayer `json:"layer"`
	Type           string      `json:"type"`
	CanonicalType  string      `json:"canonicalType,omitempty"`
}

func NormalizeMemoryType(raw string) (MemoryType, bool) {
	candidate := strings.ToLower(strings.TrimSpace(raw))
	if candidate == "" {
		return MemoryTypeFact, true
	}
	mt := MemoryType(candidate)
	if validMemoryTypes[mt] {
		return mt, true
	}
	if alias, ok := MemoryTypeAliases[candidate]; ok {
		return alias, true
	}
	return "", false
}

func ValidateMemoryType(raw string) (MemoryType, bool) {
	candidate := strings.ToLower(strings.TrimSpace(raw))
	if candidate == "" {
		return MemoryTypeFact, true
	}
	mt := MemoryType(candidate)
	if validMemoryTypes[mt] {
		return mt, true
	}
	return "", false
}

func CanonicalMemoryType(raw string) MemoryType {
	mt, _ := NormalizeMemoryType(raw)
	return mt
}

func IsValidLayer(raw string) bool {
	_, ok := validLayers[MemoryLayer(strings.ToLower(strings.TrimSpace(raw)))]
	return ok
}

func CanonicalLayer(tableKey string) MemoryLayer {
	switch tableKey {
	case "user_profiles":
		return MemoryLayerProfile
	case "episodic_memories":
		return MemoryLayerEpisodic
	case "working_memory":
		return MemoryLayerWorking
	case "world_book":
		return MemoryLayerWorldbook
	case "memory_graph", "graph":
		return MemoryLayerGraph
	default:
		return MemoryLayerFact
	}
}
