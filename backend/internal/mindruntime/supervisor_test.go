package mindruntime

import (
	"testing"
	"time"
)

func TestReviewCandidate_Approved(t *testing.T) {
	config := DefaultSupervisorConfig()
	input := SupervisorInput{
		Target:         SupervisorTargetSummary,
		CharacterID:    "char-001",
		RequestID:      "req-001",
		EvidenceCount:  5,
		Counterexample: 0,
		Authority:      7,
		Sensitive:      false,
		BudgetUsed:     0.3,
		BudgetLimit:    1.0,
		CreatedAt:      time.Now(),
	}
	record := ReviewCandidate(input, config)
	if record.Decision != SupervisorApproved {
		t.Errorf("expected APPROVED, got %s: %s", record.Decision, record.Reason)
	}
	if record.Target != SupervisorTargetSummary {
		t.Errorf("expected target summary, got %s", record.Target)
	}
	if record.CharacterID != "char-001" {
		t.Errorf("expected char-001, got %s", record.CharacterID)
	}
	if record.EvidenceCount != 5 {
		t.Errorf("expected evidence 5, got %d", record.EvidenceCount)
	}
}

func TestReviewCandidate_Rejected_LowAuthority(t *testing.T) {
	config := DefaultSupervisorConfig()
	input := SupervisorInput{
		Target:         SupervisorTargetReflection,
		CharacterID:    "char-001",
		RequestID:      "req-002",
		EvidenceCount:  3,
		Counterexample: 0,
		Authority:      1,
		Sensitive:      false,
		BudgetUsed:     0.1,
		BudgetLimit:    1.0,
		CreatedAt:      time.Now(),
	}
	record := ReviewCandidate(input, config)
	if record.Decision != SupervisorRejected {
		t.Errorf("expected REJECTED for low authority, got %s: %s", record.Decision, record.Reason)
	}
}

func TestReviewCandidate_Rejected_LowEvidence(t *testing.T) {
	config := DefaultSupervisorConfig()
	input := SupervisorInput{
		Target:         SupervisorTargetGrowth,
		CharacterID:    "char-002",
		RequestID:      "req-003",
		EvidenceCount:  0,
		Counterexample: 0,
		Authority:      5,
		Sensitive:      false,
		BudgetUsed:     0.1,
		BudgetLimit:    1.0,
		CreatedAt:      time.Now(),
	}
	record := ReviewCandidate(input, config)
	if record.Decision != SupervisorRejected {
		t.Errorf("expected REJECTED for low evidence, got %s: %s", record.Decision, record.Reason)
	}
}

func TestReviewCandidate_Rejected_TooManyCounterexamples(t *testing.T) {
	config := DefaultSupervisorConfig()
	input := SupervisorInput{
		Target:         SupervisorTargetReflection,
		CharacterID:    "char-003",
		RequestID:      "req-004",
		EvidenceCount:  5,
		Counterexample: 3,
		Authority:      6,
		Sensitive:      false,
		BudgetUsed:     0.2,
		BudgetLimit:    1.0,
		CreatedAt:      time.Now(),
	}
	record := ReviewCandidate(input, config)
	if record.Decision != SupervisorRejected {
		t.Errorf("expected REJECTED for too many counterexamples, got %s: %s", record.Decision, record.Reason)
	}
}

func TestReviewCandidate_Rejected_OverBudget(t *testing.T) {
	config := DefaultSupervisorConfig()
	input := SupervisorInput{
		Target:         SupervisorTargetGrowth,
		CharacterID:    "char-004",
		RequestID:      "req-005",
		EvidenceCount:  5,
		Counterexample: 0,
		Authority:      7,
		Sensitive:      false,
		BudgetUsed:     0.9,
		BudgetLimit:    1.0,
		CreatedAt:      time.Now(),
	}
	record := ReviewCandidate(input, config)
	if record.Decision != SupervisorRejected {
		t.Errorf("expected REJECTED for over budget, got %s: %s", record.Decision, record.Reason)
	}
}

func TestReviewCandidate_Escalate_Sensitive(t *testing.T) {
	config := DefaultSupervisorConfig()
	input := SupervisorInput{
		Target:         SupervisorTargetPersonality,
		CharacterID:    "char-005",
		RequestID:      "req-006",
		EvidenceCount:  10,
		Counterexample: 0,
		Authority:      8,
		Sensitive:      true,
		BudgetUsed:     0.2,
		BudgetLimit:    1.0,
		CreatedAt:      time.Now(),
	}
	record := ReviewCandidate(input, config)
	if record.Decision != SupervisorEscalate {
		t.Errorf("expected ESCALATE for sensitive, got %s: %s", record.Decision, record.Reason)
	}
}

