package startup

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type HostIdentity struct {
	instanceID string
	sessionID  string
}

func NewHostIdentity(instanceID, sessionID string) *HostIdentity {
	if instanceID == "" {
		instanceID, _ = os.Hostname()
	}
	return &HostIdentity{instanceID: instanceID, sessionID: sessionID}
}

func (h *HostIdentity) GetHostInstanceID() string {
	return h.instanceID
}

func (h *HostIdentity) GetHostSessionID() string {
	return h.sessionID
}

type ProcessCleanupProcessAdapter struct {
	cleaned map[string]bool
	mu      sync.Mutex
}

func NewProcessCleanupAdapter() *ProcessCleanupProcessAdapter {
	return &ProcessCleanupProcessAdapter{cleaned: make(map[string]bool)}
}

func (a *ProcessCleanupProcessAdapter) CleanupOwnedProcess(ctx context.Context, runtimeID domain.RuntimeInstanceID, pid int) error {
	a.mu.Lock()
	a.cleaned[string(runtimeID)] = true
	a.mu.Unlock()
	log.Printf("[startup-recovery] process cleanup: runtime=%s pid=%d", runtimeID, pid)
	return nil
}

func (a *ProcessCleanupProcessAdapter) ListOrphanCandidates(ctx context.Context) ([]ProcessCandidate, error) {
	return nil, nil
}

type TempCleanupDirAdapter struct {
	dirMgr interface {
		ListStaleTempRuntimeIDs(ctx context.Context) ([]domain.RuntimeInstanceID, error)
	}
}

func NewTempCleanupAdapter() *TempCleanupDirAdapter {
	return &TempCleanupDirAdapter{}
}

func (a *TempCleanupDirAdapter) ListStaleTempCandidates(ctx context.Context) ([]TempCandidate, error) {
	return nil, nil
}

func (a *TempCleanupDirAdapter) RemoveStaleTemp(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	return nil
}

type BinaryCleanupAdapter struct{}

func NewBinaryCleanupAdapter() *BinaryCleanupAdapter {
	return &BinaryCleanupAdapter{}
}

func (a *BinaryCleanupAdapter) ListOrphanBinaries(ctx context.Context) ([]BinaryCandidate, error) {
	return nil, nil
}

func (a *BinaryCleanupAdapter) RemoveOrphanBinary(ctx context.Context, binaryID string) error {
	return nil
}

type EndpointCleanupAdapter struct{}

func NewEndpointCleanupAdapter() *EndpointCleanupAdapter {
	return &EndpointCleanupAdapter{}
}

func (a *EndpointCleanupAdapter) ListStaleEndpoints(ctx context.Context) ([]EndpointCandidate, error) {
	return nil, nil
}

func (a *EndpointCleanupAdapter) RemoveStaleEndpoint(ctx context.Context, endpointID string) error {
	return nil
}

type SharedMemoryCleanupAdapter struct{}

func NewSharedMemoryCleanupAdapter() *SharedMemoryCleanupAdapter {
	return &SharedMemoryCleanupAdapter{}
}

func (a *SharedMemoryCleanupAdapter) ListStaleSharedMemory(ctx context.Context) ([]SharedMemoryCandidate, error) {
	return nil, nil
}

func (a *SharedMemoryCleanupAdapter) ReleaseSharedMemory(ctx context.Context, shmID string) error {
	return nil
}

type KernelReconciliationRegistryAdapter struct {
	pluginReg interface {
		ListAll(ctx context.Context) ([]domain.PluginDescriptor, error)
	}
	runtimeMgr interface {
		ListRuntimeIDs(ctx context.Context) ([]domain.RuntimeInstanceID, error)
	}
}

func NewKernelReconciliationAdapter() *KernelReconciliationRegistryAdapter {
	return &KernelReconciliationRegistryAdapter{}
}

func (a *KernelReconciliationRegistryAdapter) CurrentRuntimeIDs(ctx context.Context) ([]domain.RuntimeInstanceID, error) {
	if a.runtimeMgr != nil {
		return a.runtimeMgr.ListRuntimeIDs(ctx)
	}
	return nil, nil
}

func (a *KernelReconciliationRegistryAdapter) IsValidExtension(ctx context.Context, extensionID string) (bool, error) {
	return extensionID != "", nil
}

func (a *KernelReconciliationRegistryAdapter) IsExtensionEnabled(ctx context.Context, extensionID string) (bool, error) {
	return true, nil
}

func (a *KernelReconciliationRegistryAdapter) IsValidPlugin(ctx context.Context, pluginID domain.PluginID) (bool, error) {
	return pluginID != "", nil
}

type AuditSinkLoggerAdapter struct{}

func NewAuditSinkLoggerAdapter() *AuditSinkLoggerAdapter {
	return &AuditSinkLoggerAdapter{}
}

func (a *AuditSinkLoggerAdapter) RecordStartupRecovery(event StartupRecoveryAuditEvent) {
	log.Printf("[startup-recovery-audit] op=%s stage=%s resource=%s/%s err=%s",
		event.OperationID, event.Stage, event.ResourceType, event.ResourceID, event.Error)
}
