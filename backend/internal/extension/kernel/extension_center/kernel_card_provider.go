package extension_center

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/package_distribution"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

type UpdateChecker interface {
	Check(ctx context.Context, extensionID, currentVersion string) (*package_distribution.ExtensionUpdateMetadata, error)
}

type VersionAvailabilityReader interface {
	HasRollbackVersion(ctx context.Context, extensionID, currentVersion string) (bool, error)
}

type CardProviderDependencies struct {
	Updates               UpdateChecker
	Versions              VersionAvailabilityReader
	Permissions           sqlite.PermissionRepository
	PermissionDefinitions *permission.PermissionDefinitionRegistry
}

type KernelCardProvider struct {
	definitions   domain.DefinitionRepository
	installations domain.InstallationRepository
	dependencies  CardProviderDependencies
}

func NewKernelCardProvider(
	definitions domain.DefinitionRepository,
	installations domain.InstallationRepository,
	dependencies ...CardProviderDependencies,
) *KernelCardProvider {
	provider := &KernelCardProvider{
		definitions:   definitions,
		installations: installations,
	}
	if len(dependencies) > 0 {
		provider.dependencies = dependencies[0]
	}
	return provider
}

func (p *KernelCardProvider) SetDependencies(dependencies CardProviderDependencies) {
	if p == nil {
		return
	}
	p.dependencies = dependencies
}

func (p *KernelCardProvider) ListCards(ctx context.Context) ([]ExtensionCard, error) {
	if p.definitions == nil || p.installations == nil {
		return []ExtensionCard{}, nil
	}

	defs, err := p.definitions.ListExtensions(ctx)
	if err != nil {
		return nil, err
	}
	instMap, err := p.installationMap(ctx)
	if err != nil {
		return nil, err
	}

	defsByExtension := make(map[string][]domain.ExtensionDefinition)
	for _, def := range defs {
		defsByExtension[string(def.ID)] = append(defsByExtension[string(def.ID)], def)
	}

	cards := make([]ExtensionCard, 0, len(defsByExtension))
	for extensionID, versions := range defsByExtension {
		inst, installed := instMap[extensionID]
		def, ok := selectDefinition(versions, inst, installed)
		if !ok {
			continue
		}
		target, targetErr := domain.ManagementTargetForDomain(domain.NormalizeExtensionDomain(def.Domain))
		if targetErr != nil || target != domain.ManagementTargetExtensionCenter {
			continue
		}

		card := p.cardFromDefinition(def)
		if installed {
			card = p.populateInstalled(ctx, card, def, inst)
		}
		cards = append(cards, card)
	}

	p.checkUpdates(ctx, cards)
	sort.SliceStable(cards, func(i, j int) bool {
		return cards[i].ExtensionID < cards[j].ExtensionID
	})
	return cards, nil
}

func selectDefinition(versions []domain.ExtensionDefinition, inst domain.ExtensionInstallation, installed bool) (domain.ExtensionDefinition, bool) {
	if len(versions) == 0 {
		return domain.ExtensionDefinition{}, false
	}
	if installed {
		for _, def := range versions {
			if def.Version.Compare(inst.InstalledVersion) == 0 {
				return def, true
			}
		}
	}
	selected := versions[0]
	for _, def := range versions[1:] {
		if def.Version.Compare(selected.Version) > 0 {
			selected = def
		}
	}
	return selected, true
}

func (p *KernelCardProvider) cardFromDefinition(def domain.ExtensionDefinition) ExtensionCard {
	card := ExtensionCard{
		ExtensionID:          string(def.ID),
		Publisher:            def.Publisher.PublisherID,
		DisplayName:          def.Name.Default,
		Description:          def.Description.Default,
		Version:              def.Version.String(),
		PackageURI:           def.Package.PackageID,
		Hash:                 def.Package.ArchiveHash,
		ArtifactID:           def.Package.ArtifactID,
		Trust:                normalizeTrust(def.Publisher.TrustLevel),
		Status:               ExtensionStatusNotInstalled,
		Enabled:              false,
		ContributionTags:     contributionTags(def),
		ProvidedCapabilities: providedCapabilities(def),
		Platforms:            append([]string(nil), def.Compatibility.Platforms...),
		Placement:            string(def.Placement),
		SystemManaged:        metadataBool(def.Metadata, "system.managed"),
		Source:               metadataString(def.Metadata, "source"),
		DevMode:              metadataBool(def.Metadata, "devOnly"),
		UserModified:         metadataBool(def.Metadata, "user.modified"),
		Actions:              []CardAction{},
	}
	card.Icon = metadataString(def.Metadata, "icon")
	return card
}

