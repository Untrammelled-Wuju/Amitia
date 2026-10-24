package mindruntime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestRound88DurableParentAtomicRevocationAndLimits(t *testing.T) {
	store, db := usageTestStore(t)
	model := modelBudgetFixture()
	tool := testOwnedToolBudget()
	limits := OwnedBudgetParentLimits{ToolCalls: tool.Limit, Tokens: model.TokenLimit, ChildInteractionID: "child-interaction"}
	if err := db.Exec(`CREATE TABLE interaction_records (id TEXT PRIMARY KEY,space_id TEXT,character_id TEXT,status TEXT,cancel_requested_at TEXT,recovery_descriptor_json TEXT,causation_id TEXT,correlation_id TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	parent := map[string]any{"multiAgent": map[string]any{
		"coordinationId": model.CoordinationID, "status": "running",
		"ownedScope":      map[string]any{"spaceId": model.SpaceID, "roleId": model.RoleID, "resourceOwnerId": model.OwnerID},
		"resourceOwnerId": model.OwnerID,
		"maxToolCalls":    tool.Limit, "maxTotalTokens": model.TokenLimit,
		"assignmentRefs": []map[string]string{{"assignmentId": model.AssignmentID, "status": "running"}, {"assignmentId": tool.AssignmentID, "status": "running"}},
	}}
	setParent := func(status string) {
		t.Helper()
		data, err := json.Marshal(parent)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(`INSERT INTO interaction_records (id,space_id,character_id,status,cancel_requested_at,recovery_descriptor_json) VALUES (?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET status=excluded.status,recovery_descriptor_json=excluded.recovery_descriptor_json`, model.ParentInteractionID, model.SpaceID, model.RoleID, status, "0001-01-01 00:00:00", string(data)).Error; err != nil {
			t.Fatal(err)
		}
	}
	setParent("processing")
	if err := db.Exec(`INSERT INTO interaction_records (id,space_id,character_id,status,cancel_requested_at,causation_id,correlation_id) VALUES (?,?,?,?,?,?,?)`, limits.ChildInteractionID, model.SpaceID, model.RoleID, "processing", "0001-01-01 00:00:00", model.ParentInteractionID, model.CoordinationID).Error; err != nil {
		t.Fatal(err)
	}
	ctx := WithDurableOwnedBudgetParent(context.Background(), limits)
	if _, err := reserveModelCall(WithOwnedModelBudget(ctx, model), store, 128); err != nil {
		t.Fatalf("model rejected authorized durable parent: %v", err)
	}
	if err := store.ReserveOwnedToolCall(ctx, tool, "round88-one", "read_file"); err != nil {
		t.Fatalf("tool rejected authorized durable parent: %v", err)
	}
	setParent("paused")
	if _, err := reserveModelCall(WithOwnedModelBudget(ctx, model), store, 128); !errors.Is(err, ErrOwnedModelBudgetExceeded) {
		t.Fatalf("paused durable parent admitted model: %v", err)
	}
	if err := store.ReserveOwnedToolCall(ctx, tool, "round88-paused", "read_file"); !errors.Is(err, ErrOwnedToolBudgetExceeded) {
		t.Fatalf("paused durable parent admitted tool: %v", err)
	}
	setParent("processing")
	if err := db.Exec(`UPDATE interaction_records SET status='cancelled' WHERE id=?`, limits.ChildInteractionID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := reserveModelCall(WithOwnedModelBudget(ctx, model), store, 128); !errors.Is(err, ErrOwnedModelBudgetExceeded) {
		t.Fatalf("cancelled child acquired another model reservation: %v", err)
	}
	if err := store.ReserveOwnedToolCall(ctx, tool, "round88-cancelled-child", "read_file"); !errors.Is(err, ErrOwnedToolBudgetExceeded) {
		t.Fatalf("cancelled child acquired another tool reservation: %v", err)
	}
	if err := db.Exec(`UPDATE interaction_records SET status='processing' WHERE id=?`, limits.ChildInteractionID).Error; err != nil {
		t.Fatal(err)
	}
	parent["multiAgent"].(map[string]any)["maxTotalTokens"] = int64(1024)
	setParent("processing")
	if _, err := reserveModelCall(WithOwnedModelBudget(ctx, model), store, 128); !errors.Is(err, ErrOwnedModelBudgetExceeded) {
		t.Fatalf("changed durable parent budget admitted stale worker: %v", err)
	}
	if err := store.ReserveOwnedToolCall(ctx, tool, "round88-stale", "read_file"); !errors.Is(err, ErrOwnedToolBudgetExceeded) {
		t.Fatalf("changed durable parent budget admitted stale tool: %v", err)
	}
	var models, tools int64
	if err := db.Model(&ModelUsageRecord{}).Count(&models).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&OwnedToolCallRecord{}).Count(&tools).Error; err != nil {
		t.Fatal(err)
	}
	if models != 1 || tools != 1 {
		t.Fatalf("revoked or stale parent wrote ledger: models=%d tools=%d", models, tools)
	}
}
