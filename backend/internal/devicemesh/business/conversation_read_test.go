package business

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestConversationReadAuthorizesOwnerAndTurn(t *testing.T) {
	engine, db, _, _ := engineHarness(t)
	if _, err := db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES('b','core','trusted','now','now')`); err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{"a", "b"} {
		request := Request{SpaceID: "core", DeviceID: owner, CoreID: "core", RoleID: "role", ConversationID: "same-id", RequestID: "turn-" + owner, Message: "hello"}
		if _, err := engine.Run(t.Context(), request); err != nil {
			t.Fatal(err)
		}
	}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "same-id", ConversationOrigin: &ConversationOrigin{OwnerID: "a", ID: "same-id"}}
	called := false
	read := func(ctx context.Context, scope coordination.ExecutionScope, id string) error {
		called = true
		for _, test := range []struct {
			request  string
			expected bool
		}{{"turn-a", true}, {"turn-b", false}, {"missing", false}} {
			allowed, err := engine.OwnsConversationTurn(ctx, scope, id, test.request)
			if err != nil || allowed != test.expected {
				t.Fatalf("turn ownership %s=%v: %v", test.request, allowed, err)
			}
		}
		return nil
	}
	if err := engine.WithConversationRead(t.Context(), request, read); err != nil || !called {
		t.Fatalf("read %v", err)
	}
	called = false
	request.ConversationOrigin.OwnerID = "b"
	if err := engine.WithConversationRead(t.Context(), request, read); !errors.Is(err, coordination.ErrWrongOwner) || called {
		t.Fatalf("foreign owner read: %v", err)
	}
	request.ConversationOrigin.OwnerID = "a"
	request.ConversationID, request.ConversationOrigin.ID = "missing", "missing"
	if err := engine.WithConversationRead(t.Context(), request, read); !errors.Is(err, ErrConversationUnavailable) || called {
		t.Fatalf("missing conversation read: %v", err)
	}
}

func TestConversationReadResolvesContinuationAndRejectsRevocation(t *testing.T) {
	engine, _, service, _ := engineHarness(t)
	if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
		t.Fatal(err)
	}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "source", RequestID: "continued", Message: "hello", ConversationOrigin: &ConversationOrigin{OwnerID: "a", ID: "source"}}
	response, err := engine.Run(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	err = engine.WithConversationRead(t.Context(), request, func(ctx context.Context, scope coordination.ExecutionScope, id string) error {
		if id != response.ConversationID || id == "source" || scope.ResourceOwnerID != "core" {
			t.Fatalf("wrong continuation: %s %+v", id, scope)
		}
		allowed, err := engine.OwnsConversationTurn(ctx, scope, id, "continued")
		if err != nil || !allowed {
			t.Fatalf("continuation proof: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = engine.WithConversationRead(t.Context(), request, func(ctx context.Context, scope coordination.ExecutionScope, id string) error {
		service.Invalidate(scope.SpaceID, scope.InitiatorDeviceID)
		return nil
	})
	if !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("revoked response accepted: %v", err)
	}
}
