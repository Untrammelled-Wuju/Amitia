package agentplatform

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type ModelUsageActivityInput struct {
	UsageID        string
	CharacterID    string
	Model          string
	Provider       string
	Status         string
	InputTokens    int64
	OutputTokens   int64
	TotalTokens    int64
	UsageReported  bool
	ReservedTokens int64
	InputRate      int64
	OutputRate     int64
	ReservedCost   int64
}

type ModelUsageTotals struct {
	RequestID    string `json:"requestId"`
	Calls        int64  `json:"calls"`
	InputTokens  int64  `json:"inputTokens"`
	OutputTokens int64  `json:"outputTokens"`
	TotalTokens  int64  `json:"totalTokens"`
	CostMicroUSD int64  `json:"costMicroUsd"`
}

func (s *Service) ModelUsageTotalsByRequest(ctx context.Context, spaceID, requestID, roleID string) (ModelUsageTotals, error) {
	result := ModelUsageTotals{RequestID: strings.TrimSpace(requestID)}
	if result.RequestID == "" {
		return result, nil
	}
	query := s.db.WithContext(ctx).Table("runtime_model_usage").Where("space_id = ? AND request_id = ?", spaceID, result.RequestID)
	if strings.TrimSpace(roleID) != "" {
		query = query.Where("role_id = ?", roleID)
	}
	var aggregate struct {
		Calls        int64
		InputTokens  int64
		OutputTokens int64
		TotalTokens  int64
	}
	if err := query.Select(`
		COUNT(*) AS calls,
		COALESCE(SUM(input_tokens), 0) AS input_tokens,
		COALESCE(SUM(output_tokens), 0) AS output_tokens,
		COALESCE(SUM(total_tokens), 0) AS total_tokens
	`).Scan(&aggregate).Error; err != nil {
		return ModelUsageTotals{}, fmt.Errorf("汇总 Agent 模型用量: %w", err)
	}
	result.Calls = aggregate.Calls
	result.InputTokens = aggregate.InputTokens
	result.OutputTokens = aggregate.OutputTokens
	result.TotalTokens = aggregate.TotalTokens
	roleFilter := ""
	costArgs := []any{spaceID, spaceID, result.RequestID}
	if strings.TrimSpace(roleID) != "" {
		roleFilter = " AND role_id = ?"
		costArgs = append(costArgs, roleID)
	}
	var costRows []struct {
		CostMicroUSD int64
	}
	costQuery := `
		SELECT COALESCE(SUM(cost_micro_usd), 0) AS cost_micro_usd
		FROM activity_events
		WHERE space_id = ? AND kind = 'model.usage'
			AND json_extract(payload_json, '$.usageId') IN (
				SELECT id FROM runtime_model_usage
				WHERE space_id = ? AND request_id = ?` + roleFilter + `
			)
	`
	if err := s.db.WithContext(ctx).Raw(costQuery, costArgs...).Scan(&costRows).Error; err != nil {
		return ModelUsageTotals{}, fmt.Errorf("汇总 Agent 模型费用: %w", err)
	}
	for _, row := range costRows {
		result.CostMicroUSD += row.CostMicroUSD
	}
	return result, nil
}

