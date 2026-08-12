package com.amitia.amitia_app.nativeprovider.notification

import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import android.provider.Settings
import androidx.core.app.NotificationManagerCompat

internal class NotificationStateReader(private val context: Context) {

    fun readState(): NotificationCapabilityState {
        val listenerDeclared = isListenerDeclared()
        val listenerGranted = isListenerAccessGranted()
        val listenerConnected = listenerGranted && NotificationServiceRegistry.isServiceAttached()

        val postPermissionRequired = Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU
        val postPermissionGranted = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            context.checkSelfPermission(android.Manifest.permission.POST_NOTIFICATIONS) ==
                PackageManager.PERMISSION_GRANTED
        } else {
            true
        }

        val notificationsEnabled = NotificationManagerCompat.from(context).areNotificationsEnabled()

        val canRead = listenerGranted && listenerConnected
        val canDismiss = canRead && listenerConnected
        val canPost = if (postPermissionRequired) {
            postPermissionGranted && notificationsEnabled
        } else {
            notificationsEnabled
        }

        val userActionRequired = !listenerGranted || (postPermissionRequired && !postPermissionGranted)

        val state = when {
            !listenerDeclared -> STATE_LISTENER_NOT_DECLARED
            !listenerGranted -> STATE_LISTENER_NOT_GRANTED
            !listenerConnected -> STATE_LISTENER_NOT_CONNECTED
            postPermissionRequired && !postPermissionGranted -> STATE_POST_PERMISSION_DENIED
            else -> STATE_LISTENER_CONNECTED
        }

        return NotificationCapabilityState(
            supported = true,
            listenerDeclared = listenerDeclared,
            listenerGranted = listenerGranted,
            listenerConnected = listenerConnected,
            postPermissionRequired = postPermissionRequired,
            postPermissionGranted = postPermissionGranted,
            notificationsEnabled = notificationsEnabled,
            canRead = canRead,
            canDismiss = canDismiss,
            canPost = canPost,
            userActionRequired = userActionRequired,
            state = state,
        )
    }

    fun canOpenListenerSettings(): Boolean = true

    private fun isListenerDeclared(): Boolean = true

    private fun isListenerAccessGranted(): Boolean {
        val enabledListeners = Settings.Secure.getString(
            context.contentResolver,
            "enabled_notification_listeners",
        ) ?: return false
        val packageName = context.packageName
        return enabledListeners.split(":").any { entry -> entry.contains(packageName) }
    }

    companion object {
        const val STATE_LISTENER_NOT_DECLARED = "listener_not_declared"
        const val STATE_LISTENER_NOT_GRANTED = "listener_not_granted"
        const val STATE_LISTENER_NOT_CONNECTED = "listener_not_connected"
        const val STATE_POST_PERMISSION_DENIED = "post_permission_denied"
        const val STATE_LISTENER_CONNECTED = "listener_connected"
    }
}
