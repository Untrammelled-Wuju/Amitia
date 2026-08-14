package kernel

import (
	"github.com/u-ai/backend/internal/extension/kernel/manifest_v2"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

type ManifestPermissionValidator struct {
	registry *permission.PermissionDefinitionRegistry
}

func NewManifestPermissionValidator(registry *permission.PermissionDefinitionRegistry) *ManifestPermissionValidator {
	return &ManifestPermissionValidator{registry: registry}
}

func (v *ManifestPermissionValidator) Validate(manifest manifest_v2.Manifest) []PreviewIssue {
	var issues []PreviewIssue

	declared := make(map[string]bool)
	for _, perm := range manifest.Permissions {
		declared[perm.Name] = true
	}

	for _, perm := range manifest.Permissions {
		if !v.registry.Known(perm.Name) {
			issues = append(issues, PreviewIssue{
				Category: PreviewNotInstallable,
				Code:     "unknown_permission",
				Message:  "unknown permission: " + perm.Name,
				Path:     "permissions[].name",
			})
			continue
		}
		if perm.Scope != "" {
			def, _ := v.registry.Get(perm.Name)
			if !v.scopeAllowed(def.AllowedScopes, perm.Scope) {
				issues = append(issues, PreviewIssue{
					Category: PreviewNotInstallable,
					Code:     "invalid_permission_scope",
					Message:  "permission " + perm.Name + " does not allow scope: " + perm.Scope,
					Path:     "permissions[].scope",
				})
			}
		}
	}

	for _, mod := range manifest.Modules {
		for _, permID := range mod.Runtime.Permissions {
			if !v.registry.Known(permID) {
				issues = append(issues, PreviewIssue{
					Category: PreviewNotInstallable,
					Code:     "unknown_permission",
					Message:  "unknown permission: " + permID,
					Path:     "modules[].runtime.permissions[]",
				})
				continue
			}
			if !declared[permID] {
				issues = append(issues, PreviewIssue{
					Category: PreviewNotInstallable,
					Code:     "permission_not_declared",
					Message:  "runtime permission " + permID + " not declared in package permissions",
					Path:     "modules[].runtime.permissions[]",
				})
			}
		}

		for _, contribution := range mod.Contributions {
			for _, permID := range contribution.RequiredPermissions {
				if !v.registry.Known(permID) {
					issues = append(issues, PreviewIssue{
						Category: PreviewNotInstallable,
						Code:     "unknown_permission",
						Message:  "unknown permission: " + permID,
						Path:     "modules[].contributions[].requiredPermissions[]",
					})
					continue
				}
				if !declared[permID] {
					issues = append(issues, PreviewIssue{
						Category: PreviewNotInstallable,
						Code:     "permission_not_declared",
						Message:  "contribution required permission " + permID + " not declared in package permissions",
						Path:     "modules[].contributions[].requiredPermissions[]",
					})
				}
			}
		}
	}

	return issues
}

func (v *ManifestPermissionValidator) scopeAllowed(allowed []permission.ScopeType, target string) bool {
	for _, s := range allowed {
		if string(s) == target {
			return true
		}
	}
	return false
}

func validateManifestPermissions(manifest manifest_v2.Manifest, registry *permission.PermissionDefinitionRegistry) []PreviewIssue {
	validator := NewManifestPermissionValidator(registry)
	return validator.Validate(manifest)
}
