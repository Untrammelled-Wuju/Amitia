package plugincontract

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrRouteNoBuiltin   = errors.New("plugincontract: core implementation missing")
	ErrRouteUnavailable = errors.New("plugincontract: route provider unavailable")
)

type RouteFailureClass string

const (
	RouteFailureNone        RouteFailureClass = "NONE"
	RouteFailureNoBuiltin   RouteFailureClass = "NO_BUILTIN"
	RouteFailureUnavailable RouteFailureClass = "UNAVAILABLE"
)

type CoreImplementationIndex struct {
	providers map[string]bool
}

func NewCoreImplementationIndex(serviceIDs ...string) CoreImplementationIndex {
	providers := make(map[string]bool, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		trimmed := strings.TrimSpace(serviceID)
		if trimmed == "" {
			continue
		}
		if !isCoreProvider(trimmed) {
			trimmed = CoreProviderPluginID + "/" + CoreProviderModuleID + "/" + trimmed
		}
		providers[trimmed] = true
	}
	return CoreImplementationIndex{providers: providers}
}

func (i CoreImplementationIndex) Has(providerID string) bool {
	if len(i.providers) == 0 {
		return false
	}
	return i.providers[strings.TrimSpace(providerID)]
}

func (i CoreImplementationIndex) Len() int { return len(i.providers) }

type RouteFailureError struct {
	Class           RouteFailureClass
	ServiceID       string
	ProviderID      string
	Reason          string
	FallbackBlocked bool
}

func (e *RouteFailureError) Error() string {
	return fmt.Sprintf(
		"plugincontract: route failure %s for service %q provider %q: %s (fallbackBlocked=%v)",
		e.Class, e.ServiceID, e.ProviderID, e.Reason, e.FallbackBlocked,
	)
}

func (e *RouteFailureError) Unwrap() error {
	switch e.Class {
	case RouteFailureNoBuiltin:
		return ErrRouteNoBuiltin
	case RouteFailureUnavailable:
		return ErrRouteUnavailable
	default:
		return nil
	}
}

type ProviderHealth func(providerID string) bool

func SelectRouteChecked(
	candidates ProviderCandidates,
	policy RoutePolicy,
	index CoreImplementationIndex,
	health ProviderHealth,
) (RouteDecision, error) {
	decision, err := SelectRoute(candidates, policy)
	if err != nil {
		return decision, err
	}
	return CheckRouteAvailability(decision, index, health)
}

func CheckRouteAvailability(
	decision RouteDecision,
	index CoreImplementationIndex,
	health ProviderHealth,
) (RouteDecision, error) {
	providerID := strings.TrimSpace(decision.Selected.ProviderID)
	if providerID == "" {
		return decision, &RouteFailureError{
			Class:     RouteFailureNoBuiltin,
			ServiceID: decision.ServiceID,
			Reason:    "route decision carries no selected provider",
		}
	}
	if isCoreProvider(providerID) {
		if !index.Has(providerID) {
			return decision, &RouteFailureError{
				Class:           RouteFailureNoBuiltin,
				ServiceID:       decision.ServiceID,
				ProviderID:      providerID,
				Reason:          "no core implementation is registered for this provider",
				FallbackBlocked: true,
			}
		}
		return decision, nil
	}
	if health != nil && !health(providerID) {
		return decision, &RouteFailureError{
			Class:      RouteFailureUnavailable,
			ServiceID:  decision.ServiceID,
			ProviderID: providerID,
			Reason:     "provider health probe reported unavailable",
		}
	}
	return decision, nil
}
