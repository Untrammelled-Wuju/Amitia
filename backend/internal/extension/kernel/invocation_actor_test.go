package kernel

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/auth"
	coreexec "github.com/u-ai/backend/internal/execution"
)

func TestWithInvocationOriginActorRestoresAuthenticatedDeviceContext(t *testing.T) {
	scope := InvocationScope{
		SpaceID: "space-a",
		ExecContext: &coreexec.ExecutionContext{
			Metadata: map[string]any{
				coreexec.MetadataOriginPrincipalType:    string(auth.PrincipalTrustedDevice),
				coreexec.MetadataOriginDeviceID:         "device-a",
				coreexec.MetadataOriginRuntimeID:        "runtime-a",
				coreexec.MetadataOriginSessionID:        "session-a",
				coreexec.MetadataOriginRuntimeSessionID: "runtime-session-a",
			},
		},
	}
	ctx := withInvocationOriginActor(context.Background(), scope)
	actor, ok := auth.FromContext(ctx)
	if !ok || actor == nil {
		t.Fatal("expected origin actor to be restored")
	}
	if actor.PrincipalType != auth.PrincipalTrustedDevice || actor.DeviceID != "device-a" ||
		actor.SpaceID != "space-a" || actor.SessionID != "session-a" {
		t.Fatalf("unexpected restored actor: %#v", actor)
	}
}

func TestWithInvocationOriginActorDoesNotReplaceExistingActor(t *testing.T) {
	existing := &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: "space-existing"}
	ctx := auth.WithActor(context.Background(), existing)
	restored := withInvocationOriginActor(ctx, InvocationScope{})
	actor, ok := auth.FromContext(restored)
	if !ok || actor != existing {
		t.Fatalf("existing actor was replaced: %#v", actor)
	}
}
