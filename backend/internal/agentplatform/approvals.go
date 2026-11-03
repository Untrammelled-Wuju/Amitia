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
	"gorm.io/gorm/clause"
)

type CreateApprovalInput struct {
	AgentID          string         `json:"agentId"`
	TeamID           string         `json:"teamId"`
	ConversationID   string         `json:"conversationId"`
	TurnID           string         `json:"turnId"`
	ToolCallID       string         `json:"toolCallId"`
	ToolName         string         `json:"toolName"`
	ActionSummary    string         `json:"actionSummary"`
	ResourceSummary  string         `json:"resourceSummary"`
	Arguments        map[string]any `json:"arguments"`
	Evidence         map[string]any `json:"evidence"`
	Scope            map[string]any `json:"scope"`
	RiskLevel        string         `json:"riskLevel"`
	Reversible       bool           `json:"reversible"`
	AllowOnce        bool           `json:"allowOnce"`
	SessionGrant     bool           `json:"sessionGrant"`
	ExpiresInSeconds int            `json:"expiresInSeconds"`
	RequestedBy      string         `json:"requestedBy"`
}

type DecideApprovalInput struct {
	Decision         string `json:"decision"`
	GrantMode        string `json:"grantMode"`
	Reason           string `json:"reason"`
	ActorID          string `json:"actorId"`
	ExpectedRevision int64  `json:"expectedRevision"`
	TimedMinutes     int    `json:"timedMinutes"`
}

type CreateReconciliationInput struct {
	AgentID       string `json:"agentId"`
	ApprovalID    string `json:"approvalId"`
	ToolCallID    string `json:"toolCallId"`
	Reason        string `json:"reason"`
	OriginalError string `json:"originalError"`
	ResultHash    string `json:"resultHash"`
}

type ResolveReconciliationInput struct {
	Resolution       string `json:"resolution"`
	ActorID          string `json:"actorId"`
	ExpectedRevision int64  `json:"expectedRevision"`
}

func (s *Service) CreateApproval(ctx context.Context, spaceID string, input CreateApprovalInput) (ApprovalRecord, error) {
	if strings.TrimSpace(input.ToolName) == "" {
		return ApprovalRecord{}, errors.New("审批工具名称不能为空")
	}
	if strings.TrimSpace(input.ActionSummary) == "" {
		return ApprovalRecord{}, errors.New("审批动作说明不能为空")
	}
	argumentsJSON, _, err := hashJSON(input.Arguments)
	if err != nil {
		return ApprovalRecord{}, err
	}
	evidenceJSON, _, err := hashJSON(input.Evidence)
	if err != nil {
		return ApprovalRecord{}, err
	}
	scopeJSON, _, err := hashJSON(input.Scope)
	if err != nil {
		return ApprovalRecord{}, err
	}
	requestHash, _, err := hashJSON(map[string]any{
		"agentId": input.AgentID, "toolCallId": input.ToolCallID, "toolName": input.ToolName,
		"arguments": input.Arguments, "scope": input.Scope,
	})
	if err != nil {
		return ApprovalRecord{}, err
	}
	expiresIn := input.ExpiresInSeconds
	if expiresIn <= 0 {
		expiresIn = 300
	}
	if expiresIn > 86400 {
		expiresIn = 86400
	}
	risk := strings.TrimSpace(input.RiskLevel)
	if risk == "" {
		risk = "medium"
	}
	now := time.Now().UTC()
	approval := ApprovalRecord{
		ID: newID("approval"), SpaceID: spaceID, AgentID: input.AgentID, TeamID: input.TeamID,
		ConversationID: input.ConversationID, TurnID: input.TurnID, ToolCallID: input.ToolCallID,
		ToolName: input.ToolName, ActionSummary: input.ActionSummary, ResourceSummary: input.ResourceSummary,
		ArgumentsJSON: argumentsJSON, EvidenceJSON: evidenceJSON, ScopeJSON: scopeJSON,
		RiskLevel: risk, Reversible: input.Reversible, Status: "pending",
		AllowOnce: input.AllowOnce, SessionGrant: input.SessionGrant, RequestHash: requestHash,
		RequestedBy: input.RequestedBy, ExpiresAt: now.Add(time.Duration(expiresIn) * time.Second),
		Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&approval).Error; err != nil {
		return ApprovalRecord{}, fmt.Errorf("创建持久审批: %w", err)
	}
	if err := s.recordActivity(ctx, ActivityEvent{SpaceID: spaceID, AgentID: input.AgentID, TeamID: input.TeamID, ConversationID: input.ConversationID, Kind: "approval", State: "pending", Title: input.ActionSummary, WaitingReason: "等待用户审批", Detail: input.ResourceSummary}); err != nil {
		return ApprovalRecord{}, err
	}
	return approval, nil
}

