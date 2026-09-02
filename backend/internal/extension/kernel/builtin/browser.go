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
		ID:   BrowserExtensionID,
		Name: domain.LocalizedText{Default: "Browser"},
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

type browserToolSpec struct {
	id           string
	name         string
	description  string
	modelName    string
	handlerName  string
	inputSchema  string
	outputSchema string
	riskLevel    string
	sideEffect   string
	idempotent   bool
	retryable    bool
	timeoutMs    int64
	permissions  []map[string]any
}

func buildBrowserContributions(extID domain.ExtensionID, modID domain.ModuleID) []domain.ContributionDefinition {
	specs := []browserToolSpec{
		{
			id:           "browser_session_create",
			name:         "Browser Session Create",
			description:  "Create a new browser session",
			modelName:    "browser_session_create",
			handlerName:  "browser.session.create",
			inputSchema:  `{"type":"object","additionalProperties":false,"properties":{}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"sessionId":{"type":"string"},"state":{"type":"string"}}}`,
			riskLevel:    "medium",
			sideEffect:   "system",
			idempotent:   false,
			retryable:    false,
			timeoutMs:    30000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.session.create", "description": "Create browser sessions"},
			},
		},
		{
			id:           "browser_session_close",
			name:         "Browser Session Close",
			description:  "Close a browser session",
			modelName:    "browser_session_close",
			handlerName:  "browser.session.close",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId"],"properties":{"sessionId":{"type":"string"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"ok":{"type":"boolean"}}}`,
			riskLevel:    "medium",
			sideEffect:   "system",
			idempotent:   true,
			retryable:    true,
			timeoutMs:    15000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.session.manage", "description": "Manage browser sessions"},
			},
		},
		{
			id:           "browser_session_get",
			name:         "Browser Session Get",
			description:  "Get browser session info",
			modelName:    "browser_session_get",
			handlerName:  "browser.session.get",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId"],"properties":{"sessionId":{"type":"string"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"sessionId":{"type":"string"},"state":{"type":"string"},"url":{"type":"string"}}}`,
			riskLevel:    "low",
			sideEffect:   "read_only",
			idempotent:   true,
			retryable:    true,
			timeoutMs:    5000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.session.manage", "description": "Manage browser sessions"},
			},
		},
		{
			id:           "browser_session_list",
			name:         "Browser Session List",
			description:  "List browser sessions",
			modelName:    "browser_session_list",
			handlerName:  "browser.session.list",
			inputSchema:  `{"type":"object","additionalProperties":false,"properties":{}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"sessions":{"type":"array","items":{"type":"object"}}}}`,
			riskLevel:    "low",
			sideEffect:   "read_only",
			idempotent:   true,
			retryable:    true,
			timeoutMs:    5000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.session.manage", "description": "Manage browser sessions"},
			},
		},
		{
			id:           "browser_tab_create",
			name:         "Browser Tab Create",
			description:  "Create a new browser tab",
			modelName:    "browser_tab_create",
			handlerName:  "browser.tab.create",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId"],"properties":{"sessionId":{"type":"string"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"tabId":{"type":"string"},"sessionId":{"type":"string"},"url":{"type":"string"}}}`,
			riskLevel:    "medium",
			sideEffect:   "system",
			idempotent:   false,
			retryable:    false,
			timeoutMs:    15000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.tab.manage", "description": "Manage browser tabs"},
			},
		},
		{
			id:           "browser_tab_close",
			name:         "Browser Tab Close",
			description:  "Close a browser tab",
			modelName:    "browser_tab_close",
			handlerName:  "browser.tab.close",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId","tabId"],"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"ok":{"type":"boolean"}}}`,
			riskLevel:    "medium",
			sideEffect:   "system",
			idempotent:   true,
			retryable:    true,
			timeoutMs:    15000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.tab.manage", "description": "Manage browser tabs"},
			},
		},
		{
			id:           "browser_tab_get",
			name:         "Browser Tab Get",
			description:  "Get browser tab info",
			modelName:    "browser_tab_get",
			handlerName:  "browser.tab.get",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId","tabId"],"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"tabId":{"type":"string"},"sessionId":{"type":"string"},"url":{"type":"string"},"title":{"type":"string"}}}`,
			riskLevel:    "low",
			sideEffect:   "read_only",
			idempotent:   true,
			retryable:    true,
			timeoutMs:    5000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.tab.manage", "description": "Manage browser tabs"},
			},
		},
		{
			id:           "browser_tab_list",
			name:         "Browser Tab List",
			description:  "List browser tabs",
			modelName:    "browser_tab_list",
			handlerName:  "browser.tab.list",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId"],"properties":{"sessionId":{"type":"string"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"tabs":{"type":"array","items":{"type":"object"}}}}`,
			riskLevel:    "low",
			sideEffect:   "read_only",
			idempotent:   true,
			retryable:    true,
			timeoutMs:    5000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.tab.manage", "description": "Manage browser tabs"},
			},
		},
		{
			id:           "browser_tab_activate",
			name:         "Browser Tab Activate",
			description:  "Activate a browser tab",
			modelName:    "browser_tab_activate",
			handlerName:  "browser.tab.activate",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId","tabId"],"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"ok":{"type":"boolean"}}}`,
			riskLevel:    "low",
			sideEffect:   "write",
			idempotent:   true,
			retryable:    true,
			timeoutMs:    5000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.tab.manage", "description": "Manage browser tabs"},
			},
		},
		{
			id:           "browser_navigate",
			name:         "Browser Navigate",
			description:  "Navigate to a URL",
			modelName:    "browser_navigate",
			handlerName:  "browser.navigate",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId","tabId","url"],"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"},"url":{"type":"string"},"waitUntil":{"type":"string"},"timeoutMs":{"type":"integer"},"referer":{"type":"string"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"},"requestedUrl":{"type":"string"},"finalUrl":{"type":"string"},"title":{"type":"string"},"loaded":{"type":"boolean"},"timedOut":{"type":"boolean"}}}`,
			riskLevel:    "high",
			sideEffect:   "external",
			idempotent:   false,
			retryable:    false,
			timeoutMs:    60000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.navigate", "description": "Navigate browser pages"},
			},
		},
		{
			id:           "browser_navigate_reload",
			name:         "Browser Reload",
			description:  "Reload current page",
			modelName:    "browser_navigate_reload",
			handlerName:  "browser.navigate.reload",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId","tabId"],"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"},"waitUntil":{"type":"string"},"timeoutMs":{"type":"integer"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"},"finalUrl":{"type":"string"},"loaded":{"type":"boolean"}}}`,
			riskLevel:    "medium",
			sideEffect:   "external",
			idempotent:   false,
			retryable:    false,
			timeoutMs:    60000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.navigate", "description": "Navigate browser pages"},
			},
		},
		{
			id:           "browser_navigate_back",
			name:         "Browser Back",
			description:  "Go back in browser history",
			modelName:    "browser_navigate_back",
			handlerName:  "browser.navigate.back",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId","tabId"],"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"},"finalUrl":{"type":"string"}}}`,
			riskLevel:    "medium",
			sideEffect:   "external",
			idempotent:   false,
			retryable:    false,
			timeoutMs:    30000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.navigate", "description": "Navigate browser pages"},
			},
		},
		{
			id:           "browser_navigate_forward",
			name:         "Browser Forward",
			description:  "Go forward in browser history",
			modelName:    "browser_navigate_forward",
			handlerName:  "browser.navigate.forward",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId","tabId"],"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"},"finalUrl":{"type":"string"}}}`,
			riskLevel:    "medium",
			sideEffect:   "external",
			idempotent:   false,
			retryable:    false,
			timeoutMs:    30000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.navigate", "description": "Navigate browser pages"},
			},
		},
		{
			id:           "browser_navigate_stop",
			name:         "Browser Stop",
			description:  "Stop loading current page",
			modelName:    "browser_navigate_stop",
			handlerName:  "browser.navigate.stop",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId","tabId"],"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"ok":{"type":"boolean"}}}`,
			riskLevel:    "medium",
			sideEffect:   "write",
			idempotent:   true,
			retryable:    true,
			timeoutMs:    15000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.navigate", "description": "Navigate browser pages"},
			},
		},
		{
			id:           "browser_dom_snapshot",
			name:         "Browser DOM Snapshot",
			description:  "Get DOM snapshot of the page",
			modelName:    "browser_dom_snapshot",
			handlerName:  "browser.dom.snapshot",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId","tabId"],"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"},"maxDepth":{"type":"integer"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"},"url":{"type":"string"},"content":{"type":"string"},"truncated":{"type":"boolean"},"nodeCount":{"type":"integer"}}}`,
			riskLevel:    "low",
			sideEffect:   "read_only",
			idempotent:   true,
			retryable:    true,
			timeoutMs:    30000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.dom.read", "description": "Read browser DOM"},
			},
		},
		{
			id:           "browser_dom_find",
			name:         "Browser Find Element",
			description:  "Find an element on the page",
			modelName:    "browser_dom_find",
			handlerName:  "browser.dom.find",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId","tabId","selector"],"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"},"selector":{"type":"string"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"},"stableId":{"type":"string"},"selector":{"type":"string"}}}`,
			riskLevel:    "low",
			sideEffect:   "read_only",
			idempotent:   true,
			retryable:    true,
			timeoutMs:    15000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.dom.query", "description": "Query browser DOM elements"},
			},
		},
		{
			id:           "browser_dom_scroll_to_element",
			name:         "Browser Scroll To Element",
			description:  "Scroll to an element on the page",
			modelName:    "browser_dom_scroll_to_element",
			handlerName:  "browser.dom.scrollToElement",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId","tabId","element"],"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"},"element":{"type":"object"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"ok":{"type":"boolean"}}}`,
			riskLevel:    "low",
			sideEffect:   "write",
			idempotent:   true,
			retryable:    true,
			timeoutMs:    15000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.dom.query", "description": "Query browser DOM elements"},
			},
		},
		{
			id:           "browser_interact_click",
			name:         "Browser Click",
			description:  "Click an element",
			modelName:    "browser_interact_click",
			handlerName:  "browser.interact.click",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId","tabId","element"],"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"},"element":{"type":"object"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"success":{"type":"boolean"},"action":{"type":"string"},"verified":{"type":"boolean"}}}`,
			riskLevel:    "high",
			sideEffect:   "write",
			idempotent:   false,
			retryable:    false,
			timeoutMs:    30000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.interact", "description": "Interact with browser elements"},
			},
		},
		{
			id:           "browser_interact_input",
			name:         "Browser Input",
			description:  "Input text into an element",
			modelName:    "browser_interact_input",
			handlerName:  "browser.interact.input",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId","tabId","element","inputText"],"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"},"element":{"type":"object"},"inputText":{"type":"string"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"success":{"type":"boolean"},"action":{"type":"string"},"verified":{"type":"boolean"}}}`,
			riskLevel:    "high",
			sideEffect:   "write",
			idempotent:   false,
			retryable:    false,
			timeoutMs:    30000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.interact", "description": "Interact with browser elements"},
			},
		},
		{
			id:           "browser_interact_select",
			name:         "Browser Select",
			description:  "Select a value on an element",
			modelName:    "browser_interact_select",
			handlerName:  "browser.interact.select",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId","tabId","element","value"],"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"},"element":{"type":"object"},"value":{"type":"string"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"success":{"type":"boolean"},"action":{"type":"string"},"verified":{"type":"boolean"}}}`,
			riskLevel:    "high",
			sideEffect:   "write",
			idempotent:   false,
			retryable:    false,
			timeoutMs:    30000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.interact", "description": "Interact with browser elements"},
			},
		},
		{
			id:           "browser_interact_hover",
			name:         "Browser Hover",
			description:  "Hover over an element",
			modelName:    "browser_interact_hover",
			handlerName:  "browser.interact.hover",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId","tabId","element"],"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"},"element":{"type":"object"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"success":{"type":"boolean"},"action":{"type":"string"},"verified":{"type":"boolean"}}}`,
			riskLevel:    "medium",
			sideEffect:   "write",
			idempotent:   false,
			retryable:    true,
			timeoutMs:    15000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.interact", "description": "Interact with browser elements"},
			},
		},
		{
			id:           "browser_interact_scroll",
			name:         "Browser Scroll",
			description:  "Scroll the page",
			modelName:    "browser_interact_scroll",
			handlerName:  "browser.interact.scroll",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId","tabId","direction"],"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"},"direction":{"type":"string","enum":["up","down","left","right"]}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"success":{"type":"boolean"},"action":{"type":"string"}}}`,
			riskLevel:    "low",
			sideEffect:   "write",
			idempotent:   false,
			retryable:    true,
			timeoutMs:    15000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.interact", "description": "Interact with browser elements"},
			},
		},
		{
			id:           "browser_resource_download",
			name:         "Browser Download",
			description:  "Download a resource",
			modelName:    "browser_resource_download",
			handlerName:  "browser.resource.download",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId","tabId","url"],"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"},"url":{"type":"string"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"ok":{"type":"boolean"},"resource":{"type":"string"}}}`,
			riskLevel:    "high",
			sideEffect:   "external",
			idempotent:   false,
			retryable:    false,
			timeoutMs:    60000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.resources.download", "description": "Download browser resources"},
			},
		},
		{
			id:           "browser_resource_upload",
			name:         "Browser Upload",
			description:  "Upload a resource",
			modelName:    "browser_resource_upload",
			handlerName:  "browser.resource.upload",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId","tabId","element","resource"],"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"},"element":{"type":"object"},"resource":{"type":"string"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"ok":{"type":"boolean"}}}`,
			riskLevel:    "high",
			sideEffect:   "external",
			idempotent:   false,
			retryable:    false,
			timeoutMs:    60000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.resources.upload", "description": "Upload browser resources"},
			},
		},
		{
			id:           "browser_resource_screenshot",
			name:         "Browser Screenshot",
			description:  "Take a screenshot of the page",
			modelName:    "browser_resource_screenshot",
			handlerName:  "browser.resource.screenshot",
			inputSchema:  `{"type":"object","additionalProperties":false,"required":["sessionId","tabId"],"properties":{"sessionId":{"type":"string"},"tabId":{"type":"string"},"element":{"type":"object"}}}`,
			outputSchema: `{"type":"object","additionalProperties":false,"properties":{"ok":{"type":"boolean"},"resource":{"type":"string"}}}`,
			riskLevel:    "low",
			sideEffect:   "read_only",
			idempotent:   true,
			retryable:    true,
			timeoutMs:    30000,
			permissions: []map[string]any{
				{"capability": "browser.runtime", "description": "Controls a browser instance"},
				{"capability": "browser.resources.screenshot", "description": "Capture browser screenshots"},
			},
		},
	}

	contributions := make([]domain.ContributionDefinition, 0, len(specs))
	for _, s := range specs {
		contributions = append(contributions, domain.ContributionDefinition{
			ID:          domain.ContributionID(s.id),
			ModuleID:    modID,
			ExtensionID: extID,
			Kind:        domain.ContributionKindTool,
			Name:        domain.LocalizedText{Default: s.name},
			Description: domain.LocalizedText{Default: s.description},
			Definition: map[string]any{
				"capabilityId": string(BrowserCapabilityID),
				"modelName":    s.modelName,
				"inputSchema":  s.inputSchema,
				"outputSchema": s.outputSchema,
				"riskLevel":    s.riskLevel,
				"sideEffect":   s.sideEffect,
				"permissions":  s.permissions,
				"timeoutMs":    s.timeoutMs,
				"idempotent":   s.idempotent,
				"retryable":    s.retryable,
				"runtime": map[string]any{
					"runtimeType": "browser",
					"runtimeId":   "default",
					"handlerName": s.handlerName,
				},
			},
			Metadata: map[string]any{
				"system.builtin": true,
			},
		})
	}
	return contributions
}