func (s *Service) ModelUsageTotalsByWindow(ctx context.Context, spaceID, roleID string, startedAt, completedAt time.Time) (ModelUsageTotals, error) {
	result := ModelUsageTotals{}
	if startedAt.IsZero() || completedAt.IsZero() || completedAt.Before(startedAt) {
		return result, errors.New("Agent 模型用量时间范围无效")
	}
	startValue := startedAt.UTC().Format(time.RFC3339Nano)
	endValue := completedAt.UTC().Format(time.RFC3339Nano)
	query := s.db.WithContext(ctx).Table("runtime_model_usage").Where(
		"space_id = ? AND role_id = ? AND completed_at >= ? AND completed_at <= ?",
		spaceID, roleID, startValue, endValue,
	)
	var aggregate struct {
		Calls        int64
		InputTokens  int64
		OutputTokens int64
		TotalTokens  int64
	}
	if err := query.Select(`
		COUNT(*) AS calls,
		COALESCE(SUM(input_tokens), 0) AS input_tokens,
		COALESCE(SUM(output_tokens), 0) AS output_tokens,
		COALESCE(SUM(total_tokens), 0) AS total_tokens
	`).Scan(&aggregate).Error; err != nil {
		return ModelUsageTotals{}, fmt.Errorf("按时间汇总 Agent 模型用量: %w", err)
	}
	result.Calls = aggregate.Calls
	result.InputTokens = aggregate.InputTokens
	result.OutputTokens = aggregate.OutputTokens
	result.TotalTokens = aggregate.TotalTokens
	var costRows []struct {
		CostMicroUSD int64
	}
	costQuery := `
		SELECT COALESCE(SUM(cost_micro_usd), 0) AS cost_micro_usd
		FROM activity_events
		WHERE space_id = ? AND kind = 'model.usage'
			AND json_extract(payload_json, '$.usageId') IN (
				SELECT id FROM runtime_model_usage
				WHERE space_id = ? AND role_id = ? AND completed_at >= ? AND completed_at <= ?
			)
	`
	if err := s.db.WithContext(ctx).Raw(costQuery, spaceID, spaceID, roleID, startValue, endValue).Scan(&costRows).Error; err != nil {
		return ModelUsageTotals{}, fmt.Errorf("按时间汇总 Agent 模型费用: %w", err)
	}
	for _, row := range costRows {
		result.CostMicroUSD += row.CostMicroUSD
	}
	return result, nil
}

func (s *Service) RecordModelUsageActivity(ctx context.Context, spaceID string, input ModelUsageActivityInput) error {
	if s == nil || s.db == nil {
		return errors.New("Agent 模型用量服务不可用")
	}
	agentID := ""
	teamID := ""
	if strings.TrimSpace(input.CharacterID) != "" {
		var agent AgentProfile
		err := s.db.WithContext(ctx).Where("space_id = ? AND character_id = ? AND deleted_at IS NULL", spaceID, input.CharacterID).First(&agent).Error
		if err == nil {
			agentID, teamID = agent.ID, agent.TeamID
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("读取模型用量 Agent: %w", err)
		}
	}
	inputTokens := maxInt64(input.InputTokens, 0)
	outputTokens := maxInt64(input.OutputTokens, 0)
	totalTokens := input.TotalTokens
	if totalTokens <= 0 {
		totalTokens = inputTokens + outputTokens
	}
	if !input.UsageReported && input.ReservedTokens > totalTokens {
		inputTokens = input.ReservedTokens
		outputTokens = 0
		totalTokens = input.ReservedTokens
	}
	cost := input.ReservedCost
	if input.InputRate > 0 || input.OutputRate > 0 {
		rate := input.InputRate
		if input.OutputRate > rate {
			rate = input.OutputRate
		}
		uncategorized := totalTokens - inputTokens - outputTokens
		if uncategorized < 0 {
			uncategorized = 0
		}
		cost = (inputTokens*input.InputRate + outputTokens*input.OutputRate + uncategorized*rate + 999999) / 1000000
	}
	if cost < 0 {
		cost = 0
	}
	now := time.Now().UTC()
	detail := fmt.Sprintf("%s/%s · %d tokens · %d μUSD · %s", input.Provider, input.Model, totalTokens, cost, input.Status)
	event := ActivityEvent{
		ID:      stableID("activity", "model:"+input.UsageID),
		SpaceID: spaceID, AgentID: agentID, TeamID: teamID, Kind: "model.usage",
		State: firstValue(input.Status, "completed"), Title: "Agent 模型调用",
		Detail: detail, CostMicroUSD: cost, OccurredAt: now,
		PayloadJSON: mustJSON(map[string]any{
			"usageId": input.UsageID, "model": input.Model, "provider": input.Provider,
			"inputTokens": inputTokens, "outputTokens": outputTokens, "totalTokens": totalTokens,
			"usageReported": input.UsageReported, "reservedTokens": input.ReservedTokens,
		}),
	}
	applyActivityActor(ctx, &event)
	if err := s.db.WithContext(ctx).Where("id = ?", event.ID).FirstOrCreate(&event).Error; err != nil {
		return fmt.Errorf("记录 Agent 模型活动: %w", err)
	}
	return nil
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func firstValue(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}
