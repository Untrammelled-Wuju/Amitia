package extension_detail

import (
	"context"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/package_distribution"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

type VersionReader interface {
	ListVersionDetails(ctx context.Context, extensionID string) ([]VersionDetail, error)
}

type UpdateChecker interface {
	Check(ctx context.Context, extensionID, currentVersion string) (*package_distribution.ExtensionUpdateMetadata, error)
}

type KernelDetailProvider struct {
	Definitions           domain.DefinitionRepository
	Installations         domain.InstallationRepository
	Modules               sqlite.ModuleRepository
	Contributions         sqlite.ContributionRepository
	Permissions           sqlite.PermissionRepository
	PermissionDefinitions *permission.PermissionDefinitionRegistry
	Runtimes              domain.RuntimeRepository
	Versions              VersionReader
	Updates               UpdateChecker
}

func (p *KernelDetailProvider) GetExtension(ctx context.Context, extensionID string) (*ExtensionDetail, error) {
	if p == nil || p.Definitions == nil || p.Installations == nil {
		return nil, ErrDetailNotFound
	}
	inst, err := p.Installations.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return nil, fmt.Errorf("extension_detail: get installation %s: %w", extensionID, err)
	}
	def, err := p.Definitions.GetExtension(ctx, inst.ExtensionID, inst.InstalledVersion)
	if err != nil {
		return nil, fmt.Errorf("extension_detail: get definition %s@%s: %w", extensionID, inst.InstalledVersion, err)
	}

	detail := &ExtensionDetail{
		ExtensionID:       extensionID,
		Publisher:         def.Publisher.PublisherID,
		DisplayName:       def.Name.Default,
		Description:       def.Description.Default,
		Version:           inst.InstalledVersion.String(),
		Icon:              metadataString(def.Metadata, "icon"),
		Trust:             normalizeTrust(def.Publisher.TrustLevel),
		SignatureStatus:   def.Package.Signature.Status,
		Source:            metadataString(inst.Metadata, "source"),
		Enabled:           inst.EnablementState == domain.EnablementEnabled,
		EffectiveState:    string(inst.InstallationState),
		FailureCount:      metadataInt(inst.Metadata, "failureCount"),
		Platforms:         append([]string(nil), def.Compatibility.Platforms...),
		SystemManaged:     metadataBool(def.Metadata, "system.managed") || strings.HasPrefix(extensionID, "com.amitia.builtin."),
		DevMode:           metadataBool(inst.Metadata, "devOnly") || metadataBool(def.Metadata, "devOnly"),
		UserModified:      metadataBool(inst.Metadata, "user.modified") || metadataBool(def.Metadata, "user.modified"),
		Modules:           []ModuleDetail{},
		Contributions:     []ContributionDetail{},
		Permissions:       []PermissionDetail{},
		Scopes:            []ScopeDetail{},
		Runtimes:          []RuntimeDetail{},
		Storage:           []StorageNamespaceDetail{},
		Resources:         []ResourceDetail{},
		UIContributions:   []UIContributionDetail{},
		Versions:          []VersionDetail{},
		RecentLogs:        []LogEntry{},
		RecentInvocations: []InvocationDetail{},
		LifecycleEvents:   []LifecycleDetail{},
		Actions:           []ActionDescriptor{},
	}
	if detail.Source == "" {
		detail.Source = metadataString(def.Metadata, "source")
	}
	if detail.Source == "" {
		detail.Source = "installed"
	}
	if !inst.InstalledAt.IsZero() {
		t := inst.InstalledAt
		detail.InstalledAt = &t
	}
	if !inst.UpdatedAt.IsZero() {
		t := inst.UpdatedAt
		detail.UpdatedAt = &t
	}

	if err := p.populateModules(ctx, detail, def); err != nil {
		return nil, err
	}
	if err := p.populatePermissions(ctx, detail, def); err != nil {
		return nil, err
	}
	if err := p.populateVersions(ctx, detail); err != nil {
		return nil, err
	}
	if err := p.populateRuntimes(ctx, detail); err != nil {
		return nil, err
	}
	p.populateUpdate(ctx, detail)
	detail.PermissionRisk = highestPermissionRisk(detail.Permissions)
	detail.Actions = detailActions(detail)
	return detail, nil
}

