package builtin

import (
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

// defaultBuiltinVersion is the semantic version used when the caller passes an empty version string.
var defaultBuiltinVersion = domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}

// parseBuiltinVersion parses a version string (e.g. "1.0.0"), falling back to defaultBuiltinVersion when empty or invalid.
func parseBuiltinVersion(version string) domain.SemanticVersion {
	if version == "" {
		return defaultBuiltinVersion
	}
	ver, err := domain.ParseVersion(version)
	if err != nil {
		return defaultBuiltinVersion
	}
	return ver
}

// BuildAIExtension constructs a Built-in Extension definition for an AI model provider
// identified by protocolFamily (e.g. "openai", "anthropic", "gemini", "ollama").
//
//	Extension ID: com.amitia.builtin.ai.{protocolFamily}
//	Provider Capability: ai.chat.generate
func BuildAIExtension(protocolFamily, version string) Definition {
	extID := domain.ExtensionID(fmt.Sprintf("com.amitia.builtin.ai.%s", protocolFamily))
	moduleID := domain.ModuleID(fmt.Sprintf("com.amitia.builtin.ai.%s/main", protocolFamily))
	contributionName := fmt.Sprintf("AI %s Chat Generate", protocolFamily)
	capID := capability.CapabilityID("ai.chat.generate")
	ver := parseBuiltinVersion(version)

	return Definition{
		Extension: domain.ExtensionDefinition{
			ID:   extID,
			Name: domain.LocalizedText{Default: contributionName},
			Description: domain.LocalizedText{
				Default: fmt.Sprintf("AI chat generation via %s protocol family", protocolFamily),
			},
			Version:         ver,
			ManifestVersion: 1,
			Domain:          domain.ExtensionDomainGeneral,
			Placement:       domain.ExtensionPlacementCloud,
			Publisher: domain.PublisherReference{
				PublisherID: "com.amitia",
				DisplayName: "Amitia",
			},
			Package: domain.PackageReference{
				PackageID: fmt.Sprintf("builtin-ai-%s", protocolFamily),
			},
			Modules: []domain.ModuleDefinition{
				{
					ID:          moduleID,
					ExtensionID: extID,
					Name: domain.LocalizedText{
						Default: fmt.Sprintf("AI %s Provider", protocolFamily),
					},
					Description: domain.LocalizedText{
						Default: fmt.Sprintf("Provides AI chat generation capability via %s", protocolFamily),
					},
					Type: domain.ModuleTypeBuiltin,
					Runtime: &domain.RuntimeDefinition{
						Type: domain.RuntimeTypeBuiltin,
					},
					ProvidedCapabilities: []domain.ProvidedCapability{
						{ID: string(capID), Version: "1.0.0"},
					},
					Provider: &domain.ProviderMetadata{
						ID:       "builtin.ai." + protocolFamily,
						Priority: 100,
						Metadata: map[string]any{
							"protocol": protocolFamily,
						},
					},
				},
			},
			Compatibility: domain.ExtensionCompatibility{},
			Integrity:     domain.ExtensionIntegrity{},
			Policies:      domain.ExtensionPolicies{},
		},
		SystemManaged:     true,
		Required:          true,
		DisableAllowed:    false,
		BootstrapRevision: 1,
	}
}

// BuildTTSExtension constructs a Built-in Extension definition for text-to-speech.
//
//	Extension ID: com.amitia.builtin.tts
//	Provider Capabilities: speech.tts.synthesize, speech.tts.stream
func BuildTTSExtension(version string) Definition {
	extID := domain.ExtensionID(PrefixBuiltin + "tts")
	moduleID := domain.ModuleID(PrefixBuiltin + "tts/main")
	ver := parseBuiltinVersion(version)

	return Definition{
		Extension: domain.ExtensionDefinition{
			ID:   extID,
			Name: domain.LocalizedText{Default: "Text-to-Speech"},
			Description: domain.LocalizedText{
				Default: "Text-to-speech synthesis and streaming capability",
			},
			Version:         ver,
			ManifestVersion: 1,
			Domain:          domain.ExtensionDomainGeneral,
			Placement:       domain.ExtensionPlacementCloud,
			Publisher: domain.PublisherReference{
				PublisherID: "com.amitia",
				DisplayName: "Amitia",
			},
			Package: domain.PackageReference{
				PackageID: "builtin-tts",
			},
			Modules: []domain.ModuleDefinition{
				{
					ID:          moduleID,
					ExtensionID: extID,
					Name: domain.LocalizedText{
						Default: "TTS Provider",
					},
					Description: domain.LocalizedText{
						Default: "Provides text-to-speech synthesis and streaming",
					},
					Type: domain.ModuleTypeBuiltin,
					Runtime: &domain.RuntimeDefinition{
						Type: domain.RuntimeTypeBuiltin,
					},
					ProvidedCapabilities: []domain.ProvidedCapability{
						{ID: "speech.tts.synthesize", Version: "1.0.0"},
						{ID: "speech.tts.stream", Version: "1.0.0"},
					},
					Provider: &domain.ProviderMetadata{
						ID:       "builtin.tts",
						Priority: 100,
						Metadata: map[string]any{},
					},
				},
			},
			Compatibility: domain.ExtensionCompatibility{},
			Integrity:     domain.ExtensionIntegrity{},
			Policies:      domain.ExtensionPolicies{},
		},
		SystemManaged:     true,
		Required:          false,
		DisableAllowed:    true,
		BootstrapRevision: 1,
	}
}

