package gamehost

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/registry"
	"github.com/u-ai/backend/internal/gamehost/resource"
	ghruntime "github.com/u-ai/backend/internal/gamehost/runtime"
	"github.com/u-ai/backend/internal/gamehost/storage"
	platformprocess "github.com/u-ai/backend/internal/platform/process"
)

// pluginIdentityReader binds resource subjects to the authoritative RuntimeManager
// and TopologyStore rather than guessing from registry iteration order.
type pluginIdentityReader struct {
	pluginReg      *registry.Registry
	runtimeManager *ghruntime.Manager
	topologyStore  *ghruntime.TopologyStore
}

func newPluginIdentityReader(pluginReg *registry.Registry, runtimeManager *ghruntime.Manager, topologyStore *ghruntime.TopologyStore) *pluginIdentityReader {
	return &pluginIdentityReader{pluginReg: pluginReg, runtimeManager: runtimeManager, topologyStore: topologyStore}
}

func (r *pluginIdentityReader) ResolveRuntime(runtimeID string) (string, string, string, error) {
	if r == nil || r.pluginReg == nil || r.runtimeManager == nil {
		return "", "", "", fmt.Errorf("resource identity infrastructure unavailable")
	}
	rt, err := r.runtimeManager.GetRuntime(domain.RuntimeInstanceID(runtimeID))
	if err != nil {
		return "", "", "", err
	}
	desc, err := r.pluginReg.Get(context.Background(), rt.PluginID)
	if err != nil {
		return "", "", "", err
	}
	return string(rt.PluginID), desc.ExtensionID, string(rt.State), nil
}

func (r *pluginIdentityReader) ResolveService(runtimeID, serviceID string) (string, string, string, error) {
	if r == nil || r.pluginReg == nil || r.topologyStore == nil {
		return "", "", "", fmt.Errorf("resource topology infrastructure unavailable")
	}
	snapshot, err := r.topologyStore.GetTopologySnapshot(domain.RuntimeInstanceID(runtimeID))
	if err != nil {
		return "", "", "", err
	}
	for _, svc := range snapshot.Services {
		if svc.ServiceID != domain.ServiceID(serviceID) {
			continue
		}
		desc, err := r.pluginReg.Get(context.Background(), svc.PluginID)
		if err != nil {
			return "", "", "", err
		}
		return string(svc.PluginID), desc.ExtensionID, string(svc.State), nil
	}
	return "", "", "", fmt.Errorf("service %s not found in runtime %s", serviceID, runtimeID)
}

func (r *pluginIdentityReader) CurrentGeneration(runtimeID string) (int64, error) {
	if r == nil || r.runtimeManager == nil {
		return 0, fmt.Errorf("runtime manager unavailable")
	}
	return r.runtimeManager.GetCurrentGeneration(domain.RuntimeInstanceID(runtimeID))
}

func (r *pluginIdentityReader) ExtensionEnabled(extensionID string) bool {
	if r == nil || r.pluginReg == nil || extensionID == "" {
		return false
	}
	plugins, err := r.pluginReg.ListByExtension(context.Background(), extensionID)
	return err == nil && len(plugins) > 0
}

func (r *pluginIdentityReader) RuntimeIDsByExtension(extensionID string) []string {
	if r == nil || r.runtimeManager == nil || r.topologyStore == nil || strings.TrimSpace(extensionID) == "" {
		return nil
	}
	ids := make([]string, 0)
	for _, rt := range r.runtimeManager.ListRuntimes() {
		if rt == nil {
			continue
		}
		snapshot, err := r.topologyStore.GetTopologySnapshot(rt.ID)
		if err != nil {
			continue
		}
		if snapshot.ExtensionID == extensionID {
			ids = append(ids, string(rt.ID))
		}
	}
	return ids
}

func newResourceSubjectMapper(pluginReg *registry.Registry, runtimeManager *ghruntime.Manager, topologyStore *ghruntime.TopologyStore) *resource.SubjectMapper {
	return resource.NewSubjectMapper(newPluginIdentityReader(pluginReg, runtimeManager, topologyStore))
}

type resourceRequestAdmission struct {
	inner *resource.ResourceAdmissionAdapter
}

func (a resourceRequestAdmission) AdmitRequest(ctx context.Context, peer ipc.Peer) error {
	if a.inner == nil {
		return fmt.Errorf("resource admission unavailable")
	}
	decision, _ := a.inner.AcquireRPCPending(ctx, resource.RuntimeIdentitySubject{
		PluginID: string(peer.PluginID), RuntimeID: string(peer.RuntimeID),
		ServiceID: string(peer.ServiceID), Generation: peer.Generation,
	})
	if !decision.Allowed {
		return fmt.Errorf("resource admission denied: %s", decision.Reason)
	}
	return nil
}

