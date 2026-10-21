package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/continuity"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

type chatEffectsSourceCallKey struct{}

func (p *meshLocalDataPort) CommitOwnedChatEffects(ctx context.Context, scope coordination.ExecutionScope, request coordination.OwnedChatEffectsRequest) error {
	if scope.ResourceOwnerID != p.ownerID || scope.RoleOwnerID != p.ownerID {
		return coordination.ErrWrongOwner
	}
	if request.ConversationID == "" || request.MessageID == "" || request.RequestID != scope.RequestID || len(request.ConversationID) > 512 || len(request.ThreadID) > 512 {
		return coordination.ErrRequestConflict
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	ctx = coordination.WithoutAdditionalGuard(ctx)
	if callID, ok := ctx.Value(chatEffectsSourceCallKey{}).(string); ok {
		ctx = coordination.WithAdditionalGuard(ctx, func(current context.Context) error {
			db := p.services.KernelContainer.DeviceRegistry.Database()
			if err := coordination.ValidateSourceCall(current, db, scope.SpaceID, callID); err != nil {
				return err
			}
			return coordination.ValidateSourceAuthority(current, db, scope)
		})
	}
	return p.WithSourceRole(ctx, scope, func() error {
		return coordination.CommitCurrent(ctx, func() error { return p.commitOwnedChatEffectsLocked(ctx, scope, request) })
	})
}

func (p *meshLocalDataPort) commitOwnedChatEffectsLocked(ctx context.Context, scope coordination.ExecutionScope, request coordination.OwnedChatEffectsRequest) error {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(requestBody)
	fingerprint := hex.EncodeToString(digest[:])
	receiptID := "chat-effects/" + request.RequestID
	receipt, err := p.store.Get(ctx, "checkpoint", receiptID)
	if err != nil {
		return err
	}
	if receipt != nil {
		var saved struct {
			Hash string `json:"hash"`
		}
		if receipt.Deleted || receipt.RoleID != scope.RoleID || json.Unmarshal(receipt.Body, &saved) != nil || saved.Hash != fingerprint {
			return coordination.ErrRequestConflict
		}
		return nil
	}
	resource, err := p.store.Get(ctx, "message", request.MessageID)
	if err != nil {
		return err
	}
	if resource == nil || resource.Deleted || resource.RoleID != scope.RoleID {
		return coordination.ErrWrongOwner
	}
	var assistant struct {
		ConversationID string `json:"conversationId"`
		Role           string `json:"role"`
		Content        string `json:"content"`
		RequestID      string `json:"requestId"`
	}
	if err := json.Unmarshal(resource.Body, &assistant); err != nil {
		return err
	}
	if assistant.Role != "assistant" || assistant.ConversationID != request.ConversationID || assistant.RequestID != request.RequestID {
		return coordination.ErrWrongOwner
	}
	userResource, err := p.store.Get(ctx, "message", request.RequestID+"/user")
	if err != nil {
		return err
	}
	if userResource == nil || userResource.Deleted || userResource.RoleID != scope.RoleID {
		return coordination.ErrWrongOwner
	}
	var current struct {
		Content        string `json:"content"`
		RequestID      string `json:"requestId"`
		ConversationID string `json:"conversationId"`
		Role           string `json:"role"`
		CreatedAt      string `json:"createdAt"`
	}
	if err := json.Unmarshal(userResource.Body, &current); err != nil {
		return err
	}
	if current.RequestID != request.RequestID || current.ConversationID != request.ConversationID || current.Role != "user" {
		return coordination.ErrWrongOwner
	}
	currentAt, err := time.Parse(time.RFC3339Nano, current.CreatedAt)
	if err != nil {
		return err
	}
	var previousBody []byte
	err = p.services.KernelContainer.DeviceRegistry.Database().QueryRowContext(ctx, `SELECT body FROM kernel_device_owned_resources WHERE owner_id=? AND role_id=? AND kind='message' AND deleted=0 AND json_extract(body,'$.conversationId')=? AND json_extract(body,'$.role')='user' AND json_extract(body,'$.createdAt')<? ORDER BY json_extract(body,'$.createdAt') DESC LIMIT 1`, p.ownerID, scope.RoleID, request.ConversationID, current.CreatedAt).Scan(&previousBody)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var previous time.Time
	if len(previousBody) > 0 {
		var message struct {
			CreatedAt string `json:"createdAt"`
		}
		if err := json.Unmarshal(previousBody, &message); err != nil {
			return err
		}
		previous, err = time.Parse(time.RFC3339Nano, message.CreatedAt)
		if err != nil {
			return err
		}
	}
	moodReceipt, err := p.store.Get(ctx, "checkpoint", receiptID+"/mood")
	if err != nil {
		return err
	}
	if moodReceipt != nil {
		var saved struct {
			Hash string `json:"hash"`
		}
		if moodReceipt.Deleted || moodReceipt.RoleID != scope.RoleID || json.Unmarshal(moodReceipt.Body, &saved) != nil || saved.Hash != fingerprint {
			return coordination.ErrRequestConflict
		}
	} else if p.services.Chat != nil {
		recovery, ok := p.services.Chat.(interface {
			RecoverOwnedMood(context.Context, coordination.ExecutionScope, time.Time, time.Time, string, interaction.PsycheState) error
		})
		if !ok {
			return errors.New("Owner 情绪后处理端口未接通")
		}
		psycheResource, err := p.store.Get(ctx, coordination.RuntimeStateKindPsyche, "role")
		if err != nil {
			return err
		}
		if psycheResource == nil || psycheResource.Deleted || psycheResource.RoleID != scope.RoleID {
			return coordination.ErrWrongOwner
		}
		var currentPsyche interaction.PsycheState
		if err := json.Unmarshal(psycheResource.Body, &currentPsyche); err != nil {
			return err
		}
		if err := recovery.RecoverOwnedMood(ctx, scope, previous, currentAt, request.Source, currentPsyche); err != nil {
			return err
		}
		encoded, err := json.Marshal(map[string]any{"hash": fingerprint})
		if err != nil {
			return err
		}
		phaseScope := scope
		phaseScope.RequestID += "|effects-mood"
		if _, err := p.store.Apply(ctx, coordination.Commit{Scope: phaseScope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: receiptID + "/mood", RoleID: scope.RoleID, Body: encoded}}}); err != nil {
			return err
		}
	}
	if request.ThreadID != "" && p.services.Continuity != nil {
		thread, err := p.services.Continuity.GetThread(request.ThreadID, p.legacySpaceID)
		if err != nil {
			return err
		}
		if thread == nil || thread.CharacterID != scope.RoleID {
			return coordination.ErrWrongOwner
		}
		observer := continuity.NewObserver(p.services.Continuity)
		observer.SetIntelligence(continuity.NewIntelligence(p.services.Chat))
		observer.SetWaitCoordinator(p.services.ContinuityCoordinator)
		if err := observer.Observe(ctx, continuity.ObservePayload{Version: "v1", ThreadID: thread.ID, SpaceID: p.legacySpaceID, CharacterID: scope.RoleID, ConversationID: request.ConversationID, RequestID: request.RequestID, ExecutionID: scope.ExecutionID, UserMessage: current.Content, AssistantReply: assistant.Content, Source: request.Source}); err != nil {
			return err
		}
	}
	encoded, err := json.Marshal(map[string]any{"hash": fingerprint})
	if err != nil {
		return err
	}
	receiptScope := scope
	receiptScope.RequestID += "|effects"
	_, err = p.store.Apply(ctx, coordination.Commit{Scope: receiptScope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: receiptID, RoleID: scope.RoleID, Body: encoded}}})
	return err
}
