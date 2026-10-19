package chat

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type ownedPetLifecycleProbe struct{ events []DesktopPetChatLifecycle }

func (p *ownedPetLifecycleProbe) OnDesktopPetChatLifecycle(_ context.Context, event DesktopPetChatLifecycle) {
	p.events = append(p.events, event)
}

func (*ownedPetLifecycleProbe) OnDesktopPetToolLifecycle(context.Context, DesktopPetToolLifecycle) {}

func TestOwnedChatLifecyclePreservesOwnerAndOrdering(t *testing.T) {
	probe := &ownedPetLifecycleProbe{}
	s := &service{desktopPetLifecycle: probe}
	inference := business.Inference{Scope: coordination.ExecutionScope{SpaceID: "space", ResourceOwnerID: "device", RoleID: "role", RequestID: "request"}, ConversationID: "conversation", Ingress: &business.Ingress{Source: "plugin"}}
	s.beginOwnedChatLifecycle(t.Context(), inference)
	s.emitOwnedChatLifecycle(t.Context(), inference, "reply", "response.completed", 5)
	if len(probe.events) != 3 {
		t.Fatalf("missing events: %+v", probe.events)
	}
	for index, expected := range []string{"message.received", "context.loading", "response.completed"} {
		event := probe.events[index]
		if event.Phase != expected || event.CharacterID != "role" || event.SpaceID != "space" || event.ConversationID != "conversation" || event.InteractionID != "request" || event.Origin != "plugin" {
			t.Fatalf("bad lifecycle event: %+v", event)
		}
	}
}

func TestOwnedChatLifecycleSuppressesExpiredAndInternalEvents(t *testing.T) {
	probe := &ownedPetLifecycleProbe{}
	s := &service{desktopPetLifecycle: probe}
	inference := business.Inference{}
	ctx := coordination.WithAdditionalGuard(t.Context(), func(context.Context) error { return coordination.ErrScopeExpired })
	s.beginOwnedChatLifecycle(ctx, inference)
	inference.InternalWorker = true
	s.beginOwnedChatLifecycle(t.Context(), inference)
	inference.InternalWorker, inference.Preview = false, true
	s.beginOwnedChatLifecycle(t.Context(), inference)
	if len(probe.events) != 0 {
		t.Fatalf("unexpected events: %+v", probe.events)
	}
}

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
