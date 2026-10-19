package business

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedMessageSequenceRemainsMonotonicAfterGenerationFailure(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "conversation", RequestID: "first", Message: "first"}
	if _, err := engine.Run(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	model.generate = func(context.Context, Inference) (Generation, error) {
		return Generation{}, errors.New("provider failure")
	}
	request.RequestID, request.Message = "failed", "second"
	if _, err := engine.Run(t.Context(), request); err == nil {
		t.Fatal("provider failure reported as successful")
	}
	model.generate = nil
	request.RequestID, request.Message = "third", "third"
	if _, err := engine.Run(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	rows, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), "message", "role", false)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int64]bool{}
	for _, row := range rows {
		var item struct {
			Sequence int64 `json:"sequence"`
		}
		if err := json.Unmarshal(row.Body, &item); err != nil {
			t.Fatal(err)
		}
		if item.Sequence < 1 || seen[item.Sequence] {
			t.Fatalf("invalid or repeated message sequence: %d", item.Sequence)
		}
		seen[item.Sequence] = true
	}
	if len(seen) != 5 || !seen[1] || !seen[2] || !seen[3] || !seen[5] || !seen[6] {
		t.Fatalf("order lost after failure: %v", seen)
	}
}