// BuildASRExtension constructs a Built-in Extension definition for automatic speech recognition.
//
//	Extension ID: com.amitia.builtin.asr
//	Provider Capabilities: speech.asr.transcribe, speech.asr.stream
func BuildASRExtension(version string) Definition {
	extID := domain.ExtensionID(PrefixBuiltin + "asr")
	moduleID := domain.ModuleID(PrefixBuiltin + "asr/main")
	ver := parseBuiltinVersion(version)

	return Definition{
		Extension: domain.ExtensionDefinition{
			ID:   extID,
			Name: domain.LocalizedText{Default: "Automatic Speech Recognition"},
			Description: domain.LocalizedText{
				Default: "Automatic speech recognition and streaming transcription capability",
			},
			Version:         ver,
			ManifestVersion: 1,
			Domain:          domain.ExtensionDomainGeneral,
			Placement:       domain.ExtensionPlacementCloud,
			Publisher: domain.PublisherReference{
				PublisherID: "com.amitia",
				DisplayName: "Amitia",
			},
			Package: domain.PackageReference{
				PackageID: "builtin-asr",
			},
			Modules: []domain.ModuleDefinition{
				{
					ID:          moduleID,
					ExtensionID: extID,
					Name: domain.LocalizedText{
						Default: "ASR Provider",
					},
					Description: domain.LocalizedText{
						Default: "Provides automatic speech recognition and streaming transcription",
					},
					Type: domain.ModuleTypeBuiltin,
					Runtime: &domain.RuntimeDefinition{
						Type: domain.RuntimeTypeBuiltin,
					},
					ProvidedCapabilities: []domain.ProvidedCapability{
						{ID: "speech.asr.transcribe", Version: "1.0.0"},
						{ID: "speech.asr.stream", Version: "1.0.0"},
					},
					Provider: &domain.ProviderMetadata{
						ID:       "builtin.asr",
						Priority: 100,
						Metadata: map[string]any{},
					},
				},
			},
			Compatibility: domain.ExtensionCompatibility{},
			Integrity:     domain.ExtensionIntegrity{},
			Policies:      domain.ExtensionPolicies{},
		},
		SystemManaged:     true,
		Required:          false,
		DisableAllowed:    true,
		BootstrapRevision: 1,
	}
}

// BuildMediaExtension constructs a Built-in Extension definition for media processing.
//
//	Extension ID: com.amitia.builtin.media
//	Provider Capabilities: media.metadata, media.convert
func BuildMediaExtension(version string) Definition {
	extID := domain.ExtensionID(PrefixBuiltin + "media")
	moduleID := domain.ModuleID(PrefixBuiltin + "media/main")
	ver := parseBuiltinVersion(version)

	return Definition{
		Extension: domain.ExtensionDefinition{
			ID:   extID,
			Name: domain.LocalizedText{Default: "Media Processing"},
			Description: domain.LocalizedText{
				Default: "Media metadata extraction and format conversion capability",
			},
			Version:         ver,
			ManifestVersion: 1,
			Domain:          domain.ExtensionDomainGeneral,
			Placement:       domain.ExtensionPlacementCloud,
			Publisher: domain.PublisherReference{
				PublisherID: "com.amitia",
				DisplayName: "Amitia",
			},
			Package: domain.PackageReference{
				PackageID: "builtin-media",
			},
			Modules: []domain.ModuleDefinition{
				{
					ID:          moduleID,
					ExtensionID: extID,
					Name: domain.LocalizedText{
						Default: "Media Provider",
					},
					Description: domain.LocalizedText{
						Default: "Provides media metadata extraction and format conversion",
					},
					Type: domain.ModuleTypeBuiltin,
					Runtime: &domain.RuntimeDefinition{
						Type: domain.RuntimeTypeBuiltin,
					},
					Contributions: buildMediaContributions(extID, moduleID),
					ProvidedCapabilities: []domain.ProvidedCapability{
						{ID: "media.metadata", Version: "1.0.0"},
						{ID: "media.convert", Version: "1.0.0"},
						{ID: "media.ffmpeg.execute", Version: "1.0.0"},
					},
					Provider: &domain.ProviderMetadata{
						ID:       "builtin.media",
						Priority: 100,
						Metadata: map[string]any{},
					},
				},
			},
			Compatibility: domain.ExtensionCompatibility{},
			Integrity:     domain.ExtensionIntegrity{},
			Policies:      domain.ExtensionPolicies{},
		},
		SystemManaged:     true,
		Required:          false,
		DisableAllowed:    true,
		BootstrapRevision: 1,
	}
}

