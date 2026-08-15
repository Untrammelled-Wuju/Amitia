package builtin

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

const (
	WorkspaceExtensionID       = domain.ExtensionID("com.amitia.builtin.workspace")
	WorkspaceModuleID          = domain.ModuleID("workspace-runtime")
	WorkspaceReadCapabilityID  = capability.CapabilityID("workspace.read")
	WorkspaceWriteCapabilityID = capability.CapabilityID("workspace.write")
	WorkspaceManageCapabilityID = capability.CapabilityID("workspace.manage")
	WorkspaceProviderID        = capability.ProviderID("com.amitia.builtin.workspace.provider")
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
					{ID: string(WorkspaceReadCapabilityID), Version: version},
					{ID: string(WorkspaceWriteCapabilityID), Version: version},
					{ID: string(WorkspaceManageCapabilityID), Version: version},
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
		Policies: domain.ExtensionPolicies{},
	}

	return Definition{
		Extension:         extDef,
		SystemManaged:     true,
		Required:          true,
		DisableAllowed:    false,
		BootstrapRevision: 1,
	}
}

type workspaceToolSpec struct {
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

func buildWorkspaceContributions(extID domain.ExtensionID, modID domain.ModuleID) []domain.ContributionDefinition {
	specs := []workspaceToolSpec{
		{
			id:           "workspace_list",
			name:         "Workspace List",
			description:  "List entries in a workspace directory",
			modelName:    "workspace.list",
			handlerName:  "workspace.list",
			inputSchema:  `{"type":"object","properties":{"uri":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":500},"cursor":{"type":"string"}},"required":["uri"],"additionalProperties":false}`,
			outputSchema: `{"type":"object","properties":{"entries":{"type":"array","items":{"type":"object"}},"nextCursor":{"type":"string"},"hasMore":{"type":"boolean"}}}`,
			riskLevel:    "low",
			sideEffect:   "read_only",
			idempotent:   true,
			retryable:    true,
			timeoutMs:    10000,
			permissions:  []map[string]any{{"capability": "workspace.read", "risk": "low"}},
		},
		{
			id:           "workspace_stat",
			name:         "Workspace Stat",
			description:  "Get metadata of a workspace entry",
			modelName:    "workspace.stat",
			handlerName:  "workspace.stat",
			inputSchema:  `{"type":"object","properties":{"uri":{"type":"string"}},"required":["uri"],"additionalProperties":false}`,
			outputSchema: `{"type":"object","properties":{"entry":{"type":"object"}}}`,
			riskLevel:    "low",
			sideEffect:   "read_only",
			idempotent:   true,
			retryable:    true,
			timeoutMs:    5000,
			permissions:  []map[string]any{{"capability": "workspace.read", "risk": "low"}},
		},
		{
			id:           "workspace_read",
			name:         "Workspace Read",
			description:  "Read content from a workspace file",
			modelName:    "workspace.read",
			handlerName:  "workspace.read",
			inputSchema:  `{"type":"object","properties":{"uri":{"type":"string"},"offset":{"type":"integer","minimum":0},"maxBytes":{"type":"integer","minimum":1}},"required":["uri"],"additionalProperties":false}`,
			outputSchema: `{"type":"object","properties":{"content":{"type":"string"},"isText":{"type":"boolean"},"resource":{"type":"string"}}}`,
			riskLevel:    "low",
			sideEffect:   "read_only",
			idempotent:   true,
			retryable:    true,
			timeoutMs:    15000,
			permissions:  []map[string]any{{"capability": "workspace.read", "risk": "low"}},
		},
		{
			id:           "workspace_search",
			name:         "Workspace Search",
			description:  "Search for literal or regex patterns across workspace files",
			modelName:    "workspace.search",
			handlerName:  "workspace.search",
			inputSchema:  `{"type":"object","properties":{"workspaceId":{"type":"string"},"query":{"type":"string"},"regex":{"type":"boolean"},"includeGlobs":{"type":"array","items":{"type":"string"}},"excludeGlobs":{"type":"array","items":{"type":"string"}},"maxResults":{"type":"integer","minimum":1,"maximum":1000},"contextBefore":{"type":"integer","minimum":0,"maximum":10},"contextAfter":{"type":"integer","minimum":0,"maximum":10}},"required":["workspaceId","query"],"additionalProperties":false}`,
			outputSchema: `{"type":"object","properties":{"matches":{"type":"array","items":{"type":"object"}},"total":{"type":"integer"},"truncated":{"type":"boolean"}}}`,
			riskLevel:    "low",
			sideEffect:   "read_only",
			idempotent:   true,
			retryable:    true,
			timeoutMs:    30000,
			permissions:  []map[string]any{{"capability": "workspace.read", "risk": "low"}},
		},
		{
			id:           "workspace_patch",
			name:         "Workspace Patch",
			description:  "Apply a unified-diff patch to a workspace file with integrity verification",
			modelName:    "workspace.patch",
			handlerName:  "workspace.patch",
			inputSchema:  `{"type":"object","properties":{"workspaceId":{"type":"string"},"filePath":{"type":"string"},"baseSha256":{"type":"string"},"patch":{"type":"string"}},"required":["workspaceId","filePath","patch"],"additionalProperties":false}`,
			outputSchema: `{"type":"object","properties":{"applied":{"type":"boolean"},"filePath":{"type":"string"},"newSha256":{"type":"string"}}}`,
			riskLevel:    "medium",
			sideEffect:   "write",
			idempotent:   false,
			retryable:    false,
			timeoutMs:    30000,
			permissions:  []map[string]any{{"capability": "workspace.write", "risk": "medium"}},
		},
		{
			id:           "workspace_diff",
			name:         "Workspace Diff",
			description:  "Compute unified diff between before and after file snapshots",
			modelName:    "workspace.diff",
			handlerName:  "workspace.diff",
			inputSchema:  `{"type":"object","properties":{"workspaceId":{"type":"string"},"beforeFiles":{"type":"object"},"afterFiles":{"type":"object"}},"required":["workspaceId"],"additionalProperties":false}`,
			outputSchema: `{"type":"object","properties":{"changedFiles":{"type":"array","items":{"type":"string"}},"unifiedDiff":{"type":"string"},"additions":{"type":"integer"},"deletions":{"type":"integer"}}}`,
			riskLevel:    "low",
			sideEffect:   "read_only",
			idempotent:   true,
			retryable:    true,
			timeoutMs:    15000,
			permissions:  []map[string]any{{"capability": "workspace.read", "risk": "low"}},
		},
		{
			id:           "workspace_replace",
			name:         "Workspace Replace",
			description:  "Perform exact text replacement in a workspace file with occurrence validation",
			modelName:    "workspace.replace",
			handlerName:  "workspace.replace",
			inputSchema:  `{"type":"object","properties":{"workspaceId":{"type":"string"},"filePath":{"type":"string"},"oldText":{"type":"string"},"newText":{"type":"string"},"expectedOccurrences":{"type":"integer","minimum":0}},"required":["workspaceId","filePath","oldText"],"additionalProperties":false}`,
			outputSchema: `{"type":"object","properties":{"replaced":{"type":"boolean"},"actualOccurrences":{"type":"integer"},"filePath":{"type":"string"}}}`,
			riskLevel:    "medium",
			sideEffect:   "write",
			idempotent:   false,
			retryable:    false,
			timeoutMs:    30000,
			permissions:  []map[string]any{{"capability": "workspace.write", "risk": "medium"}},
		},
		{
			id:           "workspace_write",
			name:         "Workspace Write",
			description:  "Write content to a workspace file",
			modelName:    "workspace.write",
			handlerName:  "workspace.write",
			inputSchema:  `{"type":"object","properties":{"uri":{"type":"string"},"text":{"type":"string"},"sourceUri":{"type":"string"},"overwrite":{"type":"boolean"}},"required":["uri"],"additionalProperties":false}`,
			outputSchema: `{"type":"object","properties":{"entry":{"type":"object"}}}`,
			riskLevel:    "medium",
			sideEffect:   "write",
			idempotent:   false,
			retryable:    false,
			timeoutMs:    30000,
			permissions:  []map[string]any{{"capability": "workspace.write", "risk": "medium"}},
		},
		{
			id:           "workspace_mkdir",
			name:         "Workspace Mkdir",
			description:  "Create a directory in the workspace",
			modelName:    "workspace.mkdir",
			handlerName:  "workspace.mkdir",
			inputSchema:  `{"type":"object","properties":{"uri":{"type":"string"}},"required":["uri"],"additionalProperties":false}`,
			outputSchema: `{"type":"object","properties":{"entry":{"type":"object"}}}`,
			riskLevel:    "medium",
			sideEffect:   "write",
			idempotent:   false,
			retryable:    false,
			timeoutMs:    10000,
			permissions:  []map[string]any{{"capability": "workspace.write", "risk": "medium"}},
		},
		{
			id:           "workspace_rename",
			name:         "Workspace Rename",
			description:  "Rename a workspace entry",
			modelName:    "workspace.rename",
			handlerName:  "workspace.rename",
			inputSchema:  `{"type":"object","properties":{"uri":{"type":"string"},"newName":{"type":"string"}},"required":["uri","newName"],"additionalProperties":false}`,
			outputSchema: `{"type":"object","properties":{"entry":{"type":"object"}}}`,
			riskLevel:    "medium",
			sideEffect:   "write",
			idempotent:   false,
			retryable:    false,
			timeoutMs:    10000,
			permissions:  []map[string]any{{"capability": "workspace.write", "risk": "medium"}},
		},
		{
			id:           "workspace_move",
			name:         "Workspace Move",
			description:  "Move a workspace entry within the same mount",
			modelName:    "workspace.move",
			handlerName:  "workspace.move",
			inputSchema:  `{"type":"object","properties":{"sourceUri":{"type":"string"},"destinationDirUri":{"type":"string"}},"required":["sourceUri","destinationDirUri"],"additionalProperties":false}`,
			outputSchema: `{"type":"object","properties":{"entry":{"type":"object"}}}`,
			riskLevel:    "medium",
			sideEffect:   "write",
			idempotent:   false,
			retryable:    false,
			timeoutMs:    15000,
			permissions:  []map[string]any{{"capability": "workspace.write", "risk": "medium"}},
		},
		{
			id:           "workspace_copy",
			name:         "Workspace Copy",
			description:  "Copy a workspace entry within the same mount",
			modelName:    "workspace.copy",
			handlerName:  "workspace.copy",
			inputSchema:  `{"type":"object","properties":{"sourceUri":{"type":"string"},"destinationDirUri":{"type":"string"}},"required":["sourceUri","destinationDirUri"],"additionalProperties":false}`,
			outputSchema: `{"type":"object","properties":{"entry":{"type":"object"}}}`,
			riskLevel:    "medium",
			sideEffect:   "write",
			idempotent:   false,
			retryable:    false,
			timeoutMs:    15000,
			permissions:  []map[string]any{{"capability": "workspace.write", "risk": "medium"}},
		},
		{
			id:           "workspace_delete",
			name:         "Workspace Delete",
			description:  "Delete a workspace entry",
			modelName:    "workspace.delete",
			handlerName:  "workspace.delete",
			inputSchema:  `{"type":"object","properties":{"uri":{"type":"string"},"recursive":{"type":"boolean"}},"required":["uri"],"additionalProperties":false}`,
			outputSchema: `{"type":"object","properties":{"deleted":{"type":"boolean"}}}`,
			riskLevel:    "high",
			sideEffect:   "write",
			idempotent:   false,
			retryable:    false,
			timeoutMs:    15000,
			permissions:  []map[string]any{{"capability": "workspace.write", "risk": "high"}},
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
				"capabilityId": resolveWorkspaceCapabilityID(s.id),
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
					"runtimeType": "workspace",
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

func resolveWorkspaceCapabilityID(toolID string) string {
	switch toolID {
	case "workspace_read", "workspace_stat", "workspace_list":
		return string(WorkspaceReadCapabilityID)
	case "workspace_write", "workspace_mkdir", "workspace_rename", "workspace_move", "workspace_copy", "workspace_delete":
		return string(WorkspaceWriteCapabilityID)
	}
	return string(WorkspaceManageCapabilityID)
}
