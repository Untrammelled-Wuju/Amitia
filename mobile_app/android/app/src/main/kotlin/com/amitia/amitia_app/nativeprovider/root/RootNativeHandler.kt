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

    override val operations: Set<String> = setOf(OP_STATUS, OP_REQUEST, OP_EXECUTE)

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
        val workDir = payload["workDir"] as? String
        val timeoutMs = (payload["timeoutMs"] as? Number)?.toLong() ?: 30000L

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

        return try {
            val command = mutableListOf("su", "-c", executable)
            command.addAll(args)

            val processBuilder = ProcessBuilder(command)
            if (workDir != null) {
                processBuilder.directory(File(workDir))
            }
            processBuilder.redirectErrorStream(true)

            val process = processBuilder.start()
            val output = process.inputStream.bufferedReader().readText()

            val completed = process.waitFor(timeoutMs, java.util.concurrent.TimeUnit.MILLISECONDS)
            if (!completed) {
                process.destroyForcibly()
                return NativeBridgeResponse(
                    protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                    requestId = request.requestId,
                    status = NativeBridgeProtocol.STATUS_ERROR,
                    error = NativeBridgeError(
                        code = "ROOT_EXECUTION_TIMEOUT",
                        message = "root command timed out after ${timeoutMs}ms",
                    ),
                )
            }

            val exitCode = process.exitValue()
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = if (exitCode == 0) NativeBridgeProtocol.STATUS_SUCCESS else NativeBridgeProtocol.STATUS_ERROR,
                result = mapOf(
                    "exitCode" to exitCode,
                    "output" to output,
                    "completed" to true,
                ),
            )
        } catch (e: Exception) {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "ROOT_EXECUTION_FAILED",
                    message = "root execution failed: ${e.message}",
                ),
            )
        }
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
