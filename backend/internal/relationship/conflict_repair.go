package relationship

import (
	"time"
)

func DefaultConflictState() ConflictState {
	return ConflictState{
		ActiveConflicts: make([]ActiveConflict, 0),
		RepairAttempts:  make([]RepairAttempt, 0),
		ActiveRepair:    false,
		ConflictCount:   0,
		ResolvedCount:   0,
	}
}

func StartConflict(state *ConflictState, sourceID string, intensity float64) *ActiveConflict {
	if state == nil {
		return nil
	}
	now := time.Now()
	conflict := ActiveConflict{
		ID:        generateConflictID(),
		SourceID:  sourceID,
		Intensity: intensity,
		StartedAt: now,
		Escalated: false,
	}
	state.ActiveConflicts = append(state.ActiveConflicts, conflict)
	state.ConflictCount++
	return &state.ActiveConflicts[len(state.ActiveConflicts)-1]
}

func ResolveConflict(state *ConflictState, conflictID string) bool {
	if state == nil {
		return false
	}
	for i := range state.ActiveConflicts {
		c := &state.ActiveConflicts[i]
		if c.ID == conflictID && c.ResolvedAt == nil {
			now := time.Now()
			c.ResolvedAt = &now
			state.ResolvedCount++
			return true
		}
	}
	return false
}

func RecordRepairAttempt(state *ConflictState, conflictID string, effective bool, confidence float64) {
	if state == nil {
		return
	}
	attempt := RepairAttempt{
		ID:          generateRepairID(),
		ConflictID:  conflictID,
		Effective:   effective,
		Confidence:  confidence,
		AttemptedAt: time.Now(),
	}
	state.RepairAttempts = append(state.RepairAttempts, attempt)
}

func ComputeRepairConfidence(state ConflictState) float64 {
	if state.RepairAttempts == nil || len(state.RepairAttempts) == 0 {
		return 0.35
	}
	successful := 0
	recentSuccess := 0
	total := len(state.RepairAttempts)
	recentWindow := 3
	if total < recentWindow {
		recentWindow = total
	}

	for i, a := range state.RepairAttempts {
		if a.Effective {
			successful++
			if i >= total-recentWindow {
				recentSuccess++
			}
		}
	}

	overallRate := float64(successful) / float64(total)
	recentRate := float64(recentSuccess) / float64(recentWindow)
	base := 0.15 + overallRate*0.45 + recentRate*0.20

	unresolved := len(state.ActiveConflicts)
	if unresolved > 0 {
		unresolvedPenalty := float64(unresolved) * 0.05
		if unresolvedPenalty > 0.15 {
			unresolvedPenalty = 0.15
		}
		base -= unresolvedPenalty
	}

	return clamp01(base)
}

func CheckActiveRepairTrigger(state ConflictState, security float64) bool {
	if state.ActiveRepair {
		return false
	}
	repairConf := ComputeRepairConfidence(state)
	unresolved := 0
	for _, c := range state.ActiveConflicts {
		if c.ResolvedAt == nil {
			unresolved++
		}
	}
	if unresolved == 0 {
		return false
	}
	conflictRatio := float64(unresolved) / float64(state.ConflictCount+1)
	if conflictRatio > 0.3 && repairConf < 0.4 {
		return true
	}
	if security < 0.35 && repairConf < 0.45 {
		return true
	}
	return false
}

func TriggerActiveRepair(state *ConflictState) bool {
	if state == nil || state.ActiveRepair {
		return false
	}
	state.ActiveRepair = true
	state.RepairTriggeredAt = time.Now()
	return true
}

func ClearActiveRepair(state *ConflictState) {
	if state == nil {
		return
	}
	state.ActiveRepair = false
}

func ComputeConflictTension(conflicts []ActiveConflict) float64 {
	if len(conflicts) == 0 {
		return 0
	}
	tension := 0.0
	unresolvedCount := 0
	now := time.Now()

	for _, c := range conflicts {
		if c.ResolvedAt != nil {
			hoursAgo := now.Sub(*c.ResolvedAt).Hours()
			residual := c.Intensity * 0.3 * expDecay(hoursAgo, 24)
			tension += residual
		} else {
			tension += c.Intensity * 0.9
			if c.Escalated {
				tension += 0.1
			}
			unresolvedCount++
		}
	}

	if unresolvedCount > 1 {
		tension += float64(unresolvedCount-1) * 0.08
	}

	return clamp01(tension)
}

func expDecay(hours, halfLife float64) float64 {
	if halfLife <= 0 {
		return 1.0
	}
	decay := 1.0
	for i := 0.0; i < hours; i += halfLife {
		decay *= 0.5
	}
	return decay
}

var conflictIDCounter int
var repairIDCounter int

func generateConflictID() string {
	conflictIDCounter++
	return "conflict-" + itoa(conflictIDCounter)
}

func generateRepairID() string {
	repairIDCounter++
	return "repair-" + itoa(repairIDCounter)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
