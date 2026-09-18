package kernel

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/manifest_v1"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

func newInstallationPermissionPolicy(repo sqlite.PermissionRepository) func(context.Context, permission.PermissionSubject, permission.PermissionRequirement, permission.PermissionDefinition) (permission.PermissionDecision, bool) {
	return func(ctx context.Context, subject permission.PermissionSubject, requirement permission.PermissionRequirement, _ permission.PermissionDefinition) (permission.PermissionDecision, bool) {
		extensionID := installationPermissionExtensionID(subject)
		if extensionID == "" {
			return "", false
		}
		if repo == nil {
			return permission.DecisionDeny, true
		}
		granted, err := repo.IsGranted(ctx, domain.ExtensionID(extensionID), requirement.PermissionID)
		if err != nil || !granted {
			return permission.DecisionDeny, true
		}
		return permission.DecisionAllow, true
	}
}

func installationPermissionExtensionID(subject permission.PermissionSubject) string {
	switch subject.Type {
	case permission.SubjectExtension, permission.SubjectModule, permission.SubjectRuntime:
	default:
		return ""
	}
	if strings.TrimSpace(subject.ExtensionID) != "" {
		extensionID := strings.TrimSpace(subject.ExtensionID)
		if strings.HasPrefix(extensionID, "com.amitia.builtin.") {
			return ""
		}
		return extensionID
	}
	if subject.Type == permission.SubjectExtension {
		extensionID := strings.TrimSpace(subject.ID)
		if strings.HasPrefix(extensionID, "com.amitia.builtin.") {
			return ""
		}
		return extensionID
	}
	return ""
}

func (r *Runtime) syncInstalledPackagePermissions(ctx context.Context, extensionID domain.ExtensionID, permissions []manifest_v1.PermissionReq) error {
	if r == nil || r.container == nil || r.container.PermissionRepository == nil {
		return fmt.Errorf("kernel: permission repository unavailable")
	}
	specs := make([]sqlite.PermissionRequirement, 0, len(permissions))
	for _, item := range permissions {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		specs = append(specs, sqlite.PermissionRequirement{
			ExtensionID:    extensionID,
			PermissionName: name,
			Reason:         strings.TrimSpace(item.Reason),
			Required:       item.Required,
			Scope:          strings.TrimSpace(item.Scope),
		})
	}
	return persistInstalledPackagePermissions(ctx, r.container.PermissionRepository, extensionID, specs)
}

func (r *Runtime) restoreInstalledPackagePermissions(ctx context.Context, extensionID domain.ExtensionID, definition domain.ExtensionDefinition) error {
	if r == nil || r.container == nil || r.container.PermissionRepository == nil {
		return fmt.Errorf("kernel: permission repository unavailable")
	}
	return restorePackagePermissionsFromDefinition(ctx, r.container.PermissionRepository, extensionID, definition)
}

func restorePackagePermissionsFromDefinition(ctx context.Context, repo sqlite.PermissionRepository, extensionID domain.ExtensionID, definition domain.ExtensionDefinition) error {
	if repo == nil {
		return fmt.Errorf("kernel: permission repository unavailable")
	}
	specs, err := repo.ListRequirements(ctx, extensionID)
	if err != nil {
		return err
	}
	specs = normalizeInstalledPermissionRequirements(extensionID, append(specs, installedPermissionRequirementsFromDefinition(extensionID, definition)...))
	return restoreInstalledPackagePermissions(ctx, repo, extensionID, specs)
}

func restoreInstalledPackagePermissions(ctx context.Context, repo sqlite.PermissionRepository, extensionID domain.ExtensionID, requirements []sqlite.PermissionRequirement) error {
	normalized := normalizeInstalledPermissionRequirements(extensionID, requirements)
	existingGrants, err := repo.ListGrants(ctx, extensionID)
	if err != nil {
		return err
	}
	stateByName := make(map[string]struct{}, len(existingGrants))
	for _, grant := range existingGrants {
		stateByName[grant.PermissionName] = struct{}{}
	}
	grantedAt := time.Now().UTC()
	for _, requirement := range normalized {
		if err := repo.PutRequirement(ctx, requirement); err != nil {
			return err
		}
		if _, exists := stateByName[requirement.PermissionName]; exists {
			continue
		}
		if err := repo.PutGrant(ctx, sqlite.PermissionGrant{
			ExtensionID:    extensionID,
			PermissionName: requirement.PermissionName,
			State:          "granted",
			GrantedAt:      grantedAt,
		}); err != nil {
			return err
		}
	}
	return nil
}

func persistInstalledPackagePermissions(ctx context.Context, repo sqlite.PermissionRepository, extensionID domain.ExtensionID, requirements []sqlite.PermissionRequirement) error {
	if repo == nil {
		return fmt.Errorf("kernel: permission repository unavailable")
	}
	normalized := normalizeInstalledPermissionRequirements(extensionID, requirements)
	grantedAt := time.Now().UTC()
	for _, requirement := range normalized {
		if err := repo.PutRequirement(ctx, requirement); err != nil {
			return err
		}
		if err := repo.PutGrant(ctx, sqlite.PermissionGrant{
			ExtensionID:    extensionID,
			PermissionName: requirement.PermissionName,
			State:          "granted",
			GrantedAt:      grantedAt,
		}); err != nil {
			return err
		}
	}
	return nil
}

