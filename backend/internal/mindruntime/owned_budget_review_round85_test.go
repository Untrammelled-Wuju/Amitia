package mindruntime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func round85ReviewContext(space, role, owner string, permitted bool) context.Context {
	device := owner
	if !permitted {
		device = "non-owner"
	}
	ctx := coordination.WithScope(context.Background(), coordination.ExecutionScope{SpaceID: space, RoleID: role, ResourceOwnerID: owner, RoleOwnerID: owner, InitiatorDeviceID: device})
	actor := &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: runtimeidentity.SpaceID(space), DeviceID: runtimeidentity.DeviceID(device)}
	return auth.WithActor(ctx, actor)
}

func TestRound85EvidenceReviewImmutableWithoutBudgetRefund(t *testing.T) {
	store, db := usageTestStore(t)
	budget := modelBudgetFixture()
	modelID, err := store.Begin(WithOwnedModelBudget(context.Background(), budget), ModelUsageRecord{SpaceID: budget.SpaceID, ReservedTokens: 128})
	if err != nil {
		t.Fatal(err)
	}
	tool := testOwnedToolBudget()
	if err := store.ReserveOwnedToolCall(context.Background(), tool, "review-this", "read_file"); err != nil {
		t.Fatal(err)
	}
	var effect OwnedToolCallRecord
	if err := db.Where("coordination_id=?", tool.CoordinationID).First(&effect).Error; err != nil {
		t.Fatal(err)
	}
	scope := OwnedBudgetReconciliationScope{SpaceID: budget.SpaceID, RoleID: budget.RoleID, OwnerID: budget.OwnerID, ParentInteractionID: budget.ParentInteractionID, CoordinationID: budget.CoordinationID}
	ctx := round85ReviewContext(scope.SpaceID, scope.RoleID, scope.OwnerID, true)
	for _, current := range []OwnedBudgetEvidenceReview{
		{Scope: scope, Kind: "model", RecordID: modelID, EvidenceRef: "case:provider-report-1234"},
		{Scope: scope, Kind: "tool", RecordID: effect.EffectID, EvidenceRef: "case:tool-audit-1234"},
	} {
		if err := store.AttachOwnedBudgetEvidence(ctx, current); err != nil {
			t.Fatal(err)
		}
		if err := store.AttachOwnedBudgetEvidence(ctx, current); err != nil {
			t.Fatalf("exact retry must recover the recorded receipt: %v", err)
		}
		changed := current
		changed.EvidenceRef = "case:forged-12345"
		if err := store.AttachOwnedBudgetEvidence(ctx, changed); !errors.Is(err, ErrOwnedBudgetReviewConflict) {
			t.Fatalf("overwriting evidence accepted: %v", err)
		}
	}
	var recorded ModelUsageRecord
	if err := db.Where("id=?", modelID).First(&recorded).Error; err != nil {
		t.Fatal(err)
	}
	if recorded.ReservedTokens != 128 || recorded.Status != "running" || recorded.ReviewEvidenceRef != "case:provider-report-1234" || recorded.ReviewActorID != scope.OwnerID || recorded.ReviewRecordedAt == "" {
		t.Fatalf("manual note modified model settlement: %+v", recorded)
	}
	if err := db.Where("effect_id=?", effect.EffectID).First(&effect).Error; err != nil {
		t.Fatal(err)
	}
	if effect.ReviewEvidenceRef != "case:tool-audit-1234" || effect.ReviewActorID != scope.OwnerID || effect.ReviewRecordedAt == "" {
		t.Fatalf("tool evidence was not durably recorded: %+v", effect)
	}
	restarted, err := NewModelUsageStore(db, scope.SpaceID)
	if err != nil {
		t.Fatal(err)
	}
	reserved, err := restarted.OwnedModelTokenUsage(context.Background(), scope.SpaceID, scope.CoordinationID)
	if err != nil || reserved != 128 {
		t.Fatalf("review unexpectedly refunded model budget: %d %v", reserved, err)
	}
	var toolCount int64
	if err := db.Model(&OwnedToolCallRecord{}).Where("effect_id=?", effect.EffectID).Count(&toolCount).Error; err != nil || toolCount != 1 {
		t.Fatalf("review deleted uncertain tool effect: %d %v", toolCount, err)
	}
}

