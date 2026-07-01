package mindruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type SupervisorDecision string

const (
	SupervisorApproved    SupervisorDecision = "APPROVED"
	SupervisorRejected    SupervisorDecision = "REJECTED"
	SupervisorEscalate    SupervisorDecision = "ESCALATE"
	SupervisorRolledBack  SupervisorDecision = "ROLLED_BACK"
	SupervisorSuperseded  SupervisorDecision = "SUPERSEDED"
)

type SupervisorTarget string

const (
	SupervisorTargetPersonality  SupervisorTarget = "personality"
	SupervisorTargetSummary      SupervisorTarget = "summary"
	SupervisorTargetReflection   SupervisorTarget = "reflection"
	SupervisorTargetGrowth       SupervisorTarget = "growth"
)

type SupervisorInput struct {
	Target         SupervisorTarget
	CharacterID    string
	RequestID      string
	EvidenceCount  int
	Counterexample int
	Authority      int
	Sensitive      bool
	BudgetUsed     float64
	BudgetLimit    float64
	PreviousRunIDs []string
	DerivedFromIDs []string
	CreatedAt      time.Time
}

type SupervisorDecisionRecord struct {
	Version        SupervisorVersion `json:"version"`
	ID             string            `json:"id"`
	Target         SupervisorTarget  `json:"target"`
	CharacterID    string            `json:"characterId"`
	RequestID      string            `json:"requestId,omitempty"`
	Decision       SupervisorDecision `json:"decision"`
	EvidenceCount  int               `json:"evidenceCount"`
	Counterexample int               `json:"counterexample"`
	Authority      int               `json:"authority"`
	Sensitive      bool              `json:"sensitive"`
	BudgetUsed     float64           `json:"budgetUsed"`
	BudgetLimit    float64           `json:"budgetLimit"`
	CreatedAt      time.Time         `json:"createdAt"`
	ExpiresAt      time.Time         `json:"expiresAt,omitempty"`
	Reason         string            `json:"reason"`
	RevertedBy     string            `json:"revertedBy,omitempty"`
	RevertedAt     time.Time         `json:"revertedAt,omitempty"`
	PreviousRuns   []string          `json:"previousRuns,omitempty"`
	DerivedFrom    []string          `json:"derivedFrom,omitempty"`
}

func NewSupervisorVersion() SupervisorVersion {
	return SupervisorVersion(fmt.Sprintf("supervisor-v%d", time.Now().UnixNano()))
}

type SupervisorVersion string

func DefaultSupervisorConfig() SupervisorConfig {
	return SupervisorConfig{
		MinEvidenceCount:  2,
		MaxCounterexample: 1,
		MinAuthority:      3,
		BudgetThreshold:   0.8,
		SensitiveEscalate: true,
	}
}

type SupervisorConfig struct {
	MinEvidenceCount  int
	MaxCounterexample int
	MinAuthority      int
	BudgetThreshold   float64
	SensitiveEscalate bool
}

func ReviewCandidate(input SupervisorInput, config SupervisorConfig) SupervisorDecisionRecord {
	reasons := make([]string, 0)
	decision := SupervisorApproved

	if input.Authority < config.MinAuthority {
		reasons = append(reasons, fmt.Sprintf("authority %d below minimum %d", input.Authority, config.MinAuthority))
		decision = SupervisorRejected
	}

	if input.EvidenceCount < config.MinEvidenceCount {
		reasons = append(reasons, fmt.Sprintf("evidence count %d below minimum %d", input.EvidenceCount, config.MinEvidenceCount))
		if decision == SupervisorApproved {
			decision = SupervisorRejected
		}
	}

	if input.Counterexample > config.MaxCounterexample {
		reasons = append(reasons, fmt.Sprintf("counterexample count %d exceeds maximum %d", input.Counterexample, config.MaxCounterexample))
		if decision == SupervisorApproved {
			decision = SupervisorRejected
		}
	}

	if input.BudgetLimit > 0 && input.BudgetUsed/input.BudgetLimit > config.BudgetThreshold {
		reasons = append(reasons, fmt.Sprintf("budget usage %.0f%% exceeds threshold %.0f%%", input.BudgetUsed/input.BudgetLimit*100, config.BudgetThreshold*100))
		if decision == SupervisorApproved {
			decision = SupervisorRejected
		}
	}

	if input.Sensitive && config.SensitiveEscalate {
		reasons = append(reasons, "sensitive content requires manual confirmation")
		if decision == SupervisorApproved {
			decision = SupervisorEscalate
		}
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "all checks passed")
	}

	record := SupervisorDecisionRecord{
		Version:        NewSupervisorVersion(),
		ID:             supervisorDecisionID(input, decision),
		Target:         input.Target,
		CharacterID:    strings.TrimSpace(input.CharacterID),
		RequestID:      strings.TrimSpace(input.RequestID),
		Decision:       decision,
		EvidenceCount:  input.EvidenceCount,
		Counterexample: input.Counterexample,
		Authority:      input.Authority,
		Sensitive:      input.Sensitive,
		BudgetUsed:     input.BudgetUsed,
		BudgetLimit:    input.BudgetLimit,
		CreatedAt:      input.CreatedAt.UTC(),
		Reason:         strings.Join(reasons, "; "),
		PreviousRuns:   normalizeSupervisorRefs(input.PreviousRunIDs),
		DerivedFrom:    normalizeSupervisorRefs(input.DerivedFromIDs),
	}

	if input.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}

	return record
}

