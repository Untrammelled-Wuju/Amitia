package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/temporal"
	"strings"
)

type chatEffectsSourceCallKey struct{}

func (p *meshLocalDataPort) CommitOwnedChatEffects(ctx context.Context, scope coordination.ExecutionScope, request coordination.OwnedChatEffectsRequest) error {
	if scope.ResourceOwnerID != p.ownerID || scope.RoleOwnerID != p.ownerID {
		return coordination.ErrWrongOwner
	}
	if request.ConversationID == "" || !request.Abort && request.MessageID == "" || request.RequestID != scope.RequestID || len(request.ConversationID) > 512 || len(request.ThreadID) > 512 {
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
	if err := p.WithSourceRole(ctx, scope, func() error {
		return coordination.CommitCurrent(ctx, func() error { return p.commitOwnedChatEffectsLocked(ctx, scope, request) })
	}); err != nil {
		return fmt.Errorf("群聊回复后处理失败: %w", err)
	}
	return nil
}

func (p *meshLocalDataPort) commitOwnedChatEffectsLocked(ctx context.Context, scope coordination.ExecutionScope, request coordination.OwnedChatEffectsRequest) error {
	if request.Abort {
		if p.services.RelTimeCoordinator == nil {
			return nil
		}
		message, err := p.store.Get(ctx, "message", request.RequestID+"/assistant")
		if err != nil {
			return err
		}
		if message != nil && !message.Deleted && message.RoleID == coordination.StorageRole(scope, "message") {
			checkpoint, err := p.store.Get(ctx, "checkpoint", "turn/"+request.RequestID)
			if err != nil {
				return err
			}
			var state struct {
				Status string `json:"status"`
			}
			if checkpoint != nil && json.Unmarshal(checkpoint.Body, &state) == nil && state.Status == "completed" {
				return nil
			}
		}
		return p.services.RelTimeCoordinator.ForOwner(p.ownerID).FinishRequest(ctx, temporal.RelationshipOwnerSpace(p.legacySpaceID, p.ownerID), scope.RoleID, request.RequestID, "", true, false)
	}
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
	if resource == nil || resource.Deleted || resource.RoleID != coordination.StorageRole(scope, "message") {
		return fmt.Errorf("assistant resource: %w", coordination.ErrWrongOwner)
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
		return fmt.Errorf("assistant identity: %w", coordination.ErrWrongOwner)
	}
	userMessageID := request.UserMessageID
	if userMessageID == "" {
		userMessageID = request.RequestID + "/user"
	}
	userResource, err := p.store.Get(ctx, "message", userMessageID)
	if err != nil {
		return err
	}
	if userResource == nil || userResource.Deleted || userResource.RoleID != coordination.StorageRole(scope, "message") {
		return fmt.Errorf("user resource: %w", coordination.ErrWrongOwner)
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
	if (request.GroupRoundID == "" && current.RequestID != request.RequestID) || current.ConversationID != request.ConversationID || current.Role != "user" {
		return fmt.Errorf("user identity: %w", coordination.ErrWrongOwner)
	}
	currentAt, err := time.Parse(time.RFC3339Nano, current.CreatedAt)
	if err != nil {
		return err
	}
	if !request.IsInternal && p.services.RelTimeCoordinator != nil {
		proactive := strings.Contains(strings.ToLower(request.Source), "proactive") || strings.Contains(strings.ToLower(request.Source), "active_message")
		if err := p.services.RelTimeCoordinator.ForOwner(p.ownerID).FinishRequest(ctx, temporal.RelationshipOwnerSpace(p.legacySpaceID, p.ownerID), scope.RoleID, request.RequestID, current.Content, false, proactive); err != nil {
			return err
		}
	}
	var previousBody []byte
	err = p.services.KernelContainer.DeviceRegistry.Database().QueryRowContext(ctx, `SELECT body FROM kernel_device_owned_resources WHERE owner_id=? AND role_id=? AND kind='message' AND deleted=0 AND json_extract(body,'$.conversationId')=? AND json_extract(body,'$.role')='user' AND json_extract(body,'$.createdAt')<? ORDER BY json_extract(body,'$.createdAt') DESC LIMIT 1`, p.ownerID, coordination.StorageRole(scope, "message"), request.ConversationID, current.CreatedAt).Scan(&previousBody)
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
		psycheResource, err := p.ownedMoodPsycheResource(ctx, scope)
		if err != nil {
			return err
		}
		if psycheResource != nil && psycheResource.Deleted {
			psycheResource = nil
		}
		if psycheResource != nil && psycheResource.RoleID != scope.RoleID {
			return coordination.ErrWrongOwner
		}
		if psycheResource != nil {
			var currentPsyche interaction.PsycheState
			if err := json.Unmarshal(psycheResource.Body, &currentPsyche); err != nil {
				return err
			}
			enabled, threshold, err := ownedMoodDetectionSettings(ctx, p.services.DB)
			if err != nil {
				return err
			}
			encoded, err := json.Marshal(map[string]any{"hash": fingerprint})
			if err != nil {
				return err
			}
			phaseScope := scope
			phaseScope.RequestID += "|effects-mood"
			mutations := []coordination.Mutation{{Kind: "checkpoint", ID: receiptID + "/mood", RoleID: scope.RoleID, Body: encoded}}
			if enabled {
				snapshot := buildOwnedEmotionSnapshot(scope, request, previous, currentAt, currentPsyche, threshold)
				if snapshot.Confidence >= threshold {
					mutations = append(mutations, coordination.Mutation{Kind: business.EmotionSnapshotKind, ID: snapshot.ID, RoleID: scope.RoleID, Body: ownedEmotionSnapshotBody(snapshot)})
				}
			}
			if _, err := p.store.Apply(ctx, coordination.Commit{Scope: phaseScope, Mutations: mutations}); err != nil {
				return err
			}
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

func (p *meshLocalDataPort) ownedMoodPsycheResource(ctx context.Context, scope coordination.ExecutionScope) (*coordination.Resource, error) {
	resource, err := p.store.Get(ctx, coordination.RuntimeStateKindPsyche, coordination.RoleRuntimeStateID(coordination.RuntimeStateKindPsyche, scope.RoleID))
	if err != nil || resource != nil {
		return resource, err
	}
	legacy, err := p.store.Get(ctx, coordination.RuntimeStateKindPsyche, coordination.LegacyRuntimeStateID(coordination.RuntimeStateKindPsyche))
	if err != nil || legacy == nil || legacy.RoleID != scope.RoleID {
		return nil, err
	}
	return legacy, nil
}
