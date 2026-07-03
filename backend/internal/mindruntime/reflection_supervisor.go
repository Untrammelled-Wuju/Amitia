package mindruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type ReflectionApprovalConfig struct {
	MinEvidenceForApproval   int
	MinConfidenceForApproval float64
	MaxBeliefAdjustPerCycle  int
	MaxAbstractionsPerCycle  int
	RequireManualReview      bool
	AutoApproveThreshold     float64
}

func DefaultReflectionApprovalConfig() ReflectionApprovalConfig {
	return ReflectionApprovalConfig{
		MinEvidenceForApproval:   3,
		MinConfidenceForApproval: 0.5,
		MaxBeliefAdjustPerCycle:  5,
		MaxAbstractionsPerCycle:  10,
		RequireManualReview:      false,
		AutoApproveThreshold:     0.8,
	}
}

type ReflectionSupervisor struct {
	Supervisor         *VersionRollbackEngine
	HistoryBellief     VersionHistory
	HistoryNarrative   VersionHistory
	HistoryAbstraction VersionHistory
	HistoryGrowth      VersionHistory
	Config             ReflectionApprovalConfig
	SupervisorConfig   SupervisorConfig
}

func NewReflectionSupervisor(characterID string, approvalConfig ReflectionApprovalConfig, supervisorConfig SupervisorConfig) ReflectionSupervisor {
	return ReflectionSupervisor{
		Supervisor:         NewVersionRollbackEngine(DefaultRollbackEngineConfig()),
		HistoryBellief:     NewVersionHistory(characterID, SupervisorTargetReflection),
		HistoryNarrative:   NewVersionHistory(characterID, SupervisorTargetReflection),
		HistoryAbstraction: NewVersionHistory(characterID, SupervisorTargetReflection),
		HistoryGrowth:      NewVersionHistory(characterID, SupervisorTargetGrowth),
		Config:             approvalConfig,
		SupervisorConfig:   supervisorConfig,
	}
}

type ReflectionApprovalResult struct {
	ID              string
	Approved        bool
	Escalated       bool
	RejectedReasons []string
	DecisionRecord  SupervisorDecisionRecord
	VersionRecord   VersionRecord
	CreatedAt       time.Time
}

func (rs *ReflectionSupervisor) ApproveReflectionCandidate(candidate ReflectionCandidate, evidenceCount int) ReflectionApprovalResult {
	now := time.Now().UTC()
	reasons := make([]string, 0)
	approved := true

	if evidenceCount < rs.Config.MinEvidenceForApproval {
		reasons = append(reasons, fmt.Sprintf("证据数 %d 不足，至少需要 %d", evidenceCount, rs.Config.MinEvidenceForApproval))
		approved = false
	}
	if candidate.Confidence < rs.Config.MinConfidenceForApproval {
		reasons = append(reasons, fmt.Sprintf("置信度 %.2f 不足，至少需要 %.2f", candidate.Confidence, rs.Config.MinConfidenceForApproval))
		approved = false
	}
	if len(candidate.BeliefAdjustments) > rs.Config.MaxBeliefAdjustPerCycle {
		reasons = append(reasons, fmt.Sprintf("信念调整数 %d 超出限制 %d", len(candidate.BeliefAdjustments), rs.Config.MaxBeliefAdjustPerCycle))
		approved = false
	}
	if len(candidate.MemoryAbstractions) > rs.Config.MaxAbstractionsPerCycle {
		reasons = append(reasons, fmt.Sprintf("抽象数量 %d 超出限制 %d", len(candidate.MemoryAbstractions), rs.Config.MaxAbstractionsPerCycle))
		approved = false
	}

	escalated := false
	if rs.Config.RequireManualReview && candidate.Confidence < rs.Config.AutoApproveThreshold {
		escalated = true
	}

	svInput := SupervisorInput{
		Target:        SupervisorTargetReflection,
		CharacterID:   strings.TrimSpace(candidate.CharacterID),
		RequestID:     candidate.ID,
		EvidenceCount: evidenceCount,
		CreatedAt:     now,
	}
	decisionRecord := ReviewCandidate(svInput, rs.SupervisorConfig)

	if decisionRecord.Decision == SupervisorRejected {
		approved = false
		reasons = append(reasons, fmt.Sprintf("监督者驳回: %s", decisionRecord.Reason))
	}
	if decisionRecord.Decision == SupervisorEscalate {
		escalated = true
	}

	vr := VersionRecord{
		SnapshotID: candidate.ID,
		DecisionID: decisionRecord.ID,
		CreatedAt:  now,
	}
	rs.HistoryBellief = rs.HistoryBellief.Push(vr)

	id := reflectionApprovalID(candidate.ID, approved)
	return ReflectionApprovalResult{
		ID:              id,
		Approved:        approved,
		Escalated:       escalated,
		RejectedReasons: reasons,
		DecisionRecord:  decisionRecord,
		VersionRecord:   vr,
		CreatedAt:       now,
	}
}