type RollbackTarget struct {
	OriginalRecordID string
	CharacterID      string
	Target           SupervisorTarget
	Reason           string
	RequestID        string
}

func CreateRollbackEvent(target RollbackTarget, originalRecord SupervisorDecisionRecord) SupervisorDecisionRecord {
	record := SupervisorDecisionRecord{
		Version:   NewSupervisorVersion(),
		ID:        rollbackEventID(target, originalRecord),
		Target:    originalRecord.Target,
		CharacterID: strings.TrimSpace(target.CharacterID),
		RequestID: strings.TrimSpace(target.RequestID),
		Decision:  SupervisorRolledBack,
		CreatedAt: time.Now().UTC(),
		Reason:    strings.TrimSpace(target.Reason),
		PreviousRuns: []string{originalRecord.ID},
	}
	reason := strings.TrimSpace(target.Reason)
	if reason == "" {
		record.Reason = "manual rollback requested"
	} else {
		record.Reason = reason
	}
	return record
}

func IsRolledBack(record SupervisorDecisionRecord) bool {
	return record.Decision == SupervisorRolledBack
}

func IsActive(record SupervisorDecisionRecord, now time.Time) bool {
	if record.Decision == SupervisorRolledBack {
		return false
	}
	if !record.ExpiresAt.IsZero() && !now.UTC().Before(record.ExpiresAt) {
		return false
	}
	return record.Decision == SupervisorApproved
}

func ApprovedDecisions(records []SupervisorDecisionRecord) []SupervisorDecisionRecord {
	result := make([]SupervisorDecisionRecord, 0)
	now := time.Now().UTC()
	for _, r := range records {
		if IsActive(r, now) {
			result = append(result, r)
		}
	}
	return result
}

func AffectedDecisions(record SupervisorDecisionRecord, allRecords []SupervisorDecisionRecord) []SupervisorDecisionRecord {
	affected := make([]SupervisorDecisionRecord, 0)
	for _, r := range allRecords {
		if r.ID == record.ID {
			continue
		}
		if r.CharacterID != record.CharacterID {
			continue
		}
		if !strings.EqualFold(string(r.Target), string(record.Target)) {
			continue
		}
		for _, derived := range r.DerivedFrom {
			if derived == record.ID {
				affected = append(affected, r)
				break
			}
		}
	}
	return affected
}

func InvalidateDerivedRecords(record SupervisorDecisionRecord, allRecords []SupervisorDecisionRecord) []SupervisorDecisionRecord {
	updated := make([]SupervisorDecisionRecord, len(allRecords))
	copy(updated, allRecords)
	targetIDs := make(map[string]bool)
	for _, pid := range record.PreviousRuns {
		if strings.TrimSpace(pid) != "" {
			targetIDs[pid] = true
		}
	}
	if len(targetIDs) == 0 {
		return updated
	}
	now := time.Now().UTC()
	for i, r := range updated {
		if r.CharacterID != record.CharacterID {
			continue
		}
		if !strings.EqualFold(string(r.Target), string(record.Target)) {
			continue
		}
		for _, derived := range r.DerivedFrom {
			if targetIDs[derived] {
				updated[i].Decision = SupervisorSuperseded
				updated[i].RevertedBy = record.ID
				updated[i].RevertedAt = now
				break
			}
		}
	}
	return updated
}

func supervisorDecisionID(input SupervisorInput, decision SupervisorDecision) string {
	parts := []string{
		"supervisor",
		string(input.Target),
		strings.TrimSpace(input.CharacterID),
		strings.TrimSpace(input.RequestID),
		string(decision),
		fmt.Sprintf("ev%d", input.EvidenceCount),
		fmt.Sprintf("ct%d", input.Counterexample),
		fmt.Sprintf("au%d", input.Authority),
		fmt.Sprintf("bu%.2f", input.BudgetUsed),
		fmt.Sprintf("bl%.2f", input.BudgetLimit),
	}
	if !input.CreatedAt.IsZero() {
		parts = append(parts, input.CreatedAt.Format(time.RFC3339Nano))
	}
	raw := strings.Join(parts, "|")
	sum := sha256.Sum256([]byte(raw))
	return "supervisor-" + hex.EncodeToString(sum[:])[:20]
}

func rollbackEventID(target RollbackTarget, original SupervisorDecisionRecord) string {
	parts := []string{
		"rollback",
		string(target.Target),
		strings.TrimSpace(target.CharacterID),
		original.ID,
		strings.TrimSpace(target.Reason),
	}
	raw := strings.Join(parts, "|")
	sum := sha256.Sum256([]byte(raw))
	return "rollback-" + hex.EncodeToString(sum[:])[:20]
}

func normalizeSupervisorRefs(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	result := make([]string, 0, len(items))
	for _, item := range items {
		v := strings.TrimSpace(item)
		if v != "" && !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

