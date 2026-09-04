package com.amitia.amitia_app.nativeprovider.shizuku

import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import android.os.Handler
import android.os.Looper
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import rikka.shizuku.Shizuku
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.util.concurrent.CompletableFuture
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException

internal class ShizukuNativeHandler(
    private val context: Context,
) : AndroidNativeOperationHandler {

    private val mainHandler = Handler(Looper.getMainLooper())
    private val permissionWaiters = mutableMapOf<Int, CompletableFuture<Int>>()
    private var requestCode = 1000

    init {
        Shizuku.addRequestPermissionResultListener { requestCode, result ->
            mainHandler.post {
                val waiter = permissionWaiters.remove(requestCode)
                waiter?.complete(result)
            }
        }

        Shizuku.addBinderDeadListener {
            permissionWaiters.values.forEach { it.complete(-1) }
            permissionWaiters.clear()
        }

        Shizuku.addBinderReceivedListener {
            mainHandler.post {
                if (ShizukuCommandServiceHolder.currentState() == ShizukuServiceState.DEAD) {
                    ShizukuCommandServiceHolder.unbindService()
                }
            }
        }
    }

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
        val holderState = ShizukuCommandServiceHolder.currentState()
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "supported" to state.supported,
                "installed" to state.installed,
                "binderAvailable" to state.binderAvailable,
                "permissionState" to state.permissionState,
                "state" to holderState.name.lowercase(),
                "reason" to state.reason,
            ),
        )
    }

    private suspend fun handleRequestPermission(request: NativeBridgeRequest): NativeBridgeResponse {
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

        if (Shizuku.checkSelfPermission() == PackageManager.PERMISSION_GRANTED) {
            ensureServiceReady(10000L)
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_SUCCESS,
                result = mapOf(
                    "authorized" to true,
                    "state" to "authorized",
                ),
            )
        }

        val result = try {
            requestPermissionAsync().get(60, TimeUnit.SECONDS)
        } catch (e: TimeoutException) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "SHIZUKU_PERMISSION_TIMEOUT",
                    message = "Shizuku permission request timed out",
                ),
            )
        } catch (e: Exception) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "SHIZUKU_PERMISSION_CANCELLED",
                    message = "Shizuku permission request was cancelled: ${e.message}",
                ),
            )
        }

        return when (result) {
            PackageManager.PERMISSION_GRANTED -> {
                ensureServiceReady(10000L)
                NativeBridgeResponse(
                    protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                    requestId = request.requestId,
                    status = NativeBridgeProtocol.STATUS_SUCCESS,
                    result = mapOf(
                        "authorized" to true,
                        "state" to "authorized",
                    ),
                )
            }
            -1 -> NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "SHIZUKU_SERVICE_DEAD",
                    message = "Shizuku service died during permission request",
                ),
            )
            else -> NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "SHIZUKU_PERMISSION_DENIED",
                    message = "Shizuku permission was denied by user",
                ),
            )
        }
    }

    private fun requestPermissionAsync(): CompletableFuture<Int> {
        val future = CompletableFuture<Int>()
        val code = requestCode++
        permissionWaiters[code] = future

        if (!Shizuku.pingBinder()) {
            future.complete(-1)
            permissionWaiters.remove(code)
            return future
        }

        if (Shizuku.isPreV11()) {
            future.complete(-1)
            permissionWaiters.remove(code)
        } else {
            try {
                Shizuku.requestPermission(code)
            } catch (e: Exception) {
                future.complete(-1)
                permissionWaiters.remove(code)
            }
        }

        return future
    }

    private suspend fun handleExecute(request: NativeBridgeRequest): NativeBridgeResponse {
        val payload = request.payload
        val executable = payload["executable"] as? String
        val args = payload["args"] as? List<*>
        val stdin = payload["stdin"] as? String
        val timeoutMs = (payload["timeoutMs"] as? Number)?.toLong() ?: 30000L
        val maxOutputBytes = (payload["maxOutputBytes"] as? Number)?.toLong() ?: 1048576L

        if (executable.isNullOrBlank()) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "SHIZUKU_INVALID_REQUEST",
                    message = "executable is required",
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

        val service = try {
            ensureServiceReady(10000L)
        } catch (e: BinderUnavailable) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "SHIZUKU_BINDER_UNAVAILABLE",
                    message = "Shizuku binder is not available",
                ),
            )
        } catch (e: PermissionRequired) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "SHIZUKU_PERMISSION_REQUIRED",
                    message = "Shizuku permission not granted",
                ),
            )
        } catch (e: ServiceBindFailed) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "SHIZUKU_SERVICE_UNAVAILABLE",
                    message = "Shizuku UserService could not be bound",
                ),
            )
        } catch (e: ServiceBindTimeout) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "SHIZUKU_BIND_TIMEOUT",
                    message = "Shizuku UserService bind timed out",
                ),
            )
        } catch (e: Exception) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "SHIZUKU_SERVICE_UNAVAILABLE",
                    message = "Shizuku UserService bind failed: ${e.message}",
                ),
            )
        }

        val requestJson = buildRequestJson(executable, args, stdin, timeoutMs, maxOutputBytes)

        return try {
            val resultJson = withContext(Dispatchers.IO) {
                service.executeCommand(requestJson)
            }
            parseAndBuildResponse(request.requestId, resultJson)
        } catch (e: Exception) {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "SHIZUKU_EXECUTION_ERROR",
                    message = "Shizuku execution failed: ${e.message}",
                ),
            )
        }
    }

    private fun buildRequestJson(
        executable: String,
        args: List<*>?,
        stdin: String?,
        timeoutMs: Long,
        maxOutputBytes: Long,
    ): String {
        val argsJson = (args ?: emptyList<Any>()).joinToString(",") { "\"${it.toString().replace("\"", "\\\"")}\"" }
        val stdinJson = stdin?.let { "\"${it.replace("\\", "\\\\").replace("\"", "\\\"").replace("\n", "\\n")}\"" } ?: "null"
        return """{"executable":"${executable.replace("\"", "\\\"")}","args":[$argsJson],"stdin":$stdinJson,"timeoutMs":$timeoutMs,"maxOutputBytes":$maxOutputBytes}"""
    }

    private fun parseAndBuildResponse(requestId: String, resultJson: String): NativeBridgeResponse {
        return try {
            val map = mutableMapOf<String, Any>()
            val cleaned = resultJson.trim().removePrefix("{").removeSuffix("}").trim()
            if (cleaned.isNotEmpty()) {
                var i = 0
                while (i < cleaned.length) {
                    while (i < cleaned.length && cleaned[i] in " \t\r\n") i++
                    if (i >= cleaned.length || cleaned[i] != '"') break
                    i++
                    val keyStart = i
                    while (i < cleaned.length && cleaned[i] != '"') i++
                    val key = cleaned.substring(keyStart, i)
                    i++
                    while (i < cleaned.length && cleaned[i] != ':') i++
                    i++
                    while (i < cleaned.length && cleaned[i] in " \t\r\n") i++
                    if (i >= cleaned.length) break
                    when {
                        cleaned[i] == '"' -> {
                            i++
                            val sb = StringBuilder()
                            while (i < cleaned.length && cleaned[i] != '"') {
                                if (cleaned[i] == '\\' && i + 1 < cleaned.length) {
                                    when (cleaned[i + 1]) {
                                        'n' -> sb.append('\n')
                                        'r' -> sb.append('\r')
                                        't' -> sb.append('\t')
                                        else -> sb.append(cleaned[i + 1])
                                    }
                                    i += 2
                                } else {
                                    sb.append(cleaned[i])
                                    i++
                                }
                            }
                            map[key] = sb.toString()
                            i++
                        }
                        cleaned[i] == '[' -> {
                            i++
                            while (i < cleaned.length && cleaned[i] != ']') i++
                            i++
                            map[key] = emptyList<String>()
                        }
                        cleaned[i].isDigit() || cleaned[i] == '-' -> {
                            val numStart = i
                            while (i < cleaned.length && cleaned[i] in "-0123456789.") i++
                            val numStr = cleaned.substring(numStart, i)
                            map[key] = numStr.toLongOrNull() ?: numStr.toDoubleOrNull() ?: 0L
                        }
                        cleaned.startsWith("true", i) -> {
                            map[key] = true
                            i += 4
                        }
                        cleaned.startsWith("false", i) -> {
                            map[key] = false
                            i += 5
                        }
                        else -> break
                    }
                    while (i < cleaned.length && cleaned[i] in " \t\r\n,") i++
                }
            }

            if (map.containsKey("error")) {
                val errorMap = map["error"] as? Map<*, *>
                NativeBridgeResponse(
                    protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                    requestId = requestId,
                    status = NativeBridgeProtocol.STATUS_ERROR,
                    error = NativeBridgeError(
                        code = errorMap?.get("code") as? String ?: "SHIZUKU_EXECUTION_ERROR",
                        message = errorMap?.get("message") as? String ?: "execution failed",
                    ),
                )
            } else {
                NativeBridgeResponse(
                    protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                    requestId = requestId,
                    status = NativeBridgeProtocol.STATUS_SUCCESS,
                    result = map,
                )
            }
        } catch (e: Exception) {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "SHIZUKU_RESPONSE_PARSE_ERROR",
                    message = "failed to parse execution result: ${e.message}",
                ),
            )
        }
    }

    private suspend fun ensureServiceReady(timeoutMs: Long): IPrivilegedCommandService {
        if (!Shizuku.pingBinder()) throw BinderUnavailable()
        if (Shizuku.checkSelfPermission() != PackageManager.PERMISSION_GRANTED) {
            throw PermissionRequired()
        }

        ShizukuCommandServiceHolder.currentService()?.let { return it }

        val latch = CountDownLatch(1)
        ShizukuCommandServiceHolder.setServiceConnectedListener {
            latch.countDown()
        }

        val bound = ShizukuCommandServiceHolder.bindService()
        if (!bound) {
            ShizukuCommandServiceHolder.setServiceConnectedListener(null)
            throw ServiceBindFailed()
        }

        val awaited = latch.await(timeoutMs, TimeUnit.MILLISECONDS)
        ShizukuCommandServiceHolder.setServiceConnectedListener(null)

        if (!awaited) {
            throw ServiceBindTimeout()
        }

        return ShizukuCommandServiceHolder.currentService()
            ?: throw ServiceBindFailed()
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
            Shizuku.checkSelfPermission() == PackageManager.PERMISSION_GRANTED
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

    fun cleanup() {
        ShizukuCommandServiceHolder.unbindService()
    }

    companion object {
        const val OP_STATUS = "shizuku.status"
        const val OP_REQUEST_PERMISSION = "shizuku.request_permission"
        const val OP_EXECUTE = "shizuku.execute"
    }
}

internal class BinderUnavailable : Exception("Shizuku binder not available")
internal class PermissionRequired : Exception("Shizuku permission not granted")
internal class ServiceBindFailed : Exception("Shizuku UserService bind failed")
internal class ServiceBindTimeout : Exception("Shizuku UserService bind timed out")
