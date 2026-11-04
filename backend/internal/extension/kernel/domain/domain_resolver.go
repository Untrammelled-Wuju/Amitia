package domain

import "fmt"

var exclusiveContributionKinds = map[ContributionKind]bool{
	ContributionKindGamePlugin: true,
	ContributionKindPetPlugin:  true,
}

var exclusiveContributionAdapters = map[ContributionAdapter]bool{
	ContributionAdapterGame:       true,
	ContributionAdapterDesktopPet: true,
}

func IsExclusiveContributionKind(kind ContributionKind) bool {
	return exclusiveContributionKinds[kind]
}

func IsExclusiveContributionAdapter(adapter ContributionAdapter) bool {
	return exclusiveContributionAdapters[adapter]
}

type DomainClassification struct {
	Domain                 ExtensionDomain
	ManagementTarget       ManagementTarget
	ExclusiveContributions []ContributionKind
}

func ResolveExtensionDomain(contributions []ContributionDefinition) (ExtensionDomain, error) {
	adapters := make([]ContributionAdapter, 0, len(contributions))
	for _, c := range contributions {
		adapters = append(adapters, c.EffectiveAdapter())
	}
	return ResolveDomainFromAdapters(adapters)
}

func ResolveDomainFromKinds(kinds []ContributionKind) (ExtensionDomain, error) {
	adapters := make([]ContributionAdapter, 0, len(kinds))
	for _, kind := range kinds {
		adapters = append(adapters, ContributionAdapterForKind(kind))
	}
	return ResolveDomainFromAdapters(adapters)
}

func ResolveDomainFromAdapters(adapters []ContributionAdapter) (ExtensionDomain, error) {
	hasGame := false
	hasDesktopPet := false
	var exclusiveFound []ContributionAdapter

	for _, adapter := range adapters {
		switch adapter {
		case ContributionAdapterGame:
			hasGame = true
			exclusiveFound = append(exclusiveFound, adapter)
		case ContributionAdapterDesktopPet:
			hasDesktopPet = true
			exclusiveFound = append(exclusiveFound, adapter)
		}
	}

	if hasGame && hasDesktopPet {
		return ExtensionDomainGeneral, fmt.Errorf("domain: extension declares contributions for multiple exclusive domains: %v", exclusiveFound)
	}
	if hasGame {
		return ExtensionDomainGame, nil
	}
	if hasDesktopPet {
		return ExtensionDomainDesktopPet, nil
	}
	return ExtensionDomainGeneral, nil
}

func ClassifyExtension(contributions []ContributionDefinition) (DomainClassification, error) {
	domain, err := ResolveExtensionDomain(contributions)
	classification := DomainClassification{
		Domain: domain,
	}
	for _, c := range contributions {
		if IsExclusiveContributionAdapter(c.EffectiveAdapter()) {
			classification.ExclusiveContributions = append(classification.ExclusiveContributions, c.Kind)
		}
	}
	if err == nil {
		target, targetErr := ManagementTargetForDomain(domain)
		if targetErr != nil {
			return classification, targetErr
		}
		classification.ManagementTarget = target
	}
	return classification, err
}
