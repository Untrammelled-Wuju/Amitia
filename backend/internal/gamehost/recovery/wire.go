package recovery

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type KernelRollbackAdapter struct {
	executeFn func(ctx context.Context, extensionID string, operationID RecoveryOperationID) (KernelRollbackResult, error)
}

func NewKernelRollbackAdapter(executeFn func(ctx context.Context, extensionID string, operationID RecoveryOperationID) (KernelRollbackResult, error)) *KernelRollbackAdapter {
	return &KernelRollbackAdapter{executeFn: executeFn}
}

func (a *KernelRollbackAdapter) RollbackPackage(ctx context.Context, extensionID string, operationID RecoveryOperationID) (*KernelRollbackResult, error) {
	if a.executeFn == nil {
		return &KernelRollbackResult{Success: false, Error: "rollback not configured"}, nil
	}
	result, err := a.executeFn(ctx, extensionID, operationID)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

type SupervisorViewAdapter struct {
	isQuarantinedFn  func(serviceID string) bool
	getRestartCountFn func(serviceID string) int
	getMaxRestartsFn  func(serviceID string) int
}

func NewSupervisorViewAdapter(
	isQuarantinedFn func(serviceID string) bool,
	getRestartCountFn func(serviceID string) int,
	getMaxRestartsFn func(serviceID string) int,
) *SupervisorViewAdapter {
	return &SupervisorViewAdapter{
		isQuarantinedFn:  isQuarantinedFn,
		getRestartCountFn: getRestartCountFn,
		getMaxRestartsFn:  getMaxRestartsFn,
	}
}

func (a *SupervisorViewAdapter) IsQuarantined(serviceID string) bool {
	if a.isQuarantinedFn != nil {
		return a.isQuarantinedFn(serviceID)
	}
	return false
}

func (a *SupervisorViewAdapter) GetRestartCount(serviceID string) int {
	if a.getRestartCountFn != nil {
		return a.getRestartCountFn(serviceID)
	}
	return 0
}

func (a *SupervisorViewAdapter) GetMaxRestarts(serviceID string) int {
	if a.getMaxRestartsFn != nil {
		return a.getMaxRestartsFn(serviceID)
	}
	return 3
}

type AuditSinkAdapter struct {
	recordFn func(event RecoveryAuditEvent)
}

func NewAuditSinkAdapter(recordFn func(event RecoveryAuditEvent)) *AuditSinkAdapter {
	return &AuditSinkAdapter{recordFn: recordFn}
}

func (a *AuditSinkAdapter) RecordRecovery(event RecoveryAuditEvent) {
	if a.recordFn != nil {
		a.recordFn(event)
	}
}

type SecretLeaseAdapter struct {
	revokeFn func(runtimeID string) int
	issueFn  func(ctx context.Context, req SecretLeaseRequest) (SecretLeaseResult, error)
}

func NewSecretLeaseAdapter(
	revokeFn func(runtimeID string) int,
	issueFn func(ctx context.Context, req SecretLeaseRequest) (SecretLeaseResult, error),
) *SecretLeaseAdapter {
	return &SecretLeaseAdapter{revokeFn: revokeFn, issueFn: issueFn}
}

func (a *SecretLeaseAdapter) RevokeByRuntimeInstance(runtimeID string) int {
	if a.revokeFn != nil {
		return a.revokeFn(runtimeID)
	}
	return 0
}

func (a *SecretLeaseAdapter) IssueLease(ctx context.Context, req SecretLeaseRequest) (SecretLeaseResult, error) {
	if a.issueFn != nil {
		return a.issueFn(ctx, req)
	}
	return SecretLeaseResult{Success: false, Error: "not configured"}, nil
}

type HostStructureBuilderAdapter struct {
	rebuildTopologyFn func(ctx context.Context, pluginID domain.PluginID, extensionID string) (TopologyResult, error)
	rebuildLifecycleFn func(ctx context.Context, topology TopologyResult) (LifecycleResult, error)
}

func NewHostStructureBuilderAdapter(
	rebuildTopologyFn func(ctx context.Context, pluginID domain.PluginID, extensionID string) (TopologyResult, error),
	rebuildLifecycleFn func(ctx context.Context, topology TopologyResult) (LifecycleResult, error),
) *HostStructureBuilderAdapter {
	return &HostStructureBuilderAdapter{
		rebuildTopologyFn:  rebuildTopologyFn,
		rebuildLifecycleFn: rebuildLifecycleFn,
	}
}

func (a *HostStructureBuilderAdapter) RebuildTopology(ctx context.Context, pluginID domain.PluginID, extensionID string) (TopologyResult, error) {
	if a.rebuildTopologyFn != nil {
		return a.rebuildTopologyFn(ctx, pluginID, extensionID)
	}
	return TopologyResult{Valid: false}, nil
}

func (a *HostStructureBuilderAdapter) RebuildLifecyclePlan(ctx context.Context, topology TopologyResult) (LifecycleResult, error) {
	if a.rebuildLifecycleFn != nil {
		return a.rebuildLifecycleFn(ctx, topology)
	}
	return LifecycleResult{Valid: false}, nil
}

type RuntimeExecutorAdapter struct {
	startFn func(ctx context.Context, runtimeID domain.RuntimeInstanceID) error
	stopFn  func(ctx context.Context, runtimeID domain.RuntimeInstanceID) error
}

func NewRuntimeExecutorAdapter(
	startFn func(ctx context.Context, runtimeID domain.RuntimeInstanceID) error,
	stopFn func(ctx context.Context, runtimeID domain.RuntimeInstanceID) error,
) *RuntimeExecutorAdapter {
	return &RuntimeExecutorAdapter{startFn: startFn, stopFn: stopFn}
}

func (a *RuntimeExecutorAdapter) StartRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	if a.startFn != nil {
		return a.startFn(ctx, runtimeID)
	}
	return nil
}

