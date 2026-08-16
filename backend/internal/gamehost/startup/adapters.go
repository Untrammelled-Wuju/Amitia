package startup

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/integration"
	"github.com/u-ai/backend/internal/gamehost/runtime"
	"github.com/u-ai/backend/internal/gamehost/storage"
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
	mu              sync.Mutex
	cleaned         map[string]bool
	supervisor      *trusted_service.ProcessSupervisor
	hostIdentity    *HostIdentity
	runtimeResolver interface {
		GetRuntime(runtimeID domain.RuntimeInstanceID) (*runtime.RuntimeInstanceRef, error)
	}
}

func NewProcessCleanupAdapter(supervisor *trusted_service.ProcessSupervisor) *ProcessCleanupProcessAdapter {
	return &ProcessCleanupProcessAdapter{
		cleaned:    make(map[string]bool),
		supervisor: supervisor,
	}
}

func NewProcessCleanupAdapterWithIdentity(supervisor *trusted_service.ProcessSupervisor, hostIdentity *HostIdentity, runtimeResolver interface {
	GetRuntime(runtimeID domain.RuntimeInstanceID) (*runtime.RuntimeInstanceRef, error)
}) *ProcessCleanupProcessAdapter {
	return &ProcessCleanupProcessAdapter{
		cleaned:         make(map[string]bool),
		supervisor:      supervisor,
		hostIdentity:    hostIdentity,
		runtimeResolver: runtimeResolver,
	}
}

func (a *ProcessCleanupProcessAdapter) CleanupOwnedProcess(ctx context.Context, instanceID string, pid int) error {
	a.mu.Lock()
	a.cleaned[instanceID] = true
	a.mu.Unlock()
	if a.supervisor != nil {
		_, err := a.supervisor.Stop(ctx, trusted_service.StopRequest{
			ServiceID: instanceID,
			Reason:    "startup_recovery_cleanup",
			Force:     pid > 0,
		})
		if err != nil {
			return fmt.Errorf("process supervisor stop for instance %s: %w", instanceID, err)
		}
	}
	log.Printf("[startup-recovery] process cleanup: instance=%s pid=%d", instanceID, pid)
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
		runtimeID := inst.RuntimeID
		if runtimeID == "" {
			log.Printf("[startup-recovery] skip process candidate pid=%d: no owner metadata (RuntimeID empty)", inst.PID)
			continue
		}
		pluginID := resolvePluginID(a.runtimeResolver, domain.RuntimeInstanceID(runtimeID))
		hostInstanceID, hostSessionID := "", ""
		if a.hostIdentity != nil {
			hostInstanceID = a.hostIdentity.GetHostInstanceID()
			hostSessionID = a.hostIdentity.GetHostSessionID()
		}
		candidates = append(candidates, ProcessCandidate{
			PID:              inst.PID,
			ProcessInstanceID: inst.InstanceID,
			RuntimeID:        domain.RuntimeInstanceID(runtimeID),
			PluginID:         pluginID,
			ExtensionID:      inst.Definition.ExtensionID,
			ServiceID:        inst.ServiceID,
			ModuleID:         inst.Definition.ModuleID,
			Generation:       uint64(inst.Generation),
			HostInstanceID:   hostInstanceID,
			HostSessionID:    hostSessionID,
		})
	}
	return candidates, nil
}

func resolvePluginID(resolver interface {
	GetRuntime(runtimeID domain.RuntimeInstanceID) (*runtime.RuntimeInstanceRef, error)
}, runtimeID domain.RuntimeInstanceID) domain.PluginID {
	if resolver == nil {
		return ""
	}
	ref, err := resolver.GetRuntime(runtimeID)
	if err != nil {
		return ""
	}
	return ref.PluginID
}

type TempCleanupDirAdapter struct {
	dirMgr storage.DirectoryManager
}

func NewTempCleanupAdapter(dirMgr storage.DirectoryManager) *TempCleanupDirAdapter {
	return &TempCleanupDirAdapter{dirMgr: dirMgr}
}

