package com.amitia.amitia_app.virtualdisplay.host

import android.content.Context
import com.amitia.amitia_app.nativeprovider.shizuku.IPrivilegedCommandService
import com.amitia.amitia_app.nativeprovider.shizuku.ShizukuCommandServiceHolder
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.util.UUID

internal object VirtualDisplayPermissionStore {
    fun isEnabled(context: Context): Boolean {
        return context.getSharedPreferences(
            VirtualDisplayHostContract.PREFERENCES_NAME,
            Context.MODE_PRIVATE,
        ).getBoolean(VirtualDisplayHostContract.KEY_AI_ENABLED, false)
    }

    fun setEnabled(context: Context, enabled: Boolean) {
        context.getSharedPreferences(
            VirtualDisplayHostContract.PREFERENCES_NAME,
            Context.MODE_PRIVATE,
        ).edit().putBoolean(VirtualDisplayHostContract.KEY_AI_ENABLED, enabled).apply()
    }
}

internal class VirtualDisplayHostManager(
    context: Context,
) {
    private val appContext = context.applicationContext
    private val preferences = appContext.getSharedPreferences(
        VirtualDisplayHostContract.PREFERENCES_NAME,
        Context.MODE_PRIVATE,
    )
    private val startMutex = Mutex()

    fun hostRunning(): Boolean = VirtualDisplayHostBinderRegistry.current() != null

    suspend fun status(): Map<String, Any?> {
        val current = VirtualDisplayHostBinderRegistry.current()
            ?: return mapOf(
                "hostRunning" to false,
                "state" to "stopped",
                "provider" to "app_process",
            )
        return executeWithService(
            current,
            "virtual_display.status",
            emptyMap(),
        ) + mapOf("hostRunning" to true)
    }

    suspend fun ensureStarted(): Map<String, Any?> {
        return startMutex.withLock {
            VirtualDisplayHostBinderRegistry.current()?.let { current ->
                return@withLock executeWithService(
                    current,
                    "virtual_display.ping",
                    emptyMap(),
                )
            }
            val service = ensureShizukuService()
            val token = preferences.getString(VirtualDisplayHostContract.KEY_TOKEN, null)
                ?.takeIf { it.isNotBlank() }
                ?: UUID.randomUUID().toString().also { value ->
                    preferences.edit().putString(VirtualDisplayHostContract.KEY_TOKEN, value).apply()
                }
            val startResult = withContext(Dispatchers.IO) {
                service.startProcess(startRequest(token).toString())
            }
            val startObject = JSONObject(startResult)
            startObject.optJSONObject("error")?.let { error ->
                throw VirtualDisplayHostError(
                    error.optString("code", "VIRTUAL_DISPLAY_HOST_START_FAILED"),
                    error.optString("message", "failed to start virtual display host"),
                )
            }
            val processId = startObject.optString("processId")
            preferences.edit().putString(VirtualDisplayHostContract.KEY_PROCESS_ID, processId).apply()
            repeat(150) {
                VirtualDisplayHostBinderRegistry.current()?.let { current ->
                    return@withLock executeWithService(
                        current,
                        "virtual_display.ping",
                        emptyMap(),
                    )
                }
                delay(100L)
            }
            stopHostProcess()
            throw VirtualDisplayHostError(
                "VIRTUAL_DISPLAY_HOST_START_TIMEOUT",
                "virtual display host did not publish its binder",
            )
        }
    }

    suspend fun execute(operation: String, payload: Map<String, Any?>): Map<String, Any?> {
        val current = VirtualDisplayHostBinderRegistry.current() ?: run {
            ensureStarted()
            VirtualDisplayHostBinderRegistry.current()
        } ?: throw VirtualDisplayHostError(
            "VIRTUAL_DISPLAY_HOST_UNAVAILABLE",
            "virtual display host is unavailable",
        )
        return executeWithService(current, operation, payload)
    }

    suspend fun stop() {
        val current = VirtualDisplayHostBinderRegistry.current()
        if (current != null) {
            try {
                current.shutdown()
            } catch (_: Throwable) {
            }
        }
        VirtualDisplayHostBinderRegistry.clear()
        stopHostProcess()
        preferences.edit().remove(VirtualDisplayHostContract.KEY_PROCESS_ID).apply()
    }

    private suspend fun ensureShizukuService(): IPrivilegedCommandService {
        ShizukuCommandServiceHolder.currentService()?.let { return it }
        if (!ShizukuCommandServiceHolder.bindService()) {
            throw VirtualDisplayHostError(
                "VIRTUAL_DISPLAY_SHIZUKU_REQUIRED",
                "Shizuku authorization is required",
            )
        }
        repeat(100) {
            ShizukuCommandServiceHolder.currentService()?.let { return it }
            delay(100L)
        }
        throw VirtualDisplayHostError(
            "VIRTUAL_DISPLAY_SHIZUKU_UNAVAILABLE",
            "Shizuku user service is unavailable",
        )
    }

    private fun startRequest(token: String): JSONObject {
        val args = JSONArray()
            .put("/")
            .put(VirtualDisplayHostContract.HOST_CLASS)
            .put(appContext.packageName)
            .put(token)
        return JSONObject()
            .put("executable", "app_process")
            .put("args", args)
            .put(
                "env",
                JSONObject().put("CLASSPATH", appContext.applicationInfo.sourceDir),
            )
            .put("maxBufferBytes", 1024 * 1024)
    }

    private suspend fun executeWithService(
        service: IVirtualDisplayHost,
        operation: String,
        payload: Map<String, Any?>,
    ): Map<String, Any?> {
        val request = JSONObject()
        payload.forEach { (key, value) ->
            request.put(key, value)
        }
        request.put("operation", operation)
        val raw = withContext(Dispatchers.IO) {
            service.execute(request.toString())
        }
        val response = JSONObject(raw)
        val error = response.optJSONObject("error")
        if (response.optString("status") != "success" || error != null) {
            throw VirtualDisplayHostError(
                error?.optString("code").orEmpty().ifBlank { "VIRTUAL_DISPLAY_HOST_ERROR" },
                error?.optString("message").orEmpty().ifBlank { "virtual display host error" },
            )
        }
        val result = response.optJSONObject("result") ?: JSONObject()
        return jsonObjectToMap(result)
    }

    private fun stopHostProcess() {
        val processId = preferences.getString(VirtualDisplayHostContract.KEY_PROCESS_ID, null)
            ?.takeIf { it.isNotBlank() }
            ?: return
        try {
            val service = ShizukuCommandServiceHolder.currentService() ?: return
            service.killProcess(
                JSONObject()
                    .put("processId", processId)
                    .put("force", true)
                    .toString(),
            )
        } catch (_: Throwable) {
        } finally {
            preferences.edit().remove(VirtualDisplayHostContract.KEY_PROCESS_ID).apply()
        }
    }
}

private fun jsonObjectToMap(value: JSONObject): Map<String, Any?> {
    val result = linkedMapOf<String, Any?>()
    val keys = value.keys()
    while (keys.hasNext()) {
        val key = keys.next()
        result[key] = jsonValue(value.opt(key))
    }
    return result
}

private fun jsonValue(value: Any?): Any? {
    return when (value) {
        is JSONObject -> jsonObjectToMap(value)
        is JSONArray -> {
            val list = mutableListOf<Any?>()
            for (index in 0 until value.length()) {
                list.add(jsonValue(value.opt(index)))
            }
            list
        }
        else -> value
    }
}

internal class VirtualDisplayHostError(
    val code: String,
    message: String,
) : RuntimeException(message)
