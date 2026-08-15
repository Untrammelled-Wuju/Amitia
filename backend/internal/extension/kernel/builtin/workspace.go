package builtin

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

const (
	WorkspaceExtensionID      = domain.ExtensionID("com.amitia.builtin.workspace")
	WorkspaceModuleID         = domain.ModuleID("workspace-runtime")
	WorkspaceReadCapabilityID = capability.CapabilityID("workspace.read")
	WorkspaceWriteCapabilityID = capability.CapabilityID("workspace.write")
	WorkspaceManageCapabilityID = capability.CapabilityID("workspace.manage")
	WorkspaceProviderID       = capability.ProviderID("com.amitia.builtin.workspace.provider")
)

func BuildWorkspaceExtension(version string) Definition {
	ver, err := domain.ParseVersion(version)
	if err != nil {
		ver = domain.SemanticVersion{Major: 0, Minor: 1, Patch: 0}
	}

	extDef := domain.ExtensionDefinition{
		ID:      WorkspaceExtensionID,
		Name:    domain.LocalizedText{Default: "Workspace"},
		Description: domain.LocalizedText{
			Default: "Provides workspace file management capabilities including read, write, and directory operations.",
		},
		Version:         ver,
		ManifestVersion: 1,
		Domain:          domain.ExtensionDomainGeneral,
		Placement:       domain.ExtensionPlacementHybrid,
		Publisher: domain.PublisherReference{
			PublisherID: "com.amitia",
			DisplayName: "Amitia",
			TrustLevel:  "system",
		},
		Package: domain.PackageReference{
			PackageID:       "builtin-workspace",
			ManifestVersion: 1,
		},
		Modules: []domain.ModuleDefinition{
			{
				ID:          WorkspaceModuleID,
				ExtensionID: WorkspaceExtensionID,
				Name:        domain.LocalizedText{Default: "Workspace Runtime"},
				Description: domain.LocalizedText{
					Default: "Built-in module providing workspace file and directory management.",
				},
				Type:    domain.ModuleTypeBuiltin,
				Version: version,
				Runtime: &domain.RuntimeDefinition{
					Type:        domain.RuntimeTypeBuiltin,
					EntryPoint:  "workspace.manage",
					WorkerCount: 2,
				},
				Contributions: buildWorkspaceContributions(WorkspaceExtensionID, WorkspaceModuleID),
				ProvidedCapabilities: []domain.ProvidedCapability{
					{
						ID:      string(WorkspaceReadCapabilityID),
						Version: version,
					},
					{
						ID:      string(WorkspaceWriteCapabilityID),
						Version: version,
					},
					{
						ID:      string(WorkspaceManageCapabilityID),
						Version: version,
					},
				},
				Provider: &domain.ProviderMetadata{
					ID:       string(WorkspaceProviderID),
					Priority: 100,
					Labels: map[string]string{
						"component": "workspace",
					},
				},
				Placement: domain.ModulePlacementCloud,
				Compatibility: domain.ModuleCompatibility{
					Platforms: []string{"windows", "linux", "darwin"},
				},
				Policies: domain.ModulePolicies{
					NetworkAccess:    false,
					FileSystemAccess: true,
				},
			},
		},
		Compatibility: domain.ExtensionCompatibility{
			Platforms: []string{"windows", "linux", "darwin"},
		},
		Policies: domain.ExtensionPolicies{
			FileSystemAccess: true,
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

func buildWorkspaceContributions(extID domain.ExtensionID, modID domain.ModuleID) []domain.ContributionDefinition {
	contributionKinds := []struct {
		id          string
		name        string
		description string
	}{
		{"workspace_list", "Workspace List", "List entries in a workspace directory"},
		{"workspace_stat", "Workspace Stat", "Get metadata of a workspace entry"},
		{"workspace_read", "Workspace Read", "Read content from a workspace file"},
		{"workspace_write", "Workspace Write", "Write content to a workspace file"},
		{"workspace_mkdir", "Workspace Mkdir", "Create a directory in the workspace"},
		{"workspace_rename", "Workspace Rename", "Rename a workspace entry"},
		{"workspace_move", "Workspace Move", "Move a workspace entry within the same mount"},
		{"workspace_copy", "Workspace Copy", "Copy a workspace entry within the same mount"},
		{"workspace_delete", "Workspace Delete", "Delete a workspace entry"},
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
