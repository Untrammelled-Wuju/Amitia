package com.amitia.amitia_app.nativeprovider.accessibility

import android.content.ComponentName
import android.content.Context
import android.accessibilityservice.AccessibilityServiceInfo
import android.os.Build
import android.provider.Settings
import android.view.accessibility.AccessibilityManager

internal class AccessibilityStateReader(private val context: Context) {

    fun readState(): AccessibilityCapabilityState {
        val manager = context.getSystemService(Context.ACCESSIBILITY_SERVICE) as? AccessibilityManager
        val installedServiceInfo = manager?.installedAccessibilityServiceList?.firstOrNull {
            it.resolveInfo.serviceInfo.packageName == context.packageName &&
                it.resolveInfo.serviceInfo.name == AmitiaAccessibilityService::class.java.name
        }
        val connectedService = AccessibilityServiceRegistry.current()
        val serviceInfo = connectedService?.serviceInfo ?: installedServiceInfo
        val serviceDeclared = installedServiceInfo != null || connectedService != null
        val enabledInSettings = isAccessibilityEnabledInSettings()
        val connected = connectedService != null
        val canRetrieveWindowContent = serviceInfo?.capabilities?.and(
            AccessibilityServiceInfo.CAPABILITY_CAN_RETRIEVE_WINDOW_CONTENT,
        ) != 0 && serviceInfo != null
        val canRetrieveInteractiveWindows = serviceInfo?.flags?.and(
            AccessibilityServiceInfo.FLAG_RETRIEVE_INTERACTIVE_WINDOWS,
        ) != 0 && serviceInfo != null
        val canPerformGestures = serviceInfo?.capabilities?.and(
            AccessibilityServiceInfo.CAPABILITY_CAN_PERFORM_GESTURES,
        ) != 0 && serviceInfo != null
        val canTakeScreenshot = serviceInfo?.capabilities?.and(
            AccessibilityServiceInfo.CAPABILITY_CAN_TAKE_SCREENSHOT,
        ) != 0 && serviceInfo != null
        val includeNotImportantViews = serviceInfo?.flags?.and(
            AccessibilityServiceInfo.FLAG_INCLUDE_NOT_IMPORTANT_VIEWS,
        ) != 0 && serviceInfo != null
        val enhancedWebAccessibility = serviceInfo?.flags?.and(
            AccessibilityServiceInfo.FLAG_REQUEST_ENHANCED_WEB_ACCESSIBILITY,
        ) != 0 && serviceInfo != null

        val screenshotReady = Build.VERSION.SDK_INT < Build.VERSION_CODES.R || canTakeScreenshot
        val interactionReady = connected && canRetrieveWindowContent && canPerformGestures
        val visualReady = interactionReady && screenshotReady
        val ready = connected && canRetrieveWindowContent && canPerformGestures && screenshotReady

        val userActionRequired = !enabledInSettings

        val state = when {
            !serviceDeclared -> STATE_NOT_DECLARED
            !enabledInSettings -> STATE_DISABLED
            !connected -> STATE_ENABLED_NOT_CONNECTED
            !ready -> STATE_DEGRADED
            else -> STATE_CONNECTED
        }

        return AccessibilityCapabilityState(
            platformSupported = true,
            serviceDeclared = serviceDeclared,
            enabledInSettings = enabledInSettings,
            connected = connected,
            canRetrieveWindowContent = canRetrieveWindowContent,
            canRetrieveInteractiveWindows = canRetrieveInteractiveWindows,
            includeNotImportantViews = includeNotImportantViews,
            enhancedWebAccessibility = enhancedWebAccessibility,
            canPerformGestures = canPerformGestures,
            canTakeScreenshot = canTakeScreenshot,
            interactionReady = interactionReady,
            visualReady = visualReady,
            ready = ready,
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
        const val STATE_DEGRADED = "degraded"
        const val STATE_CONNECTED = "connected"
    }
}
