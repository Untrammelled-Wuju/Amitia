package mindruntime

import (
	"testing"
	"time"
)

func TestNewReflectionSupervisor(t *testing.T) {
	approvalConfig := DefaultReflectionApprovalConfig()
	supervisorConfig := DefaultSupervisorConfig()
	rs := NewReflectionSupervisor("char-1", approvalConfig, supervisorConfig)
	if rs.Supervisor == nil {
		t.Error("expected non-nil supervisor")
	}
	if rs.HistoryBellief.CharacterID != "char-1" {
		t.Errorf("expected char-1, got %s", rs.HistoryBellief.CharacterID)
	}
	if rs.HistoryGrowth.CharacterID != "char-1" {
		t.Errorf("expected char-1, got %s", rs.HistoryGrowth.CharacterID)
	}
	if rs.Config.MinEvidenceForApproval != 3 {
		t.Errorf("expected 3, got %d", rs.Config.MinEvidenceForApproval)
	}
}

func TestApproveReflectionCandidate_Approved(t *testing.T) {
	approvalConfig := DefaultReflectionApprovalConfig()
	supervisorConfig := DefaultSupervisorConfig()
	supervisorConfig.MinEvidenceCount = 2
	supervisorConfig.MinAuthority = 0
	rs := NewReflectionSupervisor("char-1", approvalConfig, supervisorConfig)

	now := time.Now()
	candidate := ReflectionCandidate{
		ID: "ref-001", CharacterID: "char-1",
		Confidence: 0.9,
		BeliefAdjustments: []BeliefAdjustment{
			{BeliefKey: "belief/music", OldStrength: 0, NewStrength: 0.5},
		},
		MemoryAbstractions: []MemoryAbstraction{
			{Topic: "音乐", SourceIDs: []string{"mem-1", "mem-2", "mem-3"}},
		},
		CreatedAt: now,
	}
	result := rs.ApproveReflectionCandidate(candidate, 5)
	if !result.Approved {
		t.Errorf("expected approved, got rejected: %v", result.RejectedReasons)
	}
	if result.Escalated {
		t.Error("expected not escalated")
	}
	if len(rs.HistoryBellief.Records) != 1 {
		t.Errorf("expected 1 history record, got %d", len(rs.HistoryBellief.Records))
	}
}

func TestApproveReflectionCandidate_InsufficientEvidence(t *testing.T) {
	approvalConfig := DefaultReflectionApprovalConfig()
	approvalConfig.MinEvidenceForApproval = 5
	supervisorConfig := DefaultSupervisorConfig()
	rs := NewReflectionSupervisor("char-1", approvalConfig, supervisorConfig)

	candidate := ReflectionCandidate{
		ID: "ref-002", CharacterID: "char-1",
		Confidence: 0.9,
		BeliefAdjustments: []BeliefAdjustment{
			{BeliefKey: "belief/test", OldStrength: 0, NewStrength: 0.5},
		},
	}
	result := rs.ApproveReflectionCandidate(candidate, 2)
	if result.Approved {
		t.Error("expected rejected due to insufficient evidence")
	}
}

func TestApproveReflectionCandidate_LowConfidence(t *testing.T) {
	approvalConfig := DefaultReflectionApprovalConfig()
	approvalConfig.MinConfidenceForApproval = 0.7
	supervisorConfig := DefaultSupervisorConfig()
	rs := NewReflectionSupervisor("char-1", approvalConfig, supervisorConfig)

	candidate := ReflectionCandidate{
		ID: "ref-003", CharacterID: "char-1",
		Confidence: 0.3,
		BeliefAdjustments: []BeliefAdjustment{
			{BeliefKey: "belief/test", OldStrength: 0, NewStrength: 0.5},
		},
	}
	result := rs.ApproveReflectionCandidate(candidate, 5)
	if result.Approved {
		t.Error("expected rejected due to low confidence")
	}
}

