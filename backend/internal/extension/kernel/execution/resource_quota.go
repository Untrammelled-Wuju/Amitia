package execution

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/platform/process"
)

type ResourceLimitResolver interface {
	Resolve(
		ctx context.Context,
		tool capability.ToolDefinition,
		inv capability.ToolInvocationContext,
	) (capability.EffectiveResourceLimits, error)
}

type DefaultResourceLimitResolver struct {
	runtimeCapabilities capability.RuntimeResourceCapabilityResolver
	platformIsolation   capability.PlatformIsolationProvider
}

func NewDefaultResourceLimitResolver(
	runtimeCaps capability.RuntimeResourceCapabilityResolver,
	platformIso capability.PlatformIsolationProvider,
) *DefaultResourceLimitResolver {
	return &DefaultResourceLimitResolver{
		runtimeCapabilities: runtimeCaps,
		platformIsolation:   platformIso,
	}
}

func (r *DefaultResourceLimitResolver) Resolve(
	ctx context.Context,
	tool capability.ToolDefinition,
	inv capability.ToolInvocationContext,
) (capability.EffectiveResourceLimits, error) {
	limits := capability.EffectiveResourceLimits{}

	toolMem := tool.ExecutionPolicy.ResourceLimits.MaxMemoryBytes
	if toolMem > 0 {
		limits.Limits = append(limits.Limits, capability.EffectiveResourceLimit{
			Dimension: capability.ResourceMemoryBytes,
			Limit:     toolMem,
			Mode:      capability.ResourceEnforcementDeclaredOnly,
			Source:    capability.ResourceLimitSourceToolPolicy,
		})
	} else if toolMem < 0 {
		return capability.EffectiveResourceLimits{}, fmt.Errorf(
			"resource_limit_invalid: memory bytes %d is negative", toolMem,
		)
	}

	toolCPU := tool.ExecutionPolicy.ResourceLimits.MaxCPUPercent
	if toolCPU > 0 {
		if toolCPU < 0 || toolCPU > 100 {
			return capability.EffectiveResourceLimits{}, fmt.Errorf(
				"resource_limit_invalid: cpu percent %d out of range [1,100]", toolCPU,
			)
		}
		limits.Limits = append(limits.Limits, capability.EffectiveResourceLimit{
			Dimension: capability.ResourceCPUPercent,
			Limit:     int64(toolCPU),
			Mode:      capability.ResourceEnforcementDeclaredOnly,
			Source:    capability.ResourceLimitSourceToolPolicy,
		})
	} else if toolCPU < 0 {
		return capability.EffectiveResourceLimits{}, fmt.Errorf(
			"resource_limit_invalid: cpu percent %d is negative", toolCPU,
		)
	}

	var platformReport capability.PlatformIsolationReport
	if r.platformIsolation != nil {
		platformReport = r.platformIsolation.IsolationReport()
	}

	runtimeCaps, runtimeErr := r.resolveRuntimeCapabilities(ctx, tool)
	if runtimeErr == nil {
		if runtimeCaps.MaxMemory > 0 {
			memMode := r.resolveMemoryMode(runtimeCaps.MemoryMode, platformReport)
			limits.Limits = append(limits.Limits, capability.EffectiveResourceLimit{
				Dimension: capability.ResourceMemoryBytes,
				Limit:     runtimeCaps.MaxMemory,
				Mode:      memMode,
				Source:    capability.ResourceLimitSourceRuntime,
			})
		}

		if runtimeCaps.MaxCPU > 0 {
			cpuMode := r.resolveCPUMode(runtimeCaps.CPUMode, platformReport)
			limits.Limits = append(limits.Limits, capability.EffectiveResourceLimit{
				Dimension: capability.ResourceCPUPercent,
				Limit:     int64(runtimeCaps.MaxCPU),
				Mode:      cpuMode,
				Source:    capability.ResourceLimitSourceRuntime,
			})
		}
	}

	if platformReport.MemoryLimit {
		limits.Limits = append(limits.Limits, capability.EffectiveResourceLimit{
			Dimension: capability.ResourceMemoryBytes,
			Limit:     math.MaxInt64,
			Mode:      capability.ResourceEnforcementEnforced,
			Source:    capability.ResourceLimitSourcePlatform,
		})
	}
	if platformReport.CPULimit {
		limits.Limits = append(limits.Limits, capability.EffectiveResourceLimit{
			Dimension: capability.ResourceCPUPercent,
			Limit:     100,
			Mode:      capability.ResourceEnforcementEnforced,
			Source:    capability.ResourceLimitSourcePlatform,
		})
	}

	return limits, nil
}

