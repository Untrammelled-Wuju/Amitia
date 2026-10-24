package mindruntime

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"gorm.io/gorm"
)

var ErrOwnedBudgetReviewForbidden = errors.New("multi_agent: current resource owner is required to register budget evidence")
var ErrOwnedBudgetReviewConflict = errors.New("multi_agent: budget evidence already reviewed or no longer matches authorized scope")

type OwnedBudgetEvidenceReview struct {
	Scope       OwnedBudgetReconciliationScope
	Kind        string
	RecordID    string
	EvidenceRef string
}

func (s *ModelUsageStore) SetCoreOwnerBudgetReview(check func(context.Context, coordination.ExecutionScope) error) {
	if s != nil {
		s.coreOwnerReview = check
	}
}

func (s *ModelUsageStore) requireBudgetEvidenceOwner(ctx context.Context, expected OwnedBudgetReconciliationScope) error {
	actor, present := auth.FromContext(ctx)
	scope, scoped := coordination.FromContext(ctx)
	if s == nil || !present || actor == nil || !scoped || actor.DeviceID == "" ||
		actor.SpaceID.String() != expected.SpaceID || actor.DeviceID.String() != scope.InitiatorDeviceID ||
		(actor.PrincipalType != auth.PrincipalLocalUI && actor.PrincipalType != auth.PrincipalTrustedDevice) ||
		scope.SpaceID != expected.SpaceID || scope.RoleID != expected.RoleID || scope.ResourceOwnerID != expected.OwnerID {
		return ErrOwnedBudgetReviewForbidden
	}
	if !scope.Coordinated {
		if actor.DeviceID.String() != scope.ResourceOwnerID || scope.RoleOwnerID != scope.ResourceOwnerID {
			return ErrOwnedBudgetReviewForbidden
		}
		return nil
	}
	if scope.ResourceOwnerID != scope.CoreID || scope.RoleOwnerID != scope.CoreID || !actor.HasPermission(auth.PermSystemAdmin) || s.coreOwnerReview == nil {
		return ErrOwnedBudgetReviewForbidden
	}
	return s.coreOwnerReview(ctx, scope)
}

func (s *ModelUsageStore) CanReviewOwnedBudgetEvidence(ctx context.Context, scope OwnedBudgetReconciliationScope) bool {
	return s.validateOwnedBudgetInspection(ctx, scope, 1) == nil && s.requireBudgetEvidenceOwner(ctx, scope) == nil
}

func (s *ModelUsageStore) AttachOwnedBudgetEvidence(ctx context.Context, review OwnedBudgetEvidenceReview) error {
	actor, present := auth.FromContext(ctx)
	if !present || actor == nil {
		return ErrOwnedBudgetReviewForbidden
	}
	if err := s.requireBudgetEvidenceOwner(ctx, review.Scope); err != nil {
		return err
	}
	ctx = coordination.WithAdditionalGuard(ctx, func(current context.Context) error {
		return s.requireBudgetEvidenceOwner(current, review.Scope)
	})
	if review.Kind != "model" && review.Kind != "tool" {
		return ErrOwnedBudgetInspectionScope
	}
	if strings.TrimSpace(review.RecordID) != review.RecordID || len(review.RecordID) < 1 || len(review.RecordID) > 255 || strings.ContainsAny(review.RecordID, "\x00\r\n") || strings.TrimSpace(review.EvidenceRef) != review.EvidenceRef || len(review.EvidenceRef) < 8 || len(review.EvidenceRef) > 512 || strings.ContainsAny(review.EvidenceRef, "\x00\r\n") {
		return ErrOwnedBudgetInspectionScope
	}
	if err := s.validateOwnedBudgetInspection(ctx, review.Scope, 1); err != nil {
		return err
	}
	timeRecorded := time.Now().UTC().Format(time.RFC3339Nano)
	var resultRows int64
	err := coordination.CommitCurrent(ctx, func() error {
		if review.Kind == "model" {
			result := s.db.WithContext(ctx).Model(&ModelUsageRecord{}).Where("id=? AND space_id=? AND role_id=? AND owner_id=? AND parent_interaction_id=? AND coordination_id=? AND COALESCE(review_evidence_ref,'')=''", review.RecordID, review.Scope.SpaceID, review.Scope.RoleID, review.Scope.OwnerID, review.Scope.ParentInteractionID, review.Scope.CoordinationID).Updates(map[string]any{"review_evidence_ref": review.EvidenceRef, "review_actor_id": actor.DeviceID.String(), "review_recorded_at": timeRecorded})
			if result.Error != nil {
				return result.Error
			}
			resultRows = result.RowsAffected
		} else {
			result := s.db.WithContext(ctx).Model(&OwnedToolCallRecord{}).Where("effect_id=? AND space_id=? AND role_id=? AND owner_id=? AND parent_interaction_id=? AND coordination_id=? AND COALESCE(review_evidence_ref,'')=''", review.RecordID, review.Scope.SpaceID, review.Scope.RoleID, review.Scope.OwnerID, review.Scope.ParentInteractionID, review.Scope.CoordinationID).Updates(map[string]any{"review_evidence_ref": review.EvidenceRef, "review_actor_id": actor.DeviceID.String(), "review_recorded_at": timeRecorded})
			if result.Error != nil {
				return result.Error
			}
			resultRows = result.RowsAffected
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	if resultRows != 1 {
		var saved struct {
			ReviewEvidenceRef string
			ReviewActorID     string
			ReviewRecordedAt  string
		}
		table, key := "runtime_model_usage", "id"
		if review.Kind == "tool" {
			table, key = "runtime_owned_tool_calls", "effect_id"
		}
		result := s.db.WithContext(ctx).Table(table).Select("review_evidence_ref, review_actor_id, review_recorded_at").Where(key+"=? AND space_id=? AND role_id=? AND owner_id=? AND parent_interaction_id=? AND coordination_id=?", review.RecordID, review.Scope.SpaceID, review.Scope.RoleID, review.Scope.OwnerID, review.Scope.ParentInteractionID, review.Scope.CoordinationID).Take(&saved)
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return result.Error
		}
		if result.Error == nil && saved.ReviewEvidenceRef == review.EvidenceRef && saved.ReviewActorID == actor.DeviceID.String() && saved.ReviewRecordedAt != "" {
			return coordination.ValidateCurrent(ctx)
		}
		return ErrOwnedBudgetReviewConflict
	}
	return nil
}
