package mindruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

type VerifiedEvent struct {
	ID         string
	Kind       string
	Summary    string
	Timestamp  time.Time
	Tags       []string
	Importance float64
}

type VerifiedRelation struct {
	ID          string
	CharacterID string
	Kind        string
	Strength    float64
	LastUpdated time.Time
}

type VerifiedMemory struct {
	ID         string
	Topic      string
	Content    string
	Importance float64
	CreatedAt  time.Time
}

type ReflectionEvidence struct {
	Events    []VerifiedEvent
	Relations []VerifiedRelation
	Memories  []VerifiedMemory
}

type ReflectionCandidate struct {
	ID                 string
	CharacterID        string
	BeliefAdjustments  []BeliefAdjustment
	RelationUpdates    []RelationNarrative
	MemoryAbstractions []MemoryAbstraction
	EvidenceRefs       []string
	Confidence         float64
	CreatedAt          time.Time
}

type BeliefAdjustment struct {
	BeliefKey   string
	OldStrength float64
	NewStrength float64
	Reason      string
}

type RelationNarrative struct {
	RelationID    string
	OldNarrative  string
	NewNarrative  string
	EvidenceCount int
}

type MemoryAbstraction struct {
	SourceIDs []string
	Topic     string
	Abstract  string
}

type ReflectionRunConfig struct {
	MinEvidenceForAdjustment int
	MinConfidenceForAdopt    float64
	MaxAbstractionsPerRun    int
}

func DefaultReflectionRunConfig() ReflectionRunConfig {
	return ReflectionRunConfig{
		MinEvidenceForAdjustment: 2,
		MinConfidenceForAdopt:    0.5,
		MaxAbstractionsPerRun:    5,
	}
}

func RunReflection(characterID string, evidence ReflectionEvidence, config ReflectionRunConfig, now time.Time) ReflectionCandidate {
	candidateRefs := make([]string, 0, len(evidence.Events)+len(evidence.Relations)+len(evidence.Memories))
	for _, e := range evidence.Events {
		candidateRefs = append(candidateRefs, "event:"+e.ID)
	}
	for _, r := range evidence.Relations {
		candidateRefs = append(candidateRefs, "relation:"+r.ID)
	}
	for _, m := range evidence.Memories {
		candidateRefs = append(candidateRefs, "memory:"+m.ID)
	}
	sort.Strings(candidateRefs)
	candidate := ReflectionCandidate{
		ID:          newReflectionCandidateID(characterID, candidateRefs, config),
		CharacterID: strings.TrimSpace(characterID),
		CreatedAt:   now.UTC(),
	}
	candidate.EvidenceRefs = candidateRefs
	candidate.BeliefAdjustments = deriveBeliefAdjustments(evidence, config)
	candidate.RelationUpdates = deriveRelationUpdates(evidence, config)
	candidate.MemoryAbstractions = deriveMemoryAbstractions(evidence, config)
	totalOutputs := len(candidate.BeliefAdjustments) + len(candidate.RelationUpdates) + len(candidate.MemoryAbstractions)
	if totalOutputs > 0 {
		candidate.Confidence = computeReflectionConfidence(evidence, totalOutputs)
	}
	return candidate
}

func deriveBeliefAdjustments(evidence ReflectionEvidence, config ReflectionRunConfig) []BeliefAdjustment {
	adjustments := make([]BeliefAdjustment, 0)
	if len(evidence.Events) < config.MinEvidenceForAdjustment {
		return adjustments
	}
	importantEvents := filterImportantEvents(evidence.Events)
	if len(importantEvents) == 0 {
		return adjustments
	}
	sort.Slice(importantEvents, func(i, j int) bool {
		return importantEvents[i].Importance > importantEvents[j].Importance
	})
	for i := 0; i < len(importantEvents) && i < config.MinEvidenceForAdjustment; i++ {
		evt := importantEvents[i]
		key := deriveBeliefKey(evt)
		if key == "" {
			continue
		}
		adj := BeliefAdjustment{
			BeliefKey:   key,
			OldStrength: 0,
			NewStrength: clampStrength(evt.Importance),
			Reason:      fmt.Sprintf("事件 %s", evt.ID),
		}
		adjustments = append(adjustments, adj)
	}
	return adjustments
}

func deriveRelationUpdates(evidence ReflectionEvidence, config ReflectionRunConfig) []RelationNarrative {
	updates := make([]RelationNarrative, 0)
	for _, rel := range evidence.Relations {
		relatedEvents := filterEventsByRelation(evidence.Events, rel.CharacterID)
		if len(relatedEvents) < config.MinEvidenceForAdjustment {
			continue
		}
		update := RelationNarrative{
			RelationID:    rel.ID,
			EvidenceCount: len(relatedEvents),
		}
		updates = append(updates, update)
	}
	return updates
}

