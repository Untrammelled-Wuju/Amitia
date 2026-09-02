package com.amitia.amitia_app.nativeprovider.accessibility

import android.content.Context

internal class AccessibilityNativeHandler(context: Context) {

    private val appContext = context.applicationContext
    private val stateReader = AccessibilityStateReader(appContext)
    private val settingsLauncher = AccessibilitySettingsLauncher(appContext)

    fun execute(request: NativeAccessibilityRequest): NativeAccessibilityResponse {
        return when (request.operation) {
            OP_STATUS -> handleStatus(request)
            OP_OPEN_SETTINGS -> handleOpenSettings(request)
            else -> NativeAccessibilityResponse(
                requestId = request.requestId,
                status = "error",
                error = NativeAccessibilityError(
                    code = "OPERATION_NOT_SUPPORTED",
                    message = "unknown accessibility operation: ${request.operation}",
                ),
            )
        }
    }

    private fun handleStatus(request: NativeAccessibilityRequest): NativeAccessibilityResponse {
        val state = stateReader.readState()
        val health = AccessibilityHealthMonitor.snapshot(
            configured = state.serviceDeclared,
            enabled = state.enabledInSettings,
        )
        val result = mapOf(
            "platformSupported" to state.platformSupported,
            "serviceDeclared" to state.serviceDeclared,
            "enabledInSettings" to state.enabledInSettings,
            "connected" to state.connected,
            "canRetrieveWindowContent" to state.canRetrieveWindowContent,
            "canRetrieveInteractiveWindows" to state.canRetrieveInteractiveWindows,
            "userActionRequired" to state.userActionRequired,
            "state" to state.state,
            "generation" to AccessibilityServiceRegistry.generation(),
            "lastConnectedAt" to health.lastConnectedAt,
            "lastEventAt" to health.lastEventAt,
            "lastDisconnectAt" to health.lastDisconnectAt,
            "healthGeneration" to health.generation,
        )
        return NativeAccessibilityResponse(
            requestId = request.requestId,
            status = "success",
            result = result,
        )
    }

    private fun handleOpenSettings(request: NativeAccessibilityRequest): NativeAccessibilityResponse {
        if (!settingsLauncher.canOpenSettings()) {
            return NativeAccessibilityResponse(
                requestId = request.requestId,
                status = "error",
                error = NativeAccessibilityError(
                    code = "ACCESSIBILITY_SETTINGS_UNAVAILABLE",
                    message = "accessibility settings activity is not available on this device",
                    domainCode = "ACCESSIBILITY_SETTINGS_UNAVAILABLE",
                ),
            )
        }

        val opened = settingsLauncher.openSettings()
        return NativeAccessibilityResponse(
            requestId = request.requestId,
            status = "success",
            result = mapOf(
                "opened" to opened,
                "userActionRequired" to true,
            ),
        )
    }

    companion object {
        const val OP_STATUS = "accessibility.status"
        const val OP_OPEN_SETTINGS = "accessibility.open_settings"
    }
}
