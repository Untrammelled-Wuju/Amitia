package interaction

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/auth"
)

func TestUnifiedEntryRequestOriginActorTakesPrecedence(t *testing.T) {
	requestActor := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "request-space"}
	request := &UnifiedEntryRequest{OriginActor: requestActor}
	actor := request.OriginActor
	if actor == nil {
		actor, _ = auth.FromContext(context.Background())
	}
	if actor != requestActor {
		t.Fatalf("request actor was not preserved: %#v", actor)
	}
}

func TestUnifiedEntryRequestFallsBackToContextActor(t *testing.T) {
	contextActor := &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: "context-space"}
	ctx := auth.WithActor(context.Background(), contextActor)
	request := &UnifiedEntryRequest{}
	actor := request.OriginActor
	if actor == nil {
		actor, _ = auth.FromContext(ctx)
	}
	if actor != contextActor {
		t.Fatalf("context actor was not preserved: %#v", actor)
	}
}
