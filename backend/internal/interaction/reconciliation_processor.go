package interaction

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/decision"
)

type agentReconciliationProcessor struct {
	goals        GoalReconciliationReader
	tasks        TaskReconciliationReader
	workflows    WorkflowReconciliationReader
	invocations  InvocationReconciliationReader
	observations AgentObservationReader
	now          func() time.Time
}

func NewAgentReconciliationProcessor(
	goals GoalReconciliationReader,
	obs AgentObservationReader,
	tasks TaskReconciliationReader,
	workflows WorkflowReconciliationReader,
	invocations InvocationReconciliationReader,
) AgentReconciliationProcessor {
	return &agentReconciliationProcessor{
		goals:        goals,
		tasks:        tasks,
		workflows:    workflows,
		invocations:  invocations,
		observations: obs,
		now:          func() time.Time { return time.Now().UTC() },
	}
}

func (p *agentReconciliationProcessor) Capture(ctx context.Context, scope ReconciliationCaptureScope) (*AgentReconciliationSnapshot, error) {
	if p == nil {
		return nil, nil
	}
	snap := &AgentReconciliationSnapshot{
		UserID:         scope.UserID,
		CharacterID:    scope.CharacterID,
		ConversationID: scope.ConversationID,
		InteractionID:  scope.InteractionID,
		Goals:          make([]decision.Goal, 0),
		Observations:   make([]decision.Observation, 0),
		Tasks:          make([]AgentTaskRef, 0),
		Workflows:      make([]AgentWorkflowRef, 0),
		Invocations:    make([]AgentInvocationRef, 0),
		CapturedAt:     p.now(),
	}
	if p.goals != nil && scope.UserID != "" {
		snap.Goals = p.goals.ActiveForScope(ctx, scope.UserID, scope.CharacterID, scope.ConversationID)
	}
	if p.observations != nil && scope.InteractionID != "" {
		snap.Observations = p.observations.ListObservationsByInteraction(ctx, scope.InteractionID)
	}
	if p.tasks != nil {
		if scope.InteractionID != "" {
			snap.Tasks = p.tasks.ListTaskRunsByInteraction(ctx, scope.InteractionID)
		} else {
			for _, id := range scope.TaskRunIDs {
				if run, ok := p.tasks.GetTaskRun(ctx, id); ok {
					snap.Tasks = append(snap.Tasks, run)
				}
			}
		}
	}
	if p.workflows != nil {
		if scope.InteractionID != "" {
			snap.Workflows = p.workflows.ListWorkflowRunsByInteraction(ctx, scope.InteractionID)
		} else {
			for _, id := range scope.ExecutionIDs {
				if run, ok := p.workflows.GetWorkflowRun(ctx, id); ok {
					snap.Workflows = append(snap.Workflows, run)
				}
			}
		}
	}
	if p.invocations != nil {
		if scope.InteractionID != "" {
			snap.Invocations = p.invocations.ListInvocationsByInteraction(ctx, scope.InteractionID)
		} else {
			for _, id := range scope.InvocationIDs {
				if inv, ok := p.invocations.GetInvocation(ctx, id); ok {
					snap.Invocations = append(snap.Invocations, inv)
				}
			}
		}
	}
	return snap, nil
}
