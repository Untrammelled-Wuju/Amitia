package acquisition

import (
	"sort"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

// basePriority returns the level-based base score for a candidate kind/state.
// Higher values indicate stronger intrinsic preference.
//
// Ranking tiers:
//  1. Installed, only needs enable        -> 1000
//  2. Installed, needs runtime recovery   -> 900
//  3. Built-in / trusted first-party       -> 800
//  4. Verified-signature extension         -> 700
//  5. Verified MCP                         -> 600
//  6. Verified Skill                       -> 500
//  7. User-specified source                -> 400
//  8. Generated Skill                      -> 300
//  9. Unverified source                    -> 100
func basePriority(c CapabilityCandidate) float64 {
	switch {
	case c.Install.Method == InstallEnableExisting:
		return 1000
	case c.Kind == CandidateInstalledExtension && c.Install.Method != InstallEnableExisting:
		return 900
	case c.Kind == CandidateBuiltin:
		return 800
	case c.Kind == CandidateExtensionPackage && c.Trust.SignatureVerified:
		return 700
	case c.Kind == CandidateMCP && c.Trust.Level == TrustVerified:
		return 600
	case c.Kind == CandidateAgentSkill && c.Trust.Level == TrustVerified:
		return 500
	case c.Kind == CandidateExtensionPackage && c.Trust.Level != TrustVerified:
		return 400
	case c.Kind == CandidateGeneratedSkill:
		return 300
	default:
		return 100
	}
}

// tieBreak ensures a deterministic ordering between two candidates that share
// the same kind tier. It compares capability-match quality, permission scope,
// runtime suitability, dependency count, declared priority, and finally the
// candidate ID as a stable lexicographic tie-break.
func tieBreak(a, b CapabilityCandidate, req AcquisitionRequest) bool {
	// exact capability match bonus
	aExact := a.Match.ExactCapabilities > 0
	bExact := b.Match.ExactCapabilities > 0
	if aExact != bExact {
		return aExact
	}

	// prefer smaller permission scope
	if len(a.Permissions) != len(b.Permissions) {
		return len(a.Permissions) < len(b.Permissions)
	}

	// prefer better runtime target suitability
	aRuntime := runtimeSuitability(a, req)
	bRuntime := runtimeSuitability(b, req)
	if aRuntime != bRuntime {
		return aRuntime > bRuntime
	}

	// prefer fewer dependencies
	if len(a.Dependencies) != len(b.Dependencies) {
		return len(a.Dependencies) < len(b.Dependencies)
	}

	// prefer lower install cost (enable < built-in < others)
	aCost := installCost(a)
	bCost := installCost(b)
	if aCost != bCost {
		return aCost < bCost
	}

	// priority field from metadata
	aPri := priorityFromMetadata(a)
	bPri := priorityFromMetadata(b)
	if aPri != bPri {
		return aPri > bPri
	}

	// stable tie-break by ID
	return a.ID < b.ID
}

// CandidateScorer computes a composite score for ranking candidates.
type CandidateScorer struct{}

// NewCandidateScorer returns a new CandidateScorer.
func NewCandidateScorer() *CandidateScorer {
	return &CandidateScorer{}
}

// Score returns the composite score for a candidate relative to the request.
// The score equals basePriority plus per-candidate adjustment bonuses.
func (s *CandidateScorer) Score(candidate CapabilityCandidate, request AcquisitionRequest) float64 {
	score := basePriority(candidate)

	if candidate.Match.ExactCapabilities > 0 {
		score += 100
	}

	if len(candidate.Permissions) == 0 || narrowScope(candidate.Permissions) {
		score += 50
	}

	if runtimeTargetMatches(candidate, request) {
		score += 30
	}

	if len(candidate.Dependencies) == 0 {
		score += 20
	}

	score += installCostBonus(candidate)
	score += priorityFromMetadata(candidate)

	return score
}

// RankCandidates returns a new slice sorted by descending score using stable
// tie-breaking rules. The input slice is not mutated.
func RankCandidates(candidates []CapabilityCandidate, request AcquisitionRequest) []CapabilityCandidate {
	if len(candidates) <= 1 {
		out := make([]CapabilityCandidate, len(candidates))
		copy(out, candidates)
		return out
	}

	scorer := NewCandidateScorer()
	scored := make([]scoredCandidate, 0, len(candidates))
	for _, c := range candidates {
		scored = append(scored, scoredCandidate{
			candidate: c,
			score:     scorer.Score(c, request),
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return tieBreak(scored[i].candidate, scored[j].candidate, request)
	})

	result := make([]CapabilityCandidate, len(scored))
	for i, s := range scored {
		result[i] = s.candidate
	}
	return result
}

type scoredCandidate struct {
	candidate CapabilityCandidate
	score     float64
}

// runtimeTargetMatches reports whether the candidate can run in the placement
// implied by the request.
func runtimeTargetMatches(c CapabilityCandidate, req AcquisitionRequest) bool {
	if req.RequiredPlacement == "" && req.RequiredDeviceID == "" && req.PreferredPlacement == "" {
		return true
	}
	target := effectivePlacement(req)
	for _, p := range c.Runtime.Placements {
		if string(p) == target {
			return true
		}
	}
	return false
}

// runtimeSuitability returns a coarse 0–3 hint of how well the candidate
// matches the request placement.
func runtimeSuitability(c CapabilityCandidate, req AcquisitionRequest) int {
	target := effectivePlacement(req)
	if target == "" {
		return 1
	}
	for _, p := range c.Runtime.Placements {
		if string(p) == target {
			return 3
		}
	}
	if len(c.Runtime.Placements) > 0 {
		return 0
	}
	return 1
}

// effectivePlacement resolves the most specific placement constraint from the
// request.
func effectivePlacement(req AcquisitionRequest) string {
	switch {
	case req.RequiredPlacement != "":
		return string(req.RequiredPlacement)
	case req.RequiredDeviceID != "":
		return string(capability.ProviderPlacementDevice)
	case req.PreferredPlacement != "":
		return string(req.PreferredPlacement)
	}
	return ""
}

// installCost returns a coarse cost ordering so that cheaper installs win.
func installCost(c CapabilityCandidate) int {
	switch c.Install.Method {
	case InstallEnableExisting:
		return 0
	case InstallExtension:
		return 3
	case InstallMCP:
		return 3
	case InstallSkill:
		return 2
	case InstallGeneratedSkill:
		return 1
	}
	switch c.Kind {
	case CandidateBuiltin:
		return 0
	}
	return 3
}

// installCostBonus returns a small score delta: lower cost -> higher bonus.
func installCostBonus(c CapabilityCandidate) float64 {
	switch installCost(c) {
	case 0:
		return 10
	case 1:
		return 7
	case 2:
		return 4
	}
	return 0
}

// narrowScope reports whether the permission set is empty or considered narrow
// (no dangerous capabilities).
func narrowScope(perms []string) bool {
	for _, p := range perms {
		if isSensitivePermission(p) {
			return false
		}
	}
	return true
}

// priorityFromMetadata reads an optional numeric "priority" from candidate
// metadata. Missing or invalid values default to zero.
func priorityFromMetadata(c CapabilityCandidate) float64 {
	if c.Metadata == nil {
		return 0
	}
	v, ok := c.Metadata["priority"]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}
