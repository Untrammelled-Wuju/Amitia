package homekit

const (
	DefaultListLimit = 100
	MaxListLimit     = 500

	DefaultSceneListLimit = 50
	MaxSceneListLimit     = 200

	DefaultAutomationListLimit = 50
	MaxAutomationListLimit     = 200

	MaxCharacteristicListLimit = 500
	MaxServiceListLimit        = 200
	MaxAccessoryListLimit      = 200
	MaxRoomListLimit           = 100
	MaxZoneListLimit           = 100
	MaxHomeListLimit           = 50

	MaxEventSubscriptionCount = 50

	MaxActionsPerScene = 100

	StatusListReadTimeout        = 5000000000
	CharacteristicReadTimeout    = 10000000000
	CharacteristicWriteTimeout   = 15000000000
	SceneExecuteTimeout          = 20000000000
	SceneManageTimeout           = 20000000000
	AutomationManageTimeout      = 30000000000

	RiskLevelLow      = "low"
	RiskLevelMedium   = "medium"
	RiskLevelHigh     = "high"
)

var HighRiskCharacteristicTypes = map[string]bool{
	"lock_target_state":            true,
	"target_door_state":            true,
	"garage_door_target_state":     true,
	"security_system_target_state": true,
	"target_lock_state":            true,
	"target_heating_cooling_state": true,
}

var MediumRiskCharacteristicTypes = map[string]bool{
	"target_temperature": true,
	"heating_cooling_state": true,
	"active":              true,
}

func ClampLimit(limit, defaultLimit, maxLimit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func RiskFromCharacteristicType(charType string) string {
	if HighRiskCharacteristicTypes[charType] {
		return RiskLevelHigh
	}
	if MediumRiskCharacteristicTypes[charType] {
		return RiskLevelMedium
	}
	return RiskLevelLow
}

func MaxRisk(risks []string) string {
	hasMedium := false
	for _, r := range risks {
		if r == RiskLevelHigh {
			return RiskLevelHigh
		}
		if r == RiskLevelMedium {
			hasMedium = true
		}
	}
	if hasMedium {
		return RiskLevelMedium
	}
	return RiskLevelLow
}
