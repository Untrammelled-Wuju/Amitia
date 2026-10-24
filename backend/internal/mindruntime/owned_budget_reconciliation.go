package mindruntime

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

var ErrOwnedBudgetInspectionScope = errors.New("multi_agent: budget reconciliation requires authorized owner, role, parent and bounded page size")

type OwnedBudgetReconciliationScope struct {
	SpaceID             string
	RoleID              string
	OwnerID             string
	ParentInteractionID string
	CoordinationID      string
}

func (s *ModelUsageStore) validateOwnedBudgetInspection(ctx context.Context, scope OwnedBudgetReconciliationScope, limit int) error {
	if s == nil || s.db == nil || scope.SpaceID == "" || scope.SpaceID != s.SpaceID || strings.TrimSpace(scope.RoleID) == "" || strings.TrimSpace(scope.OwnerID) == "" || strings.TrimSpace(scope.ParentInteractionID) == "" || strings.TrimSpace(scope.CoordinationID) == "" || limit < 1 || limit > 200 {
		return ErrOwnedBudgetInspectionScope
	}
	authority, present := coordination.FromContext(ctx)
	if !present || authority.SpaceID != scope.SpaceID || authority.RoleID != scope.RoleID || authority.ResourceOwnerID != scope.OwnerID {
		return ErrOwnedBudgetInspectionScope
	}
	return coordination.ValidateCurrent(ctx)
}

func (s *ModelUsageStore) PendingOwnedModelReconciliation(ctx context.Context, scope OwnedBudgetReconciliationScope, limit int) ([]ModelUsageRecord, error) {
	return s.PendingOwnedModelReconciliationPage(ctx, scope, 0, limit)
}

func (s *ModelUsageStore) PendingOwnedModelReconciliationPage(ctx context.Context, scope OwnedBudgetReconciliationScope, afterSequence int64, limit int) ([]ModelUsageRecord, error) {
	if afterSequence < 0 {
		return nil, ErrOwnedBudgetInspectionScope
	}
	if err := s.validateOwnedBudgetInspection(ctx, scope, limit); err != nil {
		return nil, err
	}
	var records []ModelUsageRecord
	err := s.db.WithContext(ctx).Where("space_id=? AND role_id=? AND owner_id=? AND parent_interaction_id=? AND coordination_id=? AND sequence>? AND (status='running' OR status='interrupted' OR status='failed' OR status='cancelled' OR usage_reported=0 OR budget_overrun=1)", scope.SpaceID, scope.RoleID, scope.OwnerID, scope.ParentInteractionID, scope.CoordinationID, afterSequence).Order("sequence ASC").Limit(limit).Find(&records).Error
	if err != nil {
		return nil, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *ModelUsageStore) ReservedOwnedToolReconciliation(ctx context.Context, scope OwnedBudgetReconciliationScope, limit int) ([]OwnedToolCallRecord, error) {
	return s.ReservedOwnedToolReconciliationPage(ctx, scope, time.Time{}, "", limit)
}

func (s *ModelUsageStore) ReservedOwnedToolReconciliationPage(ctx context.Context, scope OwnedBudgetReconciliationScope, afterCreatedAt time.Time, afterEffectID string, limit int) ([]OwnedToolCallRecord, error) {
	if afterCreatedAt.IsZero() != (afterEffectID == "") || len(afterEffectID) > 255 {
		return nil, ErrOwnedBudgetInspectionScope
	}
	if err := s.validateOwnedBudgetInspection(ctx, scope, limit); err != nil {
		return nil, err
	}
	query := s.db.WithContext(ctx).Where("space_id=? AND role_id=? AND owner_id=? AND parent_interaction_id=? AND coordination_id=?", scope.SpaceID, scope.RoleID, scope.OwnerID, scope.ParentInteractionID, scope.CoordinationID)
	if !afterCreatedAt.IsZero() {
		query = query.Where("(created_at>? OR (created_at=? AND effect_id>?))", afterCreatedAt, afterCreatedAt, afterEffectID)
	}
	var records []OwnedToolCallRecord
	err := query.Order("created_at ASC,effect_id ASC").Limit(limit).Find(&records).Error
	if err != nil {
		return nil, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	return records, nil
}