func (rs *ReflectionSupervisor) ApproveGrowth(growthDeltas []ParameterDelta, messageCount int, config PersonalityGrowthConfig) ReflectionApprovalResult {
	now := time.Now().UTC()
	reasons := make([]string, 0)
	approved := true

	if !config.Enabled {
		reasons = append(reasons, "人格成长已关闭")
		approved = false
	}
	if len(growthDeltas) == 0 {
		reasons = append(reasons, "无增长数据")
		approved = false
	}

	svInput := SupervisorInput{
		Target:        SupervisorTargetGrowth,
		CharacterID:   "personality-system",
		RequestID:     "growth-cycle",
		EvidenceCount: messageCount,
		CreatedAt:     now,
	}
	decisionRecord := ReviewCandidate(svInput, rs.SupervisorConfig)

	if decisionRecord.Decision == SupervisorRejected {
		approved = false
		reasons = append(reasons, fmt.Sprintf("监督者驳回: %s", decisionRecord.Reason))
	}
	if decisionRecord.Decision == SupervisorEscalate {
		approved = false
		reasons = append(reasons, "需要人工审批")
	}

	vr := VersionRecord{
		SnapshotID: "growth-snapshot",
		DecisionID: decisionRecord.ID,
		CreatedAt:  now,
	}
	rs.HistoryGrowth = rs.HistoryGrowth.Push(vr)

	id := reflectionApprovalID("growth", approved)
	return ReflectionApprovalResult{
		ID:              id,
		Approved:        approved,
		Escalated:       decisionRecord.Decision == SupervisorEscalate,
		RejectedReasons: reasons,
		DecisionRecord:  decisionRecord,
		VersionRecord:   vr,
		CreatedAt:       now,
	}
}

func (rs *ReflectionSupervisor) RollbackReflection(targetVersion int, reason, requestID string) (RollbackPlan, error) {
	if rs.HistoryBellief.CurrentVersion == 0 {
		return RollbackPlan{}, fmt.Errorf("无版本历史")
	}
	plan := rs.Supervisor.PlanRollback(rs.HistoryBellief, targetVersion, reason, nil)
	if plan.ID == "" {
		return RollbackPlan{}, fmt.Errorf("无法生成回滚计划")
	}
	return plan, nil
}

func (rs *ReflectionSupervisor) RollbackGrowth(targetVersion int, reason, requestID string) (RollbackPlan, error) {
	if rs.HistoryGrowth.CurrentVersion == 0 {
		return RollbackPlan{}, fmt.Errorf("无版本历史")
	}
	plan := rs.Supervisor.PlanRollback(rs.HistoryGrowth, targetVersion, reason, nil)
	if plan.ID == "" {
		return RollbackPlan{}, fmt.Errorf("无法生成回滚计划")
	}
	return plan, nil
}

func (rs *ReflectionSupervisor) GetReflectionHistory() []VersionRecord {
	return rs.HistoryBellief.Records
}

func (rs *ReflectionSupervisor) GetGrowthHistory() []VersionRecord {
	return rs.HistoryGrowth.Records
}

func (rs *ReflectionSupervisor) GetActiveReflectionVersions() []VersionRecord {
	return rs.HistoryBellief.ActiveVersions(time.Now().UTC())
}

func (rs *ReflectionSupervisor) GetActiveGrowthVersions() []VersionRecord {
	return rs.HistoryGrowth.ActiveVersions(time.Now().UTC())
}

func reflectionApprovalID(candidateID string, approved bool) string {
	status := "rejected"
	if approved {
		status = "approved"
	}
	raw := fmt.Sprintf("ref-approval|%s|%s|%d", candidateID, status, time.Now().UnixNano())
	sum := sha256.Sum256([]byte(raw))
	return "ref-approval-" + hex.EncodeToString(sum[:])[:16]
}
