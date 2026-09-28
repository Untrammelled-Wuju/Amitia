package com.amitia.amitia_app.nativeprovider.virtualdisplay

import android.content.Context
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import com.amitia.amitia_app.virtualdisplay.host.VirtualDisplayHostError
import com.amitia.amitia_app.virtualdisplay.host.VirtualDisplayHostManager
import com.amitia.amitia_app.virtualdisplay.host.VirtualDisplayPermissionStore

internal class VirtualDisplayNativeHandler(
    context: Context,
) : AndroidNativeOperationHandler {

    private val appContext = context.applicationContext
    private val manager = VirtualDisplayHostManager(appContext)

    override val operations: Set<String> = setOf(
        OP_PERMISSION_STATUS,
        OP_PERMISSION_SET,
        OP_HOST_ENSURE,
        OP_STATUS,
        OP_CREATE,
        OP_GET,
        OP_LIST,
        OP_RESIZE,
        OP_RELEASE,
        OP_LAUNCH,
        OP_CAPTURE,
        OP_TAP,
        OP_SWIPE,
        OP_KEY,
        OP_TEXT,
    )

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        return when (request.operation) {
            OP_PERMISSION_STATUS -> permissionStatus(request)
            OP_PERMISSION_SET -> permissionSet(request)
            OP_HOST_ENSURE -> hostEnsure(request)
            OP_STATUS -> status(request)
            else -> executeHost(request)
        }
    }

    private suspend fun permissionStatus(request: NativeBridgeRequest): NativeBridgeResponse {
        val enabled = VirtualDisplayPermissionStore.isEnabled(appContext)
        val running = enabled && manager.hostRunning()
        return success(
            request,
            mapOf(
                "enabled" to enabled,
                "supported" to true,
                "provider" to "app_process",
                "hostRunning" to running,
                "permissionState" to if (enabled) "granted" else "disabled",
                "userActionRequired" to !enabled,
                "state" to when {
                    !enabled -> "disabled"
                    running -> "ready"
                    else -> "stopped"
                },
            ),
        )
    }

    private suspend fun permissionSet(request: NativeBridgeRequest): NativeBridgeResponse {
        val enabled = request.payload["enabled"] as? Boolean
            ?: return failure(request, "VIRTUAL_DISPLAY_INVALID_REQUEST", "enabled must be a boolean")
        if (!enabled) {
            VirtualDisplayPermissionStore.setEnabled(appContext, false)
            manager.stop()
            return success(
                request,
                mapOf(
                    "enabled" to false,
                    "state" to "disabled",
                    "hostRunning" to false,
                ),
            )
        }
        return try {
            val status = manager.ensureStarted()
            VirtualDisplayPermissionStore.setEnabled(appContext, true)
            success(
                request,
                mapOf(
                    "enabled" to true,
                    "state" to "ready",
                    "hostRunning" to true,
                    "host" to status,
                ),
            )
        } catch (error: VirtualDisplayHostError) {
            VirtualDisplayPermissionStore.setEnabled(appContext, false)
            failure(request, error.code, error.message ?: "virtual display host unavailable")
        } catch (error: Throwable) {
            VirtualDisplayPermissionStore.setEnabled(appContext, false)
            failure(request, "VIRTUAL_DISPLAY_HOST_UNAVAILABLE", error.message ?: "virtual display host unavailable")
        }
    }

    private suspend fun hostEnsure(request: NativeBridgeRequest): NativeBridgeResponse {
        if (!VirtualDisplayPermissionStore.isEnabled(appContext)) {
            return failure(request, "VIRTUAL_DISPLAY_PERMISSION_REQUIRED", "virtual display access is disabled")
        }
        return try {
            success(request, manager.ensureStarted())
        } catch (error: VirtualDisplayHostError) {
            failure(request, error.code, error.message ?: "virtual display host unavailable")
        } catch (error: Throwable) {
            failure(request, "VIRTUAL_DISPLAY_HOST_UNAVAILABLE", error.message ?: "virtual display host unavailable")
        }
    }

    private suspend fun status(request: NativeBridgeRequest): NativeBridgeResponse {
        if (!VirtualDisplayPermissionStore.isEnabled(appContext)) {
            return success(
                request,
                mapOf(
                    "supported" to true,
                    "available" to false,
                    "enabled" to false,
                    "canCreate" to false,
                    "active" to false,
                    "activeCount" to 0,
                    "frameSourceSupported" to false,
                    "gestureSupported" to false,
                    "thirdPartyLaunchSupported" to false,
                    "uiTreeSupported" to false,
                    "state" to "permission_required",
                    "reason" to "virtual display access is disabled",
                ),
            )
        }
        return try {
            val result = manager.status().toMutableMap()
            result["enabled"] = true
            if (result["hostRunning"] == false) {
                result["state"] = "host_unavailable"
            }
            success(request, result)
        } catch (error: VirtualDisplayHostError) {
            success(
                request,
                mapOf(
                    "supported" to true,
                    "available" to false,
                    "enabled" to true,
                    "canCreate" to false,
                    "active" to false,
                    "activeCount" to 0,
                    "frameSourceSupported" to false,
                    "gestureSupported" to false,
                    "thirdPartyLaunchSupported" to false,
                    "uiTreeSupported" to false,
                    "state" to "host_unavailable",
                    "reason" to error.message,
                ),
            )
        } catch (error: Throwable) {
            success(
                request,
                mapOf(
                    "supported" to true,
                    "available" to false,
                    "enabled" to true,
                    "canCreate" to false,
                    "active" to false,
                    "activeCount" to 0,
                    "frameSourceSupported" to false,
                    "gestureSupported" to false,
                    "thirdPartyLaunchSupported" to false,
                    "uiTreeSupported" to false,
                    "state" to "host_unavailable",
                    "reason" to (error.message ?: "virtual display host unavailable"),
                ),
            )
        }
    }

    private suspend fun executeHost(request: NativeBridgeRequest): NativeBridgeResponse {
        if (!VirtualDisplayPermissionStore.isEnabled(appContext)) {
            return failure(request, "VIRTUAL_DISPLAY_PERMISSION_REQUIRED", "virtual display access is disabled")
        }
        return try {
            val result = manager.execute(request.operation, request.payload)
            success(request, normalizeResult(request.operation, result))
        } catch (error: VirtualDisplayHostError) {
            failure(request, error.code, error.message ?: "virtual display host error")
        } catch (error: Throwable) {
            failure(request, "VIRTUAL_DISPLAY_HOST_UNAVAILABLE", error.message ?: "virtual display host unavailable")
        }
    }

    private fun normalizeResult(operation: String, result: Map<String, Any?>): Map<String, Any?> {
        val normalized = LinkedHashMap(result)
        when (operation) {
            OP_CREATE -> {
                normalized["frameSourceReady"] = result["frameSourceReady"] ?: true
                normalized["thirdPartyLaunchSupported"] = result["thirdPartyLaunchSupported"] ?: true
                normalized["uiTreeSupported"] = result["uiTreeSupported"] ?: false
                normalized["gestureSupported"] = result["gestureSupported"] ?: true
            }
            OP_STATUS -> {
                normalized["display"] = result["display"] ?: result
            }
        }
        return normalized
    }

    private fun success(
        request: NativeBridgeRequest,
        result: Map<String, Any?>,
    ): NativeBridgeResponse = NativeBridgeResponse(
        protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
        requestId = request.requestId,
        status = NativeBridgeProtocol.STATUS_SUCCESS,
        result = result,
    )

    private fun failure(
        request: NativeBridgeRequest,
        code: String,
        message: String,
    ): NativeBridgeResponse = NativeBridgeResponse(
        protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
        requestId = request.requestId,
        status = NativeBridgeProtocol.STATUS_ERROR,
        error = NativeBridgeError(code = code, message = message),
    )

    companion object {
        const val OP_PERMISSION_STATUS = "virtual_display.permission.status"
        const val OP_PERMISSION_SET = "virtual_display.permission.set"
        const val OP_HOST_ENSURE = "virtual_display.host.ensure"
        const val OP_STATUS = "virtual_display.status"
        const val OP_CREATE = "virtual_display.create"
        const val OP_GET = "virtual_display.get"
        const val OP_LIST = "virtual_display.list"
        const val OP_RESIZE = "virtual_display.resize"
        const val OP_RELEASE = "virtual_display.release"
        const val OP_LAUNCH = "virtual_display.launch"
        const val OP_CAPTURE = "virtual_display.capture"
        const val OP_TAP = "virtual_display.tap"
        const val OP_SWIPE = "virtual_display.swipe"
        const val OP_KEY = "virtual_display.key"
        const val OP_TEXT = "virtual_display.text"
    }
}