func (p *KernelDetailProvider) populateModules(ctx context.Context, detail *ExtensionDetail, def domain.ExtensionDefinition) error {
	if p.Modules != nil {
		modules, err := p.Modules.ListModules(ctx, domain.ExtensionID(detail.ExtensionID))
		if err != nil {
			return fmt.Errorf("extension_detail: list modules %s: %w", detail.ExtensionID, err)
		}
		for _, mod := range modules {
			runtimeType := ""
			entryPoint := ""
			if mod.Runtime != nil {
				runtimeType = string(mod.Runtime.Type)
				entryPoint = mod.Runtime.EntryPoint
			}
			detail.Modules = append(detail.Modules, ModuleDetail{
				ModuleID:    string(mod.ID),
				Kind:        string(mod.Type),
				Entry:       entryPoint,
				Runtime:     runtimeType,
				DisplayName: mod.Name.Default,
				Description: mod.Description.Default,
				Enabled:     true,
				Status:      "registered",
			})
		}
	}
	if len(detail.Modules) == 0 {
		for _, mod := range def.Modules {
			runtimeType := ""
			entryPoint := ""
			if mod.Runtime != nil {
				runtimeType = string(mod.Runtime.Type)
				entryPoint = mod.Runtime.EntryPoint
			}
			detail.Modules = append(detail.Modules, ModuleDetail{
				ModuleID:    string(mod.ID),
				Kind:        string(mod.Type),
				Entry:       entryPoint,
				Runtime:     runtimeType,
				DisplayName: mod.Name.Default,
				Description: mod.Description.Default,
				Enabled:     true,
				Status:      "defined",
			})
		}
	}
	if p.Contributions == nil {
		return nil
	}
	contributions, err := p.Contributions.ListContributions(ctx, domain.ExtensionID(detail.ExtensionID))
	if err != nil {
		return fmt.Errorf("extension_detail: list contributions %s: %w", detail.ExtensionID, err)
	}
	for _, contribution := range contributions {
		detail.Contributions = append(detail.Contributions, ContributionDetail{
			ContributionID: string(contribution.ID),
			ModuleID:       string(contribution.ModuleID),
			Kind:           string(contribution.Kind),
			Title:          contribution.Name.Default,
			Description:    contribution.Description.Default,
			Enabled:        true,
			Exposure:       contributionExposure(contribution.Exposure),
			Deprecated:     metadataBool(contribution.Metadata, "deprecated"),
		})
	}
	return nil
}

func (p *KernelDetailProvider) populatePermissions(ctx context.Context, detail *ExtensionDetail, def domain.ExtensionDefinition) error {
	if p.Permissions == nil {
		return nil
	}
	requirements, err := p.Permissions.ListRequirements(ctx, domain.ExtensionID(detail.ExtensionID))
	if err != nil {
		return fmt.Errorf("extension_detail: list permission requirements %s: %w", detail.ExtensionID, err)
	}
	grants, err := p.Permissions.ListGrants(ctx, domain.ExtensionID(detail.ExtensionID))
	if err != nil {
		return fmt.Errorf("extension_detail: list permission grants %s: %w", detail.ExtensionID, err)
	}
	granted := make(map[string]bool, len(grants))
	for _, grant := range grants {
		granted[grant.PermissionName] = grant.State == "granted"
	}
	for _, requirement := range requirements {
		permissionDetail := PermissionDetail{
			Permission: requirement.PermissionName,
			Required:   requirement.Required,
			Granted:    granted[requirement.PermissionName],
			Reason:     requirement.Reason,
			Scope:      requirement.Scope,
		}
		if p.PermissionDefinitions != nil {
			if definition, ok := p.PermissionDefinitions.Get(requirement.PermissionName); ok {
				permissionDetail.Risk = string(definition.RiskLevel)
			}
		}
		detail.Permissions = append(detail.Permissions, permissionDetail)
	}
	if len(detail.Permissions) == 0 {
		seen := make(map[string]bool)
		for _, mod := range def.Modules {
			for _, contribution := range mod.Contributions {
				for _, permissionID := range contribution.RequiredPermissions {
					if permissionID == "" || seen[permissionID] {
						continue
					}
					seen[permissionID] = true
					permissionDetail := PermissionDetail{Permission: permissionID}
					if p.PermissionDefinitions != nil {
						if definition, ok := p.PermissionDefinitions.Get(permissionID); ok {
							permissionDetail.Risk = string(definition.RiskLevel)
						}
					}
					detail.Permissions = append(detail.Permissions, permissionDetail)
				}
			}
		}
	}
	return nil
}