func TestRound85EvidenceReviewRequiresCurrentOwnerAndScope(t *testing.T) {
	store, db := usageTestStore(t)
	budget := modelBudgetFixture()
	id, err := store.Begin(WithOwnedModelBudget(context.Background(), budget), ModelUsageRecord{SpaceID: budget.SpaceID, ReservedTokens: 128})
	if err != nil {
		t.Fatal(err)
	}
	scope := OwnedBudgetReconciliationScope{SpaceID: budget.SpaceID, RoleID: budget.RoleID, OwnerID: budget.OwnerID, ParentInteractionID: budget.ParentInteractionID, CoordinationID: budget.CoordinationID}
	review := OwnedBudgetEvidenceReview{Scope: scope, Kind: "model", RecordID: id, EvidenceRef: "case:provider-report-1234"}
	for _, ctx := range []context.Context{context.Background(), round85ReviewContext(scope.SpaceID, scope.RoleID, scope.OwnerID, false), round85ReviewContext("other", scope.RoleID, scope.OwnerID, true), round85ReviewContext(scope.SpaceID, "wrong-role", scope.OwnerID, true), round85ReviewContext(scope.SpaceID, scope.RoleID, "wrong-owner", true)} {
		if err := store.AttachOwnedBudgetEvidence(ctx, review); err == nil {
			t.Fatalf("unauthorized evidence accepted: %v", ctx)
		}
	}
	allowed := round85ReviewContext(scope.SpaceID, scope.RoleID, scope.OwnerID, true)
	forged := review
	forged.Scope.ParentInteractionID = "wrong-parent"
	if err := store.AttachOwnedBudgetEvidence(allowed, forged); !errors.Is(err, ErrOwnedBudgetReviewConflict) {
		t.Fatalf("foreign parent accepted: %v", err)
	}
	forged = review
	forged.EvidenceRef = "not"
	if err := store.AttachOwnedBudgetEvidence(allowed, forged); !errors.Is(err, ErrOwnedBudgetInspectionScope) {
		t.Fatalf("unbounded evidence accepted: %v", err)
	}
	var row ModelUsageRecord
	if err := db.Where("id=?", id).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ReviewEvidenceRef != "" {
		t.Fatalf("invalid review modified record: %+v", row)
	}
}

func TestRound85ConcurrentReviewFirstWriterWins(t *testing.T) {
	store, db := usageTestStore(t)
	budget := modelBudgetFixture()
	id, err := store.Begin(WithOwnedModelBudget(context.Background(), budget), ModelUsageRecord{SpaceID: budget.SpaceID, ReservedTokens: 128})
	if err != nil {
		t.Fatal(err)
	}
	scope := OwnedBudgetReconciliationScope{SpaceID: budget.SpaceID, RoleID: budget.RoleID, OwnerID: budget.OwnerID, ParentInteractionID: budget.ParentInteractionID, CoordinationID: budget.CoordinationID}
	ctx := round85ReviewContext(scope.SpaceID, scope.RoleID, scope.OwnerID, true)
	var succeeded atomic.Int64
	var group sync.WaitGroup
	for i := 0; i < 32; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			if err := store.AttachOwnedBudgetEvidence(ctx, OwnedBudgetEvidenceReview{Scope: scope, Kind: "model", RecordID: id, EvidenceRef: fmt.Sprintf("case:operator-proof-%d", i)}); err == nil {
				succeeded.Add(1)
			}
		}(i)
	}
	group.Wait()
	if succeeded.Load() != 1 {
		t.Fatalf("competing evidence writes accepted: %d", succeeded.Load())
	}
	var row ModelUsageRecord
	if err := db.Where("id=?", id).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ReservedTokens != 128 || row.Status != "running" || row.ReviewEvidenceRef == "" {
		t.Fatalf("competition modified quota or dropped evidence: %+v", row)
	}
}
