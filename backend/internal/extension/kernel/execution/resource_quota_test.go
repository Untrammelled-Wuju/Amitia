package execution

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/platform/process"
)

type fakeProcessManager struct {
	report process.PlatformIsolationReport
}

func (f *fakeProcessManager) IsolationReport() capability.PlatformIsolationReport {
	return capability.PlatformIsolationReport{
		MemoryLimit: f.report.MemoryLimit,
		CPULimit:    f.report.CPULimit,
	}
}

type fakeRuntimeCaps struct {
	caps capability.RuntimeResourceCapabilities
	err  error
}

func (f *fakeRuntimeCaps) ResolveRuntime(
	ctx context.Context,
	binding capability.RuntimeBinding,
) (capability.RuntimeResourceCapabilities, error) {
	return f.caps, f.err
}

func newToolWithLimits(memBytes int64, cpuPct int, req capability.ResourceLimitRequirement) capability.ToolDefinition {
	return capability.ToolDefinition{
		ID: "test/tool-1",
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout: 30 * time.Second,
			ResourceLimits: capability.ResourceLimits{
				MaxMemoryBytes: memBytes,
				MaxCPUPercent:  cpuPct,
				Requirement:    req,
			},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: capability.RuntimeTypeTrustedService,
			RuntimeID:   "test-runtime",
		},
	}
}

