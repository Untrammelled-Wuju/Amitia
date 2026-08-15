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

	OperationAutomationsList = "homekit.automations.list"
)

func Operations() []string {
	return []string{
		OperationStatus,
		OperationHomesList,
		OperationHomesGet,
		OperationRoomsList,
		OperationZonesList,
		OperationAccessoriesList,
		OperationAccessoriesGet,
		OperationServicesList,
		OperationCharacteristicsList,
		OperationCharacteristicsRead,
		OperationCharacteristicsWrite,
		OperationScenesList,
		OperationScenesGet,
		OperationScenesExecute,
		OperationAutomationsList,
	}
}