func (s *Service) ListApprovals(ctx context.Context, spaceID, status, agentID, conversationID string, limit int) ([]ApprovalRecord, error) {
	query := s.db.WithContext(ctx).Where("space_id = ?", spaceID)
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(status))
	}
	if strings.TrimSpace(agentID) != "" {
		query = query.Where("agent_id = ?", strings.TrimSpace(agentID))
	}
	if strings.TrimSpace(conversationID) != "" {
		query = query.Where("conversation_id = ?", strings.TrimSpace(conversationID))
	}
	approvals := make([]ApprovalRecord, 0)
	if err := query.Order("created_at DESC").Limit(trimLimit(limit, 100, 500)).Find(&approvals).Error; err != nil {
		return nil, fmt.Errorf("读取持久审批: %w", err)
	}
	return approvals, nil
}

func (s *Service) GetApproval(ctx context.Context, spaceID, approvalID string) (ApprovalRecord, []ApprovalDecision, error) {
	var approval ApprovalRecord
	if err := s.db.WithContext(ctx).Where("id = ? AND space_id = ?", approvalID, spaceID).First(&approval).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ApprovalRecord{}, nil, errors.New("审批不存在")
		}
		return ApprovalRecord{}, nil, fmt.Errorf("读取审批: %w", err)
	}
	decisions := make([]ApprovalDecision, 0)
	if err := s.db.WithContext(ctx).Where("approval_id = ?", approvalID).Order("created_at ASC").Find(&decisions).Error; err != nil {
		return ApprovalRecord{}, nil, fmt.Errorf("读取审批决定: %w", err)
	}
	return approval, decisions, nil
}

func (s *Service) DecideApproval(ctx context.Context, spaceID, approvalID string, input DecideApprovalInput) (ApprovalRecord, error) {
	decision := strings.ToLower(strings.TrimSpace(input.Decision))
	if decision != "approved" && decision != "denied" {
		return ApprovalRecord{}, errors.New("审批决定必须是 approved 或 denied")
	}
	grantMode := strings.TrimSpace(input.GrantMode)
	if grantMode == "" {
		grantMode = "once"
	}
	if grantMode != "once" && grantMode != "session" && grantMode != "timed" {
		return ApprovalRecord{}, errors.New("审批授权模式必须是 once、session 或 timed")
	}
	now := time.Now().UTC()
	var approval ApprovalRecord
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND space_id = ?", approvalID, spaceID).First(&approval).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("审批不存在")
			}
			return fmt.Errorf("读取审批: %w", err)
		}
		if approval.Revision != input.ExpectedRevision {
			return fmt.Errorf("审批已变化，当前 revision=%d", approval.Revision)
		}
		if approval.Status != "pending" {
			return fmt.Errorf("审批状态为 %s，不能再次处理", approval.Status)
		}
		if !approval.ExpiresAt.After(now) {
			if err := tx.Model(&ApprovalRecord{}).Where("id = ?", approval.ID).Updates(map[string]any{"status": "expired", "revision": approval.Revision + 1, "updated_at": now}).Error; err != nil {
				return fmt.Errorf("标记审批过期: %w", err)
			}
			return errors.New("审批已经过期")
		}
		updates := map[string]any{
			"status": decision, "decision_reason": strings.TrimSpace(input.Reason),
			"decided_by": strings.TrimSpace(input.ActorID), "decided_at": now,
			"revision": approval.Revision + 1, "updated_at": now,
		}
		if decision == "approved" && grantMode == "timed" {
			minutes := input.TimedMinutes
			if minutes <= 0 {
				minutes = 60
			}
			if minutes > 1440 {
				minutes = 1440
			}
			until := now.Add(time.Duration(minutes) * time.Minute)
			updates["timed_grant_until"] = until
		}
		if err := tx.Model(&ApprovalRecord{}).Where("id = ? AND revision = ?", approval.ID, approval.Revision).Updates(updates).Error; err != nil {
			return fmt.Errorf("保存审批决定: %w", err)
		}
		record := ApprovalDecision{ID: newID("approvaldecision"), ApprovalID: approval.ID, Decision: decision, GrantMode: grantMode, Reason: strings.TrimSpace(input.Reason), ActorID: strings.TrimSpace(input.ActorID), CreatedAt: now}
		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("保存审批审计: %w", err)
		}
		return tx.Where("id = ?", approval.ID).First(&approval).Error
	})
	if err != nil {
		return ApprovalRecord{}, err
	}
	if s.approvalBroker != nil && s.approvalBroker.HasPending(approval.ID) {
		if err := s.approvalBroker.ResolveWithActor(approval.ID, decision == "approved", execution.ApprovalActor{
			ID:            input.ActorID,
			PrincipalType: "platform_user",
			AuthMethod:    "platform_session",
			SessionID:     input.ActorID,
		}); err != nil {
			return ApprovalRecord{}, fmt.Errorf("恢复工具审批等待: %w", err)
		}
	}
	if err := s.recordActivity(ctx, ActivityEvent{SpaceID: spaceID, AgentID: approval.AgentID, TeamID: approval.TeamID, ConversationID: approval.ConversationID, Kind: "approval", State: decision, Title: approval.ActionSummary, Detail: input.Reason}); err != nil {
		return ApprovalRecord{}, err
	}
	return approval, nil
}

