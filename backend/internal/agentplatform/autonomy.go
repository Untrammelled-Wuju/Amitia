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

type UpsertAutonomyPolicyInput struct {
	Level            int      `json:"level"`
	WatchOnly        bool     `json:"watchOnly"`
	AutoLowRisk      bool     `json:"autoLowRisk"`
	ExternalApproval bool     `json:"externalApproval"`
	MaxActionsPerDay int      `json:"maxActionsPerDay"`
	MaxCostMicroUSD  int64    `json:"maxCostMicroUsd"`
	QuietHours       string   `json:"quietHours"`
	EscalationTarget string   `json:"escalationTarget"`
	Triggers         []string `json:"triggers"`
	ExpectedRevision int64    `json:"expectedRevision"`
}

func (s *Service) GetAutonomyPolicy(ctx context.Context, spaceID, agentID string) (AgentAutonomyPolicy, error) {
	var policy AgentAutonomyPolicy
	err := s.db.WithContext(ctx).Where("space_id = ? AND agent_id = ?", spaceID, agentID).First(&policy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		now := time.Now().UTC()
		return AgentAutonomyPolicy{
			ID: newID("autonomy"), SpaceID: spaceID, AgentID: agentID, Level: 1,
			WatchOnly: true, ExternalApproval: true, MaxActionsPerDay: 20,
			TriggersJSON: "[]", Revision: 0, CreatedAt: now, UpdatedAt: now,
		}, nil
	}
	if err != nil {
		return AgentAutonomyPolicy{}, fmt.Errorf("读取 Agent 自治策略: %w", err)
	}
	return policy, nil
}

func (s *Service) UpsertAutonomyPolicy(ctx context.Context, spaceID, agentID string, input UpsertAutonomyPolicyInput) (AgentAutonomyPolicy, error) {
	if input.Level < 0 || input.Level > 4 {
		return AgentAutonomyPolicy{}, errors.New("自治等级必须在 0 到 4 之间")
	}
	if input.MaxActionsPerDay <= 0 {
		input.MaxActionsPerDay = 20
	}
	triggersJSON, _, err := hashJSON(input.Triggers)
	if err != nil {
		return AgentAutonomyPolicy{}, err
	}
	now := time.Now().UTC()
	var policy AgentAutonomyPolicy
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id = ? AND agent_id = ?", spaceID, agentID).First(&policy).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			policy = AgentAutonomyPolicy{
				ID: newID("autonomy"), SpaceID: spaceID, AgentID: agentID, Revision: 0,
				CreatedAt: now,
			}
		} else if err != nil {
			return fmt.Errorf("读取 Agent 自治策略: %w", err)
		} else if policy.Revision != input.ExpectedRevision {
			return fmt.Errorf("Agent 自治策略已变化，当前 revision=%d", policy.Revision)
		}
		policy.Level = input.Level
		policy.WatchOnly = input.WatchOnly
		policy.AutoLowRisk = input.AutoLowRisk
		policy.ExternalApproval = input.ExternalApproval
		policy.MaxActionsPerDay = input.MaxActionsPerDay
		policy.MaxCostMicroUSD = input.MaxCostMicroUSD
		policy.QuietHours = strings.TrimSpace(input.QuietHours)
		policy.EscalationTarget = strings.TrimSpace(input.EscalationTarget)
		policy.TriggersJSON = triggersJSON
		policy.UpdatedAt = now
		if policy.Revision <= 0 {
			policy.Revision = 1
		} else {
			policy.Revision++
		}
		return tx.Save(&policy).Error
	})
	if err != nil {
		return AgentAutonomyPolicy{}, err
	}
	if err := s.recordActivity(ctx, ActivityEvent{SpaceID: spaceID, AgentID: agentID, Kind: "autonomy", State: "updated", Title: "Agent 自治策略已更新", Detail: fmt.Sprintf("level=%d", policy.Level), PayloadJSON: mustJSON(policy)}); err != nil {
		return AgentAutonomyPolicy{}, err
	}
	return policy, nil
}
