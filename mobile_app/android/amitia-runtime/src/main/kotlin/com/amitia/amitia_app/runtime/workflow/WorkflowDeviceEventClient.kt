package com.amitia.amitia_app.runtime.workflow

import android.content.Context
import android.provider.Settings
import com.amitia.amitia_app.runtime.AndroidRuntimeModule
import com.amitia.amitia_app.runtime.connection.BackendConnectionAvailability
import com.amitia.amitia_app.runtime.internal.DefaultRuntimeModule
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URI

internal data class WorkflowWakeRuntimeStatus(
    val required: Boolean,
    val ready: Boolean,
    val bindingCount: Int,
    val configCount: Int,
    val reason: String,
    val deviceState: String,
    val deviceReason: String,
)

internal class WorkflowDeviceEventClient(
    context: Context,
) {
    private val appContext = context.applicationContext

    fun post(eventType: String, envelope: JSONObject): Result<Unit> = postJSON(
        path = "/api/local/workflows/events/${eventType.trim()}",
        body = envelope,
        maxBytes = 128 * 1024,
        errorLabel = "local workflow event",
    )

    fun postCapabilityStatus(body: JSONObject): Result<Unit> = postJSON(
        path = "/api/local/workflows/trigger-capabilities/status",
        body = body,
        maxBytes = 32 * 1024,
        errorLabel = "workflow trigger capability status",
    )

    fun postAppCatalog(body: JSONObject): Result<Unit> = postJSON(
        path = "/api/local/workflows/trigger-app-catalog/status",
        body = body,
        maxBytes = 512 * 1024,
        errorLabel = "workflow trigger app catalog",
    )

    fun postAndroidRuntimeHealth(body: JSONObject): Result<Unit> = postJSON(
        path = "/api/local/workflows/android-runtime-health/status",
        body = body,
        maxBytes = 16 * 1024,
        errorLabel = "Android workflow runtime health",
    )

    fun getWakeRuntimeStatus(): Result<WorkflowWakeRuntimeStatus> = runCatching {
        val connection = openLocalConnection(
            path = "/api/local/workflows/wake-runtime/status",
            method = "GET",
        )
        try {
            connection.setRequestProperty("Accept", "application/json")
            val status = connection.responseCode
            val responseBody = if (status in 200..299) {
                connection.inputStream.bufferedReader().use { it.readText() }
            } else {
                runCatching { connection.errorStream?.bufferedReader()?.use { it.readText() } }.getOrNull().orEmpty()
            }
            if (status !in 200..299) {
                error("workflow wake status rejected: HTTP $status ${responseBody.take(512)}")
            }
            val json = JSONObject(responseBody)
            WorkflowWakeRuntimeStatus(
                required = json.optBoolean("required", false),
                ready = json.optBoolean("ready", false),
                bindingCount = json.optInt("bindingCount", 0),
                configCount = json.optInt("configCount", 0),
                reason = json.optString("reason", "").trim(),
                deviceState = json.optString("deviceState", "").trim(),
                deviceReason = json.optString("deviceReason", "").trim(),
            )
        } finally {
            connection.disconnect()
        }
    }

    fun postWakeDeviceStatus(state: String, reason: String = ""): Result<Unit> = postJSON(
        path = "/api/local/workflows/wake-runtime/device-status",
        body = JSONObject().put("state", state.trim()).put("reason", reason.trim().take(512)),
        maxBytes = 4 * 1024,
        errorLabel = "workflow wake device status",
    )

    fun postWakeAudio(
        pcm: ByteArray,
        sequence: Long,
        capturedAtMs: Long,
    ): Result<Unit> = runCatching {
        require(pcm.isNotEmpty() && pcm.size % 2 == 0) { "wake audio must contain PCM16 samples" }
        require(pcm.size <= 64 * 1024) { "wake audio exceeds request limit" }
        val connection = openLocalConnection(
            path = "/api/local/workflows/wake-runtime/audio",
            method = "POST",
        )
        try {
            connection.doOutput = true
            connection.setRequestProperty("Content-Type", "application/octet-stream")
            connection.setRequestProperty("X-Amitia-Audio-Sequence", sequence.coerceAtLeast(0L).toString())
            connection.setRequestProperty("X-Amitia-Captured-At-Ms", capturedAtMs.toString())
            connection.setFixedLengthStreamingMode(pcm.size)
            connection.outputStream.use { it.write(pcm) }
            val status = connection.responseCode
            if (status !in 200..299) {
                val responseBody = runCatching { connection.errorStream?.bufferedReader()?.use { it.readText() } }.getOrNull().orEmpty()
                error("workflow wake audio rejected: HTTP $status ${responseBody.take(512)}")
            }
        } finally {
            connection.disconnect()
        }
    }

    private fun postJSON(path: String, body: JSONObject, maxBytes: Int, errorLabel: String): Result<Unit> = runCatching {
        val connection = openLocalConnection(path = path, method = "POST")
        try {
            connection.doOutput = true
            connection.setRequestProperty("Content-Type", "application/json")
            val bytes = body.toString().toByteArray(Charsets.UTF_8)
            require(bytes.size <= maxBytes) { "$errorLabel exceeds request limit" }
            connection.setFixedLengthStreamingMode(bytes.size)
            connection.outputStream.use { it.write(bytes) }
            val status = connection.responseCode
            if (status !in 200..299) {
                val responseBody = runCatching { connection.errorStream?.bufferedReader()?.use { it.readText() } }.getOrNull().orEmpty()
                error("$errorLabel rejected: HTTP $status ${responseBody.take(512)}")
            }
        } finally {
            connection.disconnect()
        }
    }

    private fun openLocalConnection(path: String, method: String): HttpURLConnection {
        val module = AndroidRuntimeModule.create(appContext) as? DefaultRuntimeModule
            ?: error("android runtime module unavailable")
        val availability = module.backendConnectionProvider.current()
        val descriptor = (availability as? BackendConnectionAvailability.Available)?.descriptor
            ?: error("local backend connection unavailable")
        val host = descriptor.host.trim()
        if (host != "127.0.0.1" && host != "localhost" && host != "::1") {
            error("workflow device requests require loopback backend")
        }
        val url = URI(descriptor.httpScheme, null, host, descriptor.port, path, null, null).toURL()
        return (url.openConnection() as HttpURLConnection).apply {
            requestMethod = method
            connectTimeout = 3000
            readTimeout = 5000
            useCaches = false
            setRequestProperty("X-Amitia-Local-Token", descriptor.credential.reveal())
            setRequestProperty("X-Amitia-Device-ID", trustedDeviceID())
        }
    }

    private fun trustedDeviceID(): String {
        val androidID = Settings.Secure.getString(appContext.contentResolver, Settings.Secure.ANDROID_ID)?.trim().orEmpty()
        if (androidID.isNotEmpty()) {
            return "android:$androidID"
        }
        val preferences = appContext.getSharedPreferences("amitia_runtime_identity", Context.MODE_PRIVATE)
        val existing = preferences.getString("device_id", null)?.trim().orEmpty()
        if (existing.isNotEmpty()) {
            return existing
        }
        val generated = "android-local:${java.util.UUID.randomUUID()}"
        preferences.edit().putString("device_id", generated).apply()
        return generated
    }
}
