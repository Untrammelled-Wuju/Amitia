package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

func ownedParentKey(inference business.Inference) string {
	encoded, _ := json.Marshal([]string{inference.Scope.CoreID, inference.Scope.ResourceOwnerID, inference.Scope.InitiatorDeviceID, inference.Scope.RoleID, inference.ConversationID, inference.Scope.RequestID})
	digest := sha256.Sum256(encoded)
	return "owned-parent/" + hex.EncodeToString(digest[:])
}

func (r *meshOwnedToolRuntime) nativeScope(ctx context.Context, inference business.Inference) chat.SkillScope {
	wire := ownedToolScope(inference, "")
	requestID := ownedParentKey(inference)
	source := "owned"
	if inference.InternalWorker {
		requestID, source = inference.Scope.RequestID, "multi_agent"
	}
	return chat.SkillScope{SpaceID: inference.Scope.SpaceID, CharacterID: inference.Scope.RoleID, ConversationID: inference.ConversationID, RequestID: requestID, Channel: "device-mesh", Source: source, PermissionMode: wire.PermissionMode, ExecContext: wire.ExecContext, IsInternal: inference.InternalWorker}
}

func (r *meshOwnedToolRuntime) BeginOwnedTurn(ctx context.Context, inference business.Inference) (context.Context, func(context.Context, business.Generation, error) error, error) {
	if r.services == nil || r.services.OwnedBusiness == nil || r.services.WorkbenchTaskTracker == nil {
		return ctx, nil, errors.New("父任务账本尚未就绪")
	}
	if err := r.services.OwnedBusiness.WithExecutionScope(ctx, inference.Scope, func(context.Context) error { return nil }); err != nil {
		return ctx, nil, err
	}
	native := r.nativeScope(ctx, inference)
	encoded, _ := json.Marshal([]any{inference.Message, inference.ProjectID, inference.Workspaces, inference.PrimaryWorkspaceID, native.PermissionMode, inference.Attachments, inference.Quote, inference.Context})
	digest := sha256.Sum256(encoded)
	fingerprint := hex.EncodeToString(digest[:])
	candidate := interaction.NewInteractionRecord(interaction.InteractionScope{SpaceID: native.SpaceID, CharacterID: native.CharacterID, ConversationID: native.ConversationID, RequestID: native.RequestID, Channel: native.Channel, Source: native.Source})
	candidate.RecoveryDescriptor = &interaction.RecoveryDescriptor{
		SchemaVersion: interaction.RecoveryDescriptorSchemaVersion, Requirement: interaction.RecoveryRequired,
		State:       interaction.RecoveryDescriptorActive,
		Interaction: interaction.RecoveryInteractionRef{InteractionID: candidate.ID, RequestID: candidate.Scope.RequestID, Status: candidate.Status},
		Scope:       interaction.RecoveryScopeRef{SpaceID: native.SpaceID, CharacterID: native.CharacterID, ConversationID: native.ConversationID, Channel: native.Channel},
		OwnedTurn:   &interaction.OwnedTurnRecoveryRef{Scope: inference.Scope, RequestID: inference.Scope.RequestID, InputFingerprint: fingerprint},
	}
	candidate.RecoveryDescriptor.ComputeFingerprint()
	r.parentMu.Lock()
	parent, created, err := interaction.RegisterInteraction(ctx, r.services.WorkbenchTaskTracker, candidate)
	if err == nil && !created {
		if parent.RecoveryDescriptor == nil || parent.RecoveryDescriptor.OwnedTurn == nil || parent.RecoveryDescriptor.OwnedTurn.InputFingerprint != fingerprint {
			err = coordination.ErrRequestConflict
		} else {
			err = business.ErrUncertainExecution
		}
	}
	if err == nil {
		parent, err = r.services.WorkbenchTaskTracker.TransitionCAS(ctx, parent.ID, parent.StatusVersion, interaction.InteractionStatusProcessing)
	}
	r.parentMu.Unlock()
	if err != nil {
		return ctx, nil, err
	}
	return parent.Scope.WithContext(ctx), func(current context.Context, generation business.Generation, runErr error) error {
		latest, found, err := r.services.WorkbenchTaskTracker.Get(current, parent.ID)
		if err != nil {
			return err
		}
		if !found || latest == nil {
			return interaction.ErrInteractionNotFound
		}
		if latest.IsTerminal() {
			return nil
		}
		var task *interaction.MultiAgentRecoveryRef
		if latest.RecoveryDescriptor != nil {
			task = latest.RecoveryDescriptor.MultiAgent
		}
		if task != nil {
			if errors.Is(runErr, business.ErrInterrupted) || errors.Is(runErr, coordination.ErrScopeExpired) {
				if r.services.MultiAgentCoordinator == nil {
					return errors.New("任务协调器尚未就绪")
				}
				if err := r.services.MultiAgentCoordinator.Cancel(current, task.CoordinationID); err != nil {
					return err
				}
				for attempt := 0; attempt < 4; attempt++ {
					latest, found, err = r.services.WorkbenchTaskTracker.Get(current, parent.ID)
					if err != nil {
						return err
					}
					if !found || latest == nil {
						return interaction.ErrInteractionNotFound
					}
					if latest.IsTerminal() {
						return nil
					}
					_, err = r.services.WorkbenchTaskTracker.TransitionCAS(current, latest.ID, latest.StatusVersion, interaction.InteractionStatusCancelled)
					if !errors.Is(err, interaction.ErrVersionConflict) {
						return err
					}
				}
				return interaction.ErrVersionConflict
			}
			marker := interaction.OwnedReplyFinishedPrefix + inference.Scope.RequestID
			if runErr != nil || generation.Partial {
				marker = interaction.OwnedReplyFailedPrefix + inference.Scope.RequestID
			}
			if _, err := r.services.WorkbenchTaskTracker.UpdateMetadata(current, latest.ID, interaction.InteractionMetadataUpdate{ResultRef: &marker}); err != nil {
				return err
			}
			return interaction.FinalizeOwnedParent(current, r.services.WorkbenchTaskTracker, latest.ID)
		}
		if runErr != nil || generation.Partial {
			message := "父回复未确认完成"
			if runErr != nil {
				message = runErr.Error()
			}
			_, err = r.services.WorkbenchTaskTracker.Fail(current, latest.ID, latest.StatusVersion, "owned_turn_failed", message)
			return err
		}
		_, err = r.services.WorkbenchTaskTracker.Complete(current, latest.ID, latest.StatusVersion, "owned-turn/"+inference.Scope.RequestID)
		return err
	}, nil
}
