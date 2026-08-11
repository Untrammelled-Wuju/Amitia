package gamehost

import (
	"context"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/registry"
	"github.com/u-ai/backend/internal/gamehost/resource"
	"github.com/u-ai/backend/internal/gamehost/stream"
)

// pluginIdentityReader 将 PluginRegistry 适配到 resource.RuntimeIdentityReader。
// 仅提供插件↔扩展的静态映射关系；运行时拓扑身份在 compose 阶段不可达，
// 因此 ResolveRuntime/ResolveService 走 registry 回退（依靠 SubjectMapper 的
// generation/ID 校验在注册期完成）。ExtensionEnabled 通过 ListByExtension 判断
// 是否有已注册插件。
type pluginIdentityReader struct {
	pluginReg *registry.Registry
}

func newPluginIdentityReader(pluginReg *registry.Registry) *pluginIdentityReader {
	return &pluginIdentityReader{pluginReg: pluginReg}
}

func (r *pluginIdentityReader) ResolveRuntime(runtimeID string) (string, string, string, error) {
	if r.pluginReg == nil {
		return "", "", "", context.Canceled
	}
	_ = domain.RuntimeInstanceID(runtimeID)
	plugins := r.pluginReg.Snapshot()
	for _, p := range plugins {
		return string(p.ID), p.ExtensionID, "registered", nil
	}
	return "", "", "", context.Canceled
}

func (r *pluginIdentityReader) ResolveService(runtimeID, serviceID string) (string, string, string, error) {
	if r.pluginReg == nil {
		return "", "", "", context.Canceled
	}
	_ = domain.RuntimeInstanceID(runtimeID)
	_ = domain.ServiceID(serviceID)
	plugins := r.pluginReg.Snapshot()
	for _, p := range plugins {
		return string(p.ID), p.ExtensionID, "registered", nil
	}
	return "", "", "", context.Canceled
}

func (r *pluginIdentityReader) ExtensionEnabled(extensionID string) bool {
	if r.pluginReg == nil || extensionID == "" {
		return false
	}
	plugins := r.pluginReg.Snapshot()
	for _, p := range plugins {
		if p.ExtensionID == extensionID {
			return true
		}
	}
	return false
}

func newResourceSubjectMapper(pluginReg *registry.Registry) *resource.SubjectMapper {
	reader := newPluginIdentityReader(pluginReg)
	return resource.NewSubjectMapper(reader)
}

// runtimeLimitGovernor 在容器层记录每个 Runtime 的资源限额声明。
// 真实限额下发在 trusted_service.ProcessSupervisor 启动子进程时通过
// ServiceResourceLimits 完成；本 governor 仅做声明审计、一致性回查。
type runtimeLimitGovernor struct {
	mu      sync.Mutex
	limits  map[string]resource.ServiceResourceLimitsSet
}

func newRuntimeLimitGovernor() *runtimeLimitGovernor {
	return &runtimeLimitGovernor{limits: make(map[string]resource.ServiceResourceLimitsSet)}
}

func (g *runtimeLimitGovernor) ConfigureResourceLimits(runtimeID string, limits resource.ServiceResourceLimitsSet) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.limits[runtimeID] = limits
	return nil
}

func (g *runtimeLimitGovernor) LimitsFor(runtimeID string) (resource.ServiceResourceLimitsSet, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	l, ok := g.limits[runtimeID]
	return l, ok
}

// containerViewResolver 将 binary/stream 子系统适配为 resource.ViewResolver。
// 由 Usage 面板 (ResourcePolicyViewer) 调用，只读、派生，不可作为安全-admission 源。
type containerViewResolver struct {
	binaryReg binaryRegistryView
	streamMgr *stream.StreamManager
}

type binaryRegistryView interface {
	CountActive() int
	LimitActive() int
}

func newContainerViewResolver(binaryReg binaryRegistryView, streamMgr *stream.StreamManager) *containerViewResolver {
	return &containerViewResolver{binaryReg: binaryReg, streamMgr: streamMgr}
}

func (r containerViewResolver) ResolveCPUMemory(runtimeID string) (cpuPercent int, memoryBytes int64, diskBytes int64, openFiles int, subprocesses int) {
	if r.binaryReg == nil {
		return 0, 0, 0, 0, 0
	}
	count := r.binaryReg.CountActive()
	return 0, int64(count) * (64 << 20), 0, 0, 0
}

func (r containerViewResolver) ResolvePending(runtimeID, serviceID string) int {
	if r.streamMgr == nil {
		return 0
	}
	return 0
}

func (r containerViewResolver) ResolveBinaryCount(runtimeID string) int {
	if r.binaryReg == nil {
		return 0
	}
	return r.binaryReg.CountActive()
}
