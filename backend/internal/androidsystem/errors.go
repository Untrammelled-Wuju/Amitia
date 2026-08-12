package androidsystem

const (
	NOTIFICATION_UNSUPPORTED                    = "NOTIFICATION_UNSUPPORTED"
	NOTIFICATION_LISTENER_NOT_DECLARED          = "NOTIFICATION_LISTENER_NOT_DECLARED"
	NOTIFICATION_LISTENER_PERMISSION_REQUIRED   = "NOTIFICATION_LISTENER_PERMISSION_REQUIRED"
	NOTIFICATION_LISTENER_NOT_CONNECTED         = "NOTIFICATION_LISTENER_NOT_CONNECTED"
	NOTIFICATION_POST_PERMISSION_REQUIRED       = "NOTIFICATION_POST_PERMISSION_REQUIRED"
	NOTIFICATION_POST_DISABLED                  = "NOTIFICATION_POST_DISABLED"
	NOTIFICATION_NOT_FOUND                      = "NOTIFICATION_NOT_FOUND"
	NOTIFICATION_STALE_REFERENCE                = "NOTIFICATION_STALE_REFERENCE"
	NOTIFICATION_NOT_DISMISSIBLE                = "NOTIFICATION_NOT_DISMISSIBLE"
	NOTIFICATION_CONTENT_ACTION_UNAVAILABLE     = "NOTIFICATION_CONTENT_ACTION_UNAVAILABLE"
	NOTIFICATION_ACTION_NOT_FOUND               = "NOTIFICATION_ACTION_NOT_FOUND"
	NOTIFICATION_ACTION_STALE                   = "NOTIFICATION_ACTION_STALE"
	NOTIFICATION_REMOTE_INPUT_UNSUPPORTED       = "NOTIFICATION_REMOTE_INPUT_UNSUPPORTED"
	NOTIFICATION_POST_FAILED                    = "NOTIFICATION_POST_FAILED"
	NOTIFICATION_CANCEL_FAILED                  = "NOTIFICATION_CANCEL_FAILED"
	NOTIFICATION_ACTION_FAILED                  = "NOTIFICATION_ACTION_FAILED"
	NOTIFICATION_SETTINGS_UNAVAILABLE           = "NOTIFICATION_SETTINGS_UNAVAILABLE"
	BLOCKED_ANDROID_NATIVE_HOST_SOURCE          = "BLOCKED_ANDROID_NATIVE_HOST_SOURCE"
	ANDROID_NOTIFICATION_REAL_DEVICE_UNVERIFIED = "ANDROID_NOTIFICATION_REAL_DEVICE_UNVERIFIED"
	BLOCKED_BY_FROZEN_A_CONTRACT                = "BLOCKED_BY_FROZEN_A_CONTRACT"
)

const (
	PermissionNotificationRead   = "android.notification.read"
	PermissionNotificationPost   = "android.notification.post"
	PermissionNotificationControl = "android.notification.control"
)

const (
	PermissionOverlayInspect = "android.overlay.inspect"
	PermissionOverlayCreate  = "android.overlay.create"
)

const (
	PermissionExternalAutomationInspect  = "android.external_automation.inspect"
	PermissionExternalAutomationLaunch   = "android.external_automation.launch"
	PermissionExternalAutomationOpenURI  = "android.external_automation.open_uri"
	PermissionExternalAutomationSettings = "android.external_automation.settings"
	PermissionExternalAutomationIntent   = "android.external_automation.intent"
)

const (
	NotificationListenerGranted       = "granted"
	NotificationListenerNotGranted    = "not_granted"
	NotificationListenerNotConnected  = "not_connected"
	NotificationPostPermissionGranted = "post_granted"
	NotificationPostDisabled          = "post_disabled"
)
