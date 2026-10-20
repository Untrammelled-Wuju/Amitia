package knowledge

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"sort"
	"strings"
	"unicode"
)

const (
	defaultMMRLambda              = 0.78
	semanticDuplicateThreshold    = 0.965
	crossSourceDuplicateThreshold = 0.985
)

// evidenceCandidate is an internal retrieval-stage value. It deliberately keeps
// the matched child separate from the read chunk (which may be its parent) so
// ranking can reason about both provenance and expanded context.
type evidenceCandidate struct {
	state       *fusionState
	matched     Chunk
	read        Chunk
	readID      string
	relevance   float64
	rerankScore float64
	mmrScore    float64
	fingerprint string
	features    map[string]struct{}
	duplicates  []duplicateEvidence
}

type duplicateEvidence struct {
	SourceID       string `json:"sourceId"`
	SourceRevision int64  `json:"sourceRevision"`
	ChunkID        string `json:"chunkId"`
	LocatorJSON    string `json:"locatorJson,omitempty"`
}

func prepareEvidenceCandidates(fused []*fusionState, chunkMap map[string]Chunk, minScore float64, parentTokenBudget int, parentLookup func(string) (*Chunk, error)) []evidenceCandidate {
	// First collapse multiple matched children that expand to the same parent.
	byReadID := make(map[string]*evidenceCandidate, len(fused))
	order := make([]string, 0, len(fused))
	parentTokensUsed := 0
	countedParents := make(map[string]struct{})
	for _, s := range fused {
		if s == nil || (minScore > 0 && s.fusion < minScore) {
			continue
		}
		matched, ok := chunkMap[s.chunkID]
		if !ok {
			continue
		}
		readChunk := matched
		readID := matched.ID
		if strings.TrimSpace(matched.ParentChunkID) != "" && parentLookup != nil && parentTokenBudget > 0 {
			if parent, err := parentLookup(matched.ParentChunkID); err == nil && parent != nil && strings.TrimSpace(parent.Content) != "" {
				parentTokens := parent.TokenCount
				if parentTokens <= 0 {
					parentTokens = estimateTokens(parent.Content)
				}
				_, alreadyCounted := countedParents[parent.ID]
				if alreadyCounted || parentTokensUsed+parentTokens <= parentTokenBudget {
					readChunk = *parent
					readID = parent.ID
					if !alreadyCounted {
						parentTokensUsed += parentTokens
						countedParents[parent.ID] = struct{}{}
					}
				}
			}
		}
		key := matched.SourceID + "\x00" + readID
		if existing := byReadID[key]; existing != nil {
			// Retain the strongest child signal for score breakdown, while merging
			// complementary modality scores from sibling matches.
			if s.fusion > existing.state.fusion {
				existing.state.fusion = s.fusion
				existing.matched = matched
			}
			if s.denseScore > existing.state.denseScore {
				existing.state.denseScore = s.denseScore
			}
			if s.sparseScore > existing.state.sparseScore {
				existing.state.sparseScore = s.sparseScore
			}
			if s.exactScore > existing.state.exactScore {
				existing.state.exactScore = s.exactScore
			}
			continue
		}
		stateCopy := *s
		candidate := &evidenceCandidate{state: &stateCopy, matched: matched, read: readChunk, readID: readID}
		candidate.fingerprint = contentFingerprint(readChunk.Content)
		candidate.features = similarityFeatures(readChunk.Title + "\n" + readChunk.Content)
		byReadID[key] = candidate
		order = append(order, key)
	}
	out := make([]evidenceCandidate, 0, len(order))
	for _, key := range order {
		out = append(out, *byReadID[key])
	}
	if len(out) == 0 {
		return out
	}

	maxFusion := 0.0
	for i := range out {
		if out[i].state.fusion > maxFusion {
			maxFusion = out[i].state.fusion
		}
	}
	if maxFusion <= 0 {
		maxFusion = 1
	}
	for i := range out {
		out[i].relevance = out[i].state.fusion / maxFusion
		if out[i].relevance > 1 {
			out[i].relevance = 1
		}
	}
	return out
}