func deriveMemoryAbstractions(evidence ReflectionEvidence, config ReflectionRunConfig) []MemoryAbstraction {
	abstractions := make([]MemoryAbstraction, 0)
	byTopic := make(map[string][]VerifiedMemory)
	for _, m := range evidence.Memories {
		topic := strings.ToLower(strings.TrimSpace(m.Topic))
		if topic == "" {
			continue
		}
		byTopic[topic] = append(byTopic[topic], m)
	}
	topics := make([]string, 0, len(byTopic))
	for topic := range byTopic {
		topics = append(topics, topic)
	}
	sort.Strings(topics)
	for _, topic := range topics {
		mems := byTopic[topic]
		if len(mems) < config.MinEvidenceForAdjustment {
			continue
		}
		if len(abstractions) >= config.MaxAbstractionsPerRun {
			break
		}
		sort.Slice(mems, func(i, j int) bool {
			return mems[i].ID < mems[j].ID
		})
		sourceIDs := make([]string, 0, len(mems))
		for _, m := range mems {
			sourceIDs = append(sourceIDs, m.ID)
		}
		abstraction := MemoryAbstraction{
			SourceIDs: sourceIDs,
			Topic:     topic,
			Abstract:  fmt.Sprintf("%d 条相关记忆的抽象摘要", len(mems)),
		}
		abstractions = append(abstractions, abstraction)
	}
	return abstractions
}

func IsReflectionCandidateSignificant(candidate ReflectionCandidate, config ReflectionRunConfig) bool {
	if len(candidate.BeliefAdjustments) > 0 {
		return true
	}
	if len(candidate.RelationUpdates) > 0 {
		return true
	}
	if len(candidate.MemoryAbstractions) > 0 {
		return true
	}
	return false
}

func MergeReflectionCandidates(candidates []ReflectionCandidate) ReflectionCandidate {
	merged := ReflectionCandidate{
		BeliefAdjustments:  make([]BeliefAdjustment, 0),
		RelationUpdates:    make([]RelationNarrative, 0),
		MemoryAbstractions: make([]MemoryAbstraction, 0),
		EvidenceRefs:       make([]string, 0),
	}
	totalConf := 0.0
	for _, c := range candidates {
		merged.BeliefAdjustments = append(merged.BeliefAdjustments, c.BeliefAdjustments...)
		merged.RelationUpdates = append(merged.RelationUpdates, c.RelationUpdates...)
		merged.MemoryAbstractions = append(merged.MemoryAbstractions, c.MemoryAbstractions...)
		merged.EvidenceRefs = append(merged.EvidenceRefs, c.EvidenceRefs...)
		totalConf += c.Confidence
	}
	if len(candidates) > 0 {
		merged.Confidence = totalConf / float64(len(candidates))
		merged.CharacterID = candidates[0].CharacterID
		merged.ID = candidates[0].ID
		merged.CreatedAt = candidates[0].CreatedAt
	}
	seen := make(map[string]bool)
	unique := make([]string, 0, len(merged.EvidenceRefs))
	for _, ref := range merged.EvidenceRefs {
		if !seen[ref] {
			seen[ref] = true
			unique = append(unique, ref)
		}
	}
	merged.EvidenceRefs = unique
	return merged
}

func newReflectionCandidateID(characterID string, sortedRefs []string, config ReflectionRunConfig) string {
	raw := strings.Builder{}
	raw.WriteString("reflection|character:")
	raw.WriteString(strings.TrimSpace(characterID))
	raw.WriteString("|minEvidence:")
	raw.WriteString(fmt.Sprintf("%d", config.MinEvidenceForAdjustment))
	raw.WriteString("|minConfidence:")
	raw.WriteString(fmt.Sprintf("%.4f", config.MinConfidenceForAdopt))
	raw.WriteString("|maxAbstractions:")
	raw.WriteString(fmt.Sprintf("%d", config.MaxAbstractionsPerRun))
	raw.WriteString("|refs:")
	raw.WriteString(strings.Join(sortedRefs, ","))
	sum := sha256.Sum256([]byte(raw.String()))
	return "ref-" + hex.EncodeToString(sum[:])[:16]
}

func filterImportantEvents(events []VerifiedEvent) []VerifiedEvent {
	filtered := make([]VerifiedEvent, 0, len(events))
	for _, e := range events {
		if e.Importance > 0 {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func deriveBeliefKey(event VerifiedEvent) string {
	kind := strings.TrimSpace(event.Kind)
	if kind == "" {
		return ""
	}
	return "belief/" + strings.ToLower(kind)
}

func clampStrength(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func filterEventsByRelation(events []VerifiedEvent, characterID string) []VerifiedEvent {
	filtered := make([]VerifiedEvent, 0)
	target := strings.TrimSpace(characterID)
	for _, e := range events {
		for _, tag := range e.Tags {
			if strings.TrimSpace(tag) == target {
				filtered = append(filtered, e)
				break
			}
		}
	}
	return filtered
}

func computeReflectionConfidence(evidence ReflectionEvidence, outputCount int) float64 {
	evidenceCount := len(evidence.Events) + len(evidence.Relations) + len(evidence.Memories)
	if evidenceCount == 0 {
		return 0
	}
	ratio := float64(outputCount) / float64(evidenceCount)
	if ratio > 1 {
		ratio = 1
	}
	return ratio * 0.8
}
