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
	ToolIDScenesCreate  = "ios.homekit.scenes.create"
	ToolIDScenesUpdate  = "ios.homekit.scenes.update"
	ToolIDScenesDelete  = "ios.homekit.scenes.delete"

	ToolIDAutomationsList   = "ios.homekit.automations.list"
	ToolIDAutomationsGet    = "ios.homekit.automations.get"
	ToolIDAutomationsCreate = "ios.homekit.automations.create"
	ToolIDAutomationsUpdate = "ios.homekit.automations.update"
	ToolIDAutomationsEnable = "ios.homekit.automations.enable"
	ToolIDAutomationsDelete = "ios.homekit.automations.delete"

	ToolIDSetupPresent  = "ios.homekit.setup.present"
	ToolIDEnableHomeKit = "ios.homekit.enable"
)

const (
	ModelNameHomeKit = "ios_native_homekit"
)

var ModelNames = []string{ModelNameHomeKit}
