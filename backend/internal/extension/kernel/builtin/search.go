package builtin

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

const (
	SearchExtensionID  = domain.ExtensionID("com.amitia.builtin.search")
	SearchModuleID     = domain.ModuleID("search-runtime")
	SearchCapabilityID = capability.CapabilityID("search.web")
	SearchProviderID   = capability.ProviderID("com.amitia.builtin.search.provider")
)

func buildSearchInputSchema() string {
	return `{"type":"object","additionalProperties":false,"required":["query"],"properties":{"query":{"type":"string","minLength":1,"maxLength":2048},"kind":{"type":"string","enum":["web","news","academic","code","image","video","places","product"]},"limit":{"type":"integer","minimum":1,"maximum":20},"offset":{"type":"integer","minimum":0,"maximum":100},"language":{"type":"string"},"country":{"type":"string"},"safeSearch":{"type":"string","enum":["off","moderate","strict"]},"domains":{"type":"array","maxItems":16,"items":{"type":"string","maxLength":253}},"specialized":{"type":"object"}}}`
}

func buildSearchOutputSchema() string {
	return `{"type":"object","additionalProperties":false,"required":["query","provider","results","returned","retrievedAt"],"properties":{"query":{"type":"string"},"kind":{"type":"string"},"provider":{"type":"string"},"results":{"type":"array","items":{"type":"object"}},"returned":{"type":"integer"},"hasMore":{"type":"boolean"},"retrievedAt":{"type":"string","format":"date-time"},"citations":{"type":"array","items":{"type":"object"}}}}`
}

func buildDeepSearchInputSchema() string {
	return `{"type":"object","additionalProperties":false,"required":["query"],"properties":{"query":{"type":"string","minLength":1,"maxLength":2048},"maxDepth":{"type":"integer","minimum":1,"maximum":5},"maxResults":{"type":"integer","minimum":1,"maximum":50},"language":{"type":"string"},"country":{"type":"string"},"timeoutMs":{"type":"integer","minimum":5000,"maximum":120000}}}`
}

func buildDeepSearchOutputSchema() string {
	return `{"type":"object","additionalProperties":false,"required":["dossierId","query","status"],"properties":{"dossierId":{"type":"string"},"query":{"type":"string"},"status":{"type":"string","enum":["completed","partial","failed"]},"results":{"type":"array","items":{"type":"object"}},"sources":{"type":"array","items":{"type":"object"}},"generatedAt":{"type":"string","format":"date-time"}}}`
}

func BuildSearchExtension(version string) Definition {
	ver, err := domain.ParseVersion(version)
	if err != nil {
		ver = domain.SemanticVersion{Major: 0, Minor: 1, Patch: 0}
	}

	extDef := domain.ExtensionDefinition{
		ID:   SearchExtensionID,
		Name: domain.LocalizedText{Default: "Search"},
		Description: domain.LocalizedText{
			Default: "Provides web search capability using configured search providers.",
		},
		Version:         ver,
		ManifestVersion: 1,
		Domain:          domain.ExtensionDomainGeneral,
		Placement:       domain.ExtensionPlacementCloud,
		Publisher: domain.PublisherReference{
			PublisherID: "com.amitia",
			DisplayName: "Amitia",
			TrustLevel:  "system",
		},
		Package: domain.PackageReference{
			PackageID:       "builtin-search",
			ManifestVersion: 1,
		},
		Modules: []domain.ModuleDefinition{
			{
				ID:          SearchModuleID,
				ExtensionID: SearchExtensionID,
				Name:        domain.LocalizedText{Default: "Search Runtime"},
				Description: domain.LocalizedText{
					Default: "Built-in module providing web search capability.",
				},
				Type:    domain.ModuleTypeBuiltin,
				Version: version,
				Runtime: &domain.RuntimeDefinition{
					Type:        domain.RuntimeTypeBuiltin,
					EntryPoint:  "search.general",
					WorkerCount: 2,
				},
				Contributions: []domain.ContributionDefinition{
					{
						ID:          "web_search",
						ModuleID:    SearchModuleID,
						ExtensionID: SearchExtensionID,
						Kind:        domain.ContributionKindTool,
						Name:        domain.LocalizedText{Default: "Web Search"},
						Description: domain.LocalizedText{
							Default: "Search the web using the configured provider.",
						},
						Definition: map[string]any{
							"capabilityId": string(SearchCapabilityID),
							"modelName":    "web_search",
							"inputSchema":  buildSearchInputSchema(),
							"outputSchema": buildSearchOutputSchema(),
							"riskLevel":    "medium",
							"sideEffect":   "external",
							"permissions": []map[string]any{
								{"capability": "network.request", "description": "Sends query to external search provider"},
							},
							"timeoutMs":      int64(30000),
							"idempotent":     true,
							"retryable":      true,
							"hasSideEffects": true,
							"executionPolicy": map[string]any{
								"timeout":    "30s",
								"idempotent": true,
								"retryPolicy": map[string]any{
									"maxRetries":  1,
									"backoffBase": "1s",
								},
							},
							"resultPolicy": map[string]any{
								"sanitizeError":  true,
								"maxOutputBytes": 131072,
								"streaming": map[string]any{
									"enabled": false,
								},
							},
							"runtime": map[string]any{
								"runtimeType": "search",
								"runtimeId":   "default",
								"handlerName": "search.general",
							},
						},
						Metadata: map[string]any{
							"system.builtin": true,
						},
					},
				},
				ProvidedCapabilities: []domain.ProvidedCapability{
					{
						ID:      string(SearchCapabilityID),
						Version: version,
					},
				},
				Provider: &domain.ProviderMetadata{
					ID:       string(SearchProviderID),
					Priority: 100,
					Labels: map[string]string{
						"component": "search",
					},
				},
				Compatibility: domain.ModuleCompatibility{
					Platforms: []string{"windows", "linux", "darwin"},
				},
				Policies: domain.ModulePolicies{
					NetworkAccess: true,
				},
			},
		},
		Compatibility: domain.ExtensionCompatibility{
			Platforms: []string{"windows", "linux", "darwin"},
		},
		Policies: domain.ExtensionPolicies{
			NetworkAccess: true,
		},
	}

	return Definition{
		Extension:         extDef,
		SystemManaged:     true,
		Required:          true,
		DisableAllowed:    false,
		BootstrapRevision: 1,
	}
}

