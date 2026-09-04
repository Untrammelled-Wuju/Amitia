package com.amitia.amitia_app.nativeprovider.notification

import android.app.Notification
import android.service.notification.NotificationListenerService
import android.service.notification.StatusBarNotification
import com.amitia.amitia_app.runtime.workflow.WorkflowDeviceEventIngress
import org.json.JSONArray
import org.json.JSONObject
import java.security.MessageDigest

class AmitiaNotificationListenerService : NotificationListenerService() {

    private var handler: NotificationNativeHandler? = null
    private val workflowIngress by lazy { WorkflowDeviceEventIngress(applicationContext) }

    override fun onCreate() {
        super.onCreate()
        NotificationServiceRegistry.attach(this)
    }

    override fun onListenerConnected() {
        super.onListenerConnected()
        NotificationServiceRegistry.attach(this)
        handler?.onListenerConnected()
    }

    override fun onListenerDisconnected() {
        super.onListenerDisconnected()
        handler?.onListenerDisconnected()
    }

    override fun onNotificationPosted(sbn: StatusBarNotification?) {
        if (sbn == null || isInternalNoise(sbn)) return
        emitWorkflowNotificationEvent("device.notification.posted", sbn)
    }

    override fun onNotificationRemoved(sbn: StatusBarNotification?) {
        if (sbn == null || isInternalNoise(sbn)) return
        emitWorkflowNotificationEvent("device.notification.removed", sbn)
    }

    override fun onDestroy() {
        NotificationServiceRegistry.detach(this)
        super.onDestroy()
    }

    fun ensureHandler() {
        if (handler == null) {
            handler = NotificationNativeHandler(applicationContext)
        }
    }

    fun handleRequest(request: NativeNotificationRequest): NativeNotificationResponse {
        ensureHandler()
        return handler?.execute(request) ?: NativeNotificationResponse(
            requestId = request.requestId,
            status = "error",
            error = NativeNotificationError(
                code = "NOTIFICATION_UNSUPPORTED",
                message = "notification native handler not available",
            ),
        )
    }

    private fun emitWorkflowNotificationEvent(eventType: String, sbn: StatusBarNotification) {
        val notification = sbn.notification
        val extras = notification.extras
        val actions = JSONArray()
        notification.actions?.forEachIndexed { index, action ->
            actions.put(
                JSONObject()
                    .put("index", index)
                    .put("title", action.title?.toString().orEmpty().take(256))
                    .put("hasRemoteInput", action.remoteInputs?.isNotEmpty() == true),
            )
        }
        val payload = JSONObject()
            .put("packageName", sbn.packageName)
            .put("notificationId", sbn.id)
            .put("tag", sbn.tag.orEmpty().take(256))
            .put("keyHash", digest(sbn.key).take(32))
            .put("postedAt", sbn.postTime)
            .put("title", extras.getCharSequence(Notification.EXTRA_TITLE)?.toString().orEmpty().take(512))
            .put("text", extras.getCharSequence(Notification.EXTRA_TEXT)?.toString().orEmpty().take(4096))
            .put("subText", extras.getCharSequence(Notification.EXTRA_SUB_TEXT)?.toString().orEmpty().take(1024))
            .put("category", notification.category.orEmpty())
            .put("channelId", notification.channelId.orEmpty())
            .put("groupKey", sbn.groupKey.orEmpty().take(256))
            .put("ongoing", sbn.isOngoing)
            .put("clearable", sbn.isClearable)
            .put("hasContentAction", notification.contentIntent != null)
            .put("actions", actions)
        val eventID = "notification:${digest("$eventType\u0000${sbn.key}\u0000${sbn.postTime}").take(48)}"
        workflowIngress.emit(eventType, payload, "android.notification_listener", eventID)
    }

    private fun digest(value: String): String = MessageDigest.getInstance("SHA-256")
        .digest(value.toByteArray(Charsets.UTF_8))
        .joinToString("") { "%02x".format(it) }

    private fun isInternalNoise(sbn: StatusBarNotification): Boolean {
        if (sbn.packageName != packageName) return false
        return (sbn.notification.flags and Notification.FLAG_FOREGROUND_SERVICE) != 0
    }
}
