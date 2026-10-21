package business

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestPreviewReplyLoadsWorldbookWithoutConversationHistory(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	if _, err := db.Exec(`INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,source_id,revision,deleted,body,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`,
		"a", "worldbook", "worldbook-preview", "role", "", 1, 0, json.RawMessage(`{"scope":"role","matchType":"keyword","matchPattern":"晨星","matchScope":"full_context","injectContent":"晨星计划属于角色设定","priority":5,"enabled":true}`), "2026-10-21T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	var captured coordination.DataSnapshot
	model.generate = func(_ context.Context, inference Inference) (Generation, error) {
		captured = inference.Snapshot
		return Generation{Text: "reply"}, nil
	}
	_, err := engine.PreviewReply(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "preview-worldbook", Message: "晨星"})
	if err != nil {
		t.Fatal(err)
	}
	for _, resource := range captured.Resources {
		if resource.Kind == "worldbook" && resource.ID == "worldbook-preview" {
			return
		}
	}
	t.Fatalf("preview snapshot did not include worldbook: %+v", captured.Resources)
}
