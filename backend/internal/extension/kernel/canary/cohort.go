package canary

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

type CohortResolver struct{}

func NewCohortResolver() *CohortResolver {
	return &CohortResolver{}
}

func (r *CohortResolver) StableHash(seed string, cohortKey CohortKeyType, entityID string) uint32 {
	h := sha256.Sum256([]byte(seed + ":" + string(cohortKey) + ":" + entityID))
	return binary.BigEndian.Uint32(h[:4])
}

func (r *CohortResolver) IsInPercentage(hash uint32, percentage int) bool {
	return hash%100 < uint32(percentage)
}

func (r *CohortResolver) ResolveCohort(ctx context.Context, policy *CanaryPolicy, invCtx InvocationContext, oldGen, newGen int64) (int64, RoutingReason, error) {
	if invCtx.IsBackground {
		if policy.CohortKey == CohortKeyManualSet {
			return newGen, RoutingReasonManualSet, nil
		}
		return oldGen, RoutingReasonBackground, nil
	}

	var entityID string
	switch policy.CohortKey {
	case CohortKeyCharacter:
		entityID = invCtx.CharacterID
	case CohortKeyConversation:
		entityID = invCtx.ConversationID
	case CohortKeyInvocation:
		entityID = invCtx.InvocationID
	case CohortKeyManualSet:
		stage := r.currentStage(policy)
		if stage != nil {
			for _, id := range stage.CharacterIDs {
				if id == invCtx.CharacterID {
					return newGen, RoutingReasonStableCohort, nil
				}
			}
		}
		return oldGen, RoutingReasonFallback, nil
	default:
		return oldGen, RoutingReasonFallback, fmt.Errorf("canary: unknown cohort key: %s", policy.CohortKey)
	}

	if entityID == "" {
		return oldGen, RoutingReasonFallback, nil
	}

	stage := r.currentStage(policy)
	if stage == nil {
		return oldGen, RoutingReasonFallback, nil
	}

	if stage.Percentage >= 100 {
		return newGen, RoutingReasonPercentage, nil
	}
	if stage.Percentage <= 0 {
		return oldGen, RoutingReasonFallback, nil
	}

	hash := r.StableHash(policy.StableSeed, policy.CohortKey, entityID)
	if r.IsInPercentage(hash, stage.Percentage) {
		return newGen, RoutingReasonStableCohort, nil
	}
	return oldGen, RoutingReasonFallback, nil
}

func (r *CohortResolver) currentStage(policy *CanaryPolicy) *CanaryStage {
	for i := range policy.Stages {
		if string(policy.Stages[i].Mode) == string(policy.Mode) {
			return &policy.Stages[i]
		}
	}
	if len(policy.Stages) > 0 {
		return &policy.Stages[0]
	}
	return nil
}
