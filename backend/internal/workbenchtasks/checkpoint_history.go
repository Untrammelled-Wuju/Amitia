package workbenchtasks

import (
	"context"
	"sort"
	"time"

	"github.com/u-ai/backend/internal/interaction"
)

type TaskCheckpoint struct {
	UpdatedAt          time.Time `json:"updatedAt"`
	InteractionStatus  string    `json:"interactionStatus"`
	CoordinationStatus string    `json:"coordinationStatus"`
	AssignmentCount    int       `json:"assignmentCount"`
	CompletedCount     int       `json:"completedCount"`
	FailedCount        int       `json:"failedCount"`
	ParentTerminal     bool      `json:"parentTerminal"`
	Consistent         bool      `json:"consistent"`
}

type TaskCheckpointHistory struct {
	QueryContext    *QueryContext    `json:"queryContext,omitempty"`
	ProtocolVersion int              `json:"protocolVersion"`
	ConversationID  string           `json:"conversationId"`
	CoordinationID  string           `json:"coordinationId"`
	Source          string           `json:"source"`
	Records         []TaskCheckpoint `json:"records"`
	Truncated       bool             `json:"truncated"`
}

func ListCheckpoints(ctx context.Context, reader RecordReader, spaceID, conversationID, coordinationID string) (TaskCheckpointHistory, error) {
	out := TaskCheckpointHistory{ProtocolVersion: 1, ConversationID: conversationID, CoordinationID: coordinationID, Source: "sampled-durable-checkpoints", Records: []TaskCheckpoint{}}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	scope := interaction.InteractionScope{SpaceID: spaceID, ConversationID: conversationID}
	var records []*interaction.InteractionRecord
	var err error
	if bounded, ok := reader.(BoundedRecordReader); ok {
		records, err = bounded.ListRecentWorkbenchTaskRecords(ctx, scope, 121)
		out.Truncated = len(records) >= 121
	} else {
		records, err = reader.ListByScope(ctx, scope)
		out.Truncated = len(records) > 121
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
		if records[i].UpdatedAt.Equal(records[j].UpdatedAt) {
			return records[i].ID > records[j].ID
		}
		return records[i].UpdatedAt.After(records[j].UpdatedAt)
	})
	seen := map[string]struct{}{}
	terminalProofReads := 0
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		if record == nil || record.ID == "" || record.Scope.SpaceID != spaceID || record.Scope.ConversationID != conversationID || record.Scope.Source == "multi_agent" || record.RecoveryDescriptor == nil || record.RecoveryDescriptor.MultiAgent == nil || record.UpdatedAt.After(time.Now().Add(10*time.Minute)) {
			continue
		}
		descriptor := record.RecoveryDescriptor
		checkpoint := descriptor.MultiAgent
		if checkpoint.CoordinationID != coordinationID || !validateCheckpoint(checkpoint) || (checkpoint.ParentGoalSpaceID != "" && checkpoint.ParentGoalSpaceID != spaceID) || (checkpoint.ParentGoalConversationID != "" && checkpoint.ParentGoalConversationID != conversationID) || (checkpoint.ParentGoalCharacterID != "" && checkpoint.ParentGoalCharacterID != record.Scope.CharacterID) || (checkpoint.OwnedScope != nil && (checkpoint.OwnedScope.SpaceID != spaceID || checkpoint.OwnedScope.RoleID != record.Scope.CharacterID)) {
			continue
		}
		if descriptor.Fingerprint != "" {
			copyOfDescriptor := *descriptor
			copyOfDescriptor.ComputeFingerprint()
			if copyOfDescriptor.Fingerprint != descriptor.Fingerprint {
				continue
			}
		}
		if _, exists := seen[record.ID]; exists {
			continue
		}
		seen[record.ID] = struct{}{}
		point := TaskCheckpoint{UpdatedAt: record.UpdatedAt, InteractionStatus: string(record.Status), CoordinationStatus: checkpoint.Status, AssignmentCount: len(checkpoint.AssignmentRefs), ParentTerminal: record.IsTerminal(), Consistent: reconciliationReason(record, checkpoint) == ""}
		projection := Task{Status: checkpoint.Status, RecoveryState: "checkpoint-only", Assignments: make([]Assignment, 0, len(checkpoint.AssignmentRefs))}
		for _, child := range checkpoint.AssignmentRefs {
			projection.Assignments = append(projection.Assignments, Assignment{ID: child.AssignmentID, Status: child.Status})
		}
		if err := confirmTerminalWorkbenchChildren(ctx, reader, checkpoint, &projection, &terminalProofReads); err != nil {
			return out, err
		}
		for _, child := range projection.Assignments {
			if child.Status == "succeeded" || child.Status == "failed" || child.Status == "cancelled" {
				point.CompletedCount++
			}
			if child.Status == "failed" || child.Status == "cancelled" {
				point.FailedCount++
			}
		}
		if projection.RecoveryState == "needs-reconciliation" {
			point.Consistent = false
		}
		if len(out.Records) == 20 {
			out.Truncated = true
			break
		}
		out.Records = append(out.Records, point)
	}
	return out, nil
}
