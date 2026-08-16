package capability

import (
	"time"
)

type RoutingDecision string

const (
	RoutingResolved             RoutingDecision = "resolved"
	RoutingNoProvider           RoutingDecision = "no_provider"
	RoutingNoHealthyInstance    RoutingDecision = "no_healthy_instance"
	RoutingPlacementUnavailable RoutingDecision = "placement_unavailable"
	RoutingPlatformMismatch     RoutingDecision = "platform_mismatch"
	RoutingRuntimeUnavailable   RoutingDecision = "runtime_unavailable"
	RoutingDeviceOffline        RoutingDecision = "device_offline"
	RoutingTransportUnresolved  RoutingDecision = "transport_unresolved"
)

type ProviderRoutingMode string

const (
	RoutingModeLegacy            ProviderRoutingMode = "legacy"
	RoutingModeProviderPreferred ProviderRoutingMode = "provider_preferred"
	RoutingModeProviderRequired  ProviderRoutingMode = "provider_required"
)

type RoutingPolicy struct {
	AllowCore        bool
	AllowDevice      bool
	PreferLocalCore  bool
	RequireHealthy   bool
	RequireAvailable bool
}

func ProductionRoutingPolicy() RoutingPolicy {
	return RoutingPolicy{
		AllowCore:        true,
		AllowDevice:      false,
		PreferLocalCore:  true,
		RequireHealthy:   true,
		RequireAvailable: true,
	}
}

type CandidateRejectionReason string

const (
	RejectionCapabilityMismatch        CandidateRejectionReason = "capability_mismatch"
	RejectionPlacementNotAllowed       CandidateRejectionReason = "placement_not_allowed"
	RejectionProviderUnavailable       CandidateRejectionReason = "provider_unavailable"
	RejectionProviderUnhealthy         CandidateRejectionReason = "provider_unhealthy"
	RejectionPlatformMismatch          CandidateRejectionReason = "platform_mismatch"
	RejectionRuntimeTypeMismatch       CandidateRejectionReason = "runtime_type_mismatch"
	RejectionRuntimeAdapterUnavailable CandidateRejectionReason = "runtime_adapter_unavailable"
	RejectionDeviceOffline             CandidateRejectionReason = "device_offline"
	RejectionTransportUnresolved       CandidateRejectionReason = "transport_unresolved"
)

type CandidateRejection struct {
	ProviderID         ProviderID
	ProviderInstanceID ProviderInstanceID
	Reason             CandidateRejectionReason
}

type RoutingTrace struct {
	RequestID        string
	ToolID           string
	CapabilityID     CapabilityID
	CandidateCount   int
	RejectedCount    int
	WinnerProviderID ProviderID
	WinnerInstanceID ProviderInstanceID
	Decision         RoutingDecision
	Reason           string
	Duration         time.Duration
	Rejections       []CandidateRejection
}
