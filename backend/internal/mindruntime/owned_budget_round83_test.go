package mindruntime

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRound83BudgetStoreRejectsForeignSpace(t *testing.T) {
	store, db := usageTestStore(t)
	model := modelBudgetFixture()
	model.SpaceID = "foreign"
	if _, err := store.Begin(WithOwnedModelBudget(context.Background(), model), ModelUsageRecord{SpaceID: "foreign", ReservedTokens: 128}); err == nil || err.Error() != "multi_agent: model usage space conflicts with store" {
		t.Fatalf("foreign model reservation accepted: %v", err)
	}
	tools := testOwnedToolBudget()
	tools.SpaceID = "foreign"
	if err := store.ReserveOwnedToolCall(context.Background(), tools, "foreign-effect", "shell_run"); err == nil {
		t.Fatal("foreign tool reservation accepted")
	}
	if _, err := store.OwnedModelTokenUsage(context.Background(), "foreign", model.CoordinationID); err == nil {
		t.Fatal("foreign token balance exposed")
	}
	if _, err := store.OwnedModelCostUsage(context.Background(), "foreign", model.CoordinationID); err == nil {
		t.Fatal("foreign cost balance exposed")
	}
	var models, effects int64
	if err := db.Model(&ModelUsageRecord{}).Count(&models).Error; err != nil || models != 0 {
		t.Fatalf("foreign model record persisted: %d %v", models, err)
	}
	if err := db.Model(&OwnedToolCallRecord{}).Count(&effects).Error; err != nil || effects != 0 {
		t.Fatalf("foreign tool record persisted: %d %v", effects, err)
	}
}

func TestRound83CrossLedgerOwnerScopeIsImmutable(t *testing.T) {
	for _, first := range []string{"model", "tool"} {
		t.Run(first, func(t *testing.T) {
			store, db := usageTestStore(t)
			model := modelBudgetFixture()
			tool := testOwnedToolBudget()
			if first == "model" {
				if _, err := store.Begin(WithOwnedModelBudget(context.Background(), model), ModelUsageRecord{SpaceID: model.SpaceID, ReservedTokens: 128}); err != nil {
					t.Fatal(err)
				}
			} else if err := store.ReserveOwnedToolCall(context.Background(), tool, "initial", "read_file"); err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"owner", "role", "parent"} {
				changedModel := model
				changedTool := tool
				switch field {
				case "owner":
					changedModel.OwnerID = "foreign-owner"
					changedTool.OwnerID = "foreign-owner"
				case "role":
					changedModel.RoleID = "foreign-role"
					changedTool.RoleID = "foreign-role"
				case "parent":
					changedModel.ParentInteractionID = "foreign-parent"
					changedTool.ParentInteractionID = "foreign-parent"
				}
				if _, err := store.Begin(WithOwnedModelBudget(context.Background(), changedModel), ModelUsageRecord{SpaceID: model.SpaceID, ReservedTokens: 128}); !errors.Is(err, ErrOwnedModelBudgetExceeded) {
					t.Fatalf("%s cross-scope model accepted: %v", field, err)
				}
				if err := store.ReserveOwnedToolCall(context.Background(), changedTool, "foreign-"+field, "read_file"); !errors.Is(err, ErrOwnedToolBudgetExceeded) {
					t.Fatalf("%s cross-scope tool accepted: %v", field, err)
				}
			}
			var models, tools int64
			if err := db.Model(&ModelUsageRecord{}).Count(&models).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&OwnedToolCallRecord{}).Count(&tools).Error; err != nil {
				t.Fatal(err)
			}
			if models+tools != 1 {
				t.Fatalf("cross-scope admission wrote ledger rows: models=%d tools=%d", models, tools)
			}
		})
	}
}