func (p *KernelCardProvider) populateInstalled(
	ctx context.Context,
	card ExtensionCard,
	def domain.ExtensionDefinition,
	inst domain.ExtensionInstallation,
) ExtensionCard {
	card.InstallationID = inst.InstallationID
	card.Generation = inst.Generation
	card.Status = ExtensionStatusInstalledEnabled
	card.Enabled = true
	switch inst.EnablementState {
	case domain.EnablementEnabled:
		card.Status = ExtensionStatusInstalledEnabled
		card.Enabled = true
	case domain.EnablementDisabled, domain.EnablementPartiallyDisabled:
		card.Status = ExtensionStatusInstalledDisabled
		card.Enabled = false
	}

	switch inst.InstallationState {
	case domain.InstallationStateInstalling:
		card.Status = ExtensionStatusInstalling
	case domain.InstallationStateUpdating:
		card.Status = ExtensionStatusUpdating
	case domain.InstallationStateFailed, domain.InstallationStateUninstallFailed:
		card.Status = ExtensionStatusFailed
	}

	card.DevMode = card.DevMode || metadataBool(inst.Metadata, "devOnly")
	card.UserModified = card.UserModified || metadataBool(inst.Metadata, "user.modified")
	card.FailureCount = metadataInt(inst.Metadata, "failureCount")
	if source := metadataString(inst.Metadata, "source"); source != "" {
		card.Source = source
	}
	if card.Source == "" {
		card.Source = "installed"
	}
	if !inst.InstalledAt.IsZero() {
		t := inst.InstalledAt
		card.InstalledAt = &t
	}
	if !inst.UpdatedAt.IsZero() {
		t := inst.UpdatedAt
		card.UpdatedAt = &t
	}

	p.populatePermissions(ctx, &card, inst.ExtensionID)
	p.populateVersionAvailability(ctx, &card, inst.ExtensionID)
	card.Actions = availableActions(card)
	return card
}

func (p *KernelCardProvider) populateVersionAvailability(ctx context.Context, card *ExtensionCard, extensionID domain.ExtensionID) {
	if p.dependencies.Versions == nil {
		return
	}
	available, err := p.dependencies.Versions.HasRollbackVersion(ctx, string(extensionID), card.Version)
	if err != nil {
		card.Diagnostics = append(card.Diagnostics, CardDiagnostic{
			Code:    "ROLLBACK_STATE_UNAVAILABLE",
			Message: err.Error(),
			Source:  "package_version_repository",
		})
		return
	}
	card.RollbackAvailable = available
}

func (p *KernelCardProvider) populatePermissions(ctx context.Context, card *ExtensionCard, extensionID domain.ExtensionID) {
	if p.dependencies.Permissions == nil {
		card.PermissionRisk = "unknown"
		return
	}
	requirements, requirementsErr := p.dependencies.Permissions.ListRequirements(ctx, extensionID)
	if requirementsErr != nil {
		card.Diagnostics = append(card.Diagnostics, CardDiagnostic{
			Code:    "PERMISSION_REQUIREMENTS_UNAVAILABLE",
			Message: requirementsErr.Error(),
			Source:  "permission_repository",
		})
		return
	}
	grants, grantsErr := p.dependencies.Permissions.ListGrants(ctx, extensionID)
	if grantsErr != nil {
		card.Diagnostics = append(card.Diagnostics, CardDiagnostic{
			Code:    "PERMISSION_GRANTS_UNAVAILABLE",
			Message: grantsErr.Error(),
			Source:  "permission_repository",
		})
		return
	}
	granted := make(map[string]bool, len(grants))
	for _, grant := range grants {
		granted[grant.PermissionName] = grant.State == "granted"
	}
	risk := ""
	for _, requirement := range requirements {
		card.Permissions = append(card.Permissions, requirement.PermissionName)
		if requirement.Required && !granted[requirement.PermissionName] {
			card.MissingPermissions = append(card.MissingPermissions, requirement.PermissionName)
		}
		if p.dependencies.PermissionDefinitions != nil {
			if definition, ok := p.dependencies.PermissionDefinitions.Get(requirement.PermissionName); ok {
				risk = higherRisk(risk, string(definition.RiskLevel))
			}
		}
	}
	sort.Strings(card.Permissions)
	sort.Strings(card.MissingPermissions)
	if risk == "" {
		risk = string(capability.RiskLow)
	}
	card.PermissionRisk = risk
	if len(card.MissingPermissions) > 0 {
		card.Status = ExtensionStatusNeedsPermission
	}
}

