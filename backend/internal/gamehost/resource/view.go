package resource

import (
	"context"
	"sync"
)

type UsageDimension string

const (
	UsageCPUPercent    UsageDimension = "cpu_percent"
	UsageMemoryBytes   UsageDimension = "memory_bytes"
	UsageDiskBytes     UsageDimension = "disk_bytes"
	UsageOpenFiles     UsageDimension = "open_files"
	UsageSubprocesses  UsageDimension = "subprocesses"
	UsagePendingRPC    UsageDimension = "pending_rpc"
	UsageBinaryCount   UsageDimension = "binary_count"
)

type ResourceLimitView struct {
	Dimension UsageDimension `json:"dimension"`
	Limit     int64          `json:"limit"`
	Used      int64          `json:"used"`
}

type ResourcePolicyView struct {
	RuntimeID   string              `json:"runtime_id"`
	ServiceID   string              `json:"service_id"`
	Limits      []ResourceLimitView `json:"limits"`
	GeneratedAt int64               `json:"generated_at_ns"`
}

type ViewResolver interface {
	ResolveCPUMemory(runtimeID string) (cpuPercent int, memoryBytes int64, diskBytes int64, openFiles int, subprocesses int)
	ResolvePending(runtimeID, serviceID string) int
	ResolveBinaryCount(runtimeID string) int
}

type ResourcePolicyViewer struct {
	resolver ViewResolver
	clock    func() int64
	mu       sync.Mutex
}

func NewResourcePolicyViewer(resolver ViewResolver) *ResourcePolicyViewer {
	return &ResourcePolicyViewer{
		resolver: resolver,
		clock:    func() int64 { return 0 },
	}
}

func NewResourcePolicyViewerWithClock(resolver ViewResolver, clock func() int64) *ResourcePolicyViewer {
	return &ResourcePolicyViewer{resolver: resolver, clock: clock}
}

func (v *ResourcePolicyViewer) BuildView(ctx context.Context, runtimeID, serviceID string) (ResourcePolicyView, error) {
	if v.resolver == nil {
		return ResourcePolicyView{RuntimeID: runtimeID, ServiceID: serviceID}, nil
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	cpu, mem, disk, of, sub := v.resolver.ResolveCPUMemory(runtimeID)
	pending := v.resolver.ResolvePending(runtimeID, serviceID)
	binary := v.resolver.ResolveBinaryCount(runtimeID)

	return ResourcePolicyView{
		RuntimeID: runtimeID,
		ServiceID: serviceID,
		Limits: []ResourceLimitView{
			{Dimension: UsageCPUPercent, Limit: int64(cpu) * 10, Used: int64(cpu)},
			{Dimension: UsageMemoryBytes, Limit: mem, Used: mem},
			{Dimension: UsageDiskBytes, Limit: disk, Used: disk},
			{Dimension: UsageOpenFiles, Limit: int64(of) * 10, Used: int64(of)},
			{Dimension: UsageSubprocesses, Limit: int64(sub), Used: int64(sub)},
			{Dimension: UsagePendingRPC, Limit: int64(pending) + 8, Used: int64(pending)},
			{Dimension: UsageBinaryCount, Limit: int64(binary) + 64, Used: int64(binary)},
		},
		GeneratedAt: v.clock(),
	}, nil
}
