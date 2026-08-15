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
	contributionID := domain.ContributionID(fmt.Sprintf("com.amitia.builtin.ai.%s/chat.generate", protocolFamily))
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
					Contributions: []domain.ContributionDefinition{
						{
							ID:          contributionID,
							ModuleID:    moduleID,
							ExtensionID: extID,
							Kind:        domain.ContributionKindProvider,
							Name: domain.LocalizedText{
								Default: "Chat Generate",
							},
							Description: domain.LocalizedText{
								Default: "Generate chat responses using AI models",
							},
							RuntimeBinding: &domain.RuntimeBinding{
								RuntimeType: domain.RuntimeTypeBuiltin,
								InstanceID:  string(capID),
							},
						},
					},
					ProvidedCapabilities: []domain.ProvidedCapability{
						{ID: string(capID), Version: "1.0.0"},
					},
				},
			},
			Compatibility: domain.ExtensionCompatibility{},
			Integrity:     domain.ExtensionIntegrity{},
			Policies:      domain.ExtensionPolicies{},
		},
		SystemManaged:  true,
		Required:        true,
		DisableAllowed:  false,
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
					Contributions: []domain.ContributionDefinition{
						{
							ID:          domain.ContributionID(PrefixBuiltin + "tts/synthesize"),
							ModuleID:    moduleID,
							ExtensionID: extID,
							Kind:        domain.ContributionKindProvider,
							Name: domain.LocalizedText{
								Default: "TTS Synthesize",
							},
							Description: domain.LocalizedText{
								Default: "Synthesize text to speech audio",
							},
							RuntimeBinding: &domain.RuntimeBinding{
								RuntimeType: domain.RuntimeTypeBuiltin,
								InstanceID:  "speech.tts.synthesize",
							},
						},
						{
							ID:          domain.ContributionID(PrefixBuiltin + "tts/stream"),
							ModuleID:    moduleID,
							ExtensionID: extID,
							Kind:        domain.ContributionKindProvider,
							Name: domain.LocalizedText{
								Default: "TTS Stream",
							},
							Description: domain.LocalizedText{
								Default: "Stream text-to-speech audio in real-time",
							},
							RuntimeBinding: &domain.RuntimeBinding{
								RuntimeType: domain.RuntimeTypeBuiltin,
								InstanceID:  "speech.tts.stream",
							},
						},
					},
					ProvidedCapabilities: []domain.ProvidedCapability{
						{ID: "speech.tts.synthesize", Version: "1.0.0"},
						{ID: "speech.tts.stream", Version: "1.0.0"},
					},
				},
			},
			Compatibility: domain.ExtensionCompatibility{},
			Integrity:     domain.ExtensionIntegrity{},
			Policies:      domain.ExtensionPolicies{},
		},
		SystemManaged:  true,
		Required:        false,
		DisableAllowed:  true,
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
					Contributions: []domain.ContributionDefinition{
						{
							ID:          domain.ContributionID(PrefixBuiltin + "asr/transcribe"),
							ModuleID:    moduleID,
							ExtensionID: extID,
							Kind:        domain.ContributionKindProvider,
							Name: domain.LocalizedText{
								Default: "ASR Transcribe",
							},
							Description: domain.LocalizedText{
								Default: "Transcribe speech audio to text",
							},
							RuntimeBinding: &domain.RuntimeBinding{
								RuntimeType: domain.RuntimeTypeBuiltin,
								InstanceID:  "speech.asr.transcribe",
							},
						},
						{
							ID:          domain.ContributionID(PrefixBuiltin + "asr/stream"),
							ModuleID:    moduleID,
							ExtensionID: extID,
							Kind:        domain.ContributionKindProvider,
							Name: domain.LocalizedText{
								Default: "ASR Stream",
							},
							Description: domain.LocalizedText{
								Default: "Stream real-time speech recognition",
							},
							RuntimeBinding: &domain.RuntimeBinding{
								RuntimeType: domain.RuntimeTypeBuiltin,
								InstanceID:  "speech.asr.stream",
							},
						},
					},
					ProvidedCapabilities: []domain.ProvidedCapability{
						{ID: "speech.asr.transcribe", Version: "1.0.0"},
						{ID: "speech.asr.stream", Version: "1.0.0"},
					},
				},
			},
			Compatibility: domain.ExtensionCompatibility{},
			Integrity:     domain.ExtensionIntegrity{},
			Policies:      domain.ExtensionPolicies{},
		},
		SystemManaged:  true,
		Required:        false,
		DisableAllowed:  true,
		BootstrapRevision: 1,
	}
}

