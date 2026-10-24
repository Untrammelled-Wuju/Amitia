package main

import (
	"context"
	"errors"

	"github.com/u-ai/backend/internal/interaction"
)

type ownedWorkerBudgetSnapshot struct {
	fingerprint string
	toolLimit   int
	tokenLimit  int64
	costLimit   int64
}

func snapshotOwnedWorkerBudget(ref *interaction.MultiAgentRecoveryRef) ownedWorkerBudgetSnapshot {
	if ref == nil {
		return ownedWorkerBudgetSnapshot{}
	}
	result := ownedWorkerBudgetSnapshot{fingerprint: ref.DispatchFingerprint}
	if ref.MaxToolCalls != nil {
		result.toolLimit = *ref.MaxToolCalls
	}
	if ref.MaxTotalTokens != nil {
		result.tokenLimit = *ref.MaxTotalTokens
	}
	if ref.MaxCostMicroUSD != nil {
		result.costLimit = *ref.MaxCostMicroUSD
	}
	return result
}

func (r *ownedMultiAgentWorkerRunner) workerBudgetAdmissionGuard(childID string, req interaction.WorkerRunRequest, original *interaction.MultiAgentRecoveryRef) func(context.Context) error {
	expected := snapshotOwnedWorkerBudget(original)
	return func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		stop, err := r.workerCancellationRequested(ctx, childID, req)
		if err != nil {
			return err
		}
		if stop {
			return errors.New("multi_agent: worker is no longer authorized for budget admission")
		}
		parent, err := r.verifyWorkerParent(ctx, req)
		if err != nil {
			return err
		}
		if parent == nil || parent.RecoveryDescriptor == nil || snapshotOwnedWorkerBudget(parent.RecoveryDescriptor.MultiAgent) != expected {
			return errors.New("multi_agent: parent delegation budget or dispatch revision changed")
		}
		return nil
	}
}
