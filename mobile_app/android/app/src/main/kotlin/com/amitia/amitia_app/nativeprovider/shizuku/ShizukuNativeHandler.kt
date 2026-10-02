package com.amitia.amitia_app.nativeprovider.shizuku

import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Handler
import android.os.Looper
import android.os.Parcel
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import rikka.shizuku.Shizuku
import rikka.shizuku.ShizukuBinderWrapper
import rikka.sui.Sui
import rikka.shizuku.SystemServiceHelper
import android.util.Base64
import java.util.concurrent.CompletableFuture
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException

internal class ShizukuNativeHandler(
    context: Context,
) : AndroidNativeOperationHandler {

    private val appContext = context.applicationContext
    private val preferences =
        appContext.getSharedPreferences(PREFERENCES_NAME, Context.MODE_PRIVATE)
    private val mainHandler = Handler(Looper.getMainLooper())
    private val permissionWaiters = mutableMapOf<Int, CompletableFuture<Int>>()
    private val permissionWaiterLock = Any()
    private var requestCode = 1000

    private val binderReceivedListener = object : Shizuku.OnBinderReceivedListener {
        override fun onBinderReceived() {
            mainHandler.post {
                if (ShizukuCommandServiceHolder.currentState() == ShizukuServiceState.DEAD) {
                    ShizukuCommandServiceHolder.unbindService()
                }
            }
        }
    }

    private val binderDeadListener = object : Shizuku.OnBinderDeadListener {
        override fun onBinderDead() {
            mainHandler.post {
                synchronized(permissionWaiterLock) {
                    permissionWaiters.values.forEach { it.complete(-1) }
                    permissionWaiters.clear()
                }
                ShizukuCommandServiceHolder.unbindService()
            }
        }
    }

    private val permissionResultListener =
        object : Shizuku.OnRequestPermissionResultListener {
            override fun onRequestPermissionResult(requestCode: Int, grantResult: Int) {
                mainHandler.post {
                    val waiter = synchronized(permissionWaiterLock) {
                        permissionWaiters.remove(requestCode)
                    }
                    waiter?.complete(grantResult)
                }
            }
        }

    init {
        try {
            Shizuku.addBinderReceivedListenerSticky(binderReceivedListener)
            Shizuku.addBinderDeadListener(binderDeadListener)
            Shizuku.addRequestPermissionResultListener(permissionResultListener)
        } catch (_: Exception) {
        }
    }

    override val operations: Set<String> =
        setOf(
            OP_STATUS,
            OP_INFO,
            OP_REQUEST_PERMISSION,
            OP_PERMISSION_CHECK,
            OP_EXECUTE,
            OP_SET_ENABLED,
            OP_OPEN_MANAGER,
            OP_TEST,
            OP_USER_SERVICE_STATUS,
            OP_USER_SERVICE_BIND,
            OP_USER_SERVICE_UNBIND,
            OP_PROCESS_START,
            OP_PROCESS_WRITE,
            OP_PROCESS_READ,
            OP_PROCESS_WAIT,
            OP_PROCESS_KILL,
            OP_SYSTEM_SERVICE_RESOLVE,
            OP_BINDER_TRANSACT,
        )

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        return when (request.operation) {
            OP_STATUS -> handleStatus(request)
            OP_INFO -> handleInfo(request)
            OP_REQUEST_PERMISSION -> handleRequestPermission(request)
            OP_PERMISSION_CHECK -> handlePermissionCheck(request)
            OP_EXECUTE -> handleExecute(request)
            OP_SET_ENABLED -> handleSetEnabled(request)
            OP_OPEN_MANAGER -> handleOpenManager(request)
            OP_TEST -> handleTest(request)
            OP_USER_SERVICE_STATUS -> handleUserServiceStatus(request)
            OP_USER_SERVICE_BIND -> handleUserServiceBind(request)
            OP_USER_SERVICE_UNBIND -> handleUserServiceUnbind(request)
            OP_PROCESS_START -> handleProcessOperation(request, OP_PROCESS_START)
            OP_PROCESS_WRITE -> handleProcessOperation(request, OP_PROCESS_WRITE)
            OP_PROCESS_READ -> handleProcessOperation(request, OP_PROCESS_READ)
            OP_PROCESS_WAIT -> handleProcessOperation(request, OP_PROCESS_WAIT)
            OP_PROCESS_KILL -> handleProcessOperation(request, OP_PROCESS_KILL)
            OP_SYSTEM_SERVICE_RESOLVE -> handleSystemServiceResolve(request)
            OP_BINDER_TRANSACT -> handleBinderTransact(request)
            else -> unsupportedOperation(request)
        }
    }

    private fun handleInfo(request: NativeBridgeRequest): NativeBridgeResponse {
        val state = detectShizukuState()
        return successResponse(
            request.requestId,
            capabilityStateToMap(state) + mapOf(
                "binderAlive" to pingBinder(),
                "preV11" to try {
                    Shizuku.isPreV11()
                } catch (_: Exception) {
                    false
                },
                "latestServiceVersion" to try {
                    Shizuku.getLatestServiceVersion()
                } catch (_: Exception) {
                    -1
                },
                "serverPatchVersion" to try {
                    Shizuku.getServerPatchVersion()
                } catch (_: Exception) {
                    -1
                },
                "selinuxContext" to try {
                    Shizuku.getSELinuxContext()
                } catch (_: Exception) {
                    ""
                },
            ),
        )
    }

    private fun handlePermissionCheck(request: NativeBridgeRequest): NativeBridgeResponse {
        val permission = (request.payload["permission"] as? String)?.trim().orEmpty()
        if (permission.isEmpty()) {
            return errorResponse(request.requestId, "SHIZUKU_INVALID_REQUEST", "permission is required")
        }
        return try {
            val result = Shizuku.checkRemotePermission(permission)
            successResponse(
                request.requestId,
                mapOf(
                    "permission" to permission,
                    "grantResult" to result,
                    "granted" to (result == PackageManager.PERMISSION_GRANTED),
                ),
            )
        } catch (error: Exception) {
            errorResponse(request.requestId, "SHIZUKU_PERMISSION_CHECK_FAILED", error.message ?: "permission check failed")
        }
    }

    private fun handleUserServiceStatus(request: NativeBridgeRequest): NativeBridgeResponse {
        return successResponse(
            request.requestId,
            mapOf(
                "state" to ShizukuCommandServiceHolder.currentState().name.lowercase(),
                "available" to (ShizukuCommandServiceHolder.currentService() != null),
                "authorized" to authorized(),
                "enabled" to isAiEnabled(),
            ),
        )
    }

    private suspend fun handleUserServiceBind(request: NativeBridgeRequest): NativeBridgeResponse {
        if (!isAiEnabled()) {
            return errorResponse(request.requestId, "SHIZUKU_DISABLED", "AI access to Shizuku is disabled")
        }
        return try {
            ensureServiceReady(10000L)
            successResponse(
                request.requestId,
                mapOf(
                    "bound" to true,
                    "state" to ShizukuCommandServiceHolder.currentState().name.lowercase(),
                ),
            )
        } catch (error: Exception) {
            errorResponse(request.requestId, "SHIZUKU_SERVICE_UNAVAILABLE", error.message ?: "failed to bind user service")
        }
    }

    private fun handleUserServiceUnbind(request: NativeBridgeRequest): NativeBridgeResponse {
        ShizukuCommandServiceHolder.unbindService()
        return successResponse(
            request.requestId,
            mapOf(
                "bound" to false,
                "state" to ShizukuCommandServiceHolder.currentState().name.lowercase(),
            ),
        )
    }

    private suspend fun handleProcessOperation(
        request: NativeBridgeRequest,
        operation: String,
    ): NativeBridgeResponse {
        if (!isAiEnabled()) {
            return errorResponse(request.requestId, "SHIZUKU_DISABLED", "AI access to Shizuku is disabled")
        }
        val service = try {
            ensureServiceReady(10000L)
        } catch (error: Exception) {
            return errorResponse(request.requestId, "SHIZUKU_SERVICE_UNAVAILABLE", error.message ?: "Shizuku user service unavailable")
        }
        val payloadJson = JSONObject(request.payload).toString()
        val resultJson = try {
            withContext(Dispatchers.IO) {
                when (operation) {
                    OP_PROCESS_START -> service.startProcess(payloadJson)
                    OP_PROCESS_WRITE -> service.writeProcess(payloadJson)
                    OP_PROCESS_READ -> service.readProcess(payloadJson)
                    OP_PROCESS_WAIT -> service.waitProcess(payloadJson)
                    OP_PROCESS_KILL -> service.killProcess(payloadJson)
                    else -> """{"error":{"code":"UNSUPPORTED","message":"unsupported process operation"}}"""
                }
            }
        } catch (error: Exception) {
            return errorResponse(request.requestId, "SHIZUKU_PROCESS_FAILED", error.message ?: "process operation failed")
        }
        return parseAndBuildResponse(request.requestId, resultJson)
    }

    private fun handleSystemServiceResolve(request: NativeBridgeRequest): NativeBridgeResponse {
        if (!isAiEnabled()) {
            return errorResponse(request.requestId, "SHIZUKU_DISABLED", "AI access to Shizuku is disabled")
        }
        val serviceName = (request.payload["serviceName"] as? String)?.trim().orEmpty()
        if (serviceName.isEmpty()) {
            return errorResponse(request.requestId, "SHIZUKU_INVALID_REQUEST", "serviceName is required")
        }
        val binder = try {
            SystemServiceHelper.getSystemService(serviceName)
        } catch (error: Exception) {
            null
        } ?: return errorResponse(request.requestId, "SHIZUKU_SYSTEM_SERVICE_NOT_FOUND", "system service not found: $serviceName")
        return successResponse(
            request.requestId,
            mapOf(
                "serviceName" to serviceName,
                "available" to true,
                "binderAlive" to binder.isBinderAlive,
                "interfaceDescriptor" to try {
                    binder.interfaceDescriptor.orEmpty()
                } catch (_: Exception) {
                    ""
                },
            ),
        )
    }

    private fun handleBinderTransact(request: NativeBridgeRequest): NativeBridgeResponse {
        if (!isAiEnabled()) {
            return errorResponse(request.requestId, "SHIZUKU_DISABLED", "AI access to Shizuku is disabled")
        }
        val serviceName = (request.payload["serviceName"] as? String)?.trim().orEmpty()
        val transactionCode = (request.payload["transactionCode"] as? Number)?.toInt() ?: -1
        val flags = (request.payload["flags"] as? Number)?.toInt() ?: 0
        val dataBase64 = (request.payload["dataBase64"] as? String).orEmpty()
        if (serviceName.isEmpty() || transactionCode < 0 || dataBase64.isEmpty()) {
            return errorResponse(request.requestId, "SHIZUKU_INVALID_REQUEST", "serviceName, transactionCode and dataBase64 are required")
        }
        val maxReplyBytes = (request.payload["maxReplyBytes"] as? Number)?.toInt()?.coerceIn(1, 8 * 1024 * 1024) ?: (1024 * 1024)
        return try {
            val binder = SystemServiceHelper.getSystemService(serviceName)
                ?: return errorResponse(request.requestId, "SHIZUKU_SYSTEM_SERVICE_NOT_FOUND", "system service not found: $serviceName")
            val dataBytes = Base64.decode(dataBase64, Base64.DEFAULT)
            val data = Parcel.obtain()
            val reply = Parcel.obtain()
            try {
                data.unmarshall(dataBytes, 0, dataBytes.size)
                data.setDataPosition(0)
                val result = ShizukuBinderWrapper(binder).transact(transactionCode, data, reply, flags)
                val replyBytes = if (reply.dataSize() > 0) reply.marshall() else ByteArray(0)
                val bounded = if (replyBytes.size > maxReplyBytes) replyBytes.copyOf(maxReplyBytes) else replyBytes
                successResponse(
                    request.requestId,
                    mapOf(
                        "success" to result,
                        "serviceName" to serviceName,
                        "transactionCode" to transactionCode,
                        "replyBase64" to Base64.encodeToString(bounded, Base64.NO_WRAP),
                        "replySize" to replyBytes.size,
                        "truncated" to (replyBytes.size > maxReplyBytes),
                    ),
                )
            } finally {
                data.recycle()
                reply.recycle()
            }
        } catch (error: Exception) {
            errorResponse(request.requestId, "SHIZUKU_BINDER_TRANSACT_FAILED", error.message ?: "binder transaction failed")
        }
    }

    private fun handleStatus(request: NativeBridgeRequest): NativeBridgeResponse {
        return successResponse(request.requestId, capabilityStateToMap(detectShizukuState()))
    }

    private suspend fun handleSetEnabled(request: NativeBridgeRequest): NativeBridgeResponse {
        val enabled = request.payload["enabled"] as? Boolean
            ?: return errorResponse(
                request.requestId,
                "SHIZUKU_INVALID_REQUEST",
                "enabled must be a boolean",
            )

        if (!enabled) {
            preferences.edit().putBoolean(KEY_AI_ENABLED, false).apply()
            ShizukuCommandServiceHolder.unbindService()
            return successResponse(
                request.requestId,
                mapOf(
                    "enabled" to false,
                    "state" to "disabled",
                ),
            )
        }

        var state = detectShizukuState()
        if (!state.binderAvailable) {
            val code = if (state.managerInstalled) {
                "SHIZUKU_NOT_RUNNING"
            } else {
                "SHIZUKU_NOT_INSTALLED"
            }
            return errorResponse(request.requestId, code, state.reason)
        }

        if (state.permissionState != PERMISSION_AUTHORIZED) {
            val permissionResponse = handleRequestPermission(request)
            if (permissionResponse.status != NativeBridgeProtocol.STATUS_SUCCESS) {
                return permissionResponse
            }
            state = detectShizukuState()
        }

        val smokeTest = executeCommand(
            requestId = request.requestId,
            executable = "id",
            args = emptyList(),
            stdin = null,
            timeoutMs = 5000L,
            maxOutputBytes = 65536L,
            requireEnabled = false,
        )
        if (smokeTest.status != NativeBridgeProtocol.STATUS_SUCCESS) {
            return smokeTest
        }

        preferences.edit().putBoolean(KEY_AI_ENABLED, true).apply()
        return successResponse(
            request.requestId,
            mapOf(
                "enabled" to true,
                "state" to "ready",
                "provider" to state.provider,
                "version" to state.version,
                "uid" to state.uid,
                "test" to smokeTest.result,
            ),
        )
    }

    private suspend fun handleTest(request: NativeBridgeRequest): NativeBridgeResponse {
        return executeCommand(
            requestId = request.requestId,
            executable = "id",
            args = emptyList(),
            stdin = null,
            timeoutMs = 5000L,
            maxOutputBytes = 65536L,
            requireEnabled = true,
        )
    }

    private fun handleOpenManager(request: NativeBridgeRequest): NativeBridgeResponse {
        val packageName = installedManagerPackage()
        if (packageName != null) {
            val launchIntent = appContext.packageManager.getLaunchIntentForPackage(packageName)
            if (launchIntent != null) {
                launchIntent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                return try {
                    appContext.startActivity(launchIntent)
                    successResponse(
                        request.requestId,
                        mapOf(
                            "openedManager" to true,
                            "openedInstallPage" to false,
                            "packageName" to packageName,
                        ),
                    )
                } catch (error: Exception) {
                    errorResponse(
                        request.requestId,
                        "SHIZUKU_MANAGER_OPEN_FAILED",
                        error.message ?: "failed to open Shizuku manager",
                    )
                }
            }
        }

        val installUrl = if (isSuiBinder()) SUI_URL else SHIZUKU_URL
        val installIntent = Intent(Intent.ACTION_VIEW, Uri.parse(installUrl))
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        return try {
            appContext.startActivity(installIntent)
            successResponse(
                request.requestId,
                mapOf(
                    "openedManager" to false,
                    "openedInstallPage" to true,
                    "installUrl" to installUrl,
                ),
            )
        } catch (error: Exception) {
            errorResponse(
                request.requestId,
                "SHIZUKU_INSTALL_PAGE_OPEN_FAILED",
                error.message ?: "failed to open Shizuku install page",
            )
        }
    }

    private suspend fun handleRequestPermission(request: NativeBridgeRequest): NativeBridgeResponse {
        val state = detectShizukuState()
        if (!state.binderAvailable) {
            val code = if (state.managerInstalled) {
                "SHIZUKU_NOT_RUNNING"
            } else {
                "SHIZUKU_NOT_INSTALLED"
            }
            return errorResponse(request.requestId, code, state.reason)
        }

        if (state.permissionState == PERMISSION_AUTHORIZED) {
            return try {
                ensureServiceReady(10000L)
                successResponse(
                    request.requestId,
                    mapOf(
                        "authorized" to true,
                        "state" to "ready",
                    ),
                )
            } catch (error: Exception) {
                errorResponse(
                    request.requestId,
                    "SHIZUKU_SERVICE_UNAVAILABLE",
                    error.message ?: "Shizuku UserService is unavailable",
                )
            }
        }

        val rationaleRequired = try {
            Shizuku.shouldShowRequestPermissionRationale()
        } catch (_: Exception) {
            false
        }
        if (rationaleRequired) {
            return errorResponse(
                request.requestId,
                "SHIZUKU_PERMISSION_PERMANENTLY_DENIED",
                "Shizuku permission is disabled; open Shizuku manager and grant permission manually",
            )
        }

        val result = try {
            requestPermissionAsync().get(60, TimeUnit.SECONDS)
        } catch (_: TimeoutException) {
            return errorResponse(
                request.requestId,
                "SHIZUKU_PERMISSION_TIMEOUT",
                "Shizuku permission request timed out",
            )
        } catch (error: Exception) {
            return errorResponse(
                request.requestId,
                "SHIZUKU_PERMISSION_CANCELLED",
                "Shizuku permission request was cancelled: ${error.message}",
            )
        }

        return when (result) {
            PackageManager.PERMISSION_GRANTED -> {
                try {
                    ensureServiceReady(10000L)
                    successResponse(
                        request.requestId,
                        mapOf(
                            "authorized" to true,
                            "state" to "ready",
                        ),
                    )
                } catch (error: Exception) {
                    errorResponse(
                        request.requestId,
                        "SHIZUKU_SERVICE_UNAVAILABLE",
                        error.message ?: "Shizuku UserService is unavailable",
                    )
                }
            }
            -1 -> errorResponse(
                request.requestId,
                "SHIZUKU_SERVICE_DEAD",
                "Shizuku service died during permission request",
            )
            else -> errorResponse(
                request.requestId,
                "SHIZUKU_PERMISSION_DENIED",
                "Shizuku permission was denied by user",
            )
        }
    }

    private fun requestPermissionAsync(): CompletableFuture<Int> {
        val future = CompletableFuture<Int>()
        val code = requestCode++
        synchronized(permissionWaiterLock) {
            permissionWaiters[code] = future
        }

        if (!pingBinder()) {
            completePermissionWaiter(code, -1)
            return future
        }

        if (Shizuku.isPreV11()) {
            completePermissionWaiter(code, PackageManager.PERMISSION_GRANTED)
        } else {
            try {
                Shizuku.requestPermission(code)
            } catch (_: Exception) {
                completePermissionWaiter(code, -1)
            }
        }
        return future
    }

    private fun completePermissionWaiter(code: Int, result: Int) {
        val waiter = synchronized(permissionWaiterLock) {
            permissionWaiters.remove(code)
        }
        waiter?.complete(result)
    }

    private suspend fun handleExecute(request: NativeBridgeRequest): NativeBridgeResponse {
        val payload = request.payload
        val command = (payload["command"] as? String)?.trim().orEmpty()
        var executable = (payload["executable"] as? String)?.trim().orEmpty()
        var args = (payload["args"] as? List<*>)?.map { it.toString() } ?: emptyList()
        val stdin = payload["stdin"] as? String
        val env = (payload["env"] as? Map<*, *>)
            ?.entries
            ?.associate { it.key.toString() to it.value?.toString().orEmpty() }
            ?: emptyMap()
        val workDir = (payload["workDir"] as? String)
            ?: (payload["cwd"] as? String)
        val timeoutMs = request.executionTimeoutMillis((payload["timeoutMs"] as? Number)?.toLong() ?: 30000L)
        val maxOutputBytes = (payload["maxOutputBytes"] as? Number)?.toLong() ?: 1048576L

        if (command.isNotEmpty() && executable.isNotEmpty()) {
            return errorResponse(
                request.requestId,
                "SHIZUKU_INVALID_REQUEST",
                "command and executable are mutually exclusive",
            )
        }

        if (command.isNotEmpty()) {
            executable = "/system/bin/sh"
            args = listOf("-c", command)
        }

        if (executable.isBlank()) {
            return errorResponse(
                request.requestId,
                "SHIZUKU_INVALID_REQUEST",
                "command or executable is required",
            )
        }

        return executeCommand(
            requestId = request.requestId,
            executable = executable,
            args = args,
            stdin = stdin,
            env = env,
            workDir = workDir,
            timeoutMs = timeoutMs,
            maxOutputBytes = maxOutputBytes,
            requireEnabled = true,
        )
    }

    private suspend fun executeCommand(
        requestId: String,
        executable: String,
        args: List<String>,
        stdin: String?,
        env: Map<String, String> = emptyMap(),
        workDir: String? = null,
        timeoutMs: Long,
        maxOutputBytes: Long,
        requireEnabled: Boolean,
    ): NativeBridgeResponse {
        if (requireEnabled && !isAiEnabled()) {
            return errorResponse(
                requestId,
                "SHIZUKU_DISABLED",
                "AI access to Shizuku is disabled",
            )
        }

        val state = detectShizukuState()
        if (state.permissionState != PERMISSION_AUTHORIZED) {
            return errorResponse(
                requestId,
                "SHIZUKU_PERMISSION_REQUIRED",
                "Shizuku permission not granted",
            )
        }

        val service = try {
            ensureServiceReady(10000L)
        } catch (error: BinderUnavailable) {
            return errorResponse(
                requestId,
                "SHIZUKU_BINDER_UNAVAILABLE",
                error.message ?: "Shizuku binder is not available",
            )
        } catch (error: PermissionRequired) {
            return errorResponse(
                requestId,
                "SHIZUKU_PERMISSION_REQUIRED",
                error.message ?: "Shizuku permission not granted",
            )
        } catch (error: ServiceBindFailed) {
            return errorResponse(
                requestId,
                "SHIZUKU_SERVICE_UNAVAILABLE",
                error.message ?: "Shizuku UserService could not be bound",
            )
        } catch (error: ServiceBindTimeout) {
            return errorResponse(
                requestId,
                "SHIZUKU_BIND_TIMEOUT",
                error.message ?: "Shizuku UserService bind timed out",
            )
        } catch (error: Exception) {
            return errorResponse(
                requestId,
                "SHIZUKU_SERVICE_UNAVAILABLE",
                "Shizuku UserService bind failed: ${error.message}",
            )
        }

        val requestJson = JSONObject()
            .put("executable", executable)
            .put("args", JSONArray(args))
            .put("stdin", stdin ?: JSONObject.NULL)
            .put("env", JSONObject(env))
            .put("workDir", workDir ?: JSONObject.NULL)
            .put("timeoutMs", timeoutMs)
            .put("maxOutputBytes", maxOutputBytes)
            .toString()

        return try {
            val resultJson = withContext(Dispatchers.IO) {
                service.executeCommand(requestJson)
            }
            parseAndBuildResponse(requestId, resultJson)
        } catch (error: Exception) {
            errorResponse(
                requestId,
                "SHIZUKU_EXECUTION_ERROR",
                "Shizuku execution failed: ${error.message}",
            )
        }
    }

    private fun parseAndBuildResponse(requestId: String, resultJson: String): NativeBridgeResponse {
        return try {
            val json = JSONObject(resultJson)
            if (json.has("error")) {
                val error = json.optJSONObject("error")
                return errorResponse(
                    requestId,
                    error?.optString("code").orEmpty().ifBlank { "SHIZUKU_EXECUTION_ERROR" },
                    error?.optString("message").orEmpty().ifBlank { "execution failed" },
                )
            }

            val timedOut = json.optBoolean("timedOut", false)
            val exitCodeAvailable = json.optBoolean("exitCodeAvailable", false)
            val exitCode = json.optInt("exitCode", -1)
            if (timedOut) {
                return errorResponse(
                    requestId,
                    "SHIZUKU_TIMEOUT",
                    "Shizuku command timed out",
                )
            }
            if (exitCodeAvailable && exitCode != 0) {
                val stderr = json.optString("stderr").trim()
                val stdout = json.optString("stdout").trim()
                return errorResponse(
                    requestId,
                    "SHIZUKU_COMMAND_FAILED",
                    stderr.ifBlank { stdout }.ifBlank { "command exited with code $exitCode" },
                )
            }

            val result = linkedMapOf<String, Any?>()
            val keys = json.keys()
            while (keys.hasNext()) {
                val key = keys.next()
                result[key] = json.opt(key)
            }
            successResponse(requestId, result)
        } catch (error: Exception) {
            errorResponse(
                requestId,
                "SHIZUKU_RESPONSE_PARSE_ERROR",
                "failed to parse execution result: ${error.message}",
            )
        }
    }

    private suspend fun ensureServiceReady(timeoutMs: Long): IPrivilegedCommandService {
        if (!pingBinder()) throw BinderUnavailable()
        if (Shizuku.checkSelfPermission() != PackageManager.PERMISSION_GRANTED) {
            throw PermissionRequired()
        }

        ShizukuCommandServiceHolder.currentService()?.let { return it }

        val latch = CountDownLatch(1)
        val listener = {
            latch.countDown()
        }
        ShizukuCommandServiceHolder.addServiceConnectedListener(listener)

        val bound = ShizukuCommandServiceHolder.bindService()
        if (!bound) {
            ShizukuCommandServiceHolder.removeServiceConnectedListener(listener)
            throw ServiceBindFailed()
        }

        val awaited = latch.await(timeoutMs, TimeUnit.MILLISECONDS)
        ShizukuCommandServiceHolder.removeServiceConnectedListener(listener)
        if (!awaited) {
            throw ServiceBindTimeout()
        }

        return ShizukuCommandServiceHolder.currentService()
            ?: throw ServiceBindFailed()
    }

    private fun detectShizukuState(): ShizukuCapabilityState {
        val managerPackage = installedManagerPackage()
        val binderAvailable = pingBinder()
        val enabled = isAiEnabled()

        if (!binderAvailable) {
            val state = if (managerPackage != null) "not_running" else "not_installed"
            val reason = if (managerPackage != null) {
                "Shizuku manager is installed but the service is not running"
            } else {
                "No Shizuku-compatible manager or live privileged binder was found"
            }
            return ShizukuCapabilityState(
                installed = managerPackage != null,
                managerInstalled = managerPackage != null,
                binderAvailable = false,
                permissionState = "binder_unavailable",
                state = state,
                reason = reason,
                enabled = enabled,
                provider = providerName(managerPackage, false),
                canRequestPermission = false,
            )
        }

        val granted = try {
            Shizuku.checkSelfPermission() == PackageManager.PERMISSION_GRANTED
        } catch (_: Exception) {
            false
        }
        val uid = try {
            Shizuku.getUid()
        } catch (_: Exception) {
            -1
        }
        val version = try {
            Shizuku.getVersion()
        } catch (_: Exception) {
            -1
        }
        val sui = isSuiBinder()
        val state = when {
            !granted -> "permission_required"
            !enabled -> "disabled"
            else -> "ready"
        }
        val reason = when (state) {
            "permission_required" -> "Shizuku binder is available, but permission is not granted"
            "disabled" -> "Shizuku is authorized, but AI access is disabled"
            else -> "Shizuku is ready for AI automation"
        }
        return ShizukuCapabilityState(
            installed = true,
            managerInstalled = managerPackage != null,
            binderAvailable = true,
            permissionState = if (granted) PERMISSION_AUTHORIZED else "permission_required",
            state = state,
            reason = reason,
            enabled = enabled,
            provider = providerName(managerPackage, sui),
            version = version,
            uid = uid,
            canRequestPermission = !granted && !shouldShowPermissionRationale(),
        )
    }

    private fun capabilityStateToMap(state: ShizukuCapabilityState): Map<String, Any?> {
        return mapOf(
            "supported" to state.supported,
            "installed" to state.installed,
            "managerInstalled" to state.managerInstalled,
            "binderAvailable" to state.binderAvailable,
            "permissionState" to state.permissionState,
            "state" to state.state,
            "reason" to state.reason,
            "enabled" to state.enabled,
            "provider" to state.provider,
            "version" to state.version,
            "uid" to state.uid,
            "canRequestPermission" to state.canRequestPermission,
            "serviceState" to ShizukuCommandServiceHolder.currentState().name.lowercase(),
        )
    }

    private fun installedManagerPackage(): String? {
        return MANAGER_PACKAGES.firstOrNull { packageName ->
            try {
                appContext.packageManager.getPackageInfo(packageName, 0)
                true
            } catch (_: PackageManager.NameNotFoundException) {
                false
            } catch (_: Exception) {
                false
            }
        }
    }

    private fun providerName(managerPackage: String?, sui: Boolean): String {
        return when {
            sui -> "sui"
            managerPackage == SHIZUKU_PACKAGE -> "shizuku"
            managerPackage == AXMANAGER_PACKAGE -> "axmanager"
            managerPackage != null -> managerPackage
            else -> "none"
        }
    }

    private fun isSuiBinder(): Boolean {
        return try {
            Sui.isSui()
        } catch (_: Exception) {
            false
        }
    }

    private fun pingBinder(): Boolean {
        return try {
            Shizuku.pingBinder()
        } catch (_: Exception) {
            false
        }
    }

    private fun shouldShowPermissionRationale(): Boolean {
        return try {
            Shizuku.shouldShowRequestPermissionRationale()
        } catch (_: Exception) {
            false
        }
    }

    private fun isAiEnabled(): Boolean {
        return preferences.getBoolean(KEY_AI_ENABLED, false)
    }

    private fun authorized(): Boolean {
        return try {
            pingBinder() && Shizuku.checkSelfPermission() == PackageManager.PERMISSION_GRANTED
        } catch (_: Exception) {
            false
        }
    }

    private fun successResponse(
        requestId: String,
        result: Map<String, Any?>,
    ): NativeBridgeResponse {
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = result,
        )
    }

    private fun errorResponse(
        requestId: String,
        code: String,
        message: String,
    ): NativeBridgeResponse {
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = requestId,
            status = NativeBridgeProtocol.STATUS_ERROR,
            error = NativeBridgeError(
                code = code,
                message = message,
            ),
        )
    }

    private fun unsupportedOperation(request: NativeBridgeRequest): NativeBridgeResponse {
        return errorResponse(
            request.requestId,
            NativeBridgeProtocol.ERR_OPERATION_NOT_SUPPORTED,
            "unknown shizuku operation: ${request.operation}",
        )
    }

    fun cleanup() {
        try {
            Shizuku.removeBinderReceivedListener(binderReceivedListener)
            Shizuku.removeBinderDeadListener(binderDeadListener)
            Shizuku.removeRequestPermissionResultListener(permissionResultListener)
        } catch (_: Exception) {
        }
        ShizukuCommandServiceHolder.unbindService()
    }

    companion object {
        const val OP_STATUS = "shizuku.status"
        const val OP_INFO = "shizuku.info"
        const val OP_REQUEST_PERMISSION = "shizuku.request_permission"
        const val OP_PERMISSION_CHECK = "shizuku.permission_check"
        const val OP_EXECUTE = "shizuku.execute"
        const val OP_SET_ENABLED = "shizuku.set_enabled"
        const val OP_OPEN_MANAGER = "shizuku.open_manager"
        const val OP_TEST = "shizuku.test"
        const val OP_USER_SERVICE_STATUS = "shizuku.user_service.status"
        const val OP_USER_SERVICE_BIND = "shizuku.user_service.bind"
        const val OP_USER_SERVICE_UNBIND = "shizuku.user_service.unbind"
        const val OP_PROCESS_START = "shizuku.process.start"
        const val OP_PROCESS_WRITE = "shizuku.process.write"
        const val OP_PROCESS_READ = "shizuku.process.read"
        const val OP_PROCESS_WAIT = "shizuku.process.wait"
        const val OP_PROCESS_KILL = "shizuku.process.kill"
        const val OP_SYSTEM_SERVICE_RESOLVE = "shizuku.system_service.resolve"
        const val OP_BINDER_TRANSACT = "shizuku.binder.transact"

        private const val PREFERENCES_NAME = "amitia_shizuku"
        private const val KEY_AI_ENABLED = "ai_enabled"
        private const val PERMISSION_AUTHORIZED = "authorized"
        private const val SHIZUKU_PACKAGE = "moe.shizuku.privileged.api"
        private const val AXMANAGER_PACKAGE = "frb.axeron.manager"
        private const val SHIZUKU_URL =
            "https://github.com/RikkaApps/Shizuku/releases"
        private const val SUI_URL =
            "https://github.com/RikkaApps/Sui"
        private val MANAGER_PACKAGES = listOf(SHIZUKU_PACKAGE, AXMANAGER_PACKAGE)
    }
}

internal class BinderUnavailable : Exception("Shizuku binder not available")
internal class PermissionRequired : Exception("Shizuku permission not granted")
internal class ServiceBindFailed : Exception("Shizuku UserService bind failed")
internal class ServiceBindTimeout : Exception("Shizuku UserService bind timed out")
