package recovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type KernelRollback interface {
	RollbackPackage(ctx context.Context, extensionID string, operationID RecoveryOperationID) (*KernelRollbackResult, error)
}

type KernelRollbackResult struct {
	Success      bool
	NewVersion   string
	RequiresReconcile bool
	Error        string
}

type SupervisorView interface {
	IsQuarantined(serviceID string) bool
	GetRestartCount(serviceID string) int
	GetMaxRestarts(serviceID string) int
}

type PluginRegistryReader interface {
	ListByExtension(ctx context.Context, extensionID string) ([]domain.PluginDescriptor, error)
	Snapshot() []domain.PluginDescriptor
	Get(ctx context.Context, pluginID domain.PluginID) (domain.PluginDescriptor, error)
}

type RuntimeManagerReader interface {
	GetRuntime(runtimeID domain.RuntimeInstanceID) (*RuntimeInstanceRef, error)
	ListRuntimes() []*RuntimeInstanceRef
}

type RuntimeInstanceRef struct {
	ID       domain.RuntimeInstanceID
	PluginID domain.PluginID
	State    domain.RuntimeState
}

type RuntimeExecutor interface {
	StartRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error
	StopRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error
}

type SecretLeaseService interface {
	RevokeByRuntimeInstance(runtimeID string) int
	IssueLease(ctx context.Context, req SecretLeaseRequest) (SecretLeaseResult, error)
}

type SecretLeaseRequest struct {
	RuntimeInstanceID string
	PluginID         string
	ExtensionID      string
	InvocationID     string
}

type SecretLeaseResult struct {
	LeaseID string
	Success bool
	Error   string
}

type PermissionService interface {
	ResolveRuntimePermissions(ctx context.Context, runtimeID, pluginID string) (PermissionView, error)
}

type PermissionView struct {
	Revision    string
	Permissions []string
}

type ControlAuthorityView interface {
	GetAuthority(runtimeID domain.RuntimeInstanceID) (AuthoritySnapshot, error)
}

type AuthoritySnapshot struct {
	RuntimeID domain.RuntimeInstanceID
	Mode      string
	Epoch     uint64
}

type AuditSink interface {
	RecordRecovery(event RecoveryAuditEvent)
}

type RecoveryAuditEvent struct {
	OperationID  RecoveryOperationID
	RuntimeID    domain.RuntimeInstanceID
	ExtensionID  string
	PluginID     domain.PluginID
	FailureClass FailureClass
	Stage        RecoveryStage
	Attempt      int
	Result       string
	Error        string
	Timestamp    time.Time
}

type HostStructureBuilder interface {
	RebuildTopology(ctx context.Context, pluginID domain.PluginID, extensionID string) (TopologyResult, error)
	RebuildLifecyclePlan(ctx context.Context, topology TopologyResult) (LifecycleResult, error)
}

type TopologyResult struct {
	TopologyID    string
	ServiceIDs    []string
	Valid         bool
}

type LifecycleResult struct {
	PlanID        string
	Valid         bool
}

type RecoveryCoordinator struct {
	mu             sync.Mutex
	gate           *RecoveryGate
	classifier     *FailureClassifier
	checkpoint     CheckpointClassifier
	kernel         KernelRollback
	supervisor     SupervisorView
	pluginReg      PluginRegistryReader
	runtimeMgr     RuntimeManagerReader
	runtimeExecutor RuntimeExecutor
	secretLease    SecretLeaseService
	permission     PermissionService
	authority      ControlAuthorityView
	structureBuilder HostStructureBuilder
	audit          AuditSink
}

type RecoveryCoordinatorDeps struct {
	Kernel               KernelRollback
	Supervisor           SupervisorView
	PluginRegistry       PluginRegistryReader
	RuntimeManager       RuntimeManagerReader
	RuntimeExecutor      RuntimeExecutor
	SecretLease          SecretLeaseService
	Permission           PermissionService
	AuthorityView        ControlAuthorityView
	CheckpointClassifier CheckpointClassifier
	StructureBuilder     HostStructureBuilder
	AuditSink            AuditSink
}