func TestRound83ReadRevocationDuringReconciliationReturnsNoRecords(t *testing.T) {
	store, _ := usageTestStore(t)
	model := modelBudgetFixture()
	_, err := store.Begin(WithOwnedModelBudget(context.Background(), model), ModelUsageRecord{SpaceID: model.SpaceID, ReservedTokens: 128})
	if err != nil {
		t.Fatal(err)
	}
	tool := testOwnedToolBudget()
	if err := store.ReserveOwnedToolCall(context.Background(), tool, "known", "read_file"); err != nil {
		t.Fatal(err)
	}
	query := OwnedBudgetReconciliationScope{SpaceID: model.SpaceID, RoleID: model.RoleID, OwnerID: model.OwnerID, ParentInteractionID: model.ParentInteractionID, CoordinationID: model.CoordinationID}
	revoked := errors.New("owner revoked read scope")
	for _, kind := range []string{"model", "tool"} {
		var checks atomic.Int32
		ctx := coordination.WithScope(context.Background(), coordination.ExecutionScope{SpaceID: model.SpaceID, RoleID: model.RoleID, ResourceOwnerID: model.OwnerID})
		ctx = coordination.WithAdditionalGuard(ctx, func(context.Context) error {
			if checks.Add(1) >= 2 {
				return revoked
			}
			return nil
		})
		if kind == "model" {
			rows, err := store.PendingOwnedModelReconciliation(ctx, query, 10)
			if !errors.Is(err, revoked) || len(rows) != 0 {
				t.Fatalf("revoked model reader returned records: %+v %v", rows, err)
			}
		} else {
			rows, err := store.ReservedOwnedToolReconciliation(ctx, query, 10)
			if !errors.Is(err, revoked) || len(rows) != 0 {
				t.Fatalf("revoked tool reader returned records: %+v %v", rows, err)
			}
		}
		if checks.Load() != 2 {
			t.Fatalf("expected pre-read and post-read authority checks, got %d", checks.Load())
		}
	}
}

func TestRound83ReconciliationPagesAndExcludesForeignParents(t *testing.T) {
	store, db := usageTestStore(t)
	model := modelBudgetFixture()
	ctx := WithOwnedModelBudget(context.Background(), model)
	for n := 0; n < 3; n++ {
		if _, err := store.Begin(ctx, ModelUsageRecord{SpaceID: model.SpaceID, ReservedTokens: 128}); err != nil {
			t.Fatal(err)
		}
	}
	foreign := model
	foreign.CoordinationID = "different-parent-coordination"
	if _, err := store.Begin(WithOwnedModelBudget(context.Background(), foreign), ModelUsageRecord{SpaceID: foreign.SpaceID, ReservedTokens: 128}); err != nil {
		t.Fatal(err)
	}
	tool := testOwnedToolBudget()
	for _, id := range []string{"first", "second", "third"} {
		if err := store.ReserveOwnedToolCall(context.Background(), tool, id, "read_file"); err != nil {
			t.Fatal(err)
		}
	}
	query := OwnedBudgetReconciliationScope{SpaceID: model.SpaceID, RoleID: model.RoleID, OwnerID: model.OwnerID, ParentInteractionID: model.ParentInteractionID, CoordinationID: model.CoordinationID}
	read := coordination.WithScope(context.Background(), coordination.ExecutionScope{SpaceID: model.SpaceID, RoleID: model.RoleID, ResourceOwnerID: model.OwnerID})
	first, err := store.PendingOwnedModelReconciliationPage(read, query, 0, 2)
	if err != nil || len(first) != 2 {
		t.Fatalf("first model page: %v %v", first, err)
	}
	last, err := store.PendingOwnedModelReconciliationPage(read, query, first[1].Sequence, 2)
	if err != nil || len(last) != 1 || last[0].Sequence <= first[1].Sequence {
		t.Fatalf("last model page: %v %v", last, err)
	}
	toolsFirst, err := store.ReservedOwnedToolReconciliationPage(read, query, time.Time{}, "", 2)
	if err != nil || len(toolsFirst) != 2 {
		t.Fatalf("first tool page: %v %v", toolsFirst, err)
	}
	toolsLast, err := store.ReservedOwnedToolReconciliationPage(read, query, toolsFirst[1].CreatedAt, toolsFirst[1].EffectID, 2)
	if err != nil || len(toolsLast) != 1 || toolsLast[0].EffectID == toolsFirst[1].EffectID {
		t.Fatalf("last tool page: %v %v", toolsLast, err)
	}
	if rows, err := store.PendingOwnedModelReconciliationPage(read, query, -1, 2); !errors.Is(err, ErrOwnedBudgetInspectionScope) || len(rows) != 0 {
		t.Fatalf("negative model cursor admitted: %+v %v", rows, err)
	}
	if rows, err := store.ReservedOwnedToolReconciliationPage(read, query, time.Time{}, "forged", 2); !errors.Is(err, ErrOwnedBudgetInspectionScope) || len(rows) != 0 {
		t.Fatalf("invalid tool cursor admitted: %+v %v", rows, err)
	}
	var rows int64
	if err := db.Model(&ModelUsageRecord{}).Count(&rows).Error; err != nil || rows != 4 {
		t.Fatalf("inspection changed budget records: %d %v", rows, err)
	}
}
