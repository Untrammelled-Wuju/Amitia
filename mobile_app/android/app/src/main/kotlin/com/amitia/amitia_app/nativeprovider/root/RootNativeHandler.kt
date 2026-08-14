package com.amitia.amitia_app.nativeprovider.root

import android.content.Context
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import java.io.File

internal class RootNativeHandler(
    private val context: Context,
) : AndroidNativeOperationHandler {

    private val suPaths = listOf(
        "/system/bin/su",
        "/system/xbin/su",
        "/sbin/su",
        "/system/su",
        "/system/bin/.ext/.su",
        "/system/usr/we-need-root/su",
        "/data/local/xbin/su",
        "/data/local/bin/su",
        "/data/local/su",
    )

    override fun supports(operation: String): Boolean {
        return operation == OP_STATUS || operation == OP_REQUEST || operation == OP_EXECUTE
    }

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        return when (request.operation) {
            OP_STATUS -> handleStatus(request)
            OP_REQUEST -> handleRequest(request)
            OP_EXECUTE -> handleExecute(request)
            else -> unsupportedOperation(request)
        }
    }

    private fun handleStatus(request: NativeBridgeRequest): NativeBridgeResponse {
        val state = detectRootState()
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "supported" to state.supported,
                "frameworkDetected" to state.frameworkDetected,
                "authorizationState" to state.authorizationState,
                "suAvailable" to state.suAvailable,
                "userActionRequired" to state.userActionRequired,
                "state" to state.state,
                "reason" to state.reason,
            ),
        )
    }

    private fun handleRequest(request: NativeBridgeRequest): NativeBridgeResponse {
        val state = detectRootState()
        return if (state.suAvailable) {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_SUCCESS,
                result = mapOf(
                    "requested" to true,
                    "userActionRequired" to true,
                    "state" to "authorization_pending",
                ),
            )
        } else {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "ROOT_UNAVAILABLE",
                    message = "no root framework detected on this device",
                ),
            )
        }
    }

    private fun handleExecute(request: NativeBridgeRequest): NativeBridgeResponse {
        val payload = request.payload
        val executable = payload["executable"] as? String
        if (executable.isNullOrBlank()) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "ROOT_INVALID_REQUEST",
                    message = "executable is required",
                ),
            )
        }

        val args = (payload["args"] as? List<*>)?.mapNotNull { it as? String } ?: emptyList()
        val env = (payload["env"] as? Map<*, *>)?.entries?.associate { (k, v) ->
            (k as? String) to (v as? String)
        }?.filterKeys { it != null }?.mapKeys { it.key!! } ?: emptyMap()
        val workDir = payload["workDir"] as? String
        val timeoutSeconds = (payload["timeoutSeconds"] as? Number)?.toInt() ?: 30

        val state = detectRootState()
        if (!state.suAvailable) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "ROOT_UNAVAILABLE",
                    message = "root is not available on this device",
                ),
            )
        }

        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_ERROR,
            error = NativeBridgeError(
                code = "ROOT_AUTHORIZATION_DENIED",
                message = "root execution requires user authorization",
            ),
        )
    }

    private fun detectRootState(): RootCapabilityState {
        val suPath = suPaths.firstOrNull { path ->
            try {
                File(path).exists()
            } catch (_: Exception) {
                false
            }
        }

        if (suPath != null) {
            return RootCapabilityState(
                supported = true,
                frameworkDetected = "su",
                authorizationState = "authorized",
                suAvailable = true,
                userActionRequired = false,
                state = "available",
                reason = "su binary detected at $suPath",
            )
        }

        return RootCapabilityState(
            supported = true,
            frameworkDetected = null,
            authorizationState = "unavailable",
            suAvailable = false,
            userActionRequired = true,
            state = "unavailable",
            reason = "no su binary detected",
        )
    }

    private fun unsupportedOperation(request: NativeBridgeRequest): NativeBridgeResponse {
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_ERROR,
            error = NativeBridgeError(
                code = NativeBridgeProtocol.ERR_OPERATION_NOT_SUPPORTED,
                message = "unknown root operation: ${request.operation}",
            ),
        )
    }

    companion object {
        const val OP_STATUS = "root.status"
        const val OP_REQUEST = "root.request"
        const val OP_EXECUTE = "root.execute"
    }
}
