package chat

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedTitleSkipsCompletedHistoryBeforeCallingModel(t *testing.T) {
	role := coordination.Role{ID: "role", Revision: 1, Profile: json.RawMessage(`{"characterId":"role"}`)}
	inference := business.Inference{Scope: coordination.ExecutionScope{RoleID: "role", RoleRevision: 1, ResourceOwnerID: "device"}, ConversationID: "conversation", Message: "你好", Snapshot: coordination.DataSnapshot{OwnerID: "device", Role: role, LegacyMessages: []json.RawMessage{json.RawMessage(`{"role":"assistant","content":"已有回复"}`)}}}
	title, err := (&service{}).PlanOwnedTitle(t.Context(), inference, business.Generation{Text: "新回复"})
	if err != nil || title != "" {
		t.Fatalf("history triggered title: %q %v", title, err)
	}
	inference.Snapshot.LegacyMessages = nil
	ctx := coordination.WithAdditionalGuard(t.Context(), func(context.Context) error { return coordination.ErrScopeExpired })
	if _, err := (&service{}).PlanOwnedTitle(ctx, inference, business.Generation{Text: "新回复"}); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("expired title model access allowed: %v", err)
	}
}