func buildMediaContributions(extID domain.ExtensionID, modID domain.ModuleID) []domain.ContributionDefinition {
	return []domain.ContributionDefinition{
		{
			ID:          domain.ContributionID("media.metadata"),
			ModuleID:    modID,
			ExtensionID: extID,
			Kind:        domain.ContributionKindTool,
			Name:        domain.LocalizedText{Default: "Media Metadata"},
			Description: domain.LocalizedText{Default: "Get metadata of a media file"},
			Definition: map[string]any{
				"capabilityId": "media.metadata",
				"modelName":    "media.metadata",
				"inputSchema":  `{"type":"object","properties":{"resource":{"type":"string"}},"required":["resource"],"additionalProperties":false}`,
				"outputSchema": `{"type":"object","properties":{"metadata":{"type":"object"}}}`,
				"riskLevel":    "low",
				"sideEffect":   "read_only",
				"permissions":  []map[string]any{{"capability": "media.metadata", "risk": "low"}},
				"timeoutMs":    int64(30000),
				"idempotent":   true,
				"retryable":    true,
				"runtime": map[string]any{
					"runtimeType": "media",
					"runtimeId":   "default",
					"handlerName": "media.metadata",
				},
			},
			Metadata: map[string]any{
				"system.builtin": true,
			},
		},
		{
			ID:          domain.ContributionID("media.convert"),
			ModuleID:    modID,
			ExtensionID: extID,
			Kind:        domain.ContributionKindTool,
			Name:        domain.LocalizedText{Default: "Media Convert"},
			Description: domain.LocalizedText{Default: "Convert media to another format"},
			Definition: map[string]any{
				"capabilityId": "media.convert",
				"modelName":    "media.convert",
				"inputSchema":  `{"type":"object","properties":{"resource":{"type":"string"},"target":{"type":"string"},"targetContainer":{"type":"string"},"videoCodec":{"type":"string"},"audioCodec":{"type":"string"}},"required":["resource"],"additionalProperties":false}`,
				"outputSchema": `{"type":"object","properties":{"resource":{"type":"string"},"entry":{"type":"object"}}}`,
				"riskLevel":    "medium",
				"sideEffect":   "write",
				"permissions":  []map[string]any{{"capability": "media.convert", "risk": "medium"}},
				"timeoutMs":    int64(120000),
				"idempotent":   false,
				"retryable":    false,
				"runtime": map[string]any{
					"runtimeType": "media",
					"runtimeId":   "default",
					"handlerName": "media.convert",
				},
			},
			Metadata: map[string]any{
				"system.builtin": true,
			},
		},
		{
			ID:          domain.ContributionID("media.ffmpeg.execute"),
			ModuleID:    modID,
			ExtensionID: extID,
			Kind:        domain.ContributionKindTool,
			Name:        domain.LocalizedText{Default: "Advanced FFmpeg Execute"},
			Description: domain.LocalizedText{Default: "Run advanced bounded FFmpeg arguments against one ResourceURI input and output"},
			Definition: map[string]any{
				"capabilityId": "media.ffmpeg.execute",
				"modelName":    "ffmpeg_execute",
				"inputSchema":  `{"type":"object","required":["sourceUri","targetUri","args"],"properties":{"sourceUri":{"type":"string","minLength":1},"targetUri":{"type":"string","minLength":1},"args":{"type":"array","maxItems":128,"items":{"type":"string","maxLength":8192}}},"additionalProperties":false}`,
				"outputSchema": `{"type":"object","properties":{"resourceUri":{"type":"string"},"exitCode":{"type":"integer"},"stdout":{"type":"string"},"stderr":{"type":"string"},"durationMs":{"type":"integer"}}}`,
				"riskLevel":    "high",
				"sideEffect":   "write",
				"permissions":  []map[string]any{{"capability": "media.ffmpeg.execute", "risk": "high"}},
				"timeoutMs":    int64(120000),
				"idempotent":   false,
				"retryable":    false,
				"runtime": map[string]any{
					"runtimeType": "media",
					"runtimeId":   "default",
					"handlerName": "media.ffmpeg.execute",
				},
			},
			Metadata: map[string]any{"system.builtin": true},
		},
	}
}

