package search

import "context"

type HealthReport struct {
	Enabled         bool                      `json:"enabled"`
	DefaultProvider string                    `json:"defaultProvider"`
	Providers       map[string]ProviderHealth `json:"providers"`
	Circuits        map[string]CircuitStatus  `json:"circuits,omitempty"`
}

func (s *Service) BuildHealthReport(ctx context.Context) HealthReport {
	report := HealthReport{
		Enabled:         s.config.Enabled,
		DefaultProvider: s.providers.DefaultID(),
		Providers:       make(map[string]ProviderHealth),
		Circuits:        s.CircuitStatuses(),
	}
	if !s.config.Enabled {
		report.Providers["_global"] = ProviderHealthDisabled
		return report
	}
	for id, p := range s.providers.All() {
		report.Providers[id] = s.effectiveProviderHealth(id, p.Health(ctx))
	}
	return report
}
