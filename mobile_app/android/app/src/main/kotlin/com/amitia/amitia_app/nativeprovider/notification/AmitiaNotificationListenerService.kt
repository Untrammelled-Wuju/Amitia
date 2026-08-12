package com.amitia.amitia_app.nativeprovider.notification

import android.app.Notification
import android.service.notification.NotificationListenerService
import android.service.notification.StatusBarNotification

class AmitiaNotificationListenerService : NotificationListenerService() {

    private var handler: NotificationNativeHandler? = null

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
        if (sbn == null) return
        if (isInternalNoise(sbn)) return
    }

    override fun onNotificationRemoved(sbn: StatusBarNotification?) {
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

    private fun isInternalNoise(sbn: StatusBarNotification): Boolean {
        if (sbn.packageName != packageName) return false
        return (sbn.notification.flags and Notification.FLAG_FOREGROUND_SERVICE) != 0
    }
}
