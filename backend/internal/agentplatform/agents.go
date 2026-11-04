package agentplatform

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AgentDetail struct {
	Profile  AgentProfile    `json:"profile"`
	Revision AgentRevision   `json:"revision"`
	Release  *AgentRelease   `json:"release,omitempty"`
	History  []AgentRevision `json:"history,omitempty"`
}

func (s *Service) ListAgents(ctx context.Context, spaceID string, includeDisabled bool, limit int) ([]AgentProfile, error) {
	query := s.db.WithContext(ctx).Where("space_id = ? AND deleted_at IS NULL", strings.TrimSpace(spaceID))
	if !includeDisabled {
		query = query.Where("status = ?", "active")
	}
	limit = trimLimit(limit, 100, 500)
	agents := make([]AgentProfile, 0)
	if err := query.Order("updated_at DESC").Limit(limit).Find(&agents).Error; err != nil {
		return nil, fmt.Errorf("读取 Agent 列表: %w", err)
	}
	return agents, nil
}

func (s *Service) GetAgent(ctx context.Context, spaceID, agentID string) (AgentDetail, error) {
	var agent AgentProfile
	if err := s.db.WithContext(ctx).Where("id = ? AND space_id = ? AND deleted_at IS NULL", agentID, spaceID).First(&agent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AgentDetail{}, errors.New("Agent 不存在")
		}
		return AgentDetail{}, fmt.Errorf("读取 Agent: %w", err)
	}
	var revision AgentRevision
	if err := s.db.WithContext(ctx).Where("id = ? AND space_id = ?", agent.CurrentRevisionID, spaceID).First(&revision).Error; err != nil {
		return AgentDetail{}, fmt.Errorf("读取 Agent 当前版本: %w", err)
	}
	releases := make([]AgentRelease, 0)
	if err := s.db.WithContext(ctx).Where("agent_id = ? AND space_id = ?", agent.ID, spaceID).Order("created_at DESC").Limit(20).Find(&releases).Error; err != nil {
		return AgentDetail{}, fmt.Errorf("读取 Agent 发布记录: %w", err)
	}
	history := make([]AgentRevision, 0)
	if err := s.db.WithContext(ctx).Where("agent_id = ? AND space_id = ?", agent.ID, spaceID).Order("revision DESC").Limit(50).Find(&history).Error; err != nil {
		return AgentDetail{}, fmt.Errorf("读取 Agent 版本历史: %w", err)
	}
	detail := AgentDetail{Profile: agent, Revision: revision, History: history}
	for index := range releases {
		if releases[index].ID == agent.CurrentReleaseID {
			release := releases[index]
			detail.Release = &release
			break
		}
	}
	return detail, nil
}

func (s *Service) SeedAgentTemplates(ctx context.Context) error {
	defaults := []AgentTemplate{
		{ID: "builtin-companion", Name: "陪伴 Agent", Category: "companion", Archetype: "companion", Description: "具备长期记忆、主动关怀和语音交互的私人 Agent。", ConfigJSON: `{"identity":{"kind":"companion"},"model":{},"memory":{"scope":"agent"},"permissions":{"mode":"approval"},"runtime":{"provider":"local"}}`, PublisherID: "com.amitia", TrustLevel: "official", Builtin: true, SchemaVersion: 1},
		{ID: "builtin-worker", Name: "执行 Agent", Category: "productivity", Archetype: "worker", Description: "拥有独立电脑、应用授权和任务账本的执行型 Agent。", ConfigJSON: `{"identity":{"kind":"worker"},"model":{},"memory":{"scope":"agent"},"permissions":{"mode":"approval"},"runtime":{"provider":"oci"}}`, PublisherID: "com.amitia", TrustLevel: "official", Builtin: true, SchemaVersion: 1},
		{ID: "builtin-team-lead", Name: "团队负责人", Category: "team", Archetype: "team_lead", Description: "负责团队路由、任务交接和统一审计的负责人 Agent。", ConfigJSON: `{"identity":{"kind":"team_lead"},"model":{},"memory":{"scope":"team"},"permissions":{"mode":"approval"},"runtime":{"provider":"oci"}}`, PublisherID: "com.amitia", TrustLevel: "official", Builtin: true, SchemaVersion: 1},
	}
	now := time.Now().UTC()
	for index := range defaults {
		defaults[index].CreatedAt = now
		defaults[index].UpdatedAt = now
		if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&defaults[index]).Error; err != nil {
			return fmt.Errorf("初始化 Agent 模板: %w", err)
		}
	}
	teamTemplates := []TeamTemplate{
		{ID: "builtin-research-team", Name: "研究团队", Description: "负责人拆解问题，研究员检索，编辑完成核验。", SharedMemory: true, SharedWorkspace: true, RoutingPolicyJSON: `{"strategy":"lead","handoff":"required"}`, Builtin: true},
		{ID: "builtin-operations-team", Name: "运营团队", Description: "负责人分配任务，执行 Agent 使用应用和电脑，审计 Agent 检查结果。", SharedMemory: true, SharedWorkspace: true, RoutingPolicyJSON: `{"strategy":"capability","handoff":"required"}`, Builtin: true},
		{ID: "builtin-engineering-team", Name: "工程团队", Description: "规划、编码、测试和评审角色协作处理代码任务。", SharedMemory: true, SharedWorkspace: true, RoutingPolicyJSON: `{"strategy":"lead","worktree":"isolated"}`, Builtin: true},
	}
	for index := range teamTemplates {
		teamTemplates[index].CreatedAt = now
		teamTemplates[index].UpdatedAt = now
		if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&teamTemplates[index]).Error; err != nil {
			return fmt.Errorf("初始化团队模板: %w", err)
		}
	}
	return nil
}