// runtimeLimitGovernor records declared Runtime resource limits for admission
// and observability. OS-level CPU/memory/filesystem enforcement is reported
// separately by the platform sandbox; declarations are never presented as
// enforced limits when no backend exists.
type runtimeResourceKey struct {
	runtimeID string
	serviceID string
}

type runtimeLimitGovernor struct {
	mu     sync.Mutex
	limits map[runtimeResourceKey]resource.ServiceResourceLimitsSet
}

func newRuntimeLimitGovernor() *runtimeLimitGovernor {
	return &runtimeLimitGovernor{limits: make(map[runtimeResourceKey]resource.ServiceResourceLimitsSet)}
}

func (g *runtimeLimitGovernor) ConfigureResourceLimits(runtimeID, serviceID string, limits resource.ServiceResourceLimitsSet) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.limits[runtimeResourceKey{runtimeID: runtimeID, serviceID: serviceID}] = limits
	return nil
}

func (g *runtimeLimitGovernor) LimitsFor(runtimeID, serviceID string) (resource.ServiceResourceLimitsSet, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	l, ok := g.limits[runtimeResourceKey{runtimeID: runtimeID, serviceID: serviceID}]
	return l, ok
}

func (g *runtimeLimitGovernor) ClearServiceResourceLimits(runtimeID, serviceID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.limits, runtimeResourceKey{runtimeID: runtimeID, serviceID: serviceID})
}

func (g *runtimeLimitGovernor) ClearRuntimeResourceLimits(runtimeID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for key := range g.limits {
		if key.runtimeID == runtimeID {
			delete(g.limits, key)
		}
	}
}

func (g *runtimeLimitGovernor) ClearAllResourceLimits() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.limits = make(map[runtimeResourceKey]resource.ServiceResourceLimitsSet)
}

// containerViewResolver reports measured usage and declared limits separately.
// A metric that cannot be observed is marked unavailable rather than represented
// by a fabricated zero or an invented limit.
type containerViewResolver struct {
	binaryReg  binaryRegistryView
	pending    resource.PendingRegistry
	topology   *ghruntime.TopologyStore
	supervisor *trusted_service.ProcessSupervisor
	dirs       storage.DirectoryManager
	governor   *runtimeLimitGovernor
	admission  *resource.ResourceAdmissionAdapter
}

type binaryRegistryView interface {
	CountActive() int
	LimitActive() int
	ActiveBytes() int64
	LimitActiveBytes() int64
	CountByRuntime(runtimeID domain.RuntimeInstanceID) int
	ActiveBytesByRuntime(runtimeID domain.RuntimeInstanceID) int64
}

func newContainerViewResolver(binaryReg binaryRegistryView, pending resource.PendingRegistry, topology *ghruntime.TopologyStore, supervisor *trusted_service.ProcessSupervisor, dirs storage.DirectoryManager, governor *runtimeLimitGovernor, admission *resource.ResourceAdmissionAdapter) *containerViewResolver {
	return &containerViewResolver{binaryReg: binaryReg, pending: pending, topology: topology, supervisor: supervisor, dirs: dirs, governor: governor, admission: admission}
}

