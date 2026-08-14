package capability

import "strings"

type ProviderAvailabilityState string

const (
	ProviderAvailabilityUnknown     ProviderAvailabilityState = "unknown"
	ProviderAvailabilityAvailable   ProviderAvailabilityState = "available"
	ProviderAvailabilityUnavailable ProviderAvailabilityState = "unavailable"
	ProviderAvailabilityDraining    ProviderAvailabilityState = "draining"
)

func (s ProviderAvailabilityState) String() string {
	return string(s)
}

func (s ProviderAvailabilityState) IsValid() bool {
	switch s {
	case ProviderAvailabilityUnknown, ProviderAvailabilityAvailable, ProviderAvailabilityUnavailable, ProviderAvailabilityDraining:
		return true
	}
	return false
}

func (s ProviderAvailabilityState) IsAvailable() bool {
	return s == ProviderAvailabilityAvailable
}

func ParseProviderAvailabilityState(raw string) ProviderAvailabilityState {
	return ProviderAvailabilityState(strings.TrimSpace(raw))
}
