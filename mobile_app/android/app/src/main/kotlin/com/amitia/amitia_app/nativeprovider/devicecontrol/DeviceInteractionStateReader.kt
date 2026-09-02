package com.amitia.amitia_app.nativeprovider.devicecontrol

import android.app.ActivityManager
import android.app.KeyguardManager
import android.content.Context
import android.os.Build
import android.os.PowerManager

internal enum class DeviceInteractionAvailability {
    AVAILABLE,
    WAITING_UNLOCK,
    WAITING_SCREEN,
    BLOCKED,
}

internal data class DeviceInteractionState(
    val screenOn: Boolean,
    val interactive: Boolean,
    val keyguardLocked: Boolean,
    val userPresent: Boolean,
    val backgroundRestricted: Boolean,
    val deviceIdleMode: Boolean,
    val powerSaveMode: Boolean,
    val availability: DeviceInteractionAvailability,
) {
    fun asMap(): Map<String, Any> = mapOf(
        "screenOn" to screenOn,
        "interactive" to interactive,
        "keyguardLocked" to keyguardLocked,
        "userPresent" to userPresent,
        "backgroundRestricted" to backgroundRestricted,
        "deviceIdleMode" to deviceIdleMode,
        "powerSaveMode" to powerSaveMode,
        "interactionState" to availability.name,
    )
}

/**
 * Reads the Android state that determines whether user-visible UI automation can
 * safely execute right now.  This deliberately never attempts to dismiss the
 * keyguard or wake/unlock the device; user authentication remains a user action.
 */
internal class DeviceInteractionStateReader(
    context: Context,
) {
    private val appContext = context.applicationContext
    private val powerManager = appContext.getSystemService(Context.POWER_SERVICE) as? PowerManager
    private val keyguardManager = appContext.getSystemService(Context.KEYGUARD_SERVICE) as? KeyguardManager
    private val activityManager = appContext.getSystemService(Context.ACTIVITY_SERVICE) as? ActivityManager

    fun read(): DeviceInteractionState {
        val interactive = powerManager?.isInteractive ?: true
        val keyguardLocked = keyguardManager?.isKeyguardLocked ?: false
        val deviceIdleMode = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            powerManager?.isDeviceIdleMode ?: false
        } else {
            false
        }
        val powerSaveMode = powerManager?.isPowerSaveMode ?: false
        val backgroundRestricted = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            activityManager?.isBackgroundRestricted ?: false
        } else {
            false
        }
        val availability = when {
            keyguardLocked -> DeviceInteractionAvailability.WAITING_UNLOCK
            !interactive -> DeviceInteractionAvailability.WAITING_SCREEN
            backgroundRestricted || deviceIdleMode -> DeviceInteractionAvailability.BLOCKED
            else -> DeviceInteractionAvailability.AVAILABLE
        }
        return DeviceInteractionState(
            screenOn = interactive,
            interactive = interactive,
            keyguardLocked = keyguardLocked,
            userPresent = interactive && !keyguardLocked,
            backgroundRestricted = backgroundRestricted,
            deviceIdleMode = deviceIdleMode,
            powerSaveMode = powerSaveMode,
            availability = availability,
        )
    }
}