func (r *containerViewResolver) ResolveUsage(runtimeID, serviceID string) map[resource.UsageDimension]resource.UsageSample {
	out := make(map[resource.UsageDimension]resource.UsageSample)
	support := platformprocess.ResourceLimitsSupported()
	limits := resource.ServiceResourceLimitsSet{}
	if r.governor != nil {
		if configured, ok := r.governor.LimitsFor(runtimeID, serviceID); ok {
			limits = configured
		}
	}
	definitionID := ""
	if r.topology != nil && serviceID != "" {
		definitionID, _ = r.topology.ResolveDefinitionID(domain.RuntimeInstanceID(runtimeID), domain.ServiceID(serviceID))
	}
	if r.supervisor != nil && definitionID != "" {
		if def, err := r.supervisor.GetDefinition(definitionID); err == nil && def != nil {
			limits = def.Limits
		}
		// Supervisor instances are registered under the runtime-scoped process
		// instance key, not under the reusable definition ID. Looking them up by
		// definition ID silently made live process usage unavailable for every
		// GameHost service.
		supervisorKey := ghruntime.BuildProcessInstanceID(domain.RuntimeInstanceID(runtimeID), domain.ServiceID(serviceID))
		if inst, err := r.supervisor.Get(supervisorKey); err == nil && inst != nil && inst.PID > 0 {
			measured := measureLiveProcess(inst.PID)
			out[resource.UsageMemoryBytes] = resource.UsageSample{Used: measured.memoryBytes, Limit: mibToBytes(limits.MaxMemoryMB), Available: measured.memoryAvailable, Enforced: support.Memory && limits.MaxMemoryMB > 0}
			out[resource.UsageOpenFiles] = resource.UsageSample{Used: int64(measured.openFiles), Limit: int64(limits.MaxFileDescriptors), Available: measured.openFilesAvailable, Enforced: false}
			out[resource.UsageSubprocesses] = resource.UsageSample{Used: int64(measured.subprocesses), Limit: int64(limits.MaxSubprocesses), Available: measured.subprocessesAvailable, Enforced: support.Processes && limits.MaxSubprocesses > 0}
		}
	}
	if _, ok := out[resource.UsageMemoryBytes]; !ok {
		out[resource.UsageMemoryBytes] = resource.UsageSample{Limit: mibToBytes(limits.MaxMemoryMB), Available: false, Enforced: support.Memory && limits.MaxMemoryMB > 0}
	}
	if _, ok := out[resource.UsageOpenFiles]; !ok {
		out[resource.UsageOpenFiles] = resource.UsageSample{Limit: int64(limits.MaxFileDescriptors), Available: false, Enforced: false}
	}
	if _, ok := out[resource.UsageSubprocesses]; !ok {
		out[resource.UsageSubprocesses] = resource.UsageSample{Limit: int64(limits.MaxSubprocesses), Available: false, Enforced: support.Processes && limits.MaxSubprocesses > 0}
	}
	// Live CPU accounting is deliberately unavailable until the platform process
	// layer exposes a trustworthy counter. Do not manufacture a 0% reading.
	out[resource.UsageCPUPercent] = resource.UsageSample{Limit: int64(limits.MaxCPUPercent), Available: false, Enforced: support.CPU && limits.MaxCPUPercent > 0}

	if r.dirs != nil && serviceID != "" {
		if paths, err := r.dirs.ResolveServicePaths(domain.RuntimeInstanceID(runtimeID), domain.ServiceID(serviceID)); err == nil {
			if size, err := directorySize(paths.Root); err == nil {
				out[resource.UsageDiskBytes] = resource.UsageSample{Used: size, Limit: mibToBytes(limits.MaxDiskMB), Available: true, Enforced: false}
			}
		}
	}
	if _, ok := out[resource.UsageDiskBytes]; !ok {
		out[resource.UsageDiskBytes] = resource.UsageSample{Limit: mibToBytes(limits.MaxDiskMB), Available: false, Enforced: false}
	}

	if r.pending != nil {
		out[resource.UsagePendingRPC] = resource.UsageSample{Used: int64(r.pending.CountByPeer(runtimeID, serviceID)), Limit: int64(r.pending.LimitPerPeer()), Available: true, Enforced: true}
	}
	if r.admission != nil {
		current, limit := r.admission.QueueUsage(runtimeID, serviceID)
		out[resource.UsageQueue] = resource.UsageSample{Used: int64(current), Limit: int64(limit), Available: true, Enforced: true}
	}
	if r.binaryReg != nil {
		rid := domain.RuntimeInstanceID(runtimeID)
		out[resource.UsageBinaryCount] = resource.UsageSample{Used: int64(r.binaryReg.CountByRuntime(rid)), Limit: int64(r.binaryReg.LimitActive()), Available: true, Enforced: true}
		out[resource.UsageBinaryBytes] = resource.UsageSample{Used: r.binaryReg.ActiveBytesByRuntime(rid), Limit: r.binaryReg.LimitActiveBytes(), Available: true, Enforced: true}
	}
	return out
}

type liveProcessUsage struct {
	memoryBytes           int64
	memoryAvailable       bool
	openFiles             int
	openFilesAvailable    bool
	subprocesses          int
	subprocessesAvailable bool
}

func measureLiveProcess(pid int) liveProcessUsage {
	if pid <= 0 || stdruntime.GOOS != "linux" {
		return liveProcessUsage{}
	}
	var out liveProcessUsage
	if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/statm", pid)); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 2 {
			if pages, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
				out.memoryBytes = pages * int64(os.Getpagesize())
				out.memoryAvailable = true
			}
		}
	}
	if entries, err := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid)); err == nil {
		out.openFiles = len(entries)
		out.openFilesAvailable = true
	}
	if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/task/%d/children", pid, pid)); err == nil {
		out.subprocesses = len(strings.Fields(string(data)))
		out.subprocessesAvailable = true
	}
	return out
}

func directorySize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	if os.IsNotExist(err) {
		return 0, nil
	}
	return total, err
}

func mibToBytes(value int64) int64 {
	if value <= 0 {
		return 0
	}
	if value > (1<<63-1)/(1<<20) {
		return 1<<63 - 1
	}
	return value << 20
}
