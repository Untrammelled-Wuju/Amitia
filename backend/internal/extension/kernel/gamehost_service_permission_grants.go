package kernel

import (
	"context"
	"log"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

func (c *Container) ensureGameHostServicePermissionGrants(
	ctx context.Context,
	extensionID domain.ExtensionID,
	gameHostOwned map[domain.ModuleID]struct{},
	modules []domain.ModuleDefinition,
) {
	if c == nil || c.PermissionBroker == nil || len(gameHostOwned) == 0 {
		return
	}
	trustChecker := &repositoryPermissionTrustChecker{
		installations: c.InstallationRepository,
		definitions:   c.DefinitionRepository,
	}
	subject := permission.SubjectForExtension(string(extensionID))
	if !trustChecker.IsTrusted(subject) {
		return
	}
	for _, mod := range modules {
		if !isGameHostOwnedRuntimeModule(gameHostOwned, mod.ID) {
			continue
		}
		if mod.Runtime == nil {
			continue
		}
		scope := permission.PermissionScope{Type: permission.ScopeModule, ID: string(mod.ID)}
		permIDs := []string{permission.PermissionServiceRuntimeExecute}
		for _, declared := range mod.Runtime.Permissions {
			if declared == permission.PermissionServiceNetworkRequest {
				permIDs = append(permIDs, declared)
				break
			}
		}
		for _, permID := range permIDs {
			c.ensureServicePermissionGrant(ctx, subject, permID, scope, extensionID, mod.ID)
		}
	}
}

func (c *Container) ensureServicePermissionGrant(
	ctx context.Context,
	subject permission.PermissionSubject,
	permID string,
	scope permission.PermissionScope,
	extensionID domain.ExtensionID,
	moduleID domain.ModuleID,
) {
	if c.PermissionDefinitions != nil {
		if _, ok := c.PermissionDefinitions.Get(permID); !ok {
			return
		}
	}
	active, err := c.PermissionBroker.ListGrants(ctx, permission.PermissionGrantFilter{
		Subject:      &subject,
		PermissionID: permID,
		ActiveOnly:   true,
	})
	if err != nil {
		log.Printf("[kernel] list gamehost service grants failed: extension=%s permission=%s err=%v", extensionID, permID, err)
		return
	}
	for _, grant := range active {
		if grant.Scope.Contains(scope) {
			return
		}
	}
	if _, err := c.PermissionBroker.Grant(ctx, permission.PermissionGrantRequest{
		Subject:      subject,
		PermissionID: permID,
		Scope:        scope,
		Decision:     permission.DecisionAllowPersistent,
		IssuedBy:     permission.IssuerSystem,
		Reason:       "gamehost module declared service permission",
	}); err != nil {
		log.Printf("[kernel] grant gamehost service permission failed: extension=%s module=%s permission=%s err=%v", extensionID, moduleID, permID, err)
	}
}
