package interaction

import (
	"context"
	"strings"
	"time"
)

func (o *Orchestrator) persistParentTurnCheckpoint(ctx context.Context, record *InteractionRecord, req *ProcessRequest) (*InteractionRecord, error) {
	if record == nil || req == nil || record.RecoveryDescriptor != nil ||
		req.RecoverExistingTurn || req.IsInternal || req.SuppressReplyPersistence ||
		req.Channel != "web" || strings.TrimSpace(req.Message) == "" ||
		req.AudioUrl != "" || req.ImageUrl != "" || req.VideoUrl != "" ||
		req.VoiceMessage || req.ImageContext != "" ||
		req.ReplyToMessageID != nil || req.ExpressionPlan != nil ||
		req.ProactiveTaskInstruction != "" || req.ProactiveTimeContext != "" ||
		req.ProactiveRecentContext != "" || req.ProactiveRelationship != "" ||
		req.ProactiveEmotion != "" || req.ProactiveMemory != "" ||
		req.ForceRegenerate {
		return record, nil
	}
	now := time.Now().UTC()
	parent := &ParentTurnRecoveryRef{
		ThreadID: req.ThreadID, MessageStyle: req.MessageStyle,
		PermissionMode:  req.PermissionMode,
		ModelConfigID:   req.ModelConfigID,
		ReasoningEffort: req.ReasoningEffort,
	}
	if req.ExecContext != nil {
		parent.WorkspaceID = req.ExecContext.WorkspaceID
		if req.ExecContext.RuntimeTarget != nil {
			parent.WorkspaceDeviceID = string(req.ExecContext.RuntimeTarget.DeviceID)
		}
		if name, ok := req.ExecContext.Metadata["workspaceName"].(string); ok {
			parent.WorkspaceName = name
		}
		if kind, ok := req.ExecContext.Metadata["workspaceKind"].(string); ok {
			parent.WorkspaceKind = kind
		}
		if root, ok := req.ExecContext.Metadata["workspaceRootUri"].(string); ok {
			parent.WorkspaceRootURI = root
		}
	}
	if tracker, ok := o.tracker.(*SQLiteInteractionTracker); ok {
		snapshot, snapshotErr := tracker.loadParentTurnConversationSnapshot(ctx, req.SpaceID, req.ConversationID)
		if snapshotErr != nil {
			return nil, snapshotErr
		}
		if snapshot != nil {
			parent.ConversationSnapshot = snapshot
			if parent.ModelConfigID == 0 {
				parent.ModelConfigID = snapshot.ModelConfigID
			}
			if parent.ReasoningEffort == "" {
				parent.ReasoningEffort = snapshot.ReasoningEffort
			}
			if parent.PermissionMode == "" {
				parent.PermissionMode = snapshot.PermissionMode
			}
			if parent.WorkspaceID == "" {
				parent.WorkspaceID = snapshot.WorkspaceID
			}
			if parent.WorkspaceDeviceID == "" {
				parent.WorkspaceDeviceID = snapshot.WorkspaceDeviceID
			}
		}
	}
	desc := &RecoveryDescriptor{
		SchemaVersion: RecoveryDescriptorSchemaVersion,
		Requirement:   RecoveryRequired,
		State:         RecoveryDescriptorRecoveryRequired,
		Revision:      1,
		Interaction: RecoveryInteractionRef{
			InteractionID: record.ID, RequestID: req.RequestID,
			Status: record.Status, StatusVersion: record.StatusVersion,
		},
		Scope: RecoveryScopeRef{
			SpaceID: req.SpaceID, CharacterID: req.CharacterID,
			ConversationID: req.ConversationID, Channel: req.Channel,
			PeerID: req.PeerID, SessionID: req.SessionID,
		},
		ParentTurn: parent,
		CreatedAt:  now, UpdatedAt: now,
	}
	desc.ComputeFingerprint()
	updated, err := o.tracker.UpdateMetadata(ctx, record.ID, InteractionMetadataUpdate{RecoveryDescriptor: desc})
	if err != nil {
		return nil, err
	}
	return updated, nil
}
