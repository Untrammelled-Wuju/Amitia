package com.amitia.platform.bridge.provider

import com.amitia.platform.bridge.CapabilityProvider
import com.amitia.platform.bridge.NativeActionRequest
import com.amitia.platform.bridge.NativeActionResult
import com.amitia.platform.notification.NotificationManager
import com.amitia.platform.permissions.PermissionBroker
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class NotificationProvider @Inject constructor(
    private val permissionBroker: PermissionBroker,
    private val notificationManager: NotificationManager
) : CapabilityProvider {

    override fun action(): String = "notification"

    override fun requiredPermission(): String? = PermissionBroker.Permissions.POST_NOTIFICATIONS

    override suspend fun execute(request: NativeActionRequest): NativeActionResult {
        val op = request.params["op"] ?: "show"
        return when (op) {
            "show" -> {
                val title = request.params["title"] ?: "Amitia"
                val content = request.params["content"].orEmpty()
                val channelId = request.params["channel_id"] ?: "amitia_proactive"
                val id = (request.params["id"] ?: title).hashCode()
                val payload = NotificationManager.NotificationPayload(
                    id = id,
                    channelId = channelId,
                    title = title,
                    contentText = content,
                    autoCancel = true
                )
                val resultId = notificationManager.notify(payload)
                if (resultId >= 0) {
                    NativeActionResult.Success(mapOf("notification_id" to resultId.toString()))
                } else {
                    NativeActionResult.Failed("notify_failed")
                }
            }
            "cancel" -> {
                val id = request.params["id"]?.toIntOrNull()
                if (id != null) {
                    notificationManager.cancel(id)
                    NativeActionResult.Success(emptyMap())
                } else {
                    NativeActionResult.Failed("id required")
                }
            }
            "cancel_all" -> {
                notificationManager.cancelAll()
                NativeActionResult.Success(emptyMap())
            }
            "create_channel" -> {
                val channelId = request.params["channel_id"] ?: return NativeActionResult.Failed("channel_id required")
                val channelName = request.params["channel_name"] ?: channelId
                val channelDesc = request.params["channel_desc"].orEmpty()
                val config = NotificationManager.NotificationChannelConfig(
                    id = channelId,
                    name = channelName,
                    description = channelDesc
                )
                val ok = notificationManager.createChannel(config)
                if (ok) NativeActionResult.Success(emptyMap())
                else NativeActionResult.Failed("create_channel_failed")
            }
            else -> NativeActionResult.Failed("unsupported op: $op")
        }
    }
}