// dedupeEvidenceCandidates removes exact duplicates and only extremely close
// semantic duplicates. Cross-source provenance is retained on the surviving
// candidate so dedupe never silently erases the fact that another source said
// the same thing. The threshold is intentionally strict to avoid dropping
// distinct evidence merely because it discusses the same topic.
func dedupeEvidenceCandidates(in []evidenceCandidate) []evidenceCandidate {
	if len(in) < 2 {
		return in
	}
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].state.fusion == in[j].state.fusion {
			return in[i].readID < in[j].readID
		}
		return in[i].state.fusion > in[j].state.fusion
	})
	out := make([]evidenceCandidate, 0, len(in))
	for _, candidate := range in {
		duplicateAt := -1
		for i := range out {
			existing := &out[i]
			exactDuplicate := candidate.fingerprint != "" && candidate.fingerprint == existing.fingerprint
			if exactDuplicate {
				duplicateAt = i
				break
			}
			sim := featureJaccard(candidate.features, existing.features)
			threshold := semanticDuplicateThreshold
			if candidate.matched.SourceID != existing.matched.SourceID {
				threshold = crossSourceDuplicateThreshold
			}
			// Never treat short snippets as semantic duplicates from similarity
			// alone; short passages often share boilerplate tokens accidentally.
			if len(candidate.features) >= 12 && len(existing.features) >= 12 && sim >= threshold {
				duplicateAt = i
				break
			}
		}
		if duplicateAt < 0 {
			out = append(out, candidate)
			continue
		}
		winner := &out[duplicateAt]
		winner.duplicates = append(winner.duplicates, duplicateEvidence{
			SourceID: candidate.matched.SourceID, SourceRevision: candidate.matched.SourceRevision,
			ChunkID: candidate.matched.ID, LocatorJSON: candidate.matched.LocatorJSON,
		})
	}
	return out
}

// selectMMR applies Maximal Marginal Relevance with a soft per-source diversity
// budget. The budget is not a hard filter: if alternatives are materially less
// relevant, the best same-source evidence is still selected. This prevents one
// long document from monopolising the context without sacrificing strong facts.
func selectMMR(candidates []evidenceCandidate, limit int) []evidenceCandidate {
	if limit <= 0 || len(candidates) == 0 {
		return []evidenceCandidate{}
	}
	if limit > len(candidates) {
		limit = len(candidates)
	}
	remaining := append([]evidenceCandidate(nil), candidates...)
	selected := make([]evidenceCandidate, 0, limit)
	sourceCounts := map[string]int{}
	maxPerSource := int(math.Ceil(float64(limit) * 0.6))
	if maxPerSource < 2 {
		maxPerSource = 2
	}
	if maxPerSource > limit {
		maxPerSource = limit
	}

	for len(selected) < limit && len(remaining) > 0 {
		best := -1
		bestScore := math.Inf(-1)
		bestUncapped := -1
		bestUncappedScore := math.Inf(-1)
		for i := range remaining {
			candidate := &remaining[i]
			maxSimilarity := 0.0
			for j := range selected {
				sim := candidateSimilarity(*candidate, selected[j])
				if sim > maxSimilarity {
					maxSimilarity = sim
				}
			}
			score := defaultMMRLambda*candidate.relevance - (1-defaultMMRLambda)*maxSimilarity
			candidate.mmrScore = score
			if score > bestUncappedScore {
				bestUncapped = i
				bestUncappedScore = score
			}
			if sourceCounts[candidate.matched.SourceID] >= maxPerSource {
				continue
			}
			if score > bestScore {
				best = i
				bestScore = score
			}
		}

		// The source cap is soft. If the best uncapped candidate is much more
		// relevant than every diverse alternative, keep the stronger evidence.
		if bestUncapped >= 0 && best >= 0 && bestUncapped != best {
			uncapped := remaining[bestUncapped]
			diverse := remaining[best]
			if uncapped.relevance >= 0.90 && diverse.relevance < uncapped.relevance*0.62 {
				best = bestUncapped
				bestScore = bestUncappedScore
			}
		}
		if best < 0 {
			best = bestUncapped
			bestScore = bestUncappedScore
		}
		if best < 0 {
			break
		}
		chosen := remaining[best]
		chosen.mmrScore = bestScore
		selected = append(selected, chosen)
		sourceCounts[chosen.matched.SourceID]++
		remaining = append(remaining[:best], remaining[best+1:]...)
	}
	return selected
}

