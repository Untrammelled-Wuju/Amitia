package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

type ownedIngressActive struct {
	owner interaction.EntryOwner
	scope interaction.InteractionScope
}

type ownedChatIngress struct {
	services *AppServices
	coreID   string
	mu       sync.Mutex
	active   map[string]ownedIngressActive
}

func newOwnedChatIngress(services *AppServices, coreID string) *ownedChatIngress {
	return &ownedChatIngress{services: services, coreID: coreID, active: map[string]ownedIngressActive{}}
}

func (r *ownedChatIngress) EntryReady() bool {
	return r != nil && r.services != nil && r.services.OwnedBusiness != nil && r.services.DeviceMesh != nil && (r.services.ChatRuntime == nil || r.services.ChatRuntime.IsReady())
}

func (r *ownedChatIngress) owner(ctx context.Context, req *interaction.UnifiedEntryRequest) (interaction.EntryOwner, error) {
	owner, found := interaction.EntryOwnerFromContext(ctx)
	if scope, ok := coordination.FromContext(ctx); ok {
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return owner, err
		}
		owner = interaction.EntryOwner{SpaceID: scope.SpaceID, DeviceID: scope.InitiatorDeviceID, TargetDeviceID: scope.TargetDeviceID, CoreID: scope.CoreID}
		found = true
	} else if actor, ok := auth.FromContext(ctx); ok && actor != nil && actor.SpaceID != "" && actor.DeviceID != "" {
		if found && (owner.SpaceID != actor.SpaceID.String() || owner.DeviceID != actor.DeviceID.String()) {
			return owner, coordination.ErrWrongOwner
		}
		owner.SpaceID, owner.DeviceID, owner.CoreID = actor.SpaceID.String(), actor.DeviceID.String(), r.coreID
		found = true
	}
	if !found || owner.SpaceID == "" || owner.DeviceID == "" || owner.CoreID != r.coreID || req.SpaceID != "" && req.SpaceID != owner.SpaceID {
		return owner, coordination.ErrWrongOwner
	}
	if owner.TargetDeviceID == "" {
		owner.TargetDeviceID = owner.DeviceID
	}
	return owner, nil
}

