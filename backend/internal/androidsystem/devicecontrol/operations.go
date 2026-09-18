package devicecontrol

const (
	OperationStatus                      = "device.status"
	OperationInfo                        = "device.info"
	OperationGlobalAction                = "device.global_action"
	OperationPressKey                    = "device.press_key"
	OperationLocationCurrent             = "device.location_current"
	OperationGeofenceAdd                 = "device.geofence_add"
	OperationGeofenceRemove              = "device.geofence_remove"
	OperationGeofenceList                = "device.geofence_list"
	OperationSettingsGet                 = "device.settings_get"
	OperationSettingsSet                 = "device.settings_set"
	OperationAppList                     = "device.app_list"
	OperationAppInfo                     = "device.app_info"
	OperationAppOpen                     = "device.app_open"
	OperationAppStop                     = "device.app_stop"
	OperationAppInstall                  = "device.app_install"
	OperationAppUninstall                = "device.app_uninstall"
	OperationBluetoothStatus             = "device.bluetooth_status"
	OperationBluetoothRequestEnable      = "device.bluetooth_request_enable"
	OperationBluetoothRequestPermission  = "device.bluetooth_request_permission"
	OperationBluetoothPair               = "device.bluetooth_pair"
	OperationBluetoothPaired             = "device.bluetooth_paired"
	OperationBluetoothScan               = "device.bluetooth_scan"
	OperationBLEScan                     = "device.ble_scan"
	OperationBluetoothClassicConnect     = "device.bluetooth_classic_connect"
	OperationBluetoothClassicDisconnect  = "device.bluetooth_classic_disconnect"
	OperationBluetoothClassicRead        = "device.bluetooth_classic_read"
	OperationBluetoothClassicWrite       = "device.bluetooth_classic_write"
	OperationBluetoothClassicListen      = "device.bluetooth_classic_listen"
	OperationBluetoothClassicAccept      = "device.bluetooth_classic_accept"
	OperationBluetoothClassicCloseServer = "device.bluetooth_classic_close_server"
	OperationBLEConnect                  = "device.ble_connect"
	OperationBLEDisconnect               = "device.ble_disconnect"
	OperationBLEServices                 = "device.ble_services"
	OperationBLECharacteristics          = "device.ble_characteristics"
	OperationBLERead                     = "device.ble_read"
	OperationBLEWrite                    = "device.ble_write"
	OperationBLESubscribe                = "device.ble_subscribe"
	OperationBLEUnsubscribe              = "device.ble_unsubscribe"
	OperationBLEReadNotifications        = "device.ble_read_notifications"
	OperationTaskerRunTask               = "device.tasker_run_task"
	OperationTaskerTriggerEvent          = "device.tasker_trigger_event"
	OperationMusicPlay                   = "device.music_play"
	OperationMusicPlayQueue              = "device.music_play_queue"
	OperationMusicPause                  = "device.music_pause"
	OperationMusicResume                 = "device.music_resume"
	OperationMusicStop                   = "device.music_stop"
	OperationMusicSeek                   = "device.music_seek"
	OperationMusicSetVolume              = "device.music_set_volume"
	OperationMusicStatus                 = "device.music_status"
	OperationSendBroadcast               = "device.send_broadcast"
	OperationToast                       = "device.toast"
)

const RuntimeID = "android_native_device_control"

var Operations = []string{
	OperationStatus,
	OperationInfo,
	OperationGlobalAction,
	OperationPressKey,
	OperationLocationCurrent,
	OperationGeofenceAdd,
	OperationGeofenceRemove,
	OperationGeofenceList,
	OperationSettingsGet,
	OperationSettingsSet,
	OperationAppList,
	OperationAppInfo,
	OperationAppOpen,
	OperationAppStop,
	OperationAppInstall,
	OperationAppUninstall,
	OperationBluetoothStatus,
	OperationBluetoothRequestEnable,
	OperationBluetoothRequestPermission,
	OperationBluetoothPair,
	OperationBluetoothPaired,
	OperationBluetoothScan,
	OperationBLEScan,
	OperationBluetoothClassicConnect,
	OperationBluetoothClassicDisconnect,
	OperationBluetoothClassicRead,
	OperationBluetoothClassicWrite,
	OperationBluetoothClassicListen,
	OperationBluetoothClassicAccept,
	OperationBluetoothClassicCloseServer,
	OperationBLEConnect,
	OperationBLEDisconnect,
	OperationBLEServices,
	OperationBLECharacteristics,
	OperationBLERead,
	OperationBLEWrite,
	OperationBLESubscribe,
	OperationBLEUnsubscribe,
	OperationBLEReadNotifications,
	OperationTaskerRunTask,
	OperationTaskerTriggerEvent,
	OperationMusicPlay,
	OperationMusicPlayQueue,
	OperationMusicPause,
	OperationMusicResume,
	OperationMusicStop,
	OperationMusicSeek,
	OperationMusicSetVolume,
	OperationMusicStatus,
	OperationSendBroadcast,
	OperationToast,
}
