package domain

import "fmt"

type ManagementTarget string

const (
	ManagementTargetExtensionCenter  ManagementTarget = "extension_center"
	ManagementTargetGameCenter       ManagementTarget = "game_center"
	ManagementTargetDesktopPetCenter ManagementTarget = "desktop_pet_center"
)

func IsValidManagementTarget(target ManagementTarget) bool {
	switch target {
	case ManagementTargetExtensionCenter, ManagementTargetGameCenter, ManagementTargetDesktopPetCenter:
		return true
	default:
		return false
	}
}

func ManagementTargetForDomain(domain ExtensionDomain) (ManagementTarget, error) {
	switch domain {
	case ExtensionDomainGeneral:
		return ManagementTargetExtensionCenter, nil
	case ExtensionDomainGame:
		return ManagementTargetGameCenter, nil
	case ExtensionDomainDesktopPet:
		return ManagementTargetDesktopPetCenter, nil
	default:
		return "", fmt.Errorf("domain: unknown extension domain: %s", domain)
	}
}

func (m ManagementTarget) IsExtensionCenter() bool {
	return m == ManagementTargetExtensionCenter
}

func (m ManagementTarget) IsGameCenter() bool {
	return m == ManagementTargetGameCenter
}

func (m ManagementTarget) IsDesktopPetCenter() bool {
	return m == ManagementTargetDesktopPetCenter
}