func TestCreateRollbackEvent(t *testing.T) {
	config := DefaultSupervisorConfig()
	original := ReviewCandidate(SupervisorInput{
		Target:         SupervisorTargetSummary,
		CharacterID:    "char-001",
		RequestID:      "req-rollback",
		EvidenceCount:  5,
		Counterexample: 0,
		Authority:      7,
		Sensitive:      false,
		BudgetUsed:     0.3,
		BudgetLimit:    1.0,
		CreatedAt:      time.Now(),
	}, config)

	target := RollbackTarget{
		OriginalRecordID: original.ID,
		CharacterID:      "char-001",
		Target:           SupervisorTargetSummary,
		Reason:           "manual rollback due to incorrect data",
		RequestID:        "req-rollback-exec",
	}
	rollback := CreateRollbackEvent(target, original)

	if rollback.Decision != SupervisorRolledBack {
		t.Errorf("expected ROLLED_BACK, got %s", rollback.Decision)
	}
	if len(rollback.PreviousRuns) != 1 || rollback.PreviousRuns[0] != original.ID {
		t.Errorf("expected previous run to contain original ID %s", original.ID)
	}
}

func TestIsActive(t *testing.T) {
	now := time.Now().UTC()
	config := DefaultSupervisorConfig()

	approvedRecord := ReviewCandidate(SupervisorInput{
		Target:         SupervisorTargetReflection,
		CharacterID:    "char-active",
		RequestID:      "req-active",
		EvidenceCount:  5,
		Counterexample: 0,
		Authority:      7,
		Sensitive:      false,
		BudgetUsed:     0.2,
		BudgetLimit:    1.0,
		CreatedAt:      now,
	}, config)

	if !IsActive(approvedRecord, now) {
		t.Error("expected approved record to be active")
	}

	rollbackRecord := CreateRollbackEvent(RollbackTarget{
		OriginalRecordID: approvedRecord.ID,
		CharacterID:      "char-active",
		Target:           SupervisorTargetReflection,
		Reason:           "test rollback",
		RequestID:        "req-rollback-2",
	}, approvedRecord)

	if IsActive(rollbackRecord, now) {
		t.Error("expected rolled back record to be inactive")
	}
}

func TestInvalidateDerivedRecords(t *testing.T) {
	config := DefaultSupervisorConfig()

	original := ReviewCandidate(SupervisorInput{
		Target:         SupervisorTargetSummary,
		CharacterID:    "char-derived",
		RequestID:      "req-original",
		EvidenceCount:  5,
		Counterexample: 0,
		Authority:      7,
		Sensitive:      false,
		BudgetUsed:     0.2,
		BudgetLimit:    1.0,
		CreatedAt:      time.Now(),
	}, config)

	derived := ReviewCandidate(SupervisorInput{
		Target:         SupervisorTargetSummary,
		CharacterID:    "char-derived",
		RequestID:      "req-derived",
		EvidenceCount:  3,
		Counterexample: 1,
		Authority:      5,
		BudgetUsed:     0.1,
		BudgetLimit:    1.0,
		DerivedFromIDs: []string{original.ID},
		CreatedAt:      time.Now(),
	}, config)

	unrelated := ReviewCandidate(SupervisorInput{
		Target:         SupervisorTargetGrowth,
		CharacterID:    "char-derived",
		RequestID:      "req-unrelated",
		EvidenceCount:  4,
		Counterexample: 0,
		Authority:      6,
		Sensitive:      false,
		BudgetUsed:     0.15,
		BudgetLimit:    1.0,
		CreatedAt:      time.Now(),
	}, config)

	allRecords := []SupervisorDecisionRecord{original, derived, unrelated}

	rollback := CreateRollbackEvent(RollbackTarget{
		OriginalRecordID: original.ID,
		CharacterID:      "char-derived",
		Target:           SupervisorTargetSummary,
		Reason:           "test invalidation",
		RequestID:        "req-invalidate",
	}, original)

	updated := InvalidateDerivedRecords(rollback, allRecords)

	var foundDerived, foundOriginal, foundUnrelated bool
	for _, r := range updated {
		if r.ID == original.ID {
			foundOriginal = true
			continue
		}
		if r.ID == derived.ID {
			foundDerived = true
			if r.Decision != SupervisorSuperseded {
				t.Errorf("expected derived record to be SUPERSEDED, got %s", r.Decision)
			}
			if r.RevertedBy != rollback.ID {
				t.Errorf("expected revertedBy to be rollback ID, got %s", r.RevertedBy)
			}
		}
		if r.ID == unrelated.ID {
			foundUnrelated = true
			if r.Decision == SupervisorSuperseded {
				t.Error("expected unrelated record not to be SUPERSEDED")
			}
		}
	}
	if !foundOriginal || !foundDerived || !foundUnrelated {
		t.Error("expected all three records in updated list")
	}
}

