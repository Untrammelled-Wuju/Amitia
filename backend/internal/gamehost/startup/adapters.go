package startup

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/integration"
	"github.com/u-ai/backend/internal/gamehost/runtime"
	"github.com/u-ai/backend/internal/gamehost/stream/binary"
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
	mu         sync.Mutex
	cleaned    map[string]bool
	supervisor *trusted_service.ProcessSupervisor
}

func NewProcessCleanupAdapter(supervisor *trusted_service.ProcessSupervisor) *ProcessCleanupProcessAdapter {
	return &ProcessCleanupProcessAdapter{
		cleaned:    make(map[string]bool),
		supervisor: supervisor,
	}
}

func (a *ProcessCleanupProcessAdapter) CleanupOwnedProcess(ctx context.Context, runtimeID domain.RuntimeInstanceID, pid int) error {
	a.mu.Lock()
	a.cleaned[string(runtimeID)] = true
	a.mu.Unlock()
	if a.supervisor != nil {
		_, err := a.supervisor.Stop(ctx, trusted_service.StopRequest{
			ServiceID: string(runtimeID),
			Reason:    "startup_recovery_cleanup",
			Force:     pid > 0,
		})
		if err != nil {
			log.Printf("[startup-recovery] process supervisor stop: runtime=%s err=%v", runtimeID, err)
		}
	}
	log.Printf("[startup-recovery] process cleanup: runtime=%s pid=%d", runtimeID, pid)
	return nil
}

func (a *ProcessCleanupProcessAdapter) ListOrphanCandidates(ctx context.Context) ([]ProcessCandidate, error) {
	if a.supervisor == nil {
		return nil, nil
	}
	instances := a.supervisor.List()
	candidates := make([]ProcessCandidate, 0, len(instances))
	for _, inst := range instances {
		if inst == nil || inst.Definition == nil {
			continue
		}
		if inst.PID <= 0 {
			continue
		}
		candidates = append(candidates, ProcessCandidate{
			PID:         inst.PID,
			RuntimeID:   domain.RuntimeInstanceID(inst.ServiceID),
			PluginID:    domain.PluginID(inst.Definition.ModuleID),
			ExtensionID: inst.Definition.ExtensionID,
			Generation:  uint64(inst.Generation),
		})
	}
	return candidates, nil
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
	if a.dirMgr == nil {
		return nil, nil
	}
	ids, err := a.dirMgr.ListStaleTempRuntimeIDs(ctx)
	if err != nil {
		return nil, err
	}
	candidates := make([]TempCandidate, 0, len(ids))
	for _, rid := range ids {
		candidates = append(candidates, TempCandidate{RuntimeID: rid})
	}
	return candidates, nil
}

func (a *TempCleanupDirAdapter) RemoveStaleTemp(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	log.Printf("[startup-recovery] temp cleanup: runtime=%s", runtimeID)
	return nil
}

type BinaryCleanupAdapter struct {
	binaryReg binary.ObjectRegistry
	runtimeMgr interface {
		ListRuntimes() []*runtime.RuntimeInstanceRef
	}
}

func NewBinaryCleanupAdapter(binaryReg binary.ObjectRegistry, runtimeMgr interface {
	ListRuntimes() []*runtime.RuntimeInstanceRef
}) *BinaryCleanupAdapter {
	return &BinaryCleanupAdapter{binaryReg: binaryReg, runtimeMgr: runtimeMgr}
}

func (a *BinaryCleanupAdapter) ListOrphanBinaries(ctx context.Context) ([]BinaryCandidate, error) {
	if a.binaryReg == nil || a.runtimeMgr == nil {
		return nil, nil
	}
	activeObjects := a.binaryReg.GetActiveObjects()
	if len(activeObjects) == 0 {
		return nil, nil
	}
	refs := a.runtimeMgr.ListRuntimes()
	activeRuntimes := make(map[domain.RuntimeInstanceID]bool)
	for _, ref := range refs {
		activeRuntimes[ref.ID] = true
	}
	candidates := make([]BinaryCandidate, 0, len(activeObjects))
	for _, obj := range activeObjects {
		if !activeRuntimes[obj.Owner.RuntimeID] {
			candidates = append(candidates, BinaryCandidate{
				BinaryID:  string(obj.ID),
				RuntimeID: obj.Owner.RuntimeID,
			})
		}
	}
	return candidates, nil
}

func (a *BinaryCleanupAdapter) RemoveOrphanBinary(ctx context.Context, binaryID string) error {
	if a.binaryReg == nil {
		return nil
	}
	return a.binaryReg.Release(ctx, binary.BinaryObjectID(binaryID))
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
		List(ctx context.Context) ([]domain.PluginDescriptor, error)
		Get(ctx context.Context, pluginID domain.PluginID) (domain.PluginDescriptor, error)
	}
	runtimeMgr interface {
		ListRuntimes() []*runtime.RuntimeInstanceRef
	}
	extensionLookup interface {
		ListEnabledGamePlugins(ctx context.Context) ([]KernelGamePlugin, error)
	}
}