func (p *KernelDetailProvider) populateVersions(ctx context.Context, detail *ExtensionDetail) error {
	if p.Versions == nil {
		return nil
	}
	versions, err := p.Versions.ListVersionDetails(ctx, detail.ExtensionID)
	if err != nil {
		return fmt.Errorf("extension_detail: list versions %s: %w", detail.ExtensionID, err)
	}
	detail.Versions = versions
	return nil
}

func (p *KernelDetailProvider) populateRuntimes(ctx context.Context, detail *ExtensionDetail) error {
	if p.Runtimes == nil {
		return nil
	}
	instances, err := p.Runtimes.ListInstances(ctx, domain.ExtensionID(detail.ExtensionID))
	if err != nil {
		return fmt.Errorf("extension_detail: list runtimes %s: %w", detail.ExtensionID, err)
	}
	for _, instance := range instances {
		detail.Runtimes = append(detail.Runtimes, RuntimeDetail{
			RuntimeID:  string(instance.RuntimeID),
			Kind:       string(instance.RuntimeType),
			Status:     instance.ActualState,
			StartedAt:  instance.StartedAt,
			Generation: int(instance.Generation),
		})
	}
	return nil
}

func contributionExposure(exposure domain.Exposure) string {
	if exposure.HiddenFromDiscovery {
		return "hidden"
	}
	if exposure.VisibleByDefault {
		return "visible"
	}
	return ""
}

func (p *KernelDetailProvider) populateUpdate(ctx context.Context, detail *ExtensionDetail) {
	if p.Updates == nil || detail.SystemManaged {
		return
	}
	update, err := p.Updates.Check(ctx, detail.ExtensionID, detail.Version)
	if err != nil {
		detail.UpdateError = err.Error()
		return
	}
	if update == nil {
		return
	}
	detail.UpdateAvailable = true
	detail.LatestVersion = update.Version
}

func detailActions(detail *ExtensionDetail) []ActionDescriptor {
	actions := []ActionDescriptor{
		{ActionID: "enable", Title: "启用", Kind: "command", Available: !detail.Enabled && detail.EffectiveState != string(domain.InstallationStateFailed)},
		{ActionID: "disable", Title: "停用", Kind: "command", Available: detail.Enabled},
		{ActionID: "permissions", Title: "权限管理", Kind: "dialog", Available: true},
		{ActionID: "update", Title: "检查更新", Kind: "command", Available: true},
		{ActionID: "rollback", Title: "版本回滚", Kind: "navigation", Available: len(detail.Versions) > 1},
		{ActionID: "uninstall", Title: "卸载", Kind: "command", Available: !detail.SystemManaged},
		{ActionID: "diagnose", Title: "诊断", Kind: "navigation", Available: true},
	}
	if detail.LatestVersion != "" {
		actions[3].Available = true
	}
	if detail.EffectiveState == string(domain.InstallationStateFailed) {
		actions[0].Reason = "扩展处于失败状态，需先诊断或恢复"
	}
	return actions
}

func highestPermissionRisk(permissions []PermissionDetail) string {
	risk := ""
	for _, permission := range permissions {
		if riskRank(permission.Risk) > riskRank(risk) {
			risk = permission.Risk
		}
	}
	if risk == "" {
		return "low"
	}
	return risk
}

func riskRank(value string) int {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func normalizeTrust(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "system", "official", "trusted", "user_trusted", "development", "blocked", "revoked", "community", "verified", "untrusted":
		return normalized
	default:
		return "untrusted"
	}
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func metadataBool(metadata map[string]any, key string) bool {
	if metadata == nil {
		return false
	}
	value, _ := metadata[key].(bool)
	return value
}

func metadataInt(metadata map[string]any, key string) int {
	if metadata == nil {
		return 0
	}
	switch value := metadata[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}
