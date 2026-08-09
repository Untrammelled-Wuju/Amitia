package domain

type ExtensionDomain string

const (
	ExtensionDomainGeneral    ExtensionDomain = "general"
	ExtensionDomainGame       ExtensionDomain = "game"
	ExtensionDomainDesktopPet ExtensionDomain = "desktop_pet"
)

func DefaultExtensionDomain() ExtensionDomain {
	return ExtensionDomainGeneral
}

func IsValidExtensionDomain(domain ExtensionDomain) bool {
	switch domain {
	case ExtensionDomainGeneral, ExtensionDomainGame, ExtensionDomainDesktopPet:
		return true
	default:
		return false
	}
}

func NormalizeExtensionDomain(domain ExtensionDomain) ExtensionDomain {
	if domain == "" {
		return ExtensionDomainGeneral
	}
	return domain
}

func (d ExtensionDomain) IsGeneral() bool {
	return d == ExtensionDomainGeneral
}

func (d ExtensionDomain) IsGame() bool {
	return d == ExtensionDomainGame
}

func DomainConflict(kinds []ContributionKind) bool {
	hasGame := false
	hasDesktopPet := false
	for _, k := range kinds {
		switch k {
		case ContributionKindGamePlugin:
			hasGame = true
		case ContributionKindDesktopPetPlugin:
			hasDesktopPet = true
		}
	}
	return hasGame && hasDesktopPet
}

func (d ExtensionDomain) IsDesktopPet() bool {
	return d == ExtensionDomainDesktopPet
}

func DomainFromContributionKinds(kinds []ContributionKind) ExtensionDomain {
	domain, err := ResolveDomainFromKinds(kinds)
	if err != nil {
		return ExtensionDomainGeneral
	}
	return domain
}
