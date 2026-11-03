package agentplatform

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type ActivityFilter struct {
	AgentID        string
	TeamID         string
	ConversationID string
	Kind           string
	State          string
	Limit          int
}

type ActivitySummary struct {
	Running           int64 `json:"running"`
	Waiting           int64 `json:"waiting"`
	Failed            int64 `json:"failed"`
	Completed         int64 `json:"completed"`
	CostMicroUSD      int64 `json:"costMicroUsd"`
	AverageDurationMS int64 `json:"averageDurationMs"`
}

func (s *Service) ListActivity(ctx context.Context, spaceID string, filter ActivityFilter) ([]ActivityEvent, ActivitySummary, error) {
	query := s.db.WithContext(ctx).Where("space_id = ?", spaceID)
	if strings.TrimSpace(filter.AgentID) != "" {
		query = query.Where("agent_id = ?", filter.AgentID)
	}
	if strings.TrimSpace(filter.TeamID) != "" {
		query = query.Where("team_id = ?", filter.TeamID)
	}
	if strings.TrimSpace(filter.ConversationID) != "" {
		query = query.Where("conversation_id = ?", filter.ConversationID)
	}
	if strings.TrimSpace(filter.Kind) != "" {
		query = query.Where("kind = ?", filter.Kind)
	}
	if strings.TrimSpace(filter.State) != "" {
		query = query.Where("state = ?", filter.State)
	}
	events := make([]ActivityEvent, 0)
	if err := query.Order("occurred_at DESC").Limit(trimLimit(filter.Limit, 100, 1000)).Find(&events).Error; err != nil {
		return nil, ActivitySummary{}, fmt.Errorf("读取 Agent 活动: %w", err)
	}
	var summary ActivitySummary
	row := map[string]any{}
	if err := s.db.WithContext(ctx).Model(&ActivityEvent{}).Select(`
		COALESCE(SUM(CASE WHEN state IN ('running','active','started') THEN 1 ELSE 0 END),0) AS running,
		COALESCE(SUM(CASE WHEN state IN ('pending','waiting','approval','blocked') THEN 1 ELSE 0 END),0) AS waiting,
		COALESCE(SUM(CASE WHEN state IN ('failed','error') THEN 1 ELSE 0 END),0) AS failed,
		COALESCE(SUM(CASE WHEN state IN ('completed','passed','resolved') THEN 1 ELSE 0 END),0) AS completed,
		COALESCE(SUM(cost_micro_usd),0) AS cost_micro_usd,
		COALESCE(AVG(CASE WHEN duration_ms > 0 THEN duration_ms END),0) AS average_duration_ms
	`).Where("space_id = ?", spaceID).Scan(&row).Error; err != nil {
		return nil, ActivitySummary{}, fmt.Errorf("汇总 Agent 活动: %w", err)
	}
	summary.Running = numericValue(row["running"])
	summary.Waiting = numericValue(row["waiting"])
	summary.Failed = numericValue(row["failed"])
	summary.Completed = numericValue(row["completed"])
	summary.CostMicroUSD = numericValue(row["cost_micro_usd"])
	summary.AverageDurationMS = numericValue(row["average_duration_ms"])
	return events, summary, nil
}

func (s *Service) GetActivity(ctx context.Context, spaceID, eventID string) (ActivityEvent, error) {
	var event ActivityEvent
	if err := s.db.WithContext(ctx).Where("id = ? AND space_id = ?", eventID, spaceID).First(&event).Error; err != nil {
		return ActivityEvent{}, fmt.Errorf("读取 Agent 活动详情: %w", err)
	}
	return event, nil
}

func (s *Service) RecordExecutionActivity(ctx context.Context, event ActivityEvent) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("Agent 活动服务不可用")
	}
	if event.ID == "" {
		event.ID = newID("activity")
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	if event.PayloadJSON == "" {
		event.PayloadJSON = "{}"
	}
	if err := s.db.WithContext(ctx).Create(&event).Error; err != nil {
		return fmt.Errorf("保存 Agent 活动: %w", err)
	}
	return nil
}

func (s *Service) CountActivity(ctx context.Context, spaceID, agentID, kind string, since time.Time) (int64, error) {
	var count int64
	query := s.db.WithContext(ctx).Model(&ActivityEvent{}).Where("space_id = ? AND kind = ? AND occurred_at >= ?", spaceID, kind, since)
	if strings.TrimSpace(agentID) != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("统计 Agent 活动: %w", err)
	}
	return count, nil
}
