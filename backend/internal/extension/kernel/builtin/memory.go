package builtin

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func parseProvidedCaps(caps []string) []domain.ProvidedCapability {
	out := make([]domain.ProvidedCapability, 0, len(caps))
	for _, c := range caps {
		out = append(out, domain.ProvidedCapability{ID: c, Version: "1.0.0"})
	}
	return out
}

func buildBaseDomainDefinition(id, name, desc, ver string) (domain.ExtensionDefinition, []domain.ModuleDefinition) {
	def := domain.ExtensionDefinition{
		ID:              domain.ExtensionID(id),
		Name:            domain.LocalizedText{Default: name},
		Description:     domain.LocalizedText{Default: desc},
		Version:         parseBuiltinVersion(ver),
		ManifestVersion: 2,
		Domain:          domain.ExtensionDomain(id),
	}
	return def, nil
}

func finalizeDef(def domain.ExtensionDefinition) Definition {
	def.Compatibility = domain.ExtensionCompatibility{
		Platforms: []string{"windows", "linux", "darwin"},
	}
	for i := range def.Modules {
		if def.Modules[i].ExtensionID == "" {
			def.Modules[i].ExtensionID = def.ID
		}
	}
	return Definition{
		Extension:       def,
		SystemManaged:   true,
		Required:        true,
		DisableAllowed:  false,
	}
}

func BuildMemoryExtension(version string) Definition {
	def := domain.ExtensionDefinition{
		ID:              domain.ExtensionID("com.amitia.builtin.memory"),
		Name:            domain.LocalizedText{Default: "Memory"},
		Description:     domain.LocalizedText{Default: "Memory storage and retrieval"},
		Version:         parseBuiltinVersion(version),
		ManifestVersion: 2,
		Domain:          domain.ExtensionDomain("memory"),
	}
	def.Modules = []domain.ModuleDefinition{
		{
			ID:   domain.ModuleID("memory-storage"),
			Name: domain.LocalizedText{Default: "Memory Storage"},
			Type: domain.ModuleTypeBuiltin,
			Runtime: &domain.RuntimeDefinition{
				Type:       domain.RuntimeTypeBuiltin,
				EntryPoint: "memory://builtin",
			},
			ProvidedCapabilities: parseProvidedCaps([]string{"memory.read", "memory.write", "memory.search", "memory.embed"}),
		},
	}
	return finalizeDef(def)
}

func BuildProfileExtension(version string) Definition {
	def := domain.ExtensionDefinition{
		ID:              domain.ExtensionID("com.amitia.builtin.profile"),
		Name:            domain.LocalizedText{Default: "User Profile"},
		Description:     domain.LocalizedText{Default: "User profile storage and retrieval"},
		Version:         parseBuiltinVersion(version),
		ManifestVersion: 2,
		Domain:          domain.ExtensionDomain("profile"),
	}
	def.Modules = []domain.ModuleDefinition{
		{
			ID:   domain.ModuleID("profile-storage"),
			Name: domain.LocalizedText{Default: "Profile Storage"},
			Type: domain.ModuleTypeBuiltin,
			Runtime: &domain.RuntimeDefinition{
				Type:       domain.RuntimeTypeBuiltin,
				EntryPoint: "profile://builtin",
			},
			ProvidedCapabilities: parseProvidedCaps([]string{"profile.read", "profile.write", "profile.update", "profile.query"}),
		},
	}
	return finalizeDef(def)
}

func BuildEpisodicExtension(version string) Definition {
	def := domain.ExtensionDefinition{
		ID:              domain.ExtensionID("com.amitia.builtin.episodic"),
		Name:            domain.LocalizedText{Default: "Episodic Memory"},
		Description:     domain.LocalizedText{Default: "Episodic conversation memory"},
		Version:         parseBuiltinVersion(version),
		ManifestVersion: 2,
		Domain:          domain.ExtensionDomain("episodic"),
	}
	def.Modules = []domain.ModuleDefinition{
		{
			ID:   domain.ModuleID("episodic-storage"),
			Name: domain.LocalizedText{Default: "Episodic Storage"},
			Type: domain.ModuleTypeBuiltin,
			Runtime: &domain.RuntimeDefinition{
				Type:       domain.RuntimeTypeBuiltin,
				EntryPoint: "episodic://builtin",
			},
			ProvidedCapabilities: parseProvidedCaps([]string{"episodic.write", "episodic.search", "episodic.summarize", "episodic.prune"}),
		},
	}
	return finalizeDef(def)
}