func NewRecoveryCoordinator(deps RecoveryCoordinatorDeps) (*RecoveryCoordinator, error) {
	if deps.Kernel == nil {
		return nil, fmt.Errorf("recovery: Kernel rollback is required")
	}
	if deps.Supervisor == nil {
		return nil, fmt.Errorf("recovery: Supervisor view is required")
	}
	if deps.PluginRegistry == nil {
		return nil, fmt.Errorf("recovery: PluginRegistry is required")
	}
	if deps.RuntimeManager == nil {
		return nil, fmt.Errorf("recovery: RuntimeManager is required")
	}
	if deps.RuntimeExecutor == nil {
		return nil, fmt.Errorf("recovery: RuntimeExecutor is required")
	}
	if deps.SecretLease == nil {
		return nil, fmt.Errorf("recovery: SecretLease is required")
	}
	if deps.Permission == nil {
		return nil, fmt.Errorf("recovery: Permission is required")
	}
	if deps.AuthorityView == nil {
		return nil, fmt.Errorf("recovery: AuthorityView is required")
	}
	if deps.CheckpointClassifier == nil {
		return nil, fmt.Errorf("recovery: CheckpointClassifier is required")
	}
	if deps.StructureBuilder == nil {
		return nil, fmt.Errorf("recovery: StructureBuilder is required")
	}
	if deps.AuditSink == nil {
		return nil, fmt.Errorf("recovery: AuditSink is required")
	}

	c := &RecoveryCoordinator{
		gate:           NewRecoveryGate(),
		classifier:     NewFailureClassifier(),
		kernel:         deps.Kernel,
		supervisor:     deps.Supervisor,
		pluginReg:      deps.PluginRegistry,
		runtimeMgr:     deps.RuntimeManager,
		runtimeExecutor: deps.RuntimeExecutor,
		secretLease:    deps.SecretLease,
		permission:     deps.Permission,
		authority:      deps.AuthorityView,
		structureBuilder: deps.StructureBuilder,
		audit:          deps.AuditSink,
		checkpoint:     deps.CheckpointClassifier,
	}
	return c, nil
}

func (c *RecoveryCoordinator) ExecuteRecovery(ctx context.Context, req RecoveryRequest) (*RecoveryResponse, error) {
	operationID := generateRecoveryOperationID(req.RuntimeID)
	op := &RecoveryOperation{
		OperationID:  operationID,
		RuntimeID:    req.RuntimeID,
		FailureClass: req.FailureClass,
		Stage:        RecoveryStageClassifying,
		Attempt:      1,
		MaxAttempts:  req.MaxAttempts,
		StartedAt:    time.Now(),
	}
	if op.MaxAttempts <= 0 {
		op.MaxAttempts = 3
	}

	if err := c.gate.Acquire(req.RuntimeID, operationID); err != nil {
		return &RecoveryResponse{
			OperationID: operationID,
			Success:     false,
			Stage:       RecoveryStageFailed,
			Error:       err,
		}, err
	}
	defer c.gate.Release(req.RuntimeID)

	c.recordAudit(op, "", nil)

	result, err := c.executeRecoveryFlow(ctx, op, req)
	op.CompletedAt = timePtr(time.Now())
	op.Result = *result

	if err != nil {
		op.Error = err.Error()
		op.Stage = RecoveryStageFailed
		c.recordAudit(op, "failed", err)
	} else {
		op.Stage = RecoveryStageCompleted
		c.recordAudit(op, "completed", nil)
	}

	return &RecoveryResponse{
		OperationID: operationID,
		Success:     result.Success,
		Stage:       op.Stage,
		Result:      *result,
		Error:       err,
	}, err
}

