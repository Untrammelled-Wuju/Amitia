package chat

import (
	"context"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/mindruntime"
)

func (s *service) SetUsageStore(store *mindruntime.ModelUsageStore) { s.usageStore = store }

func (s *service) meterModel(ctx context.Context, cfg *ModelConfig) (func(*ModelResult, error) error, error) {
	if s.usageStore == nil {
		if _, limited := mindruntime.OwnedModelBudgetFromContext(ctx); limited {
			return nil, errors.New("multi_agent: durable model-token usage store is unavailable")
		}
		return func(*ModelResult, error) error { return nil }, nil
	}
	record := mindruntime.ModelUsageRecord{Model: cfg.ModelName, Provider: cfg.APIType, Source: "system"}
	if budget, limited := mindruntime.OwnedModelBudgetFromContext(ctx); limited {
		record.SpaceID = budget.SpaceID
		if cfg.ContextWindow <= 0 {
			return nil, errors.New("multi_agent: model context window is required for a conservative token reservation")
		}
		if ceiling := ownedModelOutputCeiling(cfg); ceiling < 1 || ceiling > int64(cfg.ContextWindow) {
			return nil, errors.New("multi_agent: model output ceiling exceeds reserved context window")
		}
		record.ReservedTokens = int64(cfg.ContextWindow)
		if budget.CostLimitMicroUSD > 0 {
			if cfg.BudgetRateVersion == "" || cfg.BudgetInputRate < 1 || cfg.BudgetOutputRate < 1 || cfg.BudgetInputRate > 100000000 || cfg.BudgetOutputRate > 100000000 {
				return nil, errors.New("multi_agent: verified model cost-rate ceiling is not configured")
			}
			cost, ok := mindruntime.CeilingOwnedModelCost(int64(cfg.ContextWindow), ownedModelOutputCeiling(cfg), cfg.BudgetInputRate, cfg.BudgetOutputRate)
			if !ok {
				return nil, errors.New("multi_agent: model cost reservation overflow")
			}
			record.ReservedCostMicroUSD = cost
			record.BudgetInputRate = cfg.BudgetInputRate
			record.BudgetOutputRate = cfg.BudgetOutputRate
			record.BudgetRateVersion = cfg.BudgetRateVersion
		}
	}
	if actor, ok := auth.FromContext(ctx); ok {
		record.SpaceID = actor.SpaceID.String()
		record.DeviceID = actor.DeviceID.String()
		record.RequestID = actor.RequestID
	}
	if scope, ok := coordination.FromContext(ctx); ok {
		record.SpaceID = scope.SpaceID
		record.DeviceID = scope.InitiatorDeviceID
		record.OwnerID = scope.ResourceOwnerID
		record.RoleID = scope.RoleID
		record.RequestID = scope.RequestID
		record.Source = "web"
	}
	if ingress, ok := business.IngressFromContext(ctx); ok {
		if ingress.Channel != "" {
			record.Source = ingress.Channel
		}
		if ingress.Source != "" {
			record.Source = ingress.Source
		}
	}
	if budget, limited := mindruntime.OwnedModelBudgetFromContext(ctx); limited {
		if record.SpaceID != "" && record.SpaceID != budget.SpaceID {
			return nil, errors.New("multi_agent: model metering scope conflicts with authorized parent")
		}
		record.SpaceID = budget.SpaceID
	}
	id, err := s.usageStore.Begin(ctx, record)
	if err != nil {
		return nil, err
	}
	return func(result *ModelResult, callErr error) error {
		usage := mindruntime.ModelUsageRecord{Status: "succeeded"}
		if callErr != nil {
			usage.Status = "failed"
			if errors.Is(callErr, context.Canceled) || ctx.Err() != nil {
				usage.Status = "cancelled"
			}
		}
		if result != nil {
			u := result.Usage
			usage.InputTokens = int64(max(0, u.InputTokens))
			usage.OutputTokens = int64(max(0, u.OutputTokens))
			usage.CachedTokens = int64(max(0, u.CachedInputTokens))
			usage.TotalTokens = int64(max(0, u.TotalTokens))
			if usage.TotalTokens == 0 {
				usage.TotalTokens = usage.InputTokens + usage.OutputTokens
			}
			usage.UsageReported = u.Reported
			if _, limited := mindruntime.OwnedModelBudgetFromContext(ctx); !limited && usage.TotalTokens > 0 {
				usage.UsageReported = true
			}
		}
		save, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return s.usageStore.Finish(save, id, usage)
	}, nil
}

type meteringEventSink struct {
	next  ModelEventSink
	usage *ModelUsage
}

func (s *meteringEventSink) Emit(ctx context.Context, event ModelEvent) error {
	if event.Usage != nil {
		copy := *event.Usage
		s.usage = &copy
	}
	return s.next.Emit(ctx, event)
}
