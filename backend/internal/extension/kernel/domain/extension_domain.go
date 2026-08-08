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

func DomainFromContributionKinds(kinds []ContributionKind) ExtensionDomain {
	for _, k := range kinds {
		if k == ContributionKindGamePlugin {
			return ExtensionDomainGame
		}
	}
	return ExtensionDomainGeneral
}
