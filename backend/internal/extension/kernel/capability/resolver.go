package capability

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type CapabilityResolver interface {
	Resolve(request CapabilityResolutionRequest) (CapabilityResolution, error)
}

type RuntimeAvailabilityPort interface {
	IsRuntimeOnline(ctx context.Context, runtimeID runtimeidentity.RuntimeID) (bool, error)
	IsDeviceOffline(ctx context.Context, deviceID runtimeidentity.DeviceID) (bool, error)
}

type Resolver struct {
	catalog      ProviderCatalog
	runtime      RuntimeCatalog
	host         RoutingHostContext
	policy       RoutingPolicy
	availability RuntimeAvailabilityPort
}

func NewResolver(catalog ProviderCatalog) *Resolver {
	return &Resolver{
		catalog:      catalog,
		policy:       ProductionRoutingPolicy(),
		availability: &strictRuntimeAvailability{},
	}
}

func NewResolverWithPolicy(catalog ProviderCatalog, policy RoutingPolicy) *Resolver {
	return &Resolver{
		catalog:      catalog,
		policy:       policy,
		availability: &strictRuntimeAvailability{},
	}
}

func (r *Resolver) SetRuntimeCatalog(rt RuntimeCatalog) {
	r.runtime = rt
}

func (r *Resolver) SetHostContext(host RoutingHostContext) {
	r.host = host
}

func (r *Resolver) SetPolicy(policy RoutingPolicy) {
	r.policy = policy
}

func (r *Resolver) SetAvailability(availability RuntimeAvailabilityPort) {
	if availability != nil {
		r.availability = availability
	}
}

type strictRuntimeAvailability struct{}

var ErrRuntimeAvailabilityCheck = errors.New("capability: runtime availability checker not configured")

func (n *strictRuntimeAvailability) IsRuntimeOnline(_ context.Context, _ runtimeidentity.RuntimeID) (bool, error) {
	return false, ErrRuntimeAvailabilityCheck
}

func (n *strictRuntimeAvailability) IsDeviceOffline(_ context.Context, _ runtimeidentity.DeviceID) (bool, error) {
	return false, ErrRuntimeAvailabilityCheck
}

func (r *Resolver) Resolve(request CapabilityResolutionRequest) (CapabilityResolution, error) {
	start := time.Now()
	result := CapabilityResolution{
		CapabilityID: request.CapabilityID,
		Decision:     RoutingNoProvider,
	}

	if ParseCapabilityID(string(request.CapabilityID)) == "" {
		result.ReasonCodes = append(result.ReasonCodes, string(ResolutionFailureCapabilityNotRegistered))
		result.Decision = RoutingNoProvider
		result.Trace = r.buildTrace(request, result, start)
		return result, fmt.Errorf("invalid capability id: %s", request.CapabilityID)
	}

	defs := r.catalog.ListDefinitionsByCapability(request.CapabilityID)
	if len(defs) == 0 {
		result.ReasonCodes = append(result.ReasonCodes, string(ResolutionFailureCapabilityNotRegistered))
		result.Decision = RoutingNoProvider
		result.Trace = r.buildTrace(request, result, start)
		return result, fmt.Errorf("%w: %s", ErrCapabilityNotRegistered, request.CapabilityID)
	}

	hardFiltered, rejections := r.applyHardFilter(defs, request)
	result.CandidateCount = len(hardFiltered)
	result.RejectedCount = len(rejections)

	if len(hardFiltered) == 0 {
		result.ReasonCodes = append(result.ReasonCodes, string(ResolutionFailureNoAvailableProvider))
		result.Decision = r.classifyRejection(rejections)
		if result.Decision == RoutingDeviceOffline {
			result.ReasonCodes = append(result.ReasonCodes, string(ResolutionFailureDeviceOffline))
		}
		result.Trace = r.buildTraceWithRejections(request, result, rejections, start)
		return result, fmt.Errorf("%w: %s", ErrNoAvailableProvider, request.CapabilityID)
	}

	instances, instanceRejections := r.collectExecutableInstances(hardFiltered, request)
	rejections = append(rejections, instanceRejections...)
	if len(instances) == 0 {
		result.ReasonCodes = append(result.ReasonCodes, string(ResolutionFailureNoAvailableProvider))
		result.Decision = r.classifyRejection(rejections)
		if result.Decision == RoutingDeviceOffline {
			result.ReasonCodes = append(result.ReasonCodes, string(ResolutionFailureDeviceOffline))
		}
		result.Trace = r.buildTraceWithRejections(request, result, rejections, start)
		return result, fmt.Errorf("%w: no executable instance for %s", ErrNoAvailableProvider, request.CapabilityID)
	}

	ranking := &ResolutionRanking{
		defs:      hardFiltered,
		instances: instances,
		request:   request,
	}

	ranked, err := ranking.Rank()
	if err != nil {
		result.ReasonCodes = append(result.ReasonCodes, string(ResolutionFailureProviderConflict))
		result.Decision = RoutingNoProvider
		result.Trace = r.buildTraceWithRejections(request, result, rejections, start)
		return result, fmt.Errorf("ranking conflict: %w", err)
	}
	if len(ranked) == 0 {
		result.ReasonCodes = append(result.ReasonCodes, string(ResolutionFailureNoAvailableProvider))
		result.Decision = RoutingNoHealthyInstance
		result.Trace = r.buildTraceWithRejections(request, result, rejections, start)
		return result, fmt.Errorf("%w: no ranked provider for %s", ErrNoAvailableProvider, request.CapabilityID)
	}

	winner := ranked[0]
	result.Provider = *winner.Definition
	result.ProviderInstance = *winner.Instance
	result.ExecutionTarget = buildExecutionTarget(winner.Definition, winner.Instance)
	result.Decision = RoutingResolved
	result.ReasonCodes = nil
	result.Trace = r.buildTraceWithRejections(request, result, rejections, start)

	return result, nil
}

