package belief

import (
	"math"
	"sort"
	"time"
)

const formulaVersion = "belief-priority-formula-v1"

type aggregate struct {
	Value        string
	Confidence   float64
	Source       SourceKind
	ObservedAt   time.Time
	CandidateIDs []string
}

func DefaultPolicy() ResolverPolicy {
	return ResolverPolicy{
		MinimumConfidence: 0.45,
		ConflictGap:       0.08,
		MaxCandidates:     12,
	}
}

func ResolveBelief(input ResolveInput) ResolveResult {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	policy, diagnostics := normalizePolicy(input.Policy)
	filtered := make([]Candidate, 0, len(input.Candidates))
	rejected := []string{}
	for _, candidate := range input.Candidates {
		item := normalizeCandidate(candidate)
		if item.Key != "" && input.Key != "" && item.Key != input.Key {
			if item.ID != "" {
				rejected = append(rejected, item.ID)
			}
			diagnostics = append(diagnostics, "key_mismatch_skipped")
			continue
		}
		if !item.ExpiresAt.IsZero() && !item.ExpiresAt.After(now) {
			if item.ID != "" {
				rejected = append(rejected, item.ID)
			}
			diagnostics = append(diagnostics, "expired_candidate_skipped")
			continue
		}
		if item.Confidence < policy.MinimumConfidence {
			if item.ID != "" {
				rejected = append(rejected, item.ID)
			}
			diagnostics = append(diagnostics, "low_confidence_skipped")
			continue
		}
		filtered = append(filtered, item)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].ObservedAt.Equal(filtered[j].ObservedAt) {
			if sourcePriority(filtered[i].Source) == sourcePriority(filtered[j].Source) {
				return filtered[i].Value < filtered[j].Value
			}
			return sourcePriority(filtered[i].Source) > sourcePriority(filtered[j].Source)
		}
		return filtered[i].ObservedAt.After(filtered[j].ObservedAt)
	})
	if len(filtered) > policy.MaxCandidates {
		for _, item := range filtered[policy.MaxCandidates:] {
			if item.ID != "" {
				rejected = append(rejected, item.ID)
			}
		}
		filtered = filtered[:policy.MaxCandidates]
		diagnostics = append(diagnostics, "candidate_window_truncated")
	}

	groups := map[string]*aggregate{}
	for _, item := range filtered {
		entry, ok := groups[item.Value]
		if !ok {
			entry = &aggregate{
				Value:        item.Value,
				Confidence:   0,
				Source:       item.Source,
				ObservedAt:   item.ObservedAt,
				CandidateIDs: []string{},
			}
			groups[item.Value] = entry
		}
		entry.Confidence += item.Confidence * sourceWeight(item.Source)
		if sourcePriority(item.Source) > sourcePriority(entry.Source) {
			entry.Source = item.Source
		}
		if item.ObservedAt.After(entry.ObservedAt) {
			entry.ObservedAt = item.ObservedAt
		}
		if item.ID != "" {
			entry.CandidateIDs = append(entry.CandidateIDs, item.ID)
		}
	}

	options := make([]aggregate, 0, len(groups))
	for _, item := range groups {
		item.Confidence = round4(clampRange(0, 1, item.Confidence))
		sort.Strings(item.CandidateIDs)
		options = append(options, *item)
	}
	sort.SliceStable(options, func(i, j int) bool {
		if options[i].Confidence == options[j].Confidence {
			if sourcePriority(options[i].Source) == sourcePriority(options[j].Source) {
				return options[i].Value < options[j].Value
			}
			return sourcePriority(options[i].Source) > sourcePriority(options[j].Source)
		}
		return options[i].Confidence > options[j].Confidence
	})

	belief := ResolvedBelief{Key: input.Key, Unknown: true}
	if len(options) == 0 {
		diagnostics = append(diagnostics, "unknown_belief")
	} else {
		best := options[0]
		belief = ResolvedBelief{
			Key:          input.Key,
			Value:        best.Value,
			Confidence:   best.Confidence,
			Source:       best.Source,
			ObservedAt:   best.ObservedAt,
			CandidateIDs: best.CandidateIDs,
		}
		if len(options) > 1 {
			second := options[1]
			if best.Confidence-second.Confidence < policy.ConflictGap {
				belief.Conflicted = true
				diagnostics = append(diagnostics, "conflict_detected")
			}
		}
		if belief.Source == SourceKindInference {
			diagnostics = append(diagnostics, "inference_selected")
		}
	}

	sort.Strings(rejected)
	return ResolveResult{
		Version:  EngineVersionV1,
		Belief:   belief,
		Policy:   policy,
		Rejected: uniqueStrings(rejected),
		Audit: ResolverAudit{
			FormulaVersion: formulaVersion,
			Diagnostics:    uniqueStrings(diagnostics),
		},
	}
}

func normalizePolicy(input ResolverPolicy) (ResolverPolicy, []string) {
	policy := input
	diagnostics := []string{}
	defaults := DefaultPolicy()
	if policy.MinimumConfidence <= 0 {
		policy.MinimumConfidence = defaults.MinimumConfidence
	}
	if policy.ConflictGap <= 0 {
		policy.ConflictGap = defaults.ConflictGap
	}
	if policy.MaxCandidates <= 0 {
		policy.MaxCandidates = defaults.MaxCandidates
	}
	normalized := ResolverPolicy{
		MinimumConfidence: round4(clampRange(0.05, 0.95, policy.MinimumConfidence)),
		ConflictGap:       round4(clampRange(0.01, 0.5, policy.ConflictGap)),
		MaxCandidates:     clampInt(1, 64, policy.MaxCandidates),
	}
	if normalized != policy {
		diagnostics = append(diagnostics, "policy_clamped")
	}
	return normalized, diagnostics
}

func normalizeCandidate(input Candidate) Candidate {
	return Candidate{
		ID:         input.ID,
		Key:        input.Key,
		Value:      input.Value,
		Source:     normalizeSource(input.Source),
		Confidence: clampRange(0, 1, input.Confidence),
		ObservedAt: input.ObservedAt,
		ExpiresAt:  input.ExpiresAt,
	}
}

func normalizeSource(source SourceKind) SourceKind {
	switch source {
	case SourceKindFact, SourceKindUser, SourceKindMemory, SourceKindInference:
		return source
	default:
		return SourceKindInference
	}
}

func sourcePriority(source SourceKind) int {
	switch source {
	case SourceKindFact:
		return 4
	case SourceKindUser:
		return 3
	case SourceKindMemory:
		return 2
	default:
		return 1
	}
}

func sourceWeight(source SourceKind) float64 {
	switch source {
	case SourceKindFact:
		return 1
	case SourceKindUser:
		return 0.92
	case SourceKindMemory:
		return 0.84
	default:
		return 0.72
	}
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	out := values[:0]
	var last string
	for i, value := range values {
		if i == 0 || value != last {
			out = append(out, value)
			last = value
		}
	}
	return out
}

func clampInt(minimum, maximum, value int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func clampRange(minimum, maximum, value float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}