func (r *ownedChatIngress) ExecuteEntry(ctx context.Context, req *interaction.UnifiedEntryRequest) (*interaction.OrchestrationResult, error) {
	if !r.EntryReady() || req == nil {
		return nil, interaction.ErrOrchestratorNotReady
	}
	owner, err := r.owner(ctx, req)
	if err != nil {
		return nil, err
	}
	attachments, err := ownedIngressAttachments(ctx, req)
	if err != nil {
		return nil, err
	}
	expression, err := json.Marshal(req.ExpressionPlan)
	if err != nil {
		return nil, err
	}
	ctx = business.WithIngress(ctx, business.Ingress{Source: req.Source, Channel: req.Channel, PeerID: req.PeerID, ThreadID: req.ThreadID, SessionID: req.SessionID, DeviceTimezone: req.DeviceTimezone, ProactiveTaskInstruction: req.ProactiveTaskInstruction, ProactiveTimeContext: req.ProactiveTimeContext, ProactiveRecentContext: req.ProactiveRecentContext, ProactiveRelationship: req.ProactiveRelationship, ProactiveEmotion: req.ProactiveEmotion, ProactiveMemory: req.ProactiveMemory, MessageStyle: req.MessageStyle, ExpressionPlan: expression, IsInternal: req.IsInternal, SuppressReplyPersistence: req.SuppressReplyPersistence})
	request := business.Request{SpaceID: owner.SpaceID, DeviceID: owner.DeviceID, TargetDeviceID: owner.TargetDeviceID, CoreID: owner.CoreID, RoleID: req.CharacterID, ConversationID: req.ConversationID, RequestID: req.RequestID, Message: req.Message, PermissionMode: req.PermissionMode, Attachments: attachments, ModelConfigID: req.ModelConfigID, ReasoningEffort: req.ReasoningEffort, ReasoningEnabled: req.ReasoningEnabled}
	if req.ReplyToMessageID != nil && strings.TrimSpace(*req.ReplyToMessageID) != "" {
		request.Quote, err = r.reviewReplyReference(ctx, request, strings.TrimSpace(*req.ReplyToMessageID))
		if err != nil {
			return nil, err
		}
	}
	activeKey := owner.SpaceID + "\x00" + owner.DeviceID + "\x00" + req.RequestID
	r.mu.Lock()
	if _, exists := r.active[activeKey]; exists {
		r.mu.Unlock()
		return nil, interaction.ErrOrchestratorProcessing
	}
	r.active[activeKey] = ownedIngressActive{owner: owner, scope: interaction.InteractionScope{SpaceID: owner.SpaceID, CharacterID: req.CharacterID, ConversationID: req.ConversationID, Channel: req.Channel, PeerID: req.PeerID, SessionID: req.SessionID, ThreadID: req.ThreadID, RequestID: req.RequestID, Source: req.Source}}
	r.mu.Unlock()
	defer func() { r.mu.Lock(); delete(r.active, activeKey); r.mu.Unlock() }()
	started := time.Now()
	response, err := r.services.OwnedBusiness.Run(ctx, request)
	result := &interaction.OrchestrationResult{Outcome: interaction.OutcomeCompleted, Duration: time.Since(started)}
	if err != nil {
		result.Outcome, result.Error = interaction.OutcomeFailed, err.Error()
		if errors.Is(err, business.ErrInterrupted) || errors.Is(err, context.Canceled) {
			result.Outcome = interaction.OutcomeCancelled
		}
		return result, err
	}
	if tracker := r.services.WorkbenchTaskTracker; tracker != nil {
		key := ownedParentKey(business.Inference{Scope: response.Scope, ConversationID: response.ConversationID})
		if parent, found, lookupErr := tracker.GetByRequestID(ctx, response.Scope.SpaceID, key); lookupErr != nil {
			return nil, lookupErr
		} else if found && parent != nil {
			result.InteractionID = parent.ID
		}
	}
	result.Response = &interaction.ProcessResponse{ConversationID: response.ConversationID, ThreadID: req.ThreadID, TurnID: response.TurnID, Reply: response.Text, Reasoning: response.Reasoning, CharacterID: response.Scope.RoleID, MessageIDs: []string{response.RequestID + "/user", response.RequestID + "/assistant"}, ForceVoice: response.ForceVoice, RequestID: response.RequestID}
	if response.Suppressed || req.SuppressReplyPersistence {
		result.Response.Reply, result.Response.Reasoning = "", ""
		result.Response.MessageIDs = nil
		result.Response.ForceVoice = false
		return result, nil
	}
	if err := ownedIngressResponseOutputs(result.Response, response.Outputs); err != nil {
		return nil, err
	}
	if response.Saved && req.PeerID != "" && req.Channel != "" && !strings.EqualFold(req.Channel, "web") && !strings.EqualFold(req.Channel, "voice") {
		if err := r.services.DeviceMesh.SubmitChannelDelivery(ctx, response.Scope, coordination.ChannelDeliveryRequest{ConversationID: response.ConversationID, RequestID: response.RequestID, Channel: req.Channel, PeerID: req.PeerID, Text: response.Text, Outputs: response.Outputs}); err != nil {
			return result, fmt.Errorf("回复已保存，渠道交付等待重试: %w", err)
		}
	}
	return result, nil
}

func (r *ownedChatIngress) CancelEntryScope(scope interaction.InteractionScope) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, active := range r.active {
		current := active.scope
		if scope.SpaceID != "" && scope.SpaceID != current.SpaceID || scope.CharacterID != "" && scope.CharacterID != current.CharacterID || scope.ConversationID != "" && scope.ConversationID != current.ConversationID || scope.Channel != "" && scope.Channel != current.Channel || scope.PeerID != "" && scope.PeerID != current.PeerID || scope.SessionID != "" && scope.SessionID != current.SessionID || scope.ThreadID != "" && scope.ThreadID != current.ThreadID || scope.Source != "" && scope.Source != current.Source || scope.RequestID != "" && scope.RequestID != current.RequestID {
			continue
		}
		if r.services.OwnedBusiness.Interrupt(active.owner.SpaceID, active.owner.DeviceID, current.RequestID) {
			count++
		}
	}
	return count
}

func (r *ownedChatIngress) EntryInteractionActive(id string) bool {
	if r.services.WorkbenchTaskTracker != nil {
		if parent, found, err := r.services.WorkbenchTaskTracker.Get(context.Background(), id); err == nil && found && parent != nil && parent.RecoveryDescriptor != nil && parent.RecoveryDescriptor.OwnedTurn != nil {
			id = parent.RecoveryDescriptor.OwnedTurn.RequestID
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, active := range r.active {
		if strings.TrimSpace(id) == active.scope.RequestID {
			return true
		}
	}
	return false
}
