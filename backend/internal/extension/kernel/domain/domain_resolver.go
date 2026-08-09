package domain

import "fmt"

var exclusiveContributionKinds = map[ContributionKind]bool{
	ContributionKindGamePlugin:       true,
	ContributionKindDesktopPetPlugin: true,
}

func IsExclusiveContributionKind(kind ContributionKind) bool {
	return exclusiveContributionKinds[kind]
}

type DomainClassification struct {
	Domain                 ExtensionDomain
	ManagementTarget       ManagementTarget
	ExclusiveContributions []ContributionKind
}

func ResolveExtensionDomain(contributions []ContributionDefinition) (ExtensionDomain, error) {
	kinds := make([]ContributionKind, 0, len(contributions))
	for _, c := range contributions {
		kinds = append(kinds, c.Kind)
	}
	return ResolveDomainFromKinds(kinds)
}

func ResolveDomainFromKinds(kinds []ContributionKind) (ExtensionDomain, error) {
	hasGame := false
	hasDesktopPet := false
	var exclusiveFound []ContributionKind

	for _, k := range kinds {
		switch k {
		case ContributionKindGamePlugin:
			hasGame = true
			exclusiveFound = append(exclusiveFound, k)
		case ContributionKindDesktopPetPlugin:
			hasDesktopPet = true
			exclusiveFound = append(exclusiveFound, k)
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
		if IsExclusiveContributionKind(c.Kind) {
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
