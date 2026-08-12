package homekit

const (
	OperationStatus = "homekit.status"

	OperationHomesList = "homekit.homes.list"
	OperationHomesGet  = "homekit.homes.get"

	OperationRoomsList = "homekit.rooms.list"
	OperationZonesList = "homekit.zones.list"

	OperationAccessoriesList = "homekit.accessories.list"
	OperationAccessoriesGet  = "homekit.accessories.get"

	OperationServicesList = "homekit.services.list"

	OperationCharacteristicsList = "homekit.characteristics.list"
	OperationCharacteristicsRead  = "homekit.characteristics.read"
	OperationCharacteristicsWrite = "homekit.characteristics.write"

	OperationScenesList    = "homekit.scenes.list"
	OperationScenesGet     = "homekit.scenes.get"
	OperationScenesExecute = "homekit.scenes.execute"
	OperationScenesCreate  = "homekit.scenes.create"
	OperationScenesUpdate  = "homekit.scenes.update"
	OperationScenesDelete  = "homekit.scenes.delete"

	OperationAutomationsList   = "homekit.automations.list"
	OperationAutomationsGet    = "homekit.automations.get"
	OperationAutomationsCreate = "homekit.automations.create"
	OperationAutomationsUpdate = "homekit.automations.update"
	OperationAutomationsEnable = "homekit.automations.enable"
	OperationAutomationsDelete = "homekit.automations.delete"

	OperationSetupPresent = "homekit.setup.present"
	OperationEnableHomeKit = "homekit.enable"
)
