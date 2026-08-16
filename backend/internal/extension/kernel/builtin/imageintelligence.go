package builtin

import (
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

const (
	ImageIntelligenceExtensionID = domain.ExtensionID("com.amitia.builtin.image-intelligence")
	ImageIntelligenceModuleID    = domain.ModuleID("com.amitia.builtin.image-intelligence/main")
)

func BuildImageIntelligenceExtension(version string) Definition {
	ver := parseBuiltinVersion(version)

	extDef := domain.ExtensionDefinition{
		ID:      ImageIntelligenceExtensionID,
		Name:    domain.LocalizedText{Default: "Image Intelligence"},
		Description: domain.LocalizedText{
			Default: "Image analysis, OCR, and generation capabilities",
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
			PackageID:       "builtin-image-intelligence",
			ManifestVersion: 1,
		},
		Modules: []domain.ModuleDefinition{
			{
				ID:          ImageIntelligenceModuleID,
				ExtensionID: ImageIntelligenceExtensionID,
				Name:        domain.LocalizedText{Default: "Image Intelligence Runtime"},
				Description: domain.LocalizedText{
					Default: "Built-in module providing image analysis, OCR, and generation",
				},
				Type:    domain.ModuleTypeBuiltin,
				Version: version,
				Runtime: &domain.RuntimeDefinition{
					Type:        domain.RuntimeTypeBuiltin,
					EntryPoint:  "imageintelligence.default",
					WorkerCount: 2,
				},
				Contributions: buildImageIntelligenceContributions(ImageIntelligenceExtensionID, ImageIntelligenceModuleID),
				ProvidedCapabilities: []domain.ProvidedCapability{
					{ID: "media.image.understand", Version: version},
					{ID: "media.image.ocr", Version: version},
					{ID: "media.image.generate", Version: version},
				},
				Provider: &domain.ProviderMetadata{
					ID:       "builtin.image-intelligence",
					Priority: 100,
					Labels: map[string]string{
						"component": "image-intelligence",
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

func buildImageIntelligenceContributions(extID domain.ExtensionID, modID domain.ModuleID) []domain.ContributionDefinition {
	return []domain.ContributionDefinition{
		{
			ID:          "media.image.understand",
			ModuleID:    modID,
			ExtensionID: extID,
			Kind:        domain.ContributionKindTool,
			Name:        domain.LocalizedText{Default: "Analyze Image"},
			Description: domain.LocalizedText{Default: "Analyze the content of an image. Describe scenes, objects, people, text, and any visible information."},
			Definition: map[string]any{
				"capabilityId":   "media.image.understand",
				"modelName":      "media_image_understand",
				"inputSchema":    `{"type":"object","additionalProperties":false,"required":["resourceUri"],"properties":{"resourceUri":{"type":"string"},"prompt":{"type":"string"},"detail":{"enum":["auto","low","high"]}}}`,
				"outputSchema":   `{"type":"object","properties":{"description":{"type":"string"}}}`,
				"riskLevel":      "low",
				"sideEffect":     "external",
				"internal":       true,
				"permissions":    []map[string]any{{"capability": "media.image.read", "risk": "low"}},
				"timeoutMs":      int64(60000),
				"idempotent":     true,
				"retryable":      true,
				"hasSideEffects": false,
				"exposure": map[string]any{
					"exposedByDefault": true,
				},
				"runtime": map[string]any{
					"runtimeType": "internal",
					"runtimeId":   "imageintelligence",
					"handlerName": "understand",
				},
			},
			RuntimeBinding: &domain.RuntimeBinding{
				RuntimeType: domain.RuntimeType("internal"),
				RuntimeID:   "imageintelligence",
				InstanceID:  "understand",
			},
			Exposure: domain.Exposure{
				VisibleByDefault: true,
			},
			Metadata: map[string]any{
				"system.builtin": true,
			},
		},
		{
			ID:          "media.image.ocr",
			ModuleID:    modID,
			ExtensionID: extID,
			Kind:        domain.ContributionKindTool,
			Name:        domain.LocalizedText{Default: "OCR Image"},
			Description: domain.LocalizedText{Default: "Extract text from an image using optical character recognition. Returns the recognized text content."},
			Definition: map[string]any{
				"capabilityId":   "media.image.ocr",
				"modelName":      "media_image_ocr",
				"inputSchema":    `{"type":"object","additionalProperties":false,"required":["resourceUri"],"properties":{"resourceUri":{"type":"string"},"languageHints":{"type":"array","items":{"type":"string"}},"includeBoxes":{"type":"boolean"}}}`,
				"outputSchema":   `{"type":"object","properties":{"text":{"type":"string"}}}`,
				"riskLevel":      "low",
				"sideEffect":     "external",
				"internal":       true,
				"permissions":    []map[string]any{{"capability": "media.image.read", "risk": "low"}},
				"timeoutMs":      int64(30000),
				"idempotent":     true,
				"retryable":      true,
				"hasSideEffects": false,
				"exposure": map[string]any{
					"exposedByDefault": true,
				},
				"runtime": map[string]any{
					"runtimeType": "internal",
					"runtimeId":   "imageintelligence",
					"handlerName": "ocr",
				},
			},
			RuntimeBinding: &domain.RuntimeBinding{
				RuntimeType: domain.RuntimeType("internal"),
				RuntimeID:   "imageintelligence",
				InstanceID:  "ocr",
			},
			Exposure: domain.Exposure{
				VisibleByDefault: true,
			},
			Metadata: map[string]any{
				"system.builtin": true,
			},
		},
		{
			ID:          "media.image.generate",
			ModuleID:    modID,
			ExtensionID: extID,
			Kind:        domain.ContributionKindTool,
			Name:        domain.LocalizedText{Default: "Generate Image"},
			Description: domain.LocalizedText{Default: "Generate images from text descriptions."},
			Definition: map[string]any{
				"capabilityId":   "media.image.generate",
				"modelName":      "media_image_generate",
				"inputSchema":    `{"type":"object","additionalProperties":false,"required":["prompt"],"properties":{"prompt":{"type":"string","maxLength":4096},"count":{"type":"integer","minimum":1,"maximum":4},"width":{"type":"integer","minimum":256,"maximum":4096},"height":{"type":"integer","minimum":256,"maximum":4096},"quality":{"enum":["standard","hd"]}}}`,
				"outputSchema":   `{"type":"object","properties":{"images":{"type":"array","items":{"type":"object"}}}}`,
				"riskLevel":      "medium",
				"sideEffect":     "external",
				"internal":       true,
				"permissions":    []map[string]any{{"capability": "media.image.generate", "risk": "medium"}},
				"timeoutMs":      int64(120000),
				"idempotent":     true,
				"retryable":      false,
				"hasSideEffects": true,
				"exposure": map[string]any{
					"exposedByDefault": true,
				},
				"runtime": map[string]any{
					"runtimeType": "internal",
					"runtimeId":   "imageintelligence",
					"handlerName": "generate",
				},
			},
			RuntimeBinding: &domain.RuntimeBinding{
				RuntimeType: domain.RuntimeType("internal"),
				RuntimeID:   "imageintelligence",
				InstanceID:  "generate",
			},
			Exposure: domain.Exposure{
				VisibleByDefault: true,
			},
			Metadata: map[string]any{
				"system.builtin": true,
			},
		},
		{
			ID:          "image.internal.status",
			ModuleID:    modID,
			ExtensionID: extID,
			Kind:        domain.ContributionKindTool,
			Name:        domain.LocalizedText{Default: "Image Intelligence Status"},
			Description: domain.LocalizedText{Default: "Check image intelligence capability status"},
			Definition: map[string]any{
				"capabilityId":   "image.internal.status",
				"modelName":      "",
				"inputSchema":    `{"type":"object","additionalProperties":false,"properties":{}}`,
				"outputSchema":   `{"type":"object","properties":{"status":{"type":"string"}}}`,
				"riskLevel":      "low",
				"sideEffect":     "read_only",
				"internal":       true,
				"permissions":    []map[string]any{{"capability": "media.image.read", "risk": "low"}},
				"timeoutMs":      int64(5000),
				"idempotent":     true,
				"retryable":      false,
				"hasSideEffects": false,
				"exposure": map[string]any{
					"exposedByDefault": false,
				},
				"runtime": map[string]any{
					"runtimeType": "internal",
					"runtimeId":   "imageintelligence",
					"handlerName": "status",
				},
			},
			RuntimeBinding: &domain.RuntimeBinding{
				RuntimeType: domain.RuntimeType("internal"),
				RuntimeID:   "imageintelligence",
				InstanceID:  "status",
			},
			Exposure: domain.Exposure{
				VisibleByDefault:    false,
				HiddenFromDiscovery: true,
			},
			Metadata: map[string]any{
				"system.builtin": true,
			},
		},
	}
}
