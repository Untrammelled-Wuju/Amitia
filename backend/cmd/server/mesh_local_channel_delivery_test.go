package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/delivery"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestMeshLocalChannelDeliveryRoleGuardDoesNotReenterAndIsIdempotent(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	p.services.DeliveryStore = delivery.NewSQLiteDeliveryStore(p.services.DB)
	scope := coordination.ExecutionScope{SpaceID: "core", CoreID: "core", InitiatorDeviceID: "device-a", TargetDeviceID: "device-a", RoleOwnerID: "device-a", ResourceOwnerID: "device-a", RoleID: "one", RoleRevision: 3, RequestID: "reply"}
	ctx := coordination.WithAdditionalGuard(coordination.WithScope(t.Context(), scope), func(current context.Context) error { return coordination.ValidateRoleRevision(current, p, scope) })
	request := coordination.ChannelDeliveryRequest{ConversationID: "conversation", RequestID: scope.RequestID, Channel: "telegram", PeerID: "peer", Text: "message"}
	result := make(chan error, 1)
	go func() {
		if err := p.SubmitChannelDelivery(ctx, scope, request); err != nil {
			result <- err
			return
		}
		result <- p.SubmitChannelDelivery(ctx, scope, request)
	}()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("role guard reentered the held role lock")
	}
	items, err := p.services.DeliveryStore.ListPending(10)
	if err != nil || len(items) != 1 {
		t.Fatalf("delivery not idempotent: %+v %v", items, err)
	}
	scope.RoleRevision++
	if err := p.SubmitChannelDelivery(coordination.WithScope(t.Context(), scope), scope, request); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("stale role delivered message: %v", err)
	}
}

func TestMeshLocalChannelDeliveryRechecksSourceCallAfterRemovingRoleGuard(t *testing.T) {
	for _, mode := range []string{"cancelled", "fenced"} {
		t.Run(mode, func(t *testing.T) {
			p := setupMeshLocalDataPort(t)
			p.services.DeliveryStore = delivery.NewSQLiteDeliveryStore(p.services.DB)
			scope := coordination.ExecutionScope{SpaceID: "core", CoreID: "core", InitiatorDeviceID: "caller", TargetDeviceID: "device-a", RoleOwnerID: "device-a", ResourceOwnerID: "device-a", RoleID: "one", RoleRevision: 3, RequestID: "reply", PermissionRevision: 1, TargetPermissionRevision: 1}
			db := p.services.KernelContainer.DeviceRegistry.Database()
			ctx := context.WithValue(coordination.WithScope(t.Context(), scope), channelDeliverySourceCallKey{}, "call")
			ctx = coordination.WithAdditionalGuard(ctx, func(context.Context) error {
				if mode == "cancelled" {
					return coordination.CancelSourceAuthority(t.Context(), db, scope.SpaceID, []string{"call"})
				}
				return coordination.FenceSourceAuthority(t.Context(), db, scope.SpaceID, scope.InitiatorDeviceID, 1)
			})
			err := p.SubmitChannelDelivery(ctx, scope, coordination.ChannelDeliveryRequest{ConversationID: "conversation", RequestID: scope.RequestID, Channel: "telegram", PeerID: "peer", Text: "private"})
			if !errors.Is(err, coordination.ErrScopeExpired) {
				t.Fatalf("invalidated source call delivered reply: %v", err)
			}
			items, err := p.services.DeliveryStore.ListPending(10)
			if err != nil || len(items) != 0 {
				t.Fatalf("rejected source created outbox payload: %+v %v", items, err)
			}
		})
	}
}
