package permission

import (
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
)

type UIPermissionChecker struct{}

func NewUIPermissionChecker() *UIPermissionChecker {
	return &UIPermissionChecker{}
}

func (c *UIPermissionChecker) CheckContributionPermissions(def *ui_contribution.UIContributionDefinition, grantedPerms []string) (missing []string, err error) {
	missing = make([]string, 0)
	if def == nil {
		return missing, nil
	}
	granted := make(map[string]bool, len(grantedPerms))
	for _, p := range grantedPerms {
		granted[p] = true
	}
	for _, req := range def.Permissions {
		if req.Required && !granted[req.Name] {
			missing = append(missing, req.Name)
		}
	}
	return missing, nil
}

func (c *UIPermissionChecker) CheckContributionScopes(def *ui_contribution.UIContributionDefinition, grantedScopes []string) (missing []string, err error) {
	missing = make([]string, 0)
	if def == nil {
		return missing, nil
	}
	if len(def.ScopeRule.RequiredScopes) == 0 {
		return missing, nil
	}
	granted := make(map[string]bool, len(grantedScopes))
	for _, s := range grantedScopes {
		granted[s] = true
	}
	for _, scope := range def.ScopeRule.RequiredScopes {
		if !granted[scope] {
			missing = append(missing, scope)
		}
	}
	return missing, nil
}

func (c *UIPermissionChecker) ValidateSessionRequest(def *ui_contribution.UIContributionDefinition, grantedScopes, grantedPerms []string) error {
	missingPerms, _ := c.CheckContributionPermissions(def, grantedPerms)
	if len(missingPerms) > 0 {
		return fmt.Errorf("ui_contribution: missing required permissions: %v", missingPerms)
	}
	missingScopes, _ := c.CheckContributionScopes(def, grantedScopes)
	if len(missingScopes) > 0 {
		return fmt.Errorf("ui_contribution: missing required scopes: %v", missingScopes)
	}
	return nil
}