func (r *Resolver) applyHardFilter(defs []CapabilityProviderDefinition, request CapabilityResolutionRequest) ([]CapabilityProviderDefinition, []CandidateRejection) {
	var filtered []CapabilityProviderDefinition
	var rejections []CandidateRejection

	for i := range defs {
		def := defs[i]
		if def.CapabilityID != request.CapabilityID {
			rejections = append(rejections, CandidateRejection{
				ProviderID: def.ID,
				Reason:     RejectionCapabilityMismatch,
			})
			continue
		}
		if !r.policy.AllowCore && def.Placement == ProviderPlacementCore {
			rejections = append(rejections, CandidateRejection{
				ProviderID: def.ID,
				Reason:     RejectionPlacementNotAllowed,
			})
			continue
		}
		if !r.policy.AllowDevice && def.Placement == ProviderPlacementDevice {
			rejections = append(rejections, CandidateRejection{
				ProviderID: def.ID,
				Reason:     RejectionPlacementNotAllowed,
			})
			continue
		}
		if request.RequiredPlacement != "" && def.Placement != request.RequiredPlacement {
			rejections = append(rejections, CandidateRejection{
				ProviderID: def.ID,
				Reason:     RejectionPlacementNotAllowed,
			})
			continue
		}
		if def.RoutingMode == RoutingModeProviderRequired && request.PreferredProviderID != "" && string(def.ID) != string(request.PreferredProviderID) {
			rejections = append(rejections, CandidateRejection{
				ProviderID: def.ID,
				Reason:     RejectionProviderUnavailable,
			})
			continue
		}
		if r.policy.RequireAvailable || r.policy.RequireHealthy {
			insts := r.catalog.ListInstancesByProvider(def.ID)
			hasHealthy := false
			for _, inst := range insts {
				if inst.IsExecutable() {
					hasHealthy = true
					break
				}
			}
			if !hasHealthy {
				rejections = append(rejections, CandidateRejection{
					ProviderID: def.ID,
					Reason:     RejectionProviderUnhealthy,
				})
				continue
			}
		}
		if request.Platform != "" && !matchPlatform(def.Platforms, request.Platform) {
			rejections = append(rejections, CandidateRejection{
				ProviderID: def.ID,
				Reason:     RejectionPlatformMismatch,
			})
			continue
		}
		if def.Runtime.RuntimeType == "" {
			rejections = append(rejections, CandidateRejection{
				ProviderID: def.ID,
				Reason:     RejectionRuntimeTypeMismatch,
			})
			continue
		}
		if r.runtime != nil && !r.runtime.Supports(def.Runtime.RuntimeType) {
			rejections = append(rejections, CandidateRejection{
				ProviderID: def.ID,
				Reason:     RejectionRuntimeAdapterUnavailable,
			})
			continue
		}
		if request.ExtensionID != "" && def.ExtensionID != request.ExtensionID {
			rejections = append(rejections, CandidateRejection{
				ProviderID: def.ID,
				Reason:     RejectionCapabilityMismatch,
			})
			continue
		}
		if request.ModuleID != "" && def.ModuleID != request.ModuleID {
			rejections = append(rejections, CandidateRejection{
				ProviderID: def.ID,
				Reason:     RejectionCapabilityMismatch,
			})
			continue
		}
		filtered = append(filtered, def)
	}
	return filtered, rejections
}

