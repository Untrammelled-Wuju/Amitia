package agentplatform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"gorm.io/gorm"
)

func (s *Service) SaveApprovalRequest(ctx context.Context, request execution.ApprovalRequest) error {
	if s == nil || s.db == nil {
		return errors.New("Agent 审批持久化服务不可用")
	}
	agentID := strings.TrimSpace(request.AgentID)
	teamID := strings.TrimSpace(request.TeamID)
	if request.CharacterID != "" || agentID == "" {
		agent, err := s.agentForCharacter(ctx, request.SpaceID, request.CharacterID)
		if err != nil {
			return err
		}
		if agentID == "" {
			agentID = agent.ID
		}
		if teamID == "" {
			teamID = agent.TeamID
		}
	}
	arguments := map[string]any{}
	if strings.TrimSpace(request.Arguments) != "" {
		if err := json.Unmarshal([]byte(request.Arguments), &arguments); err != nil {
			arguments = map[string]any{"raw": request.Arguments}
		}
	}
	argumentsJSON, _, err := hashJSON(arguments)
	if err != nil {
		return err
	}
	createdAt, err := time.Parse(time.RFC3339Nano, request.CreatedAt)
	if err != nil {
		createdAt = time.Now().UTC()
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, request.ExpiresAt)
	if err != nil {
		expiresAt = createdAt.Add(5 * time.Minute)
	}
	requestHash, _, err := hashJSON(map[string]any{"toolName": request.ToolName, "arguments": arguments, "toolCallId": request.ToolCallID})
	if err != nil {
		return err
	}
	record := ApprovalRecord{
		ID: request.ID, SpaceID: request.SpaceID, AgentID: agentID, TeamID: teamID,
		ConversationID: request.ConversationID, TurnID: request.TurnID, ToolCallID: request.ToolCallID,
		ToolName: request.ToolName, ActionSummary: "执行工具 " + request.ToolName,
		ResourceSummary: request.ToolName, ArgumentsJSON: argumentsJSON, EvidenceJSON: "{}", ScopeJSON: "{}",
		RiskLevel: firstNonEmpty(request.RiskLevel, "medium"), Status: "pending", AllowOnce: true,
		RequestHash: requestHash, ExpiresAt: expiresAt, Revision: 1, CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	result := s.db.WithContext(ctx).Where("id = ?", request.ID).FirstOrCreate(&record)
	if result.Error != nil {
		return fmt.Errorf("持久化工具审批: %w", result.Error)
	}
	if err := s.recordActivity(ctx, ActivityEvent{
		SpaceID: request.SpaceID, AgentID: agentID, TeamID: teamID,
		ConversationID: request.ConversationID, Kind: "approval", State: "pending",
		Title: record.ActionSummary, WaitingReason: "等待用户审批", Detail: request.ToolName,
	}); err != nil {
		return err
	}
	return nil
}

func (s *Service) SaveApprovalResolution(ctx context.Context, request execution.ApprovalRequest, actor execution.ApprovalActor) error {
	if s == nil || s.db == nil {
		return errors.New("Agent 审批持久化服务不可用")
	}
	if err := s.SaveApprovalRequest(ctx, request); err != nil {
		return err
	}
	status := string(request.Status)
	if status == "" {
		status = "pending"
	}
	now := time.Now().UTC()
	decision := ApprovalDecision{
		ID: newID("approvaldecision"), ApprovalID: request.ID, Decision: status,
		GrantMode: "once", ActorID: firstNonEmpty(actor.ID, request.ResolvedBy), CreatedAt: now,
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current ApprovalRecord
		if err := tx.Where("id = ? AND space_id = ?", request.ID, request.SpaceID).First(&current).Error; err != nil {
			return fmt.Errorf("读取持久审批: %w", err)
		}
		if current.Status == status {
			return nil
		}
		updates := map[string]any{
			"status": status, "decided_by": decision.ActorID, "decided_at": now,
			"revision": current.Revision + 1, "updated_at": now,
		}
		if request.ResolvedAt != "" {
			if parsed, parseErr := time.Parse(time.RFC3339Nano, request.ResolvedAt); parseErr == nil {
				updates["decided_at"] = parsed
			}
		}
		if err := tx.Model(&ApprovalRecord{}).Where("id = ? AND revision = ?", current.ID, current.Revision).Updates(updates).Error; err != nil {
			return fmt.Errorf("保存持久审批决定: %w", err)
		}
		if err := tx.Create(&decision).Error; err != nil {
			return fmt.Errorf("保存持久审批审计: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := s.recordActivity(ctx, ActivityEvent{
		SpaceID: request.SpaceID, AgentID: request.AgentID, TeamID: request.TeamID,
		ConversationID: request.ConversationID, Kind: "approval", State: status,
		Title: request.ToolName, Detail: request.ResolvedByType,
	}); err != nil {
		return err
	}
	return nil
}

func (s *Service) FindApprovedApproval(ctx context.Context, request execution.ApprovalRequest) (execution.ApprovalRequest, bool, error) {
	if s == nil || s.db == nil || strings.TrimSpace(request.ToolCallID) == "" {
		return execution.ApprovalRequest{}, false, nil
	}
	var record ApprovalRecord
	err := s.db.WithContext(ctx).Where(
		"space_id = ? AND conversation_id = ? AND tool_call_id = ? AND tool_name = ? AND status = ? AND resumed_at IS NULL",
		request.SpaceID, request.ConversationID, request.ToolCallID, request.ToolName, "approved",
	).Order("created_at DESC").First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return execution.ApprovalRequest{}, false, nil
	}
	if err != nil {
		return execution.ApprovalRequest{}, false, fmt.Errorf("查找已批准审批: %w", err)
	}
	var recordArguments map[string]any
	var requestArguments map[string]any
	if err := json.Unmarshal([]byte(record.ArgumentsJSON), &recordArguments); err != nil {
		return execution.ApprovalRequest{}, false, fmt.Errorf("解析持久审批参数: %w", err)
	}
	if err := json.Unmarshal([]byte(request.Arguments), &requestArguments); err != nil {
		requestArguments = map[string]any{"raw": request.Arguments}
	}
	recordHash, _, err := hashJSON(map[string]any{"toolName": record.ToolName, "arguments": recordArguments, "toolCallId": record.ToolCallID})
	if err != nil {
		return execution.ApprovalRequest{}, false, err
	}
	requestHash, _, err := hashJSON(map[string]any{"toolName": request.ToolName, "arguments": requestArguments, "toolCallId": request.ToolCallID})
	if err != nil {
		return execution.ApprovalRequest{}, false, err
	}
	if recordHash != requestHash {
		return execution.ApprovalRequest{}, false, nil
	}
	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).Model(&ApprovalRecord{}).Where("id = ? AND revision = ?", record.ID, record.Revision).Updates(map[string]any{
		"resumed_at": now, "revision": record.Revision + 1, "updated_at": now,
	}).Error; err != nil {
		return execution.ApprovalRequest{}, false, fmt.Errorf("标记审批恢复: %w", err)
	}
	request.ID = record.ID
	request.AgentID = record.AgentID
	request.TeamID = record.TeamID
	request.Status = execution.ApprovalStatusApproved
	request.ResolvedBy = record.DecidedBy
	return request, true, nil
}

func (s *Service) agentForCharacter(ctx context.Context, spaceID, characterID string) (AgentProfile, error) {
	var agent AgentProfile
	if strings.TrimSpace(characterID) == "" {
		return agent, nil
	}
	if err := s.db.WithContext(ctx).Where("space_id = ? AND character_id = ? AND deleted_at IS NULL", spaceID, characterID).Order("updated_at DESC").First(&agent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AgentProfile{}, nil
		}
		return AgentProfile{}, fmt.Errorf("读取 Agent 审批归属: %w", err)
	}
	return agent, nil
}