func (c *RecoveryCoordinator) executeRecoveryFlow(ctx context.Context, op *RecoveryOperation, req RecoveryRequest) (*RecoveryResult, error) {
	op.Stage = RecoveryStageClassifying

	rtInfo, err := c.runtimeMgr.GetRuntime(req.RuntimeID)
	if err != nil {
		return nil, fmt.Errorf("get runtime: %w", err)
	}
	op.PluginID = rtInfo.PluginID

	desc, err := c.pluginReg.Get(ctx, rtInfo.PluginID)
	if err != nil {
		return nil, fmt.Errorf("get plugin descriptor: %w", err)
	}
	op.ExtensionID = desc.ExtensionID

	level := c.classifier.DetermineLevel(op.FailureClass, 0, 3)
	op.Level = level

	result := &RecoveryResult{}

	switch level {
	case RecoveryLevelQuarantine:
		result.Success = false
		result.Quarantined = true
		return result, NewQuarantinedError(req.RuntimeID, fmt.Sprintf("failure=%s", op.FailureClass))

	case RecoveryLevelPackageRollback:
		if err := c.executeRollbackRecovery(ctx, op, req); err != nil {
			return result, err
		}
		result.RequiresRebuild = true
		result.Success = true

	case RecoveryLevelRuntimeReconstruction:
		if err := c.executeRuntimeReconstruction(ctx, op, req); err != nil {
			return result, err
		}
		result.RequiresRebuild = true

	case RecoveryLevelProcessRestart:
		if c.supervisor != nil && c.supervisor.IsQuarantined(string(req.RuntimeID)) {
			result.Success = false
			result.Quarantined = true
			return result, NewQuarantinedError(req.RuntimeID, "service quarantined")
		}
		if c.runtimeExecutor != nil {
			if err := c.runtimeExecutor.StopRuntime(ctx, req.RuntimeID); err != nil {
				return result, fmt.Errorf("recovery process restart: stop failed: %w", err)
			}
			if err := c.runtimeExecutor.StartRuntime(ctx, req.RuntimeID); err != nil {
				return result, fmt.Errorf("recovery process restart: start failed: %w", err)
			}
		}
		result.Success = true
		result.RequiresRestart = false
	}

	return result, nil
}

func (c *RecoveryCoordinator) executeRollbackRecovery(ctx context.Context, op *RecoveryOperation, req RecoveryRequest) error {
	op.Stage = RecoveryStageQuiescing

	if c.runtimeExecutor != nil {
		_ = c.runtimeExecutor.StopRuntime(ctx, req.RuntimeID)
	}

	if c.secretLease != nil {
		c.secretLease.RevokeByRuntimeInstance(string(req.RuntimeID))
	}

	op.Stage = RecoveryStageRollingBack

	if c.kernel == nil {
		return NewRollbackUnavailableError(op.ExtensionID, fmt.Errorf("kernel rollback not available"))
	}

	rollbackResult, err := c.kernel.RollbackPackage(ctx, op.ExtensionID, op.OperationID)
	if err != nil {
		return NewRollbackFailedError(op.ExtensionID, err)
	}
	if !rollbackResult.Success {
		return NewRollbackFailedError(op.ExtensionID, fmt.Errorf("kernel rollback failed: %s", rollbackResult.Error))
	}

	if rollbackResult.RequiresReconcile {
		op.Stage = RecoveryStageReconciling
		if err := c.reconcileAfterRollback(ctx, op); err != nil {
			return fmt.Errorf("post-rollback reconcile: %w", err)
		}
	}

	if c.structureBuilder != nil {
		op.Stage = RecoveryStageRebuilding
		topo, err := c.structureBuilder.RebuildTopology(ctx, op.PluginID, op.ExtensionID)
		if err != nil || !topo.Valid {
			return fmt.Errorf("rebuild topology: %w", err)
		}
		_, err = c.structureBuilder.RebuildLifecyclePlan(ctx, topo)
		if err != nil {
			return fmt.Errorf("rebuild lifecycle plan: %w", err)
		}
	}

	op.Stage = RecoveryStageRestarting

	return nil
}