func (r *Resolver) collectExecutableInstances(defs []CapabilityProviderDefinition, request CapabilityResolutionRequest) ([]CapabilityProviderInstance, []CandidateRejection) {
	seen := make(map[ProviderInstanceID]bool)
	var instances []CapabilityProviderInstance
	var rejections []CandidateRejection

	for i := range defs {
		def := defs[i]
		providerInsts := r.catalog.ListInstancesByProvider(def.ID)
		for _, inst := range providerInsts {
			if request.RequiredDeviceID != "" && string(inst.DeviceID) != string(request.RequiredDeviceID) {
				continue
			}
			if seen[inst.ID] {
				continue
			}
		if !inst.IsExecutable() {
			if r.availability != nil && inst.DeviceID != "" {
				offline, err := r.availability.IsDeviceOffline(context.Background(), inst.DeviceID)
				if err == nil && offline {
					rejections = append(rejections, CandidateRejection{
						ProviderID: def.ID,
						Reason:     RejectionDeviceOffline,
					})
					seen[inst.ID] = true
					continue
				}
			}
			continue
		}

		if inst.DeviceID != "" && r.availability != nil {
			offline, err := r.availability.IsDeviceOffline(context.Background(), inst.DeviceID)
			if err == nil && offline {
				rejections = append(rejections, CandidateRejection{
					ProviderID: def.ID,
					Reason:     RejectionDeviceOffline,
				})
				seen[inst.ID] = true
				continue
			}
		}

		if inst.RuntimeID != "" && r.availability != nil {
			online, err := r.availability.IsRuntimeOnline(context.Background(), inst.RuntimeID)
			if err != nil || !online {
				rejections = append(rejections, CandidateRejection{
					ProviderID: def.ID,
					Reason:     RejectionRuntimeAdapterUnavailable,
				})
				seen[inst.ID] = true
				continue
			}
		}

		seen[inst.ID] = true
		instances = append(instances, inst)
		}
	}
	return instances, rejections
}

func (r *Resolver) classifyRejection(rejections []CandidateRejection) RoutingDecision {
	if len(rejections) == 0 {
		return RoutingNoProvider
	}
	counts := make(map[CandidateRejectionReason]int)
	for _, rej := range rejections {
		counts[rej.Reason]++
	}
	if counts[RejectionPlacementNotAllowed] > 0 {
		return RoutingPlacementUnavailable
	}
	if counts[RejectionPlatformMismatch] > 0 {
		return RoutingPlatformMismatch
	}
	if counts[RejectionRuntimeTypeMismatch] > 0 || counts[RejectionRuntimeAdapterUnavailable] > 0 {
		return RoutingRuntimeUnavailable
	}
	if counts[RejectionDeviceOffline] > 0 {
		return RoutingDeviceOffline
	}
	if counts[RejectionProviderUnhealthy] > 0 || counts[RejectionProviderUnavailable] > 0 {
		return RoutingNoHealthyInstance
	}
	return RoutingNoProvider
}

func (r *Resolver) buildTrace(request CapabilityResolutionRequest, result CapabilityResolution, start time.Time) *RoutingTrace {
	return &RoutingTrace{
		CapabilityID:   request.CapabilityID,
		CandidateCount: result.CandidateCount,
		RejectedCount:  result.RejectedCount,
		Decision:       result.Decision,
		Duration:       time.Since(start),
	}
}

func (r *Resolver) buildTraceWithRejections(request CapabilityResolutionRequest, result CapabilityResolution, rejections []CandidateRejection, start time.Time) *RoutingTrace {
	trace := r.buildTrace(request, result, start)
	trace.Rejections = rejections
	if result.Decision == RoutingResolved {
		trace.WinnerProviderID = result.Provider.ID
		trace.WinnerInstanceID = result.ProviderInstance.ID
	}
	return trace
}

func buildExecutionTarget(def *CapabilityProviderDefinition, inst *CapabilityProviderInstance) InvocationExecutionTarget {
	if def == nil || inst == nil {
		return InvocationExecutionTarget{}
	}
	return InvocationExecutionTarget{
		Placement: string(inst.Placement),

		UserID:    inst.UserID,
		DeviceID:  inst.DeviceID,
		RuntimeID: inst.RuntimeID,

		ProviderID:         string(inst.ProviderID),
		ProviderInstanceID: string(inst.ID),

		ExtensionID: def.ExtensionID,
		ModuleID:    def.ModuleID,
	}
}

var (
	ErrCapabilityNotRegistered        = errors.New("capability: not registered")
	ErrNoAvailableProvider            = errors.New("capability: no available provider")
	ErrCapabilityPlacementUnavailable = errors.New("capability: placement unavailable")
)