// BuildImageExtension constructs a Built-in Extension definition for image generation.
//
//	Extension ID: com.amitia.builtin.image
//	Provider Capability: image.generate
func BuildImageExtension(version string) Definition {
	extID := domain.ExtensionID(PrefixBuiltin + "image")
	moduleID := domain.ModuleID(PrefixBuiltin + "image/main")
	ver := parseBuiltinVersion(version)

	return Definition{
		Extension: domain.ExtensionDefinition{
			ID:   extID,
			Name: domain.LocalizedText{Default: "Image Generation"},
			Description: domain.LocalizedText{
				Default: "AI-powered image generation capability",
			},
			Version:         ver,
			ManifestVersion: 1,
			Domain:          domain.ExtensionDomainGeneral,
			Placement:       domain.ExtensionPlacementCloud,
			Publisher: domain.PublisherReference{
				PublisherID: "com.amitia",
				DisplayName: "Amitia",
			},
			Package: domain.PackageReference{
				PackageID: "builtin-image",
			},
			Modules: []domain.ModuleDefinition{
				{
					ID:          moduleID,
					ExtensionID: extID,
					Name: domain.LocalizedText{
						Default: "Image Provider",
					},
					Description: domain.LocalizedText{
						Default: "Provides AI-powered image generation capability",
					},
					Type: domain.ModuleTypeBuiltin,
					Runtime: &domain.RuntimeDefinition{
						Type: domain.RuntimeTypeBuiltin,
					},
					ProvidedCapabilities: []domain.ProvidedCapability{
						{ID: "image.generate", Version: "1.0.0"},
					},
					Provider: &domain.ProviderMetadata{
						ID:       "builtin.image",
						Priority: 100,
						Metadata: map[string]any{},
					},
				},
			},
			Compatibility: domain.ExtensionCompatibility{},
			Integrity:     domain.ExtensionIntegrity{},
			Policies:      domain.ExtensionPolicies{},
		},
		SystemManaged:     true,
		Required:          false,
		DisableAllowed:    true,
		BootstrapRevision: 1,
	}
}

// BuildBackgroundRemovalExtension constructs a Built-in Extension definition for background removal.
//
//	Extension ID: com.amitia.builtin.background-removal
//	Provider Capability: image.background.remove
func BuildBackgroundRemovalExtension(version string) Definition {
	extID := domain.ExtensionID(PrefixBuiltin + "background-removal")
	moduleID := domain.ModuleID(PrefixBuiltin + "background-removal/main")
	ver := parseBuiltinVersion(version)

	return Definition{
		Extension: domain.ExtensionDefinition{
			ID:   extID,
			Name: domain.LocalizedText{Default: "Background Removal"},
			Description: domain.LocalizedText{
				Default: "Remove background from images using AI",
			},
			Version:         ver,
			ManifestVersion: 1,
			Domain:          domain.ExtensionDomainGeneral,
			Placement:       domain.ExtensionPlacementCloud,
			Publisher: domain.PublisherReference{
				PublisherID: "com.amitia",
				DisplayName: "Amitia",
			},
			Package: domain.PackageReference{
				PackageID: "builtin-background-removal",
			},
			Modules: []domain.ModuleDefinition{
				{
					ID:          moduleID,
					ExtensionID: extID,
					Name: domain.LocalizedText{
						Default: "Background Removal Provider",
					},
					Description: domain.LocalizedText{
						Default: "Provides AI-powered background removal from images",
					},
					Type: domain.ModuleTypeBuiltin,
					Runtime: &domain.RuntimeDefinition{
						Type: domain.RuntimeTypeBackgroundRemoval,
					},
					Contributions: buildBackgroundRemovalContributions(extID, moduleID),
					ProvidedCapabilities: []domain.ProvidedCapability{
						{ID: "image.background.remove", Version: "1.0.0"},
					},
					Provider: &domain.ProviderMetadata{
						ID:       "builtin.background-removal",
						Priority: 100,
						Metadata: map[string]any{},
					},
				},
			},
			Compatibility: domain.ExtensionCompatibility{},
			Integrity:     domain.ExtensionIntegrity{},
			Policies:      domain.ExtensionPolicies{},
		},
		SystemManaged:     true,
		Required:          false,
		DisableAllowed:    true,
		BootstrapRevision: 1,
	}
}