const (
	DeepSearchExtensionID  = domain.ExtensionID("com.amitia.builtin.deep-search")
	DeepSearchModuleID     = domain.ModuleID("deep-search-runtime")
	DeepSearchCapabilityID = capability.CapabilityID("search.deep")
	DeepSearchProviderID   = capability.ProviderID("com.amitia.builtin.deep-search.provider")
)

func BuildDeepSearchExtension(version string) Definition {
	ver, err := domain.ParseVersion(version)
	if err != nil {
		ver = domain.SemanticVersion{Major: 0, Minor: 1, Patch: 0}
	}

	extDef := domain.ExtensionDefinition{
		ID:   DeepSearchExtensionID,
		Name: domain.LocalizedText{Default: "Deep Search"},
		Description: domain.LocalizedText{
			Default: "Provides multi-round deep web search that aggregates, deduplicates, and ranks results.",
		},
		Version:         ver,
		ManifestVersion: 1,
		Domain:          domain.ExtensionDomainGeneral,
		Placement:       domain.ExtensionPlacementCloud,
		Publisher: domain.PublisherReference{
			PublisherID: "com.amitia",
			DisplayName: "Amitia",
			TrustLevel:  "system",
		},
		Package: domain.PackageReference{
			PackageID:       "builtin-deep-search",
			ManifestVersion: 1,
		},
		Modules: []domain.ModuleDefinition{
			{
				ID:          DeepSearchModuleID,
				ExtensionID: DeepSearchExtensionID,
				Name:        domain.LocalizedText{Default: "Deep Search Runtime"},
				Description: domain.LocalizedText{
					Default: "Built-in module providing deep search capability via task runtime.",
				},
				Type:    domain.ModuleTypeBuiltin,
				Version: version,
				Runtime: &domain.RuntimeDefinition{
					Type:        domain.RuntimeTypeTask,
					EntryPoint:  "deep_search",
					WorkerCount: 1,
				},
				Contributions: []domain.ContributionDefinition{
					{
						ID:          "deep_search",
						ModuleID:    DeepSearchModuleID,
						ExtensionID: DeepSearchExtensionID,
						Kind:        domain.ContributionKindTool,
						Name:        domain.LocalizedText{Default: "Deep Search"},
						Description: domain.LocalizedText{
							Default: "Run a multi-round web search that aggregates, deduplicates, and ranks results into a research dossier.",
						},
						Definition: map[string]any{
							"capabilityId": string(DeepSearchCapabilityID),
							"modelName":    "deep_search",
							"inputSchema":  buildDeepSearchInputSchema(),
							"outputSchema": buildDeepSearchOutputSchema(),
							"riskLevel":    "medium",
							"sideEffect":   "external",
							"permissions": []map[string]any{
								{"capability": "network.request", "description": "Sends queries to external search providers"},
							},
							"timeoutMs":  int64(120000),
							"idempotent": true,
							"retryable":  true,
							"runtime": map[string]any{
								"runtimeType": "task",
								"runtimeId":   "default",
								"handlerName": "deep_search",
							},
						},
						Metadata: map[string]any{
							"system.builtin": true,
						},
					},
				},
				ProvidedCapabilities: []domain.ProvidedCapability{
					{
						ID:      string(DeepSearchCapabilityID),
						Version: version,
					},
				},
				Provider: &domain.ProviderMetadata{
					ID:       string(DeepSearchProviderID),
					Priority: 90,
					Labels: map[string]string{
						"component": "deep-search",
					},
				},
				Compatibility: domain.ModuleCompatibility{
					Platforms: []string{"windows", "linux", "darwin"},
				},
				Policies: domain.ModulePolicies{
					NetworkAccess: true,
				},
			},
		},
		Compatibility: domain.ExtensionCompatibility{
			Platforms: []string{"windows", "linux", "darwin"},
		},
		Policies: domain.ExtensionPolicies{
			NetworkAccess: true,
		},
	}

	return Definition{
		Extension:         extDef,
		SystemManaged:     true,
		Required:          false,
		DisableAllowed:    true,
		BootstrapRevision: 1,
	}
}
