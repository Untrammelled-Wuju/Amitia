package domain

type ExtensionDomain string

const (
	ExtensionDomainGeneral    ExtensionDomain = "general"
	ExtensionDomainGame       ExtensionDomain = "gamex"
	ExtensionDomainDesktopPet ExtensionDomain = "petx"
	ExtensionDomainMemory     ExtensionDomain = "memory"
	ExtensionDomainProfile    ExtensionDomain = "profile"
	ExtensionDomainEpisodic   ExtensionDomain = "episodic"
	ExtensionDomainWorldbook  ExtensionDomain = "worldbook"
	ExtensionDomainCompanion  ExtensionDomain = "companion"
)

func DefaultExtensionDomain() ExtensionDomain {
	return ExtensionDomainGeneral
}

func IsValidExtensionDomain(domain ExtensionDomain) bool {
	switch NormalizeExtensionDomain(domain) {
	case ExtensionDomainGeneral, ExtensionDomainGame, ExtensionDomainDesktopPet,
		ExtensionDomainMemory, ExtensionDomainProfile, ExtensionDomainEpisodic,
		ExtensionDomainWorldbook, ExtensionDomainCompanion:
		return true
	default:
		return false
	}
}

func NormalizeExtensionDomain(domain ExtensionDomain) ExtensionDomain {
	switch domain {
	case "":
		return ExtensionDomainGeneral
	case "game":
		return ExtensionDomainGame
	case "desktop_pet":
		return ExtensionDomainDesktopPet
	default:
		return domain
	}
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
		switch NormalizeContributionKind(k) {
		case ContributionKindGamePlugin:
			hasGame = true
		case ContributionKindPetPlugin:
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