func normalizeInstalledPermissionRequirements(extensionID domain.ExtensionID, requirements []sqlite.PermissionRequirement) []sqlite.PermissionRequirement {
	byName := make(map[string]sqlite.PermissionRequirement, len(requirements))
	for _, requirement := range requirements {
		name := strings.TrimSpace(requirement.PermissionName)
		if name == "" {
			continue
		}
		requirement.ExtensionID = extensionID
		requirement.PermissionName = name
		requirement.Reason = strings.TrimSpace(requirement.Reason)
		requirement.Scope = strings.TrimSpace(requirement.Scope)
		if existing, ok := byName[name]; ok {
			existing.Required = existing.Required || requirement.Required
			if existing.Reason == "" {
				existing.Reason = requirement.Reason
			}
			if existing.Scope == "" {
				existing.Scope = requirement.Scope
			}
			byName[name] = existing
			continue
		}
		byName[name] = requirement
	}
	result := make([]sqlite.PermissionRequirement, 0, len(byName))
	for _, requirement := range byName {
		result = append(result, requirement)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].PermissionName < result[j].PermissionName
	})
	return result
}

func installedPermissionRequirementsFromDefinition(extensionID domain.ExtensionID, definition domain.ExtensionDefinition) []sqlite.PermissionRequirement {
	requirements := make([]sqlite.PermissionRequirement, 0)
	for _, module := range definition.Modules {
		if module.Runtime != nil {
			for _, permissionID := range module.Runtime.Permissions {
				requirements = append(requirements, sqlite.PermissionRequirement{
					ExtensionID:    extensionID,
					PermissionName: permissionID,
					Required:       true,
					Scope:          string(permission.ScopeExtension),
				})
			}
		}
		for _, contribution := range module.Contributions {
			for _, permissionID := range contribution.RequiredPermissions {
				requirements = append(requirements, sqlite.PermissionRequirement{
					ExtensionID:    extensionID,
					PermissionName: permissionID,
					Required:       true,
					Scope:          string(permission.ScopeExtension),
				})
			}
		}
	}
	if requirement, ok := uiToolInvocationRequirement(extensionID, definition.Modules); ok {
		requirements = append(requirements, requirement)
	}
	if requirement, ok := serviceRuntimeExecutionRequirement(extensionID, definition.Modules); ok {
		requirements = append(requirements, requirement)
	}
	return normalizeInstalledPermissionRequirements(extensionID, requirements)
}

func uiToolInvocationRequirement(extensionID domain.ExtensionID, modules []domain.ModuleDefinition) (sqlite.PermissionRequirement, bool) {
	for _, module := range modules {
		for _, contribution := range module.Contributions {
			switch contribution.Kind {
			case domain.ContributionKindUIPage, domain.ContributionKindUIPanel, domain.ContributionKindUIChat, domain.ContributionKindUIContextAction, domain.ContributionKindUIDesktop:
			default:
				continue
			}
			actions, ok := contribution.Definition["actions"].([]any)
			if !ok {
				continue
			}
			for _, rawAction := range actions {
				action, ok := rawAction.(map[string]any)
				if !ok {
					continue
				}
				target, ok := action["target"].(map[string]any)
				if !ok {
					continue
				}
				targetType, _ := target["type"].(string)
				if strings.EqualFold(strings.TrimSpace(targetType), "tool") {
					return sqlite.PermissionRequirement{
						ExtensionID:    extensionID,
						PermissionName: "tool.invoke",
						Reason:         "Required to invoke tools declared by the extension UI actions.",
						Required:       true,
						Scope:          string(permission.ScopeExtension),
					}, true
				}
			}
		}
	}
	return sqlite.PermissionRequirement{}, false
}

func serviceRuntimeExecutionRequirement(extensionID domain.ExtensionID, modules []domain.ModuleDefinition) (sqlite.PermissionRequirement, bool) {
	for _, module := range modules {
		if module.Runtime == nil {
			continue
		}
		switch module.Runtime.Type {
		case domain.RuntimeTypeJavaScript, domain.RuntimeTypeService, domain.RuntimeTypeGo:
			return sqlite.PermissionRequirement{
				ExtensionID:    extensionID,
				PermissionName: permission.PermissionServiceRuntimeExecute,
				Reason:         "Required to launch the extension service runtime.",
				Required:       true,
				Scope:          string(permission.ScopeExtension),
			}, true
		}
	}
	return sqlite.PermissionRequirement{}, false
}

func packageManifestGrantRecords(extensionID domain.ExtensionID, requirements []sqlite.PermissionRequirement) []sqlite.PermissionGrant {
	normalized := normalizeInstalledPermissionRequirements(extensionID, requirements)
	grantedAt := time.Now().UTC()
	result := make([]sqlite.PermissionGrant, 0, len(normalized))
	for _, requirement := range normalized {
		result = append(result, sqlite.PermissionGrant{
			ExtensionID:    extensionID,
			PermissionName: requirement.PermissionName,
			State:          "granted",
			GrantedAt:      grantedAt,
		})
	}
	return result
}
