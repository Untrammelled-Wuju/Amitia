package builtin

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

const (
	SearchExtensionID    = domain.ExtensionID("com.amitia.builtin.search")
	SearchModuleID       = domain.ModuleID("search-runtime")
	SearchCapabilityID   = capability.CapabilityID("search.web")
	SearchProviderID     = capability.ProviderID("com.amitia.builtin.search.provider")
)

func BuildSearchExtension(version string) Definition {
	ver, err := domain.ParseVersion(version)
	if err != nil {
		ver = domain.SemanticVersion{Major: 0, Minor: 1, Patch: 0}
	}

	extDef := domain.ExtensionDefinition{
		ID:      SearchExtensionID,
		Name:    domain.LocalizedText{Default: "Search"},
		Description: domain.LocalizedText{
			Default: "Provides web search capability using configured search providers.",
		},
		Version:         ver,
		ManifestVersion: 1,
		Domain:          domain.ExtensionDomainCore,
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
		Extension:        extDef,
		SystemManaged:    true,
		Required:         true,
		DisableAllowed:   false,
		BootstrapRevision: 1,
	}
}

const (
	DeepSearchExtensionID    = domain.ExtensionID("com.amitia.builtin.deep-search")
	DeepSearchModuleID       = domain.ModuleID("deep-search-runtime")
	DeepSearchCapabilityID   = capability.CapabilityID("search.deep")
	DeepSearchProviderID     = capability.ProviderID("com.amitia.builtin.deep-search.provider")
)

func BuildDeepSearchExtension(version string) Definition {
	ver, err := domain.ParseVersion(version)
	if err != nil {
		ver = domain.SemanticVersion{Major: 0, Minor: 1, Patch: 0}
	}

	extDef := domain.ExtensionDefinition{
		ID:      DeepSearchExtensionID,
		Name:    domain.LocalizedText{Default: "Deep Search"},
		Description: domain.LocalizedText{
			Default: "Provides multi-round deep web search that aggregates, deduplicates, and ranks results.",
		},
		Version:         ver,
		ManifestVersion: 1,
		Domain:          domain.ExtensionDomainCore,
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
		Extension:        extDef,
		SystemManaged:    true,
		Required:         false,
		DisableAllowed:   true,
		BootstrapRevision: 1,
	}
}