func candidateSimilarity(a, b evidenceCandidate) float64 {
	sim := featureJaccard(a.features, b.features)
	if a.readID != "" && a.readID == b.readID {
		return 1
	}
	// Same source/unit adjacency is often overlapping chunk material. Penalise
	// it modestly, but not enough to suppress genuinely distinct facts.
	if a.matched.SourceID == b.matched.SourceID {
		if a.matched.UnitID != "" && a.matched.UnitID == b.matched.UnitID {
			if sim < 0.72 {
				sim = 0.72
			}
		} else if sim < 0.18 {
			sim = 0.18
		}
	}
	return sim
}

func contentFingerprint(s string) string {
	normalized := normalizeContentForFingerprint(s)
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func normalizeContentForFingerprint(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsSpace(r) {
			if b.Len() > 0 {
				space = true
			}
			continue
		}
		if unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func similarityFeatures(s string) map[string]struct{} {
	normalized := normalizeContentForFingerprint(s)
	features := map[string]struct{}{}
	if normalized == "" {
		return features
	}
	words := strings.Fields(normalized)
	for _, word := range words {
		if len([]rune(word)) >= 2 {
			features["w:"+word] = struct{}{}
		}
	}
	// Rune trigrams make the similarity measure work for CJK text without a
	// language-specific tokenizer. Spaces are removed so formatting does not
	// dominate duplicate detection.
	runes := []rune(strings.ReplaceAll(normalized, " ", ""))
	if len(runes) < 3 {
		if len(runes) > 0 {
			features["r:"+string(runes)] = struct{}{}
		}
		return features
	}
	for i := 0; i+3 <= len(runes); i++ {
		features["g:"+string(runes[i:i+3])] = struct{}{}
		if len(features) >= 2048 {
			break
		}
	}
	return features
}

func featureJaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	intersection := 0
	for key := range a {
		if _, ok := b[key]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union <= 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func candidateIdentity(c evidenceCandidate) string {
	return c.matched.SourceID + "\x00" + c.readID + "\x00" + c.matched.ID
}

func candidateReferenceIDs(c evidenceCandidate) []string {
	values := []string{c.matched.ID, c.readID, c.matched.UnitID}
	metadata := unmarshalMap(c.matched.MetadataJSON)
	for _, key := range []string{"id", "fact_id", "factId", "external_id", "externalId"} {
		if raw, ok := metadata[key]; ok {
			if value := strings.TrimSpace(toString(raw)); value != "" {
				values = append(values, value)
			}
		}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func candidateEffectiveTime(c evidenceCandidate) int64 {
	if c.matched.ValidFrom != nil {
		return c.matched.ValidFrom.UnixNano()
	}
	if !c.matched.CreatedAt.IsZero() {
		return c.matched.CreatedAt.UnixNano()
	}
	return 0
}

// applySupersession removes an older candidate only when another visible
// candidate explicitly names one of its stable/internal identifiers in
// supersedes. It intentionally does not infer replacement from semantic
// similarity. Historical/all-time callers can therefore reconstruct the old
// evidence rather than having it silently erased.
func applySupersession(in []evidenceCandidate, temporal temporalQuery) []evidenceCandidate {
	if temporal.Mode == TemporalModeAll || len(in) < 2 {
		return in
	}
	refs := make(map[string]int, len(in)*3)
	for i := range in {
		for _, ref := range candidateReferenceIDs(in[i]) {
			refs[ref] = i
		}
	}
	dropped := make(map[int]struct{})
	for i := range in {
		supersedes := unmarshalStringList(in[i].matched.SupersedesJSON)
		if len(supersedes) == 0 && in[i].readID != in[i].matched.ID {
			supersedes = unmarshalStringList(in[i].read.SupersedesJSON)
		}
		for _, ref := range supersedes {
			target, ok := refs[strings.TrimSpace(ref)]
			if !ok || target == i {
				continue
			}
			// Reject cycles and backwards replacement. A superseding fact must be
			// at least as new as its target; deterministic identity breaks exact ties.
			newTime := candidateEffectiveTime(in[i])
			oldTime := candidateEffectiveTime(in[target])
			if newTime < oldTime || (newTime == oldTime && candidateIdentity(in[i]) < candidateIdentity(in[target])) {
				continue
			}
			dropped[target] = struct{}{}
		}
	}
	if len(dropped) == 0 {
		return in
	}
	out := make([]evidenceCandidate, 0, len(in)-len(dropped))
	for i := range in {
		if _, drop := dropped[i]; !drop {
			out = append(out, in[i])
		}
	}
	return out
}

func candidateConflictGroup(c evidenceCandidate) string {
	if group := strings.TrimSpace(c.matched.ConflictGroup); group != "" {
		return group
	}
	return strings.TrimSpace(c.read.ConflictGroup)
}

func conflictGroupsFromCandidates(in []evidenceCandidate) []string {
	fingerprints := map[string]map[string]struct{}{}
	for _, candidate := range in {
		group := candidateConflictGroup(candidate)
		if group == "" {
			continue
		}
		fingerprint := candidate.fingerprint
		if fingerprint == "" {
			fingerprint = contentFingerprint(candidate.read.Content)
		}
		if fingerprint == "" {
			continue
		}
		if fingerprints[group] == nil {
			fingerprints[group] = map[string]struct{}{}
		}
		fingerprints[group][fingerprint] = struct{}{}
	}
	groups := make([]string, 0, len(fingerprints))
	for group, values := range fingerprints {
		if len(values) >= 2 {
			groups = append(groups, group)
		}
	}
	sort.Strings(groups)
	return groups
}

// preserveConflictAlternatives makes ambiguity inspectable. If MMR selected one
// member of an explicit conflict group, try to include a second distinct claim
// from the same group. It only replaces the weakest non-conflict result and
// never grows beyond the caller's context budget.
func preserveConflictAlternatives(selected, pool []evidenceCandidate, limit int, conflictGroups []string) []evidenceCandidate {
	if limit < 2 || len(selected) == 0 || len(conflictGroups) == 0 {
		return selected
	}
	ambiguous := make(map[string]struct{}, len(conflictGroups))
	for _, group := range conflictGroups {
		ambiguous[group] = struct{}{}
	}
	selectedIDs := map[string]struct{}{}
	for _, candidate := range selected {
		selectedIDs[candidateIdentity(candidate)] = struct{}{}
	}
	for _, group := range conflictGroups {
		selectedInGroup := make([]evidenceCandidate, 0, 2)
		selectedFingerprints := map[string]struct{}{}
		for _, candidate := range selected {
			if candidateConflictGroup(candidate) != group {
				continue
			}
			selectedInGroup = append(selectedInGroup, candidate)
			selectedFingerprints[candidate.fingerprint] = struct{}{}
		}
		if len(selectedInGroup) == 0 || len(selectedFingerprints) >= 2 {
			continue
		}
		var alternative *evidenceCandidate
		for i := range pool {
			candidate := pool[i]
			if candidateConflictGroup(candidate) != group {
				continue
			}
			if _, exists := selectedIDs[candidateIdentity(candidate)]; exists {
				continue
			}
			if _, same := selectedFingerprints[candidate.fingerprint]; same {
				continue
			}
			if alternative == nil || candidate.relevance > alternative.relevance {
				copy := candidate
				alternative = &copy
			}
		}
		if alternative == nil {
			continue
		}
		if len(selected) < limit {
			selected = append(selected, *alternative)
			selectedIDs[candidateIdentity(*alternative)] = struct{}{}
			continue
		}
		replace := -1
		weakest := 2.0
		for i := range selected {
			candidateGroup := candidateConflictGroup(selected[i])
			if _, protected := ambiguous[candidateGroup]; protected {
				continue
			}
			if selected[i].relevance < weakest {
				weakest = selected[i].relevance
				replace = i
			}
		}
		if replace >= 0 {
			delete(selectedIDs, candidateIdentity(selected[replace]))
			selected[replace] = *alternative
			selectedIDs[candidateIdentity(*alternative)] = struct{}{}
		}
	}
	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].relevance == selected[j].relevance {
			return candidateIdentity(selected[i]) < candidateIdentity(selected[j])
		}
		return selected[i].relevance > selected[j].relevance
	})
	return selected
}

func toString(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case []byte:
		return string(value)
	default:
		return ""
	}
}
