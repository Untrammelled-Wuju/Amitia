package kernel

import (
	"context"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

// repositoryPermissionTrustChecker binds permission.TrustedOnly decisions to
// the authoritative, installed extension definition. Manifest-provided trust
// labels are normalized during package verification/install before they reach
// the definition repository, so a plugin cannot self-assert trusted status.
type repositoryPermissionTrustChecker struct {
	installations domain.InstallationRepository
	definitions   domain.DefinitionRepository
}

func newRepositoryPermissionTrustChecker(
	installations domain.InstallationRepository,
	definitions domain.DefinitionRepository,
) *repositoryPermissionTrustChecker {
	return &repositoryPermissionTrustChecker{installations: installations, definitions: definitions}
}

func (c *repositoryPermissionTrustChecker) IsTrusted(subject permission.PermissionSubject) bool {
	if c == nil || c.installations == nil || c.definitions == nil {
		return false
	}
	extensionID := strings.TrimSpace(subject.ExtensionID)
	if extensionID == "" && subject.Type == permission.SubjectExtension {
		extensionID = strings.TrimSpace(subject.ID)
	}
	if extensionID == "" {
		return false
	}

	ctx := context.Background()
	inst, err := c.installations.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil || inst.InstallationState != domain.InstallationStateInstalled {
		return false
	}
	def, err := c.definitions.GetExtension(ctx, domain.ExtensionID(extensionID), inst.InstalledVersion)
	if err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(def.Publisher.TrustLevel)) {
	case "system", "official", "trusted", "user_trusted":
		return true
	default:
		return false
	}
}

var _ permission.TrustLevelChecker = (*repositoryPermissionTrustChecker)(nil)
