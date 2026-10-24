package mindruntime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"gorm.io/gorm"
)

type OwnedToolBudget struct {
	SpaceID             string
	RoleID              string
	OwnerID             string
	ParentInteractionID string
	CoordinationID      string
	AssignmentID        string
	Limit               int
}

type OwnedToolCallRecord struct {
	EffectID            string    `gorm:"primaryKey;size:255"`
	SpaceID             string    `gorm:"not null;index:idx_owned_tool_coord,priority:1"`
	CoordinationID      string    `gorm:"not null;index:idx_owned_tool_coord,priority:2"`
	ParentInteractionID string    `gorm:"not null"`
	OwnerID             string    `gorm:"not null"`
	RoleID              string    `gorm:"not null"`
	AssignmentID        string    `gorm:"not null"`
	ToolName            string    `gorm:"not null"`
	CreatedAt           time.Time `gorm:"not null"`
	ReviewEvidenceRef   string    `gorm:"column:review_evidence_ref;not null;default:''" json:"reviewEvidenceRef,omitempty"`
	ReviewActorID       string    `gorm:"column:review_actor_id;not null;default:''" json:"reviewActorId,omitempty"`
	ReviewRecordedAt    string    `gorm:"column:review_recorded_at;not null;default:''" json:"reviewRecordedAt,omitempty"`
}

func (OwnedToolCallRecord) TableName() string { return "runtime_owned_tool_calls" }

type ownedToolBudgetKey struct{}

func WithOwnedToolBudget(ctx context.Context, budget OwnedToolBudget) context.Context {
	return context.WithValue(ctx, ownedToolBudgetKey{}, budget)
}

func OwnedToolBudgetFromContext(ctx context.Context) (OwnedToolBudget, bool) {
	value, ok := ctx.Value(ownedToolBudgetKey{}).(OwnedToolBudget)
	return value, ok
}

var ErrOwnedToolBudgetExceeded = errors.New("multi_agent: shared tool-call budget exhausted")
var ErrOwnedToolEffectUncertain = errors.New("multi_agent: tool-call effect already reserved; reconcile the existing result before retry")

func (s *ModelUsageStore) ReserveOwnedToolCall(ctx context.Context, budget OwnedToolBudget, callID, toolName string) error {
	if s == nil || s.db == nil || budget.SpaceID == "" || budget.SpaceID != s.SpaceID || budget.OwnerID == "" || budget.RoleID == "" || budget.ParentInteractionID == "" || budget.CoordinationID == "" || budget.AssignmentID == "" || budget.Limit < 1 || budget.Limit > 256 || len(callID) == 0 || len(callID) > 128 || strings.TrimSpace(toolName) == "" || len(toolName) > 256 {
		return errors.New("multi_agent: invalid tool-budget scope or effect identity")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	if s.db.Dialector.Name() != "sqlite" {
		return errors.New("multi_agent: atomic tool budget admission requires a verified shared sqlite ledger")
	}
	effectKey := sha256.Sum256([]byte(strings.Join([]string{budget.SpaceID, budget.ParentInteractionID, budget.CoordinationID, budget.AssignmentID, callID}, "\x00")))
	effectID := hex.EncodeToString(effectKey[:])
	var result *gorm.DB
	err := coordination.CommitCurrent(ctx, func() error {
		parentClause := ""
		parentArgs := []any{}
		if limits, guarded := durableOwnedBudgetParent(ctx); guarded {
			if limits.ChildInteractionID == "" || limits.ToolCalls != budget.Limit {
				return ErrOwnedToolBudgetExceeded
			}
			parentClause = " AND " + ownedBudgetParentCondition
			parentArgs = ownedBudgetParentArguments(budget.SpaceID, budget.RoleID, budget.OwnerID, budget.ParentInteractionID, budget.CoordinationID, budget.AssignmentID, limits.ChildInteractionID, limits.ToolCalls, limits.Tokens, limits.CostMicroUSD)
		}
		query := `INSERT INTO runtime_owned_tool_calls
        (effect_id,space_id,coordination_id,parent_interaction_id,owner_id,role_id,assignment_id,tool_name,created_at)
        SELECT ?,?,?,?,?,?,?,?,?
        WHERE (SELECT COUNT(*) FROM runtime_owned_tool_calls WHERE space_id=? AND coordination_id=?) < ?
          AND NOT EXISTS (SELECT 1 FROM runtime_owned_tool_calls WHERE space_id=? AND coordination_id=? AND (COALESCE(owner_id,'')<>? OR COALESCE(role_id,'')<>? OR COALESCE(parent_interaction_id,'')<>?))
          AND NOT EXISTS (SELECT 1 FROM runtime_model_usage WHERE space_id=? AND coordination_id=? AND (COALESCE(owner_id,'')<>? OR COALESCE(role_id,'')<>? OR COALESCE(parent_interaction_id,'')<>?))` + parentClause
		args := []any{
			effectID, budget.SpaceID, budget.CoordinationID, budget.ParentInteractionID, budget.OwnerID, budget.RoleID, budget.AssignmentID, toolName, time.Now().UTC(), budget.SpaceID, budget.CoordinationID, budget.Limit,
			budget.SpaceID, budget.CoordinationID, budget.OwnerID, budget.RoleID, budget.ParentInteractionID,
			budget.SpaceID, budget.CoordinationID, budget.OwnerID, budget.RoleID, budget.ParentInteractionID}
		result = s.db.WithContext(ctx).Exec(query, append(args, parentArgs...)...)
		return result.Error
	})
	if err != nil {
		if result == nil {
			return err
		}
		var count int64
		if err := s.db.WithContext(ctx).Model(&OwnedToolCallRecord{}).Where("effect_id=?", effectID).Count(&count).Error; err == nil && count > 0 {
			return ErrOwnedToolEffectUncertain
		}
		return err
	}
	if result.RowsAffected != 1 {
		var count int64
		if err := s.db.WithContext(ctx).Model(&OwnedToolCallRecord{}).Where("effect_id=?", effectID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrOwnedToolEffectUncertain
		}
		return ErrOwnedToolBudgetExceeded
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	return nil
}
