package adb

const (
	ADB_UNAVAILABLE             = "ADB_UNAVAILABLE"
	ADB_BACKEND_NOT_CONFIGURED  = "ADB_BACKEND_NOT_CONFIGURED"
	ADB_SERVER_UNAVAILABLE      = "ADB_SERVER_UNAVAILABLE"
	ADB_DEVICE_LIST_FAILED      = "ADB_DEVICE_LIST_FAILED"
	ADB_NO_DEVICE               = "ADB_NO_DEVICE"
	ADB_DEVICE_AMBIGUOUS        = "ADB_DEVICE_AMBIGUOUS"
	ADB_DEVICE_NOT_FOUND        = "ADB_DEVICE_NOT_FOUND"
	ADB_DEVICE_OFFLINE          = "ADB_DEVICE_OFFLINE"
	ADB_DEVICE_UNAUTHORIZED     = "ADB_DEVICE_UNAUTHORIZED"
	ADB_DEVICE_NO_PERMISSIONS   = "ADB_DEVICE_NO_PERMISSIONS"
	ADB_DEVICE_DISCONNECTED     = "ADB_DEVICE_DISCONNECTED"
	ADB_COMMAND_NOT_ALLOWED     = "ADB_COMMAND_NOT_ALLOWED"
	ADB_INVALID_ARGUMENT        = "ADB_INVALID_ARGUMENT"
	ADB_INPUT_TOO_LARGE         = "ADB_INPUT_TOO_LARGE"
	ADB_OUTPUT_TOO_LARGE        = "ADB_OUTPUT_TOO_LARGE"
	ADB_EXECUTION_FAILED        = "ADB_EXECUTION_FAILED"
	ADB_EXIT_CODE_UNAVAILABLE   = "ADB_EXIT_CODE_UNAVAILABLE"
	ADB_TIMEOUT                 = "ADB_TIMEOUT"
	ADB_CANCELLED               = "ADB_CANCELLED"
	ADB_INVALID_RESPONSE        = "ADB_INVALID_RESPONSE"
)

const (
	BackendUnavailable  = "unavailable"
	BackendNoServer     = "no_server"
	BackendNoDevice     = "no_device"
	BackendUnauthorized = "unauthorized"
	BackendOffline      = "offline"
	BackendAmbiguous    = "ambiguous"
	BackendReady        = "ready"
)

const (
	DeviceStateDevice        = "device"
	DeviceStateOffline       = "offline"
	DeviceStateUnauthorized  = "unauthorized"
	DeviceStateNoPermissions = "no_permissions"
	DeviceStateUnknown       = "unknown"
)

const (
	TransportUSB     = "usb"
	TransportNetwork = "network"
	TransportUnknown = "unknown"
)

const (
	PermissionADBInspect = "android.adb.inspect"
	PermissionADBExecute = "android.adb.execute"
)

const (
	OperationStatus  = "adb.status"
	OperationDevices = "adb.devices"
	OperationExecute = "adb.execute"
)

const maxSerialLength = 256
const maxStdoutBytes = 1024 * 1024 // 1 MiB
const maxStderrBytes = 512 * 1024  // 512 KiB
const maxCombinedBytes = 1572864   // 1.5 MiB
const maxInputBytes = 64 * 1024    // 64 KiB
const maxArgCount = 64
const maxSingleArgBytes = 8 * 1024 // 8 KiB
const maxTotalArgBytes = 64 * 1024  // 64 KiB
const defaultTimeoutSeconds = 10
const maxTimeoutSeconds = 30
const maxConcurrentPerDevice = 4
