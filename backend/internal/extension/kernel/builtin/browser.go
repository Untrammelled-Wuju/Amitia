package builtin

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

const (
	BrowserExtensionID  = domain.ExtensionID("com.amitia.builtin.browser")
	BrowserModuleID     = domain.ModuleID("browser-runtime")
	BrowserCapabilityID = capability.CapabilityID("browser.control")
	BrowserProviderID   = capability.ProviderID("com.amitia.builtin.browser.provider")
)

func BuildBrowserExtension(version string) Definition {
	ver, err := domain.ParseVersion(version)
	if err != nil {
		ver = domain.SemanticVersion{Major: 0, Minor: 1, Patch: 0}
	}

	extDef := domain.ExtensionDefinition{
		ID:      BrowserExtensionID,
		Name:    domain.LocalizedText{Default: "Browser"},
		Description: domain.LocalizedText{
			Default: "Provides browser control capabilities including navigation, interaction, and page inspection.",
		},
		Version:         ver,
		ManifestVersion: 1,
		Domain:          domain.ExtensionDomainGeneral,
		Placement:       domain.ExtensionPlacementDevice,
		Publisher: domain.PublisherReference{
			PublisherID: "com.amitia",
			DisplayName: "Amitia",
			TrustLevel:  "system",
		},
		Package: domain.PackageReference{
			PackageID:       "builtin-browser",
			ManifestVersion: 1,
		},
		Modules: []domain.ModuleDefinition{
			{
				ID:          BrowserModuleID,
				ExtensionID: BrowserExtensionID,
				Name:        domain.LocalizedText{Default: "Browser Runtime"},
				Description: domain.LocalizedText{
					Default: "Built-in module providing browser control capability.",
				},
				Type:    domain.ModuleTypeBuiltin,
				Version: version,
				Runtime: &domain.RuntimeDefinition{
					Type:        domain.RuntimeTypeBuiltin,
					EntryPoint:  "browser.control",
					WorkerCount: 1,
				},
				Contributions: buildBrowserContributions(BrowserExtensionID, BrowserModuleID),
				ProvidedCapabilities: []domain.ProvidedCapability{
					{
						ID:      string(BrowserCapabilityID),
						Version: version,
					},
				},
				Provider: &domain.ProviderMetadata{
					ID:       string(BrowserProviderID),
					Priority: 80,
					Labels: map[string]string{
						"component": "browser",
					},
				},
				Placement: domain.ModulePlacementDevice,
				DeviceRequirements: &domain.DeviceRequirements{
					Platforms: []string{"windows", "linux", "darwin"},
				},
				Compatibility: domain.ModuleCompatibility{
					Platforms: []string{"windows", "linux", "darwin"},
				},
				Policies: domain.ModulePolicies{
					NetworkAccess:    true,
					FileSystemAccess: false,
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

func buildBrowserContributions(extID domain.ExtensionID, modID domain.ModuleID) []domain.ContributionDefinition {
	contributionKinds := []struct {
		id          string
		name        string
		description string
	}{
		{"browser_session_create", "Browser Session Create", "Create a new browser session"},
		{"browser_session_close", "Browser Session Close", "Close a browser session"},
		{"browser_session_get", "Browser Session Get", "Get browser session info"},
		{"browser_session_list", "Browser Session List", "List browser sessions"},
		{"browser_tab_create", "Browser Tab Create", "Create a new browser tab"},
		{"browser_tab_close", "Browser Tab Close", "Close a browser tab"},
		{"browser_tab_get", "Browser Tab Get", "Get browser tab info"},
		{"browser_tab_list", "Browser Tab List", "List browser tabs"},
		{"browser_tab_activate", "Browser Tab Activate", "Activate a browser tab"},
		{"browser_navigate", "Browser Navigate", "Navigate to a URL"},
		{"browser_navigate_reload", "Browser Reload", "Reload current page"},
		{"browser_navigate_back", "Browser Back", "Go back in browser history"},
		{"browser_navigate_forward", "Browser Forward", "Go forward in browser history"},
		{"browser_navigate_stop", "Browser Stop", "Stop loading current page"},
		{"browser_dom_snapshot", "Browser DOM Snapshot", "Get DOM snapshot of the page"},
		{"browser_dom_find", "Browser Find Element", "Find an element on the page"},
		{"browser_dom_scroll_to_element", "Browser Scroll To Element", "Scroll to an element on the page"},
		{"browser_interact_click", "Browser Click", "Click an element"},
		{"browser_interact_input", "Browser Input", "Input text into an element"},
		{"browser_interact_select", "Browser Select", "Select a value on an element"},
		{"browser_interact_hover", "Browser Hover", "Hover over an element"},
		{"browser_interact_scroll", "Browser Scroll", "Scroll the page"},
		{"browser_resource_download", "Browser Download", "Download a resource"},
		{"browser_resource_upload", "Browser Upload", "Upload a resource"},
		{"browser_resource_screenshot", "Browser Screenshot", "Take a screenshot of the page"},
	}

	contributions := make([]domain.ContributionDefinition, 0, len(contributionKinds))
	for _, k := range contributionKinds {
		contributions = append(contributions, domain.ContributionDefinition{
			ID:          domain.ContributionID(k.id),
			ModuleID:    modID,
			ExtensionID: extID,
			Kind:        domain.ContributionKindTool,
			Name:        domain.LocalizedText{Default: k.name},
			Description: domain.LocalizedText{Default: k.description},
		})
	}
	return contributions
}