func TestApproveReflectionCandidate_TooManyAdjustments(t *testing.T) {
	approvalConfig := DefaultReflectionApprovalConfig()
	approvalConfig.MaxBeliefAdjustPerCycle = 3
	supervisorConfig := DefaultSupervisorConfig()
	rs := NewReflectionSupervisor("char-1", approvalConfig, supervisorConfig)

	adjustments := make([]BeliefAdjustment, 6)
	for i := 0; i < 6; i++ {
		adjustments[i] = BeliefAdjustment{BeliefKey: "key", OldStrength: 0, NewStrength: 0.5}
	}
	candidate := ReflectionCandidate{
		ID: "ref-004", CharacterID: "char-1",
		Confidence:            0.9,
		BeliefAdjustments:     adjustments,
	}
	result := rs.ApproveReflectionCandidate(candidate, 5)
	if result.Approved {
		t.Error("expected rejected due to too many adjustments")
	}
}

func TestApproveReflectionCandidate_TooManyAbstractions(t *testing.T) {
	approvalConfig := DefaultReflectionApprovalConfig()
	approvalConfig.MaxAbstractionsPerCycle = 3
	supervisorConfig := DefaultSupervisorConfig()
	rs := NewReflectionSupervisor("char-1", approvalConfig, supervisorConfig)

	abstractions := make([]MemoryAbstraction, 6)
	for i := 0; i < 6; i++ {
		abstractions[i] = MemoryAbstraction{Topic: "topic", SourceIDs: []string{"mem-1", "mem-2"}}
	}
	candidate := ReflectionCandidate{
		ID: "ref-005", CharacterID: "char-1",
		Confidence:          0.9,
		MemoryAbstractions:  abstractions,
	}
	result := rs.ApproveReflectionCandidate(candidate, 5)
	if result.Approved {
		t.Error("expected rejected due to too many abstractions")
	}
}

func TestApproveReflectionCandidate_ManualReview(t *testing.T) {
	approvalConfig := DefaultReflectionApprovalConfig()
	approvalConfig.RequireManualReview = true
	approvalConfig.AutoApproveThreshold = 0.9
	supervisorConfig := DefaultSupervisorConfig()
	rs := NewReflectionSupervisor("char-1", approvalConfig, supervisorConfig)

	candidate := ReflectionCandidate{
		ID: "ref-006", CharacterID: "char-1",
		Confidence: 0.6,
		BeliefAdjustments: []BeliefAdjustment{
			{BeliefKey: "belief/test", OldStrength: 0, NewStrength: 0.5},
		},
	}
	result := rs.ApproveReflectionCandidate(candidate, 5)
	if !result.Escalated {
		t.Error("expected escalated for manual review")
	}
}

func TestApproveGrowth_Enabled(t *testing.T) {
	approvalConfig := DefaultReflectionApprovalConfig()
	supervisorConfig := DefaultSupervisorConfig()
	supervisorConfig.MinAuthority = 0
	supervisorConfig.MinEvidenceCount = 2
	rs := NewReflectionSupervisor("char-1", approvalConfig, supervisorConfig)

	growthConfig := DefaultPersonalityGrowthConfig()
	deltas := []ParameterDelta{
		{Name: "expressiveness", Old: 0.5, New: 0.51, Delta: 0.01},
	}
	result := rs.ApproveGrowth(deltas, 200, growthConfig)
	if !result.Approved {
		t.Errorf("expected approved, got: %v", result.RejectedReasons)
	}
	if len(rs.HistoryGrowth.Records) != 1 {
		t.Errorf("expected 1 growth history record, got %d", len(rs.HistoryGrowth.Records))
	}
}

func TestApproveGrowth_Disabled(t *testing.T) {
	approvalConfig := DefaultReflectionApprovalConfig()
	supervisorConfig := DefaultSupervisorConfig()
	rs := NewReflectionSupervisor("char-1", approvalConfig, supervisorConfig)

	growthConfig := DefaultPersonalityGrowthConfig()
	growthConfig.Enabled = false
	result := rs.ApproveGrowth(nil, 200, growthConfig)
	if result.Approved {
		t.Error("expected rejected when growth disabled")
	}
}