func (p *KernelCardProvider) checkUpdates(ctx context.Context, cards []ExtensionCard) {
	if p.dependencies.Updates == nil {
		return
	}
	checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i := range cards {
		if cards[i].InstalledAt == nil || cards[i].SystemManaged ||
			cards[i].Status == ExtensionStatusInstalling || cards[i].Status == ExtensionStatusUpdating {
			continue
		}
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-checkCtx.Done():
				cards[index].Diagnostics = append(cards[index].Diagnostics, CardDiagnostic{
					Code:    "UPDATE_CHECK_CANCELED",
					Message: checkCtx.Err().Error(),
					Source:  "package_distribution",
				})
				return
			}
			update, err := p.dependencies.Updates.Check(checkCtx, cards[index].ExtensionID, cards[index].Version)
			if err != nil {
				cards[index].Diagnostics = append(cards[index].Diagnostics, CardDiagnostic{
					Code:    "UPDATE_CHECK_FAILED",
					Message: err.Error(),
					Source:  "package_distribution",
				})
				return
			}
			if update == nil {
				return
			}
			cards[index].UpdateAvailable = true
			cards[index].LatestVersion = update.Version
			cards[index].Update = update
		}(i)
	}
	wg.Wait()
}

func (p *KernelCardProvider) installationMap(ctx context.Context) (map[string]domain.ExtensionInstallation, error) {
	insts, err := p.installations.ListInstallations(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[string]domain.ExtensionInstallation, len(insts))
	for _, inst := range insts {
		m[string(inst.ExtensionID)] = inst
	}
	return m, nil
}

func availableActions(card ExtensionCard) []CardAction {
	actions := []CardAction{{ActionID: "detail", Available: true}}
	if card.Status == ExtensionStatusInstalling || card.Status == ExtensionStatusUpdating {
		return append(actions, CardAction{ActionID: "diagnose", Available: true})
	}
	if card.Enabled {
		actions = append(actions, CardAction{ActionID: "disable", Available: true})
	} else {
		actions = append(actions, CardAction{
			ActionID:  "enable",
			Available: card.Status != ExtensionStatusFailed,
			Reason:    boolReason(card.Status == ExtensionStatusFailed, "扩展处于失败状态，需先诊断或恢复"),
		})
	}
	actions = append(actions,
		CardAction{ActionID: "permissions", Available: true},
		CardAction{ActionID: "update", Available: card.UpdateAvailable},
		CardAction{
			ActionID:  "rollback",
			Available: card.RollbackAvailable,
			Reason:    boolReason(!card.RollbackAvailable, "未检测到可回滚版本"),
		},
		CardAction{
			ActionID:  "uninstall",
			Available: !card.SystemManaged,
			Reason:    boolReason(card.SystemManaged, "系统管理的扩展不可卸载"),
		},
	)
	return actions
}

func boolReason(condition bool, reason string) string {
	if condition {
		return reason
	}
	return ""
}

func normalizeTrust(value string) TrustLevel {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "system":
		return TrustLevelPlatform
	case "official":
		return TrustLevelOfficial
	case "trusted":
		return TrustLevelVerified
	case "user_trusted":
		return TrustLevelUserTrusted
	case "development":
		return TrustLevelDevelopment
	case "blocked":
		return TrustLevelBlocked
	case "revoked":
		return TrustLevelRevoked
	case "community":
		return TrustLevelCommunity
	case "verified":
		return TrustLevelVerified
	default:
		return TrustLevelUntrusted
	}
}

func higherRisk(current, candidate string) string {
	if riskRank(candidate) > riskRank(current) {
		return candidate
	}
	return current
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

func providedCapabilities(ext domain.ExtensionDefinition) []string {
	capSet := make(map[string]bool)
	for _, mod := range ext.Modules {
		for _, capabilityDefinition := range mod.ProvidedCapabilities {
			if capabilityDefinition.ID != "" {
				capSet[capabilityDefinition.ID] = true
			}
		}
	}
	caps := make([]string, 0, len(capSet))
	for capabilityID := range capSet {
		caps = append(caps, capabilityID)
	}
	sort.Strings(caps)
	return caps
}

func contributionTags(ext domain.ExtensionDefinition) []ContributionTag {
	tagSet := make(map[ContributionTag]bool)
	for _, mod := range ext.Modules {
		for _, contributionDefinition := range mod.Contributions {
			switch contributionDefinition.EffectiveAdapter() {
			case domain.ContributionAdapterTool:
				tagSet[TagTools] = true
			case domain.ContributionAdapterAgentSkill:
				tagSet[TagSkills] = true
			case domain.ContributionAdapterWorkflow:
				tagSet[TagWorkflows] = true
			case domain.ContributionAdapterMCPServer:
				tagSet[TagMCP] = true
			case domain.ContributionAdapterUI, domain.ContributionAdapterUIProvider,
				domain.ContributionAdapterUISlot:
				tagSet[TagUI] = true
			case domain.ContributionAdapterHook:
				tagSet[TagHooks] = true
			case domain.ContributionAdapterEvent:
				tagSet[TagEvents] = true
			case domain.ContributionAdapterSchedule:
				tagSet[TagTasks] = true
			case domain.ContributionAdapterBackground:
				tagSet[TagDesktop] = true
			}
		}
		if len(mod.ProvidedCapabilities) > 0 {
			tagSet[TagProviders] = true
		}
	}
	tags := make([]ContributionTag, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i] < tags[j] })
	return tags
}
