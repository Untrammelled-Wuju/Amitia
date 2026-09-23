package search

import (
	"fmt"
	"sort"
	"strings"
)

type CostModel struct {
	Metered bool   `json:"metered"`
	Unit    string `json:"unit,omitempty"`
}

type ProviderManifest struct {
	ID              string    `json:"id"`
	Capabilities    []string  `json:"capabilities"`
	RequiresSecrets []string  `json:"requires_secrets,omitempty"`
	NetworkScopes   []string  `json:"network_scopes,omitempty"`
	CostModel       CostModel `json:"cost_model"`
}

func normalizeProviderManifest(instanceID string, provider Provider, manifest ProviderManifest) (ProviderManifest, error) {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" || provider == nil {
		return ProviderManifest{}, fmt.Errorf("search provider registration requires instance id and provider")
	}
	if strings.TrimSpace(manifest.ID) == "" {
		manifest.ID = instanceID
	}
	if manifest.ID != instanceID {
		return ProviderManifest{}, fmt.Errorf("search provider manifest id %q does not match instance id %q", manifest.ID, instanceID)
	}
	manifest.Capabilities = dedupeManifestValues(manifest.Capabilities)
	if len(manifest.Capabilities) == 0 {
		manifest.Capabilities = capabilityManifestValues(provider.Capabilities())
	}
	manifest.RequiresSecrets = dedupeManifestValues(manifest.RequiresSecrets)
	manifest.NetworkScopes = dedupeManifestValues(manifest.NetworkScopes)
	manifest.CostModel.Unit = strings.ToLower(strings.TrimSpace(manifest.CostModel.Unit))
	return manifest, nil
}

func dedupeManifestValues(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func capabilityManifestValues(caps ProviderCapabilities) []string {
	values := make([]string, 0, len(caps.SearchKinds)+6)
	for _, kind := range caps.SearchKinds {
		values = append(values, "search.kind."+string(kind))
	}
	if caps.GeneralWeb {
		values = append(values, "search.web")
	}
	if caps.LanguageFilter {
		values = append(values, "filter.language")
	}
	if caps.TimeRangeFilter {
		values = append(values, "filter.time_range")
	}
	if caps.DomainFilter {
		values = append(values, "filter.domain")
	}
	if caps.ExcludeDomainFilter {
		values = append(values, "filter.exclude_domain")
	}
	if caps.SafeSearch {
		values = append(values, "filter.safe_search")
	}
	return dedupeManifestValues(values)
}
