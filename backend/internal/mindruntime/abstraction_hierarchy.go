package mindruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

type AbstractionLevel string

const (
	AbstractionSpecific  AbstractionLevel = "specific"
	AbstractionGeneral   AbstractionLevel = "general"
	AbstractionHighLevel AbstractionLevel = "high_level"
)

type HierarchicalAbstraction struct {
	ID          string
	Topic       string
	CharacterID string
	Level       AbstractionLevel
	Abstract    string
	SourceIDs   []string
	ParentID    string
	ChildrenIDs []string
	SourceCount int
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

type AbstractionHierarchyConfig struct {
	MinSourcesForGeneral   int
	MinSourcesForHighLevel int
	DefaultRetentionDays   int
	MaxAbstractionsPerRun  int
}

func DefaultAbstractionHierarchyConfig() AbstractionHierarchyConfig {
	return AbstractionHierarchyConfig{
		MinSourcesForGeneral:   3,
		MinSourcesForHighLevel: 9,
		DefaultRetentionDays:   90,
		MaxAbstractionsPerRun:  20,
	}
}

func BuildAbstractionHierarchy(memories []VerifiedMemory, characterID string, config AbstractionHierarchyConfig) []HierarchicalAbstraction {
	if len(memories) == 0 {
		return nil
	}
	byTopic := make(map[string][]VerifiedMemory)
	for _, m := range memories {
		topic := strings.ToLower(strings.TrimSpace(m.Topic))
		if topic == "" {
			continue
		}
		byTopic[topic] = append(byTopic[topic], m)
	}
	abstractions := make([]HierarchicalAbstraction, 0)
	now := time.Now().UTC()
	retention := time.Duration(config.DefaultRetentionDays) * 24 * time.Hour
	for topic, mems := range byTopic {
		if len(mems) < 2 {
			continue
		}
		levels := buildLevelsForTopic(topic, mems, characterID, config, now, retention)
		abstractions = append(abstractions, levels...)
	}
	byParentTopic := make(map[string][]VerifiedMemory)
	var parentTopics []string
	type parentWeight struct {
		topic string
		count int
	}
	weights := make([]parentWeight, 0)
	for topic, mems := range byTopic {
		weights = append(weights, parentWeight{topic: topic, count: len(mems)})
	}
	sort.Slice(weights, func(i, j int) bool {
		return weights[i].count > weights[j].count
	})
	for _, w := range weights {
		if len(parentTopics) >= 5 {
			break
		}
		parentTopics = append(parentTopics, w.topic)
		for _, m := range byTopic[w.topic] {
			byParentTopic[w.topic] = append(byParentTopic[w.topic], m)
		}
	}
	_ = byParentTopic
	sort.Slice(abstractions, func(i, j int) bool {
		return abstractions[i].SourceCount > abstractions[j].SourceCount
	})
	if len(abstractions) > config.MaxAbstractionsPerRun {
		abstractions = abstractions[:config.MaxAbstractionsPerRun]
	}
	parentMap := make(map[AbstractionLevel]map[string]string)
	parentMap[AbstractionSpecific] = make(map[string]string)
	for i := range abstractions {
		if abstractions[i].Level == AbstractionSpecific {
			parentMap[AbstractionSpecific][abstractions[i].ID] = abstractions[i].Topic
		}
	}
	for i := range abstractions {
		if abstractions[i].Level == AbstractionGeneral {
			for _, srcID := range abstractions[i].SourceIDs {
				if parentID, ok := parentMap[AbstractionSpecific][srcID]; ok {
					if parentID != "" {
						abstractions[i].ChildrenIDs = append(abstractions[i].ChildrenIDs, srcID)
					}
				}
			}
		}
	}
	return abstractions
}

func buildLevelsForTopic(topic string, mems []VerifiedMemory, characterID string, config AbstractionHierarchyConfig, now time.Time, retention time.Duration) []HierarchicalAbstraction {
	result := make([]HierarchicalAbstraction, 0)
	specific := buildSpecificAbstraction(topic, mems, characterID, now, retention)
	result = append(result, specific)
	if len(mems) >= config.MinSourcesForGeneral {
		general := buildGeneralAbstraction(topic, specific.ID, mems, characterID, now, retention)
		result = append(result, general)
	}
	if len(mems) >= config.MinSourcesForHighLevel {
		highLevel := buildHighLevelAbstraction(topic, mems, characterID, now, retention)
		result = append(result, highLevel)
	}
	return result
}

func buildSpecificAbstraction(topic string, mems []VerifiedMemory, characterID string, now time.Time, retention time.Duration) HierarchicalAbstraction {
	sourceIDs := make([]string, 0, len(mems))
	for _, m := range mems {
		sourceIDs = append(sourceIDs, m.ID)
	}
	abstract := fmt.Sprintf("关于 %s 的 %d 条具体记忆", topic, len(mems))
	id := abstractionID(topic, characterID, AbstractionSpecific, now)
	return HierarchicalAbstraction{
		ID:          id,
		Topic:       topic,
		CharacterID: characterID,
		Level:       AbstractionSpecific,
		Abstract:    abstract,
		SourceIDs:   sourceIDs,
		SourceCount: len(mems),
		CreatedAt:   now,
		ExpiresAt:   now.Add(retention),
	}
}

func buildGeneralAbstraction(topic, parentID string, mems []VerifiedMemory, characterID string, now time.Time, retention time.Duration) HierarchicalAbstraction {
	avgImportance := 0.0
	for _, m := range mems {
		avgImportance += m.Importance
	}
	avgImportance = avgImportance / float64(len(mems))
	keywords := extractKeywords(mems)
	keywordsStr := strings.Join(keywords, "、")
	if keywordsStr == "" {
		keywordsStr = topic
	}
	abstract := fmt.Sprintf("关键词: %s; 平均重要性: %.2f", keywordsStr, avgImportance)
	sourceIDs := make([]string, len(mems))
	for i, m := range mems {
		sourceIDs[i] = m.ID
	}
	id := abstractionID(topic, characterID, AbstractionGeneral, now)
	return HierarchicalAbstraction{
		ID:          id,
		Topic:       topic,
		CharacterID: characterID,
		Level:       AbstractionGeneral,
		Abstract:    abstract,
		SourceIDs:   sourceIDs,
		ParentID:    parentID,
		SourceCount: len(mems),
		CreatedAt:   now,
		ExpiresAt:   now.Add(retention),
	}
}

func buildHighLevelAbstraction(topic string, mems []VerifiedMemory, characterID string, now time.Time, retention time.Duration) HierarchicalAbstraction {
	abstract := fmt.Sprintf("用户长期对 %s 领域有 %d 条相关交互", topic, len(mems))
	id := abstractionID(topic, characterID, AbstractionHighLevel, now)
	sourceIDs := make([]string, len(mems))
	for i, m := range mems {
		sourceIDs[i] = m.ID
	}
	return HierarchicalAbstraction{
		ID:          id,
		Topic:       topic,
		CharacterID: characterID,
		Level:       AbstractionHighLevel,
		Abstract:    abstract,
		SourceIDs:   sourceIDs,
		SourceCount: len(mems),
		CreatedAt:   now,
		ExpiresAt:   now.Add(retention),
	}
}

func extractKeywords(mems []VerifiedMemory) []string {
	wordFreq := make(map[string]int)
	for _, m := range mems {
		parts := strings.FieldsFunc(strings.ToLower(m.Content), func(r rune) bool {
			return r == ' ' || r == '，' || r == ',' || r == '。' || r == '.' || r == ':'
		})
		for _, p := range parts {
			v := strings.TrimSpace(p)
			if len([]rune(v)) >= 2 {
				wordFreq[v]++
			}
		}
	}
	type wordCount struct {
		word  string
		count int
	}
	sorted := make([]wordCount, 0, len(wordFreq))
	for w, c := range wordFreq {
		sorted = append(sorted, wordCount{word: w, count: c})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})
	limit := 5
	if limit > len(sorted) {
		limit = len(sorted)
	}
	result := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		result = append(result, sorted[i].word)
	}
	return result
}

