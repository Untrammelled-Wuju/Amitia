package decision

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

type BeliefSourceKind string

const (
	BeliefSourceFact      BeliefSourceKind = "fact"
	BeliefSourceUser      BeliefSourceKind = "user"
	BeliefSourceRole      BeliefSourceKind = "role"
	BeliefSourceInference BeliefSourceKind = "inference"
)

type BeliefEntry struct {
	Key        string           `json:"key"`
	Value      string           `json:"value"`
	Confidence float64          `json:"confidence"`
	Source     BeliefSourceKind `json:"source"`
	ObservedAt time.Time        `json:"observedAt,omitempty"`
}

type BeliefSnapshot struct {
	ID          string        `json:"id"`
	CapturedAt  time.Time     `json:"capturedAt"`
	Facts       []BeliefEntry `json:"facts"`
	UserClaims  []BeliefEntry `json:"userClaims"`
	RoleBeliefs []BeliefEntry `json:"roleBeliefs"`
	Inferences  []BeliefEntry `json:"inferences"`
}

type BeliefSnapshotInput struct {
	Facts       []BeliefEntry
	UserClaims  []BeliefEntry
	RoleBeliefs []BeliefEntry
	Inferences  []BeliefEntry
	Now         time.Time
}

func BuildBeliefSnapshot(input BeliefSnapshotInput) BeliefSnapshot {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	copyFacts := make([]BeliefEntry, len(input.Facts))
	copy(copyFacts, input.Facts)
	copyUser := make([]BeliefEntry, len(input.UserClaims))
	copy(copyUser, input.UserClaims)
	copyRole := make([]BeliefEntry, len(input.RoleBeliefs))
	copy(copyRole, input.RoleBeliefs)
	copyInf := make([]BeliefEntry, len(input.Inferences))
	copy(copyInf, input.Inferences)

	snapshot := BeliefSnapshot{
		CapturedAt:  now,
		Facts:       copyFacts,
		UserClaims:  copyUser,
		RoleBeliefs: copyRole,
		Inferences:  copyInf,
	}
	snapshot.ID = computeBeliefSnapshotID(snapshot)
	return snapshot
}

func computeBeliefSnapshotID(snapshot BeliefSnapshot) string {
	parts := []string{
		snapshot.CapturedAt.Format(time.RFC3339Nano),
		fmt.Sprintf("f:%d", len(snapshot.Facts)),
		fmt.Sprintf("u:%d", len(snapshot.UserClaims)),
		fmt.Sprintf("r:%d", len(snapshot.RoleBeliefs)),
		fmt.Sprintf("i:%d", len(snapshot.Inferences)),
	}
	all := append([]BeliefEntry{}, snapshot.Facts...)
	all = append(all, snapshot.UserClaims...)
	all = append(all, snapshot.RoleBeliefs...)
	all = append(all, snapshot.Inferences...)
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].Key+string(all[i].Source) < all[j].Key+string(all[j].Source)
	})
	for _, e := range all {
		parts = append(parts, fmt.Sprintf("%s:%s:%s:%.4f", e.Source, e.Key, e.Value, e.Confidence))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return "belief-snapshot-" + hex.EncodeToString(sum[:])[:16]
}

func (s BeliefSnapshot) AllEntries() []BeliefEntry {
	total := len(s.Facts) + len(s.UserClaims) + len(s.RoleBeliefs) + len(s.Inferences)
	all := make([]BeliefEntry, 0, total)
	all = append(all, s.Facts...)
	all = append(all, s.UserClaims...)
	all = append(all, s.RoleBeliefs...)
	all = append(all, s.Inferences...)
	return all
}

func (s BeliefSnapshot) HighConfidence(minConfidence float64) []BeliefEntry {
	result := make([]BeliefEntry, 0)
	for _, e := range s.AllEntries() {
		if e.Confidence >= minConfidence {
			result = append(result, e)
		}
	}
	return result
}

func (s BeliefSnapshot) BySource(kind BeliefSourceKind) []BeliefEntry {
	switch kind {
	case BeliefSourceFact:
		return s.Facts
	case BeliefSourceUser:
		return s.UserClaims
	case BeliefSourceRole:
		return s.RoleBeliefs
	case BeliefSourceInference:
		return s.Inferences
	default:
		return nil
	}
}