func TestApproveGrowth_EmptyDeltas(t *testing.T) {
	approvalConfig := DefaultReflectionApprovalConfig()
	supervisorConfig := DefaultSupervisorConfig()
	rs := NewReflectionSupervisor("char-1", approvalConfig, supervisorConfig)

	growthConfig := DefaultPersonalityGrowthConfig()
	result := rs.ApproveGrowth(nil, 200, growthConfig)
	if result.Approved {
		t.Error("expected rejected for empty deltas")
	}
}

func TestRollbackReflection_EmptyHistory(t *testing.T) {
	approvalConfig := DefaultReflectionApprovalConfig()
	supervisorConfig := DefaultSupervisorConfig()
	rs := NewReflectionSupervisor("char-1", approvalConfig, supervisorConfig)

	_, err := rs.RollbackReflection(1, "test", "req-1")
	if err == nil {
		t.Error("expected error for empty history")
	}
}

func TestRollbackReflection_Valid(t *testing.T) {
	approvalConfig := DefaultReflectionApprovalConfig()
	supervisorConfig := DefaultSupervisorConfig()
	supervisorConfig.MinAuthority = 0
	supervisorConfig.MinEvidenceCount = 1
	rs := NewReflectionSupervisor("char-1", approvalConfig, supervisorConfig)

	candidate := ReflectionCandidate{
		ID: "ref-001", CharacterID: "char-1",
		Confidence: 0.9,
		BeliefAdjustments: []BeliefAdjustment{
			{BeliefKey: "belief/test", OldStrength: 0, NewStrength: 0.5},
		},
	}
	rs.ApproveReflectionCandidate(candidate, 5)

	rs.ApproveReflectionCandidate(candidate, 5)
	rs.ApproveReflectionCandidate(candidate, 5)

	plan, err := rs.RollbackReflection(1, "incorrect data", "req-rollback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.ID == "" {
		t.Error("expected non-empty rollback plan")
	}
	if plan.Target != SupervisorTargetReflection {
		t.Errorf("expected target reflection, got %s", plan.Target)
	}
}

func TestRollbackGrowth_EmptyHistory(t *testing.T) {
	approvalConfig := DefaultReflectionApprovalConfig()
	supervisorConfig := DefaultSupervisorConfig()
	rs := NewReflectionSupervisor("char-1", approvalConfig, supervisorConfig)

	_, err := rs.RollbackGrowth(1, "test", "req-1")
	if err == nil {
		t.Error("expected error for empty growth history")
	}
}

func TestGetReflectionHistory(t *testing.T) {
	approvalConfig := DefaultReflectionApprovalConfig()
	supervisorConfig := DefaultSupervisorConfig()
	rs := NewReflectionSupervisor("char-1", approvalConfig, supervisorConfig)

	history := rs.GetReflectionHistory()
	if len(history) != 0 {
		t.Errorf("expected 0 records, got %d", len(history))
	}
}

func TestGetActiveReflectionVersions(t *testing.T) {
	approvalConfig := DefaultReflectionApprovalConfig()
	supervisorConfig := DefaultSupervisorConfig()
	rs := NewReflectionSupervisor("char-1", approvalConfig, supervisorConfig)

	active := rs.GetActiveReflectionVersions()
	if len(active) != 0 {
		t.Errorf("expected 0 active versions, got %d", len(active))
	}
}

func TestDefaultReflectionApprovalConfig(t *testing.T) {
	config := DefaultReflectionApprovalConfig()
	if config.MinEvidenceForApproval != 3 {
		t.Errorf("expected 3, got %d", config.MinEvidenceForApproval)
	}
	if config.MinConfidenceForApproval != 0.5 {
		t.Errorf("expected 0.5, got %f", config.MinConfidenceForApproval)
	}
	if config.MaxBeliefAdjustPerCycle != 5 {
		t.Errorf("expected 5, got %d", config.MaxBeliefAdjustPerCycle)
	}
	if config.MaxAbstractionsPerCycle != 10 {
		t.Errorf("expected 10, got %d", config.MaxAbstractionsPerCycle)
	}
	if config.RequireManualReview {
		t.Error("expected ManualReview false by default")
	}
	if config.AutoApproveThreshold != 0.8 {
		t.Errorf("expected 0.8, got %f", config.AutoApproveThreshold)
	}
}
