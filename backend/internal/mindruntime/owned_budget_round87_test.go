package mindruntime

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func round87HistoricalReadContext(t *testing.T) context.Context {
	t.Helper()
	current := round87ScopeForProof()
	proof := coordination.TaskReadProof{Scope: current, TaskRunID: "run-1", DefinitionFingerprint: strings.Repeat("a", 64)}
	read, err := coordination.WithTaskReadAuthority(coordination.WithScope(context.Background(), current), current, proof)
	if err != nil {
		t.Fatal(err)
	}
	return read
}

func TestRound87HistoricalTaskReadCannotReserveBudget(t *testing.T) {
	store, db := usageTestStore(t)
	ctx := round87HistoricalReadContext(t)
	budget := modelBudgetFixture()
	if _, err := store.Begin(WithOwnedModelBudget(ctx, budget), ModelUsageRecord{SpaceID: budget.SpaceID, ReservedTokens: 128}); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("read-only task acquired model budget: %v", err)
	}
	if err := store.ReserveOwnedToolCall(ctx, testOwnedToolBudget(), "call-87", "read_file"); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("read-only task acquired tool budget: %v", err)
	}
	var models, tools int64
	if err := db.Model(&ModelUsageRecord{}).Count(&models).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&OwnedToolCallRecord{}).Count(&tools).Error; err != nil {
		t.Fatal(err)
	}
	if models != 0 || tools != 0 {
		t.Fatalf("read-only task persisted a reservation: models=%d tools=%d", models, tools)
	}
}

func TestRound87ReadOnlyTaskCannotRegisterBudgetEvidence(t *testing.T) {
	store, db := usageTestStore(t)
	budget := modelBudgetFixture()
	id, err := store.Begin(WithOwnedModelBudget(context.Background(), budget), ModelUsageRecord{ReservedTokens: 128})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ReserveOwnedToolCall(context.Background(), testOwnedToolBudget(), "call-87", "read_file"); err != nil {
		t.Fatal(err)
	}
	var effect OwnedToolCallRecord
	if err := db.First(&effect).Error; err != nil {
		t.Fatal(err)
	}
	ctx := round85ReviewContext(budget.SpaceID, budget.RoleID, budget.OwnerID, true)
	proof := coordination.TaskReadProof{Scope: round87ScopeForProof(), TaskRunID: "run-1", DefinitionFingerprint: strings.Repeat("a", 64)}
	ctx = coordination.WithScope(ctx, proof.Scope)
	read, err := coordination.WithTaskReadAuthority(ctx, proof.Scope, proof)
	if err != nil {
		t.Fatal(err)
	}
	scope := OwnedBudgetReconciliationScope{SpaceID: budget.SpaceID, RoleID: budget.RoleID, OwnerID: budget.OwnerID, ParentInteractionID: budget.ParentInteractionID, CoordinationID: budget.CoordinationID}
	for _, record := range []OwnedBudgetEvidenceReview{
		{Scope: scope, Kind: "model", RecordID: id, EvidenceRef: "case:round87-provider-report"},
		{Scope: scope, Kind: "tool", RecordID: effect.EffectID, EvidenceRef: "case:round87-tool-report"},
	} {
		if err := store.AttachOwnedBudgetEvidence(read, record); !errors.Is(err, coordination.ErrWrongOwner) {
			t.Fatalf("read-only task registered %s evidence: %v", record.Kind, err)
		}
	}
	var model ModelUsageRecord
	if err := db.Where("id=?", id).First(&model).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("effect_id=?", effect.EffectID).First(&effect).Error; err != nil {
		t.Fatal(err)
	}
	if model.ReviewEvidenceRef != "" || effect.ReviewEvidenceRef != "" {
		t.Fatal("read-only task modified persistent evidence")
	}
}

func round87ScopeForProof() coordination.ExecutionScope {
	return coordination.ExecutionScope{
		SpaceID: "core", CoreID: "core", AuthorizationRealm: "core",
		InitiatorDeviceID: "owner-a", TargetDeviceID: "owner-a",
		RoleID: "role-a", RoleOwnerID: "owner-a", ResourceOwnerID: "owner-a",
		ProviderEpoch: 1, TargetProviderEpoch: 1, ModeRevision: 1,
		PermissionRevision: 1, TargetPermissionRevision: 1, RoleRevision: 1,
		RequestID: "read-request", ExecutionID: "execution", TurnID: "turn",
	}
}

func TestRound87ModelUsageStoreCannotCrossSpace(t *testing.T) {
	store, db := usageTestStore(t)
	if _, err := store.Begin(context.Background(), ModelUsageRecord{SpaceID: "foreign", Model: "model"}); err == nil {
		t.Fatal("cross-space unbudgeted model usage accepted")
	}
	var models int64
	if err := db.Model(&ModelUsageRecord{}).Count(&models).Error; err != nil || models != 0 {
		t.Fatalf("foreign usage persisted: %d %v", models, err)
	}
	other, err := NewModelUsageStore(db, "foreign")
	if err != nil {
		t.Fatal(err)
	}
	id, err := other.Begin(context.Background(), ModelUsageRecord{Model: "model"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(context.Background(), id, ModelUsageRecord{Status: "succeeded", UsageReported: true, TotalTokens: 1}); err == nil {
		t.Fatal("foreign-space metering record settled")
	}
	var row ModelUsageRecord
	if err := db.Where("id=?", id).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Status != "running" || row.TotalTokens != 0 {
		t.Fatalf("foreign-space record was altered: %+v", row)
	}
}

func TestRound87RevocationInsideCommitGateDoesNotConsumeBudget(t *testing.T) {
	store, db := usageTestStore(t)
	var checks atomic.Int32
	revoked := errors.New("authorization revoked at commit barrier")
	ctx := coordination.WithAdditionalGuard(context.Background(), func(context.Context) error {
		if checks.Add(1) > 1 {
			return revoked
		}
		return nil
	})
	budget := modelBudgetFixture()
	if _, err := store.Begin(WithOwnedModelBudget(ctx, budget), ModelUsageRecord{ReservedTokens: 128}); !errors.Is(err, revoked) {
		t.Fatalf("model write crossed revoked commit barrier: %v", err)
	}
	checks.Store(0)
	if err := store.ReserveOwnedToolCall(ctx, testOwnedToolBudget(), "call-87", "read_file"); !errors.Is(err, revoked) {
		t.Fatalf("tool write crossed revoked commit barrier: %v", err)
	}
	var modelCount, toolCount int64
	if err := db.Model(&ModelUsageRecord{}).Count(&modelCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&OwnedToolCallRecord{}).Count(&toolCount).Error; err != nil {
		t.Fatal(err)
	}
	if modelCount != 0 || toolCount != 0 {
		t.Fatalf("revocation left budget records: model=%d tool=%d", modelCount, toolCount)
	}
}
