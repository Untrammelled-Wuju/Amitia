package control

import (
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type OutputDecisionReason string

const (
	OutputDeniedInvalidPeer       OutputDecisionReason = "invalid_peer"
	OutputDeniedRuntimeNotFound   OutputDecisionReason = "runtime_not_found"
	OutputDeniedServiceNotFound   OutputDecisionReason = "service_not_found"
	OutputDeniedNotEligible       OutputDecisionReason = "runtime_not_eligible"
	OutputDeniedNotReady          OutputDecisionReason = "not_ready"
	OutputDeniedPermission        OutputDecisionReason = "permission_denied"
	OutputDeniedAuthorityMode     OutputDecisionReason = "authority_mode_denied"
	OutputDeniedStaleEpoch        OutputDecisionReason = "stale_epoch"
	OutputDeniedStaleGeneration   OutputDecisionReason = "stale_generation"
	OutputDeniedHostPolicy        OutputDecisionReason = "host_policy_denied"
	OutputDeniedGateClosed        OutputDecisionReason = "gate_closed"
)

type OutputDecision struct {
	Allowed       bool
	Reason        OutputDecisionReason
	CurrentEpoch  uint64
	EvaluatedMode domain.ControlMode
	EvaluatedAt   time.Time
}

func (d OutputDecision) Deny() bool {
	return !d.Allowed
}

func (d OutputDecision) String() string {
	if d.Allowed {
		return fmt.Sprintf("[ALLOW] mode=%s epoch=%d", d.EvaluatedMode, d.CurrentEpoch)
	}
	return fmt.Sprintf("[DENY:%s] mode=%s epoch=%d", d.Reason, d.EvaluatedMode, d.CurrentEpoch)
}

func OutputAllowed(epoch uint64, mode domain.ControlMode, at time.Time) OutputDecision {
	return OutputDecision{
		Allowed:       true,
		CurrentEpoch:  epoch,
		EvaluatedMode: mode,
		EvaluatedAt:   at,
	}
}

func OutputDenied(reason OutputDecisionReason, epoch uint64, mode domain.ControlMode, at time.Time) OutputDecision {
	return OutputDecision{
		Allowed:       false,
		Reason:        reason,
		CurrentEpoch:  epoch,
		EvaluatedMode: mode,
		EvaluatedAt:   at,
	}
}
