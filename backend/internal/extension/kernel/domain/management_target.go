package domain

import "fmt"

type ManagementTarget string

const (
	ManagementTargetExtensionCenter  ManagementTarget = "extension_center"
	ManagementTargetGameCenter       ManagementTarget = "game_center"
	ManagementTargetPetCenter        ManagementTarget = "pet_center"
	ManagementTargetDesktopPetCenter                  = ManagementTargetPetCenter
)

func IsValidManagementTarget(target ManagementTarget) bool {
	switch NormalizeManagementTarget(target) {
	case ManagementTargetExtensionCenter, ManagementTargetGameCenter, ManagementTargetPetCenter:
		return true
	default:
		return false
	}
}

func NormalizeManagementTarget(target ManagementTarget) ManagementTarget {
	if target == "desktop_pet_center" {
		return ManagementTargetPetCenter
	}
	return target
}

func ManagementTargetForDomain(domain ExtensionDomain) (ManagementTarget, error) {
	switch domain {
	case ExtensionDomainGeneral:
		return ManagementTargetExtensionCenter, nil
	case ExtensionDomainGame:
		return ManagementTargetGameCenter, nil
	case ExtensionDomainDesktopPet:
		return ManagementTargetPetCenter, nil
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
	return NormalizeManagementTarget(m) == ManagementTargetPetCenter
}