// BuildMediaExtension constructs a Built-in Extension definition for media processing.
//
//	Extension ID: com.amitia.builtin.media
//	Provider Capabilities: media.metadata, media.convert
//
// Note: Existing Tool IDs (e.g. "media.metadata", "media.convert") are preserved
// via contribution and runtime binding for backward compatibility.
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
					Contributions: []domain.ContributionDefinition{
						{
							ID:          domain.ContributionID("media.metadata"),
							ModuleID:    moduleID,
							ExtensionID: extID,
							Kind:        domain.ContributionKindProvider,
							Name: domain.LocalizedText{
								Default: "Media Metadata",
							},
							Description: domain.LocalizedText{
								Default: "Get metadata of a media file",
							},
							RuntimeBinding: &domain.RuntimeBinding{
								RuntimeType: domain.RuntimeTypeBuiltin,
								InstanceID:  "media.metadata",
							},
							Metadata: map[string]any{
								"legacyToolId": "media.metadata",
							},
						},
						{
							ID:          domain.ContributionID("media.convert"),
							ModuleID:    moduleID,
							ExtensionID: extID,
							Kind:        domain.ContributionKindProvider,
							Name: domain.LocalizedText{
								Default: "Media Convert",
							},
							Description: domain.LocalizedText{
								Default: "Convert media to another format",
							},
							RuntimeBinding: &domain.RuntimeBinding{
								RuntimeType: domain.RuntimeTypeBuiltin,
								InstanceID:  "media.convert",
							},
							Metadata: map[string]any{
								"legacyToolId": "media.convert",
							},
						},
					},
					ProvidedCapabilities: []domain.ProvidedCapability{
						{ID: "media.metadata", Version: "1.0.0"},
						{ID: "media.convert", Version: "1.0.0"},
					},
				},
			},
			Compatibility: domain.ExtensionCompatibility{},
			Integrity:     domain.ExtensionIntegrity{},
			Policies:      domain.ExtensionPolicies{},
		},
		SystemManaged:  true,
		Required:        false,
		DisableAllowed:  true,
		BootstrapRevision: 1,
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
					Contributions: []domain.ContributionDefinition{
						{
							ID:          domain.ContributionID(PrefixBuiltin + "image/generate"),
							ModuleID:    moduleID,
							ExtensionID: extID,
							Kind:        domain.ContributionKindProvider,
							Name: domain.LocalizedText{
								Default: "Image Generate",
							},
							Description: domain.LocalizedText{
								Default: "Generate images from text descriptions",
							},
							RuntimeBinding: &domain.RuntimeBinding{
								RuntimeType: domain.RuntimeTypeBuiltin,
								InstanceID:  "image.generate",
							},
						},
					},
					ProvidedCapabilities: []domain.ProvidedCapability{
						{ID: "image.generate", Version: "1.0.0"},
					},
				},
			},
			Compatibility: domain.ExtensionCompatibility{},
			Integrity:     domain.ExtensionIntegrity{},
			Policies:      domain.ExtensionPolicies{},
		},
		SystemManaged:  true,
		Required:        false,
		DisableAllowed:  true,
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
						Type: domain.RuntimeTypeBuiltin,
					},
					Contributions: []domain.ContributionDefinition{
						{
							ID:          domain.ContributionID(PrefixBuiltin + "background-removal/remove"),
							ModuleID:    moduleID,
							ExtensionID: extID,
							Kind:        domain.ContributionKindProvider,
							Name: domain.LocalizedText{
								Default: "Background Remove",
							},
							Description: domain.LocalizedText{
								Default: "Remove the background from an image",
							},
							RuntimeBinding: &domain.RuntimeBinding{
								RuntimeType: domain.RuntimeTypeBuiltin,
								InstanceID:  "image.background.remove",
							},
						},
					},
					ProvidedCapabilities: []domain.ProvidedCapability{
						{ID: "image.background.remove", Version: "1.0.0"},
					},
				},
			},
			Compatibility: domain.ExtensionCompatibility{},
			Integrity:     domain.ExtensionIntegrity{},
			Policies:      domain.ExtensionPolicies{},
		},
		SystemManaged:  true,
		Required:        false,
		DisableAllowed:  true,
		BootstrapRevision: 1,
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
					Contributions: []domain.ContributionDefinition{
						{
							ID:          domain.ContributionID(PrefixBuiltin + "vision/analyze"),
							ModuleID:    moduleID,
							ExtensionID: extID,
							Kind:        domain.ContributionKindProvider,
							Name: domain.LocalizedText{
								Default: "Vision Analyze",
							},
							Description: domain.LocalizedText{
								Default: "Analyze and describe image content",
							},
							RuntimeBinding: &domain.RuntimeBinding{
								RuntimeType: domain.RuntimeTypeBuiltin,
								InstanceID:  "vision.analyze",
							},
						},
					},
					ProvidedCapabilities: []domain.ProvidedCapability{
						{ID: "vision.analyze", Version: "1.0.0"},
					},
				},
			},
			Compatibility: domain.ExtensionCompatibility{},
			Integrity:     domain.ExtensionIntegrity{},
			Policies:      domain.ExtensionPolicies{},
		},
		SystemManaged:  true,
		Required:        false,
		DisableAllowed:  false,
		BootstrapRevision: 1,
	}
}

