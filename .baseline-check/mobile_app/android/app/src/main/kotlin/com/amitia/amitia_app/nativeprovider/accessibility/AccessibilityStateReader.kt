package com.amitia.amitia_app.nativeprovider.accessibility

import android.content.ComponentName
import android.content.Context
import android.provider.Settings
import android.view.accessibility.AccessibilityManager

internal class AccessibilityStateReader(private val context: Context) {

    fun readState(): AccessibilityCapabilityState {
        val serviceDeclared = true
        val enabledInSettings = isAccessibilityEnabledInSettings()
        val connected = AccessibilityServiceRegistry.isServiceConnected()
        val canRetrieveWindowContent = true
        val canRetrieveInteractiveWindows = true

        val userActionRequired = !enabledInSettings

        val state = when {
            !serviceDeclared -> STATE_NOT_DECLARED
            !enabledInSettings -> STATE_DISABLED
            !connected -> STATE_ENABLED_NOT_CONNECTED
            else -> STATE_CONNECTED
        }

        return AccessibilityCapabilityState(
            platformSupported = true,
            serviceDeclared = serviceDeclared,
            enabledInSettings = enabledInSettings,
            connected = connected,
            canRetrieveWindowContent = canRetrieveWindowContent,
            canRetrieveInteractiveWindows = canRetrieveInteractiveWindows,
            userActionRequired = userActionRequired,
            state = state,
        )
    }

    private fun isAccessibilityEnabledInSettings(): Boolean {
        val enabledServices = Settings.Secure.getString(
            context.contentResolver,
            Settings.Secure.ENABLED_ACCESSIBILITY_SERVICES,
        ) ?: return false

        val expectedComponent = ComponentName(
            context.packageName,
            AmitiaAccessibilityService::class.java.name,
        )

        val colonSplitter = enabledServices.split(':')
        for (entry in colonSplitter) {
            val name = ComponentName.unflattenFromString(entry)
            if (name != null && name.packageName == expectedComponent.packageName) {
                if (name == expectedComponent) return true
                if (name.className == expectedComponent.className) return true
            }
        }
        return false
    }

    companion object {
        const val STATE_UNSUPPORTED = "unsupported"
        const val STATE_NOT_DECLARED = "not_declared"
        const val STATE_DISABLED = "disabled"
        const val STATE_ENABLED_NOT_CONNECTED = "enabled_not_connected"
        const val STATE_CONNECTED = "connected"
    }
}
