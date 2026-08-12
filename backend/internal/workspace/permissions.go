package workspace

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

const (
	PermissionWorkspaceRead  = "workspace.read"
	PermissionWorkspaceWrite = "workspace.write"
)

func BuildPermissionDefinitions() []permission.PermissionDefinition {
	return []permission.PermissionDefinition{
		{
			ID:                  PermissionWorkspaceRead,
			Name:                "Read Workspace",
			Description:         "List, stat, and read workspace entries.",
			Category:            permission.CategoryFilesystem,
			RiskLevel:           capability.RiskLow,
			AllowedScopes:       []permission.ScopeType{permission.ScopeGlobal, permission.ScopeCharacter, permission.ScopeResource},
			PersistentGrantable: true,
			RequiresPerUse:      false,
			BackgroundAllowed:   false,
			ChildInvocation:     permission.ChildInherit,
			DefaultApproval:     permission.ApprovalAuto,
		},
		{
			ID:                  PermissionWorkspaceWrite,
			Name:                "Write Workspace",
			Description:         "Write, create, rename, move, copy, and delete workspace entries.",
			Category:            permission.CategoryFilesystem,
			RiskLevel:           capability.RiskHigh,
			AllowedScopes:       []permission.ScopeType{permission.ScopeGlobal, permission.ScopeCharacter, permission.ScopeResource},
			PersistentGrantable: false,
			RequiresPerUse:      true,
			BackgroundAllowed:   false,
			ChildInvocation:     permission.ChildReevaluate,
			DefaultApproval:     permission.ApprovalManual,
		},
	}
}