func (a *TempCleanupDirAdapter) ListStaleTempCandidates(ctx context.Context) ([]TempCandidate, error) {
	if a.dirMgr == nil {
		return nil, nil
	}
	runtimeIDs, err := a.listStaleTempRuntimeIDs(ctx)
	if err != nil {
		return nil, err
	}
	candidates := make([]TempCandidate, 0, len(runtimeIDs))
	for _, rid := range runtimeIDs {
		candidates = append(candidates, TempCandidate{RuntimeID: rid})
	}
	return candidates, nil
}

func (a *TempCleanupDirAdapter) listStaleTempRuntimeIDs(ctx context.Context) ([]domain.RuntimeInstanceID, error) {
	dataRoot := a.dirMgr.Root()
	if dataRoot == "" {
		return nil, fmt.Errorf("directory manager has no root")
	}
	runtimeDir := filepath.Join(dataRoot, "gamehost", "runtimes")
	entries, err := os.ReadDir(runtimeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list runtime dirs: %w", err)
	}
	ids := make([]domain.RuntimeInstanceID, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "run-") {
			continue
		}
		runtimeID := domain.RuntimeInstanceID(strings.TrimPrefix(entry.Name(), "run-"))
		ids = append(ids, runtimeID)
	}
	return ids, nil
}

func (a *TempCleanupDirAdapter) RemoveStaleTemp(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	if a.dirMgr == nil {
		return fmt.Errorf("directory manager not available for temp cleanup")
	}
	return a.dirMgr.RemoveRuntimeTemp(ctx, runtimeID)
}

type BinaryCleanupAdapter struct {
	binaryReg  binary.ObjectRegistry
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

type EndpointCleanupAdapter struct {
	registry interface {
		ListStaleEndpoints(ctx context.Context) ([]EndpointCandidate, error)
		RemoveStaleEndpoint(ctx context.Context, endpointID string) error
	}
}

func NewEndpointCleanupAdapter() *EndpointCleanupAdapter {
	return &EndpointCleanupAdapter{}
}

func NewEndpointCleanupAdapterWithRegistry(registry interface {
	ListStaleEndpoints(ctx context.Context) ([]EndpointCandidate, error)
	RemoveStaleEndpoint(ctx context.Context, endpointID string) error
}) *EndpointCleanupAdapter {
	return &EndpointCleanupAdapter{registry: registry}
}

func (a *EndpointCleanupAdapter) ListStaleEndpoints(ctx context.Context) ([]EndpointCandidate, error) {
	if a.registry == nil {
		return nil, fmt.Errorf("NOT_APPLICABLE: no endpoint registry configured")
	}
	return a.registry.ListStaleEndpoints(ctx)
}

func (a *EndpointCleanupAdapter) RemoveStaleEndpoint(ctx context.Context, endpointID string) error {
	if a.registry == nil {
		return fmt.Errorf("NOT_APPLICABLE: no endpoint registry configured")
	}
	return a.registry.RemoveStaleEndpoint(ctx, endpointID)
}

type SharedMemoryCleanupAdapter struct {
	registry interface {
		ListStaleSharedMemory(ctx context.Context) ([]SharedMemoryCandidate, error)
		ReleaseSharedMemory(ctx context.Context, shmID string) error
	}
}

func NewSharedMemoryCleanupAdapter() *SharedMemoryCleanupAdapter {
	return &SharedMemoryCleanupAdapter{}
}

func NewSharedMemoryCleanupAdapterWithRegistry(registry interface {
	ListStaleSharedMemory(ctx context.Context) ([]SharedMemoryCandidate, error)
	ReleaseSharedMemory(ctx context.Context, shmID string) error
}) *SharedMemoryCleanupAdapter {
	return &SharedMemoryCleanupAdapter{registry: registry}
}

func (a *SharedMemoryCleanupAdapter) ListStaleSharedMemory(ctx context.Context) ([]SharedMemoryCandidate, error) {
	if a.registry == nil {
		return nil, fmt.Errorf("NOT_APPLICABLE: no shared memory registry configured")
	}
	return a.registry.ListStaleSharedMemory(ctx)
}

func (a *SharedMemoryCleanupAdapter) ReleaseSharedMemory(ctx context.Context, shmID string) error {
	if a.registry == nil {
		return fmt.Errorf("NOT_APPLICABLE: no shared memory registry configured")
	}
	return a.registry.ReleaseSharedMemory(ctx, shmID)
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
		pluginReg:       pluginReg,
		runtimeMgr:      runtimeMgr,
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
