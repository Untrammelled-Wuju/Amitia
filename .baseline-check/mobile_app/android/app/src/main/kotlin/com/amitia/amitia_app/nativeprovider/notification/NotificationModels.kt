package com.amitia.amitia_app.nativeprovider.notification

data class NotificationActionProjection(
    val actionRef: String,
    val title: String,
    val hasRemoteInput: Boolean,
)

data class NotificationProjection(
    val notificationRef: String,
    val packageName: String,
    val appLabel: String,
    val postedAt: Long,
    val title: String,
    val text: String,
    val subText: String,
    val category: String,
    val ongoing: Boolean,
    val clearable: Boolean,
    val groupKey: String,
    val channelId: String,
    val importance: Int,
    val hasContentAction: Boolean,
    val actions: List<NotificationActionProjection> = emptyList(),
    val generation: Long = 0L,
)

data class NotificationCapabilityState(
    val supported: Boolean = false,
    val listenerDeclared: Boolean = false,
    val listenerGranted: Boolean = false,
    val listenerConnected: Boolean = false,
    val postPermissionRequired: Boolean = false,
    val postPermissionGranted: Boolean = false,
    val notificationsEnabled: Boolean = false,
    val canRead: Boolean = false,
    val canDismiss: Boolean = false,
    val canPost: Boolean = false,
    val userActionRequired: Boolean = false,
    val state: String = "unsupported",
)

data class NativeNotificationRequest(
    val requestId: String,
    val operation: String,
    val payload: Map<String, Any?> = emptyMap(),
)

data class NativeNotificationResponse(
    val requestId: String,
    val status: String,
    val result: Map<String, Any?>? = null,
    val error: NativeNotificationError? = null,
)

data class NativeNotificationError(
    val code: String,
    val message: String,
    val domainCode: String? = null,
)

internal data class InternalRefMapping(
    val key: String,
    val generation: Long,
)