func (c *RecoveryCoordinator) executeRuntimeReconstruction(ctx context.Context, op *RecoveryOperation, req RecoveryRequest) error {
	op.Stage = RecoveryStageReconciling

	var currentRevision string
	if desc, err := c.pluginReg.Get(ctx, op.PluginID); err == nil {
		currentRevision = desc.Version
	}

	if c.checkpoint != nil {
		cpInfo, err := c.checkpoint.Classify(ctx, req.RuntimeID, currentRevision)
		if err != nil {
			log.Printf("[recovery-coordinator] checkpoint classification error for %s: %v", req.RuntimeID, err)
		}
		op.Checkpoint = &cpInfo

		switch cpInfo.Class {
		case CheckpointIncompatible:
			return &RecoveryError{
				Code:    ErrCodeCheckPointIncompatible,
				Message: fmt.Sprintf("checkpoint incompatible for runtime %s", req.RuntimeID),
			}
		case CheckpointCorrupt:
			return &RecoveryError{
				Code:    ErrCodeCheckpointCorrupt,
				Message: fmt.Sprintf("checkpoint corrupt for runtime %s", req.RuntimeID),
			}
		}
	}

	if c.structureBuilder != nil {
		op.Stage = RecoveryStageRebuilding
		topo, err := c.structureBuilder.RebuildTopology(ctx, op.PluginID, op.ExtensionID)
		if err != nil || !topo.Valid {
			return &RecoveryError{
				Code:    ErrCodeRuntimeRebuildFailed,
				Message: fmt.Sprintf("failed to rebuild topology for %s", op.PluginID),
				Cause:   err,
			}
		}
		_, err = c.structureBuilder.RebuildLifecyclePlan(ctx, topo)
		if err != nil {
			return &RecoveryError{
				Code:    ErrCodeRuntimeRebuildFailed,
				Message: fmt.Sprintf("failed to rebuild lifecycle plan for %s", op.PluginID),
				Cause:   err,
			}
		}
		op.Result.RequiresRebuild = true
	}

	op.Stage = RecoveryStageRestarting
	if c.runtimeExecutor != nil {
		if err := c.runtimeExecutor.StopRuntime(ctx, req.RuntimeID); err != nil {
			return fmt.Errorf("recovery runtime reconstruction: stop failed: %w", err)
		}
		if err := c.runtimeExecutor.StartRuntime(ctx, req.RuntimeID); err != nil {
			return fmt.Errorf("recovery runtime reconstruction: start failed: %w", err)
		}
	}
	op.Result.RequiresRestart = false

	return nil
}

func (c *RecoveryCoordinator) reconcileAfterRollback(ctx context.Context, op *RecoveryOperation) error {
	return nil
}

func (c *RecoveryCoordinator) IsRecovering(runtimeID domain.RuntimeInstanceID) bool {
	return c.gate.IsRecovering(runtimeID)
}

func (c *RecoveryCoordinator) recordAudit(op *RecoveryOperation, result string, err error) {
	if c.audit == nil {
		return
	}
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	c.audit.RecordRecovery(RecoveryAuditEvent{
		OperationID:  op.OperationID,
		RuntimeID:    op.RuntimeID,
		ExtensionID:  op.ExtensionID,
		PluginID:     op.PluginID,
		FailureClass: op.FailureClass,
		Stage:        op.Stage,
		Attempt:      op.Attempt,
		Result:       result,
		Error:        errMsg,
		Timestamp:    time.Now(),
	})
}

func generateRecoveryOperationID(runtimeID domain.RuntimeInstanceID) RecoveryOperationID {
	input := fmt.Sprintf("%s-%d", runtimeID, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(input))
	return RecoveryOperationID("recovery-" + hex.EncodeToString(hash[:8]))
}

func timePtr(t time.Time) *time.Time {
	return &t
}
