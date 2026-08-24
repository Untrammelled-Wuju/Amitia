package resource

import (
	"context"
	"sync"
)

type UsageDimension string

const (
	UsageCPUPercent   UsageDimension = "cpu_percent"
	UsageMemoryBytes  UsageDimension = "memory_bytes"
	UsageDiskBytes    UsageDimension = "disk_bytes"
	UsageOpenFiles    UsageDimension = "open_files"
	UsageSubprocesses UsageDimension = "subprocesses"
	UsagePendingRPC   UsageDimension = "pending_rpc"
	UsageQueue        UsageDimension = "queue_inflight"
	UsageBinaryCount  UsageDimension = "binary_count"
	UsageBinaryBytes  UsageDimension = "binary_bytes"
)

type ResourceLimitView struct {
	Dimension UsageDimension `json:"dimension"`
	Limit     int64          `json:"limit"`
	Used      int64          `json:"used"`
	Available bool           `json:"available"`
	Enforced  bool           `json:"enforced"`
}

type ResourcePolicyView struct {
	RuntimeID   string              `json:"runtime_id"`
	ServiceID   string              `json:"service_id"`
	Limits      []ResourceLimitView `json:"limits"`
	GeneratedAt int64               `json:"generated_at_ns"`
}

type UsageSample struct {
	Used      int64
	Limit     int64
	Available bool
	Enforced  bool
}

type ViewResolver interface {
	ResolveUsage(runtimeID, serviceID string) map[UsageDimension]UsageSample
}

type ResourcePolicyViewer struct {
	resolver ViewResolver
	clock    func() int64
	mu       sync.Mutex
}

func NewResourcePolicyViewer(resolver ViewResolver) *ResourcePolicyViewer {
	return &ResourcePolicyViewer{resolver: resolver, clock: func() int64 { return 0 }}
}

func NewResourcePolicyViewerWithClock(resolver ViewResolver, clock func() int64) *ResourcePolicyViewer {
	return &ResourcePolicyViewer{resolver: resolver, clock: clock}
}

func (v *ResourcePolicyViewer) BuildView(ctx context.Context, runtimeID, serviceID string) (ResourcePolicyView, error) {
	if err := ctx.Err(); err != nil {
		return ResourcePolicyView{}, err
	}
	if v.resolver == nil {
		return ResourcePolicyView{RuntimeID: runtimeID, ServiceID: serviceID}, nil
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	samples := v.resolver.ResolveUsage(runtimeID, serviceID)
	dimensions := []UsageDimension{UsageCPUPercent, UsageMemoryBytes, UsageDiskBytes, UsageOpenFiles, UsageSubprocesses, UsagePendingRPC, UsageQueue, UsageBinaryCount, UsageBinaryBytes}
	limits := make([]ResourceLimitView, 0, len(dimensions))
	for _, dimension := range dimensions {
		sample := samples[dimension]
		limits = append(limits, ResourceLimitView{Dimension: dimension, Limit: sample.Limit, Used: sample.Used, Available: sample.Available, Enforced: sample.Enforced})
	}
	return ResourcePolicyView{RuntimeID: runtimeID, ServiceID: serviceID, Limits: limits, GeneratedAt: v.clock()}, nil
}