func buildBackgroundRemovalContributions(extID domain.ExtensionID, modID domain.ModuleID) []domain.ContributionDefinition {
	return []domain.ContributionDefinition{
		{
			ID:          domain.ContributionID("background_removal.remove"),
			ModuleID:    modID,
			ExtensionID: extID,
			Kind:        domain.ContributionKindTool,
			Name:        domain.LocalizedText{Default: "Background Removal"},
			Description: domain.LocalizedText{Default: "Remove the background from an image"},
			Definition: map[string]any{
				"capabilityId": "image.background.remove",
				"modelName":    "image.background.remove",
				"inputSchema":  `{"type":"object","properties":{"image":{"type":"string","description":"Base64-encoded image or image URL"},"mode":{"type":"string","enum":["remove","keep_alpha","use_existing_alpha"],"description":"Background removal mode"},"provider":{"type":"string","description":"Optional provider name override"}},"required":["image"],"additionalProperties":false}`,
				"outputSchema": `{"type":"object","properties":{"image":{"type":"string","description":"Base64-encoded foreground image"},"mask":{"type":"string","description":"Base64-encoded mask image"},"width":{"type":"integer"},"height":{"type":"integer"},"provider":{"type":"string"},"degraded":{"type":"boolean"},"removedRatio":{"type":"number"}}}`,
				"riskLevel":    "low",
				"sideEffect":   "read_only",
				"permissions":  []map[string]any{{"capability": "image.background.remove", "risk": "low"}},
				"timeoutMs":    int64(120000),
				"idempotent":   true,
				"retryable":    true,
				"runtime": map[string]any{
					"runtimeType": "background_removal",
					"runtimeId":   "default",
					"handlerName": "image.background.remove",
				},
			},
			Metadata: map[string]any{
				"system.builtin": true,
			},
		},
	}
}

// BuildVisionExtension constructs a Built-in Extension definition for vision analysis.
//
//	Extension ID: com.amitia.builtin.vision
//	Provider Capability: vision.analyze
func BuildVisionExtension(version string) Definition {
	extID := domain.ExtensionID(PrefixBuiltin + "vision")
	moduleID := domain.ModuleID(PrefixBuiltin + "vision/main")
	ver := parseBuiltinVersion(version)

	return Definition{
		Extension: domain.ExtensionDefinition{
			ID:   extID,
			Name: domain.LocalizedText{Default: "Vision Analysis"},
			Description: domain.LocalizedText{
				Default: "Analyze and understand images using AI vision models",
			},
			Version:         ver,
			ManifestVersion: 1,
			Domain:          domain.ExtensionDomainGeneral,
			Placement:       domain.ExtensionPlacementCloud,
			Publisher: domain.PublisherReference{
				PublisherID: "com.amitia",
				DisplayName: "Amitia",
			},
			Package: domain.PackageReference{
				PackageID: "builtin-vision",
			},
			Modules: []domain.ModuleDefinition{
				{
					ID:          moduleID,
					ExtensionID: extID,
					Name: domain.LocalizedText{
						Default: "Vision Provider",
					},
					Description: domain.LocalizedText{
						Default: "Provides AI vision analysis capability",
					},
					Type: domain.ModuleTypeBuiltin,
					Runtime: &domain.RuntimeDefinition{
						Type: domain.RuntimeTypeBuiltin,
					},
					ProvidedCapabilities: []domain.ProvidedCapability{
						{ID: "vision.analyze", Version: "1.0.0"},
					},
					Provider: &domain.ProviderMetadata{
						ID:       "builtin.vision",
						Priority: 100,
						Metadata: map[string]any{},
					},
				},
			},
			Compatibility: domain.ExtensionCompatibility{},
			Integrity:     domain.ExtensionIntegrity{},
			Policies:      domain.ExtensionPolicies{},
		},
		SystemManaged:     true,
		Required:          false,
		DisableAllowed:    false,
		BootstrapRevision: 1,
	}
}
