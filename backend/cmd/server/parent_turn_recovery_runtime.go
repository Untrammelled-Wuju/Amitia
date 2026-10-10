package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

type parentTurnRecoveryCandidate struct {
	parentID string
	request  *interaction.UnifiedEntryRequest
}

type parentTurnRecoveryRuntime struct {
	mu        sync.Mutex
	attempted map[string]time.Time
	active    int
	wg        sync.WaitGroup
}

func newParentTurnRecoveryRuntime() *parentTurnRecoveryRuntime {
	return &parentTurnRecoveryRuntime{attempted: make(map[string]time.Time)}
}

func findRecoverableParentTurns(ctx context.Context, db *gorm.DB) ([]parentTurnRecoveryCandidate, error) {
	if db == nil || !db.Migrator().HasTable(&interaction.InteractionRecordModel{}) ||
		!db.Migrator().HasTable(&chat.AssistantTurn{}) ||
		!db.Migrator().HasTable(&chat.AssistantTurnItem{}) ||
		!db.Migrator().HasTable(&chat.Message{}) ||
		!db.Migrator().HasTable(&chat.Conversation{}) {
		return nil, nil
	}
	var candidates []parentTurnRecoveryCandidate
	for lastID := ""; ; {
		var records []interaction.InteractionRecordModel
		query := db.WithContext(ctx).Where("status = ? AND recovery_descriptor_json <> '' AND id > ?",
			string(interaction.InteractionStatusContextReady), lastID)
		if err := query.Order("id ASC").Limit(64).Find(&records).Error; err != nil {
			return nil, err
		}
		if len(records) == 0 {
			break
		}
		lastID = records[len(records)-1].ID
		for _, record := range records {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if record.Channel != "web" || record.SpaceID == "" || record.CharacterID == "" ||
				record.ConversationID == "" || record.RequestID == "" || record.SupersededByID != "" ||
				record.CancelReason != "" || !record.CancelRequestedAt.IsZero() || record.CommitID != "" {
				continue
			}
			descriptor, err := interaction.DescriptorFromJSON([]byte(record.RecoveryDescriptorJSON))
			if err != nil || descriptor == nil || (descriptor.MultiAgent == nil && descriptor.ParentTurn == nil) {
				continue
			}
			check := *descriptor
			oldFingerprint := check.Fingerprint
			if oldFingerprint == "" {
				continue
			}
			check.ComputeFingerprint()
			if check.Fingerprint != oldFingerprint {
				continue
			}
			var workspaceID, workspaceDeviceID, permissionMode, workspaceName, workspaceKind, workspaceRootURI string
			var threadID, messageStyle string
			if ref := descriptor.MultiAgent; ref != nil {
				if ref.CoordinationID == "" || ref.WorkspaceID == "" ||
					ref.ParentGoalSpaceID != record.SpaceID ||
					ref.ParentGoalCharacterID != record.CharacterID ||
					ref.ParentGoalConversationID != record.ConversationID ||
					!interaction.CoordinationStatus(ref.Status).IsTerminal() {
					continue
				}
				workspaceID, workspaceDeviceID, permissionMode = ref.WorkspaceID, ref.WorkspaceDeviceID, ref.PermissionMode
				workspaceName, workspaceKind, workspaceRootURI = ref.WorkspaceName, ref.WorkspaceKind, ref.WorkspaceRootURI
			} else {
				ref := descriptor.ParentTurn
				if descriptor.Scope.SpaceID != record.SpaceID ||
					descriptor.Scope.CharacterID != record.CharacterID ||
					descriptor.Scope.ConversationID != record.ConversationID ||
					descriptor.Scope.Channel != record.Channel ||
					descriptor.Scope.PeerID != record.PeerID ||
					descriptor.Scope.SessionID != record.SessionID ||
					descriptor.Interaction.InteractionID != record.ID ||
					descriptor.Interaction.RequestID != record.RequestID ||
					descriptor.Interaction.Status != interaction.InteractionStatusContextReady ||
					descriptor.Interaction.StatusVersion != record.StatusVersion ||
					descriptor.Requirement != interaction.RecoveryRequired {
					continue
				}
				workspaceID, workspaceDeviceID, permissionMode = ref.WorkspaceID, ref.WorkspaceDeviceID, ref.PermissionMode
				workspaceName, workspaceKind, workspaceRootURI = ref.WorkspaceName, ref.WorkspaceKind, ref.WorkspaceRootURI
				threadID, messageStyle = ref.ThreadID, ref.MessageStyle
			}
			var conversation chat.Conversation
			err = db.WithContext(ctx).Where("id = ? AND space_id = ?",
				record.ConversationID, record.SpaceID).First(&conversation).Error
			if err == gorm.ErrRecordNotFound {
				continue
			}
			if err != nil {
				return nil, err
			}
			if conversation.Channel != record.Channel ||
				(conversation.WorkspaceID != "" && conversation.WorkspaceID != workspaceID) ||
				(conversation.WorkspaceDeviceID != "" && conversation.WorkspaceDeviceID != workspaceDeviceID) ||
				(conversation.PermissionMode != "" && permissionMode != "" &&
					conversation.PermissionMode != permissionMode) {
				continue
			}
			if ref := descriptor.ParentTurn; descriptor.MultiAgent == nil && ref != nil {
				if baseline := ref.ConversationSnapshot; baseline != nil {
					if conversation.ModelConfigID != baseline.ModelConfigID ||
						conversation.ReasoningEffort != baseline.ReasoningEffort ||
						conversation.ReasoningEnabled != baseline.ReasoningEnabled ||
						conversation.PermissionMode != baseline.PermissionMode ||
						conversation.WorkspaceID != baseline.WorkspaceID ||
						conversation.WorkspaceDeviceID != baseline.WorkspaceDeviceID {
						continue
					}
				} else if conversation.ModelConfigID != ref.ModelConfigID ||
					conversation.ReasoningEffort != ref.ReasoningEffort ||
					conversation.PermissionMode != ref.PermissionMode {
					continue
				}
			}
			var peers []interaction.InteractionRecordModel
			err = db.WithContext(ctx).Model(&interaction.InteractionRecordModel{}).
				Select("id", "created_at").
				Where("space_id = ? AND conversation_id = ? AND character_id = ? AND id <> ?",
					record.SpaceID, record.ConversationID, record.CharacterID, record.ID).
				Order("created_at DESC").Limit(256).Find(&peers).Error
			if err != nil {
				return nil, err
			}
			if len(peers) >= 256 {
				continue
			}
			newer := false
			for _, peer := range peers {
				if !peer.CreatedAt.Before(record.CreatedAt) {
					newer = true
					break
				}
			}
			if newer {
				continue
			}
			var committed int64
			if err := db.WithContext(ctx).Model(&chat.Message{}).
				Where("conversation_id = ? AND request_id = ? AND role = ?",
					record.ConversationID, record.RequestID, "assistant").
				Count(&committed).Error; err != nil {
				return nil, err
			}
			if committed != 0 {
				continue
			}
			var turn chat.AssistantTurn
			err = db.WithContext(ctx).Where("conversation_id = ? AND character_id = ? AND request_id = ? AND status IN ?",
				record.ConversationID, record.CharacterID, record.RequestID,
				[]string{"running", "starting", "waiting_tool", "interrupted", "needs_reconciliation"}).
				Order("sequence DESC").First(&turn).Error
			if err == gorm.ErrRecordNotFound {
				continue
			}
			if err != nil {
				return nil, err
			}
			if turn.ID == "" || turn.ExecutionID == "" || turn.UserMessageID == "" {
				continue
			}
			var original chat.Message
			err = db.WithContext(ctx).Where("id = ? AND conversation_id = ? AND character_id = ? AND request_id = ?",
				turn.UserMessageID, record.ConversationID, record.CharacterID, record.RequestID).
				First(&original).Error
			if err == gorm.ErrRecordNotFound {
				continue
			}
			if err != nil {
				return nil, err
			}
			if original.Role != "user" || strings.TrimSpace(original.Content) == "" ||
				original.AudioUrl != "" || original.ImageUrl != "" || original.VideoUrl != "" {
				continue
			}
			var items []chat.AssistantTurnItem
			if err := db.WithContext(ctx).Where("turn_id = ?", turn.ID).Order("sequence ASC").Find(&items).Error; err != nil {
				return nil, err
			}
			items, err = chat.ReconcileAgentToolCheckpoint(ctx, db, turn, items)
			if err != nil {
				log.Warn("parent_turn.tool_reconciliation_failed interaction=", record.ID, " turn=", turn.ID, " error=", err)
				continue
			}
			if !parentTurnJournalSafeToResume(items) {
				continue
			}
			modelConfigID, reasoningEffort := conversation.ModelConfigID, conversation.ReasoningEffort
			if descriptor.MultiAgent == nil && descriptor.ParentTurn != nil {
				modelConfigID = descriptor.ParentTurn.ModelConfigID
				reasoningEffort = descriptor.ParentTurn.ReasoningEffort
			}
			var reasoningEnabled *bool
			if conversation.ReasoningEnabled >= 0 {
				enabled := conversation.ReasoningEnabled == 1
				reasoningEnabled = &enabled
			}
			candidates = append(candidates, parentTurnRecoveryCandidate{
				parentID: record.ID,
				request: &interaction.UnifiedEntryRequest{
					SpaceID: record.SpaceID, CharacterID: record.CharacterID,
					ConversationID: record.ConversationID, Channel: record.Channel,
					PeerID: record.PeerID, SessionID: record.SessionID,
					Source: record.Source, RequestID: record.RequestID,
					Message: original.Content, TurnID: turn.ID, ExecutionID: turn.ExecutionID,
					WorkspaceID: workspaceID, WorkspaceName: workspaceName,
					WorkspaceKind: workspaceKind, WorkspaceRootURI: workspaceRootURI,
					WorkspaceDeviceID: workspaceDeviceID, PermissionMode: permissionMode,
					ThreadID: threadID, MessageStyle: messageStyle,
					ModelConfigID: modelConfigID, ReasoningEffort: reasoningEffort,
					ReasoningEnabled:      reasoningEnabled,
					ReservedInteractionID: record.ID, RecoverExistingTurn: true,
				},
			})
		}
		if len(records) < 64 {
			break
		}
	}
	return candidates, nil
}