func TestApprovedDecisions(t *testing.T) {
	config := DefaultSupervisorConfig()
	now := time.Now().UTC()

	approved := ReviewCandidate(SupervisorInput{
		Target:         SupervisorTargetSummary,
		CharacterID:    "char-001",
		RequestID:      "req-001",
		EvidenceCount:  5,
		Counterexample: 0,
		Authority:      7,
		Sensitive:      false,
		BudgetUsed:     0.2,
		BudgetLimit:    1.0,
		CreatedAt:      now,
	}, config)

	rejected := ReviewCandidate(SupervisorInput{
		Target:         SupervisorTargetReflection,
		CharacterID:    "char-001",
		RequestID:      "req-002",
		EvidenceCount:  0,
		Counterexample: 0,
		Authority:      5,
		Sensitive:      false,
		BudgetUsed:     0.1,
		BudgetLimit:    1.0,
		CreatedAt:      now,
	}, config)

	records := []SupervisorDecisionRecord{approved, rejected}
	active := ApprovedDecisions(records)
	if len(active) != 1 {
		t.Errorf("expected 1 active record, got %d", len(active))
	}
	if active[0].ID != approved.ID {
		t.Error("expected only approved record to be active")
	}
}

func TestDefaultSupervisorConfig(t *testing.T) {
	config := DefaultSupervisorConfig()
	if config.MinEvidenceCount != 2 {
		t.Errorf("expected MinEvidenceCount 2, got %d", config.MinEvidenceCount)
	}
	if config.MaxCounterexample != 1 {
		t.Errorf("expected MaxCounterexample 1, got %d", config.MaxCounterexample)
	}
	if config.MinAuthority != 3 {
		t.Errorf("expected MinAuthority 3, got %d", config.MinAuthority)
	}
	if config.BudgetThreshold != 0.8 {
		t.Errorf("expected BudgetThreshold 0.8, got %f", config.BudgetThreshold)
	}
	if !config.SensitiveEscalate {
		t.Error("expected SensitiveEscalate true")
	}
}

func TestIsRolledBack(t *testing.T) {
	config := DefaultSupervisorConfig()
	original := ReviewCandidate(SupervisorInput{
		Target:      SupervisorTargetSummary,
		CharacterID: "char-rb",
		RequestID:   "req-rb",
		EvidenceCount: 4,
		Counterexample: 0,
		Authority:  6,
		Sensitive:  false,
		BudgetUsed: 0.2,
		BudgetLimit: 1.0,
		CreatedAt:  time.Now(),
	}, config)
	rollback := CreateRollbackEvent(RollbackTarget{
		OriginalRecordID: original.ID,
		CharacterID:      "char-rb",
		Target:           SupervisorTargetSummary,
		Reason:           "test",
		RequestID:        "req-rb-exec",
	}, original)

	if !IsRolledBack(rollback) {
		t.Error("expected rollback record to be identified as rolled back")
	}
	if IsRolledBack(original) {
		t.Error("expected original record not to be identified as rolled back")
	}
}

func TestAffectedDecisions(t *testing.T) {
	config := DefaultSupervisorConfig()

	original := ReviewCandidate(SupervisorInput{
		Target:         SupervisorTargetSummary,
		CharacterID:    "char-aff",
		RequestID:      "req-aff-1",
		EvidenceCount:  5,
		Counterexample: 0,
		Authority:      7,
		BudgetUsed:     0.2,
		BudgetLimit:    1.0,
		CreatedAt:      time.Now(),
	}, config)

	derived := ReviewCandidate(SupervisorInput{
		Target:         SupervisorTargetSummary,
		CharacterID:    "char-aff",
		RequestID:      "req-aff-2",
		EvidenceCount:  3,
		Counterexample: 0,
		Authority:      5,
		BudgetUsed:     0.1,
		BudgetLimit:    1.0,
		DerivedFromIDs: []string{original.ID},
		CreatedAt:      time.Now(),
	}, config)

	otherChar := ReviewCandidate(SupervisorInput{
		Target:         SupervisorTargetSummary,
		CharacterID:    "char-other",
		RequestID:      "req-aff-3",
		EvidenceCount:  4,
		Counterexample: 0,
		Authority:      6,
		BudgetUsed:     0.15,
		BudgetLimit:    1.0,
		DerivedFromIDs: []string{original.ID},
		CreatedAt:      time.Now(),
	}, config)

	all := []SupervisorDecisionRecord{original, derived, otherChar}
	affected := AffectedDecisions(original, all)

	if len(affected) != 1 {
		t.Errorf("expected 1 affected decision, got %d", len(affected))
	}
	if affected[0].ID != derived.ID {
		t.Error("expected only derived to be affected")
	}
}
