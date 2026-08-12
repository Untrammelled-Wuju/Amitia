package bluetooth

const (
	OperationStatus             = "bluetooth.status"
	OperationScanStart          = "bluetooth.scan.start"
	OperationScanStop           = "bluetooth.scan.stop"
	OperationPeripheralGet      = "bluetooth.peripheral.get"
	OperationPeripheralConnected = "bluetooth.peripheral.connected"
	OperationConnect            = "bluetooth.connect"
	OperationDisconnect         = "bluetooth.disconnect"
	OperationServicesDiscover   = "bluetooth.services.discover"
	OperationCharacteristicsDiscover = "bluetooth.characteristics.discover"
	OperationDescriptorsDiscover    = "bluetooth.descriptors.discover"
	OperationCharacteristicRead     = "bluetooth.characteristic.read"
	OperationCharacteristicWrite    = "bluetooth.characteristic.write"
	OperationCharacteristicSubscribe = "bluetooth.characteristic.subscribe"
	OperationCharacteristicUnsubscribe = "bluetooth.characteristic.unsubscribe"
	OperationDescriptorRead    = "bluetooth.descriptor.read"
	OperationDescriptorWrite   = "bluetooth.descriptor.write"
	OperationRSSIRead          = "bluetooth.rssi.read"
	OperationPeripheralRoleStart = "bluetooth.peripheral_role.start"
	OperationPeripheralRoleStop  = "bluetooth.peripheral_role.stop"
)