func parentTurnJournalSafeToResume(items []chat.AssistantTurnItem) bool {
	calls := make(map[string]bool)
	for _, item := range items {
		switch item.ItemType {
		case "tool_call":
			if item.CallID == "" {
				return false
			}
			if _, exists := calls[item.CallID]; exists {
				return false
			}
			calls[item.CallID] = false
		case "tool_result":
			done, exists := calls[item.CallID]
			if !exists || done || item.ResultJSON == "" || !json.Valid([]byte(item.ResultJSON)) {
				return false
			}
			calls[item.CallID] = true
		}
	}
	for _, complete := range calls {
		if !complete {
			return false
		}
	}
	return true
}

func (r *parentTurnRecoveryRuntime) Scan(ctx, parentCtx context.Context, svc *AppServices) error {
	if r == nil || svc == nil || svc.DB == nil || svc.UnifiedEntry == nil {
		return nil
	}
	candidates, err := findRecoverableParentTurns(ctx, svc.DB)
	if err != nil {
		return fmt.Errorf("parent turn recovery candidate lookup: %w", err)
	}
	for _, candidate := range candidates {
		if svc.UnifiedEntry.IsInteractionActive(candidate.parentID) {
			continue
		}
		r.mu.Lock()
		retryAt, attempted := r.attempted[candidate.parentID]
		if (attempted && (retryAt.IsZero() || time.Now().Before(retryAt))) || r.active >= 2 {
			r.mu.Unlock()
			continue
		}
		r.attempted[candidate.parentID] = time.Time{}
		r.active++
		r.wg.Add(1)
		r.mu.Unlock()
		go func(c parentTurnRecoveryCandidate) {
			defer r.wg.Done()
			defer func() { r.mu.Lock(); r.active--; r.mu.Unlock() }()
			if _, err := svc.UnifiedEntry.Handle(parentCtx, c.request); err != nil && parentCtx.Err() == nil {
				if errors.Is(err, interaction.ErrOrchestratorProcessing) ||
					errors.Is(err, interaction.ErrOrchestratorNotReady) ||
					errors.Is(err, interaction.ErrOrchestratorBusy) ||
					errors.Is(err, interaction.ErrBackpressureCooldown) ||
					errors.Is(err, interaction.ErrBackpressureShedding) {
					r.mu.Lock()
					r.attempted[c.parentID] = time.Now().Add(30 * time.Second)
					r.mu.Unlock()
				} else {
					log.Warn("parent_turn.recovery.requires_attention interaction=", c.parentID, " turn=", c.request.TurnID, " execution=", c.request.ExecutionID, " error=", err)
				}
			}
		}(candidate)
	}
	return nil
}

func (r *parentTurnRecoveryRuntime) Wait(ctx context.Context) error {
	if r == nil {
		return nil
	}
	done := make(chan struct{})
	go func() { r.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
