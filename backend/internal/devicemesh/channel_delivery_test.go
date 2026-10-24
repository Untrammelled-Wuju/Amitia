package devicemesh_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type channelRecoveryPort struct {
	pending      []coordination.PendingChannelDelivery
	roleRevision int64
	submitted    int
	completed    int
	beforeSubmit func()
}

func (p *channelRecoveryPort) Roles(context.Context, coordination.ExecutionScope) ([]coordination.Role, error) {
	return []coordination.Role{{ID: "role", Revision: p.roleRevision, Profile: json.RawMessage(`{}`)}}, nil
}

func (p *channelRecoveryPort) Snapshot(context.Context, coordination.ExecutionScope, coordination.DataQuery) (coordination.DataSnapshot, error) {
	return coordination.DataSnapshot{}, nil
}

func (p *channelRecoveryPort) Commit(context.Context, coordination.Commit) (coordination.Acknowledgement, error) {
	return coordination.Acknowledgement{}, nil
}

func (p *channelRecoveryPort) PendingChannelDeliveries(_ context.Context, cursor string) ([]coordination.PendingChannelDelivery, error) {
	var page []coordination.PendingChannelDelivery
	for _, item := range p.pending {
		if item.ID > cursor {
			item.Cursor = item.ID
			page = append(page, item)
			if len(page) == 64 {
				break
			}
		}
	}
	return page, nil
}

func (p *channelRecoveryPort) CompleteChannelDelivery(ctx context.Context, pending coordination.PendingChannelDelivery) error {
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	p.completed++
	p.pending = nil
	return nil
}

func (p *channelRecoveryPort) SubmitChannelDelivery(ctx context.Context, scope coordination.ExecutionScope, request coordination.ChannelDeliveryRequest) error {
	if current, ok := coordination.FromContext(ctx); !ok || current != scope {
		return coordination.ErrWrongOwner
	}
	if p.beforeSubmit != nil {
		p.beforeSubmit()
	}
	return coordination.CommitCurrent(ctx, func() error { p.submitted++; return nil })
}

func TestChannelDeliveryRecoveryRestoresOwnerAndRevocationGuard(t *testing.T) {
	for _, mode := range []string{"success", "coordinated", "cloud-target-offline", "changed-role", "revoked-before", "revoked-during"} {
		t.Run(mode, func(t *testing.T) {
			db := meshAuthorityDB(t)
			registry := host_registry.NewRegistry(db)
			if _, err := registry.EnsureDevice(t.Context(), host_registry.DeviceRecord{SpaceID: "core", DeviceID: "phone", Platform: runtimeidentity.PlatformAndroid, TrustState: host_registry.DeviceTrustTrusted}); err != nil {
				t.Fatal(err)
			}
			rt, err := devicemesh.NewCloudRuntime(db, registry)
			if err != nil {
				t.Fatal(err)
			}
			rt.LocalDeviceID = "phone"
			if mode == "coordinated" || mode == "cloud-target-offline" {
				if _, err := rt.Coordination.InitializeCoordinated(t.Context(), "core", "phone", 1, "role"); err != nil {
					t.Fatal(err)
				}
			}
			_, scope, finish, err := rt.Coordination.Begin(t.Context(), "core", "phone", "phone", "core", "role", "request")
			if err != nil {
				t.Fatal(err)
			}
			finish()
			scope.RoleRevision, scope.ExecutionID, scope.TurnID = 1, "execution", "turn"
			port := &channelRecoveryPort{roleRevision: 1, pending: []coordination.PendingChannelDelivery{{Pending: true, Scope: scope, ID: "channel-delivery/request", Revision: 1, Request: coordination.ChannelDeliveryRequest{RequestID: "request", ConversationID: "conversation", Channel: "telegram", PeerID: "peer", Text: "reply"}}}}
			rt.LocalDeviceDataPort = port
			if scope.Coordinated {
				rt.CoreDataPort = port
			}
			if mode == "cloud-target-offline" {
				rt.LocalDeviceID = "cloud-host"
			}
			revoke := func() {
				if _, err := db.ExecContext(t.Context(), `UPDATE kernel_devices SET trust_state='revoked' WHERE device_id='phone'`); err != nil {
					t.Fatal(err)
				}
			}
			switch mode {
			case "changed-role":
				port.roleRevision = 2
			case "revoked-before":
				revoke()
			case "revoked-during":
				port.beforeSubmit = revoke
			}
			if err := rt.ResumeChannelDeliveries(t.Context()); err != nil {
				t.Fatal(err)
			}
			if mode == "success" || mode == "coordinated" {
				if port.submitted != 1 || port.completed != 1 {
					t.Fatalf("delivery not recovered exactly once: %+v", port)
				}
				if err := rt.ResumeChannelDeliveries(t.Context()); err != nil {
					t.Fatal(err)
				}
				if port.submitted != 1 {
					t.Fatal("completed delivery was dispatched again")
				}
			} else if port.submitted != 0 || port.completed != 0 {
				t.Fatalf("stale authority delivered reply: %+v", port)
			}
		})
	}
}

func TestChannelDeliveryRecoveryContinuesPastStalePage(t *testing.T) {
	db := meshAuthorityDB(t)
	registry := host_registry.NewRegistry(db)
	if _, err := registry.EnsureDevice(t.Context(), host_registry.DeviceRecord{SpaceID: "core", DeviceID: "phone", Platform: runtimeidentity.PlatformAndroid, TrustState: host_registry.DeviceTrustTrusted}); err != nil {
		t.Fatal(err)
	}
	rt, err := devicemesh.NewCloudRuntime(db, registry)
	if err != nil {
		t.Fatal(err)
	}
	rt.LocalDeviceID = "phone"
	port := &channelRecoveryPort{roleRevision: 1}
	rt.LocalDeviceDataPort = port
	for i := 0; i < 65; i++ {
		id := fmt.Sprintf("request-%03d", i)
		_, scope, finish, err := rt.Coordination.Begin(t.Context(), "core", "phone", "phone", "core", "role", id)
		if err != nil {
			t.Fatal(err)
		}
		finish()
		scope.RoleRevision, scope.ExecutionID, scope.TurnID = 2, id, id
		if i == 64 {
			scope.RoleRevision = 1
		}
		port.pending = append(port.pending, coordination.PendingChannelDelivery{Pending: true, Scope: scope, ID: "channel-delivery/" + id, Revision: 1, Request: coordination.ChannelDeliveryRequest{RequestID: id, ConversationID: "conversation", Channel: "telegram", PeerID: "peer", Text: "reply"}})
	}
	if err := rt.ResumeChannelDeliveries(t.Context()); err != nil {
		t.Fatal(err)
	}
	if port.submitted != 1 || port.completed != 1 {
		t.Fatalf("stale first page blocked a valid later delivery: submitted=%d completed=%d", port.submitted, port.completed)
	}
}