func (r *DefaultResourceLimitResolver) resolveRuntimeCapabilities(
	ctx context.Context,
	tool capability.ToolDefinition,
) (capability.RuntimeResourceCapabilities, error) {
	if r.runtimeCapabilities == nil {
		return capability.RuntimeResourceCapabilities{}, errors.New("no runtime capability resolver")
	}
	return r.runtimeCapabilities.ResolveRuntime(ctx, tool.Runtime)
}

func (r *DefaultResourceLimitResolver) resolveMemoryMode(
	runtimeMode capability.ResourceEnforcementMode,
	platformReport capability.PlatformIsolationReport,
) capability.ResourceEnforcementMode {
	if runtimeMode == capability.ResourceEnforcementEnforced && platformReport.MemoryLimit {
		return capability.ResourceEnforcementEnforced
	}
	if runtimeMode == capability.ResourceEnforcementEnforced && !platformReport.MemoryLimit {
		return capability.ResourceEnforcementMeasured
	}
	return runtimeMode
}

func (r *DefaultResourceLimitResolver) resolveCPUMode(
	runtimeMode capability.ResourceEnforcementMode,
	platformReport capability.PlatformIsolationReport,
) capability.ResourceEnforcementMode {
	if runtimeMode == capability.ResourceEnforcementEnforced && platformReport.CPULimit {
		return capability.ResourceEnforcementEnforced
	}
	if runtimeMode == capability.ResourceEnforcementEnforced && !platformReport.CPULimit {
		return capability.ResourceEnforcementMeasured
	}
	return runtimeMode
}

type ResourceQuotaController struct {
	resolver ResourceLimitResolver
}

func NewResourceQuotaController(resolver ResourceLimitResolver) *ResourceQuotaController {
	return &ResourceQuotaController{resolver: resolver}
}

type ResourceQuotaDecision struct {
	Limits    capability.EffectiveResourceLimits
	Blocked   bool
	Error     error
	ErrorCode string
}

func (c *ResourceQuotaController) Evaluate(
	ctx context.Context,
	tool capability.ToolDefinition,
	inv capability.ToolInvocationContext,
) ResourceQuotaDecision {
	if c.resolver == nil {
		return ResourceQuotaDecision{Blocked: true, Error: errors.New("resource resolver not configured")}
	}

	limits, err := c.resolver.Resolve(ctx, tool, inv)
	if err != nil {
		ec := capability.ErrorCodeResourceLimitInvalid
		if errors.Is(err, errResourceUnavailable) {
			ec = capability.ErrorCodeResourceLimitUnavailable
		}
		return ResourceQuotaDecision{
			Blocked:   true,
			Error:     err,
			ErrorCode: ec,
		}
	}

	requireEnforced := tool.ExecutionPolicy.ResourceLimits.Requirement == capability.ResourceLimitRequireEnforced

	for i := range limits.Limits {
		l := &limits.Limits[i]
		if l.Limit <= 0 {
			continue
		}
		switch l.Dimension {
		case capability.ResourceCPUPercent:
			if l.Limit < 1 || l.Limit > 100 {
				return ResourceQuotaDecision{
					Blocked:   true,
					Error:     fmt.Errorf("resource_limit_invalid: cpu percent %d out of range [1,100]", l.Limit),
					ErrorCode: capability.ErrorCodeResourceLimitInvalid,
				}
			}
		case capability.ResourceMemoryBytes:
			if l.Limit < 0 {
				return ResourceQuotaDecision{
					Blocked:   true,
					Error:     fmt.Errorf("resource_limit_invalid: memory bytes %d is negative", l.Limit),
					ErrorCode: capability.ErrorCodeResourceLimitInvalid,
				}
			}
		}
	}

	if requireEnforced {
		declaredDims := make(map[capability.ResourceDimension]bool)
		for i := range limits.Limits {
			if limits.Limits[i].Limit > 0 {
				declaredDims[limits.Limits[i].Dimension] = true
			}
		}
		for dim := range declaredDims {
			best, ok := limits.StrictestLimit(dim)
			if !ok {
				continue
			}
			if best.Mode != capability.ResourceEnforcementEnforced {
				return ResourceQuotaDecision{
					Blocked:   true,
					Limits:    limits,
					Error:     fmt.Errorf("resource_limit_unavailable: dimension %s strictest mode=%s, require_enforced not satisfied", dim, best.Mode),
					ErrorCode: capability.ErrorCodeResourceLimitUnavailable,
				}
			}
		}
	}

	return ResourceQuotaDecision{Limits: limits}
}

