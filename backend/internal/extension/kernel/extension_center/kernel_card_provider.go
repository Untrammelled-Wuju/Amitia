package extension_center

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type KernelCardProvider struct {
	definitions   domain.DefinitionRepository
	installations domain.InstallationRepository
}

func NewKernelCardProvider(
	definitions domain.DefinitionRepository,
	installations domain.InstallationRepository,
) *KernelCardProvider {
	return &KernelCardProvider{
		definitions:   definitions,
		installations: installations,
	}
}

func (p *KernelCardProvider) ListCards(ctx context.Context) ([]ExtensionCard, error) {
	defs, err := p.definitions.ListExtensions(ctx)
	if err != nil {
		return nil, err
	}

	extensions, err := domain.FilterByManagementTarget(defs, domain.ManagementFilter{
		Targets: []domain.ManagementTarget{domain.ManagementTargetExtensionCenter},
	})
	if err != nil {
		return nil, err
	}

	instMap, err := p.installationMap(ctx)
	if err != nil {
		return nil, err
	}

	cards := make([]ExtensionCard, 0, len(extensions))
	for _, ext := range extensions {
		inst, installed := instMap[string(ext.ID)]

		card := ExtensionCard{
			ExtensionID:          string(ext.ID),
			DisplayName:          ext.Name.Default,
			Description:          ext.Description.Default,
			Version:              ext.Version.String(),
			Status:               ExtensionStatusNotInstalled,
			Enabled:              false,
			ContributionTags:     contributionTags(ext),
			ProvidedCapabilities: providedCapabilities(ext),
			Platforms:            ext.Compatibility.Platforms,
		}

		if ext.Publisher.PublisherID != "" {
			card.Publisher = ext.Publisher.PublisherID
		}

		if installed {
			card = p.populateInstalled(card, ext, inst)
		}

		cards = append(cards, card)
	}

	return cards, nil
}

func (p *KernelCardProvider) populateInstalled(card ExtensionCard, ext domain.ExtensionDefinition, inst domain.ExtensionInstallation) ExtensionCard {
	switch inst.EnablementState {
	case domain.EnablementEnabled:
		card.Status = ExtensionStatusInstalledEnabled
		card.Enabled = true
	case domain.EnablementDisabled, domain.EnablementPartiallyDisabled:
		card.Status = ExtensionStatusInstalledDisabled
		card.Enabled = false
	default:
		card.Status = ExtensionStatusInstalledEnabled
		card.Enabled = true
	}

	switch inst.InstallationState {
	case domain.InstallationStateInstalling:
		card.Status = ExtensionStatusInstalling
	case domain.InstallationStateUpdating:
		card.Status = ExtensionStatusUpdating
	case domain.InstallationStateFailed, domain.InstallationStateUninstallFailed:
		card.Status = ExtensionStatusFailed
	}

	if !inst.InstalledAt.IsZero() {
		t := inst.InstalledAt
		card.InstalledAt = &t
	}
	if !inst.UpdatedAt.IsZero() {
		t := inst.UpdatedAt
		card.UpdatedAt = &t
	}

	return card
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

func providedCapabilities(ext domain.ExtensionDefinition) []string {
	capSet := make(map[string]bool)
	for _, mod := range ext.Modules {
		for _, cap := range mod.ProvidedCapabilities {
			if cap.ID != "" {
				capSet[cap.ID] = true
			}
		}
	}
	caps := make([]string, 0, len(capSet))
	for c := range capSet {
		caps = append(caps, c)
	}
	return caps
}

func contributionTags(ext domain.ExtensionDefinition) []ContributionTag {
	tagSet := make(map[ContributionTag]bool)
	for _, mod := range ext.Modules {
		for _, c := range mod.Contributions {
			switch domain.ContributionKind(c.Kind) {
			case domain.ContributionKindTool:
				tagSet[TagTools] = true
			case domain.ContributionKindAgentSkill:
				tagSet[TagSkills] = true
			case domain.ContributionKindWorkflow:
				tagSet[TagWorkflows] = true
			case domain.ContributionKindMCPServer:
				tagSet[TagMCP] = true
			case domain.ContributionKindUIPage, domain.ContributionKindUIPanel,
				domain.ContributionKindUIChat, domain.ContributionKindUIContextAction,
				domain.ContributionKindUIDesktop, domain.ContributionKindUIProvider,
				domain.ContributionKindUISlot:
				tagSet[TagUI] = true
			case domain.ContributionKindHook:
				tagSet[TagHooks] = true
			case domain.ContributionKindEventSubscription:
				tagSet[TagEvents] = true
			case domain.ContributionKindSchedule:
				tagSet[TagTasks] = true
			case domain.ContributionKindBackgroundTask:
				tagSet[TagDesktop] = true
			}
		}
		if len(mod.ProvidedCapabilities) > 0 {
			tagSet[TagProviders] = true
		}
	}
	tags := make([]ContributionTag, 0, len(tagSet))
	for t := range tagSet {
		tags = append(tags, t)
	}
	return tags
}