func (a *RuntimeExecutorAdapter) StopRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	if a.stopFn != nil {
		return a.stopFn(ctx, runtimeID)
	}
	return nil
}

type PermissionServiceAdapter struct {
	ResolveFn func(ctx context.Context, runtimeID, pluginID string) (PermissionView, error)
}

func (a *PermissionServiceAdapter) ResolveRuntimePermissions(ctx context.Context, runtimeID, pluginID string) (PermissionView, error) {
	if a.ResolveFn != nil {
		return a.ResolveFn(ctx, runtimeID, pluginID)
	}
	return PermissionView{}, fmt.Errorf("permission service not configured")
}

type ControlAuthorityViewAdapter struct {
	GetAuthorityFn func(runtimeID domain.RuntimeInstanceID) (AuthoritySnapshot, error)
}

func (a *ControlAuthorityViewAdapter) GetAuthority(runtimeID domain.RuntimeInstanceID) (AuthoritySnapshot, error) {
	if a.GetAuthorityFn != nil {
		return a.GetAuthorityFn(runtimeID)
	}
	return AuthoritySnapshot{RuntimeID: runtimeID, Mode: "standard", Epoch: 1}, nil
}

type CheckpointStoreAdapter struct {
	hasMetadataFn func(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error)
	loadMetadataFn func(ctx context.Context, runtimeID domain.RuntimeInstanceID) (RuntimeMetadataView, error)
	loadCheckpointFn func(ctx context.Context, runtimeID domain.RuntimeInstanceID) (RuntimeCheckpointView, error)
}

func NewCheckpointStoreAdapter(
	hasMetadataFn func(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error),
	loadMetadataFn func(ctx context.Context, runtimeID domain.RuntimeInstanceID) (RuntimeMetadataView, error),
	loadCheckpointFn func(ctx context.Context, runtimeID domain.RuntimeInstanceID) (RuntimeCheckpointView, error),
) *CheckpointStoreAdapter {
	return &CheckpointStoreAdapter{
		hasMetadataFn:    hasMetadataFn,
		loadMetadataFn:   loadMetadataFn,
		loadCheckpointFn: loadCheckpointFn,
	}
}

func (a *CheckpointStoreAdapter) HasMetadata(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error) {
	if a.hasMetadataFn != nil {
		return a.hasMetadataFn(ctx, runtimeID)
	}
	return false, nil
}

func (a *CheckpointStoreAdapter) LoadMetadata(ctx context.Context, runtimeID domain.RuntimeInstanceID) (RuntimeMetadataView, error) {
	if a.loadMetadataFn != nil {
		return a.loadMetadataFn(ctx, runtimeID)
	}
	return RuntimeMetadataView{}, fmt.Errorf("metadata read not configured")
}

func (a *CheckpointStoreAdapter) LoadCheckpoint(ctx context.Context, runtimeID domain.RuntimeInstanceID) (RuntimeCheckpointView, error) {
	if a.loadCheckpointFn != nil {
		return a.loadCheckpointFn(ctx, runtimeID)
	}
	return RuntimeCheckpointView{}, fmt.Errorf("checkpoint read not configured")
}