func GetAbstractionLevels(abstractions []HierarchicalAbstraction) []HierarchicalAbstraction {
	if len(abstractions) == 0 {
		return nil
	}
	sort.Slice(abstractions, func(i, j int) bool {
		order := map[AbstractionLevel]int{
			AbstractionSpecific:  0,
			AbstractionGeneral:   1,
			AbstractionHighLevel: 2,
		}
		if abstractions[i].Topic != abstractions[j].Topic {
			return abstractions[i].Topic < abstractions[j].Topic
		}
		return order[abstractions[i].Level] < order[abstractions[j].Level]
	})
	result := make([]HierarchicalAbstraction, len(abstractions))
	copy(result, abstractions)
	return result
}

func IsAbstractionExpired(abstraction HierarchicalAbstraction, now time.Time) bool {
	if abstraction.ExpiresAt.IsZero() {
		return false
	}
	return !now.UTC().Before(abstraction.ExpiresAt)
}

func FilterActiveAbstractions(abstractions []HierarchicalAbstraction, now time.Time) []HierarchicalAbstraction {
	result := make([]HierarchicalAbstraction, 0)
	for _, a := range abstractions {
		if !IsAbstractionExpired(a, now) {
			result = append(result, a)
		}
	}
	return result
}

func abstractionID(topic, characterID string, level AbstractionLevel, now time.Time) string {
	raw := fmt.Sprintf("abstraction|%s|%s|%s|%d", topic, characterID, string(level), now.UnixNano())
	sum := sha256.Sum256([]byte(raw))
	return "abs-" + hex.EncodeToString(sum[:])[:16]
}