func BuildWorldBookExtension(version string) Definition {
	def := domain.ExtensionDefinition{
		ID:              domain.ExtensionID("com.amitia.builtin.worldbook"),
		Name:            domain.LocalizedText{Default: "World Book"},
		Description:     domain.LocalizedText{Default: "Static world-building knowledge store"},
		Version:         parseBuiltinVersion(version),
		ManifestVersion: 2,
		Domain:          domain.ExtensionDomain("worldbook"),
	}
	def.Modules = []domain.ModuleDefinition{
		{
			ID:   domain.ModuleID("worldbook-storage"),
			Name: domain.LocalizedText{Default: "WorldBook Storage"},
			Type: domain.ModuleTypeBuiltin,
			Runtime: &domain.RuntimeDefinition{
				Type:       domain.RuntimeTypeBuiltin,
				EntryPoint: "worldbook://builtin",
			},
			ProvidedCapabilities: parseProvidedCaps([]string{"worldbook.read", "worldbook.write", "worldbook.search", "worldbook.edit"}),
		},
	}
	return finalizeDef(def)
}

func BuildCompanionExtension(version string) Definition {
	def := domain.ExtensionDefinition{
		ID:              domain.ExtensionID("com.amitia.builtin.companion"),
		Name:            domain.LocalizedText{Default: "Companion"},
		Description:     domain.LocalizedText{Default: "Companion configuration and lifecycle"},
		Version:         parseBuiltinVersion(version),
		ManifestVersion: 2,
		Domain:          domain.ExtensionDomain("companion"),
	}
	def.Modules = []domain.ModuleDefinition{
		{
			ID:   domain.ModuleID("companion-core"),
			Name: domain.LocalizedText{Default: "Companion Core"},
			Type: domain.ModuleTypeBuiltin,
			Runtime: &domain.RuntimeDefinition{
				Type:       domain.RuntimeTypeBuiltin,
				EntryPoint: "companion://builtin",
			},
			ProvidedCapabilities: parseProvidedCaps([]string{"companion.load", "companion.activate", "companion.deactivate", "companion.state"}),
		},
	}
	return finalizeDef(def)
}

type MemoryProvider interface {
	Read(ctx context.Context, query string, limit int) ([]byte, error)
	Write(ctx context.Context, data []byte) error
	Search(ctx context.Context, embedding []float32, limit int) ([]byte, error)
}

type ProfileProvider interface {
	Get(ctx context.Context, userID string) ([]byte, error)
	Update(ctx context.Context, userID string, data []byte) error
}

type EpisodicProvider interface {
	Append(ctx context.Context, episode []byte) error
	Search(ctx context.Context, query string, limit int) ([]byte, error)
	Summarize(ctx context.Context, sessionID string) ([]byte, error)
}

type WorldBookProvider interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, data []byte) error
	Search(ctx context.Context, query string) ([]byte, error)
}

type CompanionProvider interface {
	Load(ctx context.Context, companionID string) ([]byte, error)
	Activate(ctx context.Context, companionID string) error
	Deactivate(ctx context.Context, companionID string) error
}

type MemoryProviderFactory interface {
	NewMemoryProvider(config map[string]any) (MemoryProvider, error)
	NewProfileProvider(config map[string]any) (ProfileProvider, error)
	NewEpisodicProvider(config map[string]any) (EpisodicProvider, error)
	NewWorldBookProvider(config map[string]any) (WorldBookProvider, error)
	NewCompanionProvider(config map[string]any) (CompanionProvider, error)
}

var _ = capability.CapabilityID("")
var _ = domain.ExtensionID("")