func (s *Service) ResumeApproval(ctx context.Context, spaceID, approvalID string) (ApprovalRecord, error) {
	var approval ApprovalRecord
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND space_id = ?", approvalID, spaceID).First(&approval).Error; err != nil {
			return fmt.Errorf("读取审批: %w", err)
		}
		if approval.Status != "approved" {
			return errors.New("只有已批准的审批可以恢复")
		}
		now := time.Now().UTC()
		if err := tx.Model(&ApprovalRecord{}).Where("id = ?", approval.ID).Updates(map[string]any{"resumed_at": now, "updated_at": now}).Error; err != nil {
			return fmt.Errorf("恢复审批等待: %w", err)
		}
		return tx.Where("id = ?", approval.ID).First(&approval).Error
	})
	if err != nil {
		return ApprovalRecord{}, err
	}
	if err := s.recordActivity(ctx, ActivityEvent{SpaceID: spaceID, AgentID: approval.AgentID, TeamID: approval.TeamID, ConversationID: approval.ConversationID, Kind: "approval", State: "resumed", Title: "审批等待已恢复", Detail: approval.ActionSummary}); err != nil {
		return ApprovalRecord{}, err
	}
	return approval, nil
}

func (s *Service) CreateReconciliation(ctx context.Context, spaceID string, input CreateReconciliationInput) (ReconciliationCase, error) {
	if strings.TrimSpace(input.Reason) == "" {
		return ReconciliationCase{}, errors.New("待对账原因不能为空")
	}
	now := time.Now().UTC()
	item := ReconciliationCase{
		ID: newID("reconciliation"), SpaceID: spaceID, AgentID: input.AgentID,
		ApprovalID: input.ApprovalID, ToolCallID: input.ToolCallID, Status: "open",
		Reason: input.Reason, OriginalError: input.OriginalError, ResultHash: input.ResultHash,
		ActionJSON: "[]", Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&item).Error; err != nil {
		return ReconciliationCase{}, fmt.Errorf("创建待对账记录: %w", err)
	}
	if err := s.recordActivity(ctx, ActivityEvent{SpaceID: spaceID, AgentID: input.AgentID, Kind: "reconciliation", State: "open", Title: "外部操作结果需要人工对账", WaitingReason: input.Reason, Detail: input.OriginalError}); err != nil {
		return ReconciliationCase{}, err
	}
	return item, nil
}

func (s *Service) ListReconciliations(ctx context.Context, spaceID, status string, limit int) ([]ReconciliationCase, error) {
	query := s.db.WithContext(ctx).Where("space_id = ?", spaceID)
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", status)
	}
	items := make([]ReconciliationCase, 0)
	if err := query.Order("created_at DESC").Limit(trimLimit(limit, 100, 500)).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("读取待对账记录: %w", err)
	}
	return items, nil
}

func (s *Service) ResolveReconciliation(ctx context.Context, spaceID, caseID string, input ResolveReconciliationInput) (ReconciliationCase, error) {
	resolution := strings.TrimSpace(input.Resolution)
	if resolution == "" {
		return ReconciliationCase{}, errors.New("对账处理结果不能为空")
	}
	now := time.Now().UTC()
	result := s.db.WithContext(ctx).Model(&ReconciliationCase{}).Where("id = ? AND space_id = ? AND revision = ? AND status <> ?", caseID, spaceID, input.ExpectedRevision, "resolved").Updates(map[string]any{
		"status": "resolved", "resolution": resolution, "resolved_by": strings.TrimSpace(input.ActorID),
		"resolved_at": now, "updated_at": now, "revision": input.ExpectedRevision + 1,
	})
	if result.Error != nil {
		return ReconciliationCase{}, fmt.Errorf("对账处理: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ReconciliationCase{}, errors.New("待对账记录不存在或版本已变化")
	}
	var item ReconciliationCase
	if err := s.db.WithContext(ctx).Where("id = ?", caseID).First(&item).Error; err != nil {
		return ReconciliationCase{}, fmt.Errorf("读取对账结果: %w", err)
	}
	if err := s.recordActivity(ctx, ActivityEvent{SpaceID: spaceID, AgentID: item.AgentID, Kind: "reconciliation", State: "resolved", Title: "待对账记录已处理", Detail: resolution}); err != nil {
		return ReconciliationCase{}, err
	}
	return item, nil
}

func (s *Service) ApprovalArguments(record ApprovalRecord) (map[string]any, error) {
	var value map[string]any
	if err := json.Unmarshal([]byte(record.ArgumentsJSON), &value); err != nil {
		return nil, fmt.Errorf("解析审批参数: %w", err)
	}
	return value, nil
}
