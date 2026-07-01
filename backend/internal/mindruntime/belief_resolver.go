package mindruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

type AppliedBeliefAdjustment struct {
	ID            string
	BeliefKey     string
	OldStrength   float64
	NewStrength   float64
	AppliedAt     time.Time
	SourceRefID   string
	CandidateID   string
	CharacterID   string
}

type ResolvedRelationNarrative struct {
	ID            string
	RelationID    string
	OldNarrative  string
	NewNarrative  string
	ResolvedAt    time.Time
	EvidenceCount int
	CharacterID   string
	SourceRefID   string
}

type BeliefResolverConfig struct {
	MinConfidence   float64
	MaxAdjustPerRun int
	StrengthSmooth  float64
	RequireApproval bool
}

func DefaultBeliefResolverConfig() BeliefResolverConfig {
	return BeliefResolverConfig{
		MinConfidence:   0.5,
		MaxAdjustPerRun: 5,
		StrengthSmooth:  0.3,
		RequireApproval: true,
	}
}

func ResolveBeliefs(candidate ReflectionCandidate, config BeliefResolverConfig) []AppliedBeliefAdjustment {
	if candidate.Confidence < config.MinConfidence {
		return nil
	}
	adjustments := candidate.BeliefAdjustments
	if len(adjustments) == 0 {
		return nil
	}
	limit := config.MaxAdjustPerRun
	if limit > len(adjustments) {
		limit = len(adjustments)
	}
	sorted := make([]BeliefAdjustment, len(adjustments))
	copy(sorted, adjustments)
	sort.Slice(sorted, func(i, j int) bool {
		return (sorted[i].NewStrength - sorted[i].OldStrength) > (sorted[j].NewStrength - sorted[j].OldStrength)
	})
	applied := make([]AppliedBeliefAdjustment, 0, limit)
	for i := 0; i < limit; i++ {
		adj := sorted[i]
		smoothed := adj.OldStrength + (adj.NewStrength-adj.OldStrength)*config.StrengthSmooth
		a := AppliedBeliefAdjustment{
			ID:          appliedBeliefID(adj.BeliefKey, candidate.CharacterID, candidate.ID),
			BeliefKey:   adj.BeliefKey,
			OldStrength: adj.OldStrength,
			NewStrength: smoothed,
			AppliedAt:   time.Now().UTC(),
			SourceRefID: candidate.ID,
			CandidateID: candidate.ID,
			CharacterID: strings.TrimSpace(candidate.CharacterID),
		}
		applied = append(applied, a)
	}
	return applied
}

func ResolveRelationNarratives(updates []RelationNarrative, characterID string) []ResolvedRelationNarrative {
	if len(updates) == 0 {
		return nil
	}
	resolved := make([]ResolvedRelationNarrative, 0, len(updates))
	now := time.Now().UTC()
	for _, u := range updates {
		r := ResolvedRelationNarrative{
			ID:            resolvedRelationID(u.RelationID, characterID, now),
			RelationID:    u.RelationID,
			OldNarrative:  u.OldNarrative,
			NewNarrative:  u.NewNarrative,
			ResolvedAt:    now,
			EvidenceCount: u.EvidenceCount,
			CharacterID:   strings.TrimSpace(characterID),
			SourceRefID:   u.RelationID,
		}
		resolved = append(resolved, r)
	}
	return resolved
}

func RevertBeliefAdjustment(applied AppliedBeliefAdjustment, reason string) AppliedBeliefAdjustment {
	return AppliedBeliefAdjustment{
		ID:          "revert-" + applied.ID,
		BeliefKey:   applied.BeliefKey,
		OldStrength: applied.NewStrength,
		NewStrength: applied.OldStrength,
		AppliedAt:   time.Now().UTC(),
		SourceRefID: applied.SourceRefID,
		CandidateID: applied.CandidateID,
		CharacterID: applied.CharacterID,
	}
}

func IsBeliefAdjustmentApplied(applied AppliedBeliefAdjustment) bool {
	return applied.ID != "" && !applied.AppliedAt.IsZero()
}

func MergeAppliedAdjustments(adjustments []AppliedBeliefAdjustment) []AppliedBeliefAdjustment {
	if len(adjustments) <= 1 {
		return adjustments
	}
	byKey := make(map[string][]AppliedBeliefAdjustment)
	for _, a := range adjustments {
		byKey[a.BeliefKey] = append(byKey[a.BeliefKey], a)
	}
	merged := make([]AppliedBeliefAdjustment, 0)
	now := time.Now().UTC()
	for key, group := range byKey {
		if len(group) == 1 {
			merged = append(merged, group[0])
			continue
		}
		latest := group[0]
		for _, a := range group[1:] {
			if a.AppliedAt.After(latest.AppliedAt) {
				latest = a
			}
		}
		merged = append(merged, AppliedBeliefAdjustment{
			ID:          appliedBeliefID(key, latest.CharacterID, "merged"),
			BeliefKey:   key,
			OldStrength: group[0].OldStrength,
			NewStrength: latest.NewStrength,
			AppliedAt:   now,
			CharacterID: latest.CharacterID,
		})
	}
	return merged
}

func appliedBeliefID(beliefKey, characterID, sourceID string) string {
	raw := fmt.Sprintf("applied-belief|%s|%s|%s|%d", beliefKey, characterID, sourceID, time.Now().UnixNano())
	sum := sha256.Sum256([]byte(raw))
	return "app-belief-" + hex.EncodeToString(sum[:])[:16]
}

func resolvedRelationID(relationID, characterID string, now time.Time) string {
	raw := fmt.Sprintf("resolved-relation|%s|%s|%d", relationID, characterID, now.UnixNano())
	sum := sha256.Sum256([]byte(raw))
	return "res-rel-" + hex.EncodeToString(sum[:])[:16]
}
