package main

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

func newDeviceOwnedDataHandler(services *AppServices, dataDir string) (agent.CancellableRuntimeInvokeHandler, error) {
	if services == nil || services.KernelContainer == nil || services.KernelContainer.DeviceRegistry == nil {
		return nil, errors.New("设备数据注册表不可用")
	}
	identity, err := agent.NewIdentityStore(dataDir).Load()
	if err != nil {
		return nil, err
	}
	port, err := newMeshLocalDataPort(services, dataDir, identity.DeviceID.String())
	if err != nil {
		return nil, err
	}
	credentials := agent.NewCredentialStore(dataDir)
	return func(ctx context.Context, invocation protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
		var req struct {
			CancelledCallIDs         []string                                 `json:"cancelledCallIds"`
			FencedDeviceID           string                                   `json:"fencedDeviceId"`
			ClosedPermissionRevision int64                                    `json:"closedPermissionRevision"`
			Operation                string                                   `json:"operation"`
			Scope                    coordination.ExecutionScope              `json:"scope"`
			TaskRead                 coordination.TaskReadProof               `json:"taskRead"`
			TaskPermission           task_runtime.SourceTaskPermissionRequest `json:"taskPermission"`
			TaskCatalog              task_runtime.DeviceTaskCatalogRequest    `json:"taskCatalog"`
			Commit                   coordination.Commit                      `json:"commit"`
			Kind                     string                                   `json:"kind"`
			RoleID                   string                                   `json:"roleId"`
			Query                    coordination.DataQuery                   `json:"query"`
			Interrupted              coordination.InterruptedReply            `json:"interrupted"`
		}
		if len(invocation.Input) > 4<<20 {
			return nil, coordination.ErrPendingLimit
		}
		if err := json.Unmarshal(invocation.Input, &req); err != nil {
			return nil, err
		}
		cred, err := credentials.LoadCredential()
		if err != nil {
			return nil, err
		}
		candidateRoles := false
		if req.Operation == "roles" && (cred == nil || invocation.SpaceID != cred.SpaceID) {
			candidate, candidateErr := credentials.LoadCandidate()
			if candidateErr != nil {
				return nil, candidateErr
			}
			if candidate != nil && candidate.SpaceID == invocation.SpaceID {
				cred = candidate
				candidateRoles = true
			}
		}
		scope := req.Scope
		if req.Operation == "apply" {
			scope = req.Commit.Scope
		}
		if req.Operation == "interrupted" {
			scope = req.Interrupted.Scope
		}
		if cred == nil || cred.ExpiresAt.Before(time.Now()) || invocation.SpaceID != cred.SpaceID || invocation.DeviceID != identity.DeviceID || invocation.RuntimeID != identity.RuntimeID || scope.SpaceID != cred.SpaceID.String() || scope.CoreID != cred.SpaceID.String() || scope.TargetDeviceID != identity.DeviceID.String() {
			return nil, coordination.ErrWrongOwner
		}
		if req.Operation != "task-definition" && req.Operation != "task-catalog-entry" && req.Operation != "task-permissions" && req.Operation != "task-catalog" && req.Operation != "task-history-resource" && req.Operation != "history" && req.Operation != "history-list" && req.Operation != "history-list-page" && req.Operation != "history-roles" && (scope.ResourceOwnerID != identity.DeviceID.String() || scope.RoleOwnerID != identity.DeviceID.String()) {
			return nil, coordination.ErrWrongOwner
		}
		var result any
		dispatch := func(ctx context.Context) error {
			if req.Operation == "authority-reconcile" {
				if err := coordination.CancelSourceAuthority(ctx, services.KernelContainer.DeviceRegistry.Database(), scope.SpaceID, req.CancelledCallIDs); err != nil {
					return err
				}
				result = map[string]any{"cancelledCallIds": req.CancelledCallIDs}
				return nil
			}
			if req.Operation == "authority-fence" {
				if err := coordination.FenceSourceAuthority(ctx, services.KernelContainer.DeviceRegistry.Database(), scope.SpaceID, req.FencedDeviceID, req.ClosedPermissionRevision); err != nil {
					return err
				}
				result = map[string]any{"deviceId": req.FencedDeviceID, "closedPermissionRevision": req.ClosedPermissionRevision}
				return nil
			}
			if !candidateRoles {
				if err := coordination.ValidateSourceCall(ctx, services.KernelContainer.DeviceRegistry.Database(), scope.SpaceID, invocation.InvocationID); err != nil {
					return err
				}
				if err := coordination.ValidateSourceAuthority(ctx, services.KernelContainer.DeviceRegistry.Database(), scope); err != nil {
					return err
				}
			}
			switch req.Operation {
			case "task-catalog":
				owner := identity.DeviceID.String()
				if scope.Coordinated {
					owner = scope.CoreID
				}
				if scope.ResourceOwnerID != owner || scope.RoleOwnerID != owner || services.KernelContainer.TaskRuntimeService == nil {
					return coordination.ErrWrongOwner
				}
				list := func() error {
					result, err = services.KernelContainer.TaskRuntimeService.DescribeInstalledTaskCatalog(ctx, identity.DeviceID.String(), req.TaskCatalog)
					if err != nil {
						return err
					}
					if err := coordination.ValidateSourceCall(ctx, services.KernelContainer.DeviceRegistry.Database(), scope.SpaceID, invocation.InvocationID); err != nil {
						return err
					}
					return coordination.ValidateSourceAuthority(ctx, services.KernelContainer.DeviceRegistry.Database(), scope)
				}
				if scope.Coordinated {
					err = list()
				} else {
					err = port.WithSourceRole(ctx, scope, list)
				}
			case "task-permissions":
				owner := identity.DeviceID.String()
				if scope.Coordinated {
					owner = scope.CoreID
				}
				if scope.ResourceOwnerID != owner || scope.RoleOwnerID != owner || req.TaskPermission.Scope != scope || services.KernelContainer.TaskRuntimeService == nil {
					return coordination.ErrWrongOwner
				}
				permissionTarget := req.TaskPermission.Run.ExecutionTarget
				if permissionTarget.RuntimeID != invocation.RuntimeID || permissionTarget.RuntimeSessionID != invocation.RuntimeSessionID || permissionTarget.ConnectionGeneration != invocation.ConnectionGeneration {
					return coordination.ErrScopeExpired
				}
				check := func() error {
					result, err = services.KernelContainer.TaskRuntimeService.CheckInstalledTaskPermissions(coordination.WithScope(ctx, scope), req.TaskPermission)
					if err != nil {
						return err
					}
					if err := coordination.ValidateSourceCall(ctx, services.KernelContainer.DeviceRegistry.Database(), scope.SpaceID, invocation.InvocationID); err != nil {
						return err
					}
					return coordination.ValidateSourceAuthority(ctx, services.KernelContainer.DeviceRegistry.Database(), scope)
				}
				if scope.Coordinated {
					err = check()
				} else {
					err = port.WithSourceRole(ctx, scope, check)
				}
			case "task-definition", "task-catalog-entry":
				owner := identity.DeviceID.String()
				if scope.Coordinated {
					owner = scope.CoreID
				}
				if scope.ResourceOwnerID != owner || scope.RoleOwnerID != owner || services.KernelContainer.TaskRuntimeService == nil {
					return coordination.ErrWrongOwner
				}
				describe := func() error {
					if req.Operation == "task-catalog-entry" {
						result, err = services.KernelContainer.TaskRuntimeService.DescribeInstalledTaskCatalogEntry(ctx, identity.DeviceID.String(), req.Query.ResourceID)
					} else {
						result, err = services.KernelContainer.TaskRuntimeService.DescribeInstalledTask(ctx, req.Query.ResourceID, identity.DeviceID.String())
					}
					if err != nil {
						return err
					}
					if err := coordination.ValidateSourceCall(ctx, services.KernelContainer.DeviceRegistry.Database(), scope.SpaceID, invocation.InvocationID); err != nil {
						return err
					}
					return coordination.ValidateSourceAuthority(ctx, services.KernelContainer.DeviceRegistry.Database(), scope)
				}
				if !scope.Coordinated {
					err = port.WithSourceRole(ctx, scope, describe)
				} else {
					err = describe()
				}
			case "interrupted":
				if scope.Coordinated {
					return coordination.ErrWrongOwner
				}
				err = port.SaveInterrupted(ctx, req.Interrupted)
				result = map[string]bool{"saved": err == nil}
			case "apply":
				if scope.Coordinated {
					return coordination.ErrWrongOwner
				}
				result, err = port.Commit(coordination.WithSourceCommitAuthority(ctx, scope, invocation.InvocationID), req.Commit)
			case "roles":
				result, err = port.Roles(ctx, scope)
			case "projection-status", "projection-rebuild":
				result, err = port.ProjectionState(ctx, scope, req.Operation == "projection-rebuild")
			case "snapshot":
				result, err = port.Snapshot(ctx, scope, req.Query)
			case "resource":
				result, err = port.Resource(ctx, scope, req.Query.ResourceKind, req.Query.ResourceID)
			case "task-history-resource":
				result, err = port.ReadTaskResource(ctx, scope, req.TaskRead, req.Query.ResourceKind, req.Query.ResourceID)
				if err == nil {
					err = coordination.ValidateSourceCall(ctx, services.KernelContainer.DeviceRegistry.Database(), scope.SpaceID, invocation.InvocationID)
				}
				if err == nil {
					err = coordination.ValidateSourceAuthority(ctx, services.KernelContainer.DeviceRegistry.Database(), scope)
				}
			case "history":
				if !scope.Coordinated || scope.ResourceOwnerID != scope.CoreID || scope.RoleOwnerID != scope.CoreID {
					return coordination.ErrWrongOwner
				}
				result, err = port.HistoricalSnapshot(ctx, scope, req.Query)
			case "history-list":
				if !scope.Coordinated || scope.ResourceOwnerID != scope.CoreID || scope.RoleOwnerID != scope.CoreID {
					return coordination.ErrWrongOwner
				}
				result, err = port.HistoricalConversations(ctx, scope)
			case "history-list-page":
				result, err = port.HistoricalConversationPage(ctx, scope, req.Query)
			case "history-roles":
				result, err = port.HistoricalRoles(ctx, scope)
			case "list":
				if req.RoleID != scope.RoleID {
					return coordination.ErrWrongOwner
				}
				if err := coordination.ValidateRoleRevision(ctx, port, scope); err != nil {
					return err
				}
				result, err = port.store.List(ctx, req.Kind, req.RoleID, false)
			default:
				return errors.New("设备数据操作不受支持")
			}
			return err
		}
		if candidateRoles {
			err = dispatch(ctx)
		} else {
			err = credentials.WithActiveCredential(ctx, cred, dispatch)
		}
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		if len(encoded) > 4<<20 {
			return nil, coordination.ErrPendingLimit
		}
		return &protocol.RuntimeResultPayload{InvocationID: invocation.InvocationID, Result: encoded}, nil
	}, nil
}