func TestResolver_StricterLimit_ToolSmaller(t *testing.T) {
	resolver := NewDefaultResourceLimitResolver(
		&fakeRuntimeCaps{
			caps: capability.RuntimeResourceCapabilities{
				MemoryMode: capability.ResourceEnforcementEnforced,
				MaxMemory:  512,
				CPUMode:    capability.ResourceEnforcementDeclaredOnly,
				MaxCPU:     100,
			},
		},
		&fakeProcessManager{report: process.PlatformIsolationReport{MemoryLimit: true, CPULimit: false}},
	)

	tool := newToolWithLimits(256, 50, capability.ResourceLimitBestEffort)
	inv := capability.ToolInvocationContext{InvocationID: "inv-1"}

	limits, err := resolver.Resolve(context.Background(), tool, inv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	best, ok := limits.StrictestLimit(capability.ResourceMemoryBytes)
	if !ok {
		t.Fatal("expected memory limit")
	}
	if best.Limit != 256 {
		t.Fatalf("expected 256 strictest, got %d", best.Limit)
	}
}

func TestResolver_StricterLimit_RuntimeSmaller(t *testing.T) {
	resolver := NewDefaultResourceLimitResolver(
		&fakeRuntimeCaps{
			caps: capability.RuntimeResourceCapabilities{
				MemoryMode: capability.ResourceEnforcementEnforced,
				MaxMemory:  256,
			},
		},
		&fakeProcessManager{report: process.PlatformIsolationReport{MemoryLimit: true}},
	)

	tool := newToolWithLimits(512, 0, capability.ResourceLimitBestEffort)
	inv := capability.ToolInvocationContext{InvocationID: "inv-2"}

	limits, err := resolver.Resolve(context.Background(), tool, inv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	best, ok := limits.StrictestLimit(capability.ResourceMemoryBytes)
	if !ok || best.Limit != 256 {
		t.Fatalf("expected 256 strictest, got %v ok=%v", best, ok)
	}
}

func TestResolver_InvalidCPUNegative(t *testing.T) {
	resolver := NewDefaultResourceLimitResolver(
		&fakeRuntimeCaps{},
		&fakeProcessManager{},
	)

	tool := newToolWithLimits(0, -1, capability.ResourceLimitBestEffort)
	_, err := resolver.Resolve(context.Background(), tool, capability.ToolInvocationContext{})
	if err == nil {
		t.Fatal("expected error for negative CPU")
	}
}

func TestResolver_InvalidCPUGreaterThan100(t *testing.T) {
	resolver := NewDefaultResourceLimitResolver(
		&fakeRuntimeCaps{},
		&fakeProcessManager{},
	)

	tool := newToolWithLimits(0, 150, capability.ResourceLimitBestEffort)
	_, err := resolver.Resolve(context.Background(), tool, capability.ToolInvocationContext{})
	if err == nil {
		t.Fatal("expected error for CPU > 100")
	}
}

func TestController_RequireEnforced_AllowedWhenEnforced(t *testing.T) {
	resolver := NewDefaultResourceLimitResolver(
		&fakeRuntimeCaps{
			caps: capability.RuntimeResourceCapabilities{
				MemoryMode: capability.ResourceEnforcementEnforced,
				MaxMemory:  256,
			},
		},
		&fakeProcessManager{report: process.PlatformIsolationReport{MemoryLimit: true}},
	)

	ctrl := NewResourceQuotaController(resolver)
	tool := newToolWithLimits(256, 0, capability.ResourceLimitRequireEnforced)

	dec := ctrl.Evaluate(context.Background(), tool, capability.ToolInvocationContext{InvocationID: "inv-3"})
	if dec.Blocked {
		t.Fatalf("expected not blocked, got blocked: %v", dec.Error)
	}
}

func TestController_RequireEnforced_BlockedWhenNotEnforced(t *testing.T) {
	resolver := NewDefaultResourceLimitResolver(
		&fakeRuntimeCaps{
			caps: capability.RuntimeResourceCapabilities{
				MemoryMode: capability.ResourceEnforcementDeclaredOnly,
				MaxMemory:  256,
			},
		},
		&fakeProcessManager{report: process.PlatformIsolationReport{MemoryLimit: false}},
	)

	ctrl := NewResourceQuotaController(resolver)
	tool := newToolWithLimits(256, 0, capability.ResourceLimitRequireEnforced)

	dec := ctrl.Evaluate(context.Background(), tool, capability.ToolInvocationContext{InvocationID: "inv-4"})
	if !dec.Blocked {
		t.Fatal("expected blocked when require_enforced not satisfied")
	}
	if dec.ErrorCode != capability.ErrorCodeResourceLimitUnavailable {
		t.Fatalf("expected resource_limit_unavailable, got %s", dec.ErrorCode)
	}
}

func TestController_BestEffort_AllowsDispatch(t *testing.T) {
	resolver := NewDefaultResourceLimitResolver(
		&fakeRuntimeCaps{
			caps: capability.RuntimeResourceCapabilities{
				MemoryMode: capability.ResourceEnforcementDeclaredOnly,
				MaxMemory:  256,
			},
		},
		&fakeProcessManager{report: process.PlatformIsolationReport{MemoryLimit: false}},
	)

	ctrl := NewResourceQuotaController(resolver)
	tool := newToolWithLimits(256, 0, capability.ResourceLimitBestEffort)

	dec := ctrl.Evaluate(context.Background(), tool, capability.ToolInvocationContext{InvocationID: "inv-5"})
	if dec.Blocked {
		t.Fatalf("best_effort should allow dispatch, got blocked: %v", dec.Error)
	}

	mem, ok := dec.Limits.ForDimension(capability.ResourceMemoryBytes)
	if !ok {
		t.Fatal("expected memory limit present")
	}
	if mem.Mode != capability.ResourceEnforcementDeclaredOnly {
		t.Fatalf("expected declared_only mode for best_effort linux, got %s", mem.Mode)
	}
}

func TestController_InvalidMemoryNegative(t *testing.T) {
	resolver := NewDefaultResourceLimitResolver(
		&fakeRuntimeCaps{},
		&fakeProcessManager{},
	)

	ctrl := NewResourceQuotaController(resolver)
	tool := capability.ToolDefinition{
		ID: "test/tool-neg",
		ExecutionPolicy: capability.ToolExecutionPolicy{
			ResourceLimits: capability.ResourceLimits{MaxMemoryBytes: -1},
		},
	}

	dec := ctrl.Evaluate(context.Background(), tool, capability.ToolInvocationContext{InvocationID: "inv-6"})
	if !dec.Blocked {
		t.Fatal("expected blocked for negative memory")
	}
	if dec.ErrorCode != capability.ErrorCodeResourceLimitInvalid {
		t.Fatalf("expected resource_limit_invalid, got %s", dec.ErrorCode)
	}
}

func TestController_InvalidCPU(t *testing.T) {
	resolver := NewDefaultResourceLimitResolver(
		&fakeRuntimeCaps{},
		&fakeProcessManager{},
	)

	ctrl := NewResourceQuotaController(resolver)
	tool := newToolWithLimits(0, 200, capability.ResourceLimitBestEffort)

	dec := ctrl.Evaluate(context.Background(), tool, capability.ToolInvocationContext{InvocationID: "inv-7"})
	if !dec.Blocked {
		t.Fatal("expected blocked for CPU > 100")
	}
	if dec.ErrorCode != capability.ErrorCodeResourceLimitInvalid {
		t.Fatalf("expected resource_limit_invalid, got %s", dec.ErrorCode)
	}
}

func TestController_NilResolver_Blocks(t *testing.T) {
	ctrl := NewResourceQuotaController(nil)
	tool := newToolWithLimits(256, 0, capability.ResourceLimitBestEffort)

	dec := ctrl.Evaluate(context.Background(), tool, capability.ToolInvocationContext{InvocationID: "inv-8"})
	if !dec.Blocked {
		t.Fatal("nil resolver should block")
	}
}

func TestController_ResolverError_Propagates(t *testing.T) {
	resolver := &errorResolver{err: errors.New("resolve failed")}

	ctrl := NewResourceQuotaController(resolver)
	tool := newToolWithLimits(256, 0, capability.ResourceLimitBestEffort)

	dec := ctrl.Evaluate(context.Background(), tool, capability.ToolInvocationContext{InvocationID: "inv-9"})
	if !dec.Blocked {
		t.Fatal("resolver error should block")
	}
}

type errorResolver struct {
	err error
}

func (r *errorResolver) Resolve(
	ctx context.Context,
	tool capability.ToolDefinition,
	inv capability.ToolInvocationContext,
) (capability.EffectiveResourceLimits, error) {
	return capability.EffectiveResourceLimits{}, r.err
}

func TestPlatformIsolationAdapter(t *testing.T) {
	calls := 0
	adapter := NewPlatformIsolationAdapter(func() process.PlatformIsolationReport {
		calls++
		return process.PlatformIsolationReport{MemoryLimit: true, CPULimit: false}
	})

	report := adapter.IsolationReport()
	if !report.MemoryLimit {
		t.Fatal("expected MemoryLimit=true")
	}
	if report.CPULimit {
		t.Fatal("expected CPULimit=false")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRuntimeCapabilityAdapter(t *testing.T) {
	adapter := NewRuntimeCapabilityAdapter(func(
		ctx context.Context,
		binding capability.RuntimeBinding,
	) (capability.RuntimeResourceCapabilities, error) {
		return capability.RuntimeResourceCapabilities{
			MemoryMode: capability.ResourceEnforcementEnforced,
			MaxMemory:  1024,
		}, nil
	})

	caps, err := adapter.ResolveRuntime(context.Background(), capability.RuntimeBinding{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if caps.MaxMemory != 1024 {
		t.Fatalf("expected 1024, got %d", caps.MaxMemory)
	}
	if caps.MemoryMode != capability.ResourceEnforcementEnforced {
		t.Fatalf("expected enforced, got %s", caps.MemoryMode)
	}
}

func TestResolver_LinuxNoPseudoEnforced(t *testing.T) {
	resolver := NewDefaultResourceLimitResolver(
		&fakeRuntimeCaps{
			caps: capability.RuntimeResourceCapabilities{
				MemoryMode: capability.ResourceEnforcementEnforced,
				MaxMemory:  256,
			},
		},
		&fakeProcessManager{report: process.PlatformIsolationReport{MemoryLimit: false, CPULimit: false}},
	)

	tool := newToolWithLimits(256, 50, capability.ResourceLimitBestEffort)
	limits, err := resolver.Resolve(context.Background(), tool, capability.ToolInvocationContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, l := range limits.Limits {
		if l.Source == capability.ResourceLimitSourceRuntime && l.Dimension == capability.ResourceMemoryBytes {
			if l.Mode == capability.ResourceEnforcementEnforced {
				t.Fatalf("linux without cgroup must NOT report enforced, got %s", l.Mode)
			}
		}
	}
}

func TestResolver_WindowsMemoryEnforced(t *testing.T) {
	resolver := NewDefaultResourceLimitResolver(
		&fakeRuntimeCaps{
			caps: capability.RuntimeResourceCapabilities{
				MemoryMode: capability.ResourceEnforcementEnforced,
				MaxMemory:  256,
			},
		},
		&fakeProcessManager{report: process.PlatformIsolationReport{MemoryLimit: true, CPULimit: false}},
	)

	tool := newToolWithLimits(256, 0, capability.ResourceLimitBestEffort)
	limits, err := resolver.Resolve(context.Background(), tool, capability.ToolInvocationContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	best, ok := limits.StrictestLimit(capability.ResourceMemoryBytes)
	if !ok {
		t.Fatal("expected memory limit found")
	}

	hasEnforced := false
	for _, l := range limits.Limits {
		if l.Mode == capability.ResourceEnforcementEnforced && l.Dimension == capability.ResourceMemoryBytes {
			hasEnforced = true
		}
	}
	_ = best
	if !hasEnforced {
		t.Fatal("windows with job object should report memory enforced")
	}
}

func TestController_RequireEnforced_CPURange(t *testing.T) {
	resolver := NewDefaultResourceLimitResolver(
		&fakeRuntimeCaps{
			caps: capability.RuntimeResourceCapabilities{
				MemoryMode: capability.ResourceEnforcementEnforced,
				CPUMode:    capability.ResourceEnforcementEnforced,
				MaxMemory:  256,
				MaxCPU:     150,
			},
		},
		&fakeProcessManager{report: process.PlatformIsolationReport{MemoryLimit: true, CPULimit: true}},
	)

	ctrl := NewResourceQuotaController(resolver)
	tool := newToolWithLimits(256, 0, capability.ResourceLimitRequireEnforced)

	dec := ctrl.Evaluate(context.Background(), tool, capability.ToolInvocationContext{InvocationID: "inv-10"})
	if !dec.Blocked {
		t.Fatal("expected blocked for CPU=150 > 100")
	}
}

func TestResourceLimitRequirement_DefaultBestEffort(t *testing.T) {
	rl := capability.ResourceLimits{
		MaxMemoryBytes: 256,
	}
	if rl.Requirement != "" {
		t.Fatalf("default requirement should be empty, got %s", rl.Requirement)
	}
}
