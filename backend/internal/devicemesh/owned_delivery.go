package devicemesh

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func (rt *Runtime) CommitDeviceData(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	if rt.Coordination == nil {
		return coordination.Acknowledgement{}, errors.New("设备数据服务未初始化")
	}
	if err := rt.Coordination.Enqueue(ctx, commit); err != nil {
		return coordination.Acknowledgement{}, err
	}
	pending, err := rt.Coordination.Pending(ctx, commit.Scope.ResourceOwnerID)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	for _, item := range pending {
		if item.Commit.Scope.RequestID == commit.Scope.RequestID {
			ack, deliveryErr := rt.deliverDeviceData(ctx, item)
			if errors.Is(deliveryErr, coordination.ErrResourceVersion) || errors.Is(deliveryErr, coordination.ErrRequestConflict) {
				if rejected := rt.Coordination.RejectPending(ctx, item, deliveryErr); rejected != nil {
					return ack, errors.Join(deliveryErr, rejected)
				}
				return ack, errors.Join(deliveryErr, coordination.ErrDeliveryRejected)
			}
			return ack, deliveryErr
		}
	}
	return coordination.Acknowledgement{}, errors.New("设备数据正在等待投递")
}

func (rt *Runtime) deliverDeviceData(ctx context.Context, pending coordination.PendingCommit) (coordination.Acknowledgement, error) {
	scope := pending.Commit.Scope
	if err := rt.Coordination.Validate(ctx, scope); err != nil {
		return coordination.Acknowledgement{}, err
	}
	if err := coordination.ValidateRoleRevision(ctx, rt, scope); err != nil {
		return coordination.Acknowledgement{}, err
	}
	if err := rt.DeviceReg.RequireTrustedDevice(ctx, runtimeidentity.SpaceID(scope.SpaceID), runtimeidentity.DeviceID(scope.ResourceOwnerID)); err != nil {
		return coordination.Acknowledgement{}, err
	}
	payload, err := json.Marshal(map[string]any{"operation": "apply", "commit": pending.Commit})
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	result, err := rt.InvokeDeviceHandlerWithRuntimeType(ctx, runtimeidentity.SpaceID(scope.SpaceID), runtimeidentity.DeviceID(scope.ResourceOwnerID), capability.RuntimeTypeInternal, "coordination.data", payload, 30*time.Second)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	var ack coordination.Acknowledgement
	if err := json.Unmarshal(result.Structured, &ack); err != nil {
		return ack, err
	}
	if err := rt.Coordination.Validate(ctx, scope); err != nil {
		return ack, err
	}
	if ack.RequestID != scope.RequestID || ack.OwnerID != scope.ResourceOwnerID {
		return ack, coordination.ErrRequestConflict
	}
	for _, mutation := range pending.Commit.Mutations {
		if ack.Versions[mutation.Kind+"/"+mutation.ID] != mutation.ExpectedRevision+1 {
			return ack, coordination.ErrResourceVersion
		}
	}
	if err := rt.Coordination.Acknowledge(ctx, ack, pending.Hash); err != nil {
		return ack, err
	}
	return ack, nil
}

func (rt *Runtime) ResumeDeviceData(ctx context.Context, device string) error {
	pending, err := rt.Coordination.Pending(ctx, device)
	if err != nil {
		return err
	}
	for _, item := range pending {
		if _, err := rt.deliverDeviceData(ctx, item); err != nil {
			if errors.Is(err, coordination.ErrResourceVersion) || errors.Is(err, coordination.ErrRequestConflict) {
				if rejected := rt.Coordination.RejectPending(ctx, item, err); rejected != nil {
					return errors.Join(err, rejected)
				}
				continue
			}
			if errors.Is(err, coordination.ErrScopeExpired) || errors.Is(err, coordination.ErrWrongOwner) || errors.Is(err, coordination.ErrRoleRequired) {
				if discardErr := rt.Coordination.DiscardPending(ctx, item, rt); discardErr != nil {
					return errors.Join(err, discardErr)
				}
				continue
			}
			if ctx.Err() == nil {
				if retryErr := rt.Coordination.RetryLater(ctx, item); retryErr != nil {
					return errors.Join(err, retryErr)
				}
			}
			return err
		}
	}
	return nil
}

func (rt *Runtime) runDeviceDataDelivery(ctx context.Context, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if sources, err := rt.Coordination.PendingRemoteSources(ctx); err == nil {
				for _, source := range sources {
					if _, connected := rt.Hub.GetByDevice(runtimeidentity.SpaceID(source.SpaceID), runtimeidentity.DeviceID(source.DeviceID)); connected {
						_ = rt.ReconcileRemoteDevice(ctx, source.SpaceID, source.DeviceID)
					}
					if ctx.Err() != nil {
						return
					}
				}
			}
			owners, err := rt.Coordination.PendingOwners(ctx)
			if err != nil {
				continue
			}
			for _, owner := range owners {
				pending, err := rt.Coordination.Pending(ctx, owner)
				if err != nil || len(pending) == 0 {
					continue
				}
				if _, connected := rt.Hub.GetByDevice(runtimeidentity.SpaceID(pending[0].Commit.Scope.SpaceID), runtimeidentity.DeviceID(owner)); connected {
					_ = rt.ResumeDeviceData(ctx, owner)
				}
				if ctx.Err() != nil {
					return
				}
			}
		}
	}
}
