package workbenchtasks

import (
	"context"
	"sort"
	"time"

	"github.com/u-ai/backend/internal/interaction"
)

type BoundedRecordReader interface {
	ListRecentWorkbenchTaskRecords(context.Context, interaction.InteractionScope, int) ([]*interaction.InteractionRecord, error)
}

type SnapshotReader interface {
	Snapshot(context.Context, string) (interaction.CoordinationSnapshot, error)
}

type RecordReader interface {
	ListByScope(context.Context, interaction.InteractionScope) ([]*interaction.InteractionRecord, error)
}

func List(ctx context.Context, reader RecordReader, spaceID, conversationID string, live ...SnapshotReader) (TaskList, error) {
	out := TaskList{ProtocolVersion: 1, ConversationID: conversationID, Source: "durable-checkpoint", Tasks: []Task{}}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	scope := interaction.InteractionScope{SpaceID: spaceID, ConversationID: conversationID}
	var records []*interaction.InteractionRecord
	var err error
	if bounded, ok := reader.(BoundedRecordReader); ok {
		records, err = bounded.ListRecentWorkbenchTaskRecords(ctx, scope, 121)
		if len(records) >= 121 {
			out.Truncated = true
		}
	} else {
		records, err = reader.ListByScope(ctx, scope)
	}
	if err != nil {
		return out, err
	}
	sort.SliceStable(records, func(i, j int) bool {
		if records[i] == nil {
			return false
		}
		if records[j] == nil {
			return true
		}
		return records[i].UpdatedAt.After(records[j].UpdatedAt)
	})
	seen := map[string]bool{}
	liveReads := 0
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		if record == nil || record.UpdatedAt.After(time.Now().Add(10*time.Minute)) || record.Scope.SpaceID != spaceID || record.Scope.ConversationID != conversationID || record.Scope.Source == "multi_agent" || record.RecoveryDescriptor == nil || record.RecoveryDescriptor.MultiAgent == nil {
			continue
		}
		descriptor := record.RecoveryDescriptor
		checkpoint := descriptor.MultiAgent
		if descriptor.Fingerprint != "" {
			copyOfDescriptor := *descriptor
			copyOfDescriptor.ComputeFingerprint()
			if copyOfDescriptor.Fingerprint != descriptor.Fingerprint {
				continue
			}
		}
		if !validateCheckpoint(checkpoint) ||
			(checkpoint.ParentGoalSpaceID != "" && checkpoint.ParentGoalSpaceID != spaceID) ||
			(checkpoint.ParentGoalConversationID != "" && checkpoint.ParentGoalConversationID != conversationID) ||
			seen[checkpoint.CoordinationID] {
			continue
		}
		if len(out.Tasks) >= 30 {
			out.Truncated = true
			break
		}
		seen[checkpoint.CoordinationID] = true
		task := Task{CoordinationID: checkpoint.CoordinationID, Status: checkpoint.Status, Strategy: string(checkpoint.Strategy), UpdatedAt: record.UpdatedAt, RecoveryState: "checkpoint-only", AssignmentCount: len(checkpoint.AssignmentRefs), Assignments: make([]Assignment, 0, len(checkpoint.AssignmentRefs))}
		markReconciliation(&task, reconciliationReason(record, checkpoint.Status))
		for _, child := range checkpoint.AssignmentRefs {
			if child.Status == "succeeded" || child.Status == "failed" || child.Status == "cancelled" {
				task.CompletedCount++
			}
			if child.Status == "failed" || child.Status == "cancelled" {
				task.FailedCount++
			}
			task.Assignments = append(task.Assignments, Assignment{ID: child.AssignmentID, Status: child.Status, Role: child.Role, Objective: trimObjective(child.Objective), ReviewRequired: child.Role == "coding_worker"})
		}
		if len(live) > 0 && live[0] != nil && ctx.Err() == nil && task.RecoveryState != "needs-reconciliation" {
			if liveReads >= 8 {
				out.LiveObservationTruncated = true
				out.Tasks = append(out.Tasks, task)
				continue
			}
			liveReads++
			snapshot, snapErr := live[0].Snapshot(ctx, checkpoint.CoordinationID)
			if snapErr == nil && snapshot.ParentInteractionID == record.ID && snapshot.CoordinationID == checkpoint.CoordinationID && task.RecoveryState != "needs-reconciliation" {
				if !validSnapshotTransition(checkpoint.Status, string(snapshot.Status), record.IsTerminal()) || !applySnapshot(&task, snapshot, record.ID) {
					markReconciliation(&task, "runtime-checkpoint-conflict")
				}
			}
		}
		out.Tasks = append(out.Tasks, task)
	}
	out.Summary = summarizeTasks(out.Tasks)
	return out, nil
}

func validateTerminalAlignment(record *interaction.InteractionRecord, status string) bool {
	if record == nil {
		return false
	}
	if record.IsTerminal() && status != "succeeded" && status != "failed" && status != "cancelled" {
		return false
	}
	return true
}

func applySnapshot(task *Task, snapshot interaction.CoordinationSnapshot, parentID string) bool {
	if task == nil || task.RecoveryState == "needs-reconciliation" || snapshot.ParentInteractionID != parentID || snapshot.CoordinationID != task.CoordinationID || !validateState(string(snapshot.Status)) || len(snapshot.Assignments) != task.AssignmentCount {
		return false
	}
	updates := make(map[string]interaction.CoordinationAssignmentSnapshot, len(snapshot.Assignments))
	for _, a := range snapshot.Assignments {
		if a.AssignmentID == "" || !validateAssignment(string(a.Status)) {
			return false
		}
		if _, exists := updates[a.AssignmentID]; exists {
			return false
		}
		updates[a.AssignmentID] = a
	}
	for _, child := range task.Assignments {
		if observed, found := updates[child.ID]; !found || (observed.WorkerRole != "" && observed.WorkerRole != child.Role) {
			return false
		}
	}
	task.Status = string(snapshot.Status)
	task.CompletedCount = 0
	task.FailedCount = 0
	for i := range task.Assignments {
		child := &task.Assignments[i]
		child.Status = string(updates[child.ID].Status)
		if child.Status == "succeeded" || child.Status == "failed" || child.Status == "cancelled" {
			task.CompletedCount++
		}
		if child.Status == "failed" || child.Status == "cancelled" {
			task.FailedCount++
		}
	}
	task.RecoveryState = "runtime-observed"
	return true
}
