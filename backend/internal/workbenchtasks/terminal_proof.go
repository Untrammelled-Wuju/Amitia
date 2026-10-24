package workbenchtasks

import (
	"context"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/interaction"
)

type terminalChildReader interface {
	Get(context.Context, string) (*interaction.InteractionRecord, bool, error)
}

const maxTerminalProofReads = 48

func confirmTerminalWorkbenchChildren(ctx context.Context, reader RecordReader, checkpoint *interaction.MultiAgentRecoveryRef, task *Task, reads *int) error {
	if checkpoint == nil || checkpoint.OwnedScope == nil || task == nil {
		return nil
	}
	source, readable := reader.(terminalChildReader)
	byID := make(map[string]interaction.AssignmentRecoveryRef, len(checkpoint.AssignmentRefs))
	for _, item := range checkpoint.AssignmentRefs {
		byID[item.AssignmentID] = item
	}
	for i := range task.Assignments {
		assignment := &task.Assignments[i]
		if assignment.Status != "succeeded" && assignment.Status != "failed" && assignment.Status != "cancelled" {
			continue
		}
		evidence, exists := byID[assignment.ID]
		if !exists {
			downgradeUnverifiedAssignment(task, i, "terminal-assignment-missing")
			continue
		}
		if evidence.ChildInteractionID == "" {
			allowedFailure := assignment.Status == "failed" && strings.HasPrefix(evidence.Error, "dependency_failed:")
			allowedCancellation := assignment.Status == "cancelled" && checkpoint.CancelRequested && evidence.Error == "cancelled before worker dispatch"
			if !allowedFailure && !allowedCancellation {
				downgradeUnverifiedAssignment(task, i, "terminal-child-evidence-missing")
			}
			continue
		}
		if !readable {
			downgradeUnverifiedAssignment(task, i, "terminal-child-reader-unavailable")
			continue
		}
		if *reads >= maxTerminalProofReads {
			downgradeUnverifiedAssignment(task, i, "terminal-child-read-budget-exceeded")
			continue
		}
		*reads = *reads + 1
		child, found, err := source.Get(ctx, evidence.ChildInteractionID)
		if err != nil {
			return err
		}
		if !found || child == nil || !workbenchChildProofMatches(child, checkpoint, evidence) {
			downgradeUnverifiedAssignment(task, i, "terminal-child-evidence-conflict")
			continue
		}
		if assignment.Status == "succeeded" && !workbenchChildWithinDeadline(child, checkpoint.DeadlineAt) {
			downgradeUnverifiedAssignment(task, i, "terminal-child-deadline-unverified")
		}
	}
	if task.RecoveryState == "needs-reconciliation" {
		task.CompletedCount, task.FailedCount = 0, 0
		for _, child := range task.Assignments {
			if child.Status == "succeeded" || child.Status == "failed" || child.Status == "cancelled" {
				task.CompletedCount++
			}
			if child.Status == "failed" || child.Status == "cancelled" {
				task.FailedCount++
			}
		}
		if task.Status == "succeeded" || task.Status == "failed" || task.Status == "cancelled" {
			task.Status = "waiting"
		}
	}
	return nil
}

func downgradeUnverifiedAssignment(task *Task, index int, reason string) {
	markReconciliation(task, reason)
	task.Assignments[index].Status = "waiting"
}

func workbenchChildProofMatches(child *interaction.InteractionRecord, checkpoint *interaction.MultiAgentRecoveryRef, assignment interaction.AssignmentRecoveryRef) bool {
	if child.ID != assignment.ChildInteractionID || child.Scope.SpaceID != checkpoint.OwnedScope.SpaceID ||
		child.Scope.CharacterID != assignment.CharacterID || child.Scope.CharacterID != checkpoint.OwnedScope.RoleID ||
		child.Scope.ConversationID != "multi-agent-worker-"+assignment.AssignmentID ||
		child.Scope.RequestID != "ma-"+assignment.AssignmentID || child.Scope.Source != "multi_agent" {
		return false
	}
	switch assignment.Status {
	case "succeeded":
		return child.Status == interaction.InteractionStatusCompleted
	case "failed":
		return child.Status == interaction.InteractionStatusFailed
	case "cancelled":
		return child.Status == interaction.InteractionStatusCancelled || child.Status == interaction.InteractionStatusSuperseded
	}
	return false
}

func workbenchChildWithinDeadline(child *interaction.InteractionRecord, deadline *time.Time) bool {
	if deadline == nil {
		return true
	}
	return child != nil && !child.CompletedAt.IsZero() && !child.CompletedAt.After(*deadline) &&
		(child.CreatedAt.IsZero() || !child.CompletedAt.Before(child.CreatedAt))
}
