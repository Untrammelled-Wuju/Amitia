package host_api

import (
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

func PermissionSubjectFromIdentity(id runtime_supervisor.RuntimeIdentity) permission.PermissionSubject {
	extID := string(id.ExtensionID)
	modID := string(id.ModuleID)

	if modID != "" {
		return permission.PermissionSubject{
			Type:        permission.SubjectModule,
			ID:          modID,
			ExtensionID: extID,
			ModuleID:    modID,
		}
	}

	if id.RuntimeType != "" {
		return permission.PermissionSubject{
			Type:        permission.SubjectRuntime,
			ID:          id.InstanceID,
			ExtensionID: extID,
		}
	}

	return permission.PermissionSubject{
		Type:        permission.SubjectExtension,
		ID:          extID,
		ExtensionID: extID,
	}
}

func ScopeSubjectTypeFromIdentity(id runtime_supervisor.RuntimeIdentity) (string, string) {
	extID := string(id.ExtensionID)
	modID := string(id.ModuleID)
	if modID != "" {
		return "module", modID
	}
	return "extension", extID
}
