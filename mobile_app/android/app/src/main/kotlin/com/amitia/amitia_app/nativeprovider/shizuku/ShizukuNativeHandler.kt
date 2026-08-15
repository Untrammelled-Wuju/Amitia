package com.amitia.amitia_app.nativeprovider.shizuku

import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import rikka.shizuku.Shizuku

internal class ShizukuNativeHandler(
    private val context: Context,
) : AndroidNativeOperationHandler {

    override val operations: Set<String> = setOf(OP_STATUS, OP_REQUEST_PERMISSION, OP_EXECUTE)

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        return when (request.operation) {
            OP_STATUS -> handleStatus(request)
            OP_REQUEST_PERMISSION -> handleRequestPermission(request)
            OP_EXECUTE -> handleExecute(request)
            else -> unsupportedOperation(request)
        }
    }

    private fun handleStatus(request: NativeBridgeRequest): NativeBridgeResponse {
        val state = detectShizukuState()
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "supported" to state.supported,
                "installed" to state.installed,
                "binderAvailable" to state.binderAvailable,
                "permissionState" to state.permissionState,
                "state" to state.state,
                "reason" to state.reason,
            ),
        )
    }

    private fun handleRequestPermission(request: NativeBridgeRequest): NativeBridgeResponse {
        val state = detectShizukuState()
        if (!state.installed) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "SHIZUKU_NOT_INSTALLED",
                    message = "Shizuku is not installed on this device",
                ),
            )
        }
        if (!state.binderAvailable) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "SHIZUKU_BINDER_UNAVAILABLE",
                    message = "Shizuku binder is not available, ensure Shizuku service is running",
                ),
            )
        }
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "requested" to true,
                "userActionRequired" to true,
                "state" to "permission_pending",
            ),
        )
    }

    private fun handleExecute(request: NativeBridgeRequest): NativeBridgeResponse {
        val payload = request.payload
        val command = payload["command"] as? String
        if (command.isNullOrBlank()) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "SHIZUKU_INVALID_REQUEST",
                    message = "command is required",
                ),
            )
        }

        val state = detectShizukuState()
        if (state.permissionState != "authorized") {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "SHIZUKU_PERMISSION_REQUIRED",
                    message = "Shizuku permission not granted",
                ),
            )
        }

        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "executed" to false,
                "state" to "adapter_pending",
                "reason" to "direct execution via Shizuku binder requires host-side adapter",
            ),
        )
    }

    private fun detectShizukuState(): ShizukuCapabilityState {
        if (!isShizukuInstalled()) {
            return ShizukuCapabilityState(
                supported = true,
                installed = false,
                binderAvailable = false,
                permissionState = "not_installed",
                state = "not_installed",
                reason = "Shizuku app is not installed",
            )
        }

        val binderAvailable = Shizuku.pingBinder()
        if (!binderAvailable) {
            return ShizukuCapabilityState(
                supported = true,
                installed = true,
                binderAvailable = false,
                permissionState = "binder_unavailable",
                state = "binder_unavailable",
                reason = "Shizuku app installed but service is not running",
            )
        }

        val granted = try {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                Shizuku.checkSelfPermission() == PackageManager.PERMISSION_GRANTED
            } else {
                Shizuku.checkSelfPermission() == PackageManager.PERMISSION_GRANTED
            }
        } catch (_: Exception) {
            false
        }

        return ShizukuCapabilityState(
            supported = true,
            installed = true,
            binderAvailable = true,
            permissionState = if (granted) "authorized" else "permission_not_requested",
            state = if (granted) "authorized" else "permission_not_requested",
            reason = if (granted) "Shizuku permission granted" else "Shizuku permission not yet requested",
        )
    }

    private fun isShizukuInstalled(): Boolean {
        return try {
            context.packageManager.getPackageInfo("moe.shizuku.privileged.api", 0)
            true
        } catch (_: PackageManager.NameNotFoundException) {
            false
        } catch (_: Exception) {
            false
        }
    }

    private fun unsupportedOperation(request: NativeBridgeRequest): NativeBridgeResponse {
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_ERROR,
            error = NativeBridgeError(
                code = NativeBridgeProtocol.ERR_OPERATION_NOT_SUPPORTED,
                message = "unknown shizuku operation: ${request.operation}",
            ),
        )
    }

    companion object {
        const val OP_STATUS = "shizuku.status"
        const val OP_REQUEST_PERMISSION = "shizuku.request_permission"
        const val OP_EXECUTE = "shizuku.execute"
    }
}