type KernelGamePlugin struct {
	ExtensionID string
	Enabled     bool
}

type KernelGamePluginAdapter struct {
	Source integration.KernelContributionSource
}

func (a *KernelGamePluginAdapter) ListEnabledGamePlugins(ctx context.Context) ([]KernelGamePlugin, error) {
	if a.Source == nil {
		return nil, nil
	}
	plugins, err := a.Source.ListEnabledGamePlugins(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]KernelGamePlugin, 0, len(plugins))
	for _, p := range plugins {
		result = append(result, KernelGamePlugin{
			ExtensionID: string(p.Extension.ID),
			Enabled:     true,
		})
	}
	return result, nil
}

func NewKernelReconciliationAdapter(
	pluginReg interface {
		List(ctx context.Context) ([]domain.PluginDescriptor, error)
		Get(ctx context.Context, pluginID domain.PluginID) (domain.PluginDescriptor, error)
	},
	runtimeMgr interface {
		ListRuntimes() []*runtime.RuntimeInstanceRef
	},
	extensionLookup interface {
		ListEnabledGamePlugins(ctx context.Context) ([]KernelGamePlugin, error)
	},
) *KernelReconciliationRegistryAdapter {
	return &KernelReconciliationRegistryAdapter{
		pluginReg:      pluginReg,
		runtimeMgr:     runtimeMgr,
		extensionLookup: extensionLookup,
	}
}

func (a *KernelReconciliationRegistryAdapter) CurrentRuntimeIDs(ctx context.Context) ([]domain.RuntimeInstanceID, error) {
	if a.runtimeMgr == nil {
		return nil, nil
	}
	refs := a.runtimeMgr.ListRuntimes()
	ids := make([]domain.RuntimeInstanceID, 0, len(refs))
	for _, ref := range refs {
		ids = append(ids, ref.ID)
	}
	return ids, nil
}

func (a *KernelReconciliationRegistryAdapter) IsValidExtension(ctx context.Context, extensionID string) (bool, error) {
	if a.extensionLookup == nil {
		return extensionID != "", nil
	}
	plugins, err := a.extensionLookup.ListEnabledGamePlugins(ctx)
	if err != nil {
		return false, err
	}
	for _, p := range plugins {
		if p.ExtensionID == extensionID {
			return true, nil
		}
	}
	return false, nil
}

func (a *KernelReconciliationRegistryAdapter) IsExtensionEnabled(ctx context.Context, extensionID string) (bool, error) {
	if a.extensionLookup == nil {
		return extensionID != "", nil
	}
	plugins, err := a.extensionLookup.ListEnabledGamePlugins(ctx)
	if err != nil {
		return false, err
	}
	for _, p := range plugins {
		if p.ExtensionID == extensionID {
			return p.Enabled, nil
		}
	}
	return false, nil
}

func (a *KernelReconciliationRegistryAdapter) IsValidPlugin(ctx context.Context, pluginID domain.PluginID) (bool, error) {
	if a.pluginReg == nil {
		return pluginID != "", nil
	}
	_, err := a.pluginReg.Get(ctx, pluginID)
	if err != nil {
		return false, nil
	}
	return true, nil
}

type AuditSinkLoggerAdapter struct{}

func NewAuditSinkLoggerAdapter() *AuditSinkLoggerAdapter {
	return &AuditSinkLoggerAdapter{}
}

func (a *AuditSinkLoggerAdapter) RecordStartupRecovery(event StartupRecoveryAuditEvent) {
	log.Printf("[startup-recovery-audit] op=%s stage=%s resource=%s/%s err=%s",
		event.OperationID, event.Stage, event.ResourceType, event.ResourceID, event.Error)
}

type noopKernelReconciliationProvider struct{}

func NewNoopKernelReconciliationProvider() KernelReconciliationProvider {
	return &noopKernelReconciliationProvider{}
}

func (n *noopKernelReconciliationProvider) CurrentRuntimeIDs(ctx context.Context) ([]domain.RuntimeInstanceID, error) {
	return nil, nil
}

func (n *noopKernelReconciliationProvider) IsValidExtension(ctx context.Context, extensionID string) (bool, error) {
	return extensionID != "", nil
}

func (n *noopKernelReconciliationProvider) IsExtensionEnabled(ctx context.Context, extensionID string) (bool, error) {
	return extensionID != "", nil
}

func (n *noopKernelReconciliationProvider) IsValidPlugin(ctx context.Context, pluginID domain.PluginID) (bool, error) {
	return pluginID != "", nil
}

type noopBinaryCleanupProvider struct{}

func NewNoopBinaryCleanupProvider() BinaryCleanupProvider {
	return &noopBinaryCleanupProvider{}
}

func (n *noopBinaryCleanupProvider) ListOrphanBinaries(ctx context.Context) ([]BinaryCandidate, error) {
	return nil, nil
}

func (n *noopBinaryCleanupProvider) RemoveOrphanBinary(ctx context.Context, binaryID string) error {
	return nil
}
