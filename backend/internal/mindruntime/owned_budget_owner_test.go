package mindruntime

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestBudgetEvidenceCoreOwnerUsesLiveAdministration(t *testing.T) {
	store, db := usageTestStore(t)
	budget := modelBudgetFixture()
	budget.OwnerID = "core"
	id, err := store.Begin(WithOwnedModelBudget(context.Background(), budget), ModelUsageRecord{ReservedTokens: 128})
	if err != nil {
		t.Fatal(err)
	}
	scope := OwnedBudgetReconciliationScope{SpaceID: budget.SpaceID, RoleID: budget.RoleID, OwnerID: budget.OwnerID, ParentInteractionID: budget.ParentInteractionID, CoordinationID: budget.CoordinationID}
	actor := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "core", DeviceID: "admin-device", Permissions: []string{auth.PermSystemAdmin}}
	execution := coordination.ExecutionScope{SpaceID: "core", CoreID: "core", RoleID: budget.RoleID, RoleOwnerID: "core", ResourceOwnerID: "core", InitiatorDeviceID: "admin-device", Coordinated: true}
	ctx := auth.WithActor(coordination.WithScope(context.Background(), execution), actor)
	review := OwnedBudgetEvidenceReview{Scope: scope, Kind: "model", RecordID: id, EvidenceRef: "case:core-owner-123"}
	if store.CanReviewOwnedBudgetEvidence(ctx, scope) {
		t.Fatal("missing live owner check allowed writing")
	}
	allowed := false
	store.SetCoreOwnerBudgetReview(func(context.Context, coordination.ExecutionScope) error {
		if !allowed {
			return ErrOwnedBudgetReviewForbidden
		}
		return nil
	})
	if err := store.AttachOwnedBudgetEvidence(ctx, review); !errors.Is(err, ErrOwnedBudgetReviewForbidden) {
		t.Fatalf("revoked administrator registered evidence: %v", err)
	}
	allowed = true
	foreign := *actor
	foreign.DeviceID = runtimeidentity.DeviceID("another-device")
	if store.CanReviewOwnedBudgetEvidence(auth.WithActor(ctx, &foreign), scope) {
		t.Fatal("initiator identity mismatch allowed writing")
	}
	foreign = *actor
	foreign.Permissions = nil
	if store.CanReviewOwnedBudgetEvidence(auth.WithActor(ctx, &foreign), scope) {
		t.Fatal("ordinary shared role member allowed writing")
	}
	if err := store.AttachOwnedBudgetEvidence(ctx, review); err != nil {
		t.Fatal(err)
	}
	var row ModelUsageRecord
	if err := db.Where("id=?", id).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ReviewActorID != "admin-device" || row.ReservedTokens != 128 || row.Status != "running" {
		t.Fatalf("unexpected owner receipt: %+v", row)
	}
	allowed = false
	if err := store.AttachOwnedBudgetEvidence(ctx, review); !errors.Is(err, ErrOwnedBudgetReviewForbidden) {
		t.Fatalf("idempotent retry bypassed revocation: %v", err)
	}
}

func TestBudgetEvidenceOwnerRevokedAtCommitDoesNotWrite(t *testing.T) {
	store, db := usageTestStore(t)
	budget := modelBudgetFixture()
	id, err := store.Begin(WithOwnedModelBudget(context.Background(), budget), ModelUsageRecord{ReservedTokens: 128})
	if err != nil {
		t.Fatal(err)
	}
	scope := OwnedBudgetReconciliationScope{SpaceID: budget.SpaceID, RoleID: budget.RoleID, OwnerID: budget.OwnerID, ParentInteractionID: budget.ParentInteractionID, CoordinationID: budget.CoordinationID}
	checks := 0
	ctx := coordination.WithAdditionalGuard(round85ReviewContext(scope.SpaceID, scope.RoleID, scope.OwnerID, true), func(context.Context) error {
		checks++
		if checks >= 2 {
			return coordination.ErrScopeExpired
		}
		return nil
	})
	if err := store.AttachOwnedBudgetEvidence(ctx, OwnedBudgetEvidenceReview{Scope: scope, Kind: "model", RecordID: id, EvidenceRef: "case:revoked-owner"}); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("commit crossed revoked authority: %v", err)
	}
	var row ModelUsageRecord
	if err := db.Where("id=?", id).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ReviewEvidenceRef != "" {
		t.Fatal("revoked owner wrote evidence")
	}
}
