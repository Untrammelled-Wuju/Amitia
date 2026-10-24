package mindruntime

import (
	"context"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"gorm.io/gorm"
)

type OwnedModelBudget struct {
	SpaceID             string
	RoleID              string
	OwnerID             string
	ParentInteractionID string
	CoordinationID      string
	AssignmentID        string
	TokenLimit          int64
	CostLimitMicroUSD   int64
}

type ownedModelBudgetKey struct{}

func WithOwnedModelBudget(ctx context.Context, budget OwnedModelBudget) context.Context {
	return context.WithValue(ctx, ownedModelBudgetKey{}, budget)
}

func OwnedModelBudgetFromContext(ctx context.Context) (OwnedModelBudget, bool) {
	value, ok := ctx.Value(ownedModelBudgetKey{}).(OwnedModelBudget)
	return value, ok
}

var ErrOwnedModelBudgetExceeded = errors.New("multi_agent: shared total-token budget exhausted")

func (s *ModelUsageStore) reserveOwnedModelTokens(ctx context.Context, record *ModelUsageRecord, budget OwnedModelBudget) error {
	if s == nil || s.db == nil || record == nil || budget.SpaceID == "" || budget.RoleID == "" || budget.OwnerID == "" || budget.ParentInteractionID == "" || budget.CoordinationID == "" || budget.AssignmentID == "" || (budget.TokenLimit < 1 && budget.CostLimitMicroUSD < 1) || budget.TokenLimit > 100000000 || budget.CostLimitMicroUSD < 0 || budget.CostLimitMicroUSD > 100000000000 || record.SpaceID != budget.SpaceID || budget.SpaceID != s.SpaceID || record.ReservedTokens < 1 || (budget.TokenLimit > 0 && record.ReservedTokens > budget.TokenLimit) || (budget.CostLimitMicroUSD > 0 && (record.ReservedCostMicroUSD < 1 || record.ReservedCostMicroUSD > budget.CostLimitMicroUSD || record.BudgetRateVersion == "" || record.BudgetInputRate < 1 || record.BudgetOutputRate < 1)) {
		return ErrOwnedModelBudgetExceeded
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	if s.db.Dialector.Name() != "sqlite" {
		return errors.New("multi_agent: model-token atomic admission requires verified shared sqlite storage")
	}
	record.OwnerID = budget.OwnerID
	record.RoleID = budget.RoleID
	record.ParentInteractionID = budget.ParentInteractionID
	record.CoordinationID = budget.CoordinationID
	record.AssignmentID = budget.AssignmentID
	record.BudgetCostLimitMicroUSD = budget.CostLimitMicroUSD
	var result *gorm.DB
	err := coordination.CommitCurrent(ctx, func() error {
		parentClause := ""
		parentArgs := []any{}
		if limits, guarded := durableOwnedBudgetParent(ctx); guarded {
			if limits.ChildInteractionID == "" || limits.Tokens != budget.TokenLimit || limits.CostMicroUSD != budget.CostLimitMicroUSD {
				return ErrOwnedModelBudgetExceeded
			}
			parentClause = " AND " + ownedBudgetParentCondition
			parentArgs = ownedBudgetParentArguments(budget.SpaceID, budget.RoleID, budget.OwnerID, budget.ParentInteractionID, budget.CoordinationID, budget.AssignmentID, limits.ChildInteractionID, limits.ToolCalls, limits.Tokens, limits.CostMicroUSD)
		}
		query := `INSERT INTO runtime_model_usage
        (id,space_id,device_id,owner_id,role_id,request_id,model,provider,source,status,started_at,coordination_id,parent_interaction_id,assignment_id,reserved_tokens,reserved_cost_micro_usd,budget_cost_limit_micro_usd,budget_input_rate,budget_output_rate,budget_rate_version)
        SELECT ?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?
        WHERE (?=0 OR COALESCE((SELECT SUM(reserved_tokens) FROM runtime_model_usage WHERE space_id=? AND coordination_id=?),0) <= ?)
          AND (?=0 OR COALESCE((SELECT SUM(reserved_cost_micro_usd) FROM runtime_model_usage WHERE space_id=? AND coordination_id=?),0) <= ?)
          AND NOT EXISTS (SELECT 1 FROM runtime_model_usage WHERE space_id=? AND coordination_id=? AND (COALESCE(owner_id,'')<>? OR COALESCE(role_id,'')<>? OR COALESCE(parent_interaction_id,'')<>?))
          AND NOT EXISTS (SELECT 1 FROM runtime_owned_tool_calls WHERE space_id=? AND coordination_id=? AND (COALESCE(owner_id,'')<>? OR COALESCE(role_id,'')<>? OR COALESCE(parent_interaction_id,'')<>?))` + parentClause
		args := []any{
			record.ID, record.SpaceID, record.DeviceID, record.OwnerID, record.RoleID, record.RequestID, record.Model, record.Provider, record.Source, record.Status, record.StartedAt, record.CoordinationID, record.ParentInteractionID, record.AssignmentID, record.ReservedTokens, record.ReservedCostMicroUSD, record.BudgetCostLimitMicroUSD, record.BudgetInputRate, record.BudgetOutputRate, record.BudgetRateVersion,
			budget.TokenLimit, record.SpaceID, record.CoordinationID, budget.TokenLimit - record.ReservedTokens,
			budget.CostLimitMicroUSD, record.SpaceID, record.CoordinationID, budget.CostLimitMicroUSD - record.ReservedCostMicroUSD,
			record.SpaceID, record.CoordinationID, budget.OwnerID, budget.RoleID, budget.ParentInteractionID,
			record.SpaceID, record.CoordinationID, budget.OwnerID, budget.RoleID, budget.ParentInteractionID}
		result = s.db.WithContext(ctx).Exec(query, append(args, parentArgs...)...)
		return result.Error
	})
	if err != nil {
		return err
	}
	if result.RowsAffected != 1 {
		return ErrOwnedModelBudgetExceeded
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		settlement, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		marked := s.db.WithContext(settlement).Model(&ModelUsageRecord{}).Where("id=? AND status='running'", record.ID).Updates(map[string]any{"status": "interrupted", "completed_at": time.Now().UTC().Format(time.RFC3339Nano)})
		if marked.Error != nil {
			return errors.Join(err, marked.Error)
		}
		if marked.RowsAffected != 1 {
			return errors.Join(err, errors.New("multi_agent: rejected model reservation could not be marked interrupted"))
		}
		return err
	}
	return nil
}

func (s *ModelUsageStore) OwnedModelTokenUsage(ctx context.Context, spaceID, coordinationID string) (int64, error) {
	if s == nil || s.db == nil || spaceID == "" || spaceID != s.SpaceID || coordinationID == "" {
		return 0, errors.New("multi_agent: missing model-token accounting scope")
	}
	var usage int64
	err := s.db.WithContext(ctx).Raw("SELECT COALESCE(SUM(reserved_tokens),0) FROM runtime_model_usage WHERE space_id=? AND coordination_id=?", spaceID, coordinationID).Scan(&usage).Error
	return usage, err
}

func validOwnedModelSettlement(usage ModelUsageRecord) bool {
	return usage.Status == "succeeded" && trustworthyOwnedTokenUsage(usage.UsageReported, usage.InputTokens, usage.OutputTokens, usage.CachedTokens, usage.TotalTokens)
}

func (s *ModelUsageStore) OwnedModelCostUsage(ctx context.Context, spaceID, coordinationID string) (int64, error) {
	if s == nil || s.db == nil || spaceID == "" || spaceID != s.SpaceID || coordinationID == "" {
		return 0, errors.New("multi_agent: missing model-cost accounting scope")
	}
	var occupancy int64
	err := s.db.WithContext(ctx).Raw("SELECT COALESCE(SUM(reserved_cost_micro_usd),0) FROM runtime_model_usage WHERE space_id=? AND coordination_id=?", spaceID, coordinationID).Scan(&occupancy).Error
	return occupancy, err
}
