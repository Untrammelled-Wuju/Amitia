package com.amitia.amitia_app.nativeprovider.notification

import android.service.notification.StatusBarNotification

internal class NotificationActionExecutor {

    fun executeAction(sbn: StatusBarNotification, actionRef: String, projection: NotificationProjectionInternal): Boolean {
        val actions = sbn.notification.actions ?: return false

        val targetAction = actionRef.let { ref ->
            val suffix = ref.removePrefix("act_").take(8)
            actions.firstOrNull { action ->
                val actionSignature = action.title?.toString() ?: ""
                actionSignature.isNotEmpty() && actionSignature.hashCode().toUInt()
                    .toString(16).take(8) == suffix
            } ?: actions.firstOrNull { a ->
                a.remoteInputs.isNullOrEmpty()
            }
        } ?: return false

        if (!targetAction.remoteInputs.isNullOrEmpty()) {
            return false
        }

        return try {
            targetAction.actionIntent.send()
            true
        } catch (e: Exception) {
            false
        }
    }

    fun openContentIntent(sbn: StatusBarNotification): Boolean {
        val pendingIntent = sbn.notification.contentIntent ?: return false
        return try {
            pendingIntent.send()
            true
        } catch (e: Exception) {
            false
        }
    }

    fun dismissNotification(sbn: StatusBarNotification): Boolean {
        if ((sbn.notification.flags and android.app.Notification.FLAG_ONGOING_EVENT) != 0) {
            return false
        }
        if (!sbn.isClearable) {
            return false
        }
        return true
    }
}
