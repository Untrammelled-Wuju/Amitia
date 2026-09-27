package com.amitia.amitia_app.nativeprovider.accessibility

import android.content.Context
import org.json.JSONObject

internal class AccessibilityNativeHandler(context: Context) {

    private val appContext = context.applicationContext
    private val stateReader = AccessibilityStateReader(appContext)
    private val settingsLauncher = AccessibilitySettingsLauncher(appContext)

    suspend fun execute(request: NativeAccessibilityRequest): NativeAccessibilityResponse {
        return when (request.operation) {
            OP_STATUS -> handleStatus(request)
            OP_OPEN_SETTINGS -> handleOpenSettings(request)
            OP_PROVIDER_STATUS -> handleProviderStatus(request)
            OP_PROVIDER_INSTALL -> handleProviderInstall(request)
            OP_PROVIDER_OPEN_SETTINGS -> handleProviderOpenSettings(request)
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

    private suspend fun handleProviderStatus(request: NativeAccessibilityRequest): NativeAccessibilityResponse {
        val installed = AccessibilityProviderInstaller.isInstalled(appContext)
        val connected = installed && AccessibilityProviderClient.isConnected(appContext)
        return NativeAccessibilityResponse(
            requestId = request.requestId,
            status = "success",
            result = mapOf(
                "installed" to installed,
                "connected" to connected,
                "ready" to connected,
                "versionName" to (AccessibilityProviderInstaller.installedVersion(appContext) ?: ""),
                "packageName" to AccessibilityProviderInstaller.PROVIDER_PACKAGE,
                "state" to when {
                    !installed -> "not_installed"
                    !connected -> "disabled"
                    else -> "connected"
                },
            ),
        )
    }

    private fun handleProviderInstall(request: NativeAccessibilityRequest): NativeAccessibilityResponse {
        val opened = AccessibilityProviderInstaller.launchInstall(appContext)
        return NativeAccessibilityResponse(
            requestId = request.requestId,
            status = if (opened) "success" else "error",
            result = mapOf("opened" to opened, "userActionRequired" to true),
            error = if (opened) null else NativeAccessibilityError(
                code = "ACCESSIBILITY_PROVIDER_INSTALL_UNAVAILABLE",
                message = "accessibility provider installer could not be opened",
            ),
        )
    }

    private fun handleProviderOpenSettings(request: NativeAccessibilityRequest): NativeAccessibilityResponse {
        val opened = AccessibilityProviderInstaller.openProviderSettings(appContext)
        return NativeAccessibilityResponse(
            requestId = request.requestId,
            status = if (opened) "success" else "error",
            result = mapOf("opened" to opened, "userActionRequired" to true),
            error = if (opened) null else NativeAccessibilityError(
                code = "ACCESSIBILITY_PROVIDER_SETTINGS_UNAVAILABLE",
                message = "accessibility provider settings could not be opened",
            ),
        )
    }

    private suspend fun handleStatus(request: NativeAccessibilityRequest): NativeAccessibilityResponse {
        AccessibilityProviderClient.status(appContext)?.let { providerStatus ->
            val provider = JSONObject(providerStatus)
            val connected = provider.optBoolean("connected", false)
            val canRetrieve = provider.optBoolean("canRetrieveWindowContent", false)
            val canPerformGestures = provider.optBoolean("canPerformGestures", false)
            val ready = connected && canRetrieve && canPerformGestures
            return NativeAccessibilityResponse(
                requestId = request.requestId,
                status = "success",
                result = mapOf(
                    "platformSupported" to true,
                    "serviceDeclared" to true,
                    "enabledInSettings" to connected,
                    "connected" to connected,
                    "canRetrieveWindowContent" to canRetrieve,
                    "canRetrieveInteractiveWindows" to provider.optBoolean("canRetrieveInteractiveWindows", false),
                    "includeNotImportantViews" to provider.optBoolean("includeNotImportantViews", false),
                    "enhancedWebAccessibility" to provider.optBoolean("enhancedWebAccessibility", false),
                    "canPerformGestures" to canPerformGestures,
                    "canTakeScreenshot" to provider.optBoolean("canTakeScreenshot", false),
                    "interactionReady" to ready,
                    "visualReady" to ready,
                    "ready" to ready,
                    "userActionRequired" to !ready,
                    "state" to if (ready) "connected" else "disabled",
                    "generation" to 0,
                ),
            )
        }

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
            "includeNotImportantViews" to state.includeNotImportantViews,
            "enhancedWebAccessibility" to state.enhancedWebAccessibility,
            "canPerformGestures" to state.canPerformGestures,
            "canTakeScreenshot" to state.canTakeScreenshot,
            "interactionReady" to state.interactionReady,
            "visualReady" to state.visualReady,
            "ready" to state.ready,
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
        if (!settingsLauncher.openSettings()) {
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
        return NativeAccessibilityResponse(
            requestId = request.requestId,
            status = "success",
            result = mapOf(
                "opened" to true,
                "userActionRequired" to true,
            ),
        )
    }

    companion object {
        const val OP_STATUS = "accessibility.status"
        const val OP_OPEN_SETTINGS = "accessibility.open_settings"
        const val OP_PROVIDER_STATUS = "accessibility.provider.status"
        const val OP_PROVIDER_INSTALL = "accessibility.provider.install"
        const val OP_PROVIDER_OPEN_SETTINGS = "accessibility.provider.open_settings"
    }
}