var errResourceUnavailable = errors.New("resource_limit_unavailable")

type PlatformIsolationAdapter struct {
	report func() process.PlatformIsolationReport
}

func NewPlatformIsolationAdapter(reportFunc func() process.PlatformIsolationReport) *PlatformIsolationAdapter {
	return &PlatformIsolationAdapter{report: reportFunc}
}

func (a *PlatformIsolationAdapter) IsolationReport() capability.PlatformIsolationReport {
	if a.report == nil {
		return capability.PlatformIsolationReport{}
	}
	r := a.report()
	return capability.PlatformIsolationReport{
		MemoryLimit: r.MemoryLimit,
		CPULimit:    r.CPULimit,
	}
}

type RuntimeCapabilityAdapter struct {
	resolveFn func(ctx context.Context, binding capability.RuntimeBinding) (capability.RuntimeResourceCapabilities, error)
}

func NewRuntimeCapabilityAdapter(
	resolveFn func(ctx context.Context, binding capability.RuntimeBinding) (capability.RuntimeResourceCapabilities, error),
) *RuntimeCapabilityAdapter {
	return &RuntimeCapabilityAdapter{resolveFn: resolveFn}
}

func (a *RuntimeCapabilityAdapter) ResolveRuntime(
	ctx context.Context,
	binding capability.RuntimeBinding,
) (capability.RuntimeResourceCapabilities, error) {
	if a.resolveFn == nil {
		return capability.RuntimeResourceCapabilities{}, errors.New("no resolve function")
	}
	return a.resolveFn(ctx, binding)
}

type ResolveRuntimeCapabilitiesFunc func(
	ctx context.Context,
	binding capability.RuntimeBinding,
) (capability.RuntimeResourceCapabilities, error)

func DefaultRuntimeCapabilities(
	ctx context.Context,
	binding capability.RuntimeBinding,
) (capability.RuntimeResourceCapabilities, error) {
	switch binding.RuntimeType {
	case capability.RuntimeTypeWASM:
		return capability.RuntimeResourceCapabilities{
			MemoryMode: capability.ResourceEnforcementEnforced,
			CPUMode:    capability.ResourceEnforcementDeclaredOnly,
			MaxMemory:  0,
			MaxCPU:     0,
			RuntimeID:  binding.RuntimeID,
		}, nil
	case capability.RuntimeTypeTrustedService,
		capability.RuntimeTypeJavaScript,
		capability.RuntimeTypePluginJS,
		capability.RuntimeTypeTask,
		capability.RuntimeTypePluginService:
		return capability.RuntimeResourceCapabilities{
			MemoryMode: capability.ResourceEnforcementEnforced,
			CPUMode:    capability.ResourceEnforcementDeclaredOnly,
			MaxMemory:  0,
			MaxCPU:     0,
			RuntimeID:  binding.RuntimeID,
		}, nil
	default:
		return capability.RuntimeResourceCapabilities{
			MemoryMode: capability.ResourceEnforcementUnsupported,
			CPUMode:    capability.ResourceEnforcementUnsupported,
			RuntimeID:  binding.RuntimeID,
		}, nil
	}
}
