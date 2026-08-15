package homekit

const (
	ToolIDStatus        = "ios.homekit.status"

	ToolIDHomesList     = "ios.homekit.homes.list"
	ToolIDHomesGet      = "ios.homekit.homes.get"

	ToolIDRoomsList     = "ios.homekit.rooms.list"
	ToolIDZonesList     = "ios.homekit.zones.list"

	ToolIDAccessoriesList = "ios.homekit.accessories.list"
	ToolIDAccessoriesGet  = "ios.homekit.accessories.get"

	ToolIDServicesList    = "ios.homekit.services.list"

	ToolIDCharacteristicsList = "ios.homekit.characteristics.list"
	ToolIDCharacteristicsRead  = "ios.homekit.characteristics.read"
	ToolIDCharacteristicsWrite = "ios.homekit.characteristics.write"

	ToolIDScenesList    = "ios.homekit.scenes.list"
	ToolIDScenesGet     = "ios.homekit.scenes.get"
	ToolIDScenesExecute = "ios.homekit.scenes.execute"

	ToolIDAutomationsList   = "ios.homekit.automations.list"
)

const (
	ModelNameHomeKit = "ios_native_homekit"
)

var ModelNames = []string{ModelNameHomeKit}
